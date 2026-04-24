// Package config provides a layered, reactive configuration management library for Go.
//
// It supports loading configuration from multiple sources (files, environment variables,
// remote providers like Consul, etcd, NATS, Vault), merging them by priority, and
// automatically propagating changes to subscribers via an event bus.
//
// Key features:
//   - Layered configuration with priority-based merging
//   - Hot-reload with file watching and remote provider polling
//   - Struct binding with validation (go-playground/validator)
//   - Secret redaction in snapshots, events, and logs
//   - Plugin system for extending loaders, decoders, and validators
//   - OpenTelemetry integration for distributed tracing
//   - Circuit breakers per layer for resilience against failing sources
//   - JSON Schema generation from Go structs
//   - Feature flags with boolean, percentage, and variant evaluation
//   - Profile-based configuration swapping (GitOps workflows)
//
// Example (basic usage):
//
//	cfg, err := config.New(ctx,
//	    config.WithLoader(loader.NewFileLoader("config.yaml")),
//	    config.WithLoader(loader.NewEnvLoader()),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	var appConfig AppConfig
//	if err := cfg.Bind(ctx, &appConfig); err != nil {
//	    log.Fatal(err)
//	}
package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"

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
	"github.com/os-gomod/config/profile"
	"github.com/os-gomod/config/provider"
	"github.com/os-gomod/config/schema"
	"github.com/os-gomod/config/validator"
	"github.com/os-gomod/config/watcher"
)

// ReloadWarning is returned by [New] when the initial reload completes
// successfully but one or more layers produced errors. It satisfies the
// error interface so callers can check with errors.Is/as, while still
// being able to use the returned *Config.
type ReloadWarning struct {
	LayerErrors []core.LayerError
}

func (w *ReloadWarning) Error() string {
	return fmt.Sprintf("config: reload completed with %d layer warning(s)", len(w.LayerErrors))
}

func (w *ReloadWarning) Unwrap() error {
	if len(w.LayerErrors) > 0 {
		return w.LayerErrors[0].Err
	}
	return nil
}

// defaultReloadErrHandler logs reload errors that occur during
// background file watching. This is the default handler used when
// no custom handler is provided via [WithReloadErrorHandler].
func defaultReloadErrHandler(err error) {
	slog.Warn("config: background reload failed", "err", err)
}

// Config is the top-level configuration manager. It wraps a [core.Engine]
// with a namespace-aware API, an event bus, struct binding, validation,
// lifecycle hooks, plugin support, and audit logging.
//
// Config is safe for concurrent use. All reads are lock-free via atomic
// pointer to the underlying [value.State]; all writes are serialized.
//
// Most applications should create a Config via [New] (functional options)
// or the [Builder] (fluent API) and use [Bind] to populate a struct.
type Config struct {
	*core.Engine
	namespace       string
	bus             *event.Bus
	binder          *binder.StructBinder
	validator       validator.Validator
	hookMgr         *hooks.Manager
	watchMgr        *watcher.Manager
	pluginReg       *plugin.Registry
	ctx             context.Context
	recorder        observability.Recorder
	auditRecorder   *event.AuditRecorder
	tracer          trace.Tracer
	strictReload    bool
	defaultDebounce time.Duration
	onReloadErr     func(error)
	schemaTarget    any
}

