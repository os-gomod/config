package bench

import (
	"context"
	"fmt"
	"testing"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
)

// generateData creates n key-value pairs.
func generateData(n int) map[string]any {
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("key.%04d", i)] = fmt.Sprintf("value.%04d", i)
	}
	return m
}

// ---------------------------------------------------------------------------
// Engine Reload benchmarks
// ---------------------------------------------------------------------------

func BenchmarkEngine_Reload_1K(b *testing.B)  { benchReload(b, 1000) }
func BenchmarkEngine_Reload_10K(b *testing.B) { benchReload(b, 10000) }
func BenchmarkEngine_Reload_50K(b *testing.B) { benchReload(b, 50000) }

func benchReload(b *testing.B, n int) {
	ctx := context.Background()
	data := generateData(n)
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	layer := core.NewLayer("bench", core.WithLayerSource(ml), core.WithLayerPriority(100))
	eng := core.New(core.WithLayer(layer))
	// Initial load
	_, _ = eng.Reload(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Reload(ctx)
	}
}

// ---------------------------------------------------------------------------
// Delta Reload benchmarks
// ---------------------------------------------------------------------------

func BenchmarkEngine_DeltaReload_10K_NoChanges(b *testing.B) {
	ctx := context.Background()
	data := generateData(10000)
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	layer := core.NewLayer("bench", core.WithLayerSource(ml), core.WithLayerPriority(100))
	eng := core.New(core.WithLayer(layer), core.WithDeltaReload())
	_, _ = eng.Reload(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Reload(ctx)
	}
}

func BenchmarkEngine_DeltaReload_10K_1PctChanges(b *testing.B) {
	ctx := context.Background()
	data := generateData(10000)
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	layer := core.NewLayer("bench", core.WithLayerSource(ml), core.WithLayerPriority(100))
	eng := core.New(core.WithLayer(layer), core.WithDeltaReload())
	_, _ = eng.Reload(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Copy all data and update 1% of keys to simulate a partial change.
		updated := make(map[string]any, 10000)
		for k, v := range data {
			updated[k] = v
		}
		for j := 0; j < 100; j++ {
			updated[fmt.Sprintf("key.%04d", j)] = fmt.Sprintf("updated.%d", i)
		}
		ml.Update(updated)
		_, _ = eng.Reload(ctx)
	}
}

// ---------------------------------------------------------------------------
// Engine Set / BatchSet benchmarks
// ---------------------------------------------------------------------------

func BenchmarkEngine_Set_10K(b *testing.B) {
	ctx := context.Background()
	data := generateData(10000)
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	layer := core.NewLayer("bench", core.WithLayerSource(ml), core.WithLayerPriority(100))
	eng := core.New(core.WithLayer(layer))
	_, _ = eng.Reload(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Set(ctx, fmt.Sprintf("bench.key.%d", i%10000), "newval")
	}
}

func BenchmarkEngine_BatchSet_1K(b *testing.B) {
	ctx := context.Background()
	data := generateData(10000)
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	layer := core.NewLayer("bench", core.WithLayerSource(ml), core.WithLayerPriority(100))
	eng := core.New(core.WithLayer(layer))
	_, _ = eng.Reload(ctx)

	batch := make(map[string]any, 1000)
	for i := 0; i < 1000; i++ {
		batch[fmt.Sprintf("batch.key.%d", i)] = fmt.Sprintf("val.%d", i)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = eng.BatchSet(ctx, batch)
	}
}

// ---------------------------------------------------------------------------
// Engine Get / GetAll benchmarks
// ---------------------------------------------------------------------------

func BenchmarkEngine_Get(b *testing.B) {
	ctx := context.Background()
	data := generateData(10000)
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	layer := core.NewLayer("bench", core.WithLayerSource(ml), core.WithLayerPriority(100))
	eng := core.New(core.WithLayer(layer))
	_, _ = eng.Reload(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Get(fmt.Sprintf("key.%04d", i%10000))
	}
}

