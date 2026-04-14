package event_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

func TestBusSubscribeAndPublishSync(t *testing.T) {
	bus := event.NewBus()
	var received atomic.Int32

	bus.Subscribe("db.*", func(_ context.Context, evt event.Event) error {
		if evt.Key == "db.host" {
			received.Add(1)
		}
		return nil
	})

	evt := event.New(event.TypeCreate, "db.host", event.WithSource(value.SourceFile))
	if err := bus.PublishSync(context.Background(), evt); err != nil {
		t.Fatalf("PublishSync error: %v", err)
	}
	if got := received.Load(); got != 1 {
		t.Errorf("expected 1 received, got %d", got)
	}
}

func TestBusPatternMatching(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		key      string
		expected bool
	}{
		{"wildcard matches all", "*", "any.key", true},
		{"empty pattern matches all", "", "any.key", true},
		{"glob match", "db.*", "db.host", true},
		{"glob no match", "db.*", "app.name", false},
		{"exact match", "db.host", "db.host", true},
		{"exact no match", "db.host", "db.port", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := event.NewBus()
			var count atomic.Int32
			bus.Subscribe(tt.pattern, func(_ context.Context, _ event.Event) error {
				count.Add(1)
				return nil
			})
			_ = bus.PublishSync(context.Background(), event.New(event.TypeCreate, tt.key))
			got := count.Load() == 1
			if got != tt.expected {
				t.Errorf(
					"pattern %q key %q: expected match=%v, got match=%v",
					tt.pattern,
					tt.key,
					tt.expected,
					got,
				)
			}
		})
	}
}

func TestBusWildcardReceivesAllEvents(t *testing.T) {
	bus := event.NewBus()
	var count atomic.Int32
	bus.Subscribe("*", func(_ context.Context, _ event.Event) error {
		count.Add(1)
		return nil
	})
	_ = bus.PublishSync(context.Background(), event.New(event.TypeCreate, "db.host"))
	_ = bus.PublishSync(context.Background(), event.New(event.TypeCreate, "app.name"))
	_ = bus.PublishSync(context.Background(), event.New(event.TypeUpdate, "cache.ttl"))
	if got := count.Load(); got != 3 {
		t.Errorf("expected wildcard to receive 3 events, got %d", got)
	}
}

func TestBusUnsubscribe(t *testing.T) {
	bus := event.NewBus()
	var count atomic.Int32
	unsub := bus.Subscribe("db.*", func(_ context.Context, _ event.Event) error {
		count.Add(1)
		return nil
	})
	_ = bus.PublishSync(context.Background(), event.New(event.TypeCreate, "db.host"))
	if got := count.Load(); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
	unsub()
	_ = bus.PublishSync(context.Background(), event.New(event.TypeCreate, "db.host"))
	if got := count.Load(); got != 1 {
		t.Errorf("expected still 1 after unsubscribe, got %d", got)
	}
}

func TestBusPublishOrdered(t *testing.T) {
	bus := event.NewBus()
	var order []string
	bus.Subscribe("*", func(_ context.Context, evt event.Event) error {
		order = append(order, evt.Key)
		return nil
	})

	events := []event.Event{
		event.New(event.TypeCreate, "a"),
		event.New(event.TypeCreate, "b"),
		event.New(event.TypeCreate, "c"),
	}
	if err := bus.PublishOrdered(context.Background(), events); err != nil {
		t.Fatalf("PublishOrdered error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 events, got %d", len(order))
	}
	for i, want := range []string{"a", "b", "c"} {
		if order[i] != want {
			t.Errorf("event %d: expected key %q, got %q", i, want, order[i])
		}
	}
}

func TestBusPanicRecovery(t *testing.T) {
	bus := event.NewBus()
	var panicHandled atomic.Int32
	bus.PanicHandler = func(_ any) {
		panicHandled.Add(1)
	}
	bus.Subscribe("*", func(_ context.Context, _ event.Event) error {
		panic("observer panic")
	})

	// Publish (async) should recover and call PanicHandler.
	bus.Publish(context.Background(), event.New(event.TypeCreate, "test"))
	// Give the goroutine time to execute.
	time.Sleep(50 * time.Millisecond)
	if got := panicHandled.Load(); got != 1 {
		t.Errorf("expected panic handler called once, got %d", got)
	}

	// PublishSync should recover and return error without calling PanicHandler.
	err := bus.PublishSync(context.Background(), event.New(event.TypeCreate, "test2"))
	if err == nil {
		t.Error("expected error from panicked observer in PublishSync")
	}
}

func TestBusSubscriberCount(t *testing.T) {
	bus := event.NewBus()
	if got := bus.SubscriberCount(); got != 0 {
		t.Errorf("expected 0 subscribers, got %d", got)
	}
	unsub1 := bus.Subscribe("*", func(_ context.Context, _ event.Event) error { return nil })
	bus.Subscribe("db.*", func(_ context.Context, _ event.Event) error { return nil })
	if got := bus.SubscriberCount(); got != 2 {
		t.Errorf("expected 2 subscribers, got %d", got)
	}
	unsub1()
	if got := bus.SubscriberCount(); got != 1 {
		t.Errorf("expected 1 after unsubscribe, got %d", got)
	}
}
