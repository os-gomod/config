// Package profile provides named configuration profiles for environment-specific
// settings. A Profile is a named collection of layers and options that can be
// applied to switch the configuration context (e.g., dev, staging, production).
package profile

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Profile represents a named collection of configuration layers and options.
// Profiles allow switching between environment-specific configurations
// (e.g., development, staging, production) without changing code.
type Profile struct {
	// Name is the unique profile name (e.g., "production", "development").
	Name string

	// Layers contains the layer definitions for this profile.
	// Each LayerDef describes a layer to be created when the profile is applied.
	Layers []LayerDef

	// Options contains arbitrary profile-specific options.
	// Common keys: "namespace", "strict_reload", "max_workers".
	Options map[string]any

	// Description is a human-readable description of this profile.
	Description string

	// Parent is the name of a parent profile to inherit from.
	// Layers from the parent are applied first, then this profile's layers.
	Parent string
}

// LayerDef describes a layer within a profile.
type LayerDef struct {
	// Name is the layer name.
	Name string

	// Type is the layer type (file, env, vault, consul, etc.).
	Type string

	// Source is the data source (file path, URI, etc.).
	Source string

	// Priority is the merge priority (higher overrides lower).
	Priority int

	// Enabled controls whether this layer is active.
	Enabled bool

	// Format is the content format (json, yaml, toml, hcl).
	// Empty means auto-detect from file extension.
	Format string

	// Opts contains additional layer-specific options.
	Opts map[string]any
}

// OptionString retrieves a string option from the profile's Options map.
// Returns the default value if the key is not found or not a string.
func (p *Profile) OptionString(key, defaultVal string) string {
	if p.Options == nil {
		return defaultVal
	}
	if v, ok := p.Options[key]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return defaultVal
}

// OptionBool retrieves a bool option from the profile's Options map.
func (p *Profile) OptionBool(key string, defaultVal bool) bool {
	if p.Options == nil {
		return defaultVal
	}
	if v, ok := p.Options[key]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b
		}
	}
	return defaultVal
}

// OptionInt retrieves an int option from the profile's Options map.
func (p *Profile) OptionInt(key string, defaultVal int) int {
	if p.Options == nil {
		return defaultVal
	}
	if v, ok := p.Options[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return defaultVal
}

// LayerNames returns the names of all layers in this profile.
func (p *Profile) LayerNames() []string {
	names := make([]string, len(p.Layers))
	for i, l := range p.Layers {
		names[i] = l.Name
	}
	return names
}

// Validate checks the profile for common configuration errors.
func (p *Profile) Validate() error {
	if p.Name == "" {
		return errors.New("profile: name must not be empty")
	}

	seen := make(map[string]bool)
	for _, l := range p.Layers {
		if l.Name == "" {
			return fmt.Errorf("profile %q: layer name must not be empty", p.Name)
		}
		if seen[l.Name] {
			return fmt.Errorf("profile %q: duplicate layer name %q", p.Name, l.Name)
		}
		seen[l.Name] = true
	}

	return nil
}

// String returns a human-readable summary of the profile.
func (p *Profile) String() string {
	var b strings.Builder

	b.WriteString("Profile{")
	b.WriteString(p.Name)
	if p.Description != "" {
		b.WriteString(", desc=")
		b.WriteString(p.Description)
	}
	if p.Parent != "" {
		b.WriteString(", parent=")
		b.WriteString(p.Parent)
	}

	b.WriteString(", layers=[")
	for i, l := range p.Layers {
		if i > 0 {
			b.WriteString(", ")
		}
		status := "enabled"
		if !l.Enabled {
			status = "disabled"
		}
		fmt.Fprintf(&b, "%s(%s, pri=%d, %s)", l.Name, l.Type, l.Priority, status)
	}
	b.WriteString("]")

	if len(p.Options) > 0 {
		b.WriteString(", opts={")
		keys := make([]string, 0, len(p.Options))
		for k := range p.Options {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s=%v", k, p.Options[k])
		}
		b.WriteString("}")
	}

	b.WriteString("}")
	return b.String()
}

// NewProfile creates a new Profile with the given name and layers.
func NewProfile(name string, layers []LayerDef, opts map[string]any) *Profile {
	if layers == nil {
		layers = make([]LayerDef, 0)
	}
	if opts == nil {
		opts = make(map[string]any)
	}
	return &Profile{
		Name:    name,
		Layers:  layers,
		Options: opts,
	}
}

// MergeLayers merges layers from a parent profile into this profile.
// Layers from the parent come first, then this profile's layers override.
func (p *Profile) MergeLayers(parent *Profile) {
	if parent == nil {
		return
	}

	// Prepend parent layers.
	merged := make([]LayerDef, 0, len(parent.Layers)+len(p.Layers))
	merged = append(merged, parent.Layers...)

	// Collect existing layer names from parent.
	parentNames := make(map[string]bool)
	for _, l := range parent.Layers {
		parentNames[l.Name] = true
	}

	// Add this profile's layers (skip duplicates with parent).
	for _, l := range p.Layers {
		if !parentNames[l.Name] {
			merged = append(merged, l)
		}
	}

	p.Layers = merged

	// Merge options (parent values as defaults, this profile overrides).
	if parent.Options != nil {
		for k, v := range parent.Options {
			if _, exists := p.Options[k]; !exists {
				p.Options[k] = v
			}
		}
	}
}

// Registry holds named profiles and provides lookup and listing.
type Registry struct {
	profiles map[string]*Profile
}

// NewRegistry creates an empty profile Registry.
func NewRegistry() *Registry {
	return &Registry{
		profiles: make(map[string]*Profile),
	}
}

// Register adds a profile to the registry.
func (r *Registry) Register(p *Profile) error {
	if p == nil || p.Name == "" {
		return errors.New("profile registry: profile name required")
	}
	if _, exists := r.profiles[p.Name]; exists {
		return fmt.Errorf("profile registry: profile %q already registered", p.Name)
	}
	r.profiles[p.Name] = p
	return nil
}

// Get returns the profile with the given name, or nil.
func (r *Registry) Get(name string) *Profile {
	return r.profiles[name]
}

// List returns all profile names sorted alphabetically.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.profiles))
	for name := range r.profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Resolve resolves a profile and its parent chain into a fully merged profile.
func (r *Registry) Resolve(name string) (*Profile, error) {
	p, ok := r.profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile registry: profile %q not found", name)
	}

	// Track visited profiles to detect cycles.
	visited := make(map[string]bool)
	return r.resolveChain(p, visited)
}

func (r *Registry) resolveChain(p *Profile, visited map[string]bool) (*Profile, error) {
	if visited[p.Name] {
		return nil, fmt.Errorf("profile registry: circular dependency detected: %q", p.Name)
	}
	visited[p.Name] = true

	// Create a copy to avoid mutating the original.
	result := &Profile{
		Name:        p.Name,
		Description: p.Description,
		Parent:      p.Parent,
		Layers:      make([]LayerDef, len(p.Layers)),
		Options:     make(map[string]any, len(p.Options)),
	}
	copy(result.Layers, p.Layers)
	for k, v := range p.Options {
		result.Options[k] = v
	}

	// Resolve parent if present.
	if p.Parent != "" {
		parent, ok := r.profiles[p.Parent]
		if !ok {
			return nil, fmt.Errorf("profile registry: parent profile %q not found", p.Parent)
		}
		resolvedParent, err := r.resolveChain(parent, visited)
		if err != nil {
			return nil, err
		}
		result.MergeLayers(resolvedParent)
	}

	return result, nil
}
