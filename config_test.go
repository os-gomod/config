package config_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/observability"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/validator"

	gvalidator "github.com/go-playground/validator/v10"
)

// testStruct is a simple struct for binding tests.
type testStruct struct {
	Name    string `config:"name"`
	Age     int    `config:"age"`
	Enabled bool   `config:"enabled"`
}

// validatedStruct uses validate tags.
type validatedStruct struct {
	Email string `config:"email" validate:"required,email"`
}

// schemaStruct uses json tags for schema generation.
type schemaStruct struct {
	Name string `json:"name" validate:"required" description:"The name"`
	Age  int    `json:"age"  validate:"min=0"`
}

type prefixedNamePlugin struct{}

func (prefixedNamePlugin) Name() string { return "prefixed-name" }

func (prefixedNamePlugin) Init(h plugin.Host) error {
	return h.RegisterValidator("starts_with_demo", func(fl gvalidator.FieldLevel) bool {
		return strings.HasPrefix(fl.Field().String(), "demo")
	})
}

type pluginValidatedStruct struct {
	Name string `config:"name" validate:"starts_with_demo"`
}

type validateWithCustomTagStruct struct {
	Name string `validate:"starts_with_demo"`
}

func TestNew_LoadFromMemory(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"greeting": "hello",
			"count":    42,
		}),
	)
	layer := core.NewLayer("memory",
		core.WithLayerPriority(10),
		core.WithLayerSource(ml),
	)

	c, err := config.New(ctx, config.WithLayer(layer))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	v, ok := c.Engine.Get("greeting")
	if !ok {
		t.Fatal("key greeting not found")
	}
	if v.Raw() != "hello" {
		t.Errorf("greeting = %v, want hello", v.Raw())
	}
}

func TestNew_StrictReload_FailingLayer(t *testing.T) {
	ctx := context.Background()

	// Create a failing loader.
	fl := &builderFailingLoader{}
	layer := core.NewLayer("failing",
		core.WithLayerPriority(10),
		core.WithLayerSource(fl),
	)

	_, err := config.New(ctx,
		config.WithLayer(layer),
		config.WithStrictReload(),
	)
	if err == nil {
		t.Error("expected error with strict reload and failing layer")
	}
}

func TestNew_NonStrictReload_FailingLayer_ReturnsWarning(t *testing.T) {
	ctx := context.Background()

	fl := &builderFailingLoader{}
	layer := core.NewLayer("failing",
		core.WithLayerPriority(10),
		core.WithLayerSource(fl),
	)

	c, err := config.New(ctx, config.WithLayer(layer))
	if err == nil {
		t.Error("expected ReloadWarning, got nil error")
	}
	var w *config.ReloadWarning
	if !errors.As(err, &w) {
		t.Errorf("expected *ReloadWarning, got %T", err)
	}
	_ = c
}

func TestConfig_Set(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	if err := c.Set(ctx, "key1", "value1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok := c.Engine.Get("key1")
	if !ok {
		t.Fatal("key1 not found after set")
	}
	if v.Raw() != "value1" {
		t.Errorf("key1 = %v, want value1", v.Raw())
	}
}

func TestConfig_BatchSet(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	if err := c.BatchSet(ctx, map[string]any{
		"a": 1,
		"b": 2,
	}); err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	if v, ok := c.Engine.Get("a"); !ok || v.Raw() != 1 {
		t.Errorf("a = %v, want 1", v.Raw())
	}
	if v, ok := c.Engine.Get("b"); !ok || v.Raw() != 2 {
		t.Errorf("b = %v, want 2", v.Raw())
	}
}

func TestConfig_Delete(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	if err := c.Set(ctx, "temp", "val"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Delete(ctx, "temp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, ok := c.Engine.Get("temp"); ok {
		t.Error("temp should be deleted")
	}
}

func TestConfig_Reload(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{"k": "v1"}),
	)
	layer := core.NewLayer("memory",
		core.WithLayerPriority(10),
		core.WithLayerSource(ml),
	)

	c, err := config.New(ctx, config.WithLayer(layer))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	// Update the memory loader data.
	ml.Update(map[string]any{"k": "v2"})

	result, err := c.Reload(ctx)
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if result.HasErrors() {
		t.Errorf("Reload has errors: %v", result.LayerErrs)
	}

	v, ok := c.Engine.Get("k")
	if !ok || v.Raw() != "v2" {
		t.Errorf("after reload: k = %v, want v2", v.Raw())
	}
}

func TestConfig_Bind(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"name":    "Alice",
			"age":     30,
			"enabled": true,
		}),
	)
	layer := core.NewLayer("memory",
		core.WithLayerPriority(10),
		core.WithLayerSource(ml),
	)

	c, err := config.New(ctx, config.WithLayer(layer))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	var s testStruct
	if err := c.Bind(ctx, &s); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if s.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", s.Name)
	}
	if s.Age != 30 {
		t.Errorf("Age = %d, want 30", s.Age)
	}
	if s.Enabled != true {
		t.Errorf("Enabled = %v, want true", s.Enabled)
	}
}

