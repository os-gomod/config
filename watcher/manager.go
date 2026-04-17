package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
)

// ChangeType identifies the type of configuration change.
type ChangeType int

const (
	// ChangeCreated indicates a new configuration key was created.
	ChangeCreated ChangeType = iota
	// ChangeModified indicates an existing configuration key was changed.
	ChangeModified
	// ChangeDeleted indicates a configuration key was removed.
	ChangeDeleted
	// ChangeReload indicates the entire configuration was reloaded.
	ChangeReload
)

// changeTypeNames maps ChangeType constants to their string representations.
var changeTypeNames = [...]string{
	ChangeCreated:  "created",
	ChangeModified: "modified",
	ChangeDeleted:  "deleted",
	ChangeReload:   "reload",
}

// String returns the human-readable name of the change type.
// Returns "unknown" for undefined types.
func (c ChangeType) String() string {
	if c >= 0 && int(c) < len(changeTypeNames) {
		return changeTypeNames[c]
	}
	return "unknown"
}

// Change represents a single configuration change with its type, key,
// old and new values, and timestamp.
type Change struct {
	Type      ChangeType
	Key       string
	OldValue  value.Value
	NewValue  value.Value
	Timestamp time.Time
}

// Callback is a function that processes a configuration change notification.
type Callback func(ctx context.Context, change Change)

// Manager orchestrates multiple Watchers and distributes change notifications
// to registered Callbacks. It also supports debounced reload triggers to
// coalesce rapid change events into a single reload.
//
// The manager is safe for concurrent use.
type Manager struct {
	mu         sync.RWMutex
	callbacks  []Callback
	stopFns    []func()
	debounceMu sync.Mutex
	debouncer  *debouncer
}

// NewManager creates a new watcher Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Add registers a watcher with the manager. The watcher's Stop function will
// be called when StopAll is invoked.
func (m *Manager) Add(w Watcher) error {
	m.mu.Lock()
	m.stopFns = append(m.stopFns, w.Stop)
	m.mu.Unlock()
	return nil
}

// Subscribe registers a callback that will be called for every change notification
// via Notify.
func (m *Manager) Subscribe(cb Callback) {
	m.mu.Lock()
	m.callbacks = append(m.callbacks, cb)
	m.mu.Unlock()
}

// Notify delivers a change notification to all registered callbacks synchronously.
func (m *Manager) Notify(ctx context.Context, change *Change) {
	m.mu.RLock()
	cbs := make([]Callback, len(m.callbacks))
	copy(cbs, m.callbacks)
	m.mu.RUnlock()
	for _, cb := range cbs {
		cb(ctx, *change)
	}
}

// TriggerReload schedules a debounced reload. If the debounce duration changes,
// a new debouncer is created. The callback is called after the debounce
// duration elapses without another trigger.
func (m *Manager) TriggerReload(dur time.Duration, fn func()) {
	m.debounceMu.Lock()
	if m.debouncer == nil || m.debouncer.dur != dur {
		m.debouncer = newDebouncer(dur)
	}
	d := m.debouncer
	m.debounceMu.Unlock()
	d.trigger(fn)
}

// StopAll stops all registered watchers and the debouncer.
func (m *Manager) StopAll() {
	m.mu.RLock()
	stops := make([]func(), len(m.stopFns))
	copy(stops, m.stopFns)
	m.mu.RUnlock()
	for _, s := range stops {
		s()
	}
	m.debounceMu.Lock()
	if m.debouncer != nil {
		m.debouncer.stop()
	}
	m.debounceMu.Unlock()
}

// StopDebouncer stops only the debounce timer without stopping watchers.
func (m *Manager) StopDebouncer() {
	m.debounceMu.Lock()
	defer m.debounceMu.Unlock()
	if m.debouncer != nil {
		m.debouncer.stop()
	}
}
