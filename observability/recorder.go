// Package observability provides interfaces and implementations for recording
// configuration operation metrics and traces. It includes a no-op recorder,
// an in-memory atomic metrics recorder, an OpenTelemetry recorder, and
// trace ID generation utilities.
package observability

import (
	"context"
	"time"
)

// Recorder defines the interface for recording configuration operation metrics.
// Implementations can forward metrics to Prometheus, OpenTelemetry, statsd, etc.
type Recorder interface {
	// RecordLoad records a configuration load operation.
	RecordLoad(ctx context.Context, source string, dur time.Duration, err error)
	// RecordReload records a full configuration reload.
	RecordReload(ctx context.Context, dur time.Duration, keyCount int, err error)
	// RecordSet records a single key set operation.
	RecordSet(ctx context.Context, key string, dur time.Duration, err error)
	// RecordBatchSet records a batch set operation.
	RecordBatchSet(ctx context.Context, dur time.Duration, err error)
	// RecordDelete records a key deletion.
	RecordDelete(ctx context.Context, key string, dur time.Duration, err error)
	// RecordBind records a struct binding operation.
	RecordBind(ctx context.Context, dur time.Duration, err error)
	// RecordHook records a hook execution.
	RecordHook(ctx context.Context, name string, dur time.Duration, err error)
	// RecordLayerLoad records a single layer load operation.
	RecordLayerLoad(ctx context.Context, layer string, dur time.Duration, keyCount int, err error)
	// RecordValidation records a validation operation.
	RecordValidation(ctx context.Context, dur time.Duration, err error)
	// RecordWatchEvent records that a watch event was received from a source.
	RecordWatchEvent(ctx context.Context, source string)
}

// NopRecorder is a no-op implementation of Recorder that discards all recordings.
// Use this when observability is not needed.
type NopRecorder struct{}

var _ Recorder = NopRecorder{}

// Nop returns a new NopRecorder.
func Nop() Recorder { return NopRecorder{} }

func (NopRecorder) RecordLoad(_ context.Context, _ string, _ time.Duration, _ error)             {}
func (NopRecorder) RecordReload(_ context.Context, _ time.Duration, _ int, _ error)              {}
func (NopRecorder) RecordSet(_ context.Context, _ string, _ time.Duration, _ error)              {}
func (NopRecorder) RecordBatchSet(_ context.Context, _ time.Duration, _ error)                   {}
func (NopRecorder) RecordDelete(_ context.Context, _ string, _ time.Duration, _ error)           {}
func (NopRecorder) RecordBind(_ context.Context, _ time.Duration, _ error)                       {}
func (NopRecorder) RecordHook(_ context.Context, _ string, _ time.Duration, _ error)             {}
func (NopRecorder) RecordLayerLoad(_ context.Context, _ string, _ time.Duration, _ int, _ error) {}
func (NopRecorder) RecordValidation(_ context.Context, _ time.Duration, _ error)                 {}
func (NopRecorder) RecordWatchEvent(_ context.Context, _ string)                                 {}
