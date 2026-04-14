package plugin_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/provider"

	gvalidator "github.com/go-playground/validator/v10"
)

// testPlugin is a minimal Plugin implementation for testing.
type testPlugin struct {
	name       string
	initErr    error
	initCalled atomic.Bool
}

func (p *testPlugin) Name() string { return p.name }
func (p *testPlugin) Init(h plugin.Host) error {
	p.initCalled.Store(true)
	return p.initErr
}

// testHost is a minimal Host implementation for testing.
type testHost struct {
	loaders    map[string]loader.Factory
	providers  map[string]provider.Factory
	decoders   []decoder.Decoder
	validators map[string]gvalidator.Func
	observers  []event.Observer
}

func newTestHost() *testHost {
	return &testHost{
		loaders:    make(map[string]loader.Factory),
		providers:  make(map[string]provider.Factory),
		validators: make(map[string]gvalidator.Func),
	}
}

func (h *testHost) RegisterLoader(name string, f loader.Factory) error {
	h.loaders[name] = f
	return nil
}

func (h *testHost) RegisterProvider(name string, f provider.Factory) error {
	h.providers[name] = f
	return nil
}

func (h *testHost) RegisterDecoder(d decoder.Decoder) error {
	h.decoders = append(h.decoders, d)
	return nil
}

func (h *testHost) RegisterValidator(tag string, fn gvalidator.Func) error {
	h.validators[tag] = fn
	return nil
}

func (h *testHost) Subscribe(obs event.Observer) func() {
	h.observers = append(h.observers, obs)
	return func() {}
}

func TestRegistry_Register_InitCalled(t *testing.T) {
	reg := plugin.NewRegistry()
	host := newTestHost()
	p := &testPlugin{name: "test-plugin"}

	if err := reg.Register(p, host); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !p.initCalled.Load() {
		t.Error("Init was not called")
	}
}

func TestRegistry_RegisterSameNameTwice(t *testing.T) {
	reg := plugin.NewRegistry()
	host := newTestHost()

	p1 := &testPlugin{name: "dup"}
	p2 := &testPlugin{name: "dup"}

	if err := reg.Register(p1, host); err != nil {
		t.Fatalf("Register p1: %v", err)
	}
	if err := reg.Register(p2, host); err == nil {
		t.Error("Register p2 should return error for duplicate name")
	}
}

func TestRegistry_PluginsReturnsNamesInOrder(t *testing.T) {
	reg := plugin.NewRegistry()
	host := newTestHost()

	p1 := &testPlugin{name: "alpha"}
	p2 := &testPlugin{name: "beta"}
	p3 := &testPlugin{name: "gamma"}

	if err := reg.Register(p1, host); err != nil {
		t.Fatalf("Register p1: %v", err)
	}
	if err := reg.Register(p2, host); err != nil {
		t.Fatalf("Register p2: %v", err)
	}
	if err := reg.Register(p3, host); err != nil {
		t.Fatalf("Register p3: %v", err)
	}

	names := reg.Plugins()
	want := []string{"alpha", "beta", "gamma"}
	if len(names) != len(want) {
		t.Fatalf("Plugins() = %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("Plugins()[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestRegistry_PluginCanRegisterLoaderFactory(t *testing.T) {
	reg := plugin.NewRegistry()
	host := newTestHost()

	p := &testPlugin{name: "loader-plugin"}
	p.initErr = nil // success

	// Override Init to register a loader.
	loaderPlugin := &loaderRegisterPlugin{name: "loader-plugin", host: host}
	if err := reg.Register(loaderPlugin, host); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, ok := host.loaders["custom-loader"]; !ok {
		t.Error("custom-loader not registered")
	}
}

func TestRegistry_PluginCanSubscribeToEvents(t *testing.T) {
	reg := plugin.NewRegistry()
	host := newTestHost()

	subPlugin := &subscribePlugin{name: "sub-plugin", host: host}
	if err := reg.Register(subPlugin, host); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(host.observers) != 1 {
		t.Errorf("observers = %d, want 1", len(host.observers))
	}
}

// loaderRegisterPlugin registers a loader factory during Init.
type loaderRegisterPlugin struct {
	name string
	host plugin.Host
}

func (p *loaderRegisterPlugin) Name() string { return p.name }
func (p *loaderRegisterPlugin) Init(h plugin.Host) error {
	return h.RegisterLoader("custom-loader", func(cfg map[string]any) (loader.Loader, error) {
		return nil, nil
	})
}

// subscribePlugin subscribes to events during Init.
type subscribePlugin struct {
	name string
	host plugin.Host
}

func (p *subscribePlugin) Name() string { return p.name }
func (p *subscribePlugin) Init(h plugin.Host) error {
	h.Subscribe(func(_ context.Context, _ event.Event) error { return nil })
	return nil
}
