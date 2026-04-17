// Package provider defines the Provider interface for remote configuration sources
// and provides a BaseProvider struct that eliminates boilerplate shared across
// all provider implementations (consul, etcd, nats, etc.).
package provider

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
	"github.com/os-gomod/config/internal/pollwatch"
)

// Provider is the interface that remote configuration sources must implement.
// Each provider connects to an external system (consul, etcd, nats, etc.) and
// provides configuration data as a flat map of typed values.
//
// Providers are typically registered via the Registry and used as layers in the
// config engine. They support both polling-based and native push-based watching.
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

// BaseProvider provides shared boilerplate for all Provider implementations.
// It manages closable state, event channels, polling, and diff emission,
// eliminating ~200 lines of identical code that was previously duplicated
// across consul, etcd, and nats providers.
//
// Embed BaseProvider in your provider struct and call its methods from your
// Provider interface implementation. Override CloseClient for provider-specific
// cleanup (e.g., closing etcd client connections).
type BaseProvider struct {
	Closable *common.Closable
	WatchMu  sync.Mutex
	Mu       sync.Mutex
	*pollwatch.Controller

	PollInterval time.Duration
	providerName string
	priority     int
}

// NewBaseProvider creates a BaseProvider with the given event buffer size.
// If bufSize <= 0, a default of 64 is used.
func NewBaseProvider(bufSize int) *BaseProvider {
	if bufSize <= 0 {
		bufSize = 64
	}
	return &BaseProvider{
		Closable:   common.NewClosable(),
		Controller: pollwatch.NewController(bufSize),
	}
}

// SetName sets the provider name.
func (b *BaseProvider) SetName(name string) { b.providerName = name }

// SetProviderPriority sets the priority.
func (b *BaseProvider) SetProviderPriority(p int) { b.priority = p }

// Name returns the provider name.
func (b *BaseProvider) Name() string { return b.providerName }

// Priority returns the provider priority.
func (b *BaseProvider) Priority() int { return b.priority }

// String returns a human-readable description.
func (b *BaseProvider) String() string { return b.providerName }

// IsClosed reports whether the provider has been closed.
func (b *BaseProvider) IsClosed() bool {
	return b.Closable.IsClosed()
}

// EnsureOpen checks if the provider is closed and returns ErrClosed if so.
// Use this as a guard at the start of Load, Watch, and Health methods.
func (b *BaseProvider) EnsureOpen() error {
	if b.Closable.IsClosed() {
		return errors.ErrClosed
	}
	return nil
}

// CloseProvider handles the common close pattern: stop the watch goroutine,
// call CloseClient (if overridden), then close the closable.
// Provider implementations should embed BaseProvider and implement CloseClient
// for provider-specific cleanup, then call CloseProvider from their Close method.
func (b *BaseProvider) CloseProvider(ctx context.Context) error {
	b.WatchMu.Lock()
	defer b.WatchMu.Unlock()
	b.Controller.Close()
	b.Closable.Close(ctx)
	return nil
}

// PollWatch starts a polling-based watch goroutine using the shared Controller.
// The callback receives a context and should call Load + EmitDiff.
// This replaces the identical pollWatch() methods previously in each provider.
func (b *BaseProvider) PollWatch(
	ctx context.Context,
	callback func(ctx context.Context),
) (<-chan event.Event, error) {
	b.WatchMu.Lock()
	defer b.WatchMu.Unlock()
	return b.StartPolling(ctx, b.PollInterval, callback), nil
}

// EmitDiff publishes create/update/delete events for the difference between
// old and new data maps. This replaces the identical emitDiff() methods
// previously in each provider.
func (b *BaseProvider) EmitDiff(
	ctx context.Context,
	old, newData map[string]value.Value,
	opts ...event.Option,
) error {
	return b.Controller.EmitDiff(ctx, old, newData, opts...)
}

// PollLoadAndWatch starts polling and automatically tracks lastData for diff
// computation. The loadFn should return the current config data. On each tick,
// it calls loadFn, computes diffs against the previous result, and emits events.
// This is the recommended method for providers that only need polling-based watching.
//
// Example:
//
//	return p.PollLoadAndWatch(ctx, p.Load)
func (b *BaseProvider) PollLoadAndWatch(
	ctx context.Context,
	loadFn func(context.Context) (map[string]value.Value, error),
) (<-chan event.Event, error) {
	var lastData map[string]value.Value
	ch, _ := b.PollWatch(ctx, func(watchCtx context.Context) {
		data, err := loadFn(watchCtx)
		if err != nil {
			return
		}
		if lastData != nil {
			_ = b.EmitDiff(watchCtx, lastData, data)
		}
		lastData = data
	})
	return ch, nil
}

// CloseClient is a hook called during Close for provider-specific cleanup.
// Override this in your provider to close client connections, etc.
// The default implementation does nothing.
func (b *BaseProvider) CloseClient() {}

// Close implements Provider.Close.
func (b *BaseProvider) Close(ctx context.Context) error {
	return b.CloseProvider(ctx)
}

// Load implements Provider.Load (no-op, returns empty map).
func (b *BaseProvider) Load(_ context.Context) (map[string]value.Value, error) {
	return make(map[string]value.Value), nil
}

// Watch implements Provider.Watch (no-op, returns nil).
func (b *BaseProvider) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Health implements Provider.Health (no-op, returns nil).
func (b *BaseProvider) Health(_ context.Context) error {
	return nil
}

// Factory is a function that creates a Provider from a configuration map.
// Factories are registered with the Registry and used to instantiate
// providers by name.
type Factory func(cfg map[string]any) (Provider, error)
