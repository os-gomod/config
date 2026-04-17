package integration

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
)

// ---------------------------------------------------------------------------
// TestMultipleLayersWithPriority
// ---------------------------------------------------------------------------

func TestMultipleLayersWithPriority(t *testing.T) {
	// Create a temp YAML file for the file loader.
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	yamlContent := "app:\n  port: 3000\n  mode: production\n  region: us-east\n"
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set env vars for env loader (priority 40).
	// APP_ prefix is stripped, then defaultKeyReplacer converts _ to . and lowercases.
	// So APP_APP_MODE -> stripped: APP_MODE -> replacer: app.mode
	t.Setenv("APP_APP_MODE", "staging")
	t.Setenv("APP_APP_HOST", "env-host")

	// Memory loader: priority 10 (lowest).
	memData := map[string]any{
		"app.port": 8080,
		"app.mode": "dev",
		"app.name": "myapp",
	}
	mem := loader.NewMemoryLoader(
		loader.WithMemoryData(memData),
		loader.WithMemoryPriority(10),
	)

	// File loader: priority 30.
	file := loader.NewFileLoader(yamlPath, loader.WithFilePriority(30))

	// Env loader: priority 40 (highest).
	env := loader.NewEnvLoader(
		loader.WithEnvPrefix("APP"),
		loader.WithEnvPriority(40),
	)

	cfg, err := config.New(t.Context(),
		config.WithLoader(mem),
		config.WithLoader(file),
		config.WithLoader(env),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	// app.mode: env (priority 40) wins over file (30) and memory (10).
	v, ok := cfg.Get("app.mode")
	if !ok {
		t.Fatal("expected app.mode to exist")
	}
	if v.Raw() != "staging" {
		t.Errorf("app.mode = %v, want 'staging' (env wins)", v.Raw())
	}

	// app.port: file (priority 30) wins over memory (10).
	v, ok = cfg.Get("app.port")
	if !ok {
		t.Fatal("expected app.port to exist")
	}
	// File loader returns int from YAML decode; value type might be int or float64.
	portRaw := v.Raw()
	switch p := portRaw.(type) {
	case int:
		if p != 3000 {
			t.Errorf("app.port = %d, want 3000", p)
		}
	case float64:
		if p != 3000 {
			t.Errorf("app.port = %v, want 3000", p)
		}
	default:
		t.Errorf("app.port unexpected type: %T = %v", portRaw, portRaw)
	}

	// app.name: only from memory (priority 10).
	v, ok = cfg.Get("app.name")
	if !ok {
		t.Fatal("expected app.name to exist")
	}
	if v.Raw() != "myapp" {
		t.Errorf("app.name = %v, want 'myapp'", v.Raw())
	}

	// app.region: only from file (priority 30).
	v, ok = cfg.Get("app.region")
	if !ok {
		t.Fatal("expected app.region to exist")
	}
	if v.Raw() != "us-east" {
		t.Errorf("app.region = %v, want 'us-east'", v.Raw())
	}

	// app.host: only from env (priority 40).
	v, ok = cfg.Get("app.host")
	if !ok {
		t.Fatal("expected app.host to exist")
	}
	if v.Raw() != "env-host" {
		t.Errorf("app.host = %v, want 'env-host'", v.Raw())
	}
}

// ---------------------------------------------------------------------------
// TestReloadWithEvents
// ---------------------------------------------------------------------------

func TestReloadWithEvents(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"key1": "v1",
		"key2": "v2",
	}))

	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	var received atomic.Int32
	cfg.Subscribe(func(_ context.Context, evt event.Event) error {
		received.Add(1)
		return nil
	})

	// Update the memory loader and reload.
	mem.Update(map[string]any{
		"key1": "v1-updated",
		"key3": "v3-new",
	})
	result, err := cfg.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Wait for async event delivery.
	time.Sleep(100 * time.Millisecond)

	// Verify events were generated.
	if len(result.Events) == 0 {
		t.Fatal("expected reload to produce events")
	}

	// At least the subscriber should have received events.
	if got := received.Load(); got == 0 {
		t.Fatal("expected subscriber to receive events after reload")
	}

	// Verify key1 was updated.
	v, ok := cfg.Get("key1")
	if !ok || v.Raw() != "v1-updated" {
		t.Errorf("key1 = %v, want 'v1-updated'", v.Raw())
	}

	// Verify key3 was created.
	v, ok = cfg.Get("key3")
	if !ok || v.Raw() != "v3-new" {
		t.Errorf("key3 = %v, want 'v3-new'", v.Raw())
	}

	// key2 is lost after mem.Update replaces all data; this is expected behavior.
	// The reload replaces the entire layer contents with the updated data.
}

// ---------------------------------------------------------------------------
// TestSetAndDeleteWorkflow
// ---------------------------------------------------------------------------

