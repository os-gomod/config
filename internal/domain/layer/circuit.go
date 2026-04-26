package layer

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

// State represents the state of a circuit breaker.
type State int

const (
	StateClosed   State = iota // Closed: requests flow normally.
	StateOpen                  // Open: requests are rejected.
	StateHalfOpen              // HalfOpen: limited requests allowed to test recovery.
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// Config defines the parameters for a circuit breaker.
type Config struct {
	// MaxRequests is the maximum number of requests allowed in the half-open state.
	// If zero, DefaultConfig values are used.
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for the circuit breaker
	// to clear the internal counts. If zero, counts are never reset.
	Interval time.Duration

	// Timeout is the period of the open state, after which the breaker
	// transitions to half-open. If zero, DefaultConfig values are used.
	Timeout time.Duration

	// ReadyToTrip is called with a copy of the breaker's counts whenever
	// a request fails in the closed state. If ReadyToTrip returns true,
	// the breaker transitions to open.
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called whenever the state of the circuit breaker changes.
	OnStateChange func(name string, from, to State)

	// IsSuccessful is called to determine if a request error should count as
	// a success or failure for the circuit breaker. If nil, any non-nil error
	// counts as a failure.
	IsSuccessful func(err error) bool

	// HalfOpenMaxSuccesses is the number of consecutive successes needed in
	// half-open state to transition back to closed. Defaults to 1.
	HalfOpenMaxSuccesses int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxRequests:          3,
		Interval:             60 * time.Second,
		Timeout:              30 * time.Second,
		HalfOpenMaxSuccesses: 1,
		ReadyToTrip:          defaultReadyToTrip,
	}
}

// defaultReadyToTrip opens the circuit after 5 consecutive failures.
func defaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > 5
}

// ---------------------------------------------------------------------------
// Counts
// ---------------------------------------------------------------------------

// Counts holds the internal statistics of a circuit breaker.
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// ---------------------------------------------------------------------------
// Breaker
// ---------------------------------------------------------------------------

// Breaker implements a circuit breaker pattern for config source operations.
// It protects against cascading failures by temporarily stopping requests
// to sources that are consistently failing.
type Breaker struct {
	name       string
	config     Config
	mu         sync.Mutex
	state      State
	counts     Counts
	expiry     time.Time // when the current state expires
	generation uint64    // incremented on state changes for stale detection
}

// New creates a new circuit breaker with the given name and config.
func NewBreaker(name string, config Config) *Breaker {
	if config.MaxRequests == 0 {
		config.MaxRequests = DefaultConfig().MaxRequests
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultConfig().Timeout
	}
	if config.HalfOpenMaxSuccesses == 0 {
		config.HalfOpenMaxSuccesses = DefaultConfig().HalfOpenMaxSuccesses
	}
	if config.ReadyToTrip == nil {
		config.ReadyToTrip = defaultReadyToTrip
	}

	return &Breaker{
		name:   name,
		config: config,
		state:  StateClosed,
		expiry: time.Time{},
	}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Name returns the breaker name.
func (b *Breaker) Name() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.name
}

// State returns the current state of the breaker.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if b.state == StateClosed && !b.expiry.IsZero() && now.After(b.expiry) {
		b.counts = Counts{}
		b.expiry = time.Time{}
	}

	if b.state == StateOpen && now.After(b.expiry) {
		b.toHalfOpen()
	}

	return b.state
}

// IsOpen returns true if the circuit breaker is open (rejecting requests).
func (b *Breaker) IsOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	if b.state == StateClosed && !b.expiry.IsZero() && now.After(b.expiry) {
		b.counts = Counts{}
		b.expiry = time.Time{}
		return false
	}

	if b.state == StateOpen {
		if now.After(b.expiry) {
			b.toHalfOpen()
			return false
		}
		return true
	}

	return false
}

// Allow checks if a request is permitted. If the breaker is closed, it always
// returns true. If open, returns false. If half-open, returns true up to
// MaxRequests times.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	switch b.state {
	case StateClosed:
		if !b.expiry.IsZero() && now.After(b.expiry) {
			b.counts = Counts{}
			b.expiry = time.Time{}
		}
		return true

	case StateOpen:
		if now.After(b.expiry) {
			b.toHalfOpen()
			return true
		}
		return false

	case StateHalfOpen:
		if b.counts.Requests < b.config.MaxRequests {
			b.counts.Requests++
			return true
		}
		return false

	default:
		return true
	}
}

