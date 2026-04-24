package core

import (
	"time"

	"github.com/os-gomod/config/core/circuit"
)

// Option is a functional option for configuring an [Engine] during creation
// with [New].
type Option func(*Engine)

// WithLayer appends a single [Layer] to the engine's layer stack.
func WithLayer(l *Layer) Option {
	return func(e *Engine) {
		e.layers = append(e.layers, l)
	}
}

// WithLayers appends multiple [Layer] values to the engine's layer stack.
func WithLayers(ls ...*Layer) Option {
	return func(e *Engine) {
		e.layers = append(e.layers, ls...)
	}
}

// WithMaxWorkers sets the maximum number of concurrent goroutines used when
// reloading layers in parallel. Values less than or equal to zero are
// ignored. Default is 8.
func WithMaxWorkers(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.maxWorkers = n
		}
	}
}

// WithDeltaReload enables delta optimization: unchanged layers are
// skipped during reload based on checksum comparison.
func WithDeltaReload() Option {
	return func(e *Engine) { e.deltaReload = true }
}

func WithBatchedReload(enabled bool) Option {
	return func(e *Engine) { e.batchedReload = enabled }
}

func WithCacheTTL(ttl time.Duration) Option {
	return func(e *Engine) { e.cacheTTL = ttl }
}

// LayerOption is a functional option for configuring a [Layer] during
// creation with [NewLayer].
type LayerOption func(*Layer)

// WithLayerPriority sets the merge priority of a layer. Higher values win
// during conflict resolution. Default is 10.
func WithLayerPriority(p int) LayerOption {
	return func(l *Layer) { l.priority = p }
}

// WithLayerSource sets the [Loadable] data source for a layer. If nil, the
// layer loads as an empty map.
func WithLayerSource(src Loadable) LayerOption {
	return func(l *Layer) { l.source = src }
}

// WithLayerTimeout sets the per-load timeout for a layer's data source.
// Default is 2 seconds.
func WithLayerTimeout(d time.Duration) LayerOption {
	return func(l *Layer) { l.timeout = d }
}

// WithLayerEnabled sets whether the layer is initially enabled. Disabled
// layers are skipped during reloads. Default is true.
func WithLayerEnabled(enabled bool) LayerOption {
	return func(l *Layer) { l.enabled.Store(enabled) }
}

// WithLayerCircuitBreaker sets the circuit breaker configuration for a layer.
// The circuit breaker protects against repeated load failures by short-
// circuiting and returning the last known good data.
func WithLayerCircuitBreaker(cfg circuit.BreakerConfig) LayerOption {
	return func(l *Layer) { l.cb = circuit.New(cfg) }
}
