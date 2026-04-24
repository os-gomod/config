// Package backoff provides exponential backoff with jitter for retry logic.
//
// It is used by RemoteProvider to retry failed client initialization and
// watch reconnection attempts, and can be used anywhere transient failures
// are expected (network blips, leader elections, etc.).
//
// The algorithm computes: delay = min(initial * multiplier^attempt, max) * (1 - jitter/2 + rand*jitter)
//
// This is the standard "equal jitter" approach recommended by AWS Architecture
// Center and used in virtually all FAANG-grade retry systems.
package backoff

import (
	"math"
	"math/rand/v2"
	"time"
)

// Config controls the exponential backoff behavior.
type Config struct {
	// InitialInterval is the delay before the first retry. Defaults to 100ms.
	InitialInterval time.Duration

	// MaxInterval is the maximum delay between retries. Defaults to 10s.
	MaxInterval time.Duration

	// Multiplier grows the interval exponentially. Defaults to 2.0.
	Multiplier float64

	// JitterFactor adds randomness to avoid thundering herd.
	// 0 = no jitter, 1 = full jitter. Defaults to 0.2 (20%).
	JitterFactor float64

	// MaxRetries is the maximum number of retry attempts.
	// 0 means unlimited retries (until context cancelled). Defaults to 5.
	MaxRetries int
}

// DefaultConfig returns sensible production defaults:
//
//	Initial: 100ms, Max: 10s, Multiplier: 2x, Jitter: 20%, MaxRetries: 5
func DefaultConfig() Config {
	return Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		JitterFactor:    0.2,
		MaxRetries:      5,
	}
}

// Backoff tracks the state of a single retry sequence.
// It is not goroutine-safe; create a new Backoff for each retry sequence.
type Backoff struct {
	cfg     Config
	attempt int
}

// New creates a Backoff with the given configuration.
func New(cfg Config) *Backoff {
	if cfg.InitialInterval <= 0 {
		cfg.InitialInterval = 100 * time.Millisecond
	}
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = 10 * time.Second
	}
	if cfg.Multiplier <= 1.0 {
		cfg.Multiplier = 2.0
	}
	if cfg.JitterFactor < 0 {
		cfg.JitterFactor = 0
	}
	if cfg.JitterFactor > 1.0 {
		cfg.JitterFactor = 1.0
	}
	return &Backoff{cfg: cfg}
}

// Next computes the delay for the next retry attempt.
//
// Returns (delay, shouldRetry):
//   - On the first call, it returns the initial interval.
//   - Each subsequent call applies the multiplier and caps at MaxInterval.
//   - If JitterFactor > 0, equal jitter is applied to spread retries.
//   - When MaxRetries > 0 and attempts are exhausted, returns (0, false).
func (b *Backoff) Next() (time.Duration, bool) {
	if b.cfg.MaxRetries > 0 && b.attempt >= b.cfg.MaxRetries {
		return 0, false
	}

	// Exponential growth: delay = initial * multiplier^attempt
	delay := float64(b.cfg.InitialInterval) * math.Pow(b.cfg.Multiplier, float64(b.attempt))
	if delay > float64(b.cfg.MaxInterval) {
		delay = float64(b.cfg.MaxInterval)
	}

	// Apply equal jitter: spread uniformly within [delay*(1-j), delay]
	if b.cfg.JitterFactor > 0 {
		jitterRange := delay * b.cfg.JitterFactor
		delay = delay - jitterRange/2 + rand.Float64()*jitterRange
	}

	b.attempt++
	return time.Duration(delay), true
}

// Attempt returns the current attempt number (0-indexed).
func (b *Backoff) Attempt() int { return b.attempt }

// Reset resets the attempt counter so the Backoff can be reused.
func (b *Backoff) Reset() { b.attempt = 0 }
