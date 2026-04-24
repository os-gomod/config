package plugin

import (
	"errors"
	"testing"

	gvalidator "github.com/go-playground/validator/v10"
	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/provider"
)

type mockPlugin struct {
	name    string
	initErr error
}

func (p *mockPlugin) Name() string { return p.name }
func (p *mockPlugin) Init(h Host) error {
	if p.initErr != nil {
		return p.initErr
	}
	return nil
}

type mockHost struct {
	loaders   map[string]bool
	providers map[string]bool
	decoders  map[string]bool
	validates map[string]bool
}

func newMockHost() *mockHost {
	return &mockHost{
		loaders:   make(map[string]bool),
		providers: make(map[string]bool),
		decoders:  make(map[string]bool),
		validates: make(map[string]bool),
	}
}

func (h *mockHost) RegisterLoader(name string, f loader.Factory) error {
	h.loaders[name] = true
	return nil
}

func (h *mockHost) RegisterProvider(name string, f provider.Factory) error {
	h.providers[name] = true
	return nil
}

func (h *mockHost) RegisterDecoder(d decoder.Decoder) error {
	h.decoders["decoder"] = true
	return nil
}

func (h *mockHost) RegisterValidator(tag string, fn gvalidator.Func) error {
	h.validates[tag] = true
	return nil
}
func (h *mockHost) Subscribe(obs event.Observer) func() { return func() {} }

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	host := newMockHost()
	p := &mockPlugin{name: "test-plugin"}
	if err := r.Register(p, host); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := r.Plugins()
	if len(names) != 1 || names[0] != "test-plugin" {
		t.Errorf("expected ['test-plugin'], got %v", names)
	}
}

func TestRegistryDuplicate(t *testing.T) {
	r := NewRegistry()
	host := newMockHost()
	p1 := &mockPlugin{name: "dup"}
	p2 := &mockPlugin{name: "dup"}

	if err := r.Register(p1, host); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Register(p2, host); err == nil {
		t.Fatal("expected error for duplicate plugin name")
	}
}

func TestRegistryInitFailure(t *testing.T) {
	r := NewRegistry()
	host := newMockHost()
	p := &mockPlugin{name: "fail-plugin", initErr: errors.New("init failed")}

	err := r.Register(p, host)
	if err == nil {
		t.Fatal("expected error when init fails")
	}
	if !strContains(err.Error(), "init") {
		t.Errorf("error should mention init, got: %v", err)
	}
	names := r.Plugins()
	if len(names) != 0 {
		t.Errorf("expected 0 plugins after rollback, got %v", names)
	}
}

func TestRegistryPlugins(t *testing.T) {
	r := NewRegistry()
	host := newMockHost()
	r.Register(&mockPlugin{name: "a"}, host)
	r.Register(&mockPlugin{name: "b"}, host)
	r.Register(&mockPlugin{name: "c"}, host)

	names := r.Plugins()
	if len(names) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(names))
	}
	if names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Errorf("expected [a, b, c], got %v", names)
	}
}

func TestRegistryPluginsEmpty(t *testing.T) {
	r := NewRegistry()
	names := r.Plugins()
	if len(names) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(names))
	}
}

func strContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
