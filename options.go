package config

import (
	"time"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/observability"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/provider"
	"github.com/os-gomod/config/validator"
)

// Option is a functional option used to configure [Config] during creation
// with [New].
type Option func(*options)

// options holds the internal configuration for [Config], populated by
// functional options and defaults.
type options struct {
	layers          []*core.Layer
	val             validator.Validator
	maxWorkers      int
	strictReload    bool
	onReloadErr     func(error)
	recorder        observability.Recorder
	plugins         []plugin.Plugin
	defaultDebounce time.Duration
}

// defaultOptions returns the baseline options used when no functional options
// are supplied to [New].
func defaultOptions() options {
	return options{
		maxWorkers:      8,
		onReloadErr:     defaultReloadErrHandler,
		recorder:        observability.Nop(),
		defaultDebounce: 500 * time.Millisecond,
	}
}

// WithLayer appends a pre-constructed [core.Layer] to the configuration
// layer stack. Layers are merged in priority order (higher priority wins).
func WithLayer(l *core.Layer) Option {
	return func(o *options) {
		o.layers = append(o.layers, l)
	}
}

// WithLoader wraps the given [loader.Loader] into a [core.Layer] and appends
// it to the layer stack. The layer's name and priority are derived from the
// loader itself.
func WithLoader(l loader.Loader) Option {
	return func(o *options) {
		layer := core.NewLayer(l.String(),
			core.WithLayerPriority(l.Priority()),
			core.WithLayerSource(l),
		)
		o.layers = append(o.layers, layer)
	}
}

// WithProvider wraps the given [provider.Provider] into a [core.Layer] and
// appends it to the layer stack. The layer's name and priority are derived
// from the provider itself.
func WithProvider(p provider.Provider) Option {
	return func(o *options) {
		layer := core.NewLayer(p.Name(),
			core.WithLayerPriority(p.Priority()),
			core.WithLayerSource(p),
		)
		o.layers = append(o.layers, layer)
	}
}

// WithValidator sets the custom validator used by [Config.Bind] and
// [Config.Validate].
func WithValidator(v validator.Validator) Option {
	return func(o *options) { o.val = v }
}

// WithReloadErrorHandler sets a custom handler that is invoked whenever a
// background reload triggered by a watcher fails. The default handler logs
// a warning via slog.
func WithReloadErrorHandler(fn func(error)) Option {
	return func(o *options) { o.onReloadErr = fn }
}

// WithStrictReload enables strict reload mode. When enabled, [New] returns
// an error if any layer fails during the initial reload. By default, partial
// failures produce a [*ReloadWarning] instead.
func WithStrictReload() Option {
	return func(o *options) { o.strictReload = true }
}

// WithRecorder sets the observability recorder used to track reload, set,
// delete, bind, and validation metrics. A nil recorder is silently ignored.
func WithRecorder(r observability.Recorder) Option {
	return func(o *options) {
		if r != nil {
			o.recorder = r
		}
	}
}

// WithPlugin registers a plugin that will be initialized when [New] is called.
// Plugins can register custom loaders, providers, decoders, and validators.
func WithPlugin(p plugin.Plugin) Option {
	return func(o *options) {
		o.plugins = append(o.plugins, p)
	}
}

// WithMaxWorkers sets the maximum number of concurrent goroutines used when
// reloading layers in parallel. Values less than or equal to zero are ignored.
// Default is 8.
func WithMaxWorkers(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxWorkers = n
		}
	}
}

// WithDebounce sets the default debounce duration used by watchers before
// triggering a reload after a configuration source change. Default is
// 500 milliseconds.
func WithDebounce(d time.Duration) Option {
	return func(o *options) { o.defaultDebounce = d }
}
