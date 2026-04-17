// Package nats provides a NATS JetStream-based configuration provider that reads
// key-value pairs from a NATS KV bucket and watches for changes via either
// the NATS JetStream WatchAll API or periodic polling.
package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/provider"
)

// Config holds the connection and behavior parameters for the NATS provider.
type Config struct {
	URL          string
	Bucket       string
	Timeout      time.Duration
	Priority     int
	PollInterval time.Duration
}

// natsKV abstracts the NATS KV interface for testability.
type natsKV interface {
	Keys(...nats.WatchOpt) ([]string, error)
	Get(key string) (nats.KeyValueEntry, error)
	WatchAll(...nats.WatchOpt) (nats.KeyWatcher, error)
}

// Provider reads configuration key-value pairs from a NATS JetStream KV bucket.
// It supports both native JetStream watching (default) and periodic polling.
// Shared polling/diff/close logic is provided by BaseProvider.
type Provider struct {
	*provider.BaseProvider
	cfg Config
	kv  natsKV
	nc  *nats.Conn
}

var _ provider.Provider = (*Provider)(nil)

// New creates a new NATS provider with the given configuration.
// Default URL is "nats://localhost:4222" and default timeout is 5 seconds.
func New(cfg Config) (*Provider, error) {
	if cfg.URL == "" {
		cfg.URL = "nats://localhost:4222"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	p := &Provider{
		BaseProvider: provider.NewBaseProvider(64),
		cfg:          cfg,
	}
	p.PollInterval = cfg.PollInterval
	return p, nil
}

// initKV establishes a NATS connection and opens the KV bucket.
func (p *Provider) initKV() error {
	nc, err := nats.Connect(p.cfg.URL)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	p.nc = nc
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("nats JetStream: %w", err)
	}
	kv, err := js.KeyValue(p.cfg.Bucket)
	if err != nil {
		return fmt.Errorf("nats KV bucket %q: %w", p.cfg.Bucket, err)
	}
	p.kv = kv
	return nil
}

// Load fetches all key-value pairs from the configured NATS KV bucket.
func (p *Provider) Load(_ context.Context) (map[string]value.Value, error) {
	if err := p.EnsureOpen(); err != nil {
		return nil, err
	}
	p.Mu.Lock()
	if p.kv == nil {
		if err := p.initKV(); err != nil {
			p.Mu.Unlock()
			return nil, err
		}
	}
	kv := p.kv
	p.Mu.Unlock()

	keys, err := kv.Keys()
	if err != nil {
		return nil, fmt.Errorf("nats KV keys: %w", err)
	}
	result := make(map[string]value.Value, len(keys))
	for _, k := range keys {
		entry, entryErr := kv.Get(k)
		if entryErr != nil {
			continue
		}
		result[k] = value.New(
			string(entry.Value()),
			value.TypeString,
			value.SourceRemote,
			p.cfg.Priority,
		)
	}
	return result, nil
}

// Watch returns a channel of configuration change events.
// Uses periodic polling if PollInterval is configured; otherwise uses
// NATS JetStream's native WatchAll API.
func (p *Provider) Watch(ctx context.Context) (<-chan event.Event, error) {
	if err := p.EnsureOpen(); err != nil {
		return nil, err
	}
	if p.PollInterval > 0 {
		return p.PollLoadAndWatch(ctx, p.Load)
	}
	return p.watchMode(ctx)
}

// Health checks if the NATS connection is alive.
func (p *Provider) Health(_ context.Context) error {
	if p.nc == nil || !p.nc.IsConnected() {
		return fmt.Errorf("nats: not connected")
	}
	return nil
}

// Name returns "nats".
func (p *Provider) Name() string { return "nats" }

// Priority returns the provider priority.
func (p *Provider) Priority() int { return p.cfg.Priority }

// String returns a human-readable description including the NATS URL.
func (p *Provider) String() string { return "nats:" + p.cfg.URL }

// Close stops the watch goroutine, closes the NATS connection, and releases resources.
func (p *Provider) Close(ctx context.Context) error {
	if p.nc != nil {
		p.nc.Close()
	}
	return p.CloseProvider(ctx)
}

// watchMode uses NATS JetStream's WatchAll API for push-based change notification.
// This is NATS-specific and cannot be shared with other providers.
func (p *Provider) watchMode(ctx context.Context) (<-chan event.Event, error) {
	p.WatchMu.Lock()
	defer p.WatchMu.Unlock()
	p.Mu.Lock()
	if p.kv == nil {
		if err := p.initKV(); err != nil {
			p.Mu.Unlock()
			return nil, err
		}
	}
	kv := p.kv
	p.Mu.Unlock()

	watcher, err := kv.WatchAll()
	if err != nil {
		return nil, fmt.Errorf("nats KV watch: %w", err)
	}

	ch := make(chan event.Event, 64)
	go func() {
		var lastData map[string]value.Value
		for {
			select {
			case <-ctx.Done():
				_ = watcher.Stop()
				return
			case <-p.Done():
				_ = watcher.Stop()
				return
			case _, ok := <-watcher.Updates():
				if !ok {
					return
				}
				newData, loadErr := p.Load(ctx)
				if loadErr != nil {
					continue
				}
				if lastData != nil {
					evts := event.NewDiffEvents(lastData, newData)
					for i := range evts {
						select {
						case ch <- evts[i]:
						case <-ctx.Done():
							_ = watcher.Stop()
							return
						case <-p.Done():
							_ = watcher.Stop()
							return
						}
					}
				}
				lastData = newData
			}
		}
	}()
	return ch, nil
}
