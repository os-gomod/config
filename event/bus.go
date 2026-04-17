package event

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/os-gomod/config/internal/pattern"
)

// Observer is a callback function that processes configuration events.
// It receives a context for cancellation and the event to process.
// If the observer returns an error, it is propagated by PublishSync.
type Observer func(ctx context.Context, evt Event) error

// defaultBusWorkers is the maximum number of concurrent goroutines
// used by the default Bus for async event dispatch.
const defaultBusWorkers = 256

// Bus is an async, pattern-matching event bus for configuration events.
// Subscribers register with glob patterns (e.g., "db.*", "server.port")
// and receive events whose keys match. The bus supports both asynchronous
// (Publish) and synchronous (PublishSync, PublishOrdered) dispatch.
//
// The bus is safe for concurrent use. All subscriptions are ordered by
// registration ID to ensure deterministic delivery order.
//
// Example:
//
//	bus := event.NewBus()
//	unsub := bus.Subscribe("db.*", func(ctx context.Context, evt event.Event) error {
//	    fmt.Printf("DB change: %s\n", evt.Key)
//	    return nil
//	})
//	defer unsub()
//	bus.Publish(ctx, event.NewUpdateEvent("db.host", oldVal, newVal))
type Bus struct {
	mu          sync.RWMutex
	catchAll    []subscription
	subscribers map[string][]subscription
	nextID      atomic.Uint64
	workers     chan struct{}

	// PanicHandler is called when an observer panics during async dispatch.
	// If nil, panics are silently recovered and discarded.
	PanicHandler func(recovered any)
}

// subscription pairs an observer callback with its unique registration ID.
type subscription struct {
	id       uint64
	observer Observer
}

// isCatchAll reports whether the pattern matches all events ("*" or "").
func isCatchAll(pat string) bool { return pat == "*" || pat == "" }

// NewBus creates a new Bus with default concurrency (256 workers).
func NewBus() *Bus {
	return NewBusWithConcurrency(defaultBusWorkers)
}

// NewBusWithConcurrency creates a new Bus that limits concurrent async
// observer dispatch to maxWorkers goroutines. If maxWorkers <= 0,
// the default of 256 is used.
func NewBusWithConcurrency(maxWorkers int) *Bus {
	if maxWorkers <= 0 {
		maxWorkers = defaultBusWorkers
	}
	return &Bus{
		subscribers: make(map[string][]subscription),
		workers:     make(chan struct{}, maxWorkers),
	}
}

// Subscribe registers an observer for events matching the given pattern.
// The pattern supports glob-style matching (e.g., "db.*", "server.*.port").
// Use "*" or "" to receive all events.
//
// Returns an unsubscribe function that removes the subscription when called.
// Events are delivered in registration order.
func (b *Bus) Subscribe(pat string, observer Observer) func() {
	id := b.nextID.Add(1)
	sub := subscription{id: id, observer: observer}
	b.mu.Lock()
	if isCatchAll(pat) {
		b.catchAll = append(b.catchAll, sub)
	} else {
		b.subscribers[pat] = append(b.subscribers[pat], sub)
	}
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if isCatchAll(pat) {
			b.catchAll = removeByID(b.catchAll, id)
		} else {
			subs := removeByID(b.subscribers[pat], id)
			if len(subs) == 0 {
				delete(b.subscribers, pat)
			} else {
				b.subscribers[pat] = subs
			}
		}
	}
}

// removeByID removes the subscription with the given ID from the slice.
func removeByID(subs []subscription, id uint64) []subscription {
	for i, s := range subs {
		if s.id == id {
			return append(subs[:i], subs[i+1:]...)
		}
	}
	return subs
}

// Publish dispatches the event asynchronously to all matching subscribers.
// Each observer runs in its own goroutine, bounded by the worker pool size.
// Panics in observers are recovered and passed to PanicHandler if set.
// Errors from observers are silently discarded; use PublishSync for error handling.
func (b *Bus) Publish(ctx context.Context, evt *Event) {
	subs := b.matched(evt.Key)
	for _, sub := range subs {
		b.workers <- struct{}{}
		go func(s subscription) {
			defer func() {
				if r := recover(); r != nil {
					if b.PanicHandler != nil {
						b.PanicHandler(r)
					}
				}
				<-b.workers
			}()
			_ = s.observer(ctx, *evt)
		}(sub)
	}
}

// PublishSync dispatches the event synchronously to all matching subscribers
// in registration order. Returns the first error from any observer, or nil
// if all observers succeed. Panics are caught and returned as errors.
func (b *Bus) PublishSync(ctx context.Context, evt *Event) error {
	subs := b.matched(evt.Key)
	var firstErr error
	for _, sub := range subs {
		if err := safeObserveSync(ctx, sub.observer, evt); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// PublishOrdered dispatches a slice of events sequentially and synchronously.
// Events are published one at a time in order using PublishSync.
// Returns the first error encountered, or nil if all events are delivered successfully.
func (b *Bus) PublishOrdered(ctx context.Context, events []Event) error {
	var firstErr error
	for i := range events {
		if err := b.PublishSync(ctx, &events[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Clear removes all subscribers from the bus.
func (b *Bus) Clear() {
	b.mu.Lock()
	b.subscribers = make(map[string][]subscription)
	b.catchAll = b.catchAll[:0]
	b.mu.Unlock()
}

// SubscriberCount returns the total number of active subscriptions,
// including catch-all subscriptions.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := len(b.catchAll)
	for _, subs := range b.subscribers {
		n += len(subs)
	}
	return n
}

// matched returns all subscriptions whose patterns match the given key,
// sorted by registration ID for deterministic delivery order.
func (b *Bus) matched(key string) []subscription {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]subscription, 0, len(b.catchAll)+4)
	out = append(out, b.catchAll...)
	for pat, subs := range b.subscribers {
		if pattern.Match(key, pat) {
			out = append(out, subs...)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// safeObserveSync calls an observer, catching any panics and returning
// them as formatted errors.
func safeObserveSync(ctx context.Context, obs Observer, evt *Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("event.Bus: observer panic: %v", r)
		}
	}()
	return obs(ctx, *evt)
}
