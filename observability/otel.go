package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OTelRecorder is a [Recorder] implementation that exports metrics
// to OpenTelemetry. It creates counters and histograms for all
// config operations.
type OTelRecorder struct {
	loads              metric.Int64Counter
	reloads            metric.Int64Counter
	reloadDuration     metric.Float64Histogram
	errors             metric.Int64Counter
	layerLoadDur       metric.Float64Histogram
	keyCount           metric.Int64UpDownCounter
	validationFails    metric.Int64Counter
	watchEvents        metric.Int64Counter
	secretsRedacted    metric.Int64Counter
	configChangeEvents metric.Int64Counter
	tracer             trace.Tracer
}

var _ Recorder = (*OTelRecorder)(nil)

// NewOTelRecorder creates an OTel-backed recorder with the given
// meter and tracer. Both must be non-nil.
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
	r.secretsRedacted, err = meter.Int64Counter(
		"config.secrets.redacted",
		metric.WithUnit("{redaction}"),
		metric.WithDescription("Number of times secret values were redacted in output"),
	)
	if err != nil {
		return nil, fmt.Errorf("create config.secrets.redacted counter: %w", err)
	}
	r.configChangeEvents, err = meter.Int64Counter(
		"config.change.events",
		metric.WithUnit("{event}"),
		metric.WithDescription("Number of configuration change events emitted"),
	)
	if err != nil {
		return nil, fmt.Errorf("create config.change.events counter: %w", err)
	}
	return r, nil
}

// RecordLoad records a load operation as a counter increment. On error, also
// increments the error counter with an "operation=load" attribute.
func (r *OTelRecorder) RecordLoad(ctx context.Context, _ string, _ time.Duration, err error) {
	r.loads.Add(ctx, 1)
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "load")))
	}
}

// RecordReload records a reload operation with duration and key count metrics.
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

// RecordSet records a set operation error (if any).
func (r *OTelRecorder) RecordSet(ctx context.Context, _ string, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "set")))
	}
}

// RecordBatchSet records a batch set operation error (if any).
func (r *OTelRecorder) RecordBatchSet(ctx context.Context, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "batch_set")))
	}
}

// RecordDelete records a delete operation error (if any).
func (r *OTelRecorder) RecordDelete(ctx context.Context, _ string, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "delete")))
	}
}

// RecordBind records a bind operation error (if any).
func (r *OTelRecorder) RecordBind(ctx context.Context, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "bind")))
	}
}

// RecordHook records a hook execution error (if any).
func (r *OTelRecorder) RecordHook(ctx context.Context, _ string, _ time.Duration, err error) {
	if err != nil {
		r.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "hook")))
	}
}

// RecordLayerLoad records a layer load duration with a "layer" attribute.
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

// RecordValidation records a validation failure (if any).
func (r *OTelRecorder) RecordValidation(ctx context.Context, _ time.Duration, err error) {
	if err != nil {
		r.validationFails.Add(ctx, 1)
	}
}

// RecordWatchEvent records a watch event with a "source" attribute.
func (r *OTelRecorder) RecordWatchEvent(ctx context.Context, source string) {
	r.watchEvents.Add(ctx, 1, metric.WithAttributes(attribute.String("source", source)))
}

func (r *OTelRecorder) RecordSecretRedacted(ctx context.Context, source string) {
	r.secretsRedacted.Add(ctx, 1, metric.WithAttributes(attribute.String("source", source)))
}

func (r *OTelRecorder) RecordConfigChangeEvent(ctx context.Context, eventType, source string) {
	r.configChangeEvents.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
			attribute.String("source", source),
		),
	)
}
