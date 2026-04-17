// Package config provides a production-grade, layered configuration management
// library for Go applications. It supports loading configuration from multiple
// sources (files, environment variables, remote providers), merging them by
// priority, watching for changes in real-time, and binding to native Go structs.
//
// # Quick Start
//
//	cfg, err := config.New(context.Background(),
//	    config.WithLoader(loader.NewMemoryLoader(
//	        loader.WithMemoryData(map[string]any{"host": "localhost", "port": 8080}),
//	    )),
//	)
//	if err != nil { log.Fatal(err) }
//	defer cfg.Close(context.Background())
//
//	host, _ := cfg.GetString("host")
//	port, _ := cfg.GetInt("port")
//
// # Key Concepts
//
//   - Layers: Configuration sources merged by priority (higher wins).
//   - Values: Typed, source-tracked configuration values.
//   - Events: Real-time pub/sub notifications on config changes.
//   - Hooks: Lifecycle callbacks (before/after reload, set, delete, etc.).
//   - Plugins: Extensible system for registering custom loaders and providers.
//   - Binders: Type-safe struct binding with tag-based field mapping.
package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gvalidator "github.com/go-playground/validator/v10"

	"github.com/os-gomod/config/binder"
	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/decoder"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/hooks"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/observability"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/provider"
	"github.com/os-gomod/config/schema"
	"github.com/os-gomod/config/validator"
	"github.com/os-gomod/config/watcher"
)

// ReloadWarning is returned by [New] when the initial reload succeeds but one
// or more layers produced errors (non-strict mode). It implements [error] so
// callers can check with errors.Is, and the underlying layer errors are
// available via [ReloadWarning.LayerErrors].
type ReloadWarning struct {
	LayerErrors []core.LayerError
}

// Error returns a human-readable summary of the reload warnings, including
// the count of affected layers.
func (w *ReloadWarning) Error() string {
	return fmt.Sprintf("config: reload completed with %d layer warning(s)", len(w.LayerErrors))
}

// Unwrap returns the first layer error, enabling errors.Is/errors.As chains.
func (w *ReloadWarning) Unwrap() error {
	if len(w.LayerErrors) > 0 {
		return w.LayerErrors[0].Err
	}
	return nil
}

// defaultReloadErrHandler logs reload errors at warn level. It is the default
// handler used when none is provided via [WithReloadErrorHandler].
func defaultReloadErrHandler(err error) {
	slog.Warn("config: background reload failed", "err", err)
}

// Config is the top-level configuration manager. It embeds a [core.Engine]
// and extends it with an event bus, struct binder, validator, hook manager,
// watcher manager, plugin registry, and observability recorder.
//
// Call [New] or [MustNew] to create an instance, and always call [Config.Close]
// when done to release resources and stop background watchers.
type Config struct {
	*core.Engine
	bus             *event.Bus
	binder          *binder.StructBinder
	validator       validator.Validator
	hookMgr         *hooks.Manager
	watchMgr        *watcher.Manager
	pluginReg       *plugin.Registry
	ctx             context.Context
	recorder        observability.Recorder
	strictReload    bool
	defaultDebounce time.Duration
	onReloadErr     func(error)
}

