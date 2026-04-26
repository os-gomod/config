package config

import (
	"log/slog"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/layer"
	"github.com/os-gomod/config/v2/internal/observability"
	"github.com/os-gomod/config/v2/internal/registry"
	"github.com/os-gomod/config/v2/internal/service"
)

// Option configures a Config instance.
type Option func(*configBuilder)

type configBuilder struct {
	layers          []*layer.Layer
	plugins         []service.Plugin
	namespace       string
	strictReload    bool
	deltaReload     bool
	maxWorkers      int
	defaultDebounce time.Duration
	recorder        observability.Recorder
	tracer          any // accepts any Tracer implementation (e.g. pipeline.Tracer)
	logger          *slog.Logger
	busWorkers      int
	busQueueSize    int
	onReloadErr     func(error)
	bundle          *registry.Bundle
}

// WithLayers adds configuration layers.
func WithLayers(layers ...*layer.Layer) Option {
	return func(c *configBuilder) { c.layers = append(c.layers, layers...) }
}

// WithLayer adds a single configuration layer.
func WithLayer(l *layer.Layer) Option {
	return func(c *configBuilder) { c.layers = append(c.layers, l) }
}

// WithPlugins adds configuration plugins.
func WithPlugins(plugins ...service.Plugin) Option {
	return func(c *configBuilder) { c.plugins = append(c.plugins, plugins...) }
}

// WithNamespace sets a namespace prefix for all keys.
func WithNamespace(ns string) Option {
	return func(c *configBuilder) { c.namespace = ns }
}

// WithStrictReload enables strict reload mode.
// In strict mode, any layer failure causes the initial reload to return an error.
func WithStrictReload(strict bool) Option {
	return func(c *configBuilder) { c.strictReload = strict }
}

// WithDeltaReload enables delta reload (skip unchanged layers).
func WithDeltaReload(enabled bool) Option {
	return func(c *configBuilder) { c.deltaReload = enabled }
}

// WithMaxWorkers sets the maximum number of concurrent layer loaders.
func WithMaxWorkers(n int) Option {
	return func(c *configBuilder) {
		if n > 0 {
			c.maxWorkers = n
		}
	}
}

// WithRecorder sets the observability recorder.
func WithRecorder(r observability.Recorder) Option {
	return func(c *configBuilder) { c.recorder = r }
}

// WithTracer sets a tracer for distributed tracing.
// The tracer must implement the pipeline.Tracer interface.
func WithTracer(tracer any) Option {
	return func(c *configBuilder) { c.tracer = tracer }
}

// WithLogger sets the structured logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *configBuilder) { c.logger = logger }
}

// WithBusWorkers sets the event bus worker count.
func WithBusWorkers(n int) Option {
	return func(c *configBuilder) {
		if n > 0 {
			c.busWorkers = n
		}
	}
}

// WithBusQueueSize sets the event bus queue capacity.
func WithBusQueueSize(n int) Option {
	return func(c *configBuilder) {
		if n > 0 {
			c.busQueueSize = n
		}
	}
}

// WithDebounce sets the default debounce interval for watch-triggered reloads.
func WithDebounce(d time.Duration) Option {
	return func(c *configBuilder) { c.defaultDebounce = d }
}

// WithOnReloadError sets the error handler for background reload failures.
func WithOnReloadError(fn func(error)) Option {
	return func(c *configBuilder) { c.onReloadErr = fn }
}

// WithRegistryBundle sets a custom registry bundle.
func WithRegistryBundle(b *registry.Bundle) Option {
	return func(c *configBuilder) { c.bundle = b }
}
