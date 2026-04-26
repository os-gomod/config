package watcher

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebouncer_RunCoalescesToSingleExecution(t *testing.T) {
	t.Parallel()

	d := NewDebouncer(25 * time.Millisecond)

	var calls atomic.Int32
	done := make(chan struct{}, 1)

	d.Run(func() {
		calls.Add(1)
	})
	d.Run(func() {
		calls.Add(1)
		done <- struct{}{}
	})

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debounced call")
	}

	assert.Equal(t, int32(1), calls.Load())
	assert.False(t, d.Pending())
}

func TestDebouncer_StopCancelsPendingCall(t *testing.T) {
	t.Parallel()

	d := NewDebouncer(50 * time.Millisecond)

	var calls atomic.Int32
	d.Run(func() {
		calls.Add(1)
	})

	require.True(t, d.Pending())
	assert.True(t, d.Stop())

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(0), calls.Load())
	assert.False(t, d.Pending())
}
