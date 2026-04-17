package pollwatch

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

func TestNewController(t *testing.T) {
	t.Run("default buffer size", func(t *testing.T) {
		c := NewController(0)
		if c == nil {
			t.Fatal("NewController returned nil")
		}
		if c.IsClosed() {
			t.Error("new controller should not be closed")
		}
	})

	t.Run("custom buffer size", func(t *testing.T) {
		c := NewController(50)
		if c == nil {
			t.Fatal("NewController returned nil")
		}
		if c.IsClosed() {
			t.Error("new controller should not be closed")
		}
	})

	t.Run("negative buffer size uses default", func(t *testing.T) {
		c := NewController(-1)
		if c == nil {
			t.Fatal("NewController returned nil")
		}
	})
}

func TestController_IsClosed(t *testing.T) {
	t.Run("not closed initially", func(t *testing.T) {
		c := NewController(10)
		if c.IsClosed() {
			t.Error("controller should not be closed initially")
		}
	})

	t.Run("closed after Close", func(t *testing.T) {
		c := NewController(10)
		c.Close()
		if !c.IsClosed() {
			t.Error("controller should be closed after Close()")
		}
	})
}

func TestController_Close_Idempotent(t *testing.T) {
	t.Run("multiple closes do not panic", func(t *testing.T) {
		c := NewController(10)
		c.Close()
		c.Close()
		c.Close()
		if !c.IsClosed() {
			t.Error("controller should be closed")
		}
	})
}

func TestController_Done(t *testing.T) {
	t.Run("done channel closed after Close", func(t *testing.T) {
		c := NewController(10)
		select {
		case <-c.Done():
			t.Error("Done() channel should not be closed initially")
		default:
		}

		c.Close()

		select {
		case <-c.Done():
			// expected
		default:
			t.Error("Done() channel should be closed after Close()")
		}
	})
}

func TestController_StartPolling(t *testing.T) {
	t.Run("callback invoked", func(t *testing.T) {
		c := NewController(10)
		defer c.Close()

		var count atomic.Int32
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := c.StartPolling(ctx, 10*time.Millisecond, func(ctx context.Context) {
			count.Add(1)
		})

		// Wait for at least a few invocations
		time.Sleep(100 * time.Millisecond)
		cancel()

		if got := count.Load(); got < 2 {
			t.Fatalf("expected at least 2 callback invocations, got %d", got)
		}
		if ch == nil {
			t.Error("expected non-nil event channel")
		}
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		c := NewController(10)
		defer c.Close()

		var count atomic.Int32
		ctx, cancel := context.WithCancel(context.Background())

		c.StartPolling(ctx, 10*time.Millisecond, func(ctx context.Context) {
			count.Add(1)
		})

		// Let a few invocations happen
		time.Sleep(50 * time.Millisecond)
		cancel()
		time.Sleep(50 * time.Millisecond)

		countBefore := count.Load()
		time.Sleep(50 * time.Millisecond)
		countAfter := count.Load()

		if countAfter != countBefore {
			t.Error("callback should not be invoked after context cancellation")
		}
	})

	t.Run("stops on controller close", func(t *testing.T) {
		c := NewController(10)

		var count atomic.Int32
		ctx := context.Background()

		c.StartPolling(ctx, 10*time.Millisecond, func(ctx context.Context) {
			count.Add(1)
		})

		time.Sleep(50 * time.Millisecond)
		c.Close()
		time.Sleep(50 * time.Millisecond)

		countBefore := count.Load()
		time.Sleep(50 * time.Millisecond)
		countAfter := count.Load()

		if countAfter != countBefore {
			t.Error("callback should not be invoked after controller close")
		}
	})
}

func TestController_EventCh(t *testing.T) {
	t.Run("can publish events through EventCh", func(t *testing.T) {
		c := NewController(10)
		defer c.Close()

		c.EventCh() <- event.New(event.TypeCreate, "test.key")

		// Read from the internal channel (same package access)
		select {
		case evt := <-c.eventCh:
			if evt.Key != "test.key" {
				t.Errorf("expected key 'test.key', got %q", evt.Key)
			}
		case <-time.After(50 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})
}

func TestController_EmitDiff(t *testing.T) {
	t.Run("publishes diff events", func(t *testing.T) {
		c := NewController(100)
		defer c.Close()

		oldData := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 0),
		}
		newData := map[string]value.Value{
			"a": value.New("updated", value.TypeString, value.SourceMemory, 0),
			"b": value.New("new", value.TypeString, value.SourceMemory, 0),
		}

		err := c.EmitDiff(context.Background(), oldData, newData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read events from channel
		events := readEvents(c, 2, 100*time.Millisecond)
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}

		typeCount := map[event.Type]int{}
		for _, e := range events {
			typeCount[e.Type]++
		}
		if typeCount[event.TypeUpdate] != 1 {
			t.Errorf("expected 1 update, got %d", typeCount[event.TypeUpdate])
		}
		if typeCount[event.TypeCreate] != 1 {
			t.Errorf("expected 1 create, got %d", typeCount[event.TypeCreate])
		}
	})

	t.Run("no changes publishes nothing", func(t *testing.T) {
		c := NewController(100)
		defer c.Close()

		data := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 0),
		}

		err := c.EmitDiff(context.Background(), data, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		events := readEvents(c, 1, 50*time.Millisecond)
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("returns context error on cancellation", func(t *testing.T) {
		c := NewController(1) // tiny buffer
		defer c.Close()

		// Fill the buffer
		c.EventCh() <- event.New(event.TypeCreate, "block")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		oldData := map[string]value.Value{}
		newData := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 0),
		}

		err := c.EmitDiff(ctx, oldData, newData)
		if err == nil {
			t.Fatal("expected error from cancelled context")
		}
	})

	t.Run("returns nil on closed controller", func(t *testing.T) {
		c := NewController(10)
		c.Close()

		oldData := map[string]value.Value{}
		newData := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 0),
		}

		err := c.EmitDiff(context.Background(), oldData, newData)
		if err != nil {
			t.Fatalf("expected nil error on closed controller, got %v", err)
		}
	})

	t.Run("passes options to events", func(t *testing.T) {
		c := NewController(100)
		defer c.Close()

		oldData := map[string]value.Value{}
		newData := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 0),
		}

		err := c.EmitDiff(context.Background(), oldData, newData, event.WithTraceID("trace-abc"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		events := readEvents(c, 1, 100*time.Millisecond)
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].TraceID != "trace-abc" {
			t.Errorf("expected trace ID 'trace-abc', got %q", events[0].TraceID)
		}
	})
}

// readEvents reads up to max events from the controller's internal event channel.
// Uses c.eventCh directly since EventCh() returns a write-only channel.
func readEvents(c *Controller, max int, timeout time.Duration) []event.Event {
	var events []event.Event
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; i < max; i++ {
		select {
		case evt := <-c.eventCh:
			events = append(events, evt)
		case <-timer.C:
			return events
		}
	}
	return events
}
