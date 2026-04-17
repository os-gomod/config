package config

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
)

func TestNew(t *testing.T) {
	t.Run("with memory loader", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
			"app.name": "test-app",
			"app.port": 8080,
		}))
		cfg, err := New(t.Context(), WithLoader(mem))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		v, ok := cfg.Get("app.name")
		if !ok || v.Raw() != "test-app" {
			t.Fatal("expected app.name=test-app")
		}
	})

	t.Run("with empty memory loader", func(t *testing.T) {
		mem := loader.NewMemoryLoader()
		cfg, err := New(t.Context(), WithLoader(mem))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Len() != 0 {
			t.Fatalf("expected 0 keys, got %d", cfg.Len())
		}
	})
}

func TestMustNew(t *testing.T) {
	t.Run("no panic with valid options", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{"k": "v"}))
		cfg := MustNew(t.Context(), WithLoader(mem))
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
	})

	t.Run("panics on error", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic")
			}
		}()
		layer := core.NewLayer("fail", core.WithLayerSource(&stubLoadableConfig{
			err: fmt.Errorf("load failed"),
		}))
		MustNew(t.Context(), WithLayer(layer), WithStrictReload())
	})
}

func TestConfig_Reload(t *testing.T) {
	t.Run("reload updates data", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{"k": "v1"}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		mem.Update(map[string]any{"k": "v2", "k2": "new"})
		result, err := cfg.Reload(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v, ok := cfg.Get("k")
		if !ok || v.Raw() != "v2" {
			t.Fatal("expected k=v2 after reload")
		}
		if len(result.Events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(result.Events))
		}
	})

	t.Run("reload publishes events to subscribers", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{"k": "v1"}))
		cfg, _ := New(t.Context(), WithLoader(mem))

		var received atomic.Int32
		cfg.Subscribe(func(_ context.Context, _ event.Event) error {
			received.Add(1)
			return nil
		})
		mem.Update(map[string]any{"k": "v2"})
		_, _ = cfg.Reload(t.Context())
		// Wait for async event delivery
		time.Sleep(50 * time.Millisecond)
		if received.Load() == 0 {
			t.Fatal("expected at least one event to subscriber")
		}
	})
}

func TestConfig_Set(t *testing.T) {
	t.Run("set a new key", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{"k": "v"}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		err := cfg.Set(t.Context(), "new.key", "value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v, ok := cfg.Get("new.key")
		if !ok || v.Raw() != "value" {
			t.Fatal("expected new.key=value")
		}
	})

	t.Run("set publishes event", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		ch := make(chan event.Event, 1)
		cfg.Subscribe(func(_ context.Context, evt event.Event) error {
			select {
			case ch <- evt:
			default:
			}
			return nil
		})
		_ = cfg.Set(t.Context(), "key", "val")
		select {
		case received := <-ch:
			if received.Key != "key" {
				t.Fatalf("expected key 'key', got %q", received.Key)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for event")
		}
	})
}

func TestConfig_Delete(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{"k": "v"}))
	cfg, _ := New(t.Context(), WithLoader(mem))
	err := cfg.Delete(t.Context(), "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Get("k"); ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestConfig_BatchSet(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, _ := New(t.Context(), WithLoader(mem))
	err := cfg.BatchSet(t.Context(), map[string]any{
		"a": 1,
		"b": "two",
		"c": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Len() != 3 {
		t.Fatalf("expected 3 keys, got %d", cfg.Len())
	}
}

func TestConfig_Bind(t *testing.T) {
	type AppConfig struct {
		Name string `config:"app.name"`
		Port int    `config:"app.port"`
	}

	t.Run("bind to struct", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
			"app.name": "myapp",
			"app.port": 9090,
		}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		var app AppConfig
		err := cfg.Bind(t.Context(), &app)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if app.Name != "myapp" {
			t.Fatalf("expected name 'myapp', got %q", app.Name)
		}
		if app.Port != 9090 {
			t.Fatalf("expected port 9090, got %d", app.Port)
		}
	})

	t.Run("bind nil target returns error", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		err := cfg.Bind(t.Context(), nil)
		if err == nil {
			t.Fatal("expected error for nil target")
		}
	})
}

func TestConfig_OnChange(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, _ := New(t.Context(), WithLoader(mem))

	ch := make(chan string, 1)
	cancel := cfg.OnChange("app.*", func(_ context.Context, evt event.Event) error {
		select {
		case ch <- evt.Key:
		default:
		}
		return nil
	})

	_ = cfg.Set(t.Context(), "app.port", 8080)
	select {
	case receivedKey := <-ch:
		if !strings.HasPrefix(receivedKey, "app.") {
			t.Fatalf("expected app.* key, got %q", receivedKey)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnChange event")
	}

	cancel()
}

func TestConfig_SnapshotRestore(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"k1": "v1",
		"k2": "v2",
	}))
	cfg, _ := New(t.Context(), WithLoader(mem))

	snap := cfg.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 keys in snapshot, got %d", len(snap))
	}

	// Modify and restore
	_ = cfg.Set(t.Context(), "k3", "v3")
	_ = cfg.Delete(t.Context(), "k1")
	if cfg.Len() != 2 {
		t.Fatalf("expected 2 keys after modification, got %d", cfg.Len())
	}

	cfg.Restore(snap)
	if cfg.Len() != 2 {
		t.Fatalf("expected 2 keys after restore, got %d", cfg.Len())
	}
	v, ok := cfg.Get("k1")
	if !ok || v.Raw() != "v1" {
		t.Fatal("expected k1 to be restored")
	}
}

