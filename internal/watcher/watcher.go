// Package watcher provides infrastructure for watching configuration sources
// for changes. It includes a WatchManager for coordinating multiple watchers
// and a Debouncer for coalescing rapid changes.
package watcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
)

// ---------------------------------------------------------------------------
// Watcher interface
// ---------------------------------------------------------------------------

// Watcher watches a configuration source for changes and emits events.
type Watcher interface {
	// Watch starts watching and returns a channel of change events.
	// The channel is closed when the context is cancelled.
	Watch(ctx context.Context) (<-chan event.Event, error)
	// Close stops the watcher and releases all resources.
	Close() error
}

// ---------------------------------------------------------------------------
// WatchManager
// ---------------------------------------------------------------------------

// WatchManager coordinates multiple named watchers. It manages their
// lifecycle and provides a way to stop all watchers at once.
// This is instance-based — NO global watch manager.
type WatchManager struct {
	mu       sync.Mutex
	watchers map[string]Watcher
	stop     chan struct{}
	wg       sync.WaitGroup
}

// NewWatchManager creates a new WatchManager.
func NewWatchManager() *WatchManager {
	return &WatchManager{
		watchers: make(map[string]Watcher),
		stop:     make(chan struct{}),
	}
}

// Register adds a named Watcher to the manager. Returns an error if a
// watcher with the same name already exists.
func (m *WatchManager) Register(name string, w Watcher) error {
	if name == "" {
		return errors.New(errors.CodeInvalidConfig, "watcher name must not be empty")
	}
	if w == nil {
		return errors.New(errors.CodeInvalidConfig,
			fmt.Sprintf("watcher %q must not be nil", name))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watchers[name]; exists {
		return errors.New(errors.CodeAlreadyExists,
			fmt.Sprintf("watcher %q is already registered", name))
	}

	m.watchers[name] = w
	return nil
}

// WatchAll starts all registered watchers in separate goroutines and
// returns a merged channel of events from all watchers.
func (m *WatchManager) WatchAll(ctx context.Context) (<-chan event.Event, error) {
	m.mu.Lock()
	watchers := make(map[string]Watcher, len(m.watchers))
	for name, w := range m.watchers {
		watchers[name] = w
	}
	m.mu.Unlock()

	if len(watchers) == 0 {
		return nil, errors.New(errors.CodeNotFound,
			"no watchers registered")
	}

	merged := make(chan event.Event, 64)

	m.wg.Add(len(watchers))
	for name, w := range watchers {
		go func(name string, w Watcher) {
			defer m.wg.Done()
			m.runWatcher(ctx, name, w, merged)
		}(name, w)
	}

	// Close merged channel when all watchers are done.
	go func() {
		m.wg.Wait()
		close(merged)
	}()

	return merged, nil
}

// runWatcher runs a single watcher and forwards events to the merged channel.
func (m *WatchManager) runWatcher(ctx context.Context, name string, w Watcher, merged chan<- event.Event) {
	ch, err := w.Watch(ctx)
	if err != nil {
		merged <- event.NewErrorEvent(name, err, event.WithSource(name))
		return
	}

	for evt := range ch {
		select {
		case <-ctx.Done():
			return
		case merged <- evt:
		}
	}
}

// StopAll stops all registered watchers and waits for them to finish.
func (m *WatchManager) StopAll() {
	m.mu.Lock()
	close(m.stop)
	m.mu.Unlock()

	m.wg.Wait()
}

// Close closes all registered watchers. After Close, the manager cannot
// be reused.
func (m *WatchManager) Close() error {
	// Stop all watchers first.
	m.StopAll()

	m.mu.Lock()
	defer m.mu.Unlock()

	for name, w := range m.watchers {
		if err := w.Close(); err != nil {
			// Log but don't fail — best effort close.
			_ = fmt.Sprintf("close watcher %q: %v", name, err)
		}
	}
	m.watchers = make(map[string]Watcher)
	return nil
}

// Unregister removes a watcher from the manager. It does NOT close the watcher.
func (m *WatchManager) Unregister(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watchers[name]; !exists {
		return false
	}
	delete(m.watchers, name)
	return true
}

// Names returns the names of all registered watchers.
func (m *WatchManager) Names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.watchers))
	for name := range m.watchers {
		names = append(names, name)
	}
	return names
}

// Len returns the number of registered watchers.
func (m *WatchManager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.watchers)
}