// New creates a new [Config] instance by applying the given functional options,
// performing an initial reload of all layers, and wiring up hooks, plugins,
// and the struct binder.
//
// If the initial reload fails entirely, an error is returned. If one or more
// layers fail but at least one succeeds, behavior depends on [WithStrictReload]:
//   - Strict mode: returns an error listing the failed layers.
//   - Non-strict (default): returns a [*ReloadWarning] alongside the Config.
//
// The returned [Config] must be closed with [Config.Close] when no longer needed.
func New(ctx context.Context, opts ...Option) (*Config, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	bus := event.NewBus()
	hookMgr := hooks.NewManager()
	hookMgr.SetRecorder(o.recorder)
	engineOpts := make([]core.Option, 0, 1+len(o.layers))
	engineOpts = append(engineOpts, core.WithMaxWorkers(o.maxWorkers))
	for _, l := range o.layers {
		engineOpts = append(engineOpts, core.WithLayer(l))
	}
	eng := core.New(engineOpts...)
	watchMgr := watcher.NewManager()
	c := &Config{
		Engine:          eng,
		bus:             bus,
		validator:       o.val,
		hookMgr:         hookMgr,
		watchMgr:        watchMgr,
		ctx:             ctx,
		recorder:        o.recorder,
		strictReload:    o.strictReload,
		defaultDebounce: o.defaultDebounce,
		onReloadErr:     o.onReloadErr,
	}
	if len(o.plugins) > 0 {
		c.pluginReg = plugin.NewRegistry()
		host := &pluginHost{c: c}
		for _, p := range o.plugins {
			if err := c.pluginReg.Register(p, host); err != nil {
				return nil, fmt.Errorf("config: plugin %q: %w", p.Name(), err)
			}
		}
	}
	binderOpts := []binder.Option{}
	if c.validator != nil {
		binderOpts = append(binderOpts, binder.WithValidator(c.validator))
	}
	c.binder = binder.New(binderOpts...)
	result, err := c.Reload(ctx)
	if err != nil {
		return nil, fmt.Errorf("config: initial reload: %w", err)
	}
	if result.HasErrors() {
		if c.strictReload {
			return nil, fmt.Errorf(
				"config: strict reload: %d layer(s) failed",
				len(result.LayerErrs),
			)
		}
		return c, &ReloadWarning{LayerErrors: result.LayerErrs}
	}
	return c, nil
}

// MustNew is like [New] but panics on error. It is intended for use in
// package-level initialisation or situations where configuration is expected
// to always succeed.
func MustNew(ctx context.Context, opts ...Option) *Config {
	c, err := New(ctx, opts...)
	if err != nil {
		panic(err)
	}
	return c
}

// Reload re-reads all enabled layers, merges their data by priority, computes
// a diff against the current state, and publishes change events. It also
// executes before/after reload hooks and records observability metrics.
//
// Returns a [core.ReloadResult] containing the generated events and any
// per-layer errors.
func (c *Config) Reload(ctx context.Context) (core.ReloadResult, error) {
	start := time.Now()
	if c.hookMgr.Has(event.HookBeforeReload) {
		hctx := &hooks.Context{
			Operation: "reload",
			StartTime: start,
		}
		if err := c.hookMgr.Execute(ctx, event.HookBeforeReload, hctx); err != nil {
			return core.ReloadResult{}, fmt.Errorf("config: before-reload hook: %w", err)
		}
	}
	result, err := c.Engine.Reload(ctx)
	if err != nil {
		c.recorder.RecordReload(ctx, time.Since(start), 0, err)
		return result, err
	}
	for i := range result.Events {
		c.bus.Publish(ctx, &result.Events[i])
	}
	c.recorder.RecordReload(ctx, time.Since(start), len(result.Events), nil)
	if c.hookMgr.Has(event.HookAfterReload) {
		hctx := &hooks.Context{
			Operation: "reload",
			StartTime: start,
		}
		if hookErr := c.hookMgr.Execute(ctx, event.HookAfterReload, hctx); hookErr != nil {
			return result, fmt.Errorf("config: after-reload hook: %w", hookErr)
		}
	}
	return result, nil
}

// Set writes a single key-value pair into the in-memory state. If the key
// already exists with an equal value, no event is emitted. Otherwise a
// Create or Update event is published. Before/after set hooks are executed
// and observability metrics are recorded.
func (c *Config) Set(ctx context.Context, key string, raw any) error {
	start := time.Now()
	if c.hookMgr.Has(event.HookBeforeSet) {
		hctx := &hooks.Context{Operation: "set", Key: key, Value: raw, StartTime: start}
		if err := c.hookMgr.Execute(ctx, event.HookBeforeSet, hctx); err != nil {
			return fmt.Errorf("config: before-set hook: %w", err)
		}
	}
	evt, err := c.Engine.Set(ctx, key, raw)
	if err != nil {
		c.recorder.RecordSet(ctx, key, time.Since(start), err)
		return err
	}
	c.bus.Publish(ctx, &evt)
	c.recorder.RecordSet(ctx, key, time.Since(start), nil)
	if c.hookMgr.Has(event.HookAfterSet) {
		hctx := &hooks.Context{Operation: "set", Key: key, Value: raw, StartTime: start}
		if hookErr := c.hookMgr.Execute(ctx, event.HookAfterSet, hctx); hookErr != nil {
			return fmt.Errorf("config: after-set hook: %w", hookErr)
		}
	}
	return nil
}

