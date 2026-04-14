package circuit_test

import (
	"testing"
	"time"

	"github.com/os-gomod/config/core/circuit"
)

func TestNewCircuitStartsClosed(t *testing.T) {
	b := circuit.New(circuit.DefaultConfig())
	if b.IsOpen() {
		t.Fatal("new breaker should start closed")
	}
	if got := b.State(); got != "closed" {
		t.Fatalf("expected state closed, got %q", got)
	}
}

func TestRecordFailureOpensAtThreshold(t *testing.T) {
	cfg := circuit.DefaultConfig()
	cfg.Threshold = 3
	b := circuit.New(cfg)

	for i := 0; i < cfg.Threshold-1; i++ {
		b.RecordFailure()
		if b.IsOpen() {
			t.Fatalf(
				"breaker should remain closed after %d failures (threshold=%d)",
				i+1,
				cfg.Threshold,
			)
		}
	}
	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("breaker should be open after reaching threshold")
	}
	if got := b.State(); got != "open" {
		t.Fatalf("expected state open, got %q", got)
	}
}

func TestOpenCircuitReturnsTrueFromIsOpen(t *testing.T) {
	cfg := circuit.DefaultConfig()
	cfg.Threshold = 1
	b := circuit.New(cfg)
	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("breaker should be open after one failure with threshold=1")
	}
}

func TestTimeoutTransitionsToHalfOpen(t *testing.T) {
	cfg := circuit.DefaultConfig()
	cfg.Threshold = 1
	cfg.Timeout = 50 * time.Millisecond
	b := circuit.New(cfg)

	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("breaker should be open")
	}

	time.Sleep(cfg.Timeout + 10*time.Millisecond)
	if b.IsOpen() {
		t.Fatal("breaker should transition to half-open after timeout")
	}
	if got := b.State(); got != "half_open" {
		t.Fatalf("expected state half_open, got %q", got)
	}
}

func TestRecordSuccessInHalfOpenClosesCircuit(t *testing.T) {
	cfg := circuit.DefaultConfig()
	cfg.Threshold = 1
	cfg.Timeout = 50 * time.Millisecond
	cfg.SuccessThreshold = 1
	b := circuit.New(cfg)

	b.RecordFailure()
	time.Sleep(cfg.Timeout + 10*time.Millisecond)
	_ = b.IsOpen() // trigger transition to half-open

	b.RecordSuccess()
	if b.IsOpen() {
		t.Fatal("breaker should be closed after success in half-open")
	}
	if got := b.State(); got != "closed" {
		t.Fatalf("expected state closed, got %q", got)
	}
}

func TestRecordFailureInHalfOpenReopensCircuit(t *testing.T) {
	cfg := circuit.DefaultConfig()
	cfg.Threshold = 1
	cfg.Timeout = 50 * time.Millisecond
	b := circuit.New(cfg)

	b.RecordFailure()
	time.Sleep(cfg.Timeout + 10*time.Millisecond)
	_ = b.IsOpen() // trigger transition to half-open

	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("breaker should re-open on failure in half-open")
	}
	if got := b.State(); got != "open" {
		t.Fatalf("expected state open, got %q", got)
	}
}

func TestStateReturnsCorrectStrings(t *testing.T) {
	tests := []struct {
		name      string
		wantState string
		setup     func(*circuit.Breaker)
		config    circuit.BreakerConfig
	}{
		{
			name:      "closed",
			wantState: "closed",
			setup:     func(_ *circuit.Breaker) {},
			config:    circuit.DefaultConfig(),
		},
		{
			name:      "open",
			wantState: "open",
			setup:     func(b *circuit.Breaker) { b.RecordFailure() },
			config:    circuit.BreakerConfig{Threshold: 1, Timeout: time.Minute},
		},
		{
			name:      "half_open",
			wantState: "half_open",
			setup: func(b *circuit.Breaker) {
				b.RecordFailure()
				time.Sleep(60 * time.Millisecond)
				_ = b.IsOpen()
			},
			config: circuit.BreakerConfig{Threshold: 1, Timeout: 50 * time.Millisecond},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := circuit.New(tt.config)
			tt.setup(b)
			if got := b.State(); got != tt.wantState {
				t.Fatalf("expected state %q, got %q", tt.wantState, got)
			}
		})
	}
}

func TestDefaultConfigValues(t *testing.T) {
	cfg := circuit.DefaultConfig()
	if cfg.Threshold != 5 {
		t.Fatalf("expected default threshold 5, got %d", cfg.Threshold)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("expected default timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.SuccessThreshold != 1 {
		t.Fatalf("expected default success threshold 1, got %d", cfg.SuccessThreshold)
	}
}

func TestZeroConfigUsesDefaults(t *testing.T) {
	b := circuit.New(circuit.BreakerConfig{})
	if b.State() != "closed" {
		t.Fatal("breaker with zero config should start closed")
	}
}