func BenchmarkEngine_GetAll_10K(b *testing.B) {
	ctx := context.Background()
	data := generateData(10000)
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	layer := core.NewLayer("bench", core.WithLayerSource(ml), core.WithLayerPriority(100))
	eng := core.New(core.WithLayer(layer))
	_, _ = eng.Reload(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = eng.GetAll()
	}
}

// ---------------------------------------------------------------------------
// Value-level benchmarks
// ---------------------------------------------------------------------------

func BenchmarkValue_Merge_10K(b *testing.B) {
	m1 := make(map[string]value.Value, 10000)
	m2 := make(map[string]value.Value, 5000)
	for i := 0; i < 10000; i++ {
		m1[fmt.Sprintf("a.%04d", i)] = value.NewInMemory(fmt.Sprintf("v.%d", i))
	}
	for i := 0; i < 5000; i++ {
		m2[fmt.Sprintf("b.%04d", i)] = value.NewInMemory(fmt.Sprintf("v.%d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = value.Merge(m1, m2)
	}
}

func BenchmarkValue_Checksum_10K(b *testing.B) {
	m := make(map[string]value.Value, 10000)
	for i := 0; i < 10000; i++ {
		m[fmt.Sprintf("key.%04d", i)] = value.NewInMemory(fmt.Sprintf("value.%04d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		value.ComputeChecksum(m)
	}
}

func BenchmarkValue_Diff_10K(b *testing.B) {
	old := make(map[string]value.Value, 10000)
	newData := make(map[string]value.Value, 10000)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key.%04d", i)
		old[key] = value.NewInMemory(fmt.Sprintf("old.%d", i))
		if i%100 == 0 {
			newData[key] = value.NewInMemory(fmt.Sprintf("new.%d", i))
		} else {
			newData[key] = old[key]
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		value.ComputeDiff(old, newData)
	}
}

// ---------------------------------------------------------------------------
// Config-level benchmarks
// ---------------------------------------------------------------------------

func BenchmarkConfig_New_10K(b *testing.B) {
	ctx := context.Background()
	data := generateData(10000)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
		cfg, _ := config.New(ctx, config.WithLoader(ml))
		_ = cfg.Close(ctx)
	}
}

func BenchmarkConfig_Bind_10K(b *testing.B) {
	ctx := context.Background()
	data := make(map[string]any, 3)
	data["name"] = "localhost"
	data["port"] = "8080"
	data["host"] = "0.0.0.0"
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	cfg, _ := config.New(ctx, config.WithLoader(ml))

	type testStruct struct {
		Name string `config:"name"`
		Port int    `config:"port"`
		Host string `config:"host"`
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var ts testStruct
		_ = cfg.Bind(ctx, &ts)
	}
}

// ---------------------------------------------------------------------------
// EventBus benchmarks
// ---------------------------------------------------------------------------

func BenchmarkEventBus_Publish_1Subscriber(b *testing.B) {
	ctx := context.Background()
	bus := event.NewBus()
	bus.Subscribe("", func(_ context.Context, _ event.Event) error { return nil })
	evt := event.NewCreateEvent("key", value.NewInMemory("val"))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bus.Publish(ctx, &evt)
	}
}

func BenchmarkEventBus_Publish_10Subscribers(b *testing.B) {
	ctx := context.Background()
	bus := event.NewBus()
	for i := 0; i < 10; i++ {
		bus.Subscribe("", func(_ context.Context, _ event.Event) error { return nil })
	}
	evt := event.NewCreateEvent("key", value.NewInMemory("val"))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bus.Publish(ctx, &evt)
	}
}

func BenchmarkEventBus_DiffEvents_10K(b *testing.B) {
	old := make(map[string]value.Value, 10000)
	newData := make(map[string]value.Value, 10000)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key.%04d", i)
		old[key] = value.NewInMemory(fmt.Sprintf("old.%d", i))
		newData[key] = value.NewInMemory(fmt.Sprintf("new.%d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = event.NewDiffEvents(old, newData)
	}
}
