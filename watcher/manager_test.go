package watcher_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/watcher"
)

func TestTriggerReloadCalledOnce(t *testing.T) {
	m := watcher.NewManager()
	var calls atomic.Int32

	m.TriggerReload(20*time.Millisecond, func() {
		calls.Add(1)
	})

	time.Sleep(50 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 call, got %d", got)
	}
}

func TestTriggerReloadCoalescesRapidTriggers(t *testing.T) {
	m := watcher.NewManager()
	var calls atomic.Int32

	// Fire 10 rapid triggers — they should coalesce into 1 call.
	for i := 0; i < 10; i++ {
		m.TriggerReload(20*time.Millisecond, func() {
			calls.Add(1)
		})
		time.Sleep(2 * time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 coalesced call, got %d", got)
	}
}

func TestSubscribeCallback(t *testing.T) {
	m := watcher.NewManager()
	var received atomic.Int32

	m.Subscribe(func(_ context.Context, change watcher.Change) {
		received.Add(1)
	})

	m.Notify(context.Background(), watcher.Change{
		Type:      watcher.ChangeReload,
		Timestamp: time.Now(),
	})

	if got := received.Load(); got != 1 {
		t.Errorf("expected 1 callback, got %d", got)
	}
}

func TestStopAllStopsWatchers(t *testing.T) {
	m := watcher.NewManager()
	var received atomic.Int32

	m.Subscribe(func(_ context.Context, _ watcher.Change) {
		received.Add(1)
	})

	// Notify before stop.
	m.Notify(context.Background(), watcher.Change{Type: watcher.ChangeReload})
	if got := received.Load(); got != 1 {
		t.Errorf("expected 1 before stop, got %d", got)
	}

	// StopAll stops watchers and debouncer. Should not panic.
	m.StopAll()
}

func TestStopDebouncerCancelsPending(t *testing.T) {
	m := watcher.NewManager()
	var calls atomic.Int32

	m.TriggerReload(50*time.Millisecond, func() {
		calls.Add(1)
	})

	// Stop the debouncer before the timer fires.
	m.StopDebouncer()
	time.Sleep(100 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Errorf("expected 0 calls after debouncer stop, got %d", got)
	}
}
