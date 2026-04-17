package loader

import (
	"sort"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// Registry manages a collection of loader Factory functions indexed by name.
// It enables dynamic loader instantiation by name using configuration maps.
// The registry is safe for concurrent use.
//
// Example:
//
//	reg := loader.NewRegistry()
//	reg.Register("file", func(cfg map[string]any) (loader.Loader, error) {
//	    return loader.NewFileLoader(cfg["path"].(string)), nil
//	})
//	l, err := reg.Create("file", map[string]any{"path": "/etc/config.yaml"})
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates an empty loader Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a named loader factory to the registry.
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
			"loader factory %q already registered",
			name,
		)
	}
	r.factories[name] = f
	return nil
}

// MustRegister adds a named loader factory to the DefaultRegistry.
// Panics if registration fails.
func MustRegister(name string, f Factory) {
	if err := DefaultRegistry.Register(name, f); err != nil {
		panic(err)
	}
}

// Create instantiates a loader by name using the provided configuration map.
// Returns an error if no factory is registered for the given name.
func (r *Registry) Create(name string, cfg map[string]any) (Loader, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, configerrors.Newf(
			configerrors.CodeNotFound,
			"loader factory %q not found",
			name,
		)
	}
	return f(cfg)
}

// Names returns a sorted list of all registered loader factory names.
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

// DefaultRegistry is the global default loader registry.
// Use MustRegister to add custom loaders.
var DefaultRegistry = NewRegistry()
