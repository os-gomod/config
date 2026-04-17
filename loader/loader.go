// Package loader defines the Loader interface and provides built-in loader
// implementations for reading configuration from various sources such as files,
// environment variables, and in-memory data. Loaders are composable and can be
// combined with priorities to control value precedence during merge.
//
// Built-in loaders:
//   - FileLoader: reads from local files with automatic format detection
//   - EnvLoader: reads from OS environment variables with prefix filtering
//   - MemoryLoader: provides in-memory key-value pairs for testing and defaults
//   - PollingLoader: wraps any load function with polling-based watching
//
// Custom loaders can be registered via the Registry for dynamic instantiation.
package loader

import (
	"context"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// Loader is the interface for configuration sources. Each loader provides a way
// to load configuration data, optionally watch for changes, and report its
// priority and identity.
type Loader interface {
	// Load reads the current configuration state and returns a flat key-value map.
	// The context can be used for cancellation.
	Load(ctx context.Context) (map[string]value.Value, error)

	// Watch returns a channel of configuration change events. If the loader
	// does not support watching, it should return (nil, nil).
	Watch(ctx context.Context) (<-chan event.Event, error)

	// Priority returns the priority of this loader's values. Higher values
	// take precedence during merge.
	Priority() int

	// String returns a human-readable identifier for this loader.
	String() string

	// Close releases any resources held by the loader. It is safe to call
	// multiple times.
	Close(ctx context.Context) error
}

// Factory is a function that creates a Loader from a configuration map.
// Factories are registered with the Registry and used for dynamic loader
// instantiation.
type Factory func(cfg map[string]any) (Loader, error)

// Base provides shared boilerplate for Loader implementations. It embeds
// common.Closable for lifecycle management and tracks the loader's name,
// type, and priority.
type Base struct {
	*common.Closable
	name     string
	typ      string
	priority int
}

// NewBase creates a new Base with the given name, type, and priority.
func NewBase(name, typ string, priority int) *Base {
	return &Base{
		Closable: common.NewClosable(),
		name:     name,
		typ:      typ,
		priority: priority,
	}
}

// Name returns the loader name.
func (b *Base) Name() string { return b.name }

// Type returns the loader type (e.g., "file", "env", "memory").
func (b *Base) Type() string { return b.typ }

// Priority returns the loader priority.
func (b *Base) Priority() int { return b.priority }

// String returns the loader name.
func (b *Base) String() string { return b.name }

// SetPriority updates the loader priority.
func (b *Base) SetPriority(p int) { b.priority = p }

// WrapErr wraps an error with the loader's source name and operation context.
// Returns nil if err is nil.
func (b *Base) WrapErr(err error, operation string) error {
	if err == nil {
		return nil
	}
	return WrapErrors(err, b.String(), operation)
}
