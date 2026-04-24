// Package providerbase provides generic building blocks for remote configuration
// providers. It eliminates the boilerplate that was previously duplicated across
// every provider implementation (NATS, Consul, etcd).
//
// The core abstraction is RemoteProvider[T], a generic type parameterized by the
// concrete client type. Each provider supplies only its unique logic via callback
// functions (InitFn, FetchFn, HealthFn, etc.), while RemoteProvider handles all
// shared concerns: lifecycle management, lazy initialization, mutex coordination,
// polling, native watch loops, diff emission, and orderly shutdown.
//
// Additionally, RemoteProvider supports exponential backoff with jitter for
// retrying failed client initialization and watch reconnection, making all
// remote providers resilient to transient network failures.
package providerbase

import (
	"context"
	"fmt"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/backoff"
	"github.com/os-gomod/config/internal/pollwatch"
)

// RetryConfig controls retry behavior for client initialization and
// watch reconnection in RemoteProvider.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts.
	// 0 means no retries (fail fast). Default when used: 5.
	MaxAttempts int
	// InitialInterval is the delay before the first retry. Defaults to 100ms.
	InitialInterval time.Duration
	// MaxInterval is the maximum delay between retries. Defaults to 10s.
	MaxInterval time.Duration
	// Multiplier grows the interval exponentially. Defaults to 2.0.
	Multiplier float64
	// JitterFactor adds randomness to avoid thundering herd. Defaults to 0.2.
	JitterFactor float64
}

// toBackoffConfig converts a RetryConfig to a backoff.Config,
// applying defaults for zero-valued fields.
func (rc RetryConfig) toBackoffConfig() backoff.Config {
	return backoff.Config{
		MaxRetries:      rc.MaxAttempts,
		InitialInterval: rc.InitialInterval,
		MaxInterval:     rc.MaxInterval,
		Multiplier:      rc.Multiplier,
		JitterFactor:    rc.JitterFactor,
	}
}

// NativeWatchFn sets up a push-based watch mechanism for a remote provider.
//
// The function receives the context and an initialized client, and returns
// a trigger channel. Each send on the trigger channel signals that the
// underlying data may have changed. When the trigger channel is closed,
// the watch has terminated.
//
// RemoteProvider handles the common watch loop: receive trigger -> fetch data ->
// diff against last known state -> emit events. Providers like NATS and etcd
// implement this to avoid duplicating the watch boilerplate.
//
// Providers without native push support (e.g., Consul with long-poll) should
// leave this nil and use PollInterval instead.
type NativeWatchFn[T any] func(ctx context.Context, client T) (<-chan struct{}, error)

// Config holds the configuration for creating a RemoteProvider.
// All callback functions are optional except FetchFn which is required.
type Config[T any] struct {
	// Name is the provider name used in logging and identification (e.g., "nats", "etcd").
	Name string

	// Priority determines merge order during config reload. Higher values win.
	Priority int

	// PollInterval enables polling-based watching when > 0.
	// When set, PollInterval takes precedence over NativeWatchFn.
	PollInterval time.Duration

	// BufferSize controls the event channel buffer size. Defaults to DefaultEventBufSize.
	BufferSize int

	// StringFormat overrides the default String() representation.
	// If empty, defaults to Name.
	StringFormat string

	// InitFn lazily initializes the client connection.
	// Called once under Mu lock on first access.
	// If nil, the client must be set externally before use.
	InitFn func() (T, error)

	// FetchFn loads all configuration data from the remote source.
	// Receives the context and an initialized client.
	// Must return the full config map (not a diff).
	// Required — panics at construction time if nil.
	FetchFn func(ctx context.Context, client T) (map[string]value.Value, error)

	// HealthFn checks the health of the remote connection.
	// Called after EnsureOpen and ensureClient succeed.
	// If nil, health defaults to "initialized = healthy".
	HealthFn func(ctx context.Context, client T) error

	// NativeWatchFn sets up a push-based watch for data changes.
	// Returns a trigger channel. See NativeWatchFn for details.
	// If nil and PollInterval <= 0, Watch returns nil (watching not supported).
	NativeWatchFn NativeWatchFn[T]

	// CloseFn cleans up the client connection resources.
	// Called once during Close. If nil, no client cleanup is performed.
	CloseFn func(client T) error

	// Retry configures retry behavior for InitFn failures and
	// watch reconnection. If zero-valued, no retries are performed.
	Retry RetryConfig
}

