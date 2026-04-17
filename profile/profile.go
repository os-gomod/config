// Package profile provides configuration profiles that define a named set of
// configuration layers with their priorities and sources. Profiles simplify
// the setup of common configuration combinations (e.g., file + env + defaults)
// and can be applied to a configuration engine in a single call.
//
// Built-in profile constructors:
//   - MemoryProfile: in-memory default values
//   - FileProfile: file-based configuration
//   - EnvProfile: environment variable configuration
package profile

import (
	"fmt"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/loader"
)

// Profile defines a named collection of configuration layers that can be applied
// to a configuration engine. Each layer specifies its name, priority, and data source.
type Profile struct {
	Name   string
	Layers []LayerSpec
}

// LayerSpec defines a single configuration layer within a profile.
type LayerSpec struct {
	Name     string        // Layer name for identification
	Priority int           // Layer priority (higher values take precedence)
	Source   loader.Loader // Configuration data source
}

// New creates a new Profile with the given name and layer specifications.
func New(name string, layers ...LayerSpec) *Profile {
	return &Profile{Name: name, Layers: layers}
}

// Apply adds all layers from this profile to the given configuration engine.
// Returns an error if any layer fails to be added.
func (p *Profile) Apply(e *core.Engine) error {
	for _, spec := range p.Layers {
		layer := core.NewLayer(spec.Name,
			core.WithLayerPriority(spec.Priority),
			core.WithLayerSource(spec.Source),
		)
		if err := e.AddLayer(layer); err != nil {
			return fmt.Errorf("profile: add layer %q: %w", spec.Name, err)
		}
	}
	return nil
}

// MemoryProfile creates a profile backed by an in-memory loader with the given data.
func MemoryProfile(name string, data map[string]any, priority int) *Profile {
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(priority),
	)
	return New(name, LayerSpec{
		Name:     "memory",
		Priority: priority,
		Source:   ml,
	})
}

// FileProfile creates a profile backed by a file loader at the given path.
func FileProfile(name, path string, priority int) *Profile {
	fl := loader.NewFileLoader(path,
		loader.WithFilePriority(priority),
	)
	return New(name, LayerSpec{
		Name:     "file",
		Priority: priority,
		Source:   fl,
	})
}

// EnvProfile creates a profile backed by an environment variable loader
// with the given prefix.
func EnvProfile(name, prefix string, priority int) *Profile {
	el := loader.NewEnvLoader(
		loader.WithEnvPrefix(prefix),
		loader.WithEnvPriority(priority),
	)
	return New(name, LayerSpec{
		Name:     "env",
		Priority: priority,
		Source:   el,
	})
}
