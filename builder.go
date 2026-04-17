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

// Builder provides a fluent API for constructing a [Config] instance.
// It offers convenience methods for registering common configuration sources
// (files, environment variables, memory, remote providers) with sensible
// default priorities, and supports optional features like file watching,
// validation, strict reload mode, and observability.
//
// # Example
//
//	cfg, err := config.NewBuilder().
//	    File("config.yaml").
//	    Env("APP").
//	    Memory(map[string]any{"debug": true}).
//	    Watch().
//	    Build()
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

// NewBuilder creates a new [Builder] with context.Background as the default
// context.
func NewBuilder() *Builder {
	return &Builder{
		ctx: context.Background(),
	}
}

// WithContext sets the context used during [Build]. The same context is also
// passed to file watchers if [Watch] is enabled.
func (b *Builder) WithContext(ctx context.Context) *Builder {
	b.ctx = ctx
	return b
}

// File adds a file-based configuration source with a default priority of 30.
// The file format is auto-detected from the extension (e.g., .json, .yaml).
func (b *Builder) File(path string) *Builder {
	return b.FileWithPriority(path, 30)
}

// FileWithPriority adds a file-based configuration source with the specified
// priority. Higher priority values take precedence over lower ones during
// merging.
func (b *Builder) FileWithPriority(path string, priority int) *Builder {
	fl := loader.NewFileLoader(path, loader.WithFilePriority(priority))
	b.loaders = append(b.loaders, fl)
	return b
}

// Env adds an environment-variable configuration source with the given prefix
// and a default priority of 40. Only variables whose names start with the
// prefix (case-insensitive) are loaded.
func (b *Builder) Env(prefix string) *Builder {
	return b.EnvWithPriority(prefix, 40)
}

// EnvWithPriority adds an environment-variable configuration source with the
// given prefix and priority.
func (b *Builder) EnvWithPriority(prefix string, priority int) *Builder {
	el := loader.NewEnvLoader(
		loader.WithEnvPrefix(prefix),
		loader.WithEnvPriority(priority),
	)
	b.loaders = append(b.loaders, el)
	return b
}

// Memory adds an in-memory configuration source with the given data and a
// default priority of 20.
func (b *Builder) Memory(data map[string]any) *Builder {
	return b.MemoryWithPriority(data, 20)
}

// MemoryWithPriority adds an in-memory configuration source with the given
// data and priority.
func (b *Builder) MemoryWithPriority(data map[string]any, priority int) *Builder {
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(priority),
	)
	b.loaders = append(b.loaders, ml)
	return b
}

// Remote adds a remote configuration provider registered under the given name.
// The cfg map is provider-specific configuration (e.g., endpoint URLs,
// authentication tokens). If the provider cannot be created, the call is
// silently ignored.
func (b *Builder) Remote(name string, cfg map[string]any) *Builder {
	p, err := provider.DefaultRegistry.Create(name, cfg)
	if err != nil {
		return b
	}
	b.providers = append(b.providers, p)
	return b
}

// Watch enables real-time file watching for all registered loaders and
// providers. When a source file changes, a debounced reload is triggered
// automatically.
func (b *Builder) Watch() *Builder {
	b.watchEnable = true
	return b
}

// Validate sets a custom validator used during struct binding.
func (b *Builder) Validate(v validator.Validator) *Builder {
	b.val = v
	return b
}

// StrictReload enables strict reload mode. When enabled, [Build] returns an
// error if any layer fails during the initial reload.
func (b *Builder) StrictReload() *Builder {
	b.strict = true
	return b
}

// OnReloadError sets a custom handler invoked when a background watcher
// triggers a reload that fails.
func (b *Builder) OnReloadError(fn func(error)) *Builder {
	b.onReloadErr = fn
	return b
}

// Recorder sets an observability recorder for tracking configuration
// operations.
func (b *Builder) Recorder(r observability.Recorder) *Builder {
	b.recorder = r
	return b
}

// Plugin registers a plugin that will be initialized when [Build] is called.
func (b *Builder) Plugin(p plugin.Plugin) *Builder {
	b.plugins = append(b.plugins, p)
	return b
}

// Build constructs the [Config] instance from all registered sources and
// options. If [Watch] was called, background watchers are started for all
// loaders and providers that support watching.
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
	for _, l := range b.loaders {
		layer := core.NewLayer(l.String(),
			core.WithLayerPriority(l.Priority()),
			core.WithLayerSource(l),
		)
		opts = append(opts, WithLayer(layer))
	}
	for _, p := range b.providers {
		layer := core.NewLayer(p.Name(),
			core.WithLayerPriority(p.Priority()),
			core.WithLayerSource(p),
		)
		opts = append(opts, WithLayer(layer))
	}
	for _, p := range b.plugins {
		opts = append(opts, WithPlugin(p))
	}
	c, err := New(b.ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("config: build: %w", err)
	}
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

// MustBuild is like [Build] but panics on error.
func (b *Builder) MustBuild() *Config {
	c, err := b.Build()
	if err != nil {
		panic(err)
	}
	return c
}

// Bind builds the [Config] and immediately binds the current configuration
// state to the given target struct. Returns both the Config and any bind
// error.
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

// MustBind is like [Bind] but panics on error.
func (b *Builder) MustBind(ctx context.Context, target any) *Config {
	c, err := b.Bind(ctx, target)
	if err != nil {
		panic(err)
	}
	return c
}
