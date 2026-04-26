// Package backoff provides exponential backoff with jitter for retry logic.
// It is used by watchers, loaders, and providers to implement resilient
// retry strategies with configurable parameters.
package backoff

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Backoff implements exponential backoff with optional jitter.
// It is safe for concurrent use.
type Backoff struct {
	Initial   time.Duration // Initial delay (default: 100ms).
	Max       time.Duration // Maximum delay cap (default: 30s).
	Factor    float64       // Multiplier per step (default: 2.0).
	Jitter    bool          // Add random jitter to prevent thundering herd (default: true).
	JitterMin float64       // Jitter minimum factor (default: 0.5).
	JitterMax float64       // Jitter maximum factor (default: 1.5).

	mu       sync.Mutex
	attempt  int
	lastTime time.Time
}

// Option configures a Backoff during construction.
type Option func(*Backoff)

// WithInitial sets the initial backoff delay.
func WithInitial(d time.Duration) Option {
	return func(b *Backoff) {
		if d > 0 {
			b.Initial = d
		}
	}
}

// WithMax sets the maximum backoff delay.
func WithMax(d time.Duration) Option {
	return func(b *Backoff) {
		if d > 0 {
			b.Max = d
		}
	}
}

// WithFactor sets the exponential multiplier.
func WithFactor(f float64) Option {
	return func(b *Backoff) {
		if f > 1.0 {
			b.Factor = f
		}
	}
}

// WithJitter enables or disables jitter.
func WithJitter(enabled bool) Option {
	return func(b *Backoff) {
		b.Jitter = enabled
	}
}

// WithJitterRange sets the jitter range factors.
func WithJitterRange(minVal, maxVal float64) Option {
	return func(b *Backoff) {
		if minVal >= 0 {
			b.JitterMin = minVal
		}
		if maxVal > 0 {
			b.JitterMax = maxVal
		}
	}
}

// New creates a Backoff with the given options applied over defaults.
// Defaults: Initial=100ms, Max=30s, Factor=2.0, Jitter=true.
func New(opts ...Option) *Backoff {
	b := &Backoff{
		Initial:   100 * time.Millisecond,
		Max:       30 * time.Second,
		Factor:    2.0,
		Jitter:    true,
		JitterMin: 0.5,
		JitterMax: 1.5,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Next calculates and returns the next backoff delay.
// Each call increments the attempt counter, resulting in exponential growth.
// If jitter is enabled, a random factor is applied to the delay.
func (b *Backoff) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	delay := b.calculateDelay(b.attempt)
	b.attempt++
	b.lastTime = time.Now()

	return delay
}

// NextWithContext waits for the backoff duration, respecting context cancellation.
// Returns an error if the context is cancelled before the wait completes.
func (b *Backoff) NextWithContext(ctx context.Context) error {
	delay := b.Next()

	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Reset resets the backoff to its initial state.
// This should be called when an operation succeeds to reset the retry counter.
func (b *Backoff) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.attempt = 0
	b.lastTime = time.Time{}
}

// Attempt returns the current attempt count (0-indexed before Next() is called).
func (b *Backoff) Attempt() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.attempt
}

// LastDuration returns the duration of the last computed backoff.
func (b *Backoff) LastDuration() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.attempt == 0 {
		return 0
	}
	return b.calculateDelay(b.attempt - 1)
}

// String returns a human-readable representation of the backoff state.
func (b *Backoff) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return fmt.Sprintf("Backoff{attempt=%d, initial=%s, max=%s, factor=%.1f, jitter=%v}",
		b.attempt, b.Initial, b.Max, b.Factor, b.Jitter)
}

// calculateDelay computes the backoff delay for a given attempt number.
// The formula is: min(initial * factor^attempt, max) * jitter_factor.
func (b *Backoff) calculateDelay(attempt int) time.Duration {
	// Exponential growth: initial * factor^attempt
	multiplier := math.Pow(b.Factor, float64(attempt))
	delay := time.Duration(float64(b.Initial) * multiplier)

	// Cap at maximum.
	if delay > b.Max {
		delay = b.Max
	}

	// Ensure minimum duration.
	if delay < time.Millisecond {
		delay = time.Millisecond
	}

	// Apply jitter.
	if b.Jitter {
		delay = b.applyJitter(delay)
	}

	return delay
}

// applyJitter adds a random factor to the delay.
func (b *Backoff) applyJitter(d time.Duration) time.Duration {
	// Calculate jitter range.
	minFactor := b.JitterMin
	maxFactor := b.JitterMax

	if minFactor >= maxFactor {
		minFactor = 0.5
		maxFactor = 1.5
	}

	// Generate random factor in [minFactor, maxFactor].
	jitterFactor := minFactor + rand.Float64()*(maxFactor-minFactor)
	return time.Duration(float64(d) * jitterFactor)
}

// Stopper provides a channel that closes after the maximum backoff
// duration is reached, indicating that retries should stop.
type Stopper struct {
	backoff *Backoff
	max     int
}

// NewStopper creates a Stopper that signals after maxVal attempts.
func NewStopper(b *Backoff, maxVal int) *Stopper {
	return &Stopper{
		backoff: b,
		max:     maxVal,
	}
}

// ShouldStop returns true if the maximum attempt count has been reached.
func (s *Stopper) ShouldStop() bool {
	return s.backoff.Attempt() >= s.max
}

// MaxAttempts returns the configured maximum attempt count.
func (s *Stopper) MaxAttempts() int {
	return s.max
}

// Remaining returns the number of remaining attempts.
func (s *Stopper) Remaining() int {
	remaining := s.max - s.backoff.Attempt()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ConstantBackoff returns a fixed-duration backoff that always returns
// the same delay. This is useful for cases where exponential backoff
// is not desired (e.g., rate-limited APIs).
func ConstantBackoff(d time.Duration) *Backoff {
	return New(
		WithInitial(d),
		WithMax(d),
		WithFactor(1.0),
		WithJitter(false),
	)
}

// NoBackoff returns a zero-duration backoff for cases where no delay
// is needed between retries.
func NoBackoff() *Backoff {
	return New(
		WithInitial(0),
		WithMax(0),
		WithFactor(1.0),
		WithJitter(false),
	)
}
