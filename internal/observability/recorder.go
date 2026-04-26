// Package observability provides infrastructure for recording metrics, traces,
// and audit information for configuration operations. All recorders are
// instance-based — NO global variables.
package observability

import (
	"context"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Recorder interface
// ---------------------------------------------------------------------------

// Recorder records observability data for configuration operations.
// Implementations may forward to Prometheus, OpenTelemetry, logs, etc.
//
//nolint:dupl // interface mirrors FuncRecorder struct fields by design
type Recorder interface {
	RecordReload(ctx context.Context, duration time.Duration, events int, err error)
	RecordSet(ctx context.Context, key string, duration time.Duration, err error)
	RecordDelete(ctx context.Context, key string, duration time.Duration, err error)
	RecordBatchSet(ctx context.Context, duration time.Duration, err error)
	RecordBind(ctx context.Context, duration time.Duration, err error)
	RecordValidation(ctx context.Context, duration time.Duration, err error)
	RecordHook(ctx context.Context, name string, duration time.Duration, err error)
	RecordSecretRedacted(ctx context.Context, operation string)
	RecordConfigChangeEvent(ctx context.Context, operation, actor string)
	RecordOperation(ctx context.Context, op string, duration time.Duration, err error)
}

// ---------------------------------------------------------------------------
// No-op recorder
// ---------------------------------------------------------------------------

// nopRecorder is a no-op implementation of Recorder.
type nopRecorder struct{}

// Nop returns a no-op recorder that discards all records. Safe to use as
// a default when no observability is needed.
func Nop() Recorder {
	return &nopRecorder{}
}

func (n *nopRecorder) RecordReload(_ context.Context, _ time.Duration, _ int, _ error)       {}
func (n *nopRecorder) RecordSet(_ context.Context, _ string, _ time.Duration, _ error)       {}
func (n *nopRecorder) RecordDelete(_ context.Context, _ string, _ time.Duration, _ error)    {}
func (n *nopRecorder) RecordBatchSet(_ context.Context, _ time.Duration, _ error)            {}
func (n *nopRecorder) RecordBind(_ context.Context, _ time.Duration, _ error)                {}
func (n *nopRecorder) RecordValidation(_ context.Context, _ time.Duration, _ error)          {}
func (n *nopRecorder) RecordHook(_ context.Context, _ string, _ time.Duration, _ error)      {}
func (n *nopRecorder) RecordSecretRedacted(_ context.Context, _ string)                      {}
func (n *nopRecorder) RecordConfigChangeEvent(_ context.Context, _, _ string)                {}
func (n *nopRecorder) RecordOperation(_ context.Context, _ string, _ time.Duration, _ error) {}

// ---------------------------------------------------------------------------
// MultiRecorder
// ---------------------------------------------------------------------------

// MultiRecorder fans out records to multiple Recorder implementations.
// If any recorder returns an error, it is silently discarded (best-effort).
type MultiRecorder struct {
	recorders []Recorder
}

// NewMultiRecorder creates a MultiRecorder that fans out to all given recorders.
func NewMultiRecorder(recorders ...Recorder) *MultiRecorder {
	// Filter out nil recorders.
	filtered := make([]Recorder, 0, len(recorders))
	for _, r := range recorders {
		if r != nil {
			filtered = append(filtered, r)
		}
	}
	return &MultiRecorder{recorders: filtered}
}

// Add adds a recorder to the multi-recorder.
func (m *MultiRecorder) Add(r Recorder) {
	if r == nil {
		return
	}
	m.recorders = append(m.recorders, r)
}

func (m *MultiRecorder) RecordReload(ctx context.Context, duration time.Duration, events int, err error) {
	for _, r := range m.recorders {
		r.RecordReload(ctx, duration, events, err)
	}
}

func (m *MultiRecorder) RecordSet(ctx context.Context, key string, duration time.Duration, err error) {
	for _, r := range m.recorders {
		r.RecordSet(ctx, key, duration, err)
	}
}

func (m *MultiRecorder) RecordDelete(ctx context.Context, key string, duration time.Duration, err error) {
	for _, r := range m.recorders {
		r.RecordDelete(ctx, key, duration, err)
	}
}

func (m *MultiRecorder) RecordBatchSet(ctx context.Context, duration time.Duration, err error) {
	for _, r := range m.recorders {
		r.RecordBatchSet(ctx, duration, err)
	}
}

func (m *MultiRecorder) RecordBind(ctx context.Context, duration time.Duration, err error) {
	for _, r := range m.recorders {
		r.RecordBind(ctx, duration, err)
	}
}

func (m *MultiRecorder) RecordValidation(ctx context.Context, duration time.Duration, err error) {
	for _, r := range m.recorders {
		r.RecordValidation(ctx, duration, err)
	}
}

func (m *MultiRecorder) RecordHook(ctx context.Context, name string, duration time.Duration, err error) {
	for _, r := range m.recorders {
		r.RecordHook(ctx, name, duration, err)
	}
}

func (m *MultiRecorder) RecordSecretRedacted(ctx context.Context, operation string) {
	for _, r := range m.recorders {
		r.RecordSecretRedacted(ctx, operation)
	}
}

func (m *MultiRecorder) RecordConfigChangeEvent(ctx context.Context, operation, actor string) {
	for _, r := range m.recorders {
		r.RecordConfigChangeEvent(ctx, operation, actor)
	}
}

func (m *MultiRecorder) RecordOperation(ctx context.Context, op string, duration time.Duration, err error) {
	for _, r := range m.recorders {
		r.RecordOperation(ctx, op, duration, err)
	}
}

// ---------------------------------------------------------------------------
// Functional recorder
// ---------------------------------------------------------------------------

// FuncRecorder is a Recorder that delegates to function callbacks.
// Only the callbacks that are set are called; nil callbacks are skipped.
//
//nolint:dupl // struct fields mirror the Recorder interface by design
type FuncRecorder struct {
	OnReload         func(ctx context.Context, duration time.Duration, events int, err error)
	OnSet            func(ctx context.Context, key string, duration time.Duration, err error)
	OnDelete         func(ctx context.Context, key string, duration time.Duration, err error)
	OnBatchSet       func(ctx context.Context, duration time.Duration, err error)
	OnBind           func(ctx context.Context, duration time.Duration, err error)
	OnValidation     func(ctx context.Context, duration time.Duration, err error)
	OnHook           func(ctx context.Context, name string, duration time.Duration, err error)
	OnSecretRedacted func(ctx context.Context, operation string)
	OnConfigChange   func(ctx context.Context, operation, actor string)
	OnOperation      func(ctx context.Context, op string, duration time.Duration, err error)
}

func (f *FuncRecorder) RecordReload(ctx context.Context, duration time.Duration, events int, err error) {
	if f.OnReload != nil {
		f.OnReload(ctx, duration, events, err)
	}
}

func (f *FuncRecorder) RecordSet(ctx context.Context, key string, duration time.Duration, err error) {
	if f.OnSet != nil {
		f.OnSet(ctx, key, duration, err)
	}
}

func (f *FuncRecorder) RecordDelete(ctx context.Context, key string, duration time.Duration, err error) {
	if f.OnDelete != nil {
		f.OnDelete(ctx, key, duration, err)
	}
}

func (f *FuncRecorder) RecordBatchSet(ctx context.Context, duration time.Duration, err error) {
	if f.OnBatchSet != nil {
		f.OnBatchSet(ctx, duration, err)
	}
}

func (f *FuncRecorder) RecordBind(ctx context.Context, duration time.Duration, err error) {
	if f.OnBind != nil {
		f.OnBind(ctx, duration, err)
	}
}

func (f *FuncRecorder) RecordValidation(ctx context.Context, duration time.Duration, err error) {
	if f.OnValidation != nil {
		f.OnValidation(ctx, duration, err)
	}
}

func (f *FuncRecorder) RecordHook(ctx context.Context, name string, duration time.Duration, err error) {
	if f.OnHook != nil {
		f.OnHook(ctx, name, duration, err)
	}
}

func (f *FuncRecorder) RecordSecretRedacted(ctx context.Context, operation string) {
	if f.OnSecretRedacted != nil {
		f.OnSecretRedacted(ctx, operation)
	}
}

func (f *FuncRecorder) RecordConfigChangeEvent(ctx context.Context, operation, actor string) {
	if f.OnConfigChange != nil {
		f.OnConfigChange(ctx, operation, actor)
	}
}

func (f *FuncRecorder) RecordOperation(ctx context.Context, op string, duration time.Duration, err error) {
	if f.OnOperation != nil {
		f.OnOperation(ctx, op, duration, err)
	}
}

// ---------------------------------------------------------------------------
// CallbackRecorder (collects records for testing)
// ---------------------------------------------------------------------------

// CallbackRecorder collects all records for inspection in tests.
// It is safe for concurrent use.
type CallbackRecorder struct {
	mu      sync.RWMutex
	records []Record
}

// Record represents a single recorded observability event.
type Record struct {
	Type      string
	Key       string
	Name      string
	Operation string
	Actor     string
	Duration  time.Duration
	Events    int
	Error     error
	Timestamp time.Time
}

// NewCallbackRecorder creates a new CallbackRecorder.
func NewCallbackRecorder() *CallbackRecorder {
	return &CallbackRecorder{
		records: make([]Record, 0),
	}
}

// Records returns a copy of all recorded items.
func (c *CallbackRecorder) Records() []Record {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Record, len(c.records))
	copy(result, c.records)
	return result
}

