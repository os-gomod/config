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
// It is an alternative to the functional options used by [New], offering
// a more readable chain of method calls.
//
// Example:
//
//	cfg, err := config.NewBuilder().
//	    File("config.yaml").
//	    Env("APP").
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

// NewBuilder creates a new Builder with sensible defaults.
func NewBuilder() *Builder {
	return &Builder{
		ctx: context.Background(),
	}
}

// WithContext sets the context used during Build. Defaults to context.Background().
func (b *Builder) WithContext(ctx context.Context) *Builder {
	b.ctx = ctx
	return b
}

// File adds a file-based configuration source with [PriorityFile] priority.
func (b *Builder) File(path string) *Builder {
	return b.FileWithPriority(path, PriorityFile)
}

// FileWithPriority adds a file-based configuration source with a custom priority.
func (b *Builder) FileWithPriority(path string, priority int) *Builder {
	fl := loader.NewFileLoader(path, loader.WithFilePriority(priority))
	b.loaders = append(b.loaders, fl)
	return b
}

// Env adds an environment variable source with the given prefix and [PriorityEnv] priority.
func (b *Builder) Env(prefix string) *Builder {
	return b.EnvWithPriority(prefix, PriorityEnv)
}

// EnvWithPriority adds an environment variable source with a custom priority.
func (b *Builder) EnvWithPriority(prefix string, priority int) *Builder {
	el := loader.NewEnvLoader(
		loader.WithEnvPrefix(prefix),
		loader.WithEnvPriority(priority),
	)
	b.loaders = append(b.loaders, el)
	return b
}

// Memory adds an in-memory configuration source with [PriorityMemory] priority.
func (b *Builder) Memory(data map[string]any) *Builder {
	return b.MemoryWithPriority(data, PriorityMemory)
}

// MemoryWithPriority adds an in-memory configuration source with a custom priority.
func (b *Builder) MemoryWithPriority(data map[string]any, priority int) *Builder {
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(data),
		loader.WithMemoryPriority(priority),
	)
	b.loaders = append(b.loaders, ml)
	return b
}

// Remote adds a remote provider created from the provider registry by name.
// If the provider factory is not found, the call is silently ignored.
// Remote adds a remote provider created from the provider registry by name.
// If the provider factory is not found, the call is silently ignored.
func (b *Builder) Remote(name string, cfg map[string]any) *Builder {
	p, err := provider.DefaultRegistry.Create(name, cfg)
	if err != nil {
		return b
	}
	b.providers = append(b.providers, p)
	return b
}

// Watch enables file system and remote provider watching. When enabled,
// the Build method starts goroutines that watch all sources for changes
// and trigger automatic reloads.
// Watch enables file system and remote provider watching. When enabled,
// the Build method starts goroutines that watch all sources for changes
// and trigger automatic reloads.
func (b *Builder) Watch() *Builder {
	b.watchEnable = true
	return b
}

// Validate sets a custom validator for post-binding validation.
func (b *Builder) Validate() *Builder {
	b.val = validator.New()
	return b
}

func (b *Builder) ValidateWith(v validator.Validator) *Builder {
	b.val = v
	return b
}

// StrictReload enables strict reload mode where layer failures cause
// the reload to return an error.
// StrictReload enables strict reload mode where layer failures cause
// the reload to return an error.
func (b *Builder) StrictReload() *Builder {
	b.strict = true
	return b
}

// OnReloadError sets a custom error handler for background reload failures.
func (b *Builder) OnReloadError(fn func(error)) *Builder {
	b.onReloadErr = fn
	return b
}

// Recorder sets the observability recorder for metrics collection.
func (b *Builder) Recorder(r observability.Recorder) *Builder {
	b.recorder = r
	return b
}

// Plugin adds a configuration plugin.
func (b *Builder) Plugin(p plugin.Plugin) *Builder {
	b.plugins = append(b.plugins, p)
	return b
}

// Build creates a new [Config] from the accumulated builder options.
// It performs an initial reload and returns an error if it fails.
// Build creates a new [Config] from the accumulated builder options.
// It performs an initial reload and returns an error if it fails.
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

// Bind builds the Config and immediately binds the merged configuration
// to the target struct. Returns an error if either Build or Bind fails.
// Bind builds the Config and immediately binds the merged configuration
// to the target struct. Returns an error if either Build or Bind fails.
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
