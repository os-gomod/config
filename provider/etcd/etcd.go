// Package etcd provides an etcd-based configuration provider that reads
// key-value pairs from an etcd cluster and watches for changes via either
// the native etcd Watch API or periodic polling.
package etcd

import (
	"context"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/keyutil"
	"github.com/os-gomod/config/provider"
)

// Config holds the connection and behavior parameters for the etcd provider.
type Config struct {
	Endpoints    []string
	Username     string
	Password     string
	Timeout      time.Duration
	Prefix       string
	Priority     int
	PollInterval time.Duration
}

// etcdClient abstracts the etcd client for testability.
type etcdClient interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

// Provider reads configuration key-value pairs from etcd.
// It supports both native etcd Watch API (default) and periodic polling.
// Shared polling/diff/close logic is provided by BaseProvider.
type Provider struct {
	*provider.BaseProvider
	cfg    Config
	client etcdClient
}

var _ provider.Provider = (*Provider)(nil)

// New creates a new etcd provider with the given configuration.
// Default endpoint is "127.0.0.1:2379" and default timeout is 5 seconds.
func New(cfg *Config) (*Provider, error) {
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"127.0.0.1:2379"}
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	p := &Provider{
		BaseProvider: provider.NewBaseProvider(64),
		cfg:          *cfg,
	}
	p.PollInterval = cfg.PollInterval
	return p, nil
}

// ensureClient lazily creates an etcd client connection.
func (p *Provider) ensureClient() error {
	if p.client != nil {
		return nil
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   p.cfg.Endpoints,
		Username:    p.cfg.Username,
		Password:    p.cfg.Password,
		DialTimeout: p.cfg.Timeout,
	})
	if err != nil {
		return fmt.Errorf("etcd client init: %w", err)
	}
	p.client = cli
	return nil
}

// Load fetches all key-value pairs under the configured prefix from etcd.
func (p *Provider) Load(ctx context.Context) (map[string]value.Value, error) {
	if err := p.EnsureOpen(); err != nil {
		return nil, err
	}
	p.Mu.Lock()
	if err := p.ensureClient(); err != nil {
		p.Mu.Unlock()
		return nil, err
	}
	cli := p.client
	p.Mu.Unlock()

	resp, err := cli.Get(ctx, p.cfg.Prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get: %w", err)
	}
	result := make(map[string]value.Value, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := keyutil.FlattenProviderKey(string(kv.Key), p.cfg.Prefix)
		if key == "" {
			continue
		}
		result[key] = value.New(
			string(kv.Value),
			value.TypeString,
			value.SourceRemote,
			p.cfg.Priority,
		)
	}
	return result, nil
}

// Watch returns a channel of configuration change events.
// Uses periodic polling if PollInterval is configured; otherwise uses
// etcd's native Watch API.
func (p *Provider) Watch(ctx context.Context) (<-chan event.Event, error) {
	if err := p.EnsureOpen(); err != nil {
		return nil, err
	}
	if p.PollInterval > 0 {
		return p.PollLoadAndWatch(ctx, p.Load)
	}
	return p.watchMode(ctx)
}

// Health checks connectivity to the etcd cluster by performing a prefix count.
func (p *Provider) Health(ctx context.Context) error {
	p.Mu.Lock()
	if err := p.ensureClient(); err != nil {
		p.Mu.Unlock()
		return fmt.Errorf("etcd health: %w", err)
	}
	cli := p.client
	p.Mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	_, err := cli.Get(ctx, p.cfg.Prefix, clientv3.WithCountOnly())
	if err != nil {
		return fmt.Errorf("etcd health check: %w", err)
	}
	return nil
}

// Name returns "etcd".
func (p *Provider) Name() string { return "etcd" }

// Priority returns the provider priority.
func (p *Provider) Priority() int { return p.cfg.Priority }

// String returns a human-readable description including the etcd endpoints.
func (p *Provider) String() string { return "etcd:" + strings.Join(p.cfg.Endpoints, ",") }

// Close stops the watch goroutine, closes the etcd client, and releases resources.
func (p *Provider) Close(ctx context.Context) error {
	if p.client != nil {
		_ = p.client.Close()
	}
	return p.CloseProvider(ctx)
}

// watchMode uses etcd's native Watch API for push-based change notification.
// This is etcd-specific and cannot be shared with other providers.
func (p *Provider) watchMode(ctx context.Context) (<-chan event.Event, error) {
	p.WatchMu.Lock()
	defer p.WatchMu.Unlock()
	p.Mu.Lock()
	if err := p.ensureClient(); err != nil {
		p.Mu.Unlock()
		return nil, err
	}
	cli := p.client
	p.Mu.Unlock()

	watchCh := cli.Watch(ctx, p.cfg.Prefix, clientv3.WithPrefix())
	ch := make(chan event.Event, 64)
	go func() {
		var lastData map[string]value.Value
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.Done():
				return
			case resp, ok := <-watchCh:
				if !ok {
					return
				}
				if err := resp.Err(); err != nil {
					continue
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
							return
						case <-p.Done():
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
