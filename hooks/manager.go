package hooks

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/observability"
)

// Manager manages lifecycle hooks organized by hook type. Hooks for each type
// are sorted by priority and executed in order. The manager is safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	hooks    map[event.HookType][]Hook
	recorder observability.Recorder
}

// NewManager creates a new hook Manager with no hooks registered.
// A no-op observability recorder is used by default.
func NewManager() *Manager {
	return &Manager{
		hooks:    make(map[event.HookType][]Hook),
		recorder: observability.Nop(),
	}
}

// SetRecorder sets the observability recorder for timing hook execution.
// If r is nil, the call is ignored.
func (m *Manager) SetRecorder(r observability.Recorder) {
	if r != nil {
		m.recorder = r
	}
}

// Register adds a hook for the given hook type. Hooks are automatically sorted
// by priority after insertion (lower values execute first).
func (m *Manager) Register(hookType event.HookType, hook Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hks := m.hooks[hookType]
	hks = append(hks, hook)
	sort.Slice(hks, func(i, j int) bool { return hks[i].Priority() < hks[j].Priority() })
	m.hooks[hookType] = hks
}

// Has reports whether any hooks are registered for the given hook type.
func (m *Manager) Has(hookType event.HookType) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hooks[hookType]) > 0
}

// Execute runs all hooks registered for the given hook type in priority order.
// Returns the first error from any hook, or nil if all hooks succeed.
// Hook execution timing is recorded via the observability recorder.
func (m *Manager) Execute(ctx context.Context, hookType event.HookType, hctx *Context) error {
	m.mu.RLock()
	hks := make([]Hook, len(m.hooks[hookType]))
	copy(hks, m.hooks[hookType])
	m.mu.RUnlock()
	for _, h := range hks {
		start := time.Now()
		err := h.Execute(ctx, hctx)
		m.recorder.RecordHook(ctx, h.Name(), time.Since(start), err)
		if err != nil {
			return err
		}
	}
	return nil
}

// Clear removes all registered hooks.
func (m *Manager) Clear() {
	m.mu.Lock()
	m.hooks = make(map[event.HookType][]Hook)
	m.mu.Unlock()
}

// Count returns the number of hooks registered for the given hook type.
func (m *Manager) Count(hookType event.HookType) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hooks[hookType])
}
