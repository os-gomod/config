package loader

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoaderRegistry_Register(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		r := NewRegistry()
		err := r.Register("test-loader", func(cfg map[string]any) (Loader, error) {
			return NewMemoryLoader(), nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate registration error", func(t *testing.T) {
		r := NewRegistry()
		factory := func(cfg map[string]any) (Loader, error) {
			return NewMemoryLoader(), nil
		}
		if err := r.Register("dup", factory); err != nil {
			t.Fatalf("first register: %v", err)
		}
		if err := r.Register("dup", factory); err == nil {
			t.Fatal("expected error on duplicate registration")
		}
	})

	t.Run("empty name returns error", func(t *testing.T) {
		r := NewRegistry()
		err := r.Register("", func(cfg map[string]any) (Loader, error) {
			return nil, nil
		})
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("nil factory returns error", func(t *testing.T) {
		r := NewRegistry()
		err := r.Register("no-factory", nil)
		if err == nil {
			t.Fatal("expected error for nil factory")
		}
	})
}

func TestLoaderRegistry_MustRegister(t *testing.T) {
	t.Run("panics on duplicate", func(t *testing.T) {
		// Use the package-level MustRegister which registers on DefaultRegistry
		// We'll test with a unique name to avoid test interference
		defer func() {
			if rec := recover(); rec == nil {
				t.Error("expected panic on duplicate MustRegister")
			}
		}()
		MustRegister("must-reg-test", func(cfg map[string]any) (Loader, error) {
			return NewMemoryLoader(), nil
		})
		MustRegister("must-reg-test", func(cfg map[string]any) (Loader, error) {
			return NewMemoryLoader(), nil
		})
	})
}

func TestLoaderRegistry_Create(t *testing.T) {
	t.Run("create registered loader", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register("memory", func(cfg map[string]any) (Loader, error) {
			return NewMemoryLoader(), nil
		}))

		l, err := r.Create("memory", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l == nil {
			t.Fatal("expected non-nil loader")
		}
	})

	t.Run("create with config", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register("memory", func(cfg map[string]any) (Loader, error) {
			if cfg == nil {
				return NewMemoryLoader(), nil
			}
			data, ok := cfg["data"].(map[string]any)
			if !ok {
				return NewMemoryLoader(), nil
			}
			return NewMemoryLoader(WithMemoryData(data)), nil
		}))

		l, err := r.Create("memory", map[string]any{
			"data": map[string]any{"k": "v"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := l.Load(context.Background())
		if data["k"].String() != "v" {
			t.Errorf("k = %q, want %q", data["k"].String(), "v")
		}
	})

	t.Run("factory error propagated", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register("fail", func(cfg map[string]any) (Loader, error) {
			return nil, errors.New("factory failure")
		}))

		_, err := r.Create("fail", nil)
		if err == nil {
			t.Fatal("expected error from failing factory")
		}
	})

	t.Run("create unregistered loader", func(t *testing.T) {
		r := NewRegistry()
		_, err := r.Create("nonexistent", nil)
		if err == nil {
			t.Fatal("expected error for unregistered loader")
		}
	})
}

func TestLoaderRegistry_Names(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register("charlie", func(cfg map[string]any) (Loader, error) {
		return NewMemoryLoader(), nil
	}))
	require.NoError(t, r.Register("alpha", func(cfg map[string]any) (Loader, error) {
		return NewMemoryLoader(), nil
	}))
	require.NoError(t, r.Register("bravo", func(cfg map[string]any) (Loader, error) {
		return NewMemoryLoader(), nil
	}))

	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %v", len(names), names)
	}
	// Verify sorted
	if names[0] != "alpha" || names[1] != "bravo" || names[2] != "charlie" {
		t.Errorf("names not sorted: %v", names)
	}
}

func TestLoaderRegistry_EmptyNames(t *testing.T) {
	r := NewRegistry()
	names := r.Names()
	if len(names) != 0 {
		t.Errorf("expected empty names, got %v", names)
	}
}

func TestDefaultLoaderRegistry(t *testing.T) {
	r := DefaultRegistry
	if r == nil {
		t.Fatal("DefaultRegistry should not be nil")
	}
	// DefaultRegistry should be usable (empty but not nil)
	names := r.Names()
	if names == nil {
		t.Error("Names() should not return nil")
	}
}
