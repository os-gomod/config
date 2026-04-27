package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/event"
)

// worker processes dispatch jobs from the shared queue.
// Each worker runs in its own goroutine, reading jobs until the
// channel is closed (on bus shutdown).
type worker struct {
	id   int
	jobs <-chan dispatchJob
	bus  *Bus
}

// newWorker creates an idle worker bound to the shared job channel.
func newWorker(id int, jobs <-chan dispatchJob, bus *Bus) *worker {
	return &worker{
		id:   id,
		jobs: jobs,
		bus:  bus,
	}
}

// start begins the worker's main loop. It blocks on the jobs channel
// and processes each dispatchJob until the channel is closed.
func (w *worker) start() {
	for job := range w.jobs {
		w.process(&job)
	}
}

// process iterates over all matched subscriptions and attempts delivery
// for each one, respecting the retry configuration.
func (w *worker) process(job *dispatchJob) {
	for _, sub := range job.subs {
		w.deliverWithRetry(&job.evt, sub)
	}
}

// deliverWithRetry attempts to deliver an event to a subscriber,
// retrying up to RetryCount times with exponential backoff.
func (w *worker) deliverWithRetry(evt *event.Event, sub subscription) {
	var err error

	for attempt := 0; attempt <= w.bus.config.RetryCount; attempt++ {
		// Exponential backoff on retries (skip delay on first attempt).
		if attempt > 0 {
			delay := w.bus.config.RetryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-time.After(delay):
			case <-w.bus.stopCtx.Done():
				// Bus is shutting down; abort retries.
				w.bus.failed.Add(1)
				return
			}
		}

		err = w.deliver(evt, sub)
		if err == nil {
			w.bus.delivered.Add(1)
			return
		}
	}

	// All retries exhausted.
	w.bus.failed.Add(1)
}

// deliver invokes the observer with panic recovery. Any panic is
// caught, forwarded to PanicHandler (if configured), and converted
// to an error so the retry mechanism can handle it.
func (w *worker) deliver(evt *event.Event, sub subscription) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if w.bus.config.PanicHandler != nil {
				w.bus.config.PanicHandler(r)
			}
			err = fmt.Errorf("eventbus: observer %d panicked: %v", sub.id, r)
		}
	}()

	return sub.observer(context.Background(), *evt)
}
