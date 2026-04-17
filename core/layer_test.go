package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/os-gomod/config/core/circuit"
	"github.com/os-gomod/config/core/value"
)

func TestNewLayer(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		l := NewLayer("test")
		if l.Name() != "test" {
			t.Fatalf("expected name 'test', got %q", l.Name())
		}
		if l.Priority() != 10 {
			t.Fatalf("expected default priority 10, got %d", l.Priority())
		}
		if !l.IsEnabled() {
			t.Fatal("expected layer to be enabled by default")
		}
		if l.CircuitBreaker() == nil {
			t.Fatal("expected circuit breaker")
		}
	})

	t.Run("with custom priority", func(t *testing.T) {
		l := NewLayer("test", WithLayerPriority(42))
		if l.Priority() != 42 {
			t.Fatalf("expected priority 42, got %d", l.Priority())
		}
	})

	t.Run("with custom timeout", func(t *testing.T) {
		l := NewLayer("test", WithLayerTimeout(5*time.Second))
		if l.timeout != 5*time.Second {
			t.Fatalf("expected timeout 5s, got %v", l.timeout)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		l := NewLayer("test", WithLayerEnabled(false))
		if l.IsEnabled() {
			t.Fatal("expected layer to be disabled")
		}
	})

	t.Run("with source", func(t *testing.T) {
		src := &stubLoadable{data: map[string]value.Value{"k": value.NewInMemory("v")}}
		l := NewLayer("test", WithLayerSource(src))
		if l.source != src {
			t.Fatal("expected source to be set")
		}
	})

	t.Run("with custom circuit breaker", func(t *testing.T) {
		cfg := circuit.BreakerConfig{
			Threshold:        3,
			Timeout:          100 * time.Millisecond,
			SuccessThreshold: 2,
		}
		l := NewLayer("test", WithLayerCircuitBreaker(cfg))
		cb := l.CircuitBreaker()
		if cb == nil {
			t.Fatal("expected circuit breaker")
		}
	})
}

func TestLayer_EnableDisable(t *testing.T) {
	l := NewLayer("test", WithLayerEnabled(false))
	if l.IsEnabled() {
		t.Fatal("expected disabled")
	}
	l.Enable()
	if !l.IsEnabled() {
		t.Fatal("expected enabled after Enable()")
	}
	l.Disable()
	if l.IsEnabled() {
		t.Fatal("expected disabled after Disable()")
	}
}

func TestLayer_Load(t *testing.T) {
	t.Run("load with nil source returns empty map", func(t *testing.T) {
		l := NewLayer("test")
		data, err := l.Load(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 0 {
			t.Fatalf("expected empty map, got %d keys", len(data))
		}
	})

	t.Run("load from source", func(t *testing.T) {
		src := &stubLoadable{
			data: map[string]value.Value{
				"key": value.NewInMemory("value"),
			},
		}
		l := NewLayer("test", WithLayerSource(src))
		data, err := l.Load(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 1 {
			t.Fatalf("expected 1 key, got %d", len(data))
		}
	})

	t.Run("load with source error returns last good data", func(t *testing.T) {
		src := &stubLoadable{
			data: map[string]value.Value{"key": value.NewInMemory("value")},
		}
		l := NewLayer("test", WithLayerSource(src))
		// First successful load
		_, err := l.Load(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Now fail
		src.err = fmt.Errorf("load failed")
		data, err := l.Load(t.Context())
		if err == nil {
			t.Fatal("expected error")
		}
		if len(data) != 1 {
			t.Fatalf("expected 1 key from last good data, got %d", len(data))
		}
	})

	t.Run("load returns copy of data", func(t *testing.T) {
		src := &stubLoadable{
			data: map[string]value.Value{"key": value.NewInMemory("value")},
		}
		l := NewLayer("test", WithLayerSource(src))
		data1, _ := l.Load(t.Context())
		data2, _ := l.Load(t.Context())
		data1["other"] = value.NewInMemory("injected")
		if _, ok := data2["other"]; ok {
			t.Fatal("Load should return a copy")
		}
	})

	t.Run("circuit breaker opens after threshold failures", func(t *testing.T) {
		cfg := circuit.BreakerConfig{
			Threshold:        3,
			Timeout:          200 * time.Millisecond,
			SuccessThreshold: 1,
		}
		src := &stubLoadable{err: fmt.Errorf("fail")}
		l := NewLayer("test", WithLayerSource(src), WithLayerCircuitBreaker(cfg))

		for i := 0; i < 3; i++ {
			_, _ = l.Load(t.Context())
		}

		if !l.CircuitBreaker().IsOpen() {
			t.Fatal("expected circuit breaker to be open after threshold failures")
		}
	})
}

func TestLayer_LastData(t *testing.T) {
	t.Run("returns empty map when no successful load", func(t *testing.T) {
		l := NewLayer("test")
		data := l.LastData()
		if len(data) != 0 {
			t.Fatalf("expected empty map, got %d keys", len(data))
		}
	})

	t.Run("returns copy of last good data", func(t *testing.T) {
		src := &stubLoadable{
			data: map[string]value.Value{"k": value.NewInMemory("v")},
		}
		l := NewLayer("test", WithLayerSource(src))
		_, _ = l.Load(t.Context())
		data := l.LastData()
		data["injected"] = value.NewInMemory("x")
		data2 := l.LastData()
		if _, ok := data2["injected"]; ok {
			t.Fatal("LastData should return a copy")
		}
	})
}

func TestLayer_HealthStatus(t *testing.T) {
	t.Run("healthy after successful load", func(t *testing.T) {
		src := &stubLoadable{
			data: map[string]value.Value{"k": value.NewInMemory("v")},
		}
		l := NewLayer("test", WithLayerSource(src))
		_, _ = l.Load(t.Context())
		hs := l.HealthStatus()
		if !hs.Healthy {
			t.Fatal("expected healthy")
		}
		if hs.ConsecutiveFails != 0 {
			t.Fatalf("expected 0 consecutive fails, got %d", hs.ConsecutiveFails)
		}
	})

	t.Run("unhealthy after failure", func(t *testing.T) {
		src := &stubLoadable{err: fmt.Errorf("fail")}
		l := NewLayer("test", WithLayerSource(src))
		_, _ = l.Load(t.Context())
		hs := l.HealthStatus()
		if hs.Healthy {
			t.Fatal("expected unhealthy")
		}
		if hs.ConsecutiveFails != 1 {
			t.Fatalf("expected 1 consecutive fail, got %d", hs.ConsecutiveFails)
		}
		if hs.LastError == nil {
			t.Fatal("expected last error")
		}
		if hs.LastFailureTime.IsZero() {
			t.Fatal("expected non-zero last failure time")
		}
	})
}

func TestLayer_Close(t *testing.T) {
	t.Run("close with nil source is no-op", func(t *testing.T) {
		l := NewLayer("test")
		err := l.Close(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("close calls source close", func(t *testing.T) {
		closed := false
		src := &closingLoadable{closeFn: func() error { closed = true; return nil }}
		l := NewLayer("test", WithLayerSource(src))
		_ = l.Close(t.Context())
		if !closed {
			t.Fatal("expected source Close to be called")
		}
	})
}

func TestLayer_IsHealthy(t *testing.T) {
	t.Run("healthy when circuit not open", func(t *testing.T) {
		l := NewLayer("test")
		if !l.IsHealthy() {
			t.Fatal("expected healthy")
		}
	})
}

type closingLoadable struct {
	stubLoadable
	closeFn func() error
}

func (c *closingLoadable) Close(_ context.Context) error {
	if c.closeFn != nil {
		return c.closeFn()
	}
	return nil
}
