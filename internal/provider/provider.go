// Package provider provides infrastructure adapters for loading configuration
// from remote providers (Consul, etcd, NATS). Like the loader package, all
// registries are instance-based with NO global variables.
package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// Provider interface
// ---------------------------------------------------------------------------

// Provider loads configuration from a remote source (Consul, etcd, NATS, etc.).
// Implementations must be safe for concurrent use.
type Provider interface {
	// Load reads configuration from the remote source.
	Load(ctx context.Context) (map[string]value.Value, error)
	// Watch returns a channel of change events from the remote source.
	Watch(ctx context.Context) (<-chan event.Event, error)
	// String returns a human-readable name for this provider.
	String() string
	// Close releases any resources held by the provider.
	Close(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

// Factory creates a Provider from configuration. Factories are registered
// with an instance-based Registry, never a global variable.
type Factory func(cfg map[string]any) (Provider, error)

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry is an instance-based provider factory registry. Use NewRegistry to
// create one — there are NO global default registries.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates a new empty provider Registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a named Factory to the registry. Returns an error if a
// factory with the same name already exists.
func (r *Registry) Register(name string, f Factory) error {
	if name == "" {
		return errors.New(errors.CodeInvalidConfig, "provider factory name must not be empty")
	}
	if f == nil {
		return errors.New(errors.CodeInvalidConfig,
			fmt.Sprintf("provider factory %q must not be nil", name))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return errors.New(errors.CodeAlreadyExists,
			fmt.Sprintf("provider factory %q is already registered", name))
	}

	r.factories[name] = f
	return nil
}

// Create instantiates a Provider using the named factory and the given config.
// Returns an error if no factory is registered under the given name.
func (r *Registry) Create(name string, cfg map[string]any) (Provider, error) {
	r.mu.RLock()
	f, exists := r.factories[name]
	r.mu.RUnlock()

	if !exists {
		return nil, errors.New(errors.CodeNotFound,
			fmt.Sprintf("no provider factory registered for %q; available: %v",
				name, r.Names()))
	}

	prov, err := f(cfg)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeSource,
			fmt.Sprintf("create provider %q failed", name)).
			WithSource(name).
			WithOperation("create")
	}

	return prov, nil
}

// Names returns a sorted list of all registered factory names.
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

// Has returns true if a factory with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[name]
	return exists
}

// Unregister removes a factory from the registry. Returns true if the
// factory was found and removed, false otherwise.
func (r *Registry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; !exists {
		return false
	}
	delete(r.factories, name)
	return true
}

// Len returns the number of registered factories.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.factories)
}
