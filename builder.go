package config

import (
	"context"
	"fmt"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/observability"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/provider"
	"github.com/os-gomod/config/validator"
)

// Builder constructs a Config using a fluent, method-chaining API.
// The zero value is not usable; always start with config.NewBuilder().
type Builder struct {
	ctx         context.Context
	loaders     []loader.Loader
	providers   []provider.Provider
	watchEnable bool
	val         validator.Validator
	strict      bool
	onReloadErr func(error)
	recorder    observability.Recorder
	plugins     []plugin.Plugin
}

// NewBuilder returns a Builder with context.Background().
func NewBuilder() *Builder {
	return &Builder{
		ctx: context.Background(),
	}
}

// WithContext replaces the builder's base context.
func (b *Builder) WithContext(ctx context.Context) *Builder {
	b.ctx = ctx
	return b
}

// File adds a FileLoader. Format is auto-detected from the file extension.
func (b *Builder) File(path string) *Builder {
	return b.FileWithPriority(path, 30)
}

// FileWithPriority adds a FileLoader with an explicit merge priority.
func (b *Builder) FileWithPriority(path string, priority int) *Builder {
	fl := loader.NewFileLoader(path, loader.WithFilePriority(priority))
	b.loaders = append(b.loaders, fl)
	return b
}

// Env adds an EnvLoader with the given prefix.
func (b *Builder) Env(prefix string) *Builder {
	return b.EnvWithPriority(prefix, 40)
}

// EnvWithPriority adds an EnvLoader with an explicit merge priority.
func (b *Builder) EnvWithPriority(prefix string, priority int) *Builder {
	el := loader.NewEnvLoader(
		loader.WithEnvPrefix(prefix),
		loader.WithEnvPriority(priority),
	)
	b.loaders = append(b.loaders, el)
	return b
}

// Memory adds a MemoryLoader.
func (b *Builder) Memory(data map[string]any) *Builder {
	return b.MemoryWithPriority(data, 20)
}

// MemoryWithPriority adds a MemoryLoader with an explicit merge priority.
func (b *Builder) MemoryWithPriority(data map[string]any, priority int) *Builder {
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(priority),
	)
	b.loaders = append(b.loaders, ml)
	return b
}

// Remote adds a provider by registered name.
// The name must be registered in provider.DefaultRegistry.
func (b *Builder) Remote(name string, cfg map[string]any) *Builder {
	p, err := provider.DefaultRegistry.Create(name, cfg)
	if err != nil {
		// Store the error; Build() will report it.
		return b
	}
	b.providers = append(b.providers, p)
	return b
}

// Watch enables background watching for all watchable loaders and providers.
func (b *Builder) Watch() *Builder {
	b.watchEnable = true
	return b
}

// Validate sets the validator run after Bind.
func (b *Builder) Validate(v validator.Validator) *Builder {
	b.val = v
	return b
}

// StrictReload causes Build to return an error if any layer fails initial load.
func (b *Builder) StrictReload() *Builder {
	b.strict = true
	return b
}

// OnReloadError sets the callback invoked on background reload failure.
// Default: log at slog WARN level.
func (b *Builder) OnReloadError(fn func(error)) *Builder {
	b.onReloadErr = fn
	return b
}

// Recorder sets the observability recorder.
func (b *Builder) Recorder(r observability.Recorder) *Builder {
	b.recorder = r
	return b
}

// Plugin registers a plugin applied during Build.
func (b *Builder) Plugin(p plugin.Plugin) *Builder {
	b.plugins = append(b.plugins, p)
	return b
}

// Build creates and returns a *Config.
func (b *Builder) Build() (*Config, error) {
	opts := []Option{
		WithMaxWorkers(8),
	}

	if b.val != nil {
		opts = append(opts, WithValidator(b.val))
	}
	if b.strict {
		opts = append(opts, WithStrictReload())
	}
	if b.onReloadErr != nil {
		opts = append(opts, WithReloadErrorHandler(b.onReloadErr))
	}
	if b.recorder != nil {
		opts = append(opts, WithRecorder(b.recorder))
	}

	// Build layers from loaders.
	for _, l := range b.loaders {
		layer := core.NewLayer(l.String(),
			core.WithLayerPriority(l.Priority()),
			core.WithLayerSource(l),
		)
		opts = append(opts, WithLayer(layer))
	}

	// Build layers from providers.
	for _, p := range b.providers {
		layer := core.NewLayer(p.Name(),
			core.WithLayerPriority(p.Priority()),
			core.WithLayerSource(p),
		)
		opts = append(opts, WithLayer(layer))
	}

	// Register plugins.
	for _, p := range b.plugins {
		opts = append(opts, WithPlugin(p))
	}

	c, err := New(b.ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("config: build: %w", err)
	}

	// Start watching if requested.
	if b.watchEnable {
		for _, l := range b.loaders {
			c.startWatch(b.ctx, l)
		}
		for _, p := range b.providers {
			c.startWatch(b.ctx, p)
		}
	}

	return c, nil
}

// MustBuild is like Build but panics on error.
func (b *Builder) MustBuild() *Config {
	c, err := b.Build()
	if err != nil {
		panic(err)
	}
	return c
}

// Bind builds the Config and immediately binds it into target.
// Validation is run if a Validator was set.
func (b *Builder) Bind(ctx context.Context, target any) (*Config, error) {
	c, err := b.Build()
	if err != nil {
		return nil, err
	}
	if bindErr := c.Bind(ctx, target); bindErr != nil {
		return nil, fmt.Errorf("config: bind: %w", bindErr)
	}
	return c, nil
}

// MustBind is like Bind but panics on error.
func (b *Builder) MustBind(ctx context.Context, target any) *Config {
	c, err := b.Bind(ctx, target)
	if err != nil {
		panic(err)
	}
	return c
}
