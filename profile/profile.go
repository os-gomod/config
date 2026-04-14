// Package profile provides named configuration profiles that map to predefined
// layer stacks. A Profile bundles a set of loaders and layer priorities so that
// common configurations (e.g. "development", "production") can be instantiated
// with a single call.
package profile

import (
	"fmt"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/loader"
)

// Profile represents a named, pre-configured set of config layers.
// It defines which loaders to use and how they are layered for a particular
// environment or deployment context (e.g. development, staging, production).
type Profile struct {
	// Name is the unique profile identifier (e.g. "production").
	Name string
	// Layers holds the ordered list of layer specifications for this profile.
	Layers []LayerSpec
}

// LayerSpec describes a single layer within a Profile.
type LayerSpec struct {
	// Name is the layer's identifier.
	Name string
	// Priority is the merge priority; higher values win on conflicts.
	Priority int
	// Source is the loader.Loader that provides data for this layer.
	Source loader.Loader
}

// New creates a Profile with the given name and layer specifications.
func New(name string, layers ...LayerSpec) *Profile {
	return &Profile{Name: name, Layers: layers}
}

// Apply configures the Engine with the profile's layers. Each LayerSpec is
// converted into a core.Layer and added to the engine in the order specified
// by the profile.
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

// MemoryProfile returns a profile backed by an in-memory loader pre-populated
// with the given data. It is primarily used in tests and for runtime overrides.
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

// FileProfile returns a profile backed by a single file loader.
// The file format is auto-detected from the extension.
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

// EnvProfile returns a profile backed by an environment variable loader.
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
