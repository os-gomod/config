// Package watcher provides live config reload management with debouncing,
// pattern-based subscriptions, and graceful shutdown.
package watcher

import (
	"context"
)

// Watcher watches a source for changes and signals when a reload is needed.
type Watcher interface {
	// Start begins watching. It returns a channel that receives one signal
	// each time a reload should be triggered. The channel is closed when
	// the watcher stops or ctx is cancelled.
	Start(ctx context.Context) (<-chan struct{}, error)

	// Stop halts the watcher and releases resources.
	// It is safe to call Stop multiple times.
	Stop()
}
