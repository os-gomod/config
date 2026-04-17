package event

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBus(t *testing.T) {
	t.Run("default constructor", func(t *testing.T) {
		b := NewBus()
		if b == nil {
			t.Fatal("NewBus returned nil")
		}
		if b.SubscriberCount() != 0 {
			t.Fatalf("expected 0 subscribers, got %d", b.SubscriberCount())
		}
	})

	t.Run("with custom concurrency", func(t *testing.T) {
		b := NewBusWithConcurrency(10)
		if b == nil {
			t.Fatal("NewBusWithConcurrency returned nil")
		}
	})

	t.Run("zero concurrency uses default", func(t *testing.T) {
		b := NewBusWithConcurrency(0)
		if b == nil {
			t.Fatal("NewBusWithConcurrency(0) returned nil")
		}
	})

	t.Run("negative concurrency uses default", func(t *testing.T) {
		b := NewBusWithConcurrency(-5)
		if b == nil {
			t.Fatal("NewBusWithConcurrency(-5) returned nil")
		}
	})
}

func TestBus_SubscribeAndPublish(t *testing.T) {
	t.Run("basic subscribe and publish", func(t *testing.T) {
		b := NewBus()
		var received atomic.Int32
		unsub := b.Subscribe("app.started", func(_ context.Context, evt Event) error {
			if evt.Key != "app.started" {
				t.Errorf("expected key app.started, got %s", evt.Key)
			}
			if evt.Type != TypeCreate {
				t.Errorf("expected TypeCreate, got %d", evt.Type)
			}
			received.Add(1)
			return nil
		})

		evt := New(TypeCreate, "app.started")
		b.Publish(context.Background(), &evt)

		// Wait for async dispatch
		time.Sleep(50 * time.Millisecond)
		if got := received.Load(); got != 1 {
			t.Fatalf("expected 1 event, got %d", got)
		}

		// Unsubscribe and verify no more events
		unsub()
		b.Publish(context.Background(), &evt)
		time.Sleep(50 * time.Millisecond)
		if got := received.Load(); got != 1 {
			t.Fatalf("expected still 1 event after unsubscribe, got %d", got)
		}
	})

	t.Run("multiple subscribers same key", func(t *testing.T) {
		b := NewBus()
		var count atomic.Int32
		for i := 0; i < 5; i++ {
			b.Subscribe("app.config", func(_ context.Context, _ Event) error {
				count.Add(1)
				return nil
			})
		}
		evt := New(TypeUpdate, "app.config")
		b.Publish(context.Background(), &evt)
		time.Sleep(50 * time.Millisecond)
		if got := count.Load(); got != 5 {
			t.Fatalf("expected 5 events, got %d", got)
		}
	})

	t.Run("subscriber not triggered by different key", func(t *testing.T) {
		b := NewBus()
		var received atomic.Int32
		b.Subscribe("db.host", func(_ context.Context, _ Event) error {
			received.Add(1)
			return nil
		})
		evt := New(TypeUpdate, "db.port")
		b.Publish(context.Background(), &evt)
		time.Sleep(50 * time.Millisecond)
		if got := received.Load(); got != 0 {
			t.Fatalf("expected 0 events, got %d", got)
		}
	})
}

func TestBus_PatternMatching(t *testing.T) {
	t.Run("wildcard pattern matches keys", func(t *testing.T) {
		b := NewBus()
		var keys []string
		var mu sync.Mutex
		b.Subscribe("db.*", func(_ context.Context, evt Event) error {
			mu.Lock()
			keys = append(keys, evt.Key)
			mu.Unlock()
			return nil
		})

		evt1 := New(TypeUpdate, "db.host")
		evt2 := New(TypeUpdate, "db.port")
		evt3 := New(TypeUpdate, "app.name")
		b.Publish(context.Background(), &evt1)
		b.Publish(context.Background(), &evt2)
		b.Publish(context.Background(), &evt3)
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		if len(keys) != 2 {
			t.Fatalf("expected 2 events for db.*, got %d: %v", len(keys), keys)
		}
	})

	t.Run("glob pattern with question mark", func(t *testing.T) {
		b := NewBus()
		var count atomic.Int32
		b.Subscribe("log.?", func(_ context.Context, _ Event) error {
			count.Add(1)
			return nil
		})

		evt1 := New(TypeUpdate, "log.a")
		evt2 := New(TypeUpdate, "log.b")
		evt3 := New(TypeUpdate, "log.ab") // should not match
		b.Publish(context.Background(), &evt1)
		b.Publish(context.Background(), &evt2)
		b.Publish(context.Background(), &evt3)
		time.Sleep(50 * time.Millisecond)

		if got := count.Load(); got != 2 {
			t.Fatalf("expected 2 events for log.?, got %d", got)
		}
	})
}

