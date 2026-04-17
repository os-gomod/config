package watcher

import (
	"context"

	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/pattern"
)

// PatternWatcher wraps an event.Observer with glob pattern filtering.
// Only events whose keys match the specified pattern are forwarded to the
// inner observer.
type PatternWatcher struct {
	pattern string
	inner   event.Observer
}

// NewPatternWatcher creates a new PatternWatcher that filters events by the
// given glob pattern before delegating to the inner observer.
func NewPatternWatcher(pat string, inner event.Observer) *PatternWatcher {
	return &PatternWatcher{pattern: pat, inner: inner}
}

// Observe checks if the event key matches the pattern and, if so, delegates
// to the inner observer. Returns nil if the event does not match.
func (pw *PatternWatcher) Observe(ctx context.Context, evt *event.Event) error {
	if !pattern.Match(evt.Key, pw.pattern) {
		return nil
	}
	return pw.inner(ctx, *evt)
}
