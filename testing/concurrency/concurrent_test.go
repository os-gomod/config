package concurrency

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
)

// ---------------------------------------------------------------------------
// TestConcurrentReads
// ---------------------------------------------------------------------------

func TestConcurrentReads(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}))
	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	const goroutines = 50
	const readsPer = 200
	var wg sync.WaitGroup
	var errors atomic.Int64

	keys := []string{"key1", "key2", "key3"}
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < readsPer; j++ {
				key := keys[j%len(keys)]
				v, ok := cfg.Get(key)
				if !ok {
					errors.Add(1)
					continue
				}
				if v.IsZero() {
					errors.Add(1)
				}
				// Also read via GetAll.
				all := cfg.GetAll()
				if all == nil || len(all) != 3 {
					errors.Add(1)
				}
				// Read via Has.
				if !cfg.Has(key) {
					errors.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if got := errors.Load(); got != 0 {
		t.Errorf("concurrent reads: %d errors", got)
	}
}

// ---------------------------------------------------------------------------
// TestConcurrentWrites
// ---------------------------------------------------------------------------

func TestConcurrentWrites(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	const goroutines = 50
	const writesPer = 100
	var wg sync.WaitGroup
	var errors atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPer; j++ {
				key := "key." + itoa(id) + "." + itoa(j)
				if err := cfg.Set(t.Context(), key, id*writesPer+j); err != nil {
					errors.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	if got := errors.Load(); got != 0 {
		t.Errorf("concurrent writes: %d errors", got)
	}

	// Verify the total count of keys matches expectations.
	expectedKeys := goroutines * writesPer
	if got := cfg.Len(); got != expectedKeys {
		t.Errorf("expected %d keys, got %d", expectedKeys, got)
	}
}

// ---------------------------------------------------------------------------
// TestConcurrentReload
// ---------------------------------------------------------------------------

func TestConcurrentReload(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"counter": 0,
	}))
	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	const goroutines = 20
	const reloadsPer = 50
	var wg sync.WaitGroup
	var errors atomic.Int64
	var successCount atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < reloadsPer; j++ {
				// Update the memory loader before reloading.
				mem.Update(map[string]any{
					"counter": id*reloadsPer + j,
				})
				_, err := cfg.Reload(t.Context())
				if err != nil {
					errors.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	if got := errors.Load(); got != 0 {
		t.Errorf("concurrent reloads: %d errors", got)
	}

	// At least some reloads should succeed.
	if got := successCount.Load(); got == 0 {
		t.Error("expected at least some successful reloads")
	}

	// Final state should be valid.
	if cfg.Len() != 1 {
		t.Errorf("expected 1 key, got %d", cfg.Len())
	}
}

// ---------------------------------------------------------------------------
// TestConcurrentBusPublish
// ---------------------------------------------------------------------------

func TestConcurrentBusPublish(t *testing.T) {
	bus := event.NewBus()

	// Subscribe a catch-all observer.
	const expectedPerPub = 1 // one catch-all subscriber
	const goroutines = 50
	const pubsPer = 100
	var received atomic.Int64

	cancel := bus.Subscribe("*", func(_ context.Context, _ event.Event) error {
		received.Add(1)
		return nil
	})
	defer cancel()

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < pubsPer; j++ {
				key := "pub." + itoa(id) + "." + itoa(j)
				evt := event.New(event.TypeCreate, key)
				bus.Publish(ctx, &evt)
			}
		}(i)
	}
	wg.Wait()

	// Wait for async delivery to complete.
	totalExpected := int64(goroutines * pubsPer * expectedPerPub)
	deadline := time.After(5 * time.Second)
	for received.Load() < totalExpected {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for events: got %d/%d", received.Load(), totalExpected)
		}
	}

	if got := received.Load(); got != totalExpected {
		t.Errorf("expected %d events, got %d", totalExpected, got)
	}
}

// ---------------------------------------------------------------------------
// TestConcurrentMixedReadWriteDelete
// ---------------------------------------------------------------------------

func TestConcurrentMixedReadWriteDelete(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"persistent": "stays",
	}))
	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	const goroutines = 30
	const opsPer = 100
	var wg sync.WaitGroup
	var errors atomic.Int64

	// Half goroutines write, half read.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPer; j++ {
				if id%3 == 0 {
					// Writer: set a unique key.
					key := "write." + itoa(id) + "." + itoa(j)
					if err := cfg.Set(t.Context(), key, j); err != nil {
						errors.Add(1)
					}
				} else if id%3 == 1 {
					// Reader: read all.
					all := cfg.GetAll()
					if all == nil {
						errors.Add(1)
					}
					_ = cfg.Has("persistent")
				} else {
					// Deleter: try to delete a key that may or may not exist.
					key := "write." + itoa(id%10) + "." + itoa(j)
					_ = cfg.Delete(t.Context(), key)
				}
			}
		}(i)
	}
	wg.Wait()

	if got := errors.Load(); got != 0 {
		t.Errorf("mixed concurrent ops: %d errors", got)
	}

	// "persistent" key should survive all operations.
	v, ok := cfg.Get("persistent")
	if !ok || v.Raw() != "stays" {
		t.Error("expected 'persistent' key to survive")
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
