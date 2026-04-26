package observability

import (
	"context"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// counter
// ---------------------------------------------------------------------------

// counter is a thread-safe monotonically increasing counter.
type counter struct {
	mu    sync.Mutex
	value uint64
}

func (c *counter) inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *counter) add(n uint64) {
	c.mu.Lock()
	c.value += n
	c.mu.Unlock()
}

func (c *counter) get() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// ---------------------------------------------------------------------------
// histogram
// ---------------------------------------------------------------------------

// histogram tracks count, sum, min, and max for observed values.
// This is a simple implementation suitable for in-process metrics.
type histogram struct {
	mu    sync.Mutex
	sum   float64
	min   float64
	max   float64
	count uint64
	init  bool
}

func (h *histogram) observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.count++
	if !h.init {
		h.min = value
		h.max = value
		h.init = true
	} else {
		if value < h.min {
			h.min = value
		}
		if value > h.max {
			h.max = value
		}
	}
}

func (h *histogram) snapshot() histogramSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return histogramSnapshot{
		Count: h.count,
		Sum:   h.sum,
		Min:   h.min,
		Max:   h.max,
		Avg:   h.avg(),
	}
}

func (h *histogram) avg() float64 {
	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

type histogramSnapshot struct {
	Count uint64
	Sum   float64
	Min   float64
	Max   float64
	Avg   float64
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

// Metrics provides an in-process metrics implementation of Recorder.
// It tracks counters and histograms for each operation type.
// This is instance-based — NO global metrics registry.
type Metrics struct {
	mu         sync.RWMutex
	counters   map[string]*counter
	histograms map[string]*histogram
}

// NewMetrics creates a new Metrics recorder.
func NewMetrics() *Metrics {
	return &Metrics{
		counters:   make(map[string]*counter),
		histograms: make(map[string]*histogram),
	}
}

// ---------------------------------------------------------------------------
// Recorder interface implementation
// ---------------------------------------------------------------------------

// RecordReload records a config reload operation.
func (m *Metrics) RecordReload(ctx context.Context, duration time.Duration, events int, err error) {
	m.record(ctx, "reload", duration, err)
	m.incCounterBy("reload_events_total", uint64(events))
}

// RecordSet records a config set operation.
func (m *Metrics) RecordSet(ctx context.Context, _ string, duration time.Duration, err error) {
	m.record(ctx, "set", duration, err)
}

// RecordDelete records a config delete operation.
func (m *Metrics) RecordDelete(ctx context.Context, _ string, duration time.Duration, err error) {
	m.record(ctx, "delete", duration, err)
}

// RecordBatchSet records a batch set operation.
func (m *Metrics) RecordBatchSet(ctx context.Context, duration time.Duration, err error) {
	m.record(ctx, "batch_set", duration, err)
}

// RecordBind records a bind operation.
func (m *Metrics) RecordBind(ctx context.Context, duration time.Duration, err error) {
	m.record(ctx, "bind", duration, err)
}

// RecordValidation records a validation operation.
func (m *Metrics) RecordValidation(ctx context.Context, duration time.Duration, err error) {
	m.record(ctx, "validation", duration, err)
}

// RecordHook records a hook execution.
func (m *Metrics) RecordHook(ctx context.Context, _ string, duration time.Duration, err error) {
	m.record(ctx, "hook", duration, err)
}

// RecordSecretRedacted records that a secret was redacted.
func (m *Metrics) RecordSecretRedacted(_ context.Context, _ string) {
	m.incCounter("secret_redacted_total")
}

// RecordConfigChangeEvent records a config change event.
func (m *Metrics) RecordConfigChangeEvent(_ context.Context, _, _ string) {
	m.incCounter("config_change_total")
}

// RecordOperation records a generic operation.
func (m *Metrics) RecordOperation(ctx context.Context, _ string, duration time.Duration, err error) {
	m.record(ctx, "operation", duration, err)
}

// ---------------------------------------------------------------------------
// Read accessors
// ---------------------------------------------------------------------------

// Counter returns the current value of a counter metric.
func (m *Metrics) Counter(name string) uint64 {
	m.mu.RLock()
	c, exists := m.counters[name]
	m.mu.RUnlock()
	if !exists {
		return 0
	}
	return c.get()
}

// Histogram returns a snapshot of a histogram metric.
//
//nolint:revive // unexported return type is intentional for internal use
func (m *Metrics) Histogram(name string) histogramSnapshot {
	m.mu.RLock()
	h, exists := m.histograms[name]
	m.mu.RUnlock()
	if !exists {
		return histogramSnapshot{}
	}
	return h.snapshot()
}

// AllCounters returns a copy of all counter values.
//
// nolint:dupl // This is similar to AllHistograms but serves a different use case for testing and introspection.
func (m *Metrics) AllCounters() map[string]uint64 {
	return withReadLock(m, func() map[string]uint64 {
		return cloneMetricMap(m.counters, func(c *counter) uint64 {
			return c.get()
		})
	})
}

// AllHistograms returns a copy of all histogram snapshots.
//
//nolint:revive,dupl // unexported return type is intentional, and the shape mirrors AllCounters
func (m *Metrics) AllHistograms() map[string]histogramSnapshot {
	return withReadLock(m, func() map[string]histogramSnapshot {
		return cloneMetricMap(m.histograms, func(h *histogram) histogramSnapshot {
			return h.snapshot()
		})
	})
}

// Reset clears all metrics.
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters = make(map[string]*counter)
	m.histograms = make(map[string]*histogram)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// record is an internal helper to record a generic operation with custom metric names.
func (m *Metrics) record(_ context.Context, name string, duration time.Duration, err error) {
	m.incCounter(name + "_total")

	if err != nil {
		m.incCounter(name + "_errors")
	}

	m.observeHistogram(name+"_duration", duration.Seconds())
}

func (m *Metrics) incCounter(name string) {
	m.counterFor(name).inc()
}

func (m *Metrics) incCounterBy(name string, n uint64) {
	m.counterFor(name).add(n)
}

func (m *Metrics) observeHistogram(name string, value float64) {
	m.histogramFor(name).observe(value)
}

func (m *Metrics) counterFor(name string) *counter {
	return getOrCreateMetric(m, m.counters, name, func() *counter {
		return &counter{}
	})
}

func (m *Metrics) histogramFor(name string) *histogram {
	return getOrCreateMetric(m, m.histograms, name, func() *histogram {
		return &histogram{}
	})
}

func withReadLock[T any](m *Metrics, fn func() T) T {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return fn()
}

func getOrCreateMetric[V any](m *Metrics, metrics map[string]V, name string, newMetric func() V) V {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, exists := metrics[name]
	if !exists {
		metric = newMetric()
		metrics[name] = metric
	}

	return metric
}

//nolint:gofumpt // cloneMetricMap is a helper to create a copy of a metric map with snapshot values.
func cloneMetricMap[V any, R any](
	src map[string]V,
	snapshot func(V) R,
) map[string]R {
	result := make(map[string]R, len(src))
	for name, metric := range src {
		result[name] = snapshot(metric)
	}
	return result
}
