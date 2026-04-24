// Package provider defines the Provider interface for remote configuration sources
// and provides a BaseProvider struct that eliminates boilerplate shared across
// all provider implementations (consul, etcd, nats, etc.).
package provider

import (
	"context"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/pollwatch"
	"github.com/os-gomod/config/internal/providerbase"
)

// Provider is the interface that all configuration providers must implement.
// Providers fetch configuration data from remote sources (NATS, Consul, etcd)
// and optionally watch for changes.
type Provider interface {
	// Load fetches the current configuration state from the remote source.
	Load(ctx context.Context) (map[string]value.Value, error)

	// Watch returns a channel of configuration change events. Implementations
	// may use polling (if PollInterval is set) or native push-based watching.
	Watch(ctx context.Context) (<-chan event.Event, error)

	// Health checks the connectivity to the remote source.
	Health(ctx context.Context) error

	// Name returns the unique provider identifier (e.g., "consul", "etcd").
	Name() string

	// Priority returns the priority of this provider's configuration values.
	// Higher values take precedence during merge.
	Priority() int

	// String returns a human-readable description of the provider.
	String() string

	// Close releases all resources held by the provider.
	Close(ctx context.Context) error
}

// BaseProvider provides default no-op implementations of the Provider interface.
// It is retained for backward compatibility. New providers should use
// providerbase.RemoteProvider[T] which eliminates all boilerplate.
//
// Deprecated: Use providerbase.RemoteProvider[T] instead.
type BaseProvider struct {
	*providerbase.ProviderLifecycle
	PollInterval time.Duration
	providerName string
	priority     int
}

// NewBaseProvider creates a BaseProvider with the given event buffer size.
//
// Deprecated: Use providerbase.New[ClientType](...) instead.
func NewBaseProvider(bufSize int) *BaseProvider {
	if bufSize <= 0 {
		bufSize = providerbase.DefaultEventBufSize
	}
	ctrl := pollwatch.NewController(bufSize)
	return &BaseProvider{
		ProviderLifecycle: providerbase.NewProviderLifecycle(ctrl),
	}
}

// SetName sets the provider name.
func (b *BaseProvider) SetName(name string) { b.providerName = name }

// SetProviderPriority sets the provider priority.
func (b *BaseProvider) SetProviderPriority(p int) { b.priority = p }

// Name returns the provider name.
func (b *BaseProvider) Name() string { return b.providerName }

// Priority returns the provider priority.
func (b *BaseProvider) Priority() int { return b.priority }

// String returns a string representation.
func (b *BaseProvider) String() string { return b.providerName }

// IsClosed reports whether the provider has been closed.
func (b *BaseProvider) IsClosed() bool {
	return b.ProviderLifecycle.IsClosed()
}

// EnsureOpen returns an error if the provider has been closed.
func (b *BaseProvider) EnsureOpen() error {
	return b.ProviderLifecycle.EnsureOpen()
}

// CloseProvider performs an orderly shutdown.
func (b *BaseProvider) CloseProvider(ctx context.Context) error {
	return b.ProviderLifecycle.CloseProvider(ctx)
}

// EmitDiff emits diff events through the poll watch controller.
func (b *BaseProvider) EmitDiff(
	ctx context.Context,
	old, newData map[string]value.Value,
	opts ...event.Option,
) error {
	return b.Controller.EmitDiff(ctx, old, newData, opts...)
}

// Close shuts down the provider.
func (b *BaseProvider) Close(ctx context.Context) error {
	return b.CloseProvider(ctx)
}

// Load returns an empty map (no-op default).
func (b *BaseProvider) Load(_ context.Context) (map[string]value.Value, error) {
	return make(map[string]value.Value), nil
}

// Watch returns nil (no-op default).
func (b *BaseProvider) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Health returns nil (no-op default).
func (b *BaseProvider) Health(_ context.Context) error {
	return nil
}

// Factory creates a [Provider] from a configuration map.
type Factory func(cfg map[string]any) (Provider, error)
