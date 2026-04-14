// Package etcd provides an etcd v3 KV store implementation of provider.Provider.
package etcd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
	"github.com/os-gomod/config/provider"
)

// Config holds etcd connection parameters.
type Config struct {
	// Endpoints is the list of etcd cluster endpoints.
	Endpoints []string
	// Username for authentication; empty means unauthenticated.
	Username string
	// Password for authentication.
	Password string
	// Timeout is the per-request timeout. Default: 5s.
	Timeout time.Duration
	// Prefix is the key prefix, e.g. "/myapp/config/".
	Prefix string
	// Priority is the merge priority of this provider.
	Priority int
	// PollInterval enables polling mode when > 0.
	// Zero means watch-based change notification (preferred).
	PollInterval time.Duration
}

// etcdClient is an unexported interface wrapping the subset of the etcd v3
// API used. This allows test injection without the real etcd SDK.
type etcdClient interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

// Provider is an etcd-backed config provider.
//
// Lock ordering: mu before watchMu. The two locks are never held simultaneously
// in the current implementation; this ordering documents the safe acquisition
// sequence if that changes.
type Provider struct {
	cfg      Config
	mu       sync.Mutex // protects client lazy-init
	watchMu  sync.Mutex // protects watch goroutine lifecycle
	closable *common.Closable
	client   etcdClient // unexported interface; allows test injection
	stopCh   chan struct{}
	eventCh  chan event.Event
}

var _ provider.Provider = (*Provider)(nil)

// New returns an etcd Provider. The etcd client is created lazily on the first
// Load or Watch call.
func New(cfg Config) (*Provider, error) {
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"127.0.0.1:2379"}
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

// ensureClient lazily creates the etcd client if it hasn't been created yet.
// It must be called while holding p.mu.
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

// Load implements provider.Provider. It fetches all keys under the configured
// prefix and flattens them into a value map.
func (p *Provider) Load(ctx context.Context) (map[string]value.Value, error) {
	if p.closable.IsClosed() {
		return nil, errors.ErrClosed
	}
	p.mu.Lock()
	if err := p.ensureClient(); err != nil {
		p.mu.Unlock()
		return nil, err
	}
	cli := p.client
	p.mu.Unlock()

	resp, err := cli.Get(ctx, p.cfg.Prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get: %w", err)
	}

	result := make(map[string]value.Value, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := flattenKey(string(kv.Key), p.cfg.Prefix)
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

// Watch implements provider.Provider. It uses etcd watch with prefix by default,
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

// Health implements provider.Provider. It verifies the etcd cluster is reachable.
func (p *Provider) Health(ctx context.Context) error {
	p.mu.Lock()
	if err := p.ensureClient(); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("etcd health: %w", err)
	}
	cli := p.client
	p.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	_, err := cli.Get(ctx, p.cfg.Prefix, clientv3.WithCountOnly())
	if err != nil {
		return fmt.Errorf("etcd health check: %w", err)
	}
	return nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return "etcd" }

// Priority implements provider.Provider.
func (p *Provider) Priority() int { return p.cfg.Priority }

// String implements provider.Provider.
func (p *Provider) String() string { return "etcd:" + strings.Join(p.cfg.Endpoints, ",") }

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
	if p.client != nil {
		_ = p.client.Close()
	}
	return p.closable.Close(ctx)
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

// watchMode starts an etcd Watch-based change notification.
func (p *Provider) watchMode(ctx context.Context) (<-chan event.Event, error) {
	p.watchMu.Lock()
	defer p.watchMu.Unlock()
	p.mu.Lock()
	if err := p.ensureClient(); err != nil {
		p.mu.Unlock()
		return nil, err
	}
	cli := p.client
	p.mu.Unlock()

	watchCh := cli.Watch(ctx, p.cfg.Prefix, clientv3.WithPrefix())
	go func() {
		var lastData map[string]value.Value
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
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

// flattenKey strips the prefix, replaces "/" with ".", and lowercases the key.
// Example: "/myapp/config/db/host" with prefix "/myapp/config/" -> "db.host".
func flattenKey(key, prefix string) string {
	key = strings.TrimPrefix(key, prefix)
	key = strings.ReplaceAll(key, "/", ".")
	key = strings.ToLower(key)
	key = strings.Trim(key, ".")
	return key
}
