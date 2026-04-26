package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/event"
)

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

// Manager coordinates file watching with debouncing. It merges events from
// multiple sources, debounces rapid changes, and calls a reload function
// when changes settle.
//
// Usage:
//
//	mgr := watcher.NewManager(watcher.WithDefaultInterval(2*time.Second))
//	mgr.SetReloadFn(func() { ... })
//	mgr.Start(ctx)
//
// This is instance-based — NO global manager.
type Manager struct {
	watchMgr        *WatchManager
	debouncer       *Debouncer
	defaultInterval time.Duration
	reloadFn        func()
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	eventCh         <-chan event.Event
	running         bool
}

// ManagerOption configures a Manager during construction.
type ManagerOption func(*Manager)

// WithDefaultInterval sets the default debounce interval.
func WithDefaultInterval(d time.Duration) ManagerOption {
	return func(m *Manager) {
		if d > 0 {
			m.defaultInterval = d
		}
	}
}

// WithWatchManager sets a custom WatchManager.
func WithWatchManager(wm *WatchManager) ManagerOption {
	return func(m *Manager) {
		if wm != nil {
			m.watchMgr = wm
		}
	}
}

// WithDebouncer sets a custom Debouncer.
func WithDebouncer(d *Debouncer) ManagerOption {
	return func(m *Manager) {
		if d != nil {
			m.debouncer = d
		}
	}
}

// NewManager creates a new Manager with the given options.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		watchMgr:        NewWatchManager(),
		debouncer:       NewDebouncer(1 * time.Second),
		defaultInterval: 1 * time.Second,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// SetReloadFn sets the function to call when changes are detected and
// debounced. This function will be called from the event loop goroutine.
func (m *Manager) SetReloadFn(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloadFn = fn
}

// Register adds a named Watcher to the underlying WatchManager.
func (m *Manager) Register(name string, w Watcher) error {
	return m.watchMgr.Register(name, w)
}

// Start begins watching all registered watchers. Events are merged and
// debounced. When changes settle, the reload function is called.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true

	// Create a cancellable context.
	m.ctx, m.cancel = context.WithCancel(ctx)
	currentCtx := m.ctx
	reloadFn := m.reloadFn
	m.mu.Unlock()

	// Start watching all registered watchers.
	merged, err := m.watchMgr.WatchAll(currentCtx) //nolint:contextcheck // derived from parent, safe for lifecycle
	if err != nil {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return err
	}

	m.mu.Lock()
	m.eventCh = merged
	m.mu.Unlock()

	// Start the event loop.
	go m.eventLoop(currentCtx, reloadFn) //nolint:contextcheck // goroutine uses derived context, safe for lifecycle

	return nil
}

// eventLoop reads events from the merged channel and triggers debounced reloads.
func (m *Manager) eventLoop(ctx context.Context, reloadFn func()) {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			m.debouncer.Stop()
			return
		case _, ok := <-m.eventCh:
			if !ok {
				m.debouncer.Stop()
				return
			}
			// Debounce: reset the timer on each event.
			if reloadFn != nil {
				m.debouncer.Run(func() {
					reloadFn()
				})
			}
		}
	}
}

// Stop stops the manager and all underlying watchers.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.debouncer.Stop()
	m.mu.Unlock()

	m.watchMgr.StopAll()
}

// Close stops the manager and closes all underlying watchers.
func (m *Manager) Close() error {
	m.Stop()
	return m.watchMgr.Close()
}

// Running returns true if the manager is currently watching for changes.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// WatchManager returns the underlying WatchManager for advanced usage.
func (m *Manager) WatchManager() *WatchManager {
	return m.watchMgr
}

// Debouncer returns the underlying Debouncer for advanced usage.
func (m *Manager) Debouncer() *Debouncer {
	return m.debouncer
}

// ForceReload triggers the reload function immediately, bypassing the
// debounce timer.
func (m *Manager) ForceReload() {
	m.mu.Lock()
	reloadFn := m.reloadFn
	m.mu.Unlock()

	if reloadFn != nil {
		m.debouncer.Stop()
		reloadFn()
	}
}
