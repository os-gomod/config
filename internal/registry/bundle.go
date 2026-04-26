// Package registry provides dependency-injected registry bundles.
// All registries are instance-based — no global state.
package registry

import (
	"github.com/os-gomod/config/v2/internal/decoder"
	"github.com/os-gomod/config/v2/internal/loader"
	"github.com/os-gomod/config/v2/internal/provider"
)

// Bundle holds all dependency-injected registries.
// This replaces all global DefaultRegistry variables from v1.
// Every dependency is explicit and injectable.
type Bundle struct {
	Loader   *loader.Registry
	Decoder  *decoder.Registry
	Provider *provider.Registry
}

// BundleOption configures a Bundle.
type BundleOption func(*Bundle)

// NewBundle creates a Bundle with the given options.
func NewBundle(opts ...BundleOption) *Bundle {
	b := &Bundle{
		Loader:   loader.NewRegistry(),
		Decoder:  decoder.NewRegistry(),
		Provider: provider.NewRegistry(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// NewDefaultBundle creates a Bundle pre-loaded with standard decoders
// (YAML, JSON, TOML, Env) and an empty loader/provider registry.
func NewDefaultBundle() *Bundle {
	return NewBundle(
		WithDecoderRegistry(decoder.NewDefaultRegistry()),
	)
}

// WithLoaderRegistry sets the loader registry.
func WithLoaderRegistry(r *loader.Registry) BundleOption {
	return func(b *Bundle) { b.Loader = r }
}

// WithDecoderRegistry sets the decoder registry.
func WithDecoderRegistry(r *decoder.Registry) BundleOption {
	return func(b *Bundle) { b.Decoder = r }
}

// WithProviderRegistry sets the provider registry.
func WithProviderRegistry(r *provider.Registry) BundleOption {
	return func(b *Bundle) { b.Provider = r }
}