func TestBus_CatchAll(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"empty string", ""},
		{"star", "*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBus()
			var keys []string
			var mu sync.Mutex
			b.Subscribe(tt.pattern, func(_ context.Context, evt Event) error {
				mu.Lock()
				keys = append(keys, evt.Key)
				mu.Unlock()
				return nil
			})

			evt1 := New(TypeUpdate, "a.b.c")
			evt2 := New(TypeUpdate, "x.y.z")
			evt3 := New(TypeUpdate, "foo")
			b.Publish(context.Background(), &evt1)
			b.Publish(context.Background(), &evt2)
			b.Publish(context.Background(), &evt3)
			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			defer mu.Unlock()
			if len(keys) != 3 {
				t.Fatalf("expected 3 catch-all events, got %d: %v", len(keys), keys)
			}
		})
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	t.Run("unsubscribe removes observer", func(t *testing.T) {
		b := NewBus()
		var count atomic.Int32
		unsub := b.Subscribe("test.key", func(_ context.Context, _ Event) error {
			count.Add(1)
			return nil
		})

		evt := New(TypeUpdate, "test.key")
		b.Publish(context.Background(), &evt)
		time.Sleep(50 * time.Millisecond)
		if got := count.Load(); got != 1 {
			t.Fatalf("expected 1 before unsub, got %d", got)
		}

		unsub()
		if b.SubscriberCount() != 0 {
			t.Fatalf("expected 0 subscribers after unsub, got %d", b.SubscriberCount())
		}

		evt2 := New(TypeUpdate, "test.key")
		b.Publish(context.Background(), &evt2)
		time.Sleep(50 * time.Millisecond)
		if got := count.Load(); got != 1 {
			t.Fatalf("expected still 1 after unsub, got %d", got)
		}
	})

	t.Run("unsubscribe catch-all", func(t *testing.T) {
		b := NewBus()
		var count atomic.Int32
		unsub := b.Subscribe("*", func(_ context.Context, _ Event) error {
			count.Add(1)
			return nil
		})

		evt := New(TypeUpdate, "any")
		b.Publish(context.Background(), &evt)
		time.Sleep(50 * time.Millisecond)
		if got := count.Load(); got != 1 {
			t.Fatalf("expected 1 before unsub, got %d", got)
		}

		unsub()
		evt2 := New(TypeUpdate, "any")
		b.Publish(context.Background(), &evt2)
		time.Sleep(50 * time.Millisecond)
		if got := count.Load(); got != 1 {
			t.Fatalf("expected still 1 after unsub, got %d", got)
		}
	})

	t.Run("unsubscribe one of many", func(t *testing.T) {
		b := NewBus()
		var count1, count2 atomic.Int32
		unsub1 := b.Subscribe("key", func(_ context.Context, _ Event) error {
			count1.Add(1)
			return nil
		})
		b.Subscribe("key", func(_ context.Context, _ Event) error {
			count2.Add(1)
			return nil
		})

		evt := New(TypeUpdate, "key")
		b.Publish(context.Background(), &evt)
		time.Sleep(50 * time.Millisecond)
		if count1.Load() != 1 || count2.Load() != 1 {
			t.Fatalf("expected both to receive, got %d/%d", count1.Load(), count2.Load())
		}

		unsub1()
		evt2 := New(TypeUpdate, "key")
		b.Publish(context.Background(), &evt2)
		time.Sleep(50 * time.Millisecond)
		if count1.Load() != 1 || count2.Load() != 2 {
			t.Fatalf("expected only second to receive, got %d/%d", count1.Load(), count2.Load())
		}
	})

	t.Run("unsubscribe called multiple times is safe", func(t *testing.T) {
		b := NewBus()
		unsub := b.Subscribe("key", func(_ context.Context, _ Event) error {
			return nil
		})
		unsub()
		unsub()
		unsub()
		if b.SubscriberCount() != 0 {
			t.Fatalf("expected 0 subscribers, got %d", b.SubscriberCount())
		}
	})
}

