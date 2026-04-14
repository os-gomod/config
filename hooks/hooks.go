// Package hooks defines the Hook interface and related types for lifecycle hooks in the config system.
package hooks

import (
	"context"
	"time"

	"github.com/os-gomod/config/core/value"
)

// Context carries contextual information to hook executions, including the
// operation being performed, the key(s) affected, and before/after state.
type Context struct {
	// Operation is the name of the lifecycle operation (e.g. "set", "delete", "reload").
	Operation string
	// Key is the config key affected, if applicable.
	Key string
	// Value is the raw value involved in the operation.
	Value any
	// OldValue is the previous value for update/delete operations.
	OldValue any
	// NewValue is the new value for create/update operations.
	NewValue any
	// OldState is the state snapshot before the operation.
	OldState *value.State
	// NewState is the state snapshot after the operation.
	NewState *value.State
	// Metadata carries arbitrary data for hook consumers.
	Metadata map[string]any
	// StartTime records when the operation began.
	StartTime time.Time
	// BatchValues holds the full key-value map for batch operations.
	BatchValues map[string]any
}

// OldStateSafe returns OldState if non-nil, otherwise an empty State.
func (c *Context) OldStateSafe() *value.State {
	if c.OldState == nil {
		return value.NewState(nil, 0)
	}
	return c.OldState
}

// NewStateSafe returns NewState if non-nil, otherwise an empty State.
func (c *Context) NewStateSafe() *value.State {
	if c.NewState == nil {
		return value.NewState(nil, 0)
	}
	return c.NewState
}

// Hook is a lifecycle hook that can be registered and executed by the Manager.
type Hook interface {
	// Name returns the hook's identifier for logging and diagnostics.
	Name() string
	// Priority returns the execution priority; lower values run first.
	Priority() int
	// Execute runs the hook logic. Returning a non-nil error halts
	// execution of subsequent hooks.
	Execute(ctx context.Context, hctx *Context) error
}

// HookFunc is an adapter that wraps a function as a Hook.
type HookFunc struct {
	name     string
	priority int
	fn       func(context.Context, *Context) error
}

// New creates a HookFunc with the given name, priority, and function.
func New(name string, priority int, fn func(context.Context, *Context) error) *HookFunc {
	return &HookFunc{name: name, priority: priority, fn: fn}
}

// Name returns the hook's name.
func (h *HookFunc) Name() string { return h.name }

// Priority returns the hook's execution priority.
func (h *HookFunc) Priority() int { return h.priority }

// Execute calls the wrapped function.
func (h *HookFunc) Execute(ctx context.Context, hctx *Context) error { return h.fn(ctx, hctx) }
