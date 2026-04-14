package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OTelRecorder implements Recorder using OpenTelemetry metrics and traces.
// Construct it with NewOTelRecorder and inject the result via config.WithRecorder.
// It never reads from the global OTEL providers to avoid implicit coupling.
type OTelRecorder struct {
	// metric instruments
	loads           metric.Int64Counter
	reloads         metric.Int64Counter
	reloadDuration  metric.Float64Histogram
	errors          metric.Int64Counter
	layerLoadDur    metric.Float64Histogram
	keyCount        metric.Int64UpDownCounter
	validationFails metric.Int64Counter
	watchEvents     metric.Int64Counter
	// tracer for slow-operation spans
	tracer trace.Tracer
}

var _ Recorder = (*OTelRecorder)(nil)

// NewOTelRecorder creates and registers all OTel instruments.
// meter and tracer must be non-nil.
func NewOTelRecorder(meter metric.Meter, tracer trace.Tracer) (*OTelRecorder, error) {
	if meter == nil {
		return nil, fmt.Errorf("observability: meter must not be nil")
	}
	if tracer == nil {
		return nil, fmt.Errorf("observability: tracer must not be nil")
	}

	r := &OTelRecorder{tracer: tracer}

	var err error
	r.loads, err = meter.Int64Counter("config.loads", metric.WithUnit("{call}"))
	if err != nil {
		return nil, fmt.Errorf("create config.loads counter: %w", err)
	}

	r.reloads, err = meter.Int64Counter("config.reloads", metric.WithUnit("{call}"))
	if err != nil {
		return nil, fmt.Errorf("create config.reloads counter: %w", err)
	}

	r.reloadDuration, err = meter.Float64Histogram("config.reload.duration", metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("create config.reload.duration histogram: %w", err)
	}

	r.errors, err = meter.Int64Counter("config.errors", metric.WithUnit("{error}"))
	if err != nil {
		return nil, fmt.Errorf("create config.errors counter: %w", err)
	}

	r.layerLoadDur, err = meter.Float64Histogram(
		"config.layer.load.duration",
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("create config.layer.load.duration histogram: %w", err)
	}

	r.keyCount, err = meter.Int64UpDownCounter("config.key_count", metric.WithUnit("{key}"))
	if err != nil {
		return nil, fmt.Errorf("create config.key_count counter: %w", err)
	}

	r.validationFails, err = meter.Int64Counter(
		"config.validation.failures",
		metric.WithUnit("{failure}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create config.validation.failures counter: %w", err)
	}

	r.watchEvents, err = meter.Int64Counter("config.watcher.events", metric.WithUnit("{event}"))
	if err != nil {
		return nil, fmt.Errorf("create config.watcher.events counter: %w", err)
	}

	return r, nil
}

// RecordLoad implements Recorder.
func (r *OTelRecorder) RecordLoad(ctx context.Context, _ string, _ time.Duration, err error) {
	r.loads.Add(ctx, 1)
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "load")))
	}
}

// RecordReload implements Recorder.
func (r *OTelRecorder) RecordReload(
	ctx context.Context,
	dur time.Duration,
	keyCount int,
	err error,
) {
	r.reloads.Add(ctx, 1)
	r.reloadDuration.Record(ctx, float64(dur.Milliseconds()))
	r.keyCount.Add(ctx, int64(keyCount))
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "reload")))
	}
}

// RecordSet implements Recorder.
func (r *OTelRecorder) RecordSet(ctx context.Context, _ string, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "set")))
	}
}

// RecordBatchSet implements Recorder.
func (r *OTelRecorder) RecordBatchSet(ctx context.Context, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "batch_set")))
	}
}

// RecordDelete implements Recorder.
func (r *OTelRecorder) RecordDelete(ctx context.Context, _ string, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "delete")))
	}
}

// RecordBind implements Recorder.
func (r *OTelRecorder) RecordBind(ctx context.Context, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "bind")))
	}
}

// RecordHook implements Recorder.
func (r *OTelRecorder) RecordHook(ctx context.Context, _ string, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "hook")))
	}
}

// RecordLayerLoad implements Recorder.
func (r *OTelRecorder) RecordLayerLoad(
	ctx context.Context,
	layer string,
	dur time.Duration,
	_ int,
	err error,
) {
	r.layerLoadDur.Record(
		ctx,
		float64(dur.Milliseconds()),
		metric.WithAttributes(attribute.String("layer", layer)),
	)
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "layer_load")))
	}
}

// RecordValidation implements Recorder.
func (r *OTelRecorder) RecordValidation(ctx context.Context, _ time.Duration, err error) {
	if err != nil {
		r.validationFails.Add(ctx, 1)
	}
}

// RecordWatchEvent implements Recorder.
func (r *OTelRecorder) RecordWatchEvent(ctx context.Context, source string) {
	r.watchEvents.Add(ctx, 1, metric.WithAttributes(attribute.String("source", source)))
}