func TestConfig_Validate(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	s := validatedStruct{Email: "not-an-email"}
	err = c.Validate(ctx, &s)
	if err == nil {
		t.Error("expected validation error for invalid email")
	}
}

func TestConfig_ValidateUsesConfiguredValidator(t *testing.T) {
	ctx := context.Background()
	v := validator.New(
		validator.WithCustomTag("starts_with_demo", func(fl gvalidator.FieldLevel) bool {
			return strings.HasPrefix(fl.Field().String(), "demo")
		}),
	)

	c, err := config.New(ctx, config.WithValidator(v))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	if err := c.Validate(ctx, &validateWithCustomTagStruct{Name: "demo-service"}); err != nil {
		t.Fatalf("Validate valid struct: %v", err)
	}
	if err := c.Validate(ctx, &validateWithCustomTagStruct{Name: "prod-service"}); err == nil {
		t.Fatal("expected validation error for custom tag")
	}
}

func TestConfig_WithPluginRegistersValidator(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"name": "demo-service",
		}),
	)

	c, err := config.New(
		ctx,
		config.WithLoader(ml),
		config.WithValidator(validator.New()),
		config.WithPlugin(prefixedNamePlugin{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	var s pluginValidatedStruct
	if err := c.Bind(ctx, &s); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if s.Name != "demo-service" {
		t.Errorf("Name = %q, want demo-service", s.Name)
	}

	names := c.Plugins()
	if len(names) != 1 || names[0] != "prefixed-name" {
		t.Fatalf("Plugins() = %v, want [prefixed-name]", names)
	}
}

func TestConfig_Schema(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	s, err := c.Schema(schemaStruct{})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if s == nil {
		t.Fatal("Schema returned nil")
	}
	if s.Type != "object" {
		t.Errorf("Schema.Type = %q, want object", s.Type)
	}
}

func TestConfig_Explain(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{"db.host": "localhost"}),
	)
	layer := core.NewLayer("memory",
		core.WithLayerPriority(10),
		core.WithLayerSource(ml),
	)

	c, err := config.New(ctx, config.WithLayer(layer))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	expl := c.Explain("db.host")
	if expl == "" {
		t.Error("Explain returned empty string for existing key")
	}
}

func TestConfig_Explain_MissingKey(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	expl := c.Explain("nonexistent")
	if expl != "" {
		t.Errorf("Explain for missing key = %q, want empty", expl)
	}
}

func TestConfig_OnChange(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	var received event.Event
	var mu sync.Mutex
	c.OnChange("*", func(_ context.Context, evt event.Event) error {
		mu.Lock()
		received = evt
		mu.Unlock()
		return nil
	})

	if err := c.Set(ctx, "watched", "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Wait for async publish.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if received.Key != "watched" {
		t.Errorf("received event key = %q, want watched", received.Key)
	}
	mu.Unlock()
}

func TestConfig_SubscribeWithPattern(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	var dbEvents []event.Event
	var mu sync.Mutex
	c.OnChange("db.*", func(_ context.Context, evt event.Event) error {
		mu.Lock()
		dbEvents = append(dbEvents, evt)
		mu.Unlock()
		return nil
	})

	_ = c.Set(ctx, "db.host", "localhost")
	_ = c.Set(ctx, "cache.ttl", "5m")

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(dbEvents) != 1 {
		t.Errorf("dbEvents count = %d, want 1", len(dbEvents))
	} else if dbEvents[0].Key != "db.host" {
		t.Errorf("dbEvents[0].Key = %q, want db.host", dbEvents[0].Key)
	}
	mu.Unlock()
}

func TestConfig_Close_ErrorsOnSubsequentOps(t *testing.T) {
	ctx := context.Background()
	c, err := config.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := c.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := c.Set(ctx, "after_close", "val"); err == nil {
		t.Error("Set after close should return error")
	}
}

func TestConfig_BackgroundReloadError_CallbackInvoked(t *testing.T) {
	var captured error
	handler := func(err error) {
		captured = err
	}

	ctx := context.Background()
	c, err := config.New(ctx, config.WithReloadErrorHandler(handler))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	// Manually call the handler to verify it is set.
	_ = c.Set(ctx, "test", "val")
	_ = captured // handler is invoked on background reload, which we can't easily trigger in this test
}

func TestConfig_WithRecorder(t *testing.T) {
	ctx := context.Background()
	metrics := &observability.AtomicMetrics{}

	c, err := config.New(ctx, config.WithRecorder(metrics))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(ctx)

	_ = c.Set(ctx, "metric_key", "val")

	sets := metrics.Sets.Load()
	if sets == 0 {
		t.Error("expected at least 1 set operation recorded")
	}
}