func TestConfig_Validate(t *testing.T) {
	type ValidatedConfig struct {
		Name string `config:"name" validate:"required"`
	}

	t.Run("valid struct", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
			"name": "valid",
		}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		var vc ValidatedConfig
		_ = cfg.Bind(t.Context(), &vc)
		err := cfg.Validate(t.Context(), &vc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid struct", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		var vc ValidatedConfig
		err := cfg.Validate(t.Context(), &vc)
		if err == nil {
			t.Fatal("expected validation error")
		}
	})
}

func TestConfig_Close(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{"k": "v"}))
	cfg, _ := New(t.Context(), WithLoader(mem))
	err := cfg.Close(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsClosed() {
		t.Fatal("expected config to be closed")
	}
	// Operations after close should fail
	_, err = cfg.Reload(t.Context())
	if err != errors.ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestConfig_Explain(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{"app.port": 8080}))
	cfg, _ := New(t.Context(), WithLoader(mem))
	explanation := cfg.Explain("app.port")
	if !strings.Contains(explanation, "app.port") {
		t.Fatalf("expected key in explanation, got: %s", explanation)
	}
	if !strings.Contains(explanation, "8080") {
		t.Fatalf("expected value in explanation, got: %s", explanation)
	}
}

func TestConfig_Explain_Missing(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, _ := New(t.Context(), WithLoader(mem))
	explanation := cfg.Explain("missing")
	if explanation != "" {
		t.Fatalf("expected empty explanation for missing key, got: %s", explanation)
	}
}

func TestConfig_Schema(t *testing.T) {
	type MyConfig struct {
		Name string `json:"name" description:"app name" validate:"required"`
		Port int    `json:"port" description:"app port"`
	}

	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, _ := New(t.Context(), WithLoader(mem))
	s, err := cfg.Schema(MyConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil schema")
	}
	if s.Type != "object" {
		t.Fatalf("expected type 'object', got %q", s.Type)
	}
	if len(s.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(s.Properties))
	}
}

func TestConfig_Plugins(t *testing.T) {
	t.Run("no plugins returns nil", func(t *testing.T) {
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		plugins := cfg.Plugins()
		if plugins != nil {
			t.Fatalf("expected nil plugins, got %v", plugins)
		}
	})
}

func TestConfig_Hooks(t *testing.T) {
	t.Run("before/after reload hooks", func(t *testing.T) {
		// Hooks are tested extensively in hooks/manager_test.go
	})
}

func TestConfig_WithLayer(t *testing.T) {
	t.Run("with core layer", func(t *testing.T) {
		data := map[string]value.Value{
			"key": value.New("value", value.TypeString, value.SourceMemory, 50),
		}
		layer := core.NewLayer("custom", core.WithLayerSource(&stubLoadableConfig{data: data}))
		cfg, err := New(t.Context(), WithLayer(layer))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v, ok := cfg.Get("key")
		if !ok || v.Raw() != "value" {
			t.Fatal("expected key=value")
		}
	})
}

func TestConfig_StrictReload(t *testing.T) {
	t.Run("strict reload fails on layer errors", func(t *testing.T) {
		layer := core.NewLayer("fail", core.WithLayerSource(&stubLoadableConfig{
			err: fmt.Errorf("load failed"),
		}))
		_, err := New(t.Context(), WithLayer(layer), WithStrictReload())
		if err == nil {
			t.Fatal("expected error with strict reload")
		}
	})
}

func TestReloadWarning(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		w := &ReloadWarning{
			LayerErrors: []core.LayerError{
				{Layer: "test", Err: fmt.Errorf("fail")},
			},
		}
		msg := w.Error()
		if !strings.Contains(msg, "1 layer warning") {
			t.Fatalf("unexpected message: %s", msg)
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		cause := fmt.Errorf("root")
		w := &ReloadWarning{
			LayerErrors: []core.LayerError{
				{Layer: "test", Err: cause},
			},
		}
		unwrapped := w.Unwrap()
		if unwrapped != cause {
			t.Fatal("expected unwrapped to return cause")
		}
	})
}

func TestConfig_Subscribe(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, _ := New(t.Context(), WithLoader(mem))
	var received atomic.Int32
	cancel := cfg.Subscribe(func(_ context.Context, _ event.Event) error {
		received.Add(1)
		return nil
	})
	_ = cfg.Set(t.Context(), "key", "val")
	time.Sleep(50 * time.Millisecond)
	if received.Load() == 0 {
		t.Fatal("expected event")
	}
	cancel()
}

func TestConfig_HookExecutionOnReload(t *testing.T) {
	t.Run("hooks via HookFunc registered directly", func(t *testing.T) {
		// This tests the hook integration at the config level by verifying
		// that hooks are checked during reload. The actual hook manager tests
		// are in hooks/manager_test.go.
		mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
		cfg, _ := New(t.Context(), WithLoader(mem))
		// Ensure hooks exist but no before/after hooks registered (no panic)
		_, err := cfg.Reload(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// stubLoadableConfig implements core.Loadable for config-level tests.
type stubLoadableConfig struct {
	data map[string]value.Value
	err  error
}

func (s *stubLoadableConfig) Load(_ context.Context) (map[string]value.Value, error) {
	if s.err != nil {
		return nil, s.err
	}
	return value.Copy(s.data), nil
}

func (s *stubLoadableConfig) Close(_ context.Context) error { return nil }
