package provider

import (
	"sort"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// Registry maps names to Provider factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a named factory. Returns an error if the name is already taken.
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

// MustRegister is like Register but panics on error.
func MustRegister(name string, f Factory) {
	if err := DefaultRegistry.Register(name, f); err != nil {
		panic(err)
	}
}

// Create instantiates a Provider by name using cfg.
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

// Names returns all registered names in sorted order.
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

// DefaultRegistry is the package-level registry.
// This is the only permitted package-level var in provider/.
var DefaultRegistry = NewRegistry()
