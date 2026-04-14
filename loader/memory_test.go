package loader_test

import (
	"context"
	"sync"
	"testing"

	"github.com/os-gomod/config/loader"
)

func TestMemoryLoaderLoad(t *testing.T) {
	m := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"db.host": "localhost",
		"db.port": 5432,
	}))
	data, err := m.Load(context.Background())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if v, ok := data["db.host"]; !ok || v.Raw() != "localhost" {
		t.Errorf("expected db.host=localhost, got %v", v)
	}
}

func TestMemoryLoaderUpdate(t *testing.T) {
	m := loader.NewMemoryLoader()
	m.Update(map[string]any{"key": "old"})
	data, _ := m.Load(context.Background())
	if v, _ := data["key"]; v.Raw() != "old" {
		t.Errorf("expected old, got %v", v.Raw())
	}

	m.Update(map[string]any{"key": "new"})
	data, _ = m.Load(context.Background())
	if v, _ := data["key"]; v.Raw() != "new" {
		t.Errorf("expected new, got %v", v.Raw())
	}
}

func TestMemoryLoaderConcurrentAccess(t *testing.T) {
	m := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"key": "initial",
	}))

	var wg sync.WaitGroup
	const goroutines = 100
	wg.Add(goroutines * 2)

	// Concurrent readers.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			data, err := m.Load(context.Background())
			if err != nil {
				t.Errorf("Load error: %v", err)
			}
			if _, ok := data["key"]; !ok {
				t.Error("missing key")
			}
		}()
	}

	// Concurrent writers.
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			m.Update(map[string]any{"key": i})
		}(i)
	}

	wg.Wait()
}