// RemoteProvider is a generic remote configuration provider that eliminates
// boilerplate across different backend implementations.
//
// Type parameter T is the concrete client type (e.g., natsKV, consulClient, etcdClient).
// RemoteProvider manages:
//   - Lazy client initialization (via InitFn, called once under Mu)
//   - Exponential backoff retry for initialization failures
//   - Thread-safe client access (via ClientLifecycle.Mu)
//   - Both polling and push-based watching (via PollableSource / NativeWatchFn)
//   - Automatic watch reconnection with backoff on disconnect
//   - Health checks (via HealthFn)
//   - Orderly shutdown (via CloseFn + ProviderLifecycle)
//
// RemoteProvider[T] satisfies the provider.Provider interface for any T.
type RemoteProvider[T any] struct {
	*ProviderLifecycle
	poll *PollableSource

	client      T
	clientReady bool

	name         string
	priority     int
	pollInterval time.Duration
	str          string

	initFn        func() (T, error)
	fetchFn       func(ctx context.Context, client T) (map[string]value.Value, error)
	healthFn      func(ctx context.Context, client T) error
	nativeWatchFn NativeWatchFn[T]
	closeFn       func(client T) error
	retry         RetryConfig
}

// New creates a new RemoteProvider from the given configuration.
// The FetchFn field must be non-nil; all other callbacks are optional.
func New[T any](cfg Config[T]) *RemoteProvider[T] {
	if cfg.FetchFn == nil {
		panic("providerbase: FetchFn is required")
	}

	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = DefaultEventBufSize
	}
	ctrl := pollwatch.NewController(bufSize)

	str := cfg.Name
	if cfg.StringFormat != "" {
		str = cfg.StringFormat
	}

	return &RemoteProvider[T]{
		ProviderLifecycle: NewProviderLifecycle(ctrl),
		poll:              NewPollableSource(ctrl),
		name:              cfg.Name,
		priority:          cfg.Priority,
		pollInterval:      cfg.PollInterval,
		str:               str,
		initFn:            cfg.InitFn,
		fetchFn:           cfg.FetchFn,
		healthFn:          cfg.HealthFn,
		nativeWatchFn:     cfg.NativeWatchFn,
		closeFn:           cfg.CloseFn,
		retry:             cfg.Retry,
	}
}

// ensureClient performs lazy initialization of the client under Mu lock.
// This is the single point of initialization logic — each provider's InitFn
// is called at most once. Subsequent calls are no-ops.
//
// If RetryConfig is set and InitFn fails, ensureClient will retry with
// exponential backoff and jitter until the context is cancelled or
// MaxAttempts is exhausted.
//
// PRECONDITION: caller must hold r.Mu (or call via a method that does).
func (r *RemoteProvider[T]) ensureClient() error {
	if r.clientReady {
		return nil
	}
	if r.initFn == nil {
		return fmt.Errorf("providerbase: %s: no init function and client not set", r.name)
	}

	c, err := r.initFn()
	if err == nil {
		r.client = c
		r.clientReady = true
		return nil
	}

	// If retry is not configured, return immediately
	if r.retry.MaxAttempts == 0 {
		return fmt.Errorf("providerbase: %s: init: %w", r.name, err)
	}

	// Retry with exponential backoff
	bo := backoff.New(r.retry.toBackoffConfig())
	lastErr := err
	for {
		delay, ok := bo.Next()
		if !ok {
			return fmt.Errorf("providerbase: %s: init failed after %d attempts: %w",
				r.name, bo.Attempt(), lastErr)
		}
		select {
		case <-time.After(delay):
			retryClient, retryErr := r.initFn()
			if retryErr == nil {
				r.client = retryClient
				r.clientReady = true
				return nil
			}
			lastErr = retryErr
		case <-r.Done():
			return fmt.Errorf("providerbase: %s: init cancelled during retry: %w", r.name, lastErr)
		}
	}
}

// Load fetches all configuration data from the remote source.
// It ensures the client is initialized, then delegates to FetchFn.
//
// This single implementation replaces the duplicated Load methods
// previously found in nats.go, consul.go, and etcd.go.
func (r *RemoteProvider[T]) Load(ctx context.Context) (map[string]value.Value, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	r.Mu.Lock()
	if err := r.ensureClient(); err != nil {
		r.Mu.Unlock()
		return nil, err
	}
	client := r.client
	r.Mu.Unlock()
	return r.fetchFn(ctx, client)
}

// Watch starts monitoring the remote source for configuration changes.
//
// Watch strategy (in priority order):
//  1. If PollInterval > 0: uses PollableSource for interval-based polling
//  2. Else if NativeWatchFn != nil: uses push-based native watching
//  3. Else: returns nil (watching not supported)
//
// This single implementation replaces the duplicated Watch + watchMode +
// blockingWatch methods previously found in each provider.
func (r *RemoteProvider[T]) Watch(ctx context.Context) (<-chan event.Event, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	if r.pollInterval > 0 {
		return r.pollWatch(ctx)
	}
	if r.nativeWatchFn != nil {
		return r.nativeWatch(ctx)
	}
	return nil, nil
}

// pollWatch uses the PollableSource for interval-based change detection.
// The WatchMu is held to prevent concurrent watchers.
func (r *RemoteProvider[T]) pollWatch(ctx context.Context) (<-chan event.Event, error) {
	r.WatchMu.Lock()
	defer r.WatchMu.Unlock()
	return r.poll.Watch(ctx, r.pollInterval, r.Load)
}

