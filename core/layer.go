package core

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/os-gomod/config/core/circuit"
	"github.com/os-gomod/config/core/value"
)

// Loadable is the interface implemented by configuration sources that can be
// loaded and closed. Both [loader.Loader] and [provider.Provider] satisfy
// this interface.
type Loadable interface {
	// Load reads configuration from the source and returns a map of typed
	// values, or an error if loading fails.
	Load(ctx context.Context) (map[string]value.Value, error)
	// Close releases any resources held by the source.
	Close(ctx context.Context) error
}

// HealthStatus describes the current health of a [Layer], including whether
// it is healthy, how many consecutive failures have occurred, when the last
// failure happened, and what the last error was.
type HealthStatus struct {
	// Healthy is true when the layer has not recently failed.
	Healthy bool
	// ConsecutiveFails is the count of consecutive load failures.
	ConsecutiveFails int64
	// LastFailureTime is the timestamp of the most recent failure.
	LastFailureTime time.Time
	// LastError is the error from the most recent load failure.
	LastError error
}

// Layer represents a single configuration source within the engine's layer
// stack. Each layer has a name, priority, optional data source ([Loadable]),
// timeout, circuit breaker, and health tracking.
//
// Layers are loaded concurrently during [Engine.Reload], and the results are
// merged by priority (higher wins). When a layer's circuit breaker is open,
// the last successfully loaded data is returned as a fallback.
type Layer struct {
	name             string
	priority         int
	source           Loadable
	enabled          atomic.Bool
	timeout          time.Duration
	lastGoodData     atomic.Pointer[map[string]value.Value]
	cb               *circuit.Breaker
	healthUnhealthy  atomic.Bool
	healthFailures   atomic.Int64
	healthLastFail   atomic.Int64
	healthLastErrPtr atomic.Pointer[error]
}

// NewLayer creates a new [Layer] with the given name and options. Defaults
// are priority 10, timeout 2 seconds, enabled true, and a default circuit
// breaker configuration.
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

// Name returns the human-readable name of this layer.
func (l *Layer) Name() string { return l.name }

// Priority returns the merge priority of this layer. Higher values win during
// conflict resolution.
func (l *Layer) Priority() int { return l.priority }

// IsEnabled reports whether this layer is enabled and will be included in
// reloads.
func (l *Layer) IsEnabled() bool { return l.enabled.Load() }

// Enable marks the layer as enabled so that it participates in future reloads.
func (l *Layer) Enable() { l.enabled.Store(true) }

// Disable marks the layer as disabled so that it is skipped during reloads.
func (l *Layer) Disable() { l.enabled.Store(false) }

// CircuitBreaker returns the circuit breaker protecting this layer. Callers
// can inspect or reset the breaker as needed.
func (l *Layer) CircuitBreaker() *circuit.Breaker { return l.cb }

// IsHealthy reports whether the layer's circuit breaker is closed (i.e., the
// layer is operational).
func (l *Layer) IsHealthy() bool { return !l.cb.IsOpen() }

// HealthStatus returns a snapshot of the layer's current health, including
// consecutive failure count, last failure time, and last error.
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

// LastData returns a defensive copy of the last successfully loaded data for
// this layer. If no successful load has occurred, an empty map is returned.
func (l *Layer) LastData() map[string]value.Value {
	if m := l.lastGoodData.Load(); m != nil {
		return value.Copy(*m)
	}
	return make(map[string]value.Value)
}

// setLastGood stores a deep copy of the given data as the last known good
// data for fallback when the circuit breaker is open.
func (l *Layer) setLastGood(data map[string]value.Value) {
	if data == nil {
		data = make(map[string]value.Value)
	}
	copied := value.Copy(data)
	l.lastGoodData.Store(&copied)
}

// Close releases any resources held by the layer's data source.
func (l *Layer) Close(ctx context.Context) error {
	if l.source != nil {
		return l.source.Close(ctx)
	}
	return nil
}

// LayerError associates a layer name with an error that occurred during
// loading. It implements the [error] interface.
type LayerError struct {
	// Layer is the name of the layer that produced the error.
	Layer string
	// Err is the underlying error.
	Err error
}

// Error returns a formatted string combining the layer name and the
// underlying error message.
func (e LayerError) Error() string { return e.Layer + ": " + e.Err.Error() }

// Unwrap returns the underlying error, enabling errors.Is/errors.As chains.
func (e LayerError) Unwrap() error { return e.Err }

// Load reads configuration from the layer's data source, respecting the
// circuit breaker and timeout. On success, the data is cached as last-good
// data. On failure, health metrics are updated and the last-good data is
// returned as a fallback.
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

// handleOpenCircuit returns the last known good data when the circuit breaker
// is open. If no fallback data exists, an empty map is returned with a
// descriptive error.
func (l *Layer) handleOpenCircuit() (map[string]value.Value, error) {
	fallback := l.LastData()
	if len(fallback) > 0 {
		return fallback, fmt.Errorf("layer %s: circuit open (using last good)", l.name)
	}
	return fallback, fmt.Errorf("layer %s: circuit open, no fallback", l.name)
}

// handleFailure records the failure in health metrics and the circuit
// breaker, then returns the last known good data alongside the original
// error.
func (l *Layer) handleFailure(err error) (map[string]value.Value, error) {
	l.recordFailure(err)
	return l.LastData(), err
}

// handleSuccess records a successful load, updates the last-good cache, and
// returns a deep copy of the loaded data.
func (l *Layer) handleSuccess(data map[string]value.Value) map[string]value.Value {
	l.recordSuccess()
	l.setLastGood(data)
	return value.Copy(data)
}

// recordFailure updates health counters and marks the circuit breaker with a
// failure.
func (l *Layer) recordFailure(err error) {
	l.healthUnhealthy.Store(true)
	l.healthFailures.Add(1)
	l.healthLastFail.Store(time.Now().UTC().UnixNano())
	l.healthLastErrPtr.Store(&err)
	l.cb.RecordFailure()
}

// recordSuccess resets health counters and marks the circuit breaker with a
// success.
func (l *Layer) recordSuccess() {
	l.healthUnhealthy.Store(false)
	l.healthFailures.Store(0)
	l.healthLastErrPtr.Store(nil)
	l.cb.RecordSuccess()
}
