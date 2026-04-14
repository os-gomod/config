package core

import (
	"time"

	"github.com/os-gomod/config/core/circuit"
)

// Option configures an Engine during creation.
type Option func(*Engine)

// WithLayer appends a Layer to the Engine's layer list.
func WithLayer(l *Layer) Option {
	return func(e *Engine) {
		e.layers = append(e.layers, l)
	}
}

// WithLayers appends multiple Layers to the Engine's layer list.
func WithLayers(ls ...*Layer) Option {
	return func(e *Engine) {
		e.layers = append(e.layers, ls...)
	}
}

// WithMaxWorkers sets the maximum number of goroutines used to load layers
// concurrently during a Reload. Default: 8.
func WithMaxWorkers(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.maxWorkers = n
		}
	}
}

// LayerOption configures a Layer during creation.
type LayerOption func(*Layer)

// WithLayerPriority sets the merge priority of a Layer. Higher values take precedence.
func WithLayerPriority(p int) LayerOption {
	return func(l *Layer) { l.priority = p }
}

// WithLayerSource sets the data source of a Layer.
func WithLayerSource(src Loadable) LayerOption {
	return func(l *Layer) { l.source = src }
}

// WithLayerTimeout sets the per-load timeout for a Layer.
func WithLayerTimeout(d time.Duration) LayerOption {
	return func(l *Layer) { l.timeout = d }
}

// WithLayerEnabled sets the initial enabled state of a Layer.
func WithLayerEnabled(enabled bool) LayerOption {
	return func(l *Layer) { l.enabled.Store(enabled) }
}

// WithLayerCircuitBreaker replaces the Layer's circuit breaker with one
// configured by cfg.
func WithLayerCircuitBreaker(cfg circuit.BreakerConfig) LayerOption {
	return func(l *Layer) { l.cb = circuit.New(cfg) }
}
