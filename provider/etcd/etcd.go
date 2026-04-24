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
	"github.com/os-gomod/config/internal/keyutil"
	"github.com/os-gomod/config/internal/providerbase"
	"github.com/os-gomod/config/provider"
)

// Config holds the etcd provider configuration.
type Config struct {
	// Endpoints is the list of etcd server addresses. Defaults to [127.0.0.1:2379].
	Endpoints []string
	// Username for etcd authentication. Optional.
	Username string
	// Password for etcd authentication. Optional.
	Password string
	// Timeout is the connection and operation timeout. Defaults to 5s.
	Timeout time.Duration
	// Prefix is the key prefix to watch. Defaults to "".
	Prefix string
	// Priority determines merge order. Higher values win.
	Priority int
	// PollInterval enables polling-based watching when > 0.
	// If both PollInterval and native watch are configured,
	// polling takes precedence.
	PollInterval time.Duration
}

// etcdClient abstracts the etcd client API for testability.
type etcdClient interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

// Provider is an etcd configuration provider backed by RemoteProvider.
// It is a type alias — all methods are provided by RemoteProvider[etcdClient].
type Provider struct {
	*providerbase.RemoteProvider[etcdClient]
	cfg Config
}

var _ provider.Provider = (*Provider)(nil)

// New creates a new etcd configuration provider.
// The returned provider lazily connects to etcd on first Load/Health call.
func New(cfg *Config) (*Provider, error) {
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"127.0.0.1:2379"}
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}

	prefix := cfg.Prefix
	timeout := cfg.Timeout
	priority := cfg.Priority
	endpoints := cfg.Endpoints
	username := cfg.Username
	password := cfg.Password

	p := &Provider{
		cfg: *cfg,
	}

	p.RemoteProvider = providerbase.New(providerbase.Config[etcdClient]{
		Name:         "etcd",
		Priority:     cfg.Priority,
		PollInterval: cfg.PollInterval,
		StringFormat: "etcd:" + strings.Join(cfg.Endpoints, ","),
		InitFn: func() (etcdClient, error) {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   endpoints,
				Username:    username,
				Password:    password,
				DialTimeout: timeout,
			})
			if err != nil {
				return nil, fmt.Errorf("etcd client init: %w", err)
			}
			return cli, nil
		},
		FetchFn: func(ctx context.Context, cli etcdClient) (map[string]value.Value, error) {
			resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix())
			if err != nil {
				return nil, fmt.Errorf("etcd get: %w", err)
			}
			result := make(map[string]value.Value, len(resp.Kvs))
			for _, kv := range resp.Kvs {
				key := keyutil.FlattenProviderKey(string(kv.Key), prefix)
				if key == "" {
					continue
				}
				result[key] = value.New(
					string(kv.Value),
					value.TypeString,
					value.SourceRemote,
					priority,
				)
			}
			return result, nil
		},
		HealthFn: func(ctx context.Context, cli etcdClient) error {
			healthCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			_, err := cli.Get(healthCtx, prefix, clientv3.WithCountOnly())
			if err != nil {
				return fmt.Errorf("etcd health check: %w", err)
			}
			return nil
		},
		CloseFn: func(cli etcdClient) error {
			return cli.Close()
		},
		NativeWatchFn: func(ctx context.Context, cli etcdClient) (<-chan struct{}, error) {
			watchCh := cli.Watch(ctx, prefix, clientv3.WithPrefix())
			trigger := make(chan struct{}, 1)
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case resp, ok := <-watchCh:
						if !ok {
							return
						}
						if err := resp.Err(); err != nil {
							continue
						}
						select {
						case trigger <- struct{}{}:
						default:
						}
					}
				}
			}()
			return trigger, nil
		},
	})

	return p, nil
}
