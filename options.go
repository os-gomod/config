package config

import (
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/circuit"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/observability"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/provider"
	"github.com/os-gomod/config/validator"
)

// Priority constants for configuration sources.
// These mirror loader.Priority* values and are provided for convenience
// when constructing providers or layers outside the loader package.
const (
	PriorityDefault = 0
	PriorityMemory  = 20
	PriorityFile    = 30
	PriorityEnv     = 40
)

// Option is a functional option for configuring [New].
type Option func(*options)

// options holds the internal configuration for [New].
type options struct {
	layers          []*core.Layer
	namespace       string
	val             validator.Validator
	maxWorkers      int
	strictReload    bool
	deltaReload     bool
	batchedReload   bool
	cacheTTL        time.Duration
	onReloadErr     func(error)
	recorder        observability.Recorder
	tracer          trace.Tracer
	plugins         []plugin.Plugin
	defaultDebounce time.Duration
	schemaTarget    any
}

func defaultOptions() options {
	return options{
		maxWorkers:      8,
		onReloadErr:     defaultReloadErrHandler,
		recorder:        observability.Nop(),
		defaultDebounce: 500 * time.Millisecond,
	}
}

// WithLayer adds a pre-configured layer to the config engine.
// Layers are sorted by priority (higher wins) before merging.
func WithLayer(l *core.Layer) Option {
	return func(o *options) {
		o.layers = append(o.layers, l)
	}
}

// WithLoader adds a loader as a configuration source layer.
// The layer name and priority are derived from the loader.
func WithLoader(l loader.Loader) Option {
	return func(o *options) {
		layer := core.NewLayer(l.String(),
			core.WithLayerPriority(l.Priority()),
			core.WithLayerSource(l),
		)
		o.layers = append(o.layers, layer)
	}
}

// WithProvider adds a remote provider as a configuration source layer.
// The layer name and priority are derived from the provider.
func WithProvider(p provider.Provider) Option {
	return func(o *options) {
		layer := core.NewLayer(p.Name(),
			core.WithLayerPriority(p.Priority()),
			core.WithLayerSource(p),
		)
		o.layers = append(o.layers, layer)
	}
}

// WithValidator sets the validator used after struct binding.
// When set, [Config.Bind] and [Config.Validate] use this validator.
func WithValidator(v validator.Validator) Option {
	return func(o *options) { o.val = v }
}

// WithReloadErrorHandler sets a custom handler for background reload errors.
// By default, reload errors are logged via slog.Warn.
func WithReloadErrorHandler(fn func(error)) Option {
	return func(o *options) { o.onReloadErr = fn }
}

// WithStrictReload enables strict mode: if any layer fails during reload,
// the reload returns an error instead of using last-good data.
func WithStrictReload() Option {
	return func(o *options) { o.strictReload = true }
}

// WithDeltaReload enables delta optimization: layers whose data has not
// changed (by checksum) since the last reload are skipped entirely.
func WithDeltaReload() Option {
	return func(o *options) { o.deltaReload = true }
}

// WithBatchedReload enables batched reload mode: concurrent reload requests
// within a short window are coalesced into a single reload operation.
func WithBatchedReload(enabled bool) Option {
	return func(o *options) { o.batchedReload = enabled }
}

// WithCacheTTL sets a time-to-live for cached configuration values.
// After the TTL expires, the next Get call will trigger a fresh reload.
func WithCacheTTL(ttl time.Duration) Option {
	return func(o *options) { o.cacheTTL = ttl }
}

// WithRecorder sets the observability recorder for metrics and traces.
// A no-op recorder is used by default.
func WithRecorder(r observability.Recorder) Option {
	return func(o *options) {
		if r != nil {
			o.recorder = r
		}
	}
}

// WithPlugin registers a plugin that can add custom loaders,
// providers, decoders, validators, and event subscribers.
func WithPlugin(p plugin.Plugin) Option {
	return func(o *options) {
		o.plugins = append(o.plugins, p)
	}
}

// WithMaxWorkers sets the maximum number of goroutines used to load
// layers concurrently during reload. Defaults to 8.
func WithMaxWorkers(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxWorkers = n
		}
	}
}

// WithDebounce sets the default debounce interval for watch-triggered
// reloads. Multiple rapid file changes within this window are coalesced
// into a single reload. Defaults to 500ms.
func WithDebounce(d time.Duration) Option {
	return func(o *options) { o.defaultDebounce = d }
}

// WithTracer sets the OpenTelemetry Tracer used to create spans for
// Reload, Set, Bind, and Watch operations. If nil, no spans are created.
func WithTracer(t trace.Tracer) Option {
	return func(o *options) {
		if t != nil {
			o.tracer = t
		}
	}
}

// WithCircuitBreakerLayer creates a layer with the given circuit breaker
// configuration. This is a convenience option that combines WithLayer and
// WithLayerCircuitBreaker from the core package.
//
// Example:
//
//	config.New(ctx,
//	        config.WithLoader(myLoader),
//	        config.WithCircuitBreakerLayer("consul", consulProvider, circuit.BreakerConfig{
//	                Threshold:        3,
//	                Timeout:          15 * time.Second,
//	                SuccessThreshold: 2,
//	        }),
//	)
func WithCircuitBreakerLayer(name string, src core.Loadable, priority int, cbCfg circuit.BreakerConfig) Option {
	return func(o *options) {
		layer := core.NewLayer(name,
			core.WithLayerPriority(priority),
			core.WithLayerSource(src),
			core.WithLayerCircuitBreaker(cbCfg),
		)
		o.layers = append(o.layers, layer)
	}
}

// WithNamespace sets a dot-separated namespace prefix (e.g., "tenant.acme.")
// that is automatically prepended to all key lookups, sets, and deletes.
// Useful for multi-tenant or environment-scoped configurations.
func WithNamespace(ns string) Option {
	return func(o *options) { o.namespace = ns }
}

// WithSchemaValidation sets a target struct that will be automatically bound
// and validated after every successful reload. If validation fails:
//   - In strict mode ([WithStrictReload]), the reload returns an error.
//   - In non-strict mode, a warning is logged and the reload succeeds.
func WithSchemaValidation(schemaTarget any) Option {
	return func(o *options) { o.schemaTarget = schemaTarget }
}
