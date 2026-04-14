package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/core/value"
)

// ChangeType classifies the kind of change.
type ChangeType int

const (
	// ChangeCreated indicates a new key was added.
	ChangeCreated ChangeType = iota
	// ChangeModified indicates an existing key changed.
	ChangeModified
	// ChangeDeleted indicates a key was removed.
	ChangeDeleted
	// ChangeReload indicates a full reload occurred.
	ChangeReload
)

var changeTypeNames = [...]string{
	ChangeCreated:  "created",
	ChangeModified: "modified",
	ChangeDeleted:  "deleted",
	ChangeReload:   "reload",
}

// String returns the human-readable name of the ChangeType.
func (c ChangeType) String() string {
	if c >= 0 && int(c) < len(changeTypeNames) {
		return changeTypeNames[c]
	}
	return "unknown"
}

// Change describes a single config change event delivered to Callbacks.
type Change struct {
	// Type is the kind of change.
	Type ChangeType
	// Key is the config key affected.
	Key string
	// OldValue is the value before the change.
	OldValue value.Value
	// NewValue is the value after the change.
	NewValue value.Value
	// Timestamp is the time at which the change occurred.
	Timestamp time.Time
}

// Callback is called on every debounced reload trigger.
type Callback func(ctx context.Context, change Change)

// Manager coordinates multiple Watchers and debounces reload triggers.
// It distributes reload notifications to registered Callback functions.
//
// Lock ordering: mu before debounceMu. Never hold debounceMu while acquiring mu.
type Manager struct {
	mu         sync.RWMutex // protects callbacks and stopFns
	callbacks  []Callback
	stopFns    []func()
	debounceMu sync.Mutex // protects debouncer
	debouncer  *debouncer
}

// NewManager returns a Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Add registers a Watcher. If the Manager is already running (i.e., at least
// one watcher has been started), the new watcher is started immediately.
func (m *Manager) Add(w Watcher) error {
	m.mu.Lock()
	m.stopFns = append(m.stopFns, w.Stop)
	m.mu.Unlock()
	return nil
}

// Subscribe registers a Callback invoked on every reload trigger.
func (m *Manager) Subscribe(cb Callback) {
	m.mu.Lock()
	m.callbacks = append(m.callbacks, cb)
	m.mu.Unlock()
}

// Notify invokes all registered callbacks with the given change.
func (m *Manager) Notify(ctx context.Context, change Change) {
	m.mu.RLock()
	cbs := make([]Callback, len(m.callbacks))
	copy(cbs, m.callbacks)
	m.mu.RUnlock()
	for _, cb := range cbs {
		cb(ctx, change)
	}
}

// TriggerReload debounces fn: if another trigger arrives within dur,
// the timer resets. fn is called once after dur of silence.
func (m *Manager) TriggerReload(dur time.Duration, fn func()) {
	m.debounceMu.Lock()
	if m.debouncer == nil || m.debouncer.dur != dur {
		m.debouncer = newDebouncer(dur)
	}
	d := m.debouncer
	m.debounceMu.Unlock()
	d.trigger(fn)
}

// StopAll stops all registered Watchers and the debouncer.
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

// StopDebouncer stops only the debouncer (used on graceful shutdown).
func (m *Manager) StopDebouncer() {
	m.debounceMu.Lock()
	defer m.debounceMu.Unlock()
	if m.debouncer != nil {
		m.debouncer.stop()
	}
}
