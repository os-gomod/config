package eventbus

import "time"

// Option configures a BusConfig during construction.
type Option func(*BusConfig)

// WithWorkerCount sets the number of concurrent dispatch workers.
// Default: 32.
func WithWorkerCount(n int) Option {
	return func(c *BusConfig) {
		if n > 0 {
			c.WorkerCount = n
		}
	}
}

// WithQueueSize sets the capacity of the event queue channel.
// Default: 4096.
func WithQueueSize(n int) Option {
	return func(c *BusConfig) {
		if n > 0 {
			c.QueueSize = n
		}
	}
}

// WithRetryCount sets how many times a failed delivery is retried.
// Default: 0 (no retries).
func WithRetryCount(n int) Option {
	return func(c *BusConfig) {
		if n >= 0 {
			c.RetryCount = n
		}
	}
}

// WithRetryDelay sets the base delay between retries. Actual delay uses
// exponential backoff: base * 2^(attempt-1).
// Default: 100ms.
func WithRetryDelay(d time.Duration) Option {
	return func(c *BusConfig) {
		if d > 0 {
			c.RetryDelay = d
		}
	}
}

// WithPanicHandler sets the function invoked when an observer panics.
// The handler receives the recovered value. If nil, panics are recovered
// silently and converted to delivery errors.
func WithPanicHandler(h func(recovered any)) Option {
	return func(c *BusConfig) {
		c.PanicHandler = h
	}
}
