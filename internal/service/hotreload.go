package service

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/watcher"
)

// HotReloadConfig configures the hot reload behavior
type HotReloadConfig struct {
	// DebounceDuration prevents too-frequent reloads (default: 500ms)
	DebounceDuration time.Duration
	// OnReloadError is called when a reload fails
	OnReloadError func(error)
	// OnReloadSuccess is called when a reload succeeds with change count
	OnReloadSuccess func(eventsCount int)
}

// DefaultHotReloadConfig returns sensible defaults
func DefaultHotReloadConfig() HotReloadConfig {
	return HotReloadConfig{
		DebounceDuration: 500 * time.Millisecond,
	}
}

// HotReloader provides automatic configuration reloading using the existing
// watcher infrastructure - NOT polling. This integrates with the event bus
// and file watchers already present in the core.
type HotReloader struct {
	mu           sync.RWMutex
	config       HotReloadConfig
	debouncer    *watcher.Debouncer
	reloadFn     func(ctx context.Context) (int, error)
	started      bool
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewHotReloader creates a new HotReloader that uses the existing event system.
// It does NOT create its own polling mechanism - it reacts to events from the
// existing watcher infrastructure.
func NewHotReloader(reloadFn func(ctx context.Context) (int, error), cfg HotReloadConfig) *HotReloader {
	if cfg.DebounceDuration <= 0 {
		cfg.DebounceDuration = 500 * time.Millisecond
	}

	return &HotReloader{
		config:    cfg,
		debouncer: watcher.NewDebouncer(cfg.DebounceDuration),
		reloadFn:  reloadFn,
	}
}

// Start begins listening for reload-triggering events.
// EventCh should be the merged channel from watcher.Manager.
func (hr *HotReloader) Start(ctx context.Context, eventCh <-chan event.Event) {
	hr.mu.Lock()
	if hr.started {
		hr.mu.Unlock()
		return
	}
	hr.started = true
	ctx, hr.cancel = context.WithCancel(ctx)
	hr.mu.Unlock()

	hr.wg.Add(1)
	go hr.eventLoop(ctx, eventCh)
}

// Stop halts the hot reloader.
func (hr *HotReloader) Stop() {
	hr.mu.Lock()
	if hr.cancel != nil {
		hr.cancel()
	}
	hr.mu.Unlock()
	hr.wg.Wait()

	hr.mu.Lock()
	hr.started = false
	hr.mu.Unlock()
}

func (hr *HotReloader) eventLoop(ctx context.Context, eventCh <-chan event.Event) {
	defer hr.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-eventCh:
			if !ok {
				return
			}
			hr.handleEvent(ctx, evt)
		}
	}
}

func (hr *HotReloader) handleEvent(ctx context.Context, evt event.Event) {
	// Only trigger reload on certain event types that indicate configuration changes
	if !hr.shouldTriggerReload(evt) {
		return
	}

	hr.debouncer.Run(func() {
		eventsCount, err := hr.reloadFn(ctx)
		if err != nil {
			if hr.config.OnReloadError != nil {
				hr.config.OnReloadError(err)
			}
			return
		}
		if hr.config.OnReloadSuccess != nil && eventsCount > 0 {
			hr.config.OnReloadSuccess(eventsCount)
		}
	})
}

func (hr *HotReloader) shouldTriggerReload(evt event.Event) bool {
	switch evt.EventType {
	case event.TypeReload, event.TypeUpdate, event.TypeCreate, event.TypeDelete:
		return true
	default:
		return false
	}
}

// Enabled returns whether the reloader is currently running
func (hr *HotReloader) Enabled() bool {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return hr.started
}
