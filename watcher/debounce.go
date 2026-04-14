package watcher

import (
	"sync"
	"time"
)

// debouncer coalesces rapid-fire triggers into a single function call
// after a quiet period of duration dur.
type debouncer struct {
	mu    sync.Mutex
	timer *time.Timer
	dur   time.Duration
}

// newDebouncer creates a debouncer with the given quiet period.
func newDebouncer(dur time.Duration) *debouncer {
	return &debouncer{dur: dur}
}

// trigger resets the debounce timer. If another trigger arrives within dur,
// the timer resets. fn is called once after dur of silence.
func (d *debouncer) trigger(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.dur, fn)
}

// stop cancels the pending timer and cleans up.
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
