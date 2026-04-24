// Package config provides a flexible configuration management system with support for
package config

import (
	"context"
	"sync"

	"github.com/os-gomod/config/event"
)

// WatchBinding represents an active watch-pattern-to-struct binding.
// When configuration changes match the pattern, the target struct is
// automatically re-bound with the latest values.
//
// This enables zero-code hot-reload for struct-tagged configuration:
//
//	type DBConfig struct {
//	    Host     string `config:"database.host"`
//	    Port     int    `config:"database.port"`
//	    Password string `config:"database.password"`
//	}
//
//	var cfg DBConfig
//	binding, err := cfg2.WatchAndBind(ctx, "database.*", &cfg)
//	// cfg is now kept in sync automatically
type WatchBinding struct {
	unsubscribe func()
	mu          sync.Mutex
	target      any
	config      *Config
	active      bool
}

// WatchAndBind subscribes to config changes matching the given pattern
// and automatically re-binds the target struct whenever a matching
// change event is received.
//
// Returns a WatchBinding that can be used to stop the automatic binding.
// The binding is active immediately after creation.
func (c *Config) WatchAndBind(ctx context.Context, pat string, target any) (*WatchBinding, error) {
	// Initial bind
	if err := c.Bind(ctx, target); err != nil {
		return nil, err
	}

	wb := &WatchBinding{
		target: target,
		config: c,
		active: true,
	}

	wb.unsubscribe = c.WatchPattern(pat, func(bindCtx context.Context, _ event.Event) error {
		wb.mu.Lock()
		defer wb.mu.Unlock()
		if !wb.active {
			return nil
		}
		return c.Bind(bindCtx, wb.target)
	})

	return wb, nil
}

// Stop deactivates the watch binding and unsubscribes from events.
// After Stop, the target struct will no longer be automatically updated.
func (wb *WatchBinding) Stop() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	if wb.active {
		wb.active = false
		if wb.unsubscribe != nil {
			wb.unsubscribe()
		}
	}
}

// IsActive reports whether the watch binding is still active.
func (wb *WatchBinding) IsActive() bool {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	return wb.active
}
