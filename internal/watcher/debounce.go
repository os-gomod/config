package watcher

import (
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Debouncer
// ---------------------------------------------------------------------------

// Debouncer coalesces rapid function invocations. When a function is scheduled,
// it waits for the configured interval. If more calls arrive during the wait,
// the timer resets. Only the last function call actually executes.
type Debouncer struct {
	interval time.Duration
	timer    *time.Timer
	mu       sync.Mutex
	pending  bool
}

// NewDebouncer creates a new Debouncer with the given interval.
// If interval is 0, functions execute immediately without debouncing.
func NewDebouncer(interval time.Duration) *Debouncer {
	return &Debouncer{
		interval: interval,
	}
}

// Run schedules fn to run after the debounce interval. If another Run call
// arrives before the interval expires, the previous call is cancelled and
// the timer restarts. Only the last fn actually executes.
func (d *Debouncer) Run(fn func()) {
	if fn == nil {
		return
	}

	// Zero interval means no debouncing — execute immediately.
	if d.interval <= 0 {
		fn()
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel any pending timer.
	if d.timer != nil {
		d.timer.Stop()
		d.pending = false
	}

	// Store the function and start a new timer.
	d.timer = time.AfterFunc(d.interval, func() {
		d.mu.Lock()
		d.pending = false
		d.mu.Unlock()
		fn()
	})
	d.pending = true
}

// Stop cancels any pending debounced function. Returns true if a function
// was pending and was cancelled, false otherwise.
func (d *Debouncer) Stop() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil && d.pending {
		d.timer.Stop()
		d.pending = false
		return true
	}
	return false
}

// Pending returns true if a function call is currently debounced (waiting
// for the interval to elapse).
func (d *Debouncer) Pending() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pending
}

// Interval returns the configured debounce interval.
func (d *Debouncer) Interval() time.Duration {
	return d.interval
}

// SetInterval changes the debounce interval. Any pending call will use the
// new interval on the next Run invocation.
func (d *Debouncer) SetInterval(interval time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.interval = interval
}
