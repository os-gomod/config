// Package loader provides infrastructure adapters for loading configuration data
// from various sources (files, environment variables, in-memory). Every loader
// implements the Loader interface and is created via constructor injection —
// there are NO global registries or singletons.
package loader

import (
	"context"
	"fmt"
	"sync/atomic"

	derrors "github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// Loader interface
// ---------------------------------------------------------------------------

// Loader loads configuration data from a source. Implementations must be
// safe for concurrent use. Every Loader is created via a constructor and
// injected into the application — no global registries.
type Loader interface {
	// Load reads the configuration from the source and returns the data.
	Load(ctx context.Context) (map[string]value.Value, error)
	// Watch returns a channel of config change events. The channel is closed
	// when ctx is cancelled or the loader is closed.
	Watch(ctx context.Context) (<-chan event.Event, error)
	// Priority returns the merge priority (higher wins).
	Priority() int
	// String returns a human-readable name for this loader.
	String() string
	// Close releases any resources held by the loader.
	Close(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

// Factory creates a Loader from configuration. Factories are registered
// with an instance-based Registry, never a global variable.
type Factory func(cfg map[string]any) (Loader, error)

// ---------------------------------------------------------------------------
// Base
// ---------------------------------------------------------------------------

// Base provides common loader functionality. Embed this in concrete loaders
// to avoid repeating boilerplate for name, type, priority, and close logic.
type Base struct {
	name     string
	typ      string
	priority int
	closed   int32 // atomic
}

// NewBase creates a new Base with the given name, type, and priority.
func NewBase(name, typ string, priority int) *Base {
	return &Base{
		name:     name,
		typ:      typ,
		priority: priority,
	}
}

// Name returns the loader name.
func (b *Base) Name() string {
	return b.name
}

// Type returns the loader type (e.g., "file", "env", "memory").
func (b *Base) Type() string {
	return b.typ
}

// Priority returns the merge priority.
func (b *Base) Priority() int {
	return b.priority
}

// String returns a human-readable name for this loader.
func (b *Base) String() string {
	return b.name
}

// SetPriority changes the loader priority.
func (b *Base) SetPriority(p int) {
	b.priority = p
}

// IsClosed returns true if the loader has been closed.
func (b *Base) IsClosed() bool {
	return atomic.LoadInt32(&b.closed) == 1
}

// CloseBase marks the loader as closed. Concrete loaders should call
// CloseBase for the atomic flag, then do their own cleanup.
func (b *Base) CloseBase() error {
	atomic.StoreInt32(&b.closed, 1)
	return nil
}

// WrapErr wraps an error with a source name and operation using the domain
// AppError type for structured error handling.
func (b *Base) WrapErr(err error, op string) error {
	if err == nil {
		return nil
	}
	return derrors.New(derrors.CodeSource,
		fmt.Sprintf("loader %q (%s) %s failed", b.name, b.typ, op)).
		WithSource(b.name).
		Wrap(err)
}

// CheckClosed returns an error if the loader is closed, nil otherwise.
func (b *Base) CheckClosed() error {
	if b.IsClosed() {
		return derrors.New(derrors.CodeClosed,
			fmt.Sprintf("loader %q is closed", b.name)).
			WithSource(b.name)
	}
	return nil
}
