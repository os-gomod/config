// Package common provides shared low-level utilities used across the config framework.
package common

import (
	"context"
	"sync"
)

// Closable provides a thread-safe, one-time close mechanism backed by a channel.
// Embed or compose Closable to add graceful shutdown semantics to any type.
type Closable struct {
	done      chan struct{}
	closeOnce sync.Once
}

// NewClosable creates a Closable in the open state.
func NewClosable() *Closable {
	return &Closable{done: make(chan struct{})}
}

// IsClosed reports whether Close has been called.
func (c *Closable) IsClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Done returns a channel that is closed when the Closable is closed.
func (c *Closable) Done() <-chan struct{} { return c.done }

// Close idempotently closes the Closable. The context is accepted for
// interface compatibility but is not used; the close is always immediate.
func (c *Closable) Close(_ context.Context) error {
	c.closeOnce.Do(func() { close(c.done) })
	return nil
}
