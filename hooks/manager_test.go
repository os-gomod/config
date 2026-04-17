package hooks

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/event"
)

func TestHooksManager_New(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.Has(event.HookBeforeReload) {
		t.Fatal("new manager should have no hooks")
	}
}

func TestHooksManager_Register(t *testing.T) {
	t.Run("register and check has", func(t *testing.T) {
		m := NewManager()
		m.Register(event.HookBeforeReload, New("test", 10, func(_ context.Context, _ *Context) error {
			return nil
		}))
		if !m.Has(event.HookBeforeReload) {
			t.Fatal("expected hook to be registered")
		}
	})

	t.Run("register multiple hooks", func(t *testing.T) {
		m := NewManager()
		m.Register(event.HookBeforeReload, New("a", 10, nil))
		m.Register(event.HookBeforeReload, New("b", 20, nil))
		if m.Count(event.HookBeforeReload) != 2 {
			t.Fatalf("expected 2 hooks, got %d", m.Count(event.HookBeforeReload))
		}
	})

	t.Run("different hook types", func(t *testing.T) {
		m := NewManager()
		m.Register(event.HookBeforeSet, New("before-set", 1, nil))
		m.Register(event.HookAfterSet, New("after-set", 1, nil))
		if !m.Has(event.HookBeforeSet) {
			t.Fatal("expected before-set hook")
		}
		if !m.Has(event.HookAfterSet) {
			t.Fatal("expected after-set hook")
		}
	})
}

func TestHooksManager_Execute(t *testing.T) {
	t.Run("execute hooks in priority order", func(t *testing.T) {
		m := NewManager()
		var order []string
		m.Register(event.HookBeforeReload, New("high", 100, func(_ context.Context, _ *Context) error {
			order = append(order, "high")
			return nil
		}))
		m.Register(event.HookBeforeReload, New("low", 1, func(_ context.Context, _ *Context) error {
			order = append(order, "low")
			return nil
		}))
		m.Register(event.HookBeforeReload, New("mid", 50, func(_ context.Context, _ *Context) error {
			order = append(order, "mid")
			return nil
		}))
		err := m.Execute(t.Context(), event.HookBeforeReload, &Context{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(order) != 3 || order[0] != "low" || order[1] != "mid" || order[2] != "high" {
			t.Fatalf("expected [low mid high], got %v", order)
		}
	})

	t.Run("execute stops on first error", func(t *testing.T) {
		m := NewManager()
		var called atomic.Bool
		m.Register(event.HookBeforeReload, New("fail", 1, func(_ context.Context, _ *Context) error {
			return errors.New("hook failed")
		}))
		m.Register(event.HookBeforeReload, New("skip", 10, func(_ context.Context, _ *Context) error {
			called.Store(true)
			return nil
		}))
		err := m.Execute(t.Context(), event.HookBeforeReload, &Context{})
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "hook failed" {
			t.Fatalf("unexpected error: %v", err)
		}
		if called.Load() {
			t.Fatal("second hook should not be called")
		}
	})

	t.Run("execute with no hooks is no-op", func(t *testing.T) {
		m := NewManager()
		err := m.Execute(t.Context(), event.HookAfterClose, &Context{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("execute receives context", func(t *testing.T) {
		m := NewManager()
		var receivedOp string
		m.Register(event.HookBeforeSet, New("check", 1, func(_ context.Context, hctx *Context) error {
			receivedOp = hctx.Operation
			return nil
		}))
		_ = m.Execute(t.Context(), event.HookBeforeSet, &Context{Operation: "set"})
		if receivedOp != "set" {
			t.Fatalf("expected operation 'set', got %q", receivedOp)
		}
	})
}

func TestHooksManager_Has(t *testing.T) {
	m := NewManager()
	if m.Has(event.HookBeforeReload) {
		t.Fatal("expected no hooks")
	}
	m.Register(event.HookBeforeReload, New("test", 1, nil))
	if !m.Has(event.HookBeforeReload) {
		t.Fatal("expected hook to be registered")
	}
	if m.Has(event.HookAfterReload) {
		t.Fatal("expected no after-reload hook")
	}
}

func TestHooksManager_Count(t *testing.T) {
	m := NewManager()
	if m.Count(event.HookBeforeReload) != 0 {
		t.Fatal("expected 0 count")
	}
	m.Register(event.HookBeforeReload, New("a", 1, nil))
	m.Register(event.HookBeforeReload, New("b", 2, nil))
	if m.Count(event.HookBeforeReload) != 2 {
		t.Fatalf("expected 2, got %d", m.Count(event.HookBeforeReload))
	}
}

func TestHooksManager_Clear(t *testing.T) {
	m := NewManager()
	m.Register(event.HookBeforeReload, New("a", 1, nil))
	m.Register(event.HookAfterReload, New("b", 1, nil))
	m.Clear()
	if m.Has(event.HookBeforeReload) || m.Has(event.HookAfterReload) {
		t.Fatal("expected all hooks to be cleared")
	}
}

func TestHooksManager_SetRecorder(t *testing.T) {
	m := NewManager()
	// SetRecorder with nil should keep Nop recorder
	m.SetRecorder(nil)
	// Should not panic
	_ = m.Execute(t.Context(), event.HookBeforeReload, &Context{})
}

func TestHookFunc(t *testing.T) {
	t.Run("name and priority", func(t *testing.T) {
		h := New("test-hook", 42, nil)
		if h.Name() != "test-hook" {
			t.Fatalf("expected name 'test-hook', got %q", h.Name())
		}
		if h.Priority() != 42 {
			t.Fatalf("expected priority 42, got %d", h.Priority())
		}
	})

	t.Run("execute calls function", func(t *testing.T) {
		var called atomic.Bool
		h := New("test", 1, func(_ context.Context, _ *Context) error {
			called.Store(true)
			return nil
		})
		err := h.Execute(t.Context(), &Context{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called.Load() {
			t.Fatal("expected hook to be called")
		}
	})
}

func TestContext_SafeStates(t *testing.T) {
	t.Run("OldStateSafe returns empty state for nil", func(t *testing.T) {
		c := &Context{}
		s := c.OldStateSafe()
		if s == nil {
			t.Fatal("expected non-nil state")
		}
	})

	t.Run("NewStateSafe returns empty state for nil", func(t *testing.T) {
		c := &Context{}
		s := c.NewStateSafe()
		if s == nil {
			t.Fatal("expected non-nil state")
		}
	})
}

func TestAllHookTypes(t *testing.T) {
	hookTypes := []event.HookType{
		event.HookBeforeReload,
		event.HookAfterReload,
		event.HookBeforeSet,
		event.HookAfterSet,
		event.HookBeforeDelete,
		event.HookAfterDelete,
		event.HookBeforeValidate,
		event.HookAfterValidate,
		event.HookBeforeClose,
		event.HookAfterClose,
	}
	for _, ht := range hookTypes {
		t.Run(ht.String(), func(t *testing.T) {
			m := NewManager()
			called := false
			m.Register(ht, New("test", 1, func(_ context.Context, _ *Context) error {
				called = true
				return nil
			}))
			if !m.Has(ht) {
				t.Fatalf("expected Has(%s) = true", ht)
			}
			_ = m.Execute(t.Context(), ht, &Context{StartTime: time.Now()})
			if !called {
				t.Fatal("expected hook to be called")
			}
		})
	}
}
