package plugin

import (
	"fmt"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// Registry manages registered plugins and their initialisation.
//
// Lock ordering: mu is the only lock held by Registry; no nested locking.
type Registry struct {
	mu      sync.RWMutex // protects plugins
	plugins []Plugin
	names   map[string]struct{}
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		names: make(map[string]struct{}),
	}
}

// Register initializes p and records it.
// Returns an error wrapping ErrAlreadyExists if a plugin with the same Name()
// is already registered.
func (r *Registry) Register(p Plugin, h Host) error {
	name := p.Name()
	r.mu.Lock()
	if _, exists := r.names[name]; exists {
		r.mu.Unlock()
		return configerrors.Newf(
			configerrors.CodeAlreadyExists,
			"plugin %q already registered",
			name,
		)
	}
	r.names[name] = struct{}{}
	r.plugins = append(r.plugins, p)
	r.mu.Unlock()

	if err := p.Init(h); err != nil {
		// Roll back registration on init failure.
		r.mu.Lock()
		delete(r.names, name)
		for i, pl := range r.plugins {
			if pl.Name() == name {
				r.plugins = append(r.plugins[:i], r.plugins[i+1:]...)
				break
			}
		}
		r.mu.Unlock()
		return fmt.Errorf("plugin: init %q: %w", name, err)
	}
	return nil
}

// Plugins returns the names of all registered plugins in registration order.
func (r *Registry) Plugins() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, len(r.plugins))
	for i, p := range r.plugins {
		names[i] = p.Name()
	}
	return names
}