// BatchSet atomically writes multiple key-value pairs into the in-memory
// state. A single lock acquisition is used, and individual Create/Update
// events are published for each changed key.
func (c *Config) BatchSet(ctx context.Context, kv map[string]any) error {
	start := time.Now()
	events, err := c.Engine.BatchSet(ctx, kv)
	if err != nil {
		c.recorder.RecordBatchSet(ctx, time.Since(start), err)
		return err
	}
	for i := range events {
		c.bus.Publish(ctx, &events[i])
	}
	c.recorder.RecordBatchSet(ctx, time.Since(start), nil)
	return nil
}

// Delete removes a single key from the in-memory state. If the key does not
// exist, no event is emitted and no error is returned. Otherwise a Delete
// event is published. Before/after delete hooks are executed.
func (c *Config) Delete(ctx context.Context, key string) error {
	start := time.Now()
	if c.hookMgr.Has(event.HookBeforeDelete) {
		hctx := &hooks.Context{Operation: "delete", Key: key, StartTime: start}
		if err := c.hookMgr.Execute(ctx, event.HookBeforeDelete, hctx); err != nil {
			return fmt.Errorf("config: before-delete hook: %w", err)
		}
	}
	evt, err := c.Engine.Delete(ctx, key)
	if err != nil {
		c.recorder.RecordDelete(ctx, key, time.Since(start), err)
		return err
	}
	c.bus.Publish(ctx, &evt)
	c.recorder.RecordDelete(ctx, key, time.Since(start), nil)
	if c.hookMgr.Has(event.HookAfterDelete) {
		hctx := &hooks.Context{Operation: "delete", Key: key, StartTime: start}
		if hookErr := c.hookMgr.Execute(ctx, event.HookAfterDelete, hctx); hookErr != nil {
			return fmt.Errorf("config: after-delete hook: %w", hookErr)
		}
	}
	return nil
}

// Bind maps the current configuration state onto a Go struct (target).
// The binder uses struct tags and reflection to populate fields.
// An optional validator (configured via [WithValidator]) is applied
// automatically if present.
func (c *Config) Bind(ctx context.Context, target any) error {
	start := time.Now()
	data := c.GetAll()
	err := c.binder.Bind(ctx, data, target)
	c.recorder.RecordBind(ctx, time.Since(start), err)
	return err
}

// Snapshot returns a defensive copy of the entire configuration state as a
// map of string keys to [value.Value] entries.
func (c *Config) Snapshot() map[string]value.Value {
	return c.GetAll()
}

// Restore replaces the current configuration state with the provided data.
// This is useful for rolling back to a previously captured [Snapshot].
func (c *Config) Restore(data map[string]value.Value) {
	c.SetState(data)
}

// OnChange registers an event observer that is notified whenever a change
// event matches the given glob pattern. Returns an unsubscribe function that
// removes the observer when called.
//
// An empty pattern matches all events.
func (c *Config) OnChange(pattern string, obs event.Observer) func() {
	return c.bus.Subscribe(pattern, obs)
}

// Subscribe registers an event observer that is notified for all change
// events (equivalent to OnChange with an empty pattern). Returns an
// unsubscribe function.
func (c *Config) Subscribe(obs event.Observer) func() {
	return c.bus.Subscribe("", obs)
}

