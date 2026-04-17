package watcher

import (
	"sync"
	"time"
)

// debouncer coalesces rapid trigger calls into a single delayed invocation.
// Each new trigger resets the timer, so only the last trigger in a burst
// results in the callback being executed.
type debouncer struct {
	mu    sync.Mutex
	timer *time.Timer
	dur   time.Duration
}

// newDebouncer creates a debouncer with the given delay duration.
func newDebouncer(dur time.Duration) *debouncer {
	return &debouncer{dur: dur}
}

// trigger resets the debounce timer. If a previous timer is pending, it is
// cancelled and replaced with a new one. The callback is called after the
// debounce duration elapses without another trigger.
func (d *debouncer) trigger(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.dur, fn)
}

// stop cancels any pending debounce timer.
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
