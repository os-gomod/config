// Package pollwatch provides shared polling and event-diff infrastructure
// used by both loaders and remote config providers. It eliminates duplication
// of the pollWatch/emitDiff pattern across consul, etcd, nats, and loader packages.
package pollwatch

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

// Controller manages an event channel with graceful close semantics.
// It is the single source of truth for polling-based config source implementations,
// replacing the identical stopCh/eventCh/emitDiff patterns previously duplicated
// across consul, etcd, nats providers and the file loader.
type Controller struct {
	eventCh   chan event.Event
	closed    chan struct{}
	closeOnce sync.Once
}

// NewController creates a Controller with a buffered event channel.
// If bufferSize <= 0, a default of 100 is used.
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

// Done returns a channel that is closed when the controller is closed.
func (c *Controller) Done() <-chan struct{} {
	return c.closed
}

// Close closes the controller. It is safe to call multiple times.
func (c *Controller) Close() {
	c.closeOnce.Do(func() { close(c.closed) })
}

// EventCh returns the underlying event channel for direct publishing
// when the consumer needs full control.
func (c *Controller) EventCh() chan<- event.Event {
	return c.eventCh
}

// StartPolling launches a goroutine that calls callback at each interval tick.
// The goroutine exits when ctx is cancelled or the controller is closed.
// Returns the event channel that callbacks should publish to.
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

// EmitDiff computes the diff between old and new data maps and publishes
// create/update/delete events to the controller's event channel.
// Returns ctx.Err() if context is cancelled, or nil on close/empty diff.
func (c *Controller) EmitDiff(
	ctx context.Context,
	old, newData map[string]value.Value,
	opts ...event.Option,
) error {
	evts := event.NewDiffEvents(old, newData, opts...)
	for i := range evts {
		select {
		case c.eventCh <- evts[i]:
		case <-ctx.Done():
			return ctx.Err()
		case <-c.closed:
			return nil
		}
	}
	return nil
}
