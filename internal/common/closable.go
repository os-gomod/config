// Package common provides shared utilities used across the configuration library,
// including the Closable lifecycle management type.
package common

import (
	"context"
	"sync"
)

// Closable provides a simple thread-safe lifecycle management mechanism.
// It uses a done channel and sync.Once to ensure Close can be called
// multiple times safely. Components embed *Closable and check IsClosed
// or use the Done channel to detect shutdown.
type Closable struct {
	done      chan struct{}
	closeOnce sync.Once
}

// NewClosable creates a new Closable instance.
func NewClosable() *Closable {
	return &Closable{done: make(chan struct{})}
}

// IsClosed reports whether the Closable has been closed.
func (c *Closable) IsClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Done returns a channel that is closed when the Closable is closed.
// Use this with select to detect shutdown.
func (c *Closable) Done() <-chan struct{} { return c.done }

// Close closes the done channel. It is safe to call multiple times;
// subsequent calls are no-ops.
func (c *Closable) Close(_ context.Context) error {
	c.closeOnce.Do(func() { close(c.done) })
	return nil
}
