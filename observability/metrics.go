package observability

import (
	"context"
	"sync/atomic"
	"time"
)

// AtomicMetrics is a Recorder that increments atomic counters for every
// operation. It is useful for tests and for lightweight in-process metrics.
// All fields are accessed atomically and are safe for concurrent use.
type AtomicMetrics struct {
	// Loads counts the number of Load calls.
	Loads atomic.Int64
	// Reloads counts the number of Reload calls.
	Reloads atomic.Int64
	// Sets counts the number of Set calls.
	Sets atomic.Int64
	// BatchSets counts the number of BatchSet calls.
	BatchSets atomic.Int64
	// Deletes counts the number of Delete calls.
	Deletes atomic.Int64
	// Binds counts the number of Bind calls.
	Binds atomic.Int64
	// HookCalls counts the number of hook executions.
	HookCalls atomic.Int64
	// TotalDuration accumulates nanoseconds across all operations.
	TotalDuration atomic.Int64
	// Errors counts total errors across all operations.
	Errors atomic.Int64
	// LoadErrors counts Load-specific errors.
	LoadErrors atomic.Int64
	// ReloadErrors counts Reload-specific errors.
	ReloadErrors atomic.Int64
	// SetErrors counts Set-specific errors.
	SetErrors atomic.Int64
	// BatchSetErrors counts BatchSet-specific errors.
	BatchSetErrors atomic.Int64
	// DeleteErrors counts Delete-specific errors.
	DeleteErrors atomic.Int64
	// HookErrors counts hook execution errors.
	HookErrors atomic.Int64
	// LayerLoads counts layer-level Load calls.
	LayerLoads atomic.Int64
	// LayerLoadErrors counts layer-level Load errors.
	LayerLoadErrors atomic.Int64
	// EventsPublished counts events emitted by the bus.
	EventsPublished atomic.Int64
	// EventsDispatched counts events delivered to observers.
	EventsDispatched atomic.Int64
	// LastReloadDuration stores the nanosecond duration of the most recent reload.
	LastReloadDuration atomic.Int64
	// LastStateVersion stores the state version after the most recent reload.
	LastStateVersion atomic.Uint64
	// KeyCount stores the current number of keys in the state.
	KeyCount atomic.Int64
	// ValidationCalls counts validation operations.
	ValidationCalls atomic.Int64
	// ValidationErrors counts validation failures.
	ValidationErrors atomic.Int64
	// WatchEvents counts watch events received from sources.
	WatchEvents atomic.Int64
}

// RecordLoad implements Recorder.
func (m *AtomicMetrics) RecordLoad(_ context.Context, _ string, d time.Duration, err error) {
	m.Loads.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.LoadErrors.Add(1)
	}
}

// RecordReload implements Recorder.
func (m *AtomicMetrics) RecordReload(_ context.Context, d time.Duration, keyCount int, err error) {
	m.Reloads.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	m.LastReloadDuration.Store(d.Nanoseconds())
	if keyCount >= 0 {
		m.KeyCount.Store(int64(keyCount))
	}
	if err != nil {
		m.Errors.Add(1)
		m.ReloadErrors.Add(1)
	}
}

// RecordSet implements Recorder.
func (m *AtomicMetrics) RecordSet(_ context.Context, _ string, d time.Duration, err error) {
	m.Sets.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.SetErrors.Add(1)
	}
}

// RecordBatchSet implements Recorder.
func (m *AtomicMetrics) RecordBatchSet(_ context.Context, d time.Duration, err error) {
	m.BatchSets.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.BatchSetErrors.Add(1)
	}
}

// RecordDelete implements Recorder.
func (m *AtomicMetrics) RecordDelete(_ context.Context, _ string, d time.Duration, err error) {
	m.Deletes.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.DeleteErrors.Add(1)
	}
}

// RecordBind implements Recorder.
func (m *AtomicMetrics) RecordBind(_ context.Context, d time.Duration, _ error) {
	m.Binds.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
}

// RecordHook implements Recorder.
func (m *AtomicMetrics) RecordHook(_ context.Context, _ string, d time.Duration, err error) {
	m.HookCalls.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.HookErrors.Add(1)
	}
}

// RecordLayerLoad implements Recorder.
func (m *AtomicMetrics) RecordLayerLoad(
	_ context.Context,
	_ string,
	d time.Duration,
	_ int,
	err error,
) {
	m.LayerLoads.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.LayerLoadErrors.Add(1)
	}
}

// RecordValidation implements Recorder.
func (m *AtomicMetrics) RecordValidation(_ context.Context, d time.Duration, err error) {
	m.ValidationCalls.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.ValidationErrors.Add(1)
	}
}

// RecordWatchEvent implements Recorder.
func (m *AtomicMetrics) RecordWatchEvent(_ context.Context, _ string) {
	m.WatchEvents.Add(1)
}

// Snapshot returns a map of all metric names to their current values.
func (m *AtomicMetrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"loads":             m.Loads.Load(),
		"reloads":           m.Reloads.Load(),
		"sets":              m.Sets.Load(),
		"batch_sets":        m.BatchSets.Load(),
		"deletes":           m.Deletes.Load(),
		"binds":             m.Binds.Load(),
		"hook_calls":        m.HookCalls.Load(),
		"errors":            m.Errors.Load(),
		"load_errors":       m.LoadErrors.Load(),
		"reload_errors":     m.ReloadErrors.Load(),
		"set_errors":        m.SetErrors.Load(),
		"batch_set_errors":  m.BatchSetErrors.Load(),
		"delete_errors":     m.DeleteErrors.Load(),
		"hook_errors":       m.HookErrors.Load(),
		"layer_loads":       m.LayerLoads.Load(),
		"layer_load_errors": m.LayerLoadErrors.Load(),
		"events_published":  m.EventsPublished.Load(),
		"events_dispatched": m.EventsDispatched.Load(),
		"total_duration_ns": m.TotalDuration.Load(),
		"last_reload_ns":    m.LastReloadDuration.Load(),
		"last_state_version": func(v uint64) int64 {
			const maxInt64 = int64(^uint64(0) >> 1)
			if v > uint64(maxInt64) {
				return maxInt64
			}
			return int64(v)
		}(m.LastStateVersion.Load()),
		"key_count":         m.KeyCount.Load(),
		"validation_calls":  m.ValidationCalls.Load(),
		"validation_errors": m.ValidationErrors.Load(),
		"watch_events":      m.WatchEvents.Load(),
	}
}

// Reset zeroes all metric counters.
func (m *AtomicMetrics) Reset() {
	m.Loads.Store(0)
	m.Reloads.Store(0)
	m.Sets.Store(0)
	m.BatchSets.Store(0)
	m.Deletes.Store(0)
	m.Binds.Store(0)
	m.HookCalls.Store(0)
	m.Errors.Store(0)
	m.LoadErrors.Store(0)
	m.ReloadErrors.Store(0)
	m.SetErrors.Store(0)
	m.BatchSetErrors.Store(0)
	m.DeleteErrors.Store(0)
	m.HookErrors.Store(0)
	m.LayerLoads.Store(0)
	m.LayerLoadErrors.Store(0)
	m.EventsPublished.Store(0)
	m.EventsDispatched.Store(0)
	m.TotalDuration.Store(0)
	m.LastReloadDuration.Store(0)
	m.LastStateVersion.Store(0)
	m.KeyCount.Store(0)
	m.ValidationCalls.Store(0)
	m.ValidationErrors.Store(0)
	m.WatchEvents.Store(0)
}
