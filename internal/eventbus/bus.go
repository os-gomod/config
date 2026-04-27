// Package eventbus implements a scalable event bus with a bounded worker-pool
// dispatcher model. Events are published asynchronously into a fixed-capacity
// queue and delivered by a configurable pool of workers.
//
// Key design goals:
//   - Bounded concurrency: no goroutine-per-subscriber explosion.
//   - Back-pressure via queue capacity (publishers see drops, not unbounded memory).
//   - Graceful shutdown with full queue drain.
//   - Per-subscriber retry with exponential backoff.
//   - Panic-safe observer invocation.
package eventbus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/pattern"
)

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

const (
	defaultWorkerCount = 32
	defaultQueueSize   = 4096
	defaultRetryCount  = 0
	defaultRetryDelay  = 100 * time.Millisecond
)

// ---------------------------------------------------------------------------
// BusConfig
// ---------------------------------------------------------------------------

// BusConfig configures the event bus. Use Option functions to override defaults.
type BusConfig struct {
	// WorkerCount is the number of concurrent dispatch workers. Default: 32.
	WorkerCount int

	// QueueSize is the capacity of the event queue. Default: 4096.
	QueueSize int

	// RetryCount is how many times to retry a failed delivery. Default: 0.
	RetryCount int

	// RetryDelay is the base delay between retries. Default: 100ms.
	RetryDelay time.Duration

	// PanicHandler is called when an observer panics. Default: logs and continues.
	PanicHandler func(recovered any)
}

// ---------------------------------------------------------------------------
// Bus
// ---------------------------------------------------------------------------

