// Package observability provides metrics recording, tracing, and OpenTelemetry
// integration for the config framework.
package observability

import (
	"context"
	"time"
)

// Recorder records metrics for config operations.
// All methods must be safe for concurrent use.
type Recorder interface {
	RecordLoad(ctx context.Context, source string, dur time.Duration, err error)
	RecordReload(ctx context.Context, dur time.Duration, keyCount int, err error)
	RecordSet(ctx context.Context, key string, dur time.Duration, err error)
	RecordBatchSet(ctx context.Context, dur time.Duration, err error)
	RecordDelete(ctx context.Context, key string, dur time.Duration, err error)
	RecordBind(ctx context.Context, dur time.Duration, err error)
	RecordHook(ctx context.Context, name string, dur time.Duration, err error)
	RecordLayerLoad(ctx context.Context, layer string, dur time.Duration, keyCount int, err error)
	// RecordValidation records a validation operation result.
	RecordValidation(ctx context.Context, dur time.Duration, err error)
	// RecordWatchEvent records a single watch event received from a source.
	RecordWatchEvent(ctx context.Context, source string)
}

// NopRecorder is a no-op Recorder used when observability is not needed.
type NopRecorder struct{}

var _ Recorder = NopRecorder{}

// Nop returns a NopRecorder.
func Nop() Recorder { return NopRecorder{} }

// RecordLoad implements Recorder.
func (NopRecorder) RecordLoad(_ context.Context, _ string, _ time.Duration, _ error) {}

// RecordReload implements Recorder.
func (NopRecorder) RecordReload(_ context.Context, _ time.Duration, _ int, _ error) {}

// RecordSet implements Recorder.
func (NopRecorder) RecordSet(_ context.Context, _ string, _ time.Duration, _ error) {}

// RecordBatchSet implements Recorder.
func (NopRecorder) RecordBatchSet(_ context.Context, _ time.Duration, _ error) {}

// RecordDelete implements Recorder.
func (NopRecorder) RecordDelete(_ context.Context, _ string, _ time.Duration, _ error) {}

// RecordBind implements Recorder.
func (NopRecorder) RecordBind(_ context.Context, _ time.Duration, _ error) {}

// RecordHook implements Recorder.
func (NopRecorder) RecordHook(_ context.Context, _ string, _ time.Duration, _ error) {}

// RecordLayerLoad implements Recorder.
func (NopRecorder) RecordLayerLoad(_ context.Context, _ string, _ time.Duration, _ int, _ error) {}

// RecordValidation implements Recorder.
func (NopRecorder) RecordValidation(_ context.Context, _ time.Duration, _ error) {}

// RecordWatchEvent implements Recorder.
func (NopRecorder) RecordWatchEvent(_ context.Context, _ string) {}
