package service

import (
	"fmt"
	"sync"

	gvalidator "github.com/go-playground/validator/v10"

	"github.com/os-gomod/config/v2/internal/decoder"
	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/eventbus"
	"github.com/os-gomod/config/v2/internal/loader"
	"github.com/os-gomod/config/v2/internal/observability"
	"github.com/os-gomod/config/v2/internal/provider"
	"github.com/os-gomod/config/v2/internal/registry"
)

// Plugin is the interface for config plugins.
type Plugin interface {
	Name() string
	Init(host PluginHost) error
}

// PluginHost provides capabilities that plugins can register.
type PluginHost interface {
	RegisterLoader(name string, f loader.Factory) error
	RegisterProvider(name string, f provider.Factory) error
	RegisterDecoder(d decoder.Decoder) error
	RegisterValidator(tag string, fn gvalidator.Func) error
	Subscribe(obs event.Observer) func()
}

// PluginService manages plugin registration and lifecycle.
type PluginService struct {
	mu       sync.RWMutex
	plugins  map[string]Plugin
	bundle   *registry.Bundle
	bus      *eventbus.Bus
	engine   Engine
	recorder observability.Recorder
}

// NewPluginService creates a PluginService with the given dependencies.
func NewPluginService(
	bundle *registry.Bundle,
	bus *eventbus.Bus,
	engine Engine,
	recorder observability.Recorder,
) *PluginService {
	return &PluginService{
		plugins:  make(map[string]Plugin),
		bundle:   bundle,
		bus:      bus,
		engine:   engine,
		recorder: recorder,
	}
}

// Register registers a plugin and initializes it.
func (s *PluginService) Register(p Plugin) error {
	name := p.Name()
	s.mu.Lock()
	if _, exists := s.plugins[name]; exists {
		s.mu.Unlock()
		return errors.Build(
			errors.CodeAlreadyExists,
			fmt.Sprintf("plugin %q already registered", name),
			errors.WithOperation("plugin.register"),
		)
	}
	s.plugins[name] = p
	s.mu.Unlock()

	host := &pluginHost{service: s}
	if err := p.Init(host); err != nil {
		// Rollback registration on init failure
		s.mu.Lock()
		delete(s.plugins, name)
		s.mu.Unlock()
		return errors.Build(
			errors.CodeInternal,
			fmt.Sprintf("plugin %q init failed", name),
			errors.WithOperation("plugin.register"),
		).Wrap(err)
	}
	return nil
}

// Plugins returns the names of all registered plugins.
func (s *PluginService) Plugins() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.plugins))
	for name := range s.plugins {
		names = append(names, name)
	}
	return names
}

// Has checks if a plugin is registered.
func (s *PluginService) Has(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.plugins[name]
	return ok
}

// pluginHost implements PluginHost using the PluginService's dependencies.
type pluginHost struct {
	service *PluginService
}

func (h *pluginHost) RegisterLoader(name string, f loader.Factory) error {
	return h.service.bundle.Loader.Register(name, f)
}

func (h *pluginHost) RegisterProvider(name string, f provider.Factory) error {
	return h.service.bundle.Provider.Register(name, f)
}

func (h *pluginHost) RegisterDecoder(d decoder.Decoder) error {
	return h.service.bundle.Decoder.Register(d)
}

func (h *pluginHost) RegisterValidator(_ string, _ gvalidator.Func) error {
	// Validator registration is handled via the validator engine
	return errors.New(errors.CodeNotImplemented, "dynamic validator registration not yet implemented via plugin host")
}

func (h *pluginHost) Subscribe(obs event.Observer) func() {
	return h.service.bus.Subscribe("", obs)
}

func init() {
	_ = value.Value{}
	_ = event.Observer(nil)
}