// Bus is a scalable event bus using a worker-pool dispatcher model.
// Events are queued and delivered by a fixed pool of workers.
//
// All operations on Bus are safe for concurrent use.
type Bus struct {
	config BusConfig

	mu          sync.RWMutex
	catchAll    []subscription
	subscribers map[string][]subscription
	nextID      atomic.Uint64

	queue   chan dispatchJob
	workers []*worker
	wg      sync.WaitGroup
	closed  atomic.Bool

	// stopCtx is cancelled on Close to abort in-flight retry delays.
	stopCtx    context.Context
	stopCancel context.CancelFunc

	// Counters (atomic).
	delivered atomic.Uint64
	dropped   atomic.Uint64
	failed    atomic.Uint64
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// subscription represents a single registered observer with its routing metadata.
type subscription struct {
	id       uint64
	observer event.Observer
	pattern  string
}

// dispatchJob bundles an event with the list of subscribers that should receive it.
type dispatchJob struct {
	evt  event.Event
	subs []subscription
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// NewBus creates a new Bus with the given options applied over the defaults.
// Workers are started immediately and begin listening on the queue.
func NewBus(opts ...Option) *Bus {
	cfg := BusConfig{
		WorkerCount:  defaultWorkerCount,
		QueueSize:    defaultQueueSize,
		RetryCount:   defaultRetryCount,
		RetryDelay:   defaultRetryDelay,
		PanicHandler: defaultPanicHandler,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &Bus{
		config:      cfg,
		subscribers: make(map[string][]subscription),
		queue:       make(chan dispatchJob, cfg.QueueSize),
		stopCtx:     ctx,
		stopCancel:  cancel,
	}

	// Spin up worker pool.
	for i := range cfg.WorkerCount {
		w := newWorker(i, b.queue, b)
		b.workers = append(b.workers, w)
		b.wg.Add(1)
		go func(wk *worker) {
			defer b.wg.Done()
			wk.start()
		}(w)
	}

	return b
}

// defaultPanicHandler is the built-in panic handler that logs to stdout.
func defaultPanicHandler(recovered any) {
	fmt.Printf("[eventbus] recovered panic: %v\n", recovered)
}

// ---------------------------------------------------------------------------
// Subscribe
// ---------------------------------------------------------------------------

// Subscribe registers an observer for a pattern and returns an unsubscribe
// function. Calling the returned function removes the subscription.
//
// Pattern rules:
//   - "" or "*" registers as a catch-all (receives every event).
//   - "exact.key" matches only that exact key.
//   - "prefix.*" matches all keys whose first segment is "prefix".
//   - "app.*.config" matches "app.db.config", "app.cache.config", etc.
//
// Panics if observer is nil.
func (b *Bus) Subscribe(pat string, observer event.Observer) func() {
	if observer == nil {
		panic(errors.ErrNilObserver)
	}

	id := b.nextID.Add(1)
	sub := subscription{
		id:       id,
		observer: observer,
		pattern:  pat,
	}

	b.mu.Lock()
	if pat == "" || pat == "*" {
		b.catchAll = append(b.catchAll, sub)
	} else {
		b.subscribers[pat] = append(b.subscribers[pat], sub)
	}
	b.mu.Unlock()

	return func() {
		b.unsubscribe(id, pat)
	}
}

// unsubscribe removes the subscription with the given ID from the routing table.
func (b *Bus) unsubscribe(id uint64, pat string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	remove := func(slice []subscription) []subscription {
		for i, s := range slice {
			if s.id == id {
				return append(slice[:i], slice[i+1:]...)
			}
		}
		return slice
	}

	if pat == "" || pat == "*" {
		b.catchAll = remove(b.catchAll)
	} else {
		if subs, ok := b.subscribers[pat]; ok {
			filtered := remove(subs)
			if len(filtered) == 0 {
				delete(b.subscribers, pat)
			} else {
				b.subscribers[pat] = filtered
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Publish (async)
// ---------------------------------------------------------------------------

// Publish enqueues an event for asynchronous delivery by the worker pool.
// If the queue is full (back-pressure), the event is dropped and the Dropped
// counter is incremented. Returns ErrBusClosed if the bus has been shut down.
func (b *Bus) Publish(_ context.Context, evt *event.Event) error {
	if b.closed.Load() {
		return errors.ErrBusClosed
	}
	if evt == nil {
		return errors.New("eventbus: event must not be nil", "")
	}

	subs := b.matched(evt.Key)
	if len(subs) == 0 {
		return nil // nobody listening, nothing to do
	}

	job := dispatchJob{
		evt:  *evt,
		subs: subs,
	}

	select {
	case b.queue <- job:
		return nil
	default:
		b.dropped.Add(1)
		return errors.ErrQueueFull
	}
}

// ---------------------------------------------------------------------------
// PublishSync (synchronous)
// ---------------------------------------------------------------------------

// PublishSync delivers an event synchronously to all matched subscribers,
// blocking until every observer returns. Retry and panic handling apply
// identically to the async path.
func (b *Bus) PublishSync(_ context.Context, evt *event.Event) error {
	if b.closed.Load() {
		return errors.ErrBusClosed
	}
	if evt == nil {
		return errors.New("eventbus: event must not be nil", "")
	}

	subs := b.matched(evt.Key)
	if len(subs) == 0 {
		return nil
	}

	var firstErr error
	for _, sub := range subs {
		if err := b.deliverSync(evt, sub); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// deliverSync performs a single synchronous delivery with retry + panic recovery,
// mirroring the worker logic but without the job channel.
func (b *Bus) deliverSync(evt *event.Event, sub subscription) error {
	var err error
	for attempt := 0; attempt <= b.config.RetryCount; attempt++ {
		if attempt > 0 {
			delay := b.config.RetryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-time.After(delay):
			case <-b.stopCtx.Done():
				b.failed.Add(1)
				return errors.New("eventbus: delivery aborted during shutdown", "")
			}
		}

		err = b.deliverSafe(evt, sub)
		if err == nil {
			b.delivered.Add(1)
			return nil
		}
	}

	b.failed.Add(1)
	return fmt.Errorf("%w: %w", errors.ErrDeliveryFailed, err)
}

// deliverSafe invokes an observer with panic recovery.
func (b *Bus) deliverSafe(evt *event.Event, sub subscription) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if b.config.PanicHandler != nil {
				b.config.PanicHandler(r)
			}
			err = fmt.Errorf("eventbus: observer %d panicked: %v", sub.id, r)
		}
	}()

	return sub.observer(b.stopCtx, *evt)
}

// ---------------------------------------------------------------------------
// PublishOrdered (synchronous, ordered)
// ---------------------------------------------------------------------------

// PublishOrdered delivers multiple events in strict order, synchronously.
// Events are delivered one at a time to all matched subscribers before
// advancing to the next event. Returns the first error encountered.
func (b *Bus) PublishOrdered(ctx context.Context, events []event.Event) error {
	if b.closed.Load() {
		return errors.ErrBusClosed
	}
	if len(events) == 0 {
		return nil
	}

	for i := range events {
		select {
		case <-ctx.Done():
			return fmt.Errorf("eventbus: ordered publish cancelled at index %d: %w", i, ctx.Err())
		default:
		}

		subs := b.matched(events[i].Key)
		if len(subs) == 0 {
			continue
		}

		for _, sub := range subs {
			if err := b.deliverSync(&events[i], sub); err != nil {
				return fmt.Errorf("eventbus: ordered publish failed at event[%d].Key=%q: %w", i, events[i].Key, err)
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

// Close performs a graceful shutdown:
//  1. Marks the bus as closed (rejects new publishes).
//  2. Cancels the stop context (aborts in-flight retry delays).
//  3. Closes the queue channel (workers drain remaining jobs then exit).
//  4. Waits for all workers to finish.
//
// It is safe to call Close multiple times.
func (b *Bus) Close() {
	if !b.closed.CompareAndSwap(false, true) {
		return // already closed
	}

	// Abort any retry delays in workers.
	b.stopCancel()

	// Close the queue; workers will drain remaining jobs and exit.
	close(b.queue)

	// Wait for every worker goroutine to finish.
	b.wg.Wait()
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// Stats returns a snapshot of delivery statistics.
func (b *Bus) Stats() Stats {
	b.mu.RLock()
	subCount := len(b.catchAll)
	for _, subs := range b.subscribers {
		subCount += len(subs)
	}
	b.mu.RUnlock()

	return Stats{
		Delivered:   b.delivered.Load(),
		Dropped:     b.dropped.Load(),
		Failed:      b.failed.Load(),
		Subscribers: subCount,
		QueueLen:    len(b.queue),
	}
}

// Stats holds a point-in-time snapshot of the bus metrics.
type Stats struct {
	// Delivered is the total number of successful event deliveries.
	Delivered uint64
	// Dropped is the number of events dropped due to a full queue.
	Dropped uint64
	// Failed is the number of events where all retry attempts were exhausted.
	Failed uint64
	// Subscribers is the total number of active subscriptions (including catch-all).
	Subscribers int
	// QueueLen is the current number of jobs waiting in the queue.
	QueueLen int
}

// ---------------------------------------------------------------------------
// Matching
// ---------------------------------------------------------------------------

// matched returns all subscribers that should receive an event with the given key.
// It collects catch-all subscribers and any pattern-specific subscribers whose
// pattern matches the key. The returned slice is a copy safe for concurrent use.
func (b *Bus) matched(key string) []subscription {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []subscription

	// Always include catch-all subscribers.
	result = append(result, b.catchAll...)

	// Check each registered pattern.
	for pat, subs := range b.subscribers {
		if pattern.Match(key, pat) {
			result = append(result, subs...)
		}
	}

	return result
}
