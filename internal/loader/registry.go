package loader

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages named loader factories.
// It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a named factory to the registry.
// Returns an error if a factory with the same name is already registered.
func (r *Registry) Register(name string, f Factory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("loader: factory %q already registered", name)
	}
	r.factories[name] = f
	return nil
}

// Get returns the factory registered under the given name.
// Returns nil if no factory is found.
func (r *Registry) Get(name string) Factory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.factories[name]
}

// Names returns a sorted list of all registered factory names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := mapKeys(r.factories)
	sort.Strings(names)
	return names
}

// Create instantiates a loader by name using the provided options.
func (r *Registry) Create(name string, opts map[string]any) (Loader, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("loader: no factory registered for %q", name)
	}
	return f(opts)
}

func mapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
