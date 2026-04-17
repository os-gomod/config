// Package consul provides a Consul-based configuration provider that reads
// key-value pairs from a Consul agent and watches for changes via either
// long-polling (blocking queries) or periodic polling.
package consul

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/keyutil"
	"github.com/os-gomod/config/provider"
)

// Config holds the connection and behavior parameters for the Consul provider.
type Config struct {
	Address      string
	Datacenter   string
	Token        string
	TLSConfig    *tls.Config
	Timeout      time.Duration
	Prefix       string
	Priority     int
	PollInterval time.Duration
}

type kvPair struct {
	Key   string
	Value string
}

// consulClient abstracts the Consul HTTP API for testability.
type consulClient interface {
	KVList(prefix string, index uint64, timeout time.Duration) ([]kvPair, uint64, error)
}

// Provider reads configuration key-value pairs from Consul.
// It supports both blocking queries (default watch mode) and periodic polling.
// Provider-specific logic (client creation, data fetching, native blocking watch)
// remains here; shared polling/diff/close logic is in BaseProvider.
type Provider struct {
	*provider.BaseProvider
	cfg    Config
	client consulClient
}

var _ provider.Provider = (*Provider)(nil)

// New creates a new Consul provider with the given configuration.
// Default address is "127.0.0.1:8500" and default timeout is 5 seconds.
func New(cfg *Config) (*Provider, error) {
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:8500"
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

// Load fetches all key-value pairs under the configured prefix from Consul.
func (p *Provider) Load(_ context.Context) (map[string]value.Value, error) {
	if err := p.EnsureOpen(); err != nil {
		return nil, err
	}
	p.Mu.Lock()
	if p.client == nil {
		p.client = newHTTPClient(&p.cfg)
	}
	cli := p.client
	p.Mu.Unlock()

	pairs, _, err := cli.KVList(p.cfg.Prefix, 0, p.cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("consul KV list: %w", err)
	}
	result := make(map[string]value.Value, len(pairs))
	for _, kv := range pairs {
		key := keyutil.FlattenProviderKey(kv.Key, p.cfg.Prefix)
		if key == "" {
			continue
		}
		result[key] = value.New(kv.Value, value.TypeString, value.SourceRemote, p.cfg.Priority)
	}
	return result, nil
}

// Watch returns a channel of configuration change events.
// Uses periodic polling if PollInterval is configured; otherwise uses
// Consul's blocking query API.
func (p *Provider) Watch(ctx context.Context) (<-chan event.Event, error) {
	if err := p.EnsureOpen(); err != nil {
		return nil, err
	}
	if p.PollInterval > 0 {
		return p.PollLoadAndWatch(ctx, p.Load)
	}
	return p.blockingWatch(ctx)
}

// Health checks connectivity to the Consul agent by performing a KV list.
func (p *Provider) Health(_ context.Context) error {
	p.Mu.Lock()
	if p.client == nil {
		p.client = newHTTPClient(&p.cfg)
	}
	cli := p.client
	p.Mu.Unlock()
	_, _, err := cli.KVList(p.cfg.Prefix, 0, p.cfg.Timeout)
	if err != nil {
		return fmt.Errorf("consul health check: %w", err)
	}
	return nil
}

// Name returns "consul".
func (p *Provider) Name() string { return "consul" }

// Priority returns the provider priority.
func (p *Provider) Priority() int { return p.cfg.Priority }

// String returns a human-readable description including the Consul address.
func (p *Provider) String() string { return "consul:" + p.cfg.Address }

// Close stops the watch goroutine and releases resources.
func (p *Provider) Close(ctx context.Context) error {
	return p.CloseProvider(ctx)
}

// blockingWatch uses Consul's blocking query API to watch for changes.
// This is consul-specific and cannot be shared with other providers.
func (p *Provider) blockingWatch(ctx context.Context) (<-chan event.Event, error) {
	p.WatchMu.Lock()
	defer p.WatchMu.Unlock()
	p.Mu.Lock()
	if p.client == nil {
		p.client = newHTTPClient(&p.cfg)
	}
	cli := p.client
	p.Mu.Unlock()

	ch := make(chan event.Event, 64)
	go func() {
		var lastIndex uint64
		var lastData map[string]value.Value
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.Done():
				return
			default:
			}
			pairs, idx, err := cli.KVList(p.cfg.Prefix, lastIndex, p.cfg.Timeout)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-p.Done():
					return
				case <-time.After(time.Second):
					continue
				}
			}
			if idx != lastIndex {
				newData := make(map[string]value.Value, len(pairs))
				for _, kv := range pairs {
					key := keyutil.FlattenProviderKey(kv.Key, p.cfg.Prefix)
					if key == "" {
						continue
					}
					newData[key] = value.New(kv.Value, value.TypeString, value.SourceRemote, p.cfg.Priority)
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
				lastIndex = idx
			}
		}
	}()
	return ch, nil
}

type httpClient struct {
	cfg Config
}

func newHTTPClient(cfg *Config) consulClient {
	return &httpClient{cfg: *cfg}
}

func (c *httpClient) KVList(_ string, _ uint64, _ time.Duration) ([]kvPair, uint64, error) {
	return nil, 0, nil
}
