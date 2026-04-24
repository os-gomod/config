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
	"github.com/os-gomod/config/internal/providerbase"
	"github.com/os-gomod/config/provider"
)

// Config holds the NATS Key-Value provider configuration.
type Config struct {
	// URL is the NATS server URL. Defaults to nats://localhost:4222.
	URL string
	// Bucket is the JetStream KV bucket name. Required.
	Bucket string
	// Timeout is the connection and operation timeout. Defaults to 5s.
	Timeout time.Duration
	// Priority determines merge order. Higher values win.
	Priority int
	// PollInterval enables polling-based watching when > 0.
	PollInterval time.Duration
}

// natsKV abstracts the NATS Key-Value store for testability.
type natsKV interface {
	Keys(...nats.WatchOpt) ([]string, error)
	Get(key string) (nats.KeyValueEntry, error)
	WatchAll(...nats.WatchOpt) (nats.KeyWatcher, error)
}

// Provider is a NATS Key-Value configuration provider backed by RemoteProvider.
// It is a type alias — all methods (Load, Watch, Health, Close, etc.) are
// provided by the generic RemoteProvider[natsKV].
type Provider struct {
	*providerbase.RemoteProvider[natsKV]
	cfg Config
}

var _ provider.Provider = (*Provider)(nil)

// New creates a new NATS Key-Value provider.
// The returned provider lazily connects to NATS on first Load/Health call.
func New(cfg *Config) (*Provider, error) {
	if cfg.URL == "" {
		cfg.URL = "nats://localhost:4222"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}

	var nc *nats.Conn

	p := &Provider{
		cfg: *cfg,
	}

	p.RemoteProvider = providerbase.New(providerbase.Config[natsKV]{
		Name:         "nats",
		Priority:     cfg.Priority,
		PollInterval: cfg.PollInterval,
		StringFormat: "nats:" + cfg.URL,
		InitFn: func() (natsKV, error) {
			conn, err := nats.Connect(cfg.URL)
			if err != nil {
				return nil, fmt.Errorf("nats connect: %w", err)
			}
			nc = conn
			js, err := conn.JetStream()
			if err != nil {
				return nil, fmt.Errorf("nats JetStream: %w", err)
			}
			kv, err := js.KeyValue(cfg.Bucket)
			if err != nil {
				return nil, fmt.Errorf("nats KV bucket %q: %w", cfg.Bucket, err)
			}
			return kv, nil
		},
		FetchFn: func(_ context.Context, kv natsKV) (map[string]value.Value, error) {
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
					cfg.Priority,
				)
			}
			return result, nil
		},
		HealthFn: func(_ context.Context, _ natsKV) error {
			if nc == nil || !nc.IsConnected() {
				return fmt.Errorf("nats: not connected")
			}
			return nil
		},
		CloseFn: func(_ natsKV) error {
			if nc != nil {
				nc.Close()
			}
			return nil
		},
		NativeWatchFn: func(ctx context.Context, kv natsKV) (<-chan struct{}, error) {
			watcher, err := kv.WatchAll()
			if err != nil {
				return nil, fmt.Errorf("nats KV watch: %w", err)
			}
			trigger := make(chan struct{}, 1)
			go func() {
				defer func() { _ = watcher.Stop() }()
				for {
					select {
					case <-ctx.Done():
						return
					case _, ok := <-watcher.Updates():
						if !ok {
							return
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
