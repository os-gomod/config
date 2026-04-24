package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ------------------------------------------------------------------
// Closable
// ------------------------------------------------------------------
func TestClosable_New(t *testing.T) {
	c := NewClosable()
	if c == nil {
		t.Fatal("expected non-nil Closable")
	}
	if c.IsClosed() {
		t.Error("expected open")
	}
}

func TestClosable_DoubleClose(t *testing.T) {
	c := NewClosable()
	require.NoError(t, c.Close(context.Background()))
	require.NoError(t, c.Close(context.Background()))
}

func TestClosable_Done_AfterClose(t *testing.T) {
	c := NewClosable()
	c.Close(context.Background())
	done := c.Done()
	select {
	case <-done:
		// expected
	default:
		t.Error("done should be closed immediately after Close")
	}
}

// ------------------------------------------------------------------
// ClientLifecycle
// ------------------------------------------------------------------
func TestClientLifecycle_New(t *testing.T) {
	cl := NewClientLifecycle()
	if cl == nil {
		t.Fatal("expected non-nil ClientLifecycle")
	}
	if cl.IsClosed() {
		t.Error("expected open")
	}
}

func TestClientLifecycle_EnsureOpen(t *testing.T) {
	cl := NewClientLifecycle()
	err := cl.EnsureOpen()
	assert.NoError(t, err)

	cl.Close(context.Background())
	err = cl.EnsureOpen()
	assert.Error(t, err)
}

func TestClientLifecycle_Close(t *testing.T) {
	cl := NewClientLifecycle()
	require.NoError(t, cl.Close(context.Background()))
	assert.True(t, cl.IsClosed())
}

func TestClientLifecycle_DoubleClose(t *testing.T) {
	cl := NewClientLifecycle()
	cl.Close(context.Background())
	cl.Close(context.Background())
}

func TestClientLifecycle_Done(t *testing.T) {
	cl := NewClientLifecycle()
	done := cl.Done()
	if done == nil {
		t.Fatal("expected non-nil done channel")
	}
	cl.Close(context.Background())
	select {
	case <-done:
		// expected
	default:
		t.Error("done should be closed")
	}
}

func TestClientLifecycle_IsClosed(t *testing.T) {
	cl := NewClientLifecycle()
	assert.False(t, cl.IsClosed())
	cl.Close(context.Background())
	assert.True(t, cl.IsClosed())
}