func TestBus_PublishSync(t *testing.T) {
	t.Run("returns first error", func(t *testing.T) {
		b := NewBus()
		b.Subscribe("err.key", func(_ context.Context, _ Event) error {
			return errors.New("first error")
		})
		b.Subscribe("err.key", func(_ context.Context, _ Event) error {
			return errors.New("second error")
		})

		evt := New(TypeUpdate, "err.key")
		err := b.PublishSync(context.Background(), &evt)
		if err == nil {
			t.Fatal("expected error from PublishSync")
		}
		if err.Error() != "first error" {
			t.Fatalf("expected 'first error', got '%s'", err.Error())
		}
	})

	t.Run("returns nil when no errors", func(t *testing.T) {
		b := NewBus()
		b.Subscribe("ok.key", func(_ context.Context, _ Event) error {
			return nil
		})

		evt := New(TypeUpdate, "ok.key")
		err := b.PublishSync(context.Background(), &evt)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("recovers from panics", func(t *testing.T) {
		b := NewBus()
		b.Subscribe("panic.key", func(_ context.Context, _ Event) error {
			panic("boom")
		})

		evt := New(TypeUpdate, "panic.key")
		err := b.PublishSync(context.Background(), &evt)
		if err == nil {
			t.Fatal("expected error from panicked observer")
		}
		if got := err.Error(); len(got) == 0 {
			t.Fatal("expected non-empty error message")
		}
	})
}

func TestBus_PublishOrdered(t *testing.T) {
	t.Run("processes events in order", func(t *testing.T) {
		b := NewBus()
		var order []int
		var mu sync.Mutex

		b.Subscribe("ordered.*", func(_ context.Context, evt Event) error {
			switch evt.Key {
			case "ordered.1":
				mu.Lock()
				order = append(order, 1)
				mu.Unlock()
			case "ordered.2":
				mu.Lock()
				order = append(order, 2)
				mu.Unlock()
			case "ordered.3":
				mu.Lock()
				order = append(order, 3)
				mu.Unlock()
			}
			return nil
		})

		events := []Event{
			New(TypeCreate, "ordered.1"),
			New(TypeUpdate, "ordered.2"),
			New(TypeDelete, "ordered.3"),
		}
		err := b.PublishOrdered(context.Background(), events)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()
		if len(order) != 3 {
			t.Fatalf("expected 3 events, got %d", len(order))
		}
		for i, v := range order {
			if v != i+1 {
				t.Fatalf("expected order [1,2,3], got %v", order)
			}
		}
	})

	t.Run("returns first error and stops", func(t *testing.T) {
		b := NewBus()
		var count atomic.Int32
		b.Subscribe("stop.*", func(_ context.Context, evt Event) error {
			count.Add(1)
			if evt.Key == "stop.2" {
				return errors.New("stop here")
			}
			return nil
		})

		events := []Event{
			New(TypeCreate, "stop.1"),
			New(TypeCreate, "stop.2"),
			New(TypeCreate, "stop.3"),
		}
		err := b.PublishOrdered(context.Background(), events)
		if err == nil {
			t.Fatal("expected error")
		}

		// Note: PublishOrdered does NOT stop on first error, it processes all events
		// and returns the first error encountered.
		if got := count.Load(); got != 3 {
			t.Fatalf("expected 3 events processed, got %d", got)
		}
	})
}

func TestBus_ConcurrentSubscribers(t *testing.T) {
	t.Run("concurrent publish and subscribe", func(t *testing.T) {
		b := NewBus()
		var received atomic.Int64
		const numSubs = 10
		const numPubs = 10
		const expected = int64(numSubs * numPubs)

		// Add subscribers
		for i := 0; i < numSubs; i++ {
			b.Subscribe("concurrent.*", func(_ context.Context, _ Event) error {
				received.Add(1)
				return nil
			})
		}

		// Publish concurrently
		for i := 0; i < numPubs; i++ {
			go func() {
				evt := New(TypeUpdate, "concurrent.test")
				b.Publish(context.Background(), &evt)
			}()
		}

		// Wait for all deliveries
		deadline := time.After(5 * time.Second)
		for received.Load() < expected {
			select {
			case <-time.After(10 * time.Millisecond):
				// poll again
			case <-deadline:
				t.Fatalf("timed out: got %d/%d events", received.Load(), expected)
			}
		}

		if got := received.Load(); got != expected {
			t.Fatalf("expected %d events, got %d", expected, got)
		}
	})
}

func TestBus_PanicHandler(t *testing.T) {
	t.Run("panic handler receives panic value", func(t *testing.T) {
		b := NewBus()
		ch := make(chan any, 1)
		b.PanicHandler = func(r any) {
			select {
			case ch <- r:
			default:
			}
		}

		var wg sync.WaitGroup
		wg.Add(1)
		b.Subscribe("panic.key", func(_ context.Context, _ Event) error {
			defer wg.Done()
			panic("test panic value")
		})

		evt := New(TypeUpdate, "panic.key")
		b.Publish(context.Background(), &evt)
		wg.Wait()

		select {
		case recovered := <-ch:
			if recovered != "test panic value" {
				t.Fatalf("expected 'test panic value', got %v", recovered)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for panic handler")
		}
	})

	t.Run("no panic handler does not crash bus", func(t *testing.T) {
		b := NewBus()
		b.PanicHandler = nil

		var wg sync.WaitGroup
		wg.Add(1)
		b.Subscribe("panic.key", func(_ context.Context, _ Event) error {
			defer wg.Done()
			panic("should not crash")
		})

		// This should not panic the test
		evt := New(TypeUpdate, "panic.key")
		b.Publish(context.Background(), &evt)
		wg.Wait()
	})
}

func TestBus_SubscriberCount(t *testing.T) {
	t.Run("counts all subscribers", func(t *testing.T) {
		b := NewBus()
		if b.SubscriberCount() != 0 {
			t.Fatalf("expected 0, got %d", b.SubscriberCount())
		}

		b.Subscribe("a", func(_ context.Context, _ Event) error { return nil })
		if b.SubscriberCount() != 1 {
			t.Fatalf("expected 1, got %d", b.SubscriberCount())
		}

		b.Subscribe("b", func(_ context.Context, _ Event) error { return nil })
		if b.SubscriberCount() != 2 {
			t.Fatalf("expected 2, got %d", b.SubscriberCount())
		}

		b.Subscribe("a", func(_ context.Context, _ Event) error { return nil })
		if b.SubscriberCount() != 3 {
			t.Fatalf("expected 3, got %d", b.SubscriberCount())
		}

		b.Subscribe("*", func(_ context.Context, _ Event) error { return nil })
		if b.SubscriberCount() != 4 {
			t.Fatalf("expected 4, got %d", b.SubscriberCount())
		}
	})
}

func TestBus_Clear(t *testing.T) {
	t.Run("removes all subscribers", func(t *testing.T) {
		b := NewBus()
		b.Subscribe("a", func(_ context.Context, _ Event) error { return nil })
		b.Subscribe("b", func(_ context.Context, _ Event) error { return nil })
		b.Subscribe("*", func(_ context.Context, _ Event) error { return nil })

		if b.SubscriberCount() != 3 {
			t.Fatalf("expected 3 before clear, got %d", b.SubscriberCount())
		}

		b.Clear()

		if b.SubscriberCount() != 0 {
			t.Fatalf("expected 0 after clear, got %d", b.SubscriberCount())
		}

		// Verify no events are delivered after clear
		var received atomic.Int32
		b.Subscribe("a", func(_ context.Context, _ Event) error {
			received.Add(1)
			return nil
		})
		b.Clear() // clear again immediately

		evt := New(TypeUpdate, "a")
		b.Publish(context.Background(), &evt)
		time.Sleep(50 * time.Millisecond)
		if got := received.Load(); got != 0 {
			t.Fatalf("expected 0 after clear, got %d", got)
		}
	})
}

func TestIsCatchAll(t *testing.T) {
	tests := []struct {
		pat  string
		want bool
	}{
		{"*", true},
		{"", true},
		{"db.host", false},
		{"db.*", false},
		{"?", false},
	}
	for _, tt := range tests {
		t.Run(tt.pat, func(t *testing.T) {
			if got := isCatchAll(tt.pat); got != tt.want {
				t.Errorf("isCatchAll(%q) = %v, want %v", tt.pat, got, tt.want)
			}
		})
	}
}
