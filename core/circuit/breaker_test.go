package circuit

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := DefaultConfig()
		if cfg.Threshold != 5 {
			t.Fatalf("expected threshold 5, got %d", cfg.Threshold)
		}
		if cfg.Timeout != 30*time.Second {
			t.Fatalf("expected timeout 30s, got %v", cfg.Timeout)
		}
		if cfg.SuccessThreshold != 1 {
			t.Fatalf("expected success threshold 1, got %d", cfg.SuccessThreshold)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		b := New(BreakerConfig{
			Threshold:        3,
			Timeout:          100 * time.Millisecond,
			SuccessThreshold: 2,
		})
		if b.threshold != 3 {
			t.Fatalf("expected threshold 3, got %d", b.threshold)
		}
		if b.successThreshold != 2 {
			t.Fatalf("expected success threshold 2, got %d", b.successThreshold)
		}
	})

	t.Run("zero values get defaults", func(t *testing.T) {
		b := New(BreakerConfig{})
		if b.threshold != 5 {
			t.Fatalf("expected default threshold 5, got %d", b.threshold)
		}
		if b.timeout != 30*time.Second {
			t.Fatalf("expected default timeout 30s, got %v", b.timeout)
		}
		if b.successThreshold != 1 {
			t.Fatalf("expected default success threshold 1, got %d", b.successThreshold)
		}
	})
}

func TestBreaker_ClosedState(t *testing.T) {
	b := New(BreakerConfig{
		Threshold:        5,
		Timeout:          100 * time.Millisecond,
		SuccessThreshold: 1,
	})

	if b.IsOpen() {
		t.Fatal("new breaker should not be open")
	}
	if b.State() != "closed" {
		t.Fatalf("expected state 'closed', got %q", b.State())
	}

	// Record success should keep closed
	b.RecordSuccess()
	if b.IsOpen() {
		t.Fatal("should remain closed after success")
	}
	if b.State() != "closed" {
		t.Fatalf("expected state 'closed', got %q", b.State())
	}
}

func TestBreaker_OpenAfterThreshold(t *testing.T) {
	cfg := BreakerConfig{
		Threshold:        3,
		Timeout:          200 * time.Millisecond,
		SuccessThreshold: 1,
	}
	b := New(cfg)

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		b.RecordFailure()
		if b.IsOpen() && i < 2 {
			t.Fatalf("should not be open before threshold at failure %d", i+1)
		}
	}
	if !b.IsOpen() {
		t.Fatal("expected breaker to be open after threshold failures")
	}
	if b.State() != "open" {
		t.Fatalf("expected state 'open', got %q", b.State())
	}
}

func TestBreaker_HalfOpenAfterTimeout(t *testing.T) {
	b := New(BreakerConfig{
		Threshold:        2,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 1,
	})

	// Open the breaker
	b.RecordFailure()
	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("expected open")
	}

	// Wait for timeout
	time.Sleep(80 * time.Millisecond)

	// IsOpen should transition to half-open
	if b.IsOpen() {
		t.Fatal("expected half-open (IsOpen returns false)")
	}
	// State should be half_open
	if b.State() != "half_open" {
		t.Fatalf("expected state 'half_open', got %q", b.State())
	}
}

func TestBreaker_CloseAfterHalfOpenSuccess(t *testing.T) {
	b := New(BreakerConfig{
		Threshold:        2,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 1,
	})

	b.RecordFailure()
	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("expected open")
	}

	time.Sleep(80 * time.Millisecond)
	// Now in half-open
	if b.IsOpen() {
		t.Fatal("expected half-open")
	}

	// Record success should close the breaker
	b.RecordSuccess()
	if b.IsOpen() {
		t.Fatal("expected closed after half-open success")
	}
	if b.State() != "closed" {
		t.Fatalf("expected state 'closed', got %q", b.State())
	}
}

func TestBreaker_ReopenAfterHalfOpenFailure(t *testing.T) {
	b := New(BreakerConfig{
		Threshold:        2,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 1,
	})

	b.RecordFailure()
	b.RecordFailure()

	time.Sleep(80 * time.Millisecond)

	// Trigger transition to half-open via IsOpen()
	_ = b.IsOpen()
	if b.State() != "half_open" {
		t.Fatalf("expected half_open before failure, got %q", b.State())
	}
	// Failure should re-open
	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("expected re-open after half-open failure")
	}
	if b.State() != "open" {
		t.Fatalf("expected state 'open', got %q", b.State())
	}
}

func TestBreaker_SuccessThreshold(t *testing.T) {
	b := New(BreakerConfig{
		Threshold:        2,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 3,
	})

	b.RecordFailure()
	b.RecordFailure()

	time.Sleep(80 * time.Millisecond)

	// Trigger transition to half-open via IsOpen()
	_ = b.IsOpen()
	// Need 3 consecutive successes in half-open
	b.RecordSuccess()
	if b.State() != "half_open" {
		t.Fatalf("expected half_open after 1 success, got %q", b.State())
	}
	b.RecordSuccess()
	if b.State() != "half_open" {
		t.Fatalf("expected half_open after 2 successes, got %q", b.State())
	}
	b.RecordSuccess()
	if b.State() != "closed" {
		t.Fatalf("expected closed after 3 successes, got %q", b.State())
	}
}

func TestBreaker_FailuresResetOnClose(t *testing.T) {
	b := New(BreakerConfig{
		Threshold:        5,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 1,
	})

	// Record some failures
	b.RecordFailure()
	b.RecordFailure()

	// Record success should reset failure count
	b.RecordSuccess()

	// Need 5 failures again to open
	for i := 0; i < 4; i++ {
		b.RecordFailure()
	}
	if b.IsOpen() {
		t.Fatal("should not be open after 4 + 2 failures (with reset)")
	}
	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("should be open after 5 failures post-reset")
	}
}

func TestBreaker_StateString(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"closed", "closed"},
		{"open", "open"},
		{"half_open", "half_open"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(BreakerConfig{
				Threshold:        1,
				Timeout:          50 * time.Millisecond,
				SuccessThreshold: 1,
			})

			switch tt.state {
			case "open":
				b.RecordFailure()
				if b.State() != "open" {
					t.Fatal("failed to reach open state")
				}
			case "half_open":
				b.RecordFailure()
				time.Sleep(80 * time.Millisecond)
				_ = b.IsOpen() // trigger transition
				if b.State() != "half_open" {
					t.Fatal("failed to reach half_open state")
				}
			}
		})
	}
}
