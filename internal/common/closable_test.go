package common

import (
	"context"
	"testing"
	"time"
)

func TestNewClosable(t *testing.T) {
	c := NewClosable()
	if c == nil {
		t.Fatal("expected non-nil closable")
	}
	if c.IsClosed() {
		t.Fatal("new closable should not be closed")
	}
}

func TestClosable_Close(t *testing.T) {
	c := NewClosable()
	err := c.Close(context.TODO())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.IsClosed() {
		t.Fatal("should be closed after Close")
	}
}

func TestClosable_IsClosed(t *testing.T) {
	t.Run("before close", func(t *testing.T) {
		c := NewClosable()
		if c.IsClosed() {
			t.Fatal("should not be closed initially")
		}
	})

	t.Run("after close", func(t *testing.T) {
		c := NewClosable()
		_ = c.Close(context.TODO())
		if !c.IsClosed() {
			t.Fatal("should be closed after Close")
		}
	})
}

func TestClosable_IdempotentClose(t *testing.T) {
	c := NewClosable()
	// Close multiple times should not panic
	_ = c.Close(context.TODO())
	_ = c.Close(context.TODO())
	_ = c.Close(context.TODO())
	if !c.IsClosed() {
		t.Fatal("should still be closed")
	}
}

func TestClosable_Done(t *testing.T) {
	t.Run("not closed initially", func(t *testing.T) {
		c := NewClosable()
		done := c.Done()
		if done == nil {
			t.Fatal("expected non-nil channel")
		}
		select {
		case <-done:
			t.Fatal("done should not be closed initially")
		default:
			// expected
		}
	})

	t.Run("closed after Close", func(t *testing.T) {
		c := NewClosable()
		done := c.Done()
		_ = c.Close(context.TODO())

		select {
		case <-done:
			// expected
		case <-time.After(100 * time.Millisecond):
			t.Fatal("done should be closed after Close")
		}
	})

	t.Run("done channel is consistent", func(t *testing.T) {
		c := NewClosable()
		d1 := c.Done()
		d2 := c.Done()
		if d1 != d2 {
			t.Fatal("Done() should return the same channel")
		}
	})
}
