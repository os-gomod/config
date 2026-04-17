package plugin

import (
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/provider"
)

type stubPlugin struct {
	name    string
	initErr error
}

func (p *stubPlugin) Name() string { return p.name }
func (p *stubPlugin) Init(_ Host) error {
	return p.initErr
}

type stubHost struct{}

func (h *stubHost) RegisterLoader(name string, f loader.Factory) error     { return nil }
func (h *stubHost) RegisterProvider(name string, f provider.Factory) error { return nil }
func (h *stubHost) RegisterDecoder(d decoder.Decoder) error                { return nil }
func (h *stubHost) RegisterValidator(tag string, fn validator.Func) error  { return nil }
func (h *stubHost) Subscribe(obs event.Observer) func()                    { return func() {} }

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if len(r.Plugins()) != 0 {
		t.Fatalf("expected empty plugins list, got %d", len(r.Plugins()))
	}
}

func TestRegistry_Register(t *testing.T) {
	t.Run("register plugin successfully", func(t *testing.T) {
		r := NewRegistry()
		p := &stubPlugin{name: "test"}
		err := r.Register(p, &stubHost{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		plugins := r.Plugins()
		if len(plugins) != 1 || plugins[0] != "test" {
			t.Fatalf("expected ['test'], got %v", plugins)
		}
	})

	t.Run("duplicate registration error", func(t *testing.T) {
		r := NewRegistry()
		p := &stubPlugin{name: "dup"}
		_ = r.Register(p, &stubHost{})
		err := r.Register(p, &stubHost{})
		if err == nil {
			t.Fatal("expected error for duplicate registration")
		}
	})

	t.Run("init failure rolls back", func(t *testing.T) {
		r := NewRegistry()
		p := &stubPlugin{name: "fail", initErr: errors.New("init failed")}
		err := r.Register(p, &stubHost{})
		if err == nil {
			t.Fatal("expected error for init failure")
		}
		if len(r.Plugins()) != 0 {
			t.Fatalf("expected 0 plugins after rollback, got %d", len(r.Plugins()))
		}
	})

	t.Run("init failure allows re-registration", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(&stubPlugin{name: "retry", initErr: errors.New("fail")}, &stubHost{})
		err := r.Register(&stubPlugin{name: "retry"}, &stubHost{})
		if err != nil {
			t.Fatalf("expected successful re-registration, got: %v", err)
		}
	})

	t.Run("multiple plugins", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(&stubPlugin{name: "a"}, &stubHost{})
		_ = r.Register(&stubPlugin{name: "b"}, &stubHost{})
		_ = r.Register(&stubPlugin{name: "c"}, &stubHost{})
		plugins := r.Plugins()
		if len(plugins) != 3 {
			t.Fatalf("expected 3 plugins, got %d", len(plugins))
		}
		if plugins[0] != "a" || plugins[1] != "b" || plugins[2] != "c" {
			t.Fatalf("expected [a b c], got %v", plugins)
		}
	})
}

func TestRegistry_Plugins(t *testing.T) {
	t.Run("empty registry", func(t *testing.T) {
		r := NewRegistry()
		plugins := r.Plugins()
		if plugins == nil {
			t.Fatal("expected non-nil, empty slice")
		}
		if len(plugins) != 0 {
			t.Fatalf("expected 0 plugins, got %d", len(plugins))
		}
	})
}
