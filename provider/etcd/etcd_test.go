package etcd

import (
	"context"
	"testing"
	"time"
)

func TestEtcd_New(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		p, err := New(&Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.cfg.Endpoints) != 1 || p.cfg.Endpoints[0] != "127.0.0.1:2379" {
			t.Errorf("default endpoints = %v, want [127.0.0.1:2379]", p.cfg.Endpoints)
		}
		if p.cfg.Timeout != 5*time.Second {
			t.Errorf("default timeout = %v, want %v", p.cfg.Timeout, 5*time.Second)
		}
	})

	t.Run("with custom values", func(t *testing.T) {
		p, err := New(&Config{
			Endpoints: []string{"10.0.0.1:2379", "10.0.0.2:2379"},
			Timeout:   10 * time.Second,
			Username:  "admin",
			Password:  "secret",
			Prefix:    "/config/",
			Priority:  70,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.cfg.Endpoints) != 2 {
			t.Errorf("endpoints count = %d, want 2", len(p.cfg.Endpoints))
		}
		if p.cfg.Timeout != 10*time.Second {
			t.Errorf("timeout = %v, want %v", p.cfg.Timeout, 10*time.Second)
		}
		if p.cfg.Username != "admin" {
			t.Errorf("username = %q, want %q", p.cfg.Username, "admin")
		}
		if p.cfg.Prefix != "/config/" {
			t.Errorf("prefix = %q, want %q", p.cfg.Prefix, "/config/")
		}
		if p.cfg.Priority != 70 {
			t.Errorf("priority = %d, want 70", p.cfg.Priority)
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

func TestEtcd_Name(t *testing.T) {
	p, err := New(&Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "etcd" {
		t.Errorf("Name() = %q, want %q", p.Name(), "etcd")
	}
}

func TestEtcd_String(t *testing.T) {
	p, err := New(&Config{Endpoints: []string{"10.0.0.1:2379", "10.0.0.2:2379"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "etcd:10.0.0.1:2379,10.0.0.2:2379"
	if p.String() != want {
		t.Errorf("String() = %q, want %q", p.String(), want)
	}
}

func TestEtcd_Close(t *testing.T) {
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

func TestEtcd_EnsureOpen(t *testing.T) {
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
