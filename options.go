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

// Option configures a Config during creation.
type Option func(*options)

// options holds all configuration for a Config instance.
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

// defaultOptions returns options with sensible defaults.
func defaultOptions() options {
	return options{
		maxWorkers:      8,
		onReloadErr:     defaultReloadErrHandler,
		recorder:        observability.Nop(),
		defaultDebounce: 500 * time.Millisecond,
	}
}

// WithLayer adds a pre-constructed Layer to the engine.
func WithLayer(l *core.Layer) Option {
	return func(o *options) {
		o.layers = append(o.layers, l)
	}
}

// WithLoader adds a Loader as a new Layer with its own priority.
func WithLoader(l loader.Loader) Option {
	return func(o *options) {
		layer := core.NewLayer(l.String(),
			core.WithLayerPriority(l.Priority()),
			core.WithLayerSource(l),
		)
		o.layers = append(o.layers, layer)
	}
}

// WithProvider adds a Provider as a new Layer.
func WithProvider(p provider.Provider) Option {
	return func(o *options) {
		layer := core.NewLayer(p.Name(),
			core.WithLayerPriority(p.Priority()),
			core.WithLayerSource(p),
		)
		o.layers = append(o.layers, layer)
	}
}

// WithValidator sets the validation engine used after binding.
func WithValidator(v validator.Validator) Option {
	return func(o *options) { o.val = v }
}

// WithReloadErrorHandler sets the callback invoked when a background reload fails.
// Default: log at slog WARN.
func WithReloadErrorHandler(fn func(error)) Option {
	return func(o *options) { o.onReloadErr = fn }
}

// WithStrictReload causes New to return an error if any layer fails during
// the initial reload, rather than wrapping failures in a ReloadWarning.
func WithStrictReload() Option {
	return func(o *options) { o.strictReload = true }
}

// WithRecorder sets the observability recorder.
// Default: observability.Nop().
func WithRecorder(r observability.Recorder) Option {
	return func(o *options) {
		if r != nil {
			o.recorder = r
		}
	}
}

// WithPlugin registers a plugin with the config.
func WithPlugin(p plugin.Plugin) Option {
	return func(o *options) {
		o.plugins = append(o.plugins, p)
	}
}

// WithMaxWorkers sets the maximum number of concurrent layer-load goroutines.
// Passed through to the underlying core.Engine. Default: 8.
func WithMaxWorkers(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxWorkers = n
		}
	}
}

// WithDebounce sets the default debounce duration for watch-triggered reloads.
// Default: 500ms.
func WithDebounce(d time.Duration) Option {
	return func(o *options) { o.defaultDebounce = d }
}
