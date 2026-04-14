// Package consul provides a Consul KV store implementation of provider.Provider.
// It uses Consul's blocking-query API for push-style change notification.
package consul

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
	"github.com/os-gomod/config/provider"
)

// Config holds Consul connection parameters.
type Config struct {
	// Address is the Consul agent address, e.g. "127.0.0.1:8500".
	Address string
	// Datacenter selects the target datacenter.
	Datacenter string
	// Token is the Consul ACL token; empty means unauthenticated.
	Token string
	// TLSConfig enables TLS. Nil means plaintext.
	TLSConfig *tls.Config
	// Timeout is the per-request timeout. Default: 5s.
	Timeout time.Duration
	// Prefix is the KV path prefix, e.g. "myapp/config/".
	Prefix string
	// Priority is the merge priority of this provider.
	Priority int
	// PollInterval enables polling mode when > 0.
	// Zero means blocking-query (preferred) watch mode.
	PollInterval time.Duration
}

// kvPair represents a single Consul KV entry for test injection.
type kvPair struct {
	Key   string
	Value string
}

// consulClient is an unexported interface wrapping the subset of the Consul
// HTTP API used. This allows test injection without the real Consul SDK.
type consulClient interface {
	KVList(prefix string, index uint64, timeout time.Duration) ([]kvPair, uint64, error)
}

// Provider is a Consul-backed config provider.
//
// Lock ordering: mu before watchMu. The two locks are never held simultaneously
// in the current implementation; this ordering documents the safe acquisition
// sequence if that changes.
type Provider struct {
	cfg      Config
	mu       sync.Mutex // protects client lazy-init
	watchMu  sync.Mutex // protects watch goroutine lifecycle
	closable *common.Closable
	client   consulClient // unexported interface; allows test injection
	stopCh   chan struct{}
	eventCh  chan event.Event
}

var _ provider.Provider = (*Provider)(nil)

// New returns a Consul Provider. The Consul HTTP client is created lazily
// on the first Load or Watch call.
func New(cfg Config) (*Provider, error) {
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:8500"
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

// Load implements provider.Provider. It fetches all KV pairs under the
// configured prefix and flattens them into a value map.
func (p *Provider) Load(_ context.Context) (map[string]value.Value, error) {
	if p.closable.IsClosed() {
		return nil, errors.ErrClosed
	}
	p.mu.Lock()
	if p.client == nil {
		p.client = newHTTPClient(p.cfg)
	}
	cli := p.client
	p.mu.Unlock()

	pairs, _, err := cli.KVList(p.cfg.Prefix, 0, p.cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("consul KV list: %w", err)
	}

	result := make(map[string]value.Value, len(pairs))
	for _, kv := range pairs {
		key := flattenKey(kv.Key, p.cfg.Prefix)
		if key == "" {
			continue
		}
		result[key] = value.New(kv.Value, value.TypeString, value.SourceRemote, p.cfg.Priority)
	}
	return result, nil
}

// Watch implements provider.Provider. It uses Consul blocking queries by default
// or polling if PollInterval is configured.
func (p *Provider) Watch(ctx context.Context) (<-chan event.Event, error) {
	if p.closable.IsClosed() {
		return nil, errors.ErrClosed
	}
	if p.cfg.PollInterval > 0 {
		return p.pollWatch(ctx)
	}
	return p.blockingWatch(ctx)
}

// Health implements provider.Provider. It verifies the Consul agent is reachable.
func (p *Provider) Health(_ context.Context) error {
	p.mu.Lock()
	if p.client == nil {
		p.client = newHTTPClient(p.cfg)
	}
	cli := p.client
	p.mu.Unlock()

	_, _, err := cli.KVList(p.cfg.Prefix, 0, p.cfg.Timeout)
	if err != nil {
		return fmt.Errorf("consul health check: %w", err)
	}
	return nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return "consul" }

// Priority implements provider.Provider.
func (p *Provider) Priority() int { return p.cfg.Priority }

// String implements provider.Provider.
func (p *Provider) String() string { return "consul:" + p.cfg.Address }

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

// blockingWatch starts a blocking-query-based watch.
func (p *Provider) blockingWatch(ctx context.Context) (<-chan event.Event, error) {
	p.watchMu.Lock()
	defer p.watchMu.Unlock()
	p.mu.Lock()
	if p.client == nil {
		p.client = newHTTPClient(p.cfg)
	}
	cli := p.client
	p.mu.Unlock()

	go func() {
		var lastIndex uint64
		var lastData map[string]value.Value
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			default:
			}
			pairs, idx, err := cli.KVList(p.cfg.Prefix, lastIndex, p.cfg.Timeout)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-p.stopCh:
					return
				case <-time.After(time.Second):
					continue
				}
			}
			if idx != lastIndex {
				newData := make(map[string]value.Value, len(pairs))
				for _, kv := range pairs {
					key := flattenKey(kv.Key, p.cfg.Prefix)
					if key == "" {
						continue
					}
					newData[key] = value.New(
						kv.Value,
						value.TypeString,
						value.SourceRemote,
						p.cfg.Priority,
					)
				}
				if lastData != nil {
					_ = p.emitDiff(ctx, lastData, newData)
				}
				lastData = newData
				lastIndex = idx
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
// Example: "myapp/config/db/host" with prefix "myapp/config/" -> "db.host".
func flattenKey(key, prefix string) string {
	key = strings.TrimPrefix(key, prefix)
	key = strings.ReplaceAll(key, "/", ".")
	key = strings.ToLower(key)
	key = strings.Trim(key, ".")
	return key
}

// httpClient is the default consulClient implementation that makes real HTTP calls.
// It is a placeholder that returns empty results; a full implementation would
// use the Consul SDK. The real SDK is added via a consul build-tagged file.
type httpClient struct {
	cfg Config
}

// newHTTPClient creates a default HTTP-based consul client.
func newHTTPClient(cfg Config) consulClient {
	return &httpClient{cfg: cfg}
}

// KVList implements consulClient. This default implementation returns an empty
// result. The real implementation is provided via a consul build-tagged file.
func (c *httpClient) KVList(_ string, _ uint64, _ time.Duration) ([]kvPair, uint64, error) {
	return nil, 0, nil
}
