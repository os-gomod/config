package observability

import (
	"context"
	"sync/atomic"
	"time"
)

// AtomicMetrics is an in-memory Recorder implementation that tracks configuration
// operation counts, durations, and errors using atomic operations. It is safe for
// concurrent use and provides a Snapshot method for exporting current metrics.
//
// This is useful for testing, dashboards, and debugging without requiring an
// external metrics backend.
type AtomicMetrics struct {
	Loads              atomic.Int64
	Reloads            atomic.Int64
	Sets               atomic.Int64
	BatchSets          atomic.Int64
	Deletes            atomic.Int64
	Binds              atomic.Int64
	HookCalls          atomic.Int64
	TotalDuration      atomic.Int64
	Errors             atomic.Int64
	LoadErrors         atomic.Int64
	ReloadErrors       atomic.Int64
	SetErrors          atomic.Int64
	BatchSetErrors     atomic.Int64
	DeleteErrors       atomic.Int64
	HookErrors         atomic.Int64
	LayerLoads         atomic.Int64
	LayerLoadErrors    atomic.Int64
	EventsPublished    atomic.Int64
	EventsDispatched   atomic.Int64
	LastReloadDuration atomic.Int64
	LastStateVersion   atomic.Uint64
	KeyCount           atomic.Int64
	ValidationCalls    atomic.Int64
	ValidationErrors   atomic.Int64
	WatchEvents        atomic.Int64
	SecretsRedacted    atomic.Int64
	ConfigChangeEvents atomic.Int64
}

func (m *AtomicMetrics) RecordLoad(_ context.Context, _ string, d time.Duration, err error) {
	m.Loads.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.LoadErrors.Add(1)
	}
}

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

func (m *AtomicMetrics) RecordSet(_ context.Context, _ string, d time.Duration, err error) {
	m.Sets.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.SetErrors.Add(1)
	}
}

func (m *AtomicMetrics) RecordBatchSet(_ context.Context, d time.Duration, err error) {
	m.BatchSets.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.BatchSetErrors.Add(1)
	}
}

func (m *AtomicMetrics) RecordDelete(_ context.Context, _ string, d time.Duration, err error) {
	m.Deletes.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.DeleteErrors.Add(1)
	}
}

func (m *AtomicMetrics) RecordBind(_ context.Context, d time.Duration, _ error) {
	m.Binds.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
}

func (m *AtomicMetrics) RecordHook(_ context.Context, _ string, d time.Duration, err error) {
	m.HookCalls.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.HookErrors.Add(1)
	}
}

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

func (m *AtomicMetrics) RecordValidation(_ context.Context, d time.Duration, err error) {
	m.ValidationCalls.Add(1)
	m.TotalDuration.Add(d.Nanoseconds())
	if err != nil {
		m.Errors.Add(1)
		m.ValidationErrors.Add(1)
	}
}

func (m *AtomicMetrics) RecordWatchEvent(_ context.Context, _ string) {
	m.WatchEvents.Add(1)
}

func (m *AtomicMetrics) RecordSecretRedacted(_ context.Context, _ string) {
	m.SecretsRedacted.Add(1)
}

func (m *AtomicMetrics) RecordConfigChangeEvent(_ context.Context, _, _ string) {
	m.ConfigChangeEvents.Add(1)
}

// Snapshot returns all counters as a map, suitable for export to
// monitoring systems.
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
		"key_count":            m.KeyCount.Load(),
		"validation_calls":     m.ValidationCalls.Load(),
		"validation_errors":    m.ValidationErrors.Load(),
		"watch_events":         m.WatchEvents.Load(),
		"secrets_redacted":     m.SecretsRedacted.Load(),
		"config_change_events": m.ConfigChangeEvents.Load(),
	}
}

// Reset resets all counters to zero.
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
	m.SecretsRedacted.Store(0)
	m.ConfigChangeEvents.Store(0)
}