// New creates a new Config instance, applies the given options, and
// performs an initial reload of all layers.
//
// If the initial reload has layer errors and [WithStrictReload] is set,
// New returns an error. Otherwise it returns the Config along with a
// [ReloadWarning] (which still satisfies error) describing the layer failures.
//
// Returns an error if the reload itself fails catastrophically.
func New(ctx context.Context, opts ...Option) (*Config, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	bus := event.NewBus()
	hookMgr := hooks.NewManager()
	hookMgr.SetRecorder(o.recorder)
	engineOpts := make([]core.Option, 0, 2+len(o.layers))
	engineOpts = append(engineOpts, core.WithMaxWorkers(o.maxWorkers))
	if o.deltaReload {
		engineOpts = append(engineOpts, core.WithDeltaReload())
	}
	if o.batchedReload {
		engineOpts = append(engineOpts, core.WithBatchedReload(true))
	}
	if o.cacheTTL > 0 {
		engineOpts = append(engineOpts, core.WithCacheTTL(o.cacheTTL))
	}
	for _, l := range o.layers {
		engineOpts = append(engineOpts, core.WithLayer(l))
	}
	eng := core.New(engineOpts...)
	watchMgr := watcher.NewManager()
	auditRec := event.NewAuditRecorder(o.recorder)
	c := &Config{
		Engine:          eng,
		namespace:       o.namespace,
		bus:             bus,
		validator:       o.val,
		hookMgr:         hookMgr,
		watchMgr:        watchMgr,
		ctx:             ctx,
		recorder:        o.recorder,
		auditRecorder:   auditRec,
		tracer:          o.tracer,
		strictReload:    o.strictReload,
		defaultDebounce: o.defaultDebounce,
		onReloadErr:     o.onReloadErr,
		schemaTarget:    o.schemaTarget,
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

// MustNew is like [New] but panics on error. Use this in package-level
// init functions or tests where failure is unrecoverable.
func MustNew(ctx context.Context, opts ...Option) *Config {
	c, err := New(ctx, opts...)
	if err != nil {
		panic(err)
	}
	return c
}

// Reload reloads all enabled layers, merges their data by priority, computes
// a diff against the previous state, and publishes change events to the bus.
//
// It runs before/after-reload hooks and emits an audit event for compliance.
// If [WithSchemaValidation] was configured and the reload succeeds, it also
// validates the merged config against the schema target.
func (c *Config) Reload(ctx context.Context) (core.ReloadResult, error) {
	ctx, span := c.startSpan(ctx, "config.reload")
	defer span.End()
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
	// Emit audit event for reload
	{
		auditEntry := event.NewAuditEntry("reload", "", "system", traceIDFromContext(ctx))
		auditEvt := event.NewAuditEvent(auditEntry)
		c.bus.Publish(ctx, &auditEvt)
		c.auditRecorder.RecordAudit(ctx, auditEntry)
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
	// Schema validation on reload
	if c.schemaTarget != nil && !result.HasErrors() {
		if bindErr := c.Bind(ctx, c.schemaTarget); bindErr != nil {
			if c.strictReload {
				return result, fmt.Errorf("config: schema validation on reload: %w", bindErr)
			}
			slog.Warn("config: schema validation failed on reload (non-strict)", "err", bindErr)
		}
	}
	return result, nil
}

// Set sets a single configuration key (with namespace prefix if configured).
// It emits a create or update event and an audit event.
// Before/after-set hooks are executed if registered.
func (c *Config) Set(ctx context.Context, key string, raw any) error {
	ctx, span := c.startSpan(ctx, "config.set", "key", key)
	defer span.End()
	start := time.Now()
	if c.hookMgr.Has(event.HookBeforeSet) {
		hctx := &hooks.Context{Operation: "set", Key: key, Value: raw, StartTime: start}
		if err := c.hookMgr.Execute(ctx, event.HookBeforeSet, hctx); err != nil {
			return fmt.Errorf("config: before-set hook: %w", err)
		}
	}
	evt, err := c.Engine.Set(ctx, c.resolveKey(key), raw)
	if err != nil {
		c.recorder.RecordSet(ctx, key, time.Since(start), err)
		return err
	}
	c.bus.Publish(ctx, &evt)
	c.recorder.RecordSet(ctx, key, time.Since(start), nil)
	// Emit audit event for set
	{
		auditEntry := event.NewAuditEntry("set", key, "", traceIDFromContext(ctx), event.WithLabel("source", "user"))
		auditEntry.OldValue = evt.OldValue
		auditEntry.NewValue = evt.NewValue
		auditEvt := event.NewAuditEvent(auditEntry)
		c.bus.Publish(ctx, &auditEvt)
		c.auditRecorder.RecordAudit(ctx, auditEntry)
	}
	if c.hookMgr.Has(event.HookAfterSet) {
		hctx := &hooks.Context{Operation: "set", Key: key, Value: raw, StartTime: start}
		if hookErr := c.hookMgr.Execute(ctx, event.HookAfterSet, hctx); hookErr != nil {
			return fmt.Errorf("config: after-set hook: %w", hookErr)
		}
	}
	return nil
}

// BatchSet sets multiple configuration keys atomically within a single
// lock acquisition. Events are published for each changed key.
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

// Delete removes a configuration key (with namespace prefix). It emits
// a delete event and an audit event. Before/after-delete hooks are executed.
func (c *Config) Delete(ctx context.Context, key string) error {
	start := time.Now()
	if c.hookMgr.Has(event.HookBeforeDelete) {
		hctx := &hooks.Context{Operation: "delete", Key: key, StartTime: start}
		if err := c.hookMgr.Execute(ctx, event.HookBeforeDelete, hctx); err != nil {
			return fmt.Errorf("config: before-delete hook: %w", err)
		}
	}
	evt, err := c.Engine.Delete(ctx, c.resolveKey(key))
	if err != nil {
		c.recorder.RecordDelete(ctx, key, time.Since(start), err)
		return err
	}
	c.bus.Publish(ctx, &evt)
	c.recorder.RecordDelete(ctx, key, time.Since(start), nil)
	// Emit audit event for delete
	{
		auditEntry := event.NewAuditEntry("delete", key, "", traceIDFromContext(ctx), event.WithLabel("source", "user"))
		auditEntry.OldValue = evt.OldValue
		auditEntry.NewValue = evt.NewValue
		auditEvt := event.NewAuditEvent(auditEntry)
		c.bus.Publish(ctx, &auditEvt)
		c.auditRecorder.RecordAudit(ctx, auditEntry)
	}
	if c.hookMgr.Has(event.HookAfterDelete) {
		hctx := &hooks.Context{Operation: "delete", Key: key, StartTime: start}
		if hookErr := c.hookMgr.Execute(ctx, event.HookAfterDelete, hctx); hookErr != nil {
			return fmt.Errorf("config: after-delete hook: %w", hookErr)
		}
	}
	return nil
}

// Bind unmarshals the current configuration state into the target struct
// using the "config" struct tag for field mapping. If a validator is
// configured, the target is validated after binding.
func (c *Config) Bind(ctx context.Context, target any) error {
	ctx, span := c.startSpan(ctx, "config.bind")
	defer span.End()
	start := time.Now()
	data := c.GetAll()
	err := c.binder.Bind(ctx, data, target)
	c.recorder.RecordBind(ctx, time.Since(start), err)
	// Emit audit event for bind
	{
		auditEntry := event.NewAuditEntry("bind", "", "", traceIDFromContext(ctx))
		auditEvt := event.NewAuditEvent(auditEntry)
		c.bus.Publish(ctx, &auditEvt)
		c.auditRecorder.RecordAudit(ctx, auditEntry)
	}
	return err
}

// Namespace returns the current namespace prefix. Returns empty string if no namespace is set.
func (c *Config) Namespace() string {
	return c.namespace
}

// resolveKey prefixes the key with the namespace if one is set.
func (c *Config) resolveKey(key string) string {
	if c.namespace == "" {
		return key
	}
	return c.namespace + key
}

// SetNamespace changes the active namespace and triggers a reload
// to populate the config with the namespaced keys.
// This is useful for runtime tenant switching.
func (c *Config) SetNamespace(ctx context.Context, ns string) error {
	if c.namespace == ns {
		return nil
	}
	c.namespace = ns
	_, err := c.Reload(ctx)
	return err
}

// Get resolves the key with the namespace prefix and looks it up.
// This shadows the embedded Engine.Get with a namespace-aware version.
func (c *Config) Get(key string) (value.Value, bool) {
	return c.Engine.Get(c.resolveKey(key))
}

// Has checks if a key exists (with namespace prefix).
func (c *Config) Has(key string) bool {
	return c.Engine.Has(c.resolveKey(key))
}

// Snapshot returns a redacted copy of the current configuration state.
// All secret values are replaced with [REDACTED]. Use Get() for
// unredacted access when needed by the application.
func (c *Config) Snapshot() map[string]value.Value {
	state := c.State()
	if state == nil {
		return make(map[string]value.Value)
	}
	return state.RedactedCopy().GetAll()
}

// Restore replaces the current configuration state with the provided data.
// It is typically used to restore from a previously saved snapshot.
func (c *Config) Restore(data map[string]value.Value) {
	c.SetState(data)
}

// OnChange subscribes to config change events matching the given pattern.
// It is an alias for [WatchPattern].
func (c *Config) OnChange(pattern string, obs event.Observer) func() {
	return c.bus.Subscribe(pattern, obs)
}

// WatchPattern subscribes to config changes matching the given glob pattern.
// Returns an unsubscribe function. The observer is called asynchronously
// for every event whose key matches the pattern.
//
// Pattern syntax supports * (matches any sequence) and ? (matches single char).
// Empty pattern or "*" matches all keys.
func (c *Config) WatchPattern(pattern string, obs event.Observer) func() {
	return c.bus.Subscribe(pattern, obs)
}

// LoadProfile applies the given profile's layers to the engine and triggers
// a reload. This enables GitOps workflows where profiles are swapped at
// runtime (e.g., switching from "staging" to "production" configuration).
func (c *Config) LoadProfile(ctx context.Context, p *profile.Profile) (core.ReloadResult, error) {
	if err := p.Apply(c.Engine); err != nil {
		return core.ReloadResult{}, fmt.Errorf("config: load profile: %w", err)
	}
	return c.Reload(ctx)
}

// Subscribe subscribes to all configuration events (no pattern filter).
// Returns an unsubscribe function.
func (c *Config) Subscribe(obs event.Observer) func() {
	return c.bus.Subscribe("", obs)
}

// Schema generates a JSON Schema from the given struct type.
// It uses [schema.Generator] internally.
func (c *Config) Schema(v any) (*schema.Schema, error) {
	gen := schema.New()
	return gen.Generate(v)
}

// Validate runs the configured validator against the target struct.
// If no custom validator was provided via [WithValidator], a default
// validator with built-in tags is used.
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

// Plugins returns the names of all registered plugins, or nil if
// no plugins are configured.
func (c *Config) Plugins() []string {
	if c.pluginReg == nil {
		return nil
	}
	return c.pluginReg.Plugins()
}

// Explain returns a human-readable description of the configuration key,
// including its value (redacted if it is a secret), source, and priority.
func (c *Config) Explain(key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	displayVal := v.Raw()
	if value.IsSecret(key) {
		displayVal = "[REDACTED]"
		c.recorder.RecordSecretRedacted(ctxForRecord(), "explain")
	}
	return fmt.Sprintf(
		"key %q: value=%v, source=%s, priority=%d",
		key,
		displayVal,
		v.Source(),
		v.Priority(),
	)
}

// startSpan creates an OpenTelemetry span if a tracer is configured.
// The span is started with the given name and optional key-value attributes.
// If no tracer is configured, the returned span is a no-op and the context
// is returned unchanged.
func (c *Config) startSpan(ctx context.Context, name string, attrs ...string) (context.Context, trace.Span) {
	if c.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	var spanOpts []trace.SpanStartOption
	if len(attrs) > 0 {
		// Build key-value attribute pairs for the span
		// Note: OTel attributes use attribute.Key(string) and attribute.Value
		// In a production setup, you'd use go.opentelemetry.io/otel/attribute.String()
		// to create properly typed attributes. For simplicity, we pass the
		// span name and let consumers add attributes via the returned Span.
		_ = attrs // attributes available for future typed attribute support
	}
	ctx, span := c.tracer.Start(ctx, name, spanOpts...)
	return ctx, span
}

// ctxForRecord returns a context for metric recording when no user context
// is available (e.g., in Explain which doesn't take a context).
func ctxForRecord() context.Context {
	return context.Background()
}

// traceIDFromContext extracts a distributed trace ID from the context.
// If no OTel span is found, it generates a new trace ID.
func traceIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	return observability.GenerateTraceID()
}

// Close shuts down the Config: stops all watchers, clears the event bus,
// runs before/after-close hooks, and closes the engine.
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

type pluginHost struct {
	c *Config
}

func (h *pluginHost) RegisterLoader(name string, f loader.Factory) error {
	return loader.DefaultRegistry.Register(name, f)
}

func (h *pluginHost) RegisterProvider(name string, f provider.Factory) error {
	return provider.DefaultRegistry.Register(name, f)
}

func (h *pluginHost) RegisterDecoder(d decoder.Decoder) error {
	return decoder.DefaultRegistry.Register(d)
}

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

func (h *pluginHost) Subscribe(obs event.Observer) func() {
	return h.c.bus.Subscribe("", obs)
}
