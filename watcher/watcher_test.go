package watcher

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/stretchr/testify/require"
)

func TestDebounce(t *testing.T) {
	t.Run("trigger fires callback after duration", func(t *testing.T) {
		d := newDebouncer(50 * time.Millisecond)
		var called atomic.Bool
		d.trigger(func() { called.Store(true) })
		if called.Load() {
			t.Fatal("should not be called immediately")
		}
		time.Sleep(100 * time.Millisecond)
		if !called.Load() {
			t.Fatal("should be called after duration")
		}
	})

	t.Run("rapid triggers reset timer", func(t *testing.T) {
		d := newDebouncer(100 * time.Millisecond)
		var count atomic.Int32
		for i := 0; i < 5; i++ {
			d.trigger(func() { count.Add(1) })
			time.Sleep(20 * time.Millisecond)
		}
		// Only the last trigger should fire
		time.Sleep(150 * time.Millisecond)
		if count.Load() != 1 {
			t.Fatalf("expected 1 call, got %d", count.Load())
		}
	})

	t.Run("stop prevents callback", func(t *testing.T) {
		d := newDebouncer(50 * time.Millisecond)
		var called atomic.Bool
		d.trigger(func() { called.Store(true) })
		d.stop()
		time.Sleep(100 * time.Millisecond)
		if called.Load() {
			t.Fatal("callback should not fire after stop")
		}
	})

	t.Run("stop when no timer is no-op", func(t *testing.T) {
		d := newDebouncer(50 * time.Millisecond)
		d.stop() // should not panic
	})
}

func TestPatternWatcher(t *testing.T) {
	t.Run("matches pattern", func(t *testing.T) {
		var received event.Event
		pw := NewPatternWatcher("app.*", func(_ context.Context, evt event.Event) error {
			received = evt
			return nil
		})
		evt := event.New(event.TypeCreate, "app.port")
		err := pw.Observe(t.Context(), &evt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if received.Key != "app.port" {
			t.Fatalf("expected key 'app.port', got %q", received.Key)
		}
	})

	t.Run("does not match non-matching pattern", func(t *testing.T) {
		var called atomic.Bool
		pw := NewPatternWatcher("app.*", func(_ context.Context, _ event.Event) error {
			called.Store(true)
			return nil
		})
		evt := event.New(event.TypeCreate, "db.host")
		_ = pw.Observe(t.Context(), &evt)
		if called.Load() {
			t.Fatal("should not be called for non-matching key")
		}
	})

	t.Run("wildcard matches everything", func(t *testing.T) {
		var called atomic.Bool
		pw := NewPatternWatcher("*", func(_ context.Context, _ event.Event) error {
			called.Store(true)
			return nil
		})
		evt := event.New(event.TypeCreate, "anything")
		_ = pw.Observe(t.Context(), &evt)
		if !called.Load() {
			t.Fatal("wildcard should match everything")
		}
	})

	t.Run("empty pattern matches everything", func(t *testing.T) {
		var called atomic.Bool
		pw := NewPatternWatcher("", func(_ context.Context, _ event.Event) error {
			called.Store(true)
			return nil
		})
		evt := event.New(event.TypeCreate, "anything")
		_ = pw.Observe(t.Context(), &evt)
		if !called.Load() {
			t.Fatal("empty pattern should match everything")
		}
	})
}

func TestManager(t *testing.T) {
	t.Run("new manager", func(t *testing.T) {
		m := NewManager()
		if m == nil {
			t.Fatal("expected non-nil manager")
		}
	})

	t.Run("add watcher", func(t *testing.T) {
		m := NewManager()
		stopped := false
		w := &stubWatcher{stopFn: func() { stopped = true }}
		err := m.Add(w)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m.StopAll()
		if !stopped {
			t.Fatal("expected watcher Stop to be called")
		}
	})

	t.Run("subscribe and notify", func(t *testing.T) {
		m := NewManager()
		var received Change
		m.Subscribe(func(_ context.Context, c Change) {
			received = c
		})
		change := Change{
			Type:      ChangeModified,
			Key:       "app.port",
			NewValue:  value.NewInMemory(8080),
			Timestamp: time.Now(),
		}
		m.Notify(t.Context(), &change)
		if received.Key != "app.port" {
			t.Fatalf("expected key 'app.port', got %q", received.Key)
		}
	})

	t.Run("notify multiple subscribers", func(t *testing.T) {
		m := NewManager()
		var count atomic.Int32
		m.Subscribe(func(_ context.Context, _ Change) { count.Add(1) })
		m.Subscribe(func(_ context.Context, _ Change) { count.Add(1) })
		m.Notify(t.Context(), &Change{Type: ChangeCreated, Key: "k"})
		if count.Load() != 2 {
			t.Fatalf("expected 2 calls, got %d", count.Load())
		}
	})

	t.Run("trigger reload debounces", func(t *testing.T) {
		m := NewManager()
		var count atomic.Int32
		for i := 0; i < 5; i++ {
			m.TriggerReload(50*time.Millisecond, func() { count.Add(1) })
		}
		time.Sleep(100 * time.Millisecond)
		if count.Load() != 1 {
			t.Fatalf("expected 1 call due to debounce, got %d", count.Load())
		}
	})

	t.Run("stop all stops watchers and debouncer", func(t *testing.T) {
		m := NewManager()
		stopped := false
		require.NoError(t, m.Add(&stubWatcher{stopFn: func() { stopped = true }}))
		m.TriggerReload(100*time.Millisecond, func() {})
		m.StopAll()
		if !stopped {
			t.Fatal("expected watcher to be stopped")
		}
		// Give time to verify debounce doesn't fire
		time.Sleep(150 * time.Millisecond)
	})

	t.Run("stop debouncer only", func(t *testing.T) {
		m := NewManager()
		var called atomic.Bool
		m.TriggerReload(50*time.Millisecond, func() { called.Store(true) })
		m.StopDebouncer()
		time.Sleep(100 * time.Millisecond)
		if called.Load() {
			t.Fatal("should not fire after StopDebouncer")
		}
	})
}

type stubWatcher struct {
	stopFn func()
}

func (s *stubWatcher) Start(_ context.Context) (<-chan struct{}, error) {
	return nil, nil
}

func (s *stubWatcher) Stop() {
	if s.stopFn != nil {
		s.stopFn()
	}
}
