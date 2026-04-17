// Package circuit provides a simple circuit breaker implementation for protecting
// configuration operations from cascading failures. It supports three states:
// closed (normal operation), open (failing fast), and half-open (probing for recovery).
package circuit

import (
	"sync/atomic"
	"time"
)

// Internal circuit breaker state constants.
const (
	stateClosed   int32 = 0
	stateOpen     int32 = 1
	stateHalfOpen int32 = 2
)

// Breaker implements a circuit breaker pattern for configuration operations.
// It tracks consecutive failures and transitions between closed, open, and
// half-open states based on configurable thresholds.
//
// State transitions:
//   - Closed → Open: when failures reach the threshold
//   - Open → Half-Open: after the timeout elapses
//   - Half-Open → Closed: after enough consecutive successes
//   - Half-Open → Open: on any failure
type Breaker struct {
	state                atomic.Int32
	failures             atomic.Int64
	lastOpen             atomic.Int64
	consecutiveSuccesses atomic.Int64
	threshold            int64
	timeout              time.Duration
	successThreshold     int64
}

// BreakerConfig holds the configuration parameters for a Breaker.
type BreakerConfig struct {
	// Threshold is the number of consecutive failures before opening the circuit.
	Threshold int
	// Timeout is how long the circuit stays open before transitioning to half-open.
	Timeout time.Duration
	// SuccessThreshold is the number of consecutive successes in half-open
	// state needed to close the circuit.
	SuccessThreshold int
}

// DefaultConfig returns a BreakerConfig with sensible defaults:
// threshold=5, timeout=30s, successThreshold=1.
func DefaultConfig() BreakerConfig {
	return BreakerConfig{
		Threshold:        5,
		Timeout:          30 * time.Second,
		SuccessThreshold: 1,
	}
}

// New creates a new Breaker with the given configuration. Invalid values
// (<=0) are replaced with defaults from DefaultConfig.
func New(cfg BreakerConfig) *Breaker {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 1
	}
	return &Breaker{
		threshold:        int64(cfg.Threshold),
		timeout:          cfg.Timeout,
		successThreshold: int64(cfg.SuccessThreshold),
	}
}

// IsOpen reports whether the circuit breaker is currently preventing operations.
// Returns false in closed and half-open states, or true in open state (unless
// the timeout has elapsed, in which case it transitions to half-open).
func (b *Breaker) IsOpen() bool {
	s := b.state.Load()
	if s == stateClosed {
		return false
	}
	if s == stateOpen {
		elapsed := time.Since(time.Unix(0, b.lastOpen.Load()))
		if elapsed > b.timeout {
			if b.state.CompareAndSwap(stateOpen, stateHalfOpen) {
				b.consecutiveSuccesses.Store(0)
			}
			return false
		}
		return true
	}
	return false
}

// RecordFailure records a failure. In half-open state, this immediately
// reopens the circuit. In closed state, it increments the failure counter
// and opens the circuit if the threshold is reached.
func (b *Breaker) RecordFailure() {
	cur := b.state.Load()
	if cur == stateHalfOpen {
		b.consecutiveSuccesses.Store(0)
		if b.state.CompareAndSwap(stateHalfOpen, stateOpen) {
			b.lastOpen.Store(time.Now().UTC().UnixNano())
		}
		return
	}
	if b.failures.Add(1) >= b.threshold {
		if b.state.CompareAndSwap(stateClosed, stateOpen) {
			b.lastOpen.Store(time.Now().UTC().UnixNano())
		}
	}
}

// RecordSuccess records a success. In half-open state, it increments the
// success counter and closes the circuit if the threshold is reached.
// In closed state, it resets the failure counter.
func (b *Breaker) RecordSuccess() {
	cur := b.state.Load()
	if cur == stateHalfOpen {
		n := b.consecutiveSuccesses.Add(1)
		if n >= b.successThreshold {
			b.state.CompareAndSwap(stateHalfOpen, stateClosed)
			b.failures.Store(0)
		}
		return
	}
	if cur == stateClosed {
		b.failures.Store(0)
	}
}

// State returns the current state as a string: "closed", "open", or "half_open".
func (b *Breaker) State() string {
	switch b.state.Load() {
	case stateClosed:
		return "closed"
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}
