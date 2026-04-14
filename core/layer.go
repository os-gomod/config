package core

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/os-gomod/config/core/circuit"
	"github.com/os-gomod/config/core/value"
)

// Loadable is a minimal interface that Layer uses to load data.
// It is satisfied by any loader.Loader implementation (added in Phase 2).
type Loadable interface {
	Load(ctx context.Context) (map[string]value.Value, error)
	Close(ctx context.Context) error
}

// HealthStatus describes the current health of a Layer.
type HealthStatus struct {
	Healthy          bool
	ConsecutiveFails int64
	LastFailureTime  time.Time
	LastError        error
}

// Layer represents a single config source (file, env, remote, etc.) with its
// own circuit breaker, priority, and health tracking.
//
// Lock order: Layer has no mutex; it relies on atomic operations.
// The Engine.mu lock is always held when reading Layer fields that are
// set only during construction (name, priority, source, timeout).
type Layer struct {
	name     string
	priority int
	source   Loadable
	enabled  atomic.Bool
	timeout  time.Duration
	// Atomics and mutexes first to minimize false sharing.
	lastGoodData     atomic.Pointer[map[string]value.Value]
	cb               *circuit.Breaker
	healthUnhealthy  atomic.Bool
	healthFailures   atomic.Int64
	healthLastFail   atomic.Int64
	healthLastErrPtr atomic.Pointer[error] // stores a pointer to the most recent load error, or nil
}

// NewLayer creates a Layer with the given name and options. The default
// priority is 10, the default timeout is 2 seconds, and the circuit breaker
// uses DefaultConfig.
func NewLayer(name string, opts ...LayerOption) *Layer {
	l := &Layer{
		name:     name,
		priority: 10,
		timeout:  2 * time.Second,
		cb:       circuit.New(circuit.DefaultConfig()),
	}
	l.enabled.Store(true)
	for _, opt := range opts {
		opt(l)
	}
	empty := make(map[string]value.Value)
	l.lastGoodData.Store(&empty)
	return l
}

// Name returns the layer's name.
func (l *Layer) Name() string { return l.name }

// Priority returns the layer's merge priority.
func (l *Layer) Priority() int { return l.priority }

// IsEnabled reports whether the layer is enabled for loading.
func (l *Layer) IsEnabled() bool { return l.enabled.Load() }

// Enable marks the layer as enabled.
func (l *Layer) Enable() { l.enabled.Store(true) }

// Disable marks the layer as disabled; it will be skipped during Reload.
func (l *Layer) Disable() { l.enabled.Store(false) }

// CircuitBreaker returns the layer's circuit breaker for external inspection.
func (l *Layer) CircuitBreaker() *circuit.Breaker { return l.cb }

// IsHealthy reports whether the layer's circuit breaker is closed.
func (l *Layer) IsHealthy() bool { return !l.cb.IsOpen() }

// HealthStatus returns a snapshot of the layer's health metrics.
func (l *Layer) HealthStatus() HealthStatus {
	var lastErr error
	if p := l.healthLastErrPtr.Load(); p != nil {
		lastErr = *p
	}
	var lastFail time.Time
	if ns := l.healthLastFail.Load(); ns != 0 {
		lastFail = time.Unix(0, ns)
	}
	return HealthStatus{
		Healthy:          !l.healthUnhealthy.Load(),
		ConsecutiveFails: l.healthFailures.Load(),
		LastFailureTime:  lastFail,
		LastError:        lastErr,
	}
}

// LastData returns a safe copy of the most recently loaded data.
func (l *Layer) LastData() map[string]value.Value {
	if m := l.lastGoodData.Load(); m != nil {
		return value.Copy(*m)
	}
	return make(map[string]value.Value)
}

// setLastGood stores a copy of the successfully loaded data.
func (l *Layer) setLastGood(data map[string]value.Value) {
	if data == nil {
		data = make(map[string]value.Value)
	}
	copied := value.Copy(data)
	l.lastGoodData.Store(&copied)
}

// Close delegates to the underlying source's Close method, if one is set.
func (l *Layer) Close(ctx context.Context) error {
	if l.source != nil {
		return l.source.Close(ctx)
	}
	return nil
}

// LayerError wraps an error from a specific layer.
type LayerError struct {
	Layer string
	Err   error
}

// Error formats the LayerError as "layer: error".
func (e LayerError) Error() string { return e.Layer + ": " + e.Err.Error() }

// Unwrap returns the underlying error for errors.Is/errors.As traversal.
func (e LayerError) Unwrap() error { return e.Err }

// Load attempts to load data from the layer's source. If the circuit breaker
// is open, it returns the last good data with an error. On failure it records
// the error and returns fallback data.
func (l *Layer) Load(ctx context.Context) (map[string]value.Value, error) {
	if l.source == nil {
		return make(map[string]value.Value), nil
	}
	if l.cb.IsOpen() {
		return l.handleOpenCircuit()
	}
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	data, err := l.source.Load(ctx)
	if err != nil {
		return l.handleFailure(err)
	}
	return l.handleSuccess(data), nil
}

// handleOpenCircuit returns fallback data when the circuit breaker is open.
func (l *Layer) handleOpenCircuit() (map[string]value.Value, error) {
	fallback := l.LastData()
	if len(fallback) > 0 {
		return fallback, fmt.Errorf("layer %s: circuit open (using last good)", l.name)
	}
	return fallback, fmt.Errorf("layer %s: circuit open, no fallback", l.name)
}

// handleFailure records the failure and returns fallback data.
func (l *Layer) handleFailure(err error) (map[string]value.Value, error) {
	l.recordFailure(err)
	return l.LastData(), err
}

// handleSuccess records the success, persists the data, and returns a copy.
func (l *Layer) handleSuccess(data map[string]value.Value) map[string]value.Value {
	l.recordSuccess()
	l.setLastGood(data)
	return value.Copy(data)
}

// recordFailure updates health metrics and trips the circuit breaker.
func (l *Layer) recordFailure(err error) {
	l.healthUnhealthy.Store(true)
	l.healthFailures.Add(1)
	l.healthLastFail.Store(time.Now().UTC().UnixNano())
	l.healthLastErrPtr.Store(&err)
	l.cb.RecordFailure()
}

// recordSuccess resets health metrics and records success in the circuit breaker.
func (l *Layer) recordSuccess() {
	l.healthUnhealthy.Store(false)
	l.healthFailures.Store(0)
	l.healthLastErrPtr.Store(nil)
	l.cb.RecordSuccess()
}