// Len returns the number of recorded items.
func (c *CallbackRecorder) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.records)
}

// Clear removes all records.
func (c *CallbackRecorder) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = c.records[:0]
}

func (c *CallbackRecorder) add(typ, key, name, op, actor string, dur time.Duration, events int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, Record{
		Type:      typ,
		Key:       key,
		Name:      name,
		Operation: op,
		Actor:     actor,
		Duration:  dur,
		Events:    events,
		Error:     err,
		Timestamp: time.Now().UTC(),
	})
}

func (c *CallbackRecorder) RecordReload(_ context.Context, duration time.Duration, events int, err error) {
	c.add("reload", "", "", "", "", duration, events, err)
}

func (c *CallbackRecorder) RecordSet(_ context.Context, key string, duration time.Duration, err error) {
	c.add("set", key, "", "", "", duration, 0, err)
}

func (c *CallbackRecorder) RecordDelete(_ context.Context, key string, duration time.Duration, err error) {
	c.add("delete", key, "", "", "", duration, 0, err)
}

func (c *CallbackRecorder) RecordBatchSet(_ context.Context, duration time.Duration, err error) {
	c.add("batch_set", "", "", "", "", duration, 0, err)
}

func (c *CallbackRecorder) RecordBind(_ context.Context, duration time.Duration, err error) {
	c.add("bind", "", "", "", "", duration, 0, err)
}

func (c *CallbackRecorder) RecordValidation(_ context.Context, duration time.Duration, err error) {
	c.add("validation", "", "", "", "", duration, 0, err)
}

func (c *CallbackRecorder) RecordHook(_ context.Context, name string, duration time.Duration, err error) {
	c.add("hook", "", name, "", "", duration, 0, err)
}

func (c *CallbackRecorder) RecordSecretRedacted(_ context.Context, operation string) {
	c.add("secret_redacted", "", "", operation, "", 0, 0, nil)
}

func (c *CallbackRecorder) RecordConfigChangeEvent(_ context.Context, operation, actor string) {
	c.add("config_change", "", "", operation, actor, 0, 0, nil)
}

func (c *CallbackRecorder) RecordOperation(_ context.Context, op string, duration time.Duration, err error) {
	c.add("operation", "", "", op, "", duration, 0, err)
}
