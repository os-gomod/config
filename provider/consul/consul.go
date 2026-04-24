// Package consul provides a Consul-based configuration provider that reads
// key-value pairs from a Consul agent and watches for changes via either
// long-polling (blocking queries) or periodic polling.
package consul

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/internal/keyutil"
	"github.com/os-gomod/config/internal/providerbase"
	"github.com/os-gomod/config/provider"
)

// Config holds the Consul KV provider configuration.
type Config struct {
	// Address is the Consul agent address. Defaults to 127.0.0.1:8500.
	Address string
	// Datacenter is the Consul datacenter. Optional.
	Datacenter string
	// Token is the Consul ACL token. Optional.
	Token string
	// TLSConfig is the optional TLS configuration for the Consul connection.
	TLSConfig *tls.Config
	// Timeout is the HTTP timeout. Defaults to 5s.
	Timeout time.Duration
	// Prefix is the KV path prefix to watch. Defaults to "".
	Prefix string
	// Priority determines merge order. Higher values win.
	Priority int
	// PollInterval enables polling-based watching when > 0.
	PollInterval time.Duration
	// Retry configures retry behavior for failed operations.
	Retry providerbase.RetryConfig

	testClient consulClient
}

// kvPair represents a single Consul KV pair.
type kvPair struct {
	Key   string
	Value string
}

// consulClient abstracts the Consul KV API for testability.
type consulClient interface {
	KVList(prefix string, index uint64, timeout time.Duration) ([]kvPair, uint64, error)
}

// Provider is a Consul KV configuration provider backed by RemoteProvider.
// It is a type alias — all methods are provided by RemoteProvider[consulClient].
type Provider struct {
	*providerbase.RemoteProvider[consulClient]
	cfg Config
}

var _ provider.Provider = (*Provider)(nil)

// New creates a new Consul KV provider using the official hashicorp/consul/api.
// The returned provider lazily initializes the HTTP client on first Load/Health call.
func New(cfg *Config) (*Provider, error) {
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:8500"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}

	prefix := cfg.Prefix
	timeout := cfg.Timeout
	priority := cfg.Priority

	p := &Provider{
		cfg: *cfg,
	}

	p.RemoteProvider = providerbase.New(providerbase.Config[consulClient]{
		Name:         "consul",
		Priority:     cfg.Priority,
		PollInterval: cfg.PollInterval,
		StringFormat: "consul:" + cfg.Address,
		Retry:        cfg.Retry,
		InitFn: func() (consulClient, error) {
			if cfg.testClient != nil {
				return cfg.testClient, nil
			}
			return newAPIClient(cfg)
		},
		FetchFn: func(_ context.Context, cli consulClient) (map[string]value.Value, error) {
			pairs, _, err := cli.KVList(prefix, 0, timeout)
			if err != nil {
				return nil, fmt.Errorf("consul KV list: %w", err)
			}
			result := make(map[string]value.Value, len(pairs))
			for _, kv := range pairs {
				key := keyutil.FlattenProviderKey(kv.Key, prefix)
				if key == "" {
					continue
				}
				result[key] = value.New(kv.Value, value.TypeString, value.SourceRemote, priority)
			}
			return result, nil
		},
		HealthFn: func(_ context.Context, cli consulClient) error {
			_, _, err := cli.KVList(prefix, 0, timeout)
			if err != nil {
				return fmt.Errorf("consul health check: %w", err)
			}
			return nil
		},
	})

	return p, nil
}

// apiClient wraps the official github.com/hashicorp/consul/api KV client
// to satisfy the consulClient interface for production use.
type apiClient struct {
	kv *api.KV
}

// newAPIClient creates a real Consul API client from the provider configuration.
// It configures the address, datacenter, token, and TLS settings.
func newAPIClient(cfg *Config) (*apiClient, error) {
	consulCfg := api.DefaultConfig()
	consulCfg.Address = cfg.Address
	if cfg.Datacenter != "" {
		consulCfg.Datacenter = cfg.Datacenter
	}
	if cfg.Token != "" {
		consulCfg.Token = cfg.Token
	}
	if cfg.TLSConfig != nil {
		consulCfg.Transport.TLSClientConfig = cfg.TLSConfig.Clone()
	}
	client, err := api.NewClient(consulCfg)
	if err != nil {
		return nil, fmt.Errorf("consul api client: %w", err)
	}
	return &apiClient{kv: client.KV()}, nil
}

// KVList retrieves all KV pairs under the given prefix using the Consul HTTP API.
// It supports blocking queries via the WaitIndex parameter for long-poll watching.
//
// Parameters:
//   - prefix: the KV path prefix (e.g., "config/")
//   - index:  the last known ModifyIndex for blocking queries (0 = no blocking)
//   - timeout: the maximum duration to wait for a blocking query
//
// Returns the list of KV pairs, the latest ModifyIndex, and any error.
func (a *apiClient) KVList(prefix string, index uint64, timeout time.Duration) ([]kvPair, uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	qOpts := (&api.QueryOptions{
		WaitIndex: index,
	}).WithContext(ctx)

	pairs, qm, err := a.kv.List(prefix, qOpts)
	if err != nil {
		return nil, 0, err
	}

	result := make([]kvPair, len(pairs))
	for i, p := range pairs {
		result[i] = kvPair{
			Key:   p.Key,
			Value: string(p.Value),
		}
	}
	return result, qm.LastIndex, nil
}
