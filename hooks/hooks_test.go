package hooks

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

func TestHookFunc(t *testing.T) {
	var called atomic.Bool
	h := New("test-hook", 10, func(ctx context.Context, hctx *Context) error {
		called.Store(true)
		return nil
	})
	if h.Name() != "test-hook" {
		t.Errorf("expected name 'test-hook', got %q", h.Name())
	}
	if h.Priority() != 10 {
		t.Errorf("expected priority 10, got %d", h.Priority())
	}
	if err := h.Execute(context.Background(), &Context{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called.Load() {
		t.Error("hook should have been called")
	}
}

func TestHookFuncError(t *testing.T) {
	h := New("fail-hook", 5, func(ctx context.Context, hctx *Context) error {
		return context.DeadlineExceeded
	})
	err := h.Execute(context.Background(), &Context{})
	if err == nil {
		t.Fatal("expected error from hook")
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestManagerRegisterAndHas(t *testing.T) {
	m := NewManager()
	h := New("hook1", 10, func(ctx context.Context, hctx *Context) error { return nil })

	if m.Has(event.HookBeforeReload) {
		t.Error("should not have hooks before registering")
	}

	m.Register(event.HookBeforeReload, h)
	if !m.Has(event.HookBeforeReload) {
		t.Error("should have hook after registering")
	}
}

func TestManagerExecute(t *testing.T) {
	m := NewManager()
	var order []int
	var mu sync.Mutex

	m.Register(event.HookAfterReload, New("first", 1, func(ctx context.Context, hctx *Context) error {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
		return nil
	}))
	m.Register(event.HookAfterReload, New("second", 2, func(ctx context.Context, hctx *Context) error {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
		return nil
	}))

	err := m.Execute(context.Background(), event.HookAfterReload, &Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 {
		t.Errorf("expected order [1, 2], got %v", order)
	}
}

func TestManagerExecutePriorityOrdering(t *testing.T) {
	m := NewManager()
	var order []int
	var mu sync.Mutex

	m.Register(event.HookBeforeSet, New("low", 100, func(ctx context.Context, hctx *Context) error {
		mu.Lock()
		order = append(order, 100)
		mu.Unlock()
		return nil
	}))
	m.Register(event.HookBeforeSet, New("high", 5, func(ctx context.Context, hctx *Context) error {
		mu.Lock()
		order = append(order, 5)
		mu.Unlock()
		return nil
	}))
	m.Register(event.HookBeforeSet, New("mid", 50, func(ctx context.Context, hctx *Context) error {
		mu.Lock()
		order = append(order, 50)
		mu.Unlock()
		return nil
	}))

	m.Execute(context.Background(), event.HookBeforeSet, &Context{})

	if len(order) != 3 || order[0] != 5 || order[1] != 50 || order[2] != 100 {
		t.Errorf("expected priority order [5, 50, 100], got %v", order)
	}
}

func TestManagerExecuteStopsOnError(t *testing.T) {
	m := NewManager()
	var secondCalled atomic.Bool

	m.Register(event.HookAfterSet, New("first", 1, func(ctx context.Context, hctx *Context) error {
		return context.Canceled
	}))
	m.Register(event.HookAfterSet, New("second", 2, func(ctx context.Context, hctx *Context) error {
		secondCalled.Store(true)
		return nil
	}))

	err := m.Execute(context.Background(), event.HookAfterSet, &Context{})
	if err == nil {
		t.Fatal("expected error")
	}
	if secondCalled.Load() {
		t.Error("second hook should not have been called after error")
	}
}

func TestManagerClear(t *testing.T) {
	m := NewManager()
	m.Register(event.HookBeforeDelete, New("hook1", 1, func(ctx context.Context, hctx *Context) error { return nil }))
	if !m.Has(event.HookBeforeDelete) {
		t.Error("should have hook")
	}
	m.Clear()
	if m.Has(event.HookBeforeDelete) {
		t.Error("should not have hook after clear")
	}
}

func TestManagerCount(t *testing.T) {
	m := NewManager()
	if m.Count(event.HookAfterValidate) != 0 {
		t.Error("expected count 0")
	}
	m.Register(event.HookAfterValidate, New("h1", 1, func(ctx context.Context, hctx *Context) error { return nil }))
	m.Register(event.HookAfterValidate, New("h2", 2, func(ctx context.Context, hctx *Context) error { return nil }))
	if m.Count(event.HookAfterValidate) != 2 {
		t.Errorf("expected count 2, got %d", m.Count(event.HookAfterValidate))
	}
}

func TestContextOldStateSafe(t *testing.T) {
	c := &Context{OldState: nil}
	state := c.OldStateSafe()
	if state == nil {
		t.Fatal("OldStateSafe should return non-nil even when OldState is nil")
	}
}

func TestContextOldStateSafeWithValue(t *testing.T) {
	s := value.NewState(map[string]value.Value{"k": value.New("v", value.TypeString, value.SourceMemory, 0)}, 1)
	c := &Context{OldState: s}
	state := c.OldStateSafe()
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if !state.Has("k") {
		t.Error("state should have key 'k'")
	}
}

func TestContextNewStateSafe(t *testing.T) {
	c := &Context{NewState: nil}
	state := c.NewStateSafe()
	if state == nil {
		t.Fatal("NewStateSafe should return non-nil even when NewState is nil")
	}
}

func TestContextNewStateSafeWithValue(t *testing.T) {
	s := value.NewState(map[string]value.Value{"k": value.New("v", value.TypeString, value.SourceMemory, 0)}, 1)
	c := &Context{NewState: s}
	state := c.NewStateSafe()
	if state == nil {
		t.Fatal("expected non-nil state")
	}
}

func TestManagerSetRecorder(t *testing.T) {
	m := NewManager()
	m.SetRecorder(nil) // should not panic
	// Nop recorder is set by default, this is fine
}
