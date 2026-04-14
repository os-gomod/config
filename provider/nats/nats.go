// Package nats provides a NATS JetStream KV store implementation of provider.Provider.
package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
	"github.com/os-gomod/config/provider"
)

// Config holds NATS connection parameters.
type Config struct {
	// URL is the NATS server URL, e.g. "nats://127.0.0.1:4222".
	URL string
	// Bucket is the JetStream KV bucket name.
	Bucket string
	// Timeout is the per-request timeout. Default: 5s.
	Timeout time.Duration
	// Priority is the merge priority of this provider.
	Priority int
	// PollInterval enables polling mode when > 0.
	// Zero means watch-based change notification (preferred).
	PollInterval time.Duration
}

// natsKV is an unexported interface wrapping the subset of the NATS KV API used.
// This allows test injection without the real NATS SDK.
type natsKV interface {
	Keys(...nats.WatchOpt) ([]string, error)
	Get(key string) (nats.KeyValueEntry, error)
	WatchAll(...nats.WatchOpt) (nats.KeyWatcher, error)
}

// Provider is a NATS JetStream KV-backed config provider.
//
// Lock ordering: mu before watchMu. The two locks are never held simultaneously
// in the current implementation; this ordering documents the safe acquisition
// sequence if that changes.
type Provider struct {
	cfg      Config
	mu       sync.Mutex // protects kv lazy-init
	watchMu  sync.Mutex // protects watch goroutine lifecycle
	closable *common.Closable
	kv       natsKV // unexported interface; allows test injection
	nc       *nats.Conn
	stopCh   chan struct{}
	eventCh  chan event.Event
}

var _ provider.Provider = (*Provider)(nil)

// New returns a NATS Provider. The NATS connection and KV bucket are created
// lazily on the first Load or Watch call.
func New(cfg Config) (*Provider, error) {
	if cfg.URL == "" {
		cfg.URL = "nats://127.0.0.1:4222"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Provider{
		cfg:      cfg,
		closable: common.NewClosable(),
		stopCh:   make(chan struct{}),
		eventCh:  make(chan event.Event, 64),
	}, nil
}

// Load implements provider.Provider. It fetches all keys from the KV bucket
// and flattens them into a value map.
func (p *Provider) Load(_ context.Context) (map[string]value.Value, error) {
	if p.closable.IsClosed() {
		return nil, errors.ErrClosed
	}
	p.mu.Lock()
	if p.kv == nil {
		if err := p.initKV(); err != nil {
			p.mu.Unlock()
			return nil, err
		}
	}
	kv := p.kv
	p.mu.Unlock()

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

// Watch implements provider.Provider. It uses NATS KV WatchAll by default,
// or polling if PollInterval is configured.
func (p *Provider) Watch(ctx context.Context) (<-chan event.Event, error) {
	if p.closable.IsClosed() {
		return nil, errors.ErrClosed
	}
	if p.cfg.PollInterval > 0 {
		return p.pollWatch(ctx)
	}
	return p.watchMode(ctx)
}

// Health implements provider.Provider. It verifies the NATS connection is alive.
func (p *Provider) Health(_ context.Context) error {
	if p.nc == nil || !p.nc.IsConnected() {
		return fmt.Errorf("nats: not connected")
	}
	return nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return "nats" }

// Priority implements provider.Provider.
func (p *Provider) Priority() int { return p.cfg.Priority }

// String implements provider.Provider.
func (p *Provider) String() string { return "nats:" + p.cfg.URL }

// Close implements provider.Provider.
func (p *Provider) Close(ctx context.Context) error {
	p.watchMu.Lock()
	defer p.watchMu.Unlock()
	select {
	case <-p.stopCh:
		// already closed
	default:
		close(p.stopCh)
	}
	if p.nc != nil {
		p.nc.Close()
	}
	return p.closable.Close(ctx)
}

// initKV creates the NATS connection and opens the KV bucket.
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

// pollWatch starts a polling-based watch.
func (p *Provider) pollWatch(ctx context.Context) (<-chan event.Event, error) {
	p.watchMu.Lock()
	defer p.watchMu.Unlock()
	ticker := time.NewTicker(p.cfg.PollInterval)
	go func() {
		defer ticker.Stop()
		var lastData map[string]value.Value
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case <-ticker.C:
				data, err := p.Load(ctx)
				if err != nil {
					continue
				}
				if lastData != nil {
					_ = p.emitDiff(ctx, lastData, data)
				}
				lastData = data
			}
		}
	}()
	return p.eventCh, nil
}

// watchMode starts a NATS KV WatchAll-based change notification.
func (p *Provider) watchMode(ctx context.Context) (<-chan event.Event, error) {
	p.watchMu.Lock()
	defer p.watchMu.Unlock()
	p.mu.Lock()
	if p.kv == nil {
		if err := p.initKV(); err != nil {
			p.mu.Unlock()
			return nil, err
		}
	}
	kv := p.kv
	p.mu.Unlock()

	watcher, err := kv.WatchAll()
	if err != nil {
		return nil, fmt.Errorf("nats KV watch: %w", err)
	}

	go func() {
		var lastData map[string]value.Value
		for {
			select {
			case <-ctx.Done():
				_ = watcher.Stop()
				return
			case <-p.stopCh:
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
					_ = p.emitDiff(ctx, lastData, newData)
				}
				lastData = newData
			}
		}
	}()
	return p.eventCh, nil
}

// emitDiff computes and emits diff events between old and new data.
func (p *Provider) emitDiff(ctx context.Context, old, newData map[string]value.Value) error {
	for _, evt := range event.NewDiffEvents(old, newData) {
		select {
		case p.eventCh <- evt:
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopCh:
			return nil
		}
	}
	return nil
}
