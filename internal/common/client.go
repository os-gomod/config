package common

import (
	"context"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// ClientLifecycle manages the lifecycle of a lazily-initialized resource.
// It provides the EnsureOpen guard, mutex management, and close coordination
// that would otherwise be duplicated across every provider and loader.
//
// This is a mixin struct designed to be embedded in types that need
// open/close state tracking with concurrent access protection.
//
// Usage:
//
//	type MyProvider struct {
//	    *common.ClientLifecycle
//	    // ... provider-specific fields
//	}
type ClientLifecycle struct {
	Closable *Closable
	Mu       sync.Mutex
	WatchMu  sync.Mutex
}

// NewClientLifecycle creates a new ClientLifecycle with a fresh Closable.
func NewClientLifecycle() *ClientLifecycle {
	return &ClientLifecycle{
		Closable: NewClosable(),
	}
}

// EnsureOpen returns an error if the lifecycle has been closed.
// Call this at the top of every public method to short-circuit on closed state.
// This is the single point of open-check logic — all providers delegate here
// instead of repeating the check.
func (cl *ClientLifecycle) EnsureOpen() error {
	if cl.Closable.IsClosed() {
		return configerrors.ErrClosed
	}
	return nil
}

// Done returns a channel that is closed when Close is called.
func (cl *ClientLifecycle) Done() <-chan struct{} {
	return cl.Closable.Done()
}

// IsClosed reports whether the lifecycle has been closed.
func (cl *ClientLifecycle) IsClosed() bool {
	return cl.Closable.IsClosed()
}

// Close closes the lifecycle. Safe to call multiple times.
func (cl *ClientLifecycle) Close(ctx context.Context) error {
	cl.Closable.Close(ctx)
	return nil
}
