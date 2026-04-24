package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/loader"
)

func generateLargeData(n int) map[string]any {
	data := make(map[string]any, n)
	for i := 0; i < n; i++ {
		data[fmt.Sprintf("key.%d", i)] = fmt.Sprintf("value.%d", i)
	}
	return data
}

// BenchmarkReload_Full benchmarks a full reload of 10,000 keys on every
// iteration.  The engine is freshly constructed so no delta optimisation
// is available — the entire merge pipeline runs each time.
func BenchmarkReload_Full(b *testing.B) {
	ctx := context.Background()
	data := generateLargeData(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ml := loader.NewMemoryLoader(
			loader.WithMemoryData(data),
			loader.WithMemoryPriority(50),
		)
		layer := NewLayer("memory",
			WithLayerPriority(50),
			WithLayerSource(ml),
		)
		eng := New(WithLayer(layer))
		b.StartTimer()

		_, err := eng.Reload(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReload_Delta benchmarks a delta reload where the underlying data
// has NOT changed.  The checksum comparison detects no modifications and the
// reload short-circuits, returning an empty ReloadResult.
func BenchmarkReload_Delta(b *testing.B) {
	ctx := context.Background()
	data := generateLargeData(10000)

	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(50),
	)
	layer := NewLayer("memory",
		WithLayerPriority(50),
		WithLayerSource(ml),
	)
	eng := New(WithLayer(layer), WithDeltaReload())

	// Initial reload to populate checksums
	_, err := eng.Reload(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := eng.Reload(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReload_DeltaWithChanges benchmarks a delta reload where 1 % of the
// keys are modified between iterations via MemoryLoader.Update.  This
// exercises the checksum-differencing path that must still merge changed
// layers while skipping unchanged ones.
func BenchmarkReload_DeltaWithChanges(b *testing.B) {
	ctx := context.Background()
	data := generateLargeData(10000)

	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(50),
	)
	layer := NewLayer("memory",
		WithLayerPriority(50),
		WithLayerSource(ml),
	)
	eng := New(WithLayer(layer), WithDeltaReload())

	// Initial reload to populate checksums
	_, err := eng.Reload(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Modify 1 % of keys each iteration
		modified := make(map[string]any, len(data))
		for k, v := range data {
			modified[k] = v
		}
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("key.%d", j)
			modified[key] = fmt.Sprintf("value.%d.changed", i)
		}
		ml.Update(modified)
		b.StartTimer()

		_, err := eng.Reload(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMerge_10000Keys benchmarks the priority-based merge of a single
// 10 000-key map.  This is the core operation that runs during every reload.
func BenchmarkMerge_10000Keys(b *testing.B) {
	data := generateLargeData(10000)
	values := make(map[string]value.Value, len(data))
	for k, v := range data {
		values[k] = value.NewInMemory(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value.MergeWithPriorityPlan(values)
	}
}

// BenchmarkComputeDiff_10000Keys benchmarks the diff computation between two
// 10 000-key maps where 100 keys have been changed.  This runs after every
// reload to generate change events.
func BenchmarkComputeDiff_10000Keys(b *testing.B) {
	oldData := make(map[string]value.Value, 10000)
	newData := make(map[string]value.Value, 10000)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key.%d", i)
		oldData[key] = value.NewInMemory(fmt.Sprintf("value.%d", i))
		newData[key] = value.NewInMemory(fmt.Sprintf("value.%d", i))
	}
	// Modify 100 keys
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key.%d", i)
		newData[key] = value.NewInMemory(fmt.Sprintf("changed.%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value.ComputeDiff(oldData, newData)
	}
}

// TestDeltaReload_NoChange verifies that a delta reload produces zero events
// when the underlying data has not changed between two consecutive Reload calls.
func TestDeltaReload_NoChange(t *testing.T) {
	ctx := context.Background()
	data := map[string]any{"key1": "val1", "key2": "val2"}

	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(50),
	)
	layer := NewLayer("memory",
		WithLayerPriority(50),
		WithLayerSource(ml),
	)
	eng := New(WithLayer(layer), WithDeltaReload())

	// First reload populates data
	result1, err := eng.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result1.Events) == 0 {
		t.Fatal("first reload should produce events (all keys are new)")
	}

	// Second reload should be a no-op (same data)
	result2, err := eng.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result2.Events) != 0 {
		t.Fatalf("delta reload with no changes should produce no events, got %d", len(result2.Events))
	}
}

// TestDeltaReload_WithChanges verifies that a delta reload correctly detects
// and propagates events when some keys have been modified via Update.
func TestDeltaReload_WithChanges(t *testing.T) {
	ctx := context.Background()
	data := map[string]any{"key1": "val1", "key2": "val2"}

	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(50),
	)
	layer := NewLayer("memory",
		WithLayerPriority(50),
		WithLayerSource(ml),
	)
	eng := New(WithLayer(layer), WithDeltaReload())

	// Initial reload
	_, err := eng.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Modify one key via Update
	ml.Update(map[string]any{"key1": "new_val1", "key2": "val2"})

	// Delta reload should detect the change
	result, err := eng.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) == 0 {
		t.Fatal("delta reload should produce events after Update")
	}

	// Verify key1 was updated
	v, ok := eng.Get("key1")
	if !ok {
		t.Fatal("key1 should exist")
	}
	if v.String() != "new_val1" {
		t.Fatalf("key1 = %q, want %q", v.String(), "new_val1")
	}
}

// TestDeltaReload_LargeNoChange verifies that a delta reload with 10 000
// unchanged keys short-circuits (returns empty result).
func TestDeltaReload_LargeNoChange(t *testing.T) {
	ctx := context.Background()
	data := generateLargeData(10000)

	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(50),
	)
	layer := NewLayer("memory",
		WithLayerPriority(50),
		WithLayerSource(ml),
	)
	eng := New(WithLayer(layer), WithDeltaReload())

	// Initial reload
	result1, err := eng.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result1.MergePlan.TotalKeys != 10000 {
		t.Fatalf("expected 10000 keys, got %d", result1.MergePlan.TotalKeys)
	}

	// Delta reload — no changes
	result2, err := eng.Reload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result2.Events) != 0 {
		t.Fatalf("delta reload with no changes: expected 0 events, got %d", len(result2.Events))
	}
}
