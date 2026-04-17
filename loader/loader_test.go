package loader

import (
	"context"
	"errors"
	"testing"
)

func TestBase_Name(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"my-loader", "my-loader"},
		{"file:/tmp/config.yaml", "file:/tmp/config.yaml"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBase(tt.name, "test", 10)
			if got := b.Name(); got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBase_Type(t *testing.T) {
	b := NewBase("test", "file", 10)
	if got := b.Type(); got != "file" {
		t.Errorf("Type() = %q, want %q", got, "file")
	}
}

func TestBase_Priority(t *testing.T) {
	t.Run("default priority", func(t *testing.T) {
		b := NewBase("test", "test", 42)
		if got := b.Priority(); got != 42 {
			t.Errorf("Priority() = %d, want 42", got)
		}
	})

	t.Run("set priority", func(t *testing.T) {
		b := NewBase("test", "test", 10)
		b.SetPriority(99)
		if got := b.Priority(); got != 99 {
			t.Errorf("Priority() after SetPriority(99) = %d, want 99", got)
		}
	})
}

func TestBase_String(t *testing.T) {
	b := NewBase("my-loader", "test", 10)
	if got := b.String(); got != "my-loader" {
		t.Errorf("String() = %q, want %q", got, "my-loader")
	}
}

func TestBase_WrapErr(t *testing.T) {
	b := NewBase("test-loader", "test", 10)

	t.Run("nil error returns nil", func(t *testing.T) {
		got := b.WrapErr(nil, "op")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("non-nil error is wrapped", func(t *testing.T) {
		original := errors.New("some failure")
		got := b.WrapErr(original, "load")
		if got == nil {
			t.Fatal("expected non-nil wrapped error")
		}
		if got == original {
			t.Error("error should be wrapped, not the same instance")
		}
		// The wrapped error should contain source name and operation info
		errStr := got.Error()
		if errStr == "" {
			t.Error("error string should not be empty")
		}
	})
}

func TestBase_Close(t *testing.T) {
	b := NewBase("test", "test", 10)

	t.Run("close succeeds", func(t *testing.T) {
		err := b.Close(context.Background())
		if err != nil {
			t.Fatalf("unexpected close error: %v", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		b := NewBase("test", "test", 10)
		err1 := b.Close(context.Background())
		err2 := b.Close(context.Background())
		if err1 != nil || err2 != nil {
			t.Fatalf("close errors: %v, %v", err1, err2)
		}
	})

	t.Run("is closed after close", func(t *testing.T) {
		b := NewBase("test", "test", 10)
		if b.IsClosed() {
			t.Error("should not be closed initially")
		}
		b.Close(context.Background())
		if !b.IsClosed() {
			t.Error("should be closed after Close()")
		}
	})
}

func TestBase_Done(t *testing.T) {
	b := NewBase("test", "test", 10)
	done := b.Done()
	if done == nil {
		t.Fatal("Done() returned nil channel")
	}

	select {
	case <-done:
		t.Error("Done() channel should not be closed initially")
	default:
	}

	b.Close(context.Background())

	select {
	case <-done:
		// expected
	default:
		t.Error("Done() channel should be closed after Close()")
	}
}
