package loader

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

// PollingLoader wraps a load function with periodic polling and watch support.
// It embeds a Controller for managing watch goroutines.
type PollingLoader struct {
	*Base
	*Controller
	loadFn   func(context.Context) (map[string]value.Value, error)
	label    string
	interval time.Duration
	mu       sync.RWMutex
	lastData map[string]value.Value
}

// NewPollingLoader creates a PollingLoader with the given base, load function,
// label, polling interval, and event channel buffer size.
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
		Controller: NewController(bufSize),
		loadFn:     loadFn,
		label:      label,
		interval:   interval,
		lastData:   make(map[string]value.Value),
	}
}

// Load calls the underlying load function and caches the result.
// If the load function returns an error, the last known data is returned
// alongside the wrapped error.
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

// Watch starts the polling goroutine if a polling interval is configured.
// Returns (nil, nil) if polling is not enabled.
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

// LastData returns a safe copy of the most recently loaded data.
func (p *PollingLoader) LastData() map[string]value.Value {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return value.Copy(p.lastData)
}

// SetLastData replaces the cached data with a copy of the given map.
func (p *PollingLoader) SetLastData(data map[string]value.Value) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastData = value.Copy(data)
}

// Close stops the polling controller and closes the base.
func (p *PollingLoader) Close(ctx context.Context) error {
	if p.Controller != nil {
		p.Controller.Close()
	}
	return p.Base.Close(ctx)
}

// Controller manages the watch event channel and polling lifecycle.
type Controller struct {
	eventCh   chan event.Event
	closed    chan struct{}
	closeOnce sync.Once
}

// NewController creates a Controller with the given event channel buffer size.
func NewController(bufferSize int) *Controller {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &Controller{
		eventCh: make(chan event.Event, bufferSize),
		closed:  make(chan struct{}),
	}
}

// IsClosed reports whether the controller has been closed.
func (c *Controller) IsClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

// Close idempotently closes the controller, stopping any active polling.
func (c *Controller) Close() {
	c.closeOnce.Do(func() { close(c.closed) })
}

// StartPolling starts a goroutine that calls callback at the given interval.
// It returns the event channel on which diff events are emitted.
func (c *Controller) StartPolling(
	ctx context.Context,
	interval time.Duration,
	callback func(context.Context),
) <-chan event.Event {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.closed:
				return
			case <-ticker.C:
				if c.IsClosed() {
					return
				}
				callback(ctx)
			}
		}
	}()
	return c.eventCh
}

// EmitDiff computes and emits diff events between old and newData.
func (c *Controller) EmitDiff(
	ctx context.Context,
	old, newData map[string]value.Value,
	opts ...event.Option,
) error {
	for _, evt := range event.NewDiffEvents(old, newData, opts...) {
		select {
		case c.eventCh <- evt:
		case <-ctx.Done():
			return ctx.Err()
		case <-c.closed:
			return nil
		}
	}
	return nil
}
