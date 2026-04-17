package profile

// Option is a functional option for configuring a Profile.
type Option func(*Profile)

// WithName sets the profile name.
func WithName(name string) Option {
	return func(p *Profile) { p.Name = name }
}

// WithLayers appends additional layer specifications to the profile.
func WithLayers(layers ...LayerSpec) Option {
	return func(p *Profile) { p.Layers = append(p.Layers, layers...) }
}
