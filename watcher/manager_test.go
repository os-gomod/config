package watcher

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestManager_TriggerReload(t *testing.T) {
	m := NewManager()
	var called atomic.Bool
	m.TriggerReload(10*time.Millisecond, func() {
		called.Store(true)
	})
	time.Sleep(100 * time.Millisecond)
	if !called.Load() {
		t.Error("expected reload to be triggered")
	}
	m.StopAll()
}

func TestManager_StopAll(t *testing.T) {
	m := NewManager()
	m.StopAll()
	// Should not panic
}

func TestManager_Subscribe(t *testing.T) {
	m := NewManager()
	var received atomic.Pointer[Change]
	m.Subscribe(func(_ context.Context, c Change) {
		received.Store(&c)
	})
	// Trigger a notification
	change := Change{Type: ChangeCreated, Key: "test.key", Timestamp: time.Now()}
	m.Notify(context.Background(), &change)
	time.Sleep(50 * time.Millisecond)
	got := received.Load()
	if got == nil || got.Key != "test.key" {
		t.Errorf("expected key 'test.key', got %v", got)
	}
}

func TestManager_StopDebouncer(t *testing.T) {
	m := NewManager()
	m.StopDebouncer()
	// Should not panic
}

func TestChangeType_String(t *testing.T) {
	tests := []struct {
		c    ChangeType
		want string
	}{
		{ChangeCreated, "created"},
		{ChangeModified, "modified"},
		{ChangeDeleted, "deleted"},
		{ChangeReload, "reload"},
		{ChangeType(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.c.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
