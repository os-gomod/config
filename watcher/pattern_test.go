package watcher

import (
	"context"
	"testing"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

func TestNewPatternWatcher(t *testing.T) {
	pw := NewPatternWatcher("*", func(ctx context.Context, evt event.Event) error {
		return nil
	})
	if pw == nil {
		t.Fatal("expected non-nil PatternWatcher")
	}
}

func TestPatternWatcherObserveMatch(t *testing.T) {
	var observed bool
	pw := NewPatternWatcher("app.*", func(ctx context.Context, evt event.Event) error {
		observed = true
		return nil
	})

	evt := event.Event{
		Type: event.TypeUpdate,
		Key:  "app.port",
	}
	err := pw.Observe(context.Background(), &evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !observed {
		t.Error("expected event to be observed for matching pattern")
	}
}

func TestPatternWatcherObserveNoMatch(t *testing.T) {
	var observed bool
	pw := NewPatternWatcher("app.*", func(ctx context.Context, evt event.Event) error {
		observed = true
		return nil
	})

	evt := event.Event{
		Type: event.TypeUpdate,
		Key:  "other.port",
	}
	err := pw.Observe(context.Background(), &evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observed {
		t.Error("event should not be observed for non-matching pattern")
	}
}

func TestPatternWatcherObserveWildcard(t *testing.T) {
	var count int
	pw := NewPatternWatcher("*", func(ctx context.Context, evt event.Event) error {
		count++
		return nil
	})

	pw.Observe(context.Background(), &event.Event{Key: "any.key"})
	pw.Observe(context.Background(), &event.Event{Key: "another.key"})
	if count != 2 {
		t.Errorf("expected 2 observations, got %d", count)
	}
}

func TestPatternWatcherObserveEmpty(t *testing.T) {
	var observed bool
	pw := NewPatternWatcher("", func(ctx context.Context, evt event.Event) error {
		observed = true
		return nil
	})
	pw.Observe(context.Background(), &event.Event{Key: "any.key"})
	if !observed {
		t.Error("empty pattern should match everything")
	}
}

func TestPatternWatcherObserveError(t *testing.T) {
	pw := NewPatternWatcher("*", func(ctx context.Context, evt event.Event) error {
		return context.Canceled
	})

	err := pw.Observe(context.Background(), &event.Event{Key: "test"})
	if err == nil {
		t.Fatal("expected error from inner observer")
	}
}

func TestPatternWatcherGlobPattern(t *testing.T) {
	var observed bool
	pw := NewPatternWatcher("server.*.port", func(ctx context.Context, evt event.Event) error {
		observed = true
		return nil
	})

	// Matching
	pw.Observe(context.Background(), &event.Event{Key: "server.main.port"})
	if !observed {
		t.Error("expected match for server.main.port")
	}

	observed = false
	// Not matching
	pw.Observe(context.Background(), &event.Event{Key: "server.port"})
	if observed {
		t.Error("should not match server.port")
	}
}

// Suppress unused import warnings
func _init() {
	_ = value.Value{}
	_ = context.Background()
}
