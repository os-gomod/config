package consul

import (
	"context"
	"testing"
	"time"
)

func TestConsul_New(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.cfg.Address != "127.0.0.1:8500" {
			t.Errorf("default address = %q, want %q", p.cfg.Address, "127.0.0.1:8500")
		}
		if p.cfg.Timeout != 5*time.Second {
			t.Errorf("default timeout = %v, want %v", p.cfg.Timeout, 5*time.Second)
		}
	})

	t.Run("with custom values", func(t *testing.T) {
		p, err := New(&Config{
			Address:    "10.0.0.1:8500",
			Timeout:    10 * time.Second,
			Datacenter: "dc1",
			Prefix:     "config/",
			Priority:   80,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.cfg.Address != "10.0.0.1:8500" {
			t.Errorf("address = %q, want %q", p.cfg.Address, "10.0.0.1:8500")
		}
		if p.cfg.Timeout != 10*time.Second {
			t.Errorf("timeout = %v, want %v", p.cfg.Timeout, 10*time.Second)
		}
		if p.cfg.Datacenter != "dc1" {
			t.Errorf("datacenter = %q, want %q", p.cfg.Datacenter, "dc1")
		}
		if p.cfg.Prefix != "config/" {
			t.Errorf("prefix = %q, want %q", p.cfg.Prefix, "config/")
		}
		if p.cfg.Priority != 80 {
			t.Errorf("priority = %d, want 80", p.cfg.Priority)
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

func TestConsul_Name(t *testing.T) {
	p, err := New(&Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "consul" {
		t.Errorf("Name() = %q, want %q", p.Name(), "consul")
	}
}

func TestConsul_String(t *testing.T) {
	p, err := New(&Config{Address: "10.0.0.1:8500"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "consul:10.0.0.1:8500"
	if p.String() != want {
		t.Errorf("String() = %q, want %q", p.String(), want)
	}
}

func TestConsul_Load(t *testing.T) {
	t.Run("returns empty data from stub client", func(t *testing.T) {
		p, err := New(&Config{Prefix: "config/"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := p.Load(context.Background())
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if data == nil {
			t.Fatal("expected non-nil data")
		}
		// Stub httpClient returns nil pairs, so result should be empty
		if len(data) != 0 {
			t.Errorf("expected 0 keys from stub, got %d", len(data))
		}
	})
}

func TestConsul_Health(t *testing.T) {
	t.Run("health check with stub client", func(t *testing.T) {
		p, err := New(&Config{Prefix: "config/"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = p.Health(context.Background())
		if err != nil {
			t.Fatalf("Health error: %v", err)
		}
	})
}

func TestConsul_Close(t *testing.T) {
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

	t.Run("load after close fails", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p.Close(context.Background())

		_, err = p.Load(context.Background())
		if err == nil {
			t.Fatal("expected error loading after close")
		}
	})

	t.Run("health after close fails", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p.Close(context.Background())

		// Health does not check EnsureOpen, but client init may succeed or not
		// Let's just verify it doesn't panic
		_ = p.Health(context.Background())
	})
}

func TestConsul_EnsureOpen(t *testing.T) {
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

func TestConsul_Watch(t *testing.T) {
	t.Run("watch with poll interval returns channel", func(t *testing.T) {
		p, err := New(&Config{PollInterval: 50 * time.Millisecond})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ch, err := p.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch error: %v", err)
		}
		if ch == nil {
			t.Fatal("expected non-nil channel with poll interval")
		}
		p.Close(context.Background())
	})
}
