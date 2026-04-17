package loader

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/pollwatch"
)

// PollingLoader wraps a load function with polling-based watching and diff tracking.
// It caches the last loaded data and emits change events when data changes between
// polls. This is useful for custom data sources that need periodic refresh.
//
// The polling goroutine uses the shared pollwatch.Controller for consistent
// lifecycle management across all loaders and providers.
type PollingLoader struct {
	*Base
	*pollwatch.Controller
	loadFn   func(context.Context) (map[string]value.Value, error)
	label    string
	interval time.Duration
	mu       sync.RWMutex
	lastData map[string]value.Value
}

// NewPollingLoader creates a polling loader that calls loadFn at each interval.
func NewPollingLoader(
	base *Base,
	loadFn func(context.Context) (map[string]value.Value, error),
	label string,
	interval time.Duration,
	bufSize int,
) *PollingLoader {
	if bufSize <= 0 {
		bufSize = 100
	}
	return &PollingLoader{
		Base:       base,
		Controller: pollwatch.NewController(bufSize),
		loadFn:     loadFn,
		label:      label,
		interval:   interval,
		lastData:   make(map[string]value.Value),
	}
}

// Load calls the underlying load function and caches the result.
func (p *PollingLoader) Load(ctx context.Context) (map[string]value.Value, error) {
	if p.IsClosed() {
		return nil, ErrClosed
	}
	data, err := p.loadFn(ctx)
	if err != nil {
		return p.LastData(), p.WrapErr(err, "load")
	}
	p.SetLastData(data)
	return value.Copy(data), nil
}

// Watch starts a polling goroutine that loads data at each interval and
// emits change events via the shared Controller.
func (p *PollingLoader) Watch(ctx context.Context) (<-chan event.Event, error) {
	if p.interval <= 0 {
		return nil, nil
	}
	return p.StartPolling(ctx, p.interval, func(ctx context.Context) {
		if p.IsClosed() {
			return
		}
		newData, err := p.loadFn(ctx)
		if err != nil {
			return
		}
		oldData := p.LastData()
		p.SetLastData(newData)
		_ = p.EmitDiff(ctx, oldData, newData,
			event.WithLabel("source", p.label),
			event.WithLabel("type", p.Type()),
		)
	}), nil
}

// LastData returns a copy of the most recently loaded data.
func (p *PollingLoader) LastData() map[string]value.Value {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return value.Copy(p.lastData)
}

// SetLastData updates the cached data (thread-safe copy).
func (p *PollingLoader) SetLastData(data map[string]value.Value) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastData = value.Copy(data)
}

// Close stops the polling goroutine and releases resources.
func (p *PollingLoader) Close(ctx context.Context) error {
	if p.Controller != nil {
		p.Controller.Close()
	}
	return p.Base.Close(ctx)
}