func TestSetAndDeleteWorkflow(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"existing": "original",
	}))

	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	// Set new values.
	if err := cfg.Set(t.Context(), "new.key", "new-value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cfg.Set(t.Context(), "another.key", 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Verify all keys exist.
	if cfg.Len() != 3 {
		t.Errorf("expected 3 keys, got %d", cfg.Len())
	}

	// Update an existing key.
	if err := cfg.Set(t.Context(), "new.key", "updated-value"); err != nil {
		t.Fatalf("Set update: %v", err)
	}

	v, ok := cfg.Get("new.key")
	if !ok || v.Raw() != "updated-value" {
		t.Errorf("new.key = %v, want 'updated-value'", v.Raw())
	}

	// Delete a key.
	if err := cfg.Delete(t.Context(), "another.key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if cfg.Len() != 2 {
		t.Errorf("expected 2 keys after delete, got %d", cfg.Len())
	}

	// Verify the deleted key is gone.
	if _, ok := cfg.Get("another.key"); ok {
		t.Error("another.key should not exist after delete")
	}

	// Delete the originally loaded key.
	if err := cfg.Delete(t.Context(), "existing"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if cfg.Len() != 1 {
		t.Errorf("expected 1 key after second delete, got %d", cfg.Len())
	}

	// Delete a non-existent key should not error.
	if err := cfg.Delete(t.Context(), "nonexistent"); err != nil {
		t.Errorf("deleting non-existent key: expected no error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestLayerPriorityMergeCore
// ---------------------------------------------------------------------------

func TestLayerPriorityMergeCore(t *testing.T) {
	m1 := map[string]value.Value{
		"a": value.New("from-layer1", value.TypeString, value.SourceMemory, 10),
		"b": value.New("only-in-1", value.TypeString, value.SourceMemory, 10),
	}
	m2 := map[string]value.Value{
		"a": value.New("from-layer2", value.TypeString, value.SourceFile, 30),
		"c": value.New("only-in-2", value.TypeString, value.SourceFile, 30),
	}

	merged, plan := value.Merge(m1, m2)

	if plan.TotalKeys != 3 {
		t.Errorf("expected 3 total keys, got %d", plan.TotalKeys)
	}

	// Key "a": layer2 (priority 30) wins.
	va := merged["a"]
	if va.Raw() != "from-layer2" {
		t.Errorf("merged[a] = %v, want 'from-layer2'", va.Raw())
	}

	// Key "b": only in layer1.
	vb, ok := merged["b"]
	if !ok || vb.Raw() != "only-in-1" {
		t.Error("merged[b] should be 'only-in-1'")
	}

	// Key "c": only in layer2.
	vc, ok := merged["c"]
	if !ok || vc.Raw() != "only-in-2" {
		t.Error("merged[c] should be 'only-in-2'")
	}

	// Check overridden keys.
	found := false
	for _, k := range plan.OverriddenKeys {
		if k == "a" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected key 'a' in OverriddenKeys")
	}
}

// ---------------------------------------------------------------------------
// TestReloadNoChangesProducesNoEvents
// ---------------------------------------------------------------------------

func TestReloadNoChangesProducesNoEvents(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{
		"stable": "value",
	}))

	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	// Reload without changing data.
	result, err := cfg.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if len(result.Events) != 0 {
		t.Errorf("expected 0 events for no-change reload, got %d", len(result.Events))
	}
}

// ---------------------------------------------------------------------------
// TestBatchSetAndVerify
// ---------------------------------------------------------------------------

func TestBatchSetAndVerify(t *testing.T) {
	mem := loader.NewMemoryLoader(loader.WithMemoryData(map[string]any{}))
	cfg, err := config.New(t.Context(), config.WithLoader(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cfg.Close(t.Context())

	err = cfg.BatchSet(t.Context(), map[string]any{
		"batch.a": 1,
		"batch.b": "two",
		"batch.c": true,
		"batch.d": 3.14,
	})
	if err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	if cfg.Len() != 4 {
		t.Errorf("expected 4 keys, got %d", cfg.Len())
	}

	va, ok := cfg.Get("batch.a")
	if !ok || va.Raw() != 1 {
		t.Errorf("batch.a = %v, want 1", va.Raw())
	}

	vb, ok := cfg.Get("batch.b")
	if !ok || vb.Raw() != "two" {
		t.Errorf("batch.b = %v, want 'two'", vb.Raw())
	}

	vc, ok := cfg.Get("batch.c")
	if !ok || vc.Raw() != true {
		t.Errorf("batch.c = %v, want true", vc.Raw())
	}

	vd, ok := cfg.Get("batch.d")
	if !ok || vd.Raw() != 3.14 {
		t.Errorf("batch.d = %v, want 3.14", vd.Raw())
	}
}
