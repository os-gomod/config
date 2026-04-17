package plugin

import (
	"fmt"
	"sync"

	configerrors "github.com/os-gomod/config/errors"
)

// Registry manages a collection of plugins and ensures each plugin name is unique.
// Plugins are initialized immediately upon registration. If initialization fails,
// the plugin is rolled back and removed from the registry.
// The registry is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	plugins []Plugin
	names   map[string]struct{}
}

// NewRegistry creates an empty plugin Registry.
func NewRegistry() *Registry {
	return &Registry{
		names: make(map[string]struct{}),
	}
}

// Register adds a plugin to the registry and calls its Init method with the
// provided Host. If a plugin with the same name already exists, or if Init
// returns an error, the plugin is not registered and an error is returned.
// On Init failure, the plugin is fully rolled back.
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
