// Package provider defines the Provider interface for remote configuration sources
// and provides a BaseProvider struct that eliminates boilerplate shared across
// all provider implementations (consul, etcd, nats, etc.).
package provider

import (
	"sort"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// Registry manages a collection of provider Factory functions indexed by name.
// It enables dynamic provider instantiation by name using configuration maps.
// The registry is safe for concurrent use.
//
// Example:
//
//	reg := provider.NewRegistry()
//	reg.Register("consul", func(cfg map[string]any) (provider.Provider, error) {
//	    return consul.New(consul.Config{Address: cfg["address"].(string)})
//	})
//	p, err := reg.Create("consul", map[string]any{"address": "127.0.0.1:8500"})
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates an empty provider Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a named provider factory to the registry.
// Returns an error if name is empty, factory is nil, or a factory with the
// same name is already registered.
func (r *Registry) Register(name string, f Factory) error {
	if name == "" || f == nil {
		return configerrors.New(
			configerrors.CodeInvalidConfig,
			"invalid factory: name and factory must be provided",
		)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[name]; exists {
		return configerrors.Newf(
			configerrors.CodeAlreadyExists,
			"provider factory %q already registered",
			name,
		)
	}
	r.factories[name] = f
	return nil
}

// MustRegister adds a named provider factory to the DefaultRegistry.
// Panics if registration fails.
func MustRegister(name string, f Factory) {
	if err := DefaultRegistry.Register(name, f); err != nil {
		panic(err)
	}
}

// Create instantiates a provider by name using the provided configuration map.
// Returns an error if no factory is registered for the given name.
func (r *Registry) Create(name string, cfg map[string]any) (Provider, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, configerrors.Newf(
			configerrors.CodeNotFound,
			"provider factory %q not found",
			name,
		)
	}
	return f(cfg)
}

// Names returns a sorted list of all registered provider factory names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultRegistry is the global default provider registry.
// Use MustRegister to add custom providers.
var DefaultRegistry = NewRegistry()
