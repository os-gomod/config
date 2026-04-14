package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/observability"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/validator"
)

func TestBuilder_MemoryBuild(t *testing.T) {
	ctx := context.Background()
	c, err := config.NewBuilder().
		Memory(map[string]any{"key": "value"}).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer c.Close(ctx)

	v, ok := c.Engine.Get("key")
	if !ok {
		t.Fatal("key not found")
	}
	if v.Raw() != "value" {
		t.Errorf("key = %v, want value", v.Raw())
	}
}

func TestBuilder_EnvPriority(t *testing.T) {
	// Env with higher priority should override memory.
	ctx := context.Background()
	c, err := config.NewBuilder().
		MemoryWithPriority(map[string]any{"db.host": "memory-host"}, 10).
		EnvWithPriority("MYAPP", 40).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer c.Close(ctx)

	// The env loader will pick up MYAPP_ prefixed vars if set.
	// In this test environment they are likely not set, so memory value wins.
	_ = c
}

func TestBuilder_ValidateInvalidStruct(t *testing.T) {
	ctx := context.Background()
	c, err := config.NewBuilder().
		Memory(map[string]any{"email": "not-an-email"}).
		Validate(validator.New()).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer c.Close(ctx)

	type EmailConfig struct {
		Email string `config:"email" validate:"required,email"`
	}
	var cfg EmailConfig
	if err := c.Bind(ctx, &cfg); err == nil {
		t.Error("expected validation error for invalid email")
	}
}

func TestBuilder_StrictReload(t *testing.T) {
	ctx := context.Background()

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

func TestBuilder_OnReloadError(t *testing.T) {
	var captured error
	handler := func(err error) {
		captured = err
	}

	ctx := context.Background()
	c, err := config.NewBuilder().
		Memory(map[string]any{"k": "v"}).
		OnReloadError(handler).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer c.Close(ctx)

	_ = captured
}

func TestBuilder_WithRecorder(t *testing.T) {
	ctx := context.Background()
	metrics := &observability.AtomicMetrics{}

	c, err := config.NewBuilder().
		Memory(map[string]any{"k": "v"}).
		Recorder(metrics).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer c.Close(ctx)

	_ = c.Set(ctx, "new_key", "val")
	if metrics.Sets.Load() == 0 {
		t.Error("expected set operation recorded")
	}
}

func TestBuilder_Plugin(t *testing.T) {
	ctx := context.Background()
	p := &builderTestPlugin{name: "test-plugin"}

	c, err := config.NewBuilder().
		Memory(map[string]any{"k": "v"}).
		Plugin(p).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer c.Close(ctx)

	if !p.initCalled {
		t.Error("plugin Init was not called")
	}
}

func TestBuilder_MustBuild(t *testing.T) {
	c := config.NewBuilder().
		Memory(map[string]any{"k": "v"}).
		MustBuild()
	ctx := context.Background()
	defer c.Close(ctx)

	v, ok := c.Engine.Get("k")
	if !ok || v.Raw() != "v" {
		t.Errorf("k = %v, want v", v.Raw())
	}
}

func TestBuilder_MustBind(t *testing.T) {
	ctx := context.Background()

	type SimpleConfig struct {
		Key string `config:"key"`
	}

	c := config.NewBuilder().
		Memory(map[string]any{"key": "bound_value"}).
		MustBind(ctx, &SimpleConfig{})

	defer c.Close(ctx)

	v, ok := c.Engine.Get("key")
	if !ok || v.Raw() != "bound_value" {
		t.Errorf("key = %v, want bound_value", v.Raw())
	}
}

func TestBuilder_MustBind_PanicsOnError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on bind error")
		}
	}()

	ctx := context.Background()

	// Bind to non-struct should fail.
	config.NewBuilder().
		Memory(map[string]any{"key": "val"}).
		MustBind(ctx, "not-a-struct")
}

// builderTestPlugin is a test plugin for builder tests.
type builderTestPlugin struct {
	name       string
	initCalled bool
}

func (p *builderTestPlugin) Name() string { return p.name }
func (p *builderTestPlugin) Init(_ plugin.Host) error {
	p.initCalled = true
	return nil
}

// builderFailingLoader always returns an error.
type builderFailingLoader struct{}

func (f *builderFailingLoader) Load(_ context.Context) (map[string]value.Value, error) {
	return nil, errors.New("load failed")
}

func (f *builderFailingLoader) Watch(
	_ context.Context,
) (<-chan event.Event, error) {
	return nil, nil
}
func (f *builderFailingLoader) Priority() int { return 10 }

func (f *builderFailingLoader) String() string                { return "failing" }
func (f *builderFailingLoader) Close(_ context.Context) error { return nil }
