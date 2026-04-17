// Package hooks provides a lifecycle hook system for configuration operations.
// Hooks can be registered for before/after operations such as reload, set,
// delete, validate, and close. Each hook has a priority that controls execution
// order (lower values run first).
package hooks

import (
	"context"
	"time"

	"github.com/os-gomod/config/core/value"
)

// Context carries the state and metadata for a hook execution. It provides
// access to the current operation, affected key, old/new values, old/new
// configuration states, and optional metadata.
type Context struct {
	Operation   string
	Key         string
	Value       any
	OldValue    any
	NewValue    any
	OldState    *value.State
	NewState    *value.State
	Metadata    map[string]any
	StartTime   time.Time
	BatchValues map[string]any
}

// OldStateSafe returns the old configuration state, or an empty state if nil.
func (c *Context) OldStateSafe() *value.State {
	if c.OldState == nil {
		return value.NewState(nil, 0)
	}
	return c.OldState
}

// NewStateSafe returns the new configuration state, or an empty state if nil.
func (c *Context) NewStateSafe() *value.State {
	if c.NewState == nil {
		return value.NewState(nil, 0)
	}
	return c.NewState
}

// Hook is the interface for lifecycle hooks. Each hook has a name, priority,
// and an Execute method that is called with the hook context.
type Hook interface {
	// Name returns the unique identifier for this hook.
	Name() string
	// Priority returns the execution priority (lower values run first).
	Priority() int
	// Execute runs the hook logic with the given context.
	Execute(ctx context.Context, hctx *Context) error
}

// HookFunc is a convenience adapter that turns a function into a Hook implementation.
type HookFunc struct {
	name     string
	priority int
	fn       func(context.Context, *Context) error
}

// New creates a new HookFunc with the given name, priority, and callback function.
func New(name string, priority int, fn func(context.Context, *Context) error) *HookFunc {
	return &HookFunc{name: name, priority: priority, fn: fn}
}

// Name returns the hook name.
func (h *HookFunc) Name() string { return h.name }

// Priority returns the hook priority.
func (h *HookFunc) Priority() int { return h.priority }

// Execute calls the underlying hook function.
func (h *HookFunc) Execute(ctx context.Context, hctx *Context) error { return h.fn(ctx, hctx) }
