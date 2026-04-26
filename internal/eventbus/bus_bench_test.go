package eventbus

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/pattern"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// noopObserver returns an observer that does nothing (fast path).
func noopObserver() event.Observer {
	return func(ctx context.Context, evt event.Event) error {
		return nil
	}
}

// countingObserver returns an observer that atomically increments a counter.
// Useful for verifying delivery without blocking.
func countingObserver(counter *atomic.Int64) event.Observer {
	return func(ctx context.Context, evt event.Event) error {
		counter.Add(1)
		return nil
	}
}

// benchEvent creates a test event for benchmarking.
func benchEvent(key string) *event.Event {
	return &event.Event{
		EventType: event.TypeUpdate,
		Key:       key,
		NewValue:  value.New("benchmark-value"),
		Timestamp: time.Now().UTC(),
		Source:    "bench",
	}
}

// newTestBus creates a bus optimised for benchmarking:
// large queue to avoid drops, no retries, no panic handler noise.
func newTestBus(workerCount, queueSize int) *Bus {
	return NewBus(
		WithWorkerCount(workerCount),
		WithQueueSize(queueSize),
		WithRetryCount(0),
		WithPanicHandler(nil), // suppress panic output during benchmarks
	)
}

// subscriberCounts is the set of subscriber counts to benchmark.
var subscriberCounts = []int{1, 10, 100, 1000}

// ---------------------------------------------------------------------------
// BenchmarkPublish — async publish throughput with varying subscribers
// ---------------------------------------------------------------------------

func BenchmarkPublish(b *testing.B) {
	b.ReportAllocs()

	for _, subs := range subscriberCounts {
		bus := newTestBus(32, 8192)

		// Subscribe subs catch-all observers.
		for i := 0; i < subs; i++ {
			bus.Subscribe("*", noopObserver())
		}

		evt := benchEvent("bench.key")

		b.Run(fmt.Sprintf("subscribers_%d", subs), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = bus.Publish(context.Background(), evt)
			}
		})

		bus.Close()
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPublishSync — synchronous publish with varying subscribers
// ---------------------------------------------------------------------------

func BenchmarkPublishSync(b *testing.B) {
	b.ReportAllocs()

	for _, subs := range subscriberCounts {
		bus := newTestBus(32, 8192)

		for i := 0; i < subs; i++ {
			bus.Subscribe("*", noopObserver())
		}

		evt := benchEvent("bench.key")

		b.Run(fmt.Sprintf("subscribers_%d", subs), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = bus.PublishSync(context.Background(), evt)
			}
		})

		bus.Close()
	}
}

// ---------------------------------------------------------------------------
// BenchmarkSubscribe — registration throughput
// ---------------------------------------------------------------------------

func BenchmarkSubscribe(b *testing.B) {
	b.ReportAllocs()

	for _, subs := range subscriberCounts {
		bus := newTestBus(4, 256)
		var unsubscribers []func()

		b.Run(fmt.Sprintf("register_%d", subs), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				unsub := bus.Subscribe("bench.topic", noopObserver())
				unsubscribers = append(unsubscribers, unsub)
			}
		})

		// Clean up to avoid leaks.
		for _, unsub := range unsubscribers {
			unsub()
		}
		bus.Close()
	}

	// Benchmark unsubscribe performance.
	for _, subs := range []int{10, 100} {
		bus := newTestBus(4, 256)
		var unsubscribers []func()
		for i := 0; i < subs; i++ {
			unsub := bus.Subscribe("bench.topic", noopObserver())
			unsubscribers = append(unsubscribers, unsub)
		}

		b.Run(fmt.Sprintf("unsubscribe_%d", subs), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				// Re-register and immediately unsubscribe in the loop
				// to measure the overhead of both operations.
				unsub := bus.Subscribe("bench.topic", noopObserver())
				unsub()
			}
		})

		for _, unsub := range unsubscribers {
			unsub()
		}
		bus.Close()
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPatternMatch — pattern matching throughput
// ---------------------------------------------------------------------------

func BenchmarkPatternMatch(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name    string
		key     string
		pattern string
	}{
		{"exact_match", "database.host", "database.host"},
		{"wildcard_catchall", "any.key.here", "*"},
		{"wildcard_prefix", "database.host", "database.*"},
		{"wildcard_middle", "app.db.config", "app.*.config"},
		{"wildcard_suffix", "config.changed", "*.changed"},
		{"no_match", "logging.level", "database.*"},
		{"deep_key_match", "a.b.c.d.e.f", "a.b.c.d.e.f"},
		{"deep_wildcard", "a.b.c.d.e.f", "a.*.c.*.e.f"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = pattern.Match(tc.key, tc.pattern)
			}
		})
	}
}

// BenchmarkPatternMatchWithBus measures pattern matching through the bus's
// matched() method which iterates over all registered subscriptions.
func BenchmarkPatternMatchWithBus(b *testing.B) {
	b.ReportAllocs()

	patternCounts := []int{10, 100, 1000}
	for _, pats := range patternCounts {
		bus := newTestBus(4, 256)

		// Register diverse patterns.
		for i := 0; i < pats; i++ {
			switch i % 4 {
			case 0:
				bus.Subscribe(fmt.Sprintf("prefix.%d.*", i), noopObserver())
			case 1:
				bus.Subscribe(fmt.Sprintf("exact.key.%d", i), noopObserver())
			case 2:
				bus.Subscribe(fmt.Sprintf("a.*.b.%d", i), noopObserver())
			case 3:
				bus.Subscribe("*", noopObserver())
			}
		}

		evt := benchEvent("prefix.42.something")

		b.Run(fmt.Sprintf("patterns_%d", pats), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = bus.Publish(context.Background(), evt)
			}
		})

		bus.Close()
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPublishWithObserverWork — measures throughput when observers do work
// ---------------------------------------------------------------------------

func BenchmarkPublishWithObserverWork(b *testing.B) {
	b.ReportAllocs()

	for _, subs := range []int{1, 10, 100} {
		bus := newTestBus(32, 8192)
		var counter atomic.Int64

		for i := 0; i < subs; i++ {
			bus.Subscribe("*", countingObserver(&counter))
		}

		evt := benchEvent("work.key")

		b.Run(fmt.Sprintf("subscribers_%d", subs), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = bus.Publish(context.Background(), evt)
			}
		})

		// Wait for workers to drain so counter is consistent.
		bus.Close()
	}
}

// ---------------------------------------------------------------------------
// BenchmarkBusStats — overhead of polling Stats()
// ---------------------------------------------------------------------------

func BenchmarkBusStats(b *testing.B) {
	b.ReportAllocs()

	bus := newTestBus(4, 256)
	for i := 0; i < 100; i++ {
		bus.Subscribe("bench.topic", noopObserver())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bus.Stats()
	}

	bus.Close()
}
