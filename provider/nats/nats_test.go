package nats

import (
	"context"
	"testing"
	"time"
)

func TestNats_New(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.cfg.URL != "nats://localhost:4222" {
			t.Errorf("default URL = %q, want %q", p.cfg.URL, "nats://localhost:4222")
		}
		if p.cfg.Timeout != 5*time.Second {
			t.Errorf("default timeout = %v, want %v", p.cfg.Timeout, 5*time.Second)
		}
	})

	t.Run("with custom values", func(t *testing.T) {
		p, err := New(&Config{
			URL:      "nats://10.0.0.1:4222",
			Timeout:  10 * time.Second,
			Bucket:   "config",
			Priority: 60,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.cfg.URL != "nats://10.0.0.1:4222" {
			t.Errorf("URL = %q, want %q", p.cfg.URL, "nats://10.0.0.1:4222")
		}
		if p.cfg.Timeout != 10*time.Second {
			t.Errorf("timeout = %v, want %v", p.cfg.Timeout, 10*time.Second)
		}
		if p.cfg.Bucket != "config" {
			t.Errorf("bucket = %q, want %q", p.cfg.Bucket, "config")
		}
		if p.cfg.Priority != 60 {
			t.Errorf("priority = %d, want 60", p.cfg.Priority)
		}
	})

	t.Run("zero timeout gets default", func(t *testing.T) {
		p, err := New(&Config{Timeout: 0})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.cfg.Timeout != 5*time.Second {
			t.Errorf("timeout = %v, want %v", p.cfg.Timeout, 5*time.Second)
		}
	})

	t.Run("negative timeout gets default", func(t *testing.T) {
		p, err := New(&Config{Timeout: -1 * time.Second})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.cfg.Timeout != 5*time.Second {
			t.Errorf("timeout = %v, want %v", p.cfg.Timeout, 5*time.Second)
		}
	})
}

func TestNats_Name(t *testing.T) {
	p, err := New(&Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "nats" {
		t.Errorf("Name() = %q, want %q", p.Name(), "nats")
	}
}

func TestNats_String(t *testing.T) {
	p, err := New(&Config{URL: "nats://10.0.0.1:4222"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "nats:nats://10.0.0.1:4222"
	if p.String() != want {
		t.Errorf("String() = %q, want %q", p.String(), want)
	}
}

func TestNats_Close(t *testing.T) {
	t.Run("close succeeds", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = p.Close(context.Background())
		if err != nil {
			t.Fatalf("Close error: %v", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err1 := p.Close(context.Background())
		err2 := p.Close(context.Background())
		if err1 != nil || err2 != nil {
			t.Fatalf("close errors: %v, %v", err1, err2)
		}
	})
}

func TestNats_EnsureOpen(t *testing.T) {
	t.Run("not closed allows operations", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = p.EnsureOpen()
		if err != nil {
			t.Errorf("EnsureOpen should succeed: %v", err)
		}
	})

	t.Run("closed returns error", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p.Close(context.Background())

		err = p.EnsureOpen()
		if err == nil {
			t.Error("expected error from EnsureOpen after close")
		}
	})
}
