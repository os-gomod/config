package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/eventbus"
	"github.com/os-gomod/config/v2/internal/interceptor"
	"github.com/os-gomod/config/v2/internal/observability"
	"github.com/os-gomod/config/v2/internal/pipeline"
	"github.com/os-gomod/config/v2/internal/registry"
	"github.com/os-gomod/config/v2/internal/service"
)

// Config is the public facade for the configuration system.
// It delegates to bounded services and provides a backward-compatible API.
//
// Architecture:
//   - QueryService: read-only state access
//   - MutationService: set/delete/batch_set via pipeline
//   - RuntimeService: reload/bind/watch/lifecycle via pipeline
//   - PluginService: plugin registration
//
// All operations route through the Command Pipeline for centralized orchestration.
type Config struct {
	query    *service.QueryService
	mutation *service.MutationService
	runtime  *service.RuntimeService
	plugins  *service.PluginService
	bus      *eventbus.Bus
}

// New creates a new Config instance.
// It wires up all services, the pipeline, and the event bus.
func New(ctx context.Context, opts ...Option) (*Config, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// 1. Registry bundle
	bundle := cfg.bundle
	if bundle == nil {
		bundle = registry.NewDefaultBundle()
	}

	// 2. Event bus
	busOpts := []eventbus.Option{
		eventbus.WithWorkerCount(cfg.busWorkers),
		eventbus.WithQueueSize(cfg.busQueueSize),
	}
	bus := eventbus.NewBus(busOpts...) //nolint:contextcheck // NewBus creates internal context, safe

	// 3. Core engine
	engine := service.NewCoreEngine(cfg.layers,
		service.WithMaxWorkers(cfg.maxWorkers),
		service.WithDeltaReload(cfg.deltaReload),
	)

	// 4. Interceptor chain
	chain := interceptor.NewChain()

	// 5. Pipeline with middleware
	pipe := pipeline.New(
		pipeline.WithLogger(cfg.logger),
		pipeline.WithMetricsRecorder(cfg.recorder),
		pipeline.WithTracer(cfg.tracer),
	)

	// 6. Services
	querySvc := service.NewQueryService(engine)
	mutationSvc := service.NewMutationService(
		pipe, engine, bus, chain, cfg.recorder,
		service.WithMutationNamespace(cfg.namespace),
	)
	runtimeSvc := service.NewRuntimeService(
		pipe, engine, bus, chain, cfg.recorder,
		service.WithOnReloadError(cfg.onReloadErr),
	)
	pluginSvc := service.NewPluginService(bundle, bus, engine, cfg.recorder)

	c := &Config{
		query:    querySvc,
		mutation: mutationSvc,
		runtime:  runtimeSvc,
		plugins:  pluginSvc,
		bus:      bus,
	}

	// 7. Register plugins
	for _, p := range cfg.plugins {
		if err := c.plugins.Register(p); err != nil {
			_ = c.runtime.Close(ctx)
			bus.Close()
			return nil, errors.Build(
				errors.CodeInternal,
				fmt.Sprintf("plugin %q registration failed", p.Name()),
				errors.WithOperation("config.new"),
			).Wrap(err)
		}
	}

	// 8. Initial reload
	reloadResult, err := c.runtime.Reload(ctx)
	if err != nil {
		_ = c.runtime.Close(ctx)
		bus.Close()
		return nil, errors.Build(
			errors.CodeInternal,
			"initial reload failed",
			errors.WithOperation("config.new"),
		).Wrap(err)
	}
	if reloadResult.HasErrors() {
		if cfg.strictReload {
			_ = c.runtime.Close(ctx)
			bus.Close()
			return nil, errors.Build(
				errors.CodeSource,
				fmt.Sprintf("strict reload: %d layer(s) failed", len(reloadResult.LayerErrs)),
				errors.WithOperation("config.new"),
			)
		}
		// Non-strict: return with warning
	}

	return c, nil
}

// Get retrieves a single config value by key.
func (c *Config) Get(key string) (value.Value, bool) {
	return c.query.Get(key)
}

// GetAll returns a copy of all config values.
func (c *Config) GetAll() map[string]value.Value {
	return c.query.GetAll()
}

// Has checks if a key exists in the configuration.
func (c *Config) Has(key string) bool {
	return c.query.Has(key)
}

// Set sets a config value. Routes through the pipeline.
func (c *Config) Set(ctx context.Context, key string, raw any) error {
	return c.mutation.Set(ctx, key, raw)
}

// Delete removes a config value. Routes through the pipeline.
func (c *Config) Delete(ctx context.Context, key string) error {
	return c.mutation.Delete(ctx, key)
}

// BatchSet sets multiple values atomically. Routes through the pipeline.
func (c *Config) BatchSet(ctx context.Context, kv map[string]any) error {
	return c.mutation.BatchSet(ctx, kv)
}

// Reload reloads configuration from all layers. Routes through the pipeline.
func (c *Config) Reload(ctx context.Context) (*service.ReloadResult, error) {
	result, err := c.runtime.Reload(ctx)
	return &result, err
}

// Bind binds config values to a target struct. Routes through the pipeline.
func (c *Config) Bind(ctx context.Context, target any) error {
	return c.runtime.Bind(ctx, target)
}

// Subscribe registers an observer for all events.
func (c *Config) Subscribe(obs event.Observer) func() {
	return c.bus.Subscribe("", obs)
}

// WatchPattern registers an observer for events matching a pattern.
func (c *Config) WatchPattern(pattern string, obs event.Observer) func() {
	return c.bus.Subscribe(pattern, obs)
}

// Close shuts down the config system gracefully.
func (c *Config) Close(ctx context.Context) error {
	return c.runtime.Close(ctx)
}

// Validate validates a target struct.
func (c *Config) Validate(ctx context.Context, target any) error {
	return c.mutation.Validate(ctx, target)
}

// Plugins returns the names of registered plugins.
func (c *Config) Plugins() []string {
	return c.plugins.Plugins()
}

// Snapshot returns a redacted copy of all config values.
func (c *Config) Snapshot() map[string]value.Value {
	return c.query.Snapshot()
}

// Restore replaces the entire config state.
func (c *Config) Restore(data map[string]value.Value) {
	c.mutation.Restore(data)
}

// Explain returns a human-readable explanation of a config value.
func (c *Config) Explain(key string) string {
	return c.query.Explain(key)
}

// Version returns the current config state version.
func (c *Config) Version() uint64 {
	return c.query.Version()
}

// Len returns the number of config keys.
func (c *Config) Len() int {
	return c.query.Len()
}

// Keys returns all config keys in sorted order.
func (c *Config) Keys() []string {
	return c.query.Keys()
}

// SetNamespace updates the namespace prefix.
func (c *Config) SetNamespace(ctx context.Context, ns string) error {
	return c.mutation.SetNamespace(ctx, ns)
}

// Namespace returns the current namespace.
func (c *Config) Namespace() string {
	return c.mutation.Namespace()
}

func defaultConfig() configBuilder {
	return configBuilder{
		maxWorkers:      8,
		busWorkers:      32,
		busQueueSize:    4096,
		defaultDebounce: 2 * time.Second,
		recorder:        observability.Nop(),
		logger:          slog.Default(),
	}
}
