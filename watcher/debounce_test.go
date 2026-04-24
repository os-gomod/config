package watcher

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_Trigger(t *testing.T) {
	d := newDebouncer(50 * time.Millisecond)
	var count atomic.Int32
	d.trigger(func() { count.Add(1) })
	d.trigger(func() { count.Add(1) })
	d.trigger(func() { count.Add(1) })
	time.Sleep(150 * time.Millisecond)
	if count.Load() != 1 {
		t.Errorf("expected 1 call after debounce, got %d", count.Load())
	}
}

func TestDebouncer_Stop(t *testing.T) {
	d := newDebouncer(50 * time.Millisecond)
	d.stop()
	// Should not panic
}

func TestDebouncer_NoTimer(t *testing.T) {
	d := newDebouncer(50 * time.Millisecond)
	// Calling stop before any trigger should not panic
	d.stop()
}
