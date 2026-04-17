package loader

import (
	"context"
	"testing"

	"github.com/os-gomod/config/core/value"
)

func TestMemoryLoader_Load(t *testing.T) {
	t.Run("with initial data", func(t *testing.T) {
		m := NewMemoryLoader(WithMemoryData(map[string]any{
			"app.name": "test-app",
			"app.port": 8080,
		}))

		data, err := m.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(data))
		}
		if data["app.name"].String() != "test-app" {
			t.Errorf("expected 'test-app', got %q", data["app.name"].String())
		}
	})

	t.Run("returns copy (mutation isolation)", func(t *testing.T) {
		m := NewMemoryLoader(WithMemoryData(map[string]any{
			"key": "value",
		}))

		data, err := m.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Mutate the returned data
		data["key"] = value.New("mutated", value.TypeString, value.SourceMemory, 0)
		data["new"] = value.New("added", value.TypeString, value.SourceMemory, 0)

		// Load again - original should be unchanged
		data2, err := m.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data2["key"].String() != "value" {
			t.Errorf("mutation leaked: expected 'value', got %q", data2["key"].String())
		}
		if len(data2) != 1 {
			t.Errorf("mutation leaked: expected 1 key, got %d", len(data2))
		}
	})

	t.Run("empty loader returns empty map", func(t *testing.T) {
		m := NewMemoryLoader()
		data, err := m.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 0 {
			t.Fatalf("expected 0 keys, got %d", len(data))
		}
	})
}

func TestMemoryLoader_Update(t *testing.T) {
	t.Run("updates data", func(t *testing.T) {
		m := NewMemoryLoader(WithMemoryData(map[string]any{
			"old.key": "old-value",
		}))

		m.Update(map[string]any{
			"new.key": "new-value",
		})

		data, err := m.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 1 {
			t.Fatalf("expected 1 key, got %d", len(data))
		}
		if data["new.key"].String() != "new-value" {
			t.Errorf("expected 'new-value', got %q", data["new.key"].String())
		}
	})

	t.Run("update replaces all data", func(t *testing.T) {
		m := NewMemoryLoader(WithMemoryData(map[string]any{
			"a": "1",
			"b": "2",
		}))

		m.Update(map[string]any{
			"c": "3",
		})

		data, _ := m.Load(context.Background())
		if len(data) != 1 {
			t.Fatalf("expected 1 key after update, got %d", len(data))
		}
	})
}

func TestMemoryLoader_Close(t *testing.T) {
	t.Run("load after close returns error", func(t *testing.T) {
		m := NewMemoryLoader(WithMemoryData(map[string]any{
			"key": "value",
		}))

		err := m.Close(context.Background())
		if err != nil {
			t.Fatalf("unexpected close error: %v", err)
		}

		_, err = m.Load(context.Background())
		if err == nil {
			t.Fatal("expected error after close")
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		m := NewMemoryLoader()

		err1 := m.Close(context.Background())
		err2 := m.Close(context.Background())
		if err1 != nil || err2 != nil {
			t.Fatalf("close errors: %v, %v", err1, err2)
		}
	})
}

func TestMemoryLoader_Watch(t *testing.T) {
	t.Run("returns nil channel", func(t *testing.T) {
		m := NewMemoryLoader()
		ch, err := m.Watch(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ch != nil {
			t.Error("expected nil channel for memory loader")
		}
	})
}

func TestMemoryLoader_Options(t *testing.T) {
	t.Run("WithMemoryData infers types", func(t *testing.T) {
		m := NewMemoryLoader(WithMemoryData(map[string]any{
			"str":  "hello",
			"num":  42,
			"flag": true,
		}))

		data, _ := m.Load(context.Background())
		if data["str"].Type() != value.TypeString {
			t.Errorf("expected TypeString, got %d", data["str"].Type())
		}
		if data["num"].Type() != value.TypeInt {
			t.Errorf("expected TypeInt, got %d", data["num"].Type())
		}
		if data["flag"].Type() != value.TypeBool {
			t.Errorf("expected TypeBool, got %d", data["flag"].Type())
		}
	})

	t.Run("WithMemoryPriority sets priority", func(t *testing.T) {
		m := NewMemoryLoader(
			WithMemoryData(map[string]any{"key": "val"}),
			WithMemoryPriority(200),
		)

		data, _ := m.Load(context.Background())
		if data["key"].Priority() != 200 {
			t.Errorf("expected priority 200, got %d", data["key"].Priority())
		}
	})

	t.Run("WithMemoryPriority without data", func(t *testing.T) {
		m := NewMemoryLoader(WithMemoryPriority(300))
		if m.Priority() != 300 {
			t.Errorf("expected priority 300, got %d", m.Priority())
		}
	})

	t.Run("default priority is 20", func(t *testing.T) {
		m := NewMemoryLoader()
		if m.Priority() != 20 {
			t.Errorf("expected default priority 20, got %d", m.Priority())
		}
	})
}

func TestMemoryLoader_String(t *testing.T) {
	m := NewMemoryLoader()
	if m.String() != "memory" {
		t.Errorf("expected 'memory', got %q", m.String())
	}
}

func TestMemoryLoader_NameAndType(t *testing.T) {
	m := NewMemoryLoader()
	if m.Name() != "memory" {
		t.Errorf("expected name 'memory', got %q", m.Name())
	}
	if m.Type() != "memory" {
		t.Errorf("expected type 'memory', got %q", m.Type())
	}
}