// Schema generates a configuration schema from the given struct. The schema
// describes the expected keys, types, and default values for validation and
// documentation purposes.
func (c *Config) Schema(v any) (*schema.Schema, error) {
	gen := schema.New()
	return gen.Generate(v)
}

// Validate runs the configured validator against the given target struct.
// If no custom validator was provided at construction time, the default
// validator is used.
func (c *Config) Validate(ctx context.Context, target any) error {
	start := time.Now()
	val := c.validator
	if val == nil {
		val = validator.New()
	}
	err := val.Validate(ctx, target)
	c.recorder.RecordValidation(ctx, time.Since(start), err)
	return err
}

// Plugins returns the names of all registered plugins, or nil if no plugins
// were configured.
func (c *Config) Plugins() []string {
	if c.pluginReg == nil {
		return nil
	}
	return c.pluginReg.Plugins()
}

// Explain returns a human-readable description of the value stored at the
// given key, including its raw value, source origin, and priority level.
// Returns an empty string if the key does not exist.
func (c *Config) Explain(key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		"key %q: value=%v, source=%s, priority=%d",
		key,
		v.Raw(),
		v.Source(),
		v.Priority(),
	)
}

// Close stops all watchers, clears the event bus, closes all layers, and
// runs before/after close lifecycle hooks. It should be called when the
// [Config] instance is no longer needed, typically via defer.
func (c *Config) Close(ctx context.Context) error {
	if c.hookMgr.Has(event.HookBeforeClose) {
		hctx := &hooks.Context{Operation: "close"}
		_ = c.hookMgr.Execute(ctx, event.HookBeforeClose, hctx)
	}
	c.watchMgr.StopAll()
	c.bus.Clear()
	err := c.Engine.Close(ctx)
	if c.hookMgr.Has(event.HookAfterClose) {
		hctx := &hooks.Context{Operation: "close"}
		_ = c.hookMgr.Execute(ctx, event.HookAfterClose, hctx)
	}
	return err
}

// startWatch launches a background goroutine that listens for change
// notifications from the given source and triggers debounced reloads.
// It stops automatically when the provided context is cancelled.
func (c *Config) startWatch(ctx context.Context, src interface {
	Watch(context.Context) (<-chan event.Event, error)
},
) {
	ch, err := src.Watch(ctx)
	if err != nil || ch == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				c.watchMgr.TriggerReload(c.defaultDebounce, func() {
					if _, reloadErr := c.Reload(ctx); reloadErr != nil {
						c.onReloadErr(reloadErr)
					}
				})
			}
		}
	}()
}

// pluginHost is an internal adapter that implements [plugin.Host] so that
// plugins can register loaders, providers, decoders, and validators.
type pluginHost struct {
	c *Config
}

// RegisterLoader registers a named loader factory via the global loader registry.
func (h *pluginHost) RegisterLoader(name string, f loader.Factory) error {
	return loader.DefaultRegistry.Register(name, f)
}

// RegisterProvider registers a named provider factory via the global provider registry.
func (h *pluginHost) RegisterProvider(name string, f provider.Factory) error {
	return provider.DefaultRegistry.Register(name, f)
}

// RegisterDecoder registers a decoder via the global decoder registry.
func (h *pluginHost) RegisterDecoder(d decoder.Decoder) error {
	return decoder.DefaultRegistry.Register(d)
}

// RegisterValidator registers a custom validation function under the given
// tag on the underlying validator engine. If the configured validator does
// not support dynamic plugin tags, an error is returned.
func (h *pluginHost) RegisterValidator(tag string, fn gvalidator.Func) error {
	if h.c.validator == nil {
		h.c.validator = validator.New()
	}
	engine, ok := h.c.validator.(*validator.Engine)
	if !ok {
		return fmt.Errorf("config: configured validator does not support dynamic plugin tags")
	}
	return engine.RegisterValidation(tag, fn)
}

// Subscribe registers an event observer on the plugin host's event bus,
// allowing plugins to listen for configuration change events.
func (h *pluginHost) Subscribe(obs event.Observer) func() {
	return h.c.bus.Subscribe("", obs)
}
