package benchmark

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/os-gomod/config/event"
)

// ---------------------------------------------------------------------------
// BenchmarkBusPublishSingleSub
// ---------------------------------------------------------------------------

func BenchmarkBusPublishSingleSub(b *testing.B) {
	bus := event.NewBus()
	bus.Subscribe("*", func(_ context.Context, _ event.Event) error { return nil })
	evt := event.New(event.TypeCreate, "bench.key")
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bus.Publish(ctx, &evt)
	}
}

// ---------------------------------------------------------------------------
// BenchmarkBusPublishManySub
// ---------------------------------------------------------------------------

func BenchmarkBusPublishManySub(b *testing.B) {
	ctx := context.Background()

	b.Run("10 subscribers", func(b *testing.B) {
		bus := event.NewBus()
		var ready atomic.Int64
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				bus.Subscribe("*", func(_ context.Context, _ event.Event) error {
					ready.Add(1)
					return nil
				})
			}()
		}
		wg.Wait()
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bus.Publish(ctx, &evt)
		}
	})

	b.Run("100 subscribers", func(b *testing.B) {
		bus := event.NewBus()
		for i := 0; i < 100; i++ {
			bus.Subscribe("*", func(_ context.Context, _ event.Event) error { return nil })
		}
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bus.Publish(ctx, &evt)
		}
	})

	b.Run("1000 subscribers", func(b *testing.B) {
		bus := event.NewBus()
		for i := 0; i < 1000; i++ {
			bus.Subscribe("*", func(_ context.Context, _ event.Event) error { return nil })
		}
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bus.Publish(ctx, &evt)
		}
	})
}

// ---------------------------------------------------------------------------
// BenchmarkBusPublishSync
// ---------------------------------------------------------------------------

func BenchmarkBusPublishSync(b *testing.B) {
	ctx := context.Background()

	b.Run("1 subscriber", func(b *testing.B) {
		bus := event.NewBus()
		bus.Subscribe("bench.key", func(_ context.Context, _ event.Event) error { return nil })
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.PublishSync(ctx, &evt)
		}
	})

	b.Run("10 subscribers", func(b *testing.B) {
		bus := event.NewBus()
		for i := 0; i < 10; i++ {
			bus.Subscribe("bench.key", func(_ context.Context, _ event.Event) error { return nil })
		}
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.PublishSync(ctx, &evt)
		}
	})

	b.Run("100 subscribers", func(b *testing.B) {
		bus := event.NewBus()
		for i := 0; i < 100; i++ {
			bus.Subscribe("bench.key", func(_ context.Context, _ event.Event) error { return nil })
		}
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.PublishSync(ctx, &evt)
		}
	})
}