// nativeWatch sets up a push-based watch via NativeWatchFn and runs the
// standard watch loop: receive trigger -> fetch -> diff -> emit events.
//
// When the trigger channel closes unexpectedly (e.g., network disconnect),
// the goroutine attempts to reconnect using exponential backoff if Retry
// is configured. This prevents silent data staleness on transient failures.
//
// This is the single watch loop that replaces the nearly-identical watchMode
// methods previously duplicated in nats.go and etcd.go.
func (r *RemoteProvider[T]) nativeWatch(ctx context.Context) (<-chan event.Event, error) {
	r.WatchMu.Lock()
	defer r.WatchMu.Unlock()

	r.Mu.Lock()
	if err := r.ensureClient(); err != nil {
		r.Mu.Unlock()
		return nil, err
	}
	client := r.client
	r.Mu.Unlock()

	eventCh := make(chan event.Event, DefaultEventBufSize)
	go func() {
		var lastData map[string]value.Value

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.Done():
				return
			default:
			}

			triggerCh, err := r.nativeWatchFn(ctx, client)
			if err != nil {
				if !r.retryWithReinit(ctx, &client) {
					return
				}
				continue
			}

			// Watch loop: consume trigger events until channel closes
			lastData = r.consumeTriggerLoop(ctx, eventCh, triggerCh, client, lastData)
		}
	}()
	return eventCh, nil
}

// retryWithReinit attempts to reinitialize the client with exponential backoff.
// Returns true if reconnection was successful (caller should re-establish watch).
// Returns false if retries were exhausted or context was cancelled.
func (r *RemoteProvider[T]) retryWithReinit(ctx context.Context, client *T) bool {
	if r.retry.MaxAttempts <= 0 {
		return false
	}
	bo := backoff.New(r.retry.toBackoffConfig())
	reconnected := false
retryLoop:
	for {
		delay, ok := bo.Next()
		if !ok {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-r.Done():
			return false
		case <-time.After(delay):
			r.Mu.Lock()
			r.clientReady = false
			reinitErr := r.ensureClient()
			if reinitErr == nil {
				*client = r.client
				r.Mu.Unlock()
				reconnected = true
				break retryLoop
			}
			r.Mu.Unlock()
		}
	}
	return reconnected
}

// consumeTriggerLoop reads from the trigger channel, fetches new data on each
// trigger, diffs against the last known state, and emits change events.
// Returns the latest data and whether the trigger channel closed normally
// (true = closed, caller should reconnect; false = context/done cancelled).
func (r *RemoteProvider[T]) consumeTriggerLoop(
	ctx context.Context,
	eventCh chan event.Event,
	triggerCh <-chan struct{},
	client T,
	lastData map[string]value.Value,
) map[string]value.Value {
	for {
		select {
		case <-ctx.Done():
			return lastData
		case <-r.Done():
			return lastData
		case _, ok := <-triggerCh:
			if !ok {
				return lastData // trigger closed, caller should reconnect
			}
			newData, loadErr := r.fetchFn(ctx, client)
			if loadErr != nil {
				continue
			}
			if lastData != nil {
				evts := event.NewDiffEvents(lastData, newData)
				for i := range evts {
					select {
					case eventCh <- evts[i]:
					case <-ctx.Done():
						return lastData
					case <-r.Done():
						return lastData
					}
				}
			}
			lastData = newData
		}
	}
}

// Health checks the health of the remote connection.
// It ensures the client is initialized, then delegates to HealthFn.
// If HealthFn is nil, returns nil (initialized = healthy).
//
// This single implementation replaces the duplicated Health methods
// previously found in each provider.
func (r *RemoteProvider[T]) Health(ctx context.Context) error {
	if err := r.EnsureOpen(); err != nil {
		return err
	}
	r.Mu.Lock()
	if err := r.ensureClient(); err != nil {
		r.Mu.Unlock()
		return err
	}
	client := r.client
	r.Mu.Unlock()
	if r.healthFn != nil {
		return r.healthFn(ctx, client)
	}
	return nil
}

// Name returns the provider name.
func (r *RemoteProvider[T]) Name() string { return r.name }

// Priority returns the provider's merge priority.
func (r *RemoteProvider[T]) Priority() int { return r.priority }

// String returns a human-readable representation of the provider.
func (r *RemoteProvider[T]) String() string { return r.str }

// Close performs an orderly shutdown:
//  1. Calls CloseFn to clean up the client connection
//  2. Calls CloseProvider to stop polling and signal Done
//
// This single implementation replaces the duplicated Close methods
// previously found in each provider.
func (r *RemoteProvider[T]) Close(ctx context.Context) error {
	r.Mu.Lock()
	if r.closeFn != nil && r.clientReady {
		_ = r.closeFn(r.client)
	}
	r.clientReady = false
	r.Mu.Unlock()
	return r.CloseProvider(ctx)
}
