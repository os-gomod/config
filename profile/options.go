package profile

// ProfileOption configures a Profile during creation.
type ProfileOption func(*Profile)

// WithName overrides the profile name.
func WithName(name string) ProfileOption {
	return func(p *Profile) { p.Name = name }
}

// WithLayers appends layer specifications to the profile.
func WithLayers(layers ...LayerSpec) ProfileOption {
	return func(p *Profile) { p.Layers = append(p.Layers, layers...) }
}
