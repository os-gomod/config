package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderRegistry_Register(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		r := NewRegistry()
		err := r.Register("test-provider", func(cfg map[string]any) (Provider, error) {
			return nil, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate registration error", func(t *testing.T) {
		r := NewRegistry()
		factory := func(cfg map[string]any) (Provider, error) {
			return nil, nil
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
		err := r.Register("", func(cfg map[string]any) (Provider, error) {
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

func TestProviderRegistry_MustRegister(t *testing.T) {
	t.Run("panics on duplicate", func(t *testing.T) {
		defer func() {
			if rec := recover(); rec == nil {
				t.Error("expected panic on duplicate MustRegister")
			}
		}()
		MustRegister("must-reg-test", func(cfg map[string]any) (Provider, error) {
			return nil, nil
		})
		MustRegister("must-reg-test", func(cfg map[string]any) (Provider, error) {
			return nil, nil
		})
	})
}

func TestProviderRegistry_Create(t *testing.T) {
	t.Run("create registered provider", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register("stub", func(cfg map[string]any) (Provider, error) {
			return NewBaseProvider(64), nil
		}))

		p, err := r.Create("stub", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p == nil {
			t.Fatal("expected non-nil provider")
		}
	})

	t.Run("factory error propagated", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register("fail", func(cfg map[string]any) (Provider, error) {
			return nil, errors.New("factory failure")
		}))

		_, err := r.Create("fail", nil)
		if err == nil {
			t.Fatal("expected error from failing factory")
		}
	})

	t.Run("create unregistered provider", func(t *testing.T) {
		r := NewRegistry()
		_, err := r.Create("nonexistent", nil)
		if err == nil {
			t.Fatal("expected error for unregistered provider")
		}
	})
}

func TestProviderRegistry_Names(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register("charlie", func(cfg map[string]any) (Provider, error) {
		return nil, nil
	}))
	require.NoError(t, r.Register("alpha", func(cfg map[string]any) (Provider, error) {
		return nil, nil
	}))
	require.NoError(t, r.Register("bravo", func(cfg map[string]any) (Provider, error) {
		return nil, nil
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

func TestProviderRegistry_EmptyNames(t *testing.T) {
	r := NewRegistry()
	names := r.Names()
	if len(names) != 0 {
		t.Errorf("expected empty names, got %v", names)
	}
}

func TestDefaultProviderRegistry(t *testing.T) {
	r := DefaultRegistry
	if r == nil {
		t.Fatal("DefaultRegistry should not be nil")
	}
	names := r.Names()
	if names == nil {
		t.Error("Names() should not return nil")
	}
}

func TestBaseProvider_EnsureOpen(t *testing.T) {
	t.Run("not closed returns nil", func(t *testing.T) {
		bp := NewBaseProvider(64)
		if err := bp.EnsureOpen(); err != nil {
			t.Errorf("EnsureOpen should succeed: %v", err)
		}
	})

	t.Run("closed returns error", func(t *testing.T) {
		bp := NewBaseProvider(64)
		require.NoError(t, bp.CloseProvider(context.Background()))
		if err := bp.EnsureOpen(); err == nil {
			t.Error("expected error from EnsureOpen after close")
		}
	})
}

func TestBaseProvider_CloseProvider(t *testing.T) {
	t.Run("close is idempotent", func(t *testing.T) {
		bp := NewBaseProvider(64)
		err1 := bp.CloseProvider(context.Background())
		err2 := bp.CloseProvider(context.Background())
		if err1 != nil || err2 != nil {
			t.Fatalf("close errors: %v, %v", err1, err2)
		}
	})
}

func TestBaseProvider_NewWithZeroBufferSize(t *testing.T) {
	bp := NewBaseProvider(0)
	if bp == nil {
		t.Fatal("expected non-nil BaseProvider")
	}
}

func TestBaseProvider_NewWithNegativeBufferSize(t *testing.T) {
	bp := NewBaseProvider(-10)
	if bp == nil {
		t.Fatal("expected non-nil BaseProvider")
	}
}
