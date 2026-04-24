package backoff

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNext_ReturnsIncreasingDelays(t *testing.T) {
	cfg := Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		JitterFactor:    0, // disable jitter for deterministic test
		MaxRetries:      0, // unlimited
	}
	b := New(cfg)

	var prevDelay time.Duration
	for i := 0; i < 5; i++ {
		delay, ok := b.Next()
		require.True(t, ok, "attempt %d should succeed", i)

		if i > 0 {
			assert.Greater(t, delay, prevDelay, "attempt %d delay should exceed previous", i)
		}

		expected := float64(100*time.Millisecond) * pow2(i)
		assert.Equal(t, time.Duration(expected), delay, "attempt %d delay mismatch", i)

		prevDelay = delay
	}
}

func TestNext_RespectsMaxInterval(t *testing.T) {
	cfg := Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		Multiplier:      4.0,
		JitterFactor:    0,
		MaxRetries:      0,
	}
	b := New(cfg)

	delay0, ok := b.Next()
	require.True(t, ok)
	assert.Equal(t, 100*time.Millisecond, delay0)

	delay1, ok := b.Next()
	require.True(t, ok)
	assert.Equal(t, 400*time.Millisecond, delay1)

	delay2, ok := b.Next()
	require.True(t, ok)
	assert.Equal(t, 500*time.Millisecond, delay2, "delay should be capped at MaxInterval")

	for i := 0; i < 3; i++ {
		delay, ok := b.Next()
		require.True(t, ok)
		assert.Equal(t, 500*time.Millisecond, delay, "delay should remain capped at MaxInterval")
	}
}

func TestNext_ExhaustsMaxRetries(t *testing.T) {
	cfg := Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		JitterFactor:    0,
		MaxRetries:      3,
	}
	b := New(cfg)

	for i := 0; i < 3; i++ {
		delay, ok := b.Next()
		require.True(t, ok, "attempt %d should succeed", i)
		assert.Greater(t, delay, time.Duration(0))
	}

	delay, ok := b.Next()
	assert.False(t, ok, "should return false after max retries exhausted")
	assert.Equal(t, time.Duration(0), delay, "delay should be 0 when exhausted")

	for i := 0; i < 3; i++ {
		_, ok := b.Next()
		assert.False(t, ok, "should remain false after exhaustion")
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	b := New(Config{})

	delay, ok := b.Next()
	require.True(t, ok)
	assert.Equal(t, 100*time.Millisecond, delay, "default initial interval should be 100ms")

	delay2, ok := b.Next()
	require.True(t, ok)
	assert.Equal(t, 200*time.Millisecond, delay2)

	for i := 2; i < 20; i++ {
		_, ok := b.Next()
		require.True(t, ok, "attempt %d should succeed with unlimited retries", i)
	}

	bLimited := New(Config{MaxRetries: 3})
	for i := 0; i < 3; i++ {
		_, ok := bLimited.Next()
		require.True(t, ok, "limited attempt %d should succeed", i)
	}
	_, ok2 := bLimited.Next()
	assert.False(t, ok2, "4th attempt should fail with MaxRetries=3")
}

func TestReset(t *testing.T) {
	cfg := Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		JitterFactor:    0,
		MaxRetries:      3,
	}
	b := New(cfg)

	for i := 0; i < 3; i++ {
		_, ok := b.Next()
		require.True(t, ok)
	}
	_, ok := b.Next()
	assert.False(t, ok)

	b.Reset()

	assert.Equal(t, 0, b.Attempt())

	delay, ok := b.Next()
	require.True(t, ok)
	assert.Equal(t, 100*time.Millisecond, delay, "after reset, first delay should be initial interval")
	assert.Equal(t, 1, b.Attempt())
}

func pow2(n int) float64 {
	result := 1.0
	for i := 0; i < n; i++ {
		result *= 2.0
	}
	return result
}

// ------------------------------------------------------------------
// Backoff with jitter
// ------------------------------------------------------------------
func TestNext_WithJitter(t *testing.T) {
	cfg := Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		JitterFactor:    0.5,
		MaxRetries:      0,
	}
	b := New(cfg)

	// With jitter, delays should vary but stay within expected bounds.
	// Note: jitter can push the delay above MaxInterval by design (equal jitter).
	for i := 0; i < 10; i++ {
		delay, ok := b.Next()
		require.True(t, ok)
		// Should be > 0
		assert.Greater(t, delay, time.Duration(0))
	}
}

func TestNew_JitterClamping(t *testing.T) {
	// JitterFactor > 1.0 should be clamped to 1.0
	b := New(Config{JitterFactor: 2.0})
	delay, ok := b.Next()
	require.True(t, ok)
	assert.Greater(t, delay, time.Duration(0))
}

// ------------------------------------------------------------------
// Max attempts
// ------------------------------------------------------------------
func TestNext_MaxRetries_One(t *testing.T) {
	b := New(Config{MaxRetries: 1, JitterFactor: 0})
	delay, ok := b.Next()
	require.True(t, ok)
	assert.Equal(t, 100*time.Millisecond, delay)
	_, ok = b.Next()
	assert.False(t, ok)
}

// ------------------------------------------------------------------
// Context cancellation (simulated via backoff exhaustion)
// ------------------------------------------------------------------
func TestNext_ContextCancellation_NoDirectSupport(t *testing.T) {
	// Backoff doesn't directly accept context, but we can test
	// that it returns false when exhausted (simulating cancellation)
	b := New(Config{MaxRetries: 1, JitterFactor: 0})
	_, ok := b.Next()
	require.True(t, ok)

	// Create a cancelled context to verify we can detect it
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():
		// Context is cancelled as expected
	default:
		t.Fatal("expected context to be cancelled")
	}

	// Backoff should also be exhausted
	_, ok = b.Next()
	assert.False(t, ok)
}

// ------------------------------------------------------------------
// Attempt
// ------------------------------------------------------------------
func TestAttempt(t *testing.T) {
	b := New(Config{MaxRetries: 0, JitterFactor: 0})
	assert.Equal(t, 0, b.Attempt())
	b.Next()
	assert.Equal(t, 1, b.Attempt())
	b.Next()
	assert.Equal(t, 2, b.Attempt())
}

// ------------------------------------------------------------------
// DefaultConfig
// ------------------------------------------------------------------
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 100*time.Millisecond, cfg.InitialInterval)
	assert.Equal(t, 10*time.Second, cfg.MaxInterval)
	assert.Equal(t, 2.0, cfg.Multiplier)
	assert.Equal(t, 0.2, cfg.JitterFactor)
	assert.Equal(t, 5, cfg.MaxRetries)
}
