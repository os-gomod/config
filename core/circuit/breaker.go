// Package circuit implements a three-state circuit breaker (closed -> open -> half-open)
// for protecting config layer load operations against cascading failures.
package circuit

import (
	"sync/atomic"
	"time"
)

// Circuit breaker states: closed allows all calls, open rejects all calls,
// and half-open allows a limited number of probe calls to test recovery.
const (
	// stateClosed represents the normal operating state where all calls are permitted.
	stateClosed int32 = 0
	// stateOpen represents the tripped state where all calls are rejected.
	stateOpen int32 = 1
	// stateHalfOpen represents the recovery probe state where limited calls are permitted.
	stateHalfOpen int32 = 2
)

// Breaker is a lock-free, three-state circuit breaker.
// State transitions: closed -> open (on threshold failures) -> half-open (after timeout)
// -> closed (on success threshold) or back to open (on any failure).
type Breaker struct {
	state                atomic.Int32 // current breaker state
	failures             atomic.Int64 // consecutive failure count in closed state
	lastOpen             atomic.Int64 // unix-nano timestamp when the breaker opened
	consecutiveSuccesses atomic.Int64 // success count in half-open state
	threshold            int64        // failures required to trip
	timeout              time.Duration
	successThreshold     int64 // successes in half-open required to close
}

// BreakerConfig holds the configuration for creating a new Breaker.
type BreakerConfig struct {
	Threshold        int           // consecutive failures to trip the breaker
	Timeout          time.Duration // duration in open state before transitioning to half-open
	SuccessThreshold int           // consecutive successes in half-open to close the breaker
}

// DefaultConfig returns a BreakerConfig with sensible defaults:
// 5 failures, 30-second timeout, 1 success to close.
func DefaultConfig() BreakerConfig {
	return BreakerConfig{
		Threshold:        5,
		Timeout:          30 * time.Second,
		SuccessThreshold: 1,
	}
}

// New creates a Breaker with the given configuration.
// Zero or negative values are replaced with defaults.
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

// IsOpen reports whether the breaker is in the open state, blocking calls.
// If the breaker is open but the timeout has elapsed, it atomically
// transitions to half-open and returns false.
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

// RecordFailure records a failure. In closed state it increments the failure
// counter and trips to open when the threshold is reached. In half-open state
// it immediately re-opens the breaker.
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

// RecordSuccess records a success. In half-open state it counts consecutive
// successes and closes the breaker when the success threshold is reached.
// In closed state it resets the failure counter.
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

// State returns the current breaker state as a human-readable string:
// "closed", "open", or "half_open".
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
