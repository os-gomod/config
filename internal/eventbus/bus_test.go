package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/config/v2/internal/domain/event"
)

// ---------------------------------------------------------------------------
// Bus creation
// ---------------------------------------------------------------------------

func TestNewBus(t *testing.T) {
	bus := NewBus()
	require.NotNil(t, bus)
	defer bus.Close()

	stats := bus.Stats()
	assert.Equal(t, uint64(0), stats.Delivered)
	assert.Equal(t, uint64(0), stats.Dropped)
	assert.Equal(t, uint64(0), stats.Failed)
	assert.Equal(t, 0, stats.Subscribers)
}

func TestNewBus_WithOptions(t *testing.T) {
	bus := NewBus(
		WithWorkerCount(2),
		WithQueueSize(10),
		WithRetryCount(3),
		WithRetryDelay(50*time.Millisecond),
	)
	defer bus.Close()

	stats := bus.Stats()
	assert.Equal(t, 0, stats.Subscribers)
}

// ---------------------------------------------------------------------------
// Subscribe and Publish (async)
// ---------------------------------------------------------------------------

func TestSubscribeAndPublish(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var received atomic.Int32
	unsub := bus.Subscribe("config.changed", func(ctx context.Context, evt event.Event) error {
		received.Add(1)
		assert.Equal(t, "config.changed", evt.Key)
		return nil
	})

	evt := event.New(event.TypeUpdate, "config.changed")
	err := bus.Publish(context.Background(), &evt)
	require.NoError(t, err)

	// Wait for async delivery
	require.Eventually(t, func() bool {
		return received.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Unsubscribe
	unsub()

	received.Store(0)
	err = bus.Publish(context.Background(), &evt)
	require.NoError(t, err)

	// Should not receive after unsubscribe
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), received.Load())
}

// ---------------------------------------------------------------------------
// Pattern matching
// ---------------------------------------------------------------------------

func TestSubscribe_PatternMatching(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var received atomic.Int32
	bus.Subscribe("app.*.config", func(ctx context.Context, evt event.Event) error {
		received.Add(1)
		return nil
	})

	// Should match
	bus.Publish(context.Background(), &event.Event{Key: "app.db.config"})
	require.Eventually(t, func() bool { return received.Load() >= 1 }, 2*time.Second, 10*time.Millisecond)

	// Should not match (wrong segment count)
	bus.Publish(context.Background(), &event.Event{Key: "app.config"})
	bus.Publish(context.Background(), &event.Event{Key: "app.db.extra.config"})

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), received.Load())
}

// ---------------------------------------------------------------------------
// Catch-all subscription
// ---------------------------------------------------------------------------

func TestSubscribe_CatchAll(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var received atomic.Int32
	bus.Subscribe("", func(ctx context.Context, evt event.Event) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), &event.Event{Key: "anything"})
	require.Eventually(t, func() bool { return received.Load() >= 1 }, 2*time.Second, 10*time.Millisecond)
}

func TestSubscribe_CatchAll_Star(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var received atomic.Int32
	bus.Subscribe("*", func(ctx context.Context, evt event.Event) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), &event.Event{Key: "anything"})
	require.Eventually(t, func() bool { return received.Load() >= 1 }, 2*time.Second, 10*time.Millisecond)
}

// ---------------------------------------------------------------------------
// Unsubscribe
// ---------------------------------------------------------------------------

func TestUnsubscribe(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var count atomic.Int32
	unsub := bus.Subscribe("test.key", func(ctx context.Context, evt event.Event) error {
		count.Add(1)
		return nil
	})

	bus.Publish(context.Background(), &event.Event{Key: "test.key"})
	require.Eventually(t, func() bool { return count.Load() >= 1 }, 2*time.Second, 10*time.Millisecond)

	unsub()

	bus.Publish(context.Background(), &event.Event{Key: "test.key"})
	time.Sleep(200 * time.Millisecond)
	// Count should not increase after unsubscribe
	assert.Equal(t, int32(1), count.Load())
}

func TestUnsubscribe_NoSubscribers(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	// Subscribe then immediately unsubscribe
	unsub := bus.Subscribe("test", func(ctx context.Context, evt event.Event) error {
		return nil
	})
	unsub()

	stats := bus.Stats()
	assert.Equal(t, 0, stats.Subscribers)
}

// ---------------------------------------------------------------------------
// Sync delivery
// ---------------------------------------------------------------------------

func TestPublishSync(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var received atomic.Int32
	bus.Subscribe("sync.test", func(ctx context.Context, evt event.Event) error {
		received.Add(1)
		return nil
	})

	evt := event.Event{Key: "sync.test"}
	err := bus.PublishSync(context.Background(), &evt)
	require.NoError(t, err)
	assert.Equal(t, int32(1), received.Load())
}

func TestPublishSync_MultipleSubscribers(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var count atomic.Int32
	bus.Subscribe("multi", func(ctx context.Context, evt event.Event) error {
		count.Add(1)
		return nil
	})
	bus.Subscribe("multi", func(ctx context.Context, evt event.Event) error {
		count.Add(1)
		return nil
	})

	err := bus.PublishSync(context.Background(), &event.Event{Key: "multi"})
	require.NoError(t, err)
	assert.Equal(t, int32(2), count.Load())
}

