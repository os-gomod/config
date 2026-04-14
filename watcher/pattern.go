package watcher

import (
	"context"

	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/pattern"
)

// PatternWatcher filters event.Observer calls to only those events
// whose key matches a glob pattern.
// It is used by config.Config.Subscribe to implement key-pattern subscriptions.
type PatternWatcher struct {
	pattern string
	inner   event.Observer
}

// NewPatternWatcher returns a PatternWatcher that calls inner only when
// the event key matches pattern. Pattern syntax follows event.Bus.Subscribe.
func NewPatternWatcher(pat string, inner event.Observer) *PatternWatcher {
	return &PatternWatcher{pattern: pat, inner: inner}
}

// Observe implements event.Observer.
// It forwards evt to the inner observer only if evt.Key matches the pattern.
func (pw *PatternWatcher) Observe(ctx context.Context, evt event.Event) error {
	if !pattern.Match(evt.Key, pw.pattern) {
		return nil
	}
	return pw.inner(ctx, evt)
}