// AllowWithGeneration is like Allow, but also returns the current generation
// for stale detection. The caller should check that the generation hasn't
// changed between Allow and RecordSuccess/RecordFailure.
func (b *Breaker) AllowWithGeneration() (bool, uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	generation := b.generation

	now := time.Now()

	switch b.state {
	case StateClosed:
		if !b.expiry.IsZero() && now.After(b.expiry) {
			b.counts = Counts{}
			b.expiry = time.Time{}
		}
		return true, generation

	case StateOpen:
		if now.After(b.expiry) {
			b.toHalfOpen()
			generation = b.generation
			return true, generation
		}
		return false, generation

	case StateHalfOpen:
		if b.counts.Requests < b.config.MaxRequests {
			b.counts.Requests++
			return true, generation
		}
		return false, generation

	default:
		return true, generation
	}
}

// RecordSuccess records a successful request.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counts.Requests++
	b.counts.TotalSuccesses++
	b.counts.ConsecutiveSuccesses++
	b.counts.ConsecutiveFailures = 0

	if b.state == StateHalfOpen {
		if b.counts.ConsecutiveSuccesses >= uint32(b.config.HalfOpenMaxSuccesses) {
			b.toClosed()
		}
	}
}

// RecordFailure records a failed request.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counts.Requests++
	b.counts.TotalFailures++
	b.counts.ConsecutiveFailures++
	b.counts.ConsecutiveSuccesses = 0

	switch b.state {
	case StateClosed:
		if b.config.ReadyToTrip(b.counts) {
			b.toOpen()
		}
	case StateHalfOpen:
		b.toOpen()
	}
}

// RecordFailureWithErr records a failure, checking IsSuccessful first if configured.
func (b *Breaker) RecordFailureWithErr(err error) {
	if err == nil {
		b.RecordSuccess()
		return
	}
	if b.config.IsSuccessful != nil && b.config.IsSuccessful(err) {
		b.RecordSuccess()
		return
	}
	b.RecordFailure()
}

// Counts returns a copy of the current breaker counts.
func (b *Breaker) Counts() Counts {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.counts
}

// Generation returns the current generation counter.
func (b *Breaker) Generation() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.generation
}

// Config returns a copy of the breaker configuration.
func (b *Breaker) Config() Config {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.config
}

// Reset resets the breaker to the closed state and clears all counts.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldState := b.state
	b.state = StateClosed
	b.counts = Counts{}
	b.expiry = time.Time{}
	b.generation++

	b.notify(oldState, StateClosed)
}

// ---------------------------------------------------------------------------
// State transitions
// ---------------------------------------------------------------------------

func (b *Breaker) toClosed() {
	oldState := b.state
	b.state = StateClosed
	if b.config.Interval > 0 {
		b.expiry = time.Now().Add(b.config.Interval)
	} else {
		b.expiry = time.Time{}
	}
	b.counts = Counts{}
	b.generation++
	b.notify(oldState, StateClosed)
}

func (b *Breaker) toOpen() {
	oldState := b.state
	b.state = StateOpen
	b.expiry = time.Now().Add(b.config.Timeout)
	b.counts.Requests = 0
	b.generation++
	b.notify(oldState, StateOpen)
}

func (b *Breaker) toHalfOpen() {
	oldState := b.state
	b.state = StateHalfOpen
	b.expiry = time.Time{} // no expiry for half-open
	b.counts.Requests = 0
	b.generation++
	b.notify(oldState, StateHalfOpen)
}

func (b *Breaker) notify(from, to State) {
	if b.config.OnStateChange != nil {
		b.config.OnStateChange(b.name, from, to)
	}
}

// ---------------------------------------------------------------------------
// String / info
// ---------------------------------------------------------------------------

// String returns a human-readable representation of the breaker state.
func (b *Breaker) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return fmt.Sprintf("Breaker{name=%q, state=%s, requests=%d, successes=%d, failures=%d, consecutive_failures=%d}",
		b.name,
		b.state,
		b.counts.Requests,
		b.counts.TotalSuccesses,
		b.counts.TotalFailures,
		b.counts.ConsecutiveFailures,
	)
}

// SuccessRate returns the success rate as a float between 0.0 and 1.0.
// Returns 0.0 if no requests have been made.
func (b *Breaker) SuccessRate() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.counts.Requests == 0 {
		return 0.0
	}
	return float64(b.counts.TotalSuccesses) / float64(b.counts.Requests)
}

// FailureRate returns the failure rate as a float between 0.0 and 1.0.
// Returns 0.0 if no requests have been made.
func (b *Breaker) FailureRate() float64 {
	return 1.0 - b.SuccessRate()
}

// ---------------------------------------------------------------------------
// Counts helper
// ---------------------------------------------------------------------------

// FailureRatio returns the ratio of consecutive failures to total requests.
func (c Counts) FailureRatio() float64 {
	if c.Requests == 0 {
		return 0.0
	}
	ratio := float64(c.ConsecutiveFailures) / float64(c.Requests)
	return math.Round(ratio*1000) / 1000 // 3 decimal places
}
