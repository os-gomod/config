// Package config provides the top-level Config facade that wires together
// the core engine, loaders, providers, binder, validator, watcher, event bus,
// hooks, observability, schema generation, and secret handling.
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

// ReloadWarning wraps one or more layer errors that occurred during a non-strict
// reload. When strict mode is off, these errors are not fatal and are instead
// attached to the warning so callers can inspect them.
type ReloadWarning struct {
	// LayerErrors contains the per-layer errors from the reload.
	LayerErrors []core.LayerError
}

// Error formats the ReloadWarning as a human-readable string listing each layer error.
func (w *ReloadWarning) Error() string {
	return fmt.Sprintf("config: reload completed with %d layer warning(s)", len(w.LayerErrors))
}

// Unwrap returns the first layer error for errors.Is traversal, or nil.
func (w *ReloadWarning) Unwrap() error {
	if len(w.LayerErrors) > 0 {
		return w.LayerErrors[0].Err
	}
	return nil
}

// defaultReloadErrHandler logs the error at WARN level using slog.
func defaultReloadErrHandler(err error) {
	slog.Warn("config: background reload failed", "err", err)
}

// Config is the top-level config handle.
//
// It embeds core.Engine and wires together the binder, validator, watcher,
// event bus, hooks, and observability recorder.
//
// Lock ordering: Config holds no locks of its own. All locking is delegated to
// core.Engine (mu) for state and to event.Bus (mu) for subscriptions.
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
	onReloadErr     func(error) // never nil; see defaultReloadErrHandler
}

// New creates a Config by applying the given options and performing an initial Reload.
// If no WithStrictReload option was set, layer errors during the initial reload
// are wrapped in a *ReloadWarning and returned; they do not prevent startup.
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

	// Perform initial reload.
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

// MustNew is like New but panics on error.
func MustNew(ctx context.Context, opts ...Option) *Config {
	c, err := New(ctx, opts...)
	if err != nil {
		panic(err)
	}
	return c
}

// Reload re-fetches all layers, merges by priority, updates the engine state,
// and publishes change events through the event bus.
func (c *Config) Reload(ctx context.Context) (core.ReloadResult, error) {
	start := time.Now()

	// Execute before-reload hooks.
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

	// Publish events.
	for _, evt := range result.Events {
		c.bus.Publish(ctx, evt)
	}

	c.recorder.RecordReload(ctx, time.Since(start), len(result.Events), nil)

	// Execute after-reload hooks.
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

// Set sets a single key to the given raw value and publishes a change event.
func (c *Config) Set(ctx context.Context, key string, raw any) error {
	start := time.Now()

	// Execute before-set hooks.
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
	c.bus.Publish(ctx, evt)
	c.recorder.RecordSet(ctx, key, time.Since(start), nil)

	// Execute after-set hooks.
	if c.hookMgr.Has(event.HookAfterSet) {
		hctx := &hooks.Context{Operation: "set", Key: key, Value: raw, StartTime: start}
		if hookErr := c.hookMgr.Execute(ctx, event.HookAfterSet, hctx); hookErr != nil {
			return fmt.Errorf("config: after-set hook: %w", hookErr)
		}
	}

	return nil
}

// BatchSet sets multiple keys atomically and publishes change events.
func (c *Config) BatchSet(ctx context.Context, kv map[string]any) error {
	start := time.Now()
	events, err := c.Engine.BatchSet(ctx, kv)
	if err != nil {
		c.recorder.RecordBatchSet(ctx, time.Since(start), err)
		return err
	}
	for _, evt := range events {
		c.bus.Publish(ctx, evt)
	}
	c.recorder.RecordBatchSet(ctx, time.Since(start), nil)
	return nil
}

// Delete removes a key from the config state and publishes a delete event.
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
	c.bus.Publish(ctx, evt)
	c.recorder.RecordDelete(ctx, key, time.Since(start), nil)

	if c.hookMgr.Has(event.HookAfterDelete) {
		hctx := &hooks.Context{Operation: "delete", Key: key, StartTime: start}
		if hookErr := c.hookMgr.Execute(ctx, event.HookAfterDelete, hctx); hookErr != nil {
			return fmt.Errorf("config: after-delete hook: %w", hookErr)
		}
	}

	return nil
}

// Bind populates the target struct from the current config state using the
// configured binder. If a validator was set, it is called after binding.
func (c *Config) Bind(ctx context.Context, target any) error {
	start := time.Now()
	data := c.GetAll()
	err := c.binder.Bind(ctx, data, target)
	c.recorder.RecordBind(ctx, time.Since(start), err)
	return err
}

// Snapshot returns the current state as a map of values.
func (c *Config) Snapshot() map[string]value.Value {
	return c.GetAll()
}

// Restore replaces the engine state with the given data map.
func (c *Config) Restore(data map[string]value.Value) {
	c.SetState(data)
}

// OnChange registers an observer for events matching the given key pattern.
// The returned function unsubscribes the observer when called.
func (c *Config) OnChange(pattern string, obs event.Observer) func() {
	return c.bus.Subscribe(pattern, obs)
}

// Subscribe registers a catch-all event observer.
// The returned function unsubscribes the observer when called.
func (c *Config) Subscribe(obs event.Observer) func() {
	return c.bus.Subscribe("", obs)
}

// Schema generates the JSON Schema for the given struct type.
// v must be a struct or pointer to a struct.
func (c *Config) Schema(v any) (*schema.Schema, error) {
	gen := schema.New()
	return gen.Generate(v)
}

// Validate explicitly re-runs validation on target.
// It uses the configured validator when present, otherwise it falls back to
// the default validator engine.
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

// Plugins returns the names of plugins registered on this Config.
func (c *Config) Plugins() []string {
	if c.pluginReg == nil {
		return nil
	}
	return c.pluginReg.Plugins()
}

// Explain returns a human-readable description of how the value for key was
// determined: which layer contributed it and what its merge priority was.
// Returns an empty string if key is not found.
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

// Close shuts down all subsystems: hooks, watcher, event bus, and the engine.
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

// startWatch begins watching the given source for changes, triggering a
// debounced reload when changes are detected.
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

// pluginHost implements plugin.Host, giving plugins access to framework registries.
type pluginHost struct {
	c *Config
}

// RegisterLoader adds a Loader factory under name.
func (h *pluginHost) RegisterLoader(name string, f loader.Factory) error {
	return loader.DefaultRegistry.Register(name, f)
}

// RegisterProvider adds a Provider factory under name.
func (h *pluginHost) RegisterProvider(name string, f provider.Factory) error {
	return provider.DefaultRegistry.Register(name, f)
}

// RegisterDecoder adds a Decoder to the decoder registry.
func (h *pluginHost) RegisterDecoder(d decoder.Decoder) error {
	return decoder.DefaultRegistry.Register(d)
}

// RegisterValidator adds a named validation tag function.
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

// Subscribe registers a catch-all event observer.
func (h *pluginHost) Subscribe(obs event.Observer) func() {
	return h.c.bus.Subscribe("", obs)
}
