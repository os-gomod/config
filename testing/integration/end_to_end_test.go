package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
)

// ---------------------------------------------------------------------------
// TestEndToEndWorkflow
// ---------------------------------------------------------------------------

type e2eConfig struct {
	Name    string `config:"app.name"`
	Port    int    `config:"app.port"`
	Enabled bool   `config:"app.enabled"`
	Debug   bool   `config:"app.debug"`
	Host    string `config:"server.host"`
}

func TestEndToEndWorkflow(t *testing.T) {
	// Step 1: Create config with memory loader and initial data.
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"app.name":    "v1-app",
		"app.port":    8080,
		"app.enabled": true,
		"server.host": "0.0.0.0",
	}))

	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	// Step 2: Bind to struct and verify initial values.
	var app e2eConfig
	if err = cfg.Bind(t.Context(), &app); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if app.Name != "v1-app" {
		t.Errorf("initial Name = %q, want 'v1-app'", app.Name)
	}
	if app.Port != 8080 {
		t.Errorf("initial Port = %d, want 8080", app.Port)
	}
	if app.Enabled != true {
		t.Errorf("initial Enabled = %v, want true", app.Enabled)
	}
	if app.Host != "0.0.0.0" {
		t.Errorf("initial Host = %q, want '0.0.0.0'", app.Host)
	}

	// Step 3: Set new values programmatically.
	if err = cfg.Set(t.Context(), "app.name", "v2-app"); err != nil {
		t.Fatalf("Set app.name: %v", err)
	}
	if err = cfg.Set(t.Context(), "app.debug", true); err != nil {
		t.Fatalf("Set app.debug: %v", err)
	}
	if err = cfg.Set(t.Context(), "server.host", "192.168.1.1"); err != nil {
		t.Fatalf("Set server.host: %v", err)
	}

	// Step 4: Verify changes via direct Get.
	v, ok := cfg.Get("app.name")
	if !ok || v.Raw() != "v2-app" {
		t.Errorf("after set, app.name = %v, want 'v2-app'", v.Raw())
	}

	// Step 5: Reload (simulating external config change).
	mem.Update(map[string]any{
		"app.name":    "v3-app",
		"app.port":    9090,
		"app.enabled": false,
		"app.debug":   true,
		"server.host": "10.0.0.1",
	})
	result, err := cfg.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if len(result.Events) == 0 {
		t.Error("expected reload events")
	}

	// Step 6: Bind again and verify post-reload values.
	var app2 e2eConfig
	if err := cfg.Bind(t.Context(), &app2); err != nil {
		t.Fatalf("Bind after reload: %v", err)
	}
	if app2.Name != "v3-app" {
		t.Errorf("after reload Name = %q, want 'v3-app'", app2.Name)
	}
	// Port comes from memory loader (int), YAML might decode as float64.
	portVal, ok := cfg.Get("app.port")
	if !ok {
		t.Fatal("expected app.port to exist after reload")
	}
	switch p := portVal.Raw().(type) {
	case int:
		if p != 9090 {
			t.Errorf("after reload Port = %d, want 9090", p)
		}
	case float64:
		if p != 9090 {
			t.Errorf("after reload Port = %v, want 9090", p)
		}
	default:
		t.Errorf("after reload Port unexpected type %T: %v", portVal.Raw(), portVal.Raw())
	}
	if app2.Enabled != false {
		t.Errorf("after reload Enabled = %v, want false", app2.Enabled)
	}

	// Step 7: Snapshot, modify, restore, verify.
	snap := cfg.Snapshot()
	if len(snap) == 0 {
		t.Fatal("snapshot should have entries")
	}

	if err := cfg.Set(t.Context(), "app.name", "modified"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, _ = cfg.Get("app.name")
	if v.Raw() != "modified" {
		t.Errorf("after modify, app.name = %v, want 'modified'", v.Raw())
	}

	cfg.Restore(snap)
	v, _ = cfg.Get("app.name")
	if v.Raw() != "v3-app" {
		t.Errorf("after restore, app.name = %v, want 'v3-app'", v.Raw())
	}

	// Step 8: Subscribe to changes and verify event fires on set.
	ch := make(chan event.Event, 1)
	cancel := cfg.Subscribe(func(_ context.Context, evt event.Event) error {
		select {
		case ch <- evt:
		default:
		}
		return nil
	})

	_ = cfg.Set(t.Context(), "trigger.event", "fire")
	select {
	case received := <-ch:
		if received.Key != "trigger.event" {
			t.Errorf("expected event key 'trigger.event', got %q", received.Key)
		}
		if received.Type != event.TypeCreate {
			t.Errorf("expected event type Create, got %d", received.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	cancel()

	// Step 9: Delete a key and verify.
	if err := cfg.Delete(t.Context(), "trigger.event"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := cfg.Get("trigger.event"); ok {
		t.Error("key should be deleted")
	}
}

// ---------------------------------------------------------------------------
// TestCloseCleanup
// ---------------------------------------------------------------------------

func TestCloseCleanup(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"key": "value",
	}))

	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Verify config is functional before close.
	v, ok := cfg.Get("key")
	if !ok || v.Raw() != "value" {
		t.Fatal("expected key=value before close")
	}

	// Close the config.
	err = cfg.Close(t.Context())
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify IsClosed reports true.
	if !cfg.IsClosed() {
		t.Error("expected IsClosed to return true")
	}

	// Verify operations fail after close.
	_, err = cfg.Reload(t.Context())
	if err == nil {
		t.Error("expected Reload to fail after close")
	}

	err = cfg.Set(t.Context(), "new.key", "val")
	if err == nil {
		t.Error("expected Set to fail after close")
	}

	err = cfg.Delete(t.Context(), "key")
	if err == nil {
		t.Error("expected Delete to fail after close")
	}

	// Verify the Done channel is closed.
	select {
	case <-cfg.Done():
		// Expected.
	default:
		t.Error("expected Done channel to be closed")
	}
}

// ---------------------------------------------------------------------------
// TestCloseMultipleTimes
// ---------------------------------------------------------------------------

func TestCloseMultipleTimes(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Close twice — must not panic or error.
	err1 := cfg.Close(t.Context())
	err2 := cfg.Close(t.Context())
	if err1 != nil {
		t.Errorf("first Close: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Close: %v", err2)
	}
}

// ---------------------------------------------------------------------------
// TestEndToEndWithSubscriberTracking
// ---------------------------------------------------------------------------

func TestEndToEndWithSubscriberTracking(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	var createCount, updateCount, deleteCount atomic.Int32

	cfg.Subscribe(func(_ context.Context, evt event.Event) error {
		switch evt.Type {
		case event.TypeCreate:
			createCount.Add(1)
		case event.TypeUpdate:
			updateCount.Add(1)
		case event.TypeDelete:
			deleteCount.Add(1)
		}
		return nil
	})

	// Create.
	_ = cfg.Set(t.Context(), "tracked.key", "v1")
	// Update.
	_ = cfg.Set(t.Context(), "tracked.key", "v2")
	// Delete.
	_ = cfg.Delete(t.Context(), "tracked.key")

	// Wait for async events.
	time.Sleep(100 * time.Millisecond)

	if got := createCount.Load(); got != 1 {
		t.Errorf("expected 1 create event, got %d", got)
	}
	if got := updateCount.Load(); got != 1 {
		t.Errorf("expected 1 update event, got %d", got)
	}
	if got := deleteCount.Load(); got != 1 {
		t.Errorf("expected 1 delete event, got %d", got)
	}
}
