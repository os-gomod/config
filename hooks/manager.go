package hooks

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/observability"
)

// Manager executes lifecycle hooks in priority order.
// It holds a single lock (mu); no nested locking occurs.
type Manager struct {
	mu       sync.RWMutex
	hooks    map[event.HookType][]Hook
	recorder observability.Recorder
}

// NewManager creates a Manager with a no-op recorder.
func NewManager() *Manager {
	return &Manager{
		hooks:    make(map[event.HookType][]Hook),
		recorder: observability.Nop(),
	}
}

// SetRecorder sets the observability recorder. If r is nil, the call is ignored.
func (m *Manager) SetRecorder(r observability.Recorder) {
	if r != nil {
		m.recorder = r
	}
}

// Register adds a hook for the given hook type. Hooks are sorted by ascending
// priority after insertion.
func (m *Manager) Register(hookType event.HookType, hook Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hks := m.hooks[hookType]
	hks = append(hks, hook)
	sort.Slice(hks, func(i, j int) bool { return hks[i].Priority() < hks[j].Priority() })
	m.hooks[hookType] = hks
}

// Has reports whether any hooks are registered for the given type.
func (m *Manager) Has(hookType event.HookType) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hooks[hookType]) > 0
}

// Execute runs all hooks of the given type in priority order, recording
// each execution via the configured recorder. It returns the first error
// encountered, or nil on success.
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

// Count returns the number of hooks registered for the given type.
func (m *Manager) Count(hookType event.HookType) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hooks[hookType])
}
