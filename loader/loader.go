// Package loader provides config data sources: files, environment variables,
// and in-memory maps. Remote config stores are in the provider package.
package loader

import (
	"context"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// Loader loads config key-value pairs from a local source.
// Implementations include FileLoader, EnvLoader, and MemoryLoader.
type Loader interface {
	// Load fetches the current config data.
	// It must be safe to call concurrently.
	Load(ctx context.Context) (map[string]value.Value, error)

	// Watch returns a channel that emits events when the source data changes.
	// Implementations that do not support watching return (nil, nil).
	Watch(ctx context.Context) (<-chan event.Event, error)

	// Priority returns the merge priority of this loader.
	// Higher values override lower values during merging.
	Priority() int

	// String returns a human-readable identifier, used in logs and diagnostics.
	String() string

	// Close releases any resources held by the loader.
	Close(ctx context.Context) error
}

// Factory creates a Loader from a generic configuration map.
// Used by the provider registry for dynamic instantiation.
type Factory func(cfg map[string]any) (Loader, error)

// Base provides common fields and behavior shared by all Loader implementations.
type Base struct {
	*common.Closable
	name     string
	typ      string
	priority int
}

// NewBase creates a Base with the given name, type, and priority.
func NewBase(name, typ string, priority int) *Base {
	return &Base{
		Closable: common.NewClosable(),
		name:     name,
		typ:      typ,
		priority: priority,
	}
}

// Name returns the loader's name.
func (b *Base) Name() string { return b.name }

// Type returns the loader's type identifier.
func (b *Base) Type() string { return b.typ }

// Priority returns the loader's merge priority.
func (b *Base) Priority() int { return b.priority }

// String returns the loader's name.
func (b *Base) String() string { return b.name }

// SetPriority sets the merge priority.
func (b *Base) SetPriority(p int) { b.priority = p }

// WrapErr wraps an error with the source name and operation context.
func (b *Base) WrapErr(err error, operation string) error {
	if err == nil {
		return nil
	}
	return WrapErrors(err, b.String(), operation)
}