// ---------------------------------------------------------------------------
// Queue back-pressure (drop when full)
// ---------------------------------------------------------------------------

func TestPublish_BackPressure(t *testing.T) {
	bus := NewBus(
		WithWorkerCount(1),
		WithQueueSize(2),
	)
	defer bus.Close()

	// Create a slow subscriber that blocks the single worker
	blockCh := make(chan struct{})
	bus.Subscribe("slow", func(ctx context.Context, evt event.Event) error {
		<-blockCh // block forever
		return nil
	})

	// Fill the queue
	bus.Publish(context.Background(), &event.Event{Key: "slow"})

	// Give it a moment to start processing
	time.Sleep(100 * time.Millisecond)

	// Now the worker is blocked on blockCh, queue should be empty.
	// Fill the queue to capacity.
	bus.Publish(context.Background(), &event.Event{Key: "slow"})
	bus.Publish(context.Background(), &event.Event{Key: "slow"})

	// Next publish should fail because queue is full
	err := bus.Publish(context.Background(), &event.Event{Key: "slow"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue is full")

	// Check stats
	stats := bus.Stats()
	assert.Greater(t, stats.Dropped, uint64(0))

	close(blockCh)
}

// ---------------------------------------------------------------------------
// Stats tracking
// ---------------------------------------------------------------------------

func TestStats_Subscribers(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	bus.Subscribe("a", func(ctx context.Context, evt event.Event) error { return nil })
	bus.Subscribe("b", func(ctx context.Context, evt event.Event) error { return nil })
	bus.Subscribe("a", func(ctx context.Context, evt event.Event) error { return nil }) // second sub for "a"

	stats := bus.Stats()
	assert.Equal(t, 3, stats.Subscribers)
}

// ---------------------------------------------------------------------------
// Graceful shutdown
// ---------------------------------------------------------------------------

func TestClose_Idempotent(t *testing.T) {
	bus := NewBus()
	bus.Close()
	bus.Close() // Should not panic
	bus.Close()
}

func TestClose_RejectsPublish(t *testing.T) {
	bus := NewBus()
	bus.Close()

	err := bus.Publish(context.Background(), &event.Event{Key: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bus is closed")
}

func TestClose_RejectsPublishSync(t *testing.T) {
	bus := NewBus()
	bus.Close()

	err := bus.PublishSync(context.Background(), &event.Event{Key: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bus is closed")
}

func TestClose_PublishNilEvent(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	err := bus.Publish(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be nil")
}

// ---------------------------------------------------------------------------
// Panic recovery in observers
// ---------------------------------------------------------------------------

func TestPanicRecovery(t *testing.T) {
	var recovered atomic.Value
	bus := NewBus(
		WithWorkerCount(4),
		WithQueueSize(100),
		WithPanicHandler(func(r any) {
			recovered.Store(r)
		}),
	)
	defer bus.Close()

	bus.Subscribe("panic.test", func(ctx context.Context, evt event.Event) error {
		panic("observer panic!")
	})

	bus.PublishSync(context.Background(), &event.Event{Key: "panic.test"})
	// The bus should not crash; the panic should be recovered
	time.Sleep(50 * time.Millisecond)

	val := recovered.Load()
	assert.Equal(t, "observer panic!", val)
}

// ---------------------------------------------------------------------------
// No subscribers (no-op)
// ---------------------------------------------------------------------------

func TestPublish_NoSubscribers(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	err := bus.Publish(context.Background(), &event.Event{Key: "nobody.listening"})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Ordered publish
// ---------------------------------------------------------------------------

func TestPublishOrdered(t *testing.T) {
	bus := NewBus(WithWorkerCount(4), WithQueueSize(100))
	defer bus.Close()

	var mu sync.Mutex
	var order []string
	bus.Subscribe("ordered.*", func(ctx context.Context, evt event.Event) error {
		mu.Lock()
		order = append(order, evt.Key)
		mu.Unlock()
		return nil
	})

	events := []event.Event{
		{Key: "ordered.1"},
		{Key: "ordered.2"},
		{Key: "ordered.3"},
	}

	err := bus.PublishOrdered(context.Background(), events)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(order) == 3
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"ordered.1", "ordered.2", "ordered.3"}, order)
}

func TestPublishOrdered_Empty(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	err := bus.PublishOrdered(context.Background(), nil)
	require.NoError(t, err)
}

func TestPublishOrdered_Cancelled(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := bus.PublishOrdered(ctx, []event.Event{
		{Key: "a"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestConcurrent_SubscribeAndPublish(t *testing.T) {
	bus := NewBus(WithWorkerCount(8), WithQueueSize(1000))
	defer bus.Close()

	var wg sync.WaitGroup
	var count atomic.Int32

	// Concurrently subscribe and publish
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pat := "key"
			bus.Subscribe(pat, func(ctx context.Context, evt event.Event) error {
				count.Add(1)
				return nil
			})
			bus.Publish(context.Background(), &event.Event{Key: pat})
		}(i)
	}

	wg.Wait()

	// Wait for async delivery to settle
	time.Sleep(500 * time.Millisecond)
	assert.GreaterOrEqual(t, count.Load(), int32(1))
}
