package event

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/os-gomod/config/internal/pattern"
)

// Observer is a callback invoked for each matching event.
type Observer func(ctx context.Context, evt Event) error

const defaultBusWorkers = 256

// Bus is a publish-subscribe hub for config events. Subscribers register
// patterns (e.g. "db.*") and receive events whose keys match. The zero
// value is not usable; use NewBus or NewBusWithConcurrency.
type Bus struct {
	mu          sync.RWMutex
	catchAll    []subscription
	subscribers map[string][]subscription
	nextID      atomic.Uint64
	workers     chan struct{}
	// PanicHandler is called when an observer panics. If nil, the panic
	// is silently recovered.
	PanicHandler func(recovered any)
}

type subscription struct {
	id       uint64
	observer Observer
}

// isCatchAll reports whether the pattern matches every key.
func isCatchAll(pat string) bool { return pat == "*" || pat == "" }

// NewBus creates a Bus with the default worker pool size (256).
func NewBus() *Bus {
	return NewBusWithConcurrency(defaultBusWorkers)
}

// NewBusWithConcurrency creates a Bus with the given maximum number of
// concurrent observer goroutines. Values less than 1 are clamped to the default.
func NewBusWithConcurrency(maxWorkers int) *Bus {
	if maxWorkers <= 0 {
		maxWorkers = defaultBusWorkers
	}
	return &Bus{
		subscribers: make(map[string][]subscription),
		workers:     make(chan struct{}, maxWorkers),
	}
}

// Subscribe registers an observer for events whose keys match the given pattern.
// The returned function unsubscribes the observer when called.
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

// Publish dispatches the event to all matching observers concurrently.
// Observer panics are recovered; PanicHandler is called if set.
func (b *Bus) Publish(ctx context.Context, evt Event) {
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
			_ = s.observer(ctx, evt)
		}(sub)
	}
}

// PublishSync dispatches the event to all matching observers synchronously,
// returning the first error encountered. Observer panics are recovered and
// returned as errors.
func (b *Bus) PublishSync(ctx context.Context, evt Event) error {
	subs := b.matched(evt.Key)
	var firstErr error
	for _, sub := range subs {
		if err := safeObserveSync(ctx, sub.observer, evt); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// PublishOrdered dispatches a slice of events sequentially via PublishSync,
// returning the first error encountered.
func (b *Bus) PublishOrdered(ctx context.Context, events []Event) error {
	var firstErr error
	for _, evt := range events {
		if err := b.PublishSync(ctx, evt); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Clear removes all subscriptions.
func (b *Bus) Clear() {
	b.mu.Lock()
	b.subscribers = make(map[string][]subscription)
	b.catchAll = b.catchAll[:0]
	b.mu.Unlock()
}

// SubscriberCount returns the total number of active subscriptions.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := len(b.catchAll)
	for _, subs := range b.subscribers {
		n += len(subs)
	}
	return n
}

// matched returns all subscriptions whose pattern matches key.
// O(n) over all subscriptions; acceptable for <=1000 subscribers per Bus instance.
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

// safeObserveSync calls the observer and recovers from panics, converting them
// to errors.
func safeObserveSync(ctx context.Context, obs Observer, evt Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("event.Bus: observer panic: %v", r)
		}
	}()
	return obs(ctx, evt)
}
