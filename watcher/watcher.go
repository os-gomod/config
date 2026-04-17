// Package watcher provides file and configuration change watching capabilities.
// Watchers can be composed via a Manager that supports debounced reload triggers
// and pattern-based event filtering.
package watcher

import (
	"context"
)

// Watcher is the interface for configuration change watchers. A watcher
// monitors a source for changes and signals when a reload may be needed.
type Watcher interface {
	// Start begins watching and returns a channel that is closed or signaled
	// when a change is detected. The context can be used for cancellation.
	Start(ctx context.Context) (<-chan struct{}, error)
	// Stop stops the watcher and releases any resources.
	Stop()
}
