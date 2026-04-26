# os-gomod/config v2 — Comprehensive Feature Example

> Production-grade Go configuration management with Clean Architecture, Command Pipeline, Dependency Injection, Typed Interceptors, and AppError contracts.

**Module:** `github.com/os-gomod/config/v2`

---

## Table of Contents

- [Overview](#overview)
- [v2 Architectural Changes](#v2-architectural-changes-from-v1)
- [Architecture](#architecture)
- [Configuration Files](#configuration-files)
- [Quick Start](#quick-start)
- [All 28 Features](#all-28-features)
  1. [Multi-Format File Loading](#1-multi-format-file-loading)
  2. [Functional Options](#2-functional-options)
  3. [Struct Binding](#3-struct-binding)
  4. [Validation](#4-validation)
  5. [Layered Loading with Priorities](#5-layered-loading-with-priorities)
  6. [Event System](#6-event-system)
  7. [Typed Interceptors](#7-typed-interceptors)
  8. [Schema Generation](#8-schema-generation)
  9. [Snapshot / Restore](#9-snapshot--restore)
  10. [Command Pipeline](#10-command-pipeline)
  11. [Explain (Key Provenance)](#11-explain-key-provenance)
  12. [Profiles](#12-profiles)
  13. [Feature Flags](#13-feature-flags)
  14. [Multi-Tenancy](#14-multi-tenancy)
  15. [Observability](#15-observability)
  16. [Secure Storage](#16-secure-storage)
  17. [Plugin System](#17-plugin-system)
  18. [Watcher / Debounce](#18-watcher--debounce)
  19. [Value Types](#19-value-types)
  20. [AppError Handling](#20-apperror-handling)
  21. [Environment Variable Loader](#21-environment-variable-loader)
  22. [Decoder Registry](#22-decoder-registry)
  23. [BatchSet](#23-batchset)
  24. [Delta Reload](#24-delta-reload)
  25. [Event Bus (Direct)](#25-event-bus-direct)
  26. [Registry Bundle](#26-registry-bundle)
  27. [Audit System](#27-audit-system)
  28. [Backoff Strategy](#28-backoff-strategy)
- [v1 to v2 Migration Guide](#v1--v2-migration-guide)
- [Feature Summary Table](#feature-summary-table)

---

## Overview

`os-gomod/config/v2` is a production-grade Go configuration management library that provides:

- **Multi-format loading** — JSON, TOML, YAML, in-memory, and environment variables
- **Priority-based layering** — Higher-priority layers override lower ones deterministically
- **Type-safe struct binding** — Map flat or nested config keys to Go structs
- **Validation** — Built-in validators wrapping `go-playground/validator` with custom tags
- **Command Pipeline** — All mutations route through a middleware chain (Tracing, Metrics, Audit, Logging, Recovery, CorrelationID)
- **Event System** — Bounded async dispatcher with pattern-based subscriptions
- **Feature Flags** — Boolean, Percentage (rollout), and Variant (A/B/n) flags
- **Multi-Tenancy** — Namespace-prefixed key isolation
- **Secure Storage** — Pluggable backends (Vault, KMS) with caching
- **Plugin Architecture** — Extend via `service.Plugin` interface with `PluginHost` injection
- **AppError Contracts** — 22 error codes with Severity, Retryable, CorrelationID, StackTrace
- **Dependency Injection** — `RegistryBundle` holds Decoder, Loader, and Provider registries

This example demonstrates all **28 features** in a single `go run main.go` invocation.

---

## v2 Architectural Changes (from v1)

| Aspect | v1 | v2 |
|--------|----|----|
| **Construction** | Builder Pattern (`config.NewBuilder().Build()`) | Functional Options (`config.New(ctx, opts...)`) |
| **Pre/Post Hooks** | Generic `BeforeFn`/`AfterFn` closures | Typed Interceptors (`SetFunc`, `DeleteFunc`, `ReloadFunc`, `BindFunc`, `CloseFunc`) |
| **Plugin Registration** | Manual registration calls | `service.Plugin` interface with `PluginHost` providing typed host methods |
| **Error Handling** | `fmt.Errorf` strings | `AppError` interface with `Code()`, `Severity()`, `Retryable()`, `CorrelationID()` |
| **Registries** | Global/package-level singletons | Dependency-injected `RegistryBundle` (Decoder, Loader, Provider) |
| **Mutations** | Direct method calls | Command Pipeline with middleware chain |
| **Events** | Simple callback dispatch | Worker-pool async dispatcher with bounded queue |
| **State** | Mutable in-place | Immutable snapshots with redacted restore |

---

## Architecture

```
                          ┌─────────────────────────────┐
                          │      config.Config           │
                          │   (Public Facade)            │
                          │  config.New(ctx, opts...)    │
                          └──────┬──────────────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
    ┌─────────▼────────┐  ┌─────▼──────┐  ┌────────▼────────┐
    │  QueryService    │  │ Mutation   │  │  RuntimeService │
    │  Get/Has/GetAll  │  │ Service    │  │  Reload/Snapshot│
    │  Explain/Search  │  │ Set/Delete │  │  Restore/Close  │
    └──────────────────┘  │ BatchSet  │  └─────────────────┘
                          └─────┬──────┘
                                │
                    ┌───────────▼───────────┐
                    │   Command Pipeline     │
                    │  ┌───────────────────┐ │
                    │  │ CorrelationID      │ │
                    │  │ Tracing            │ │
                    │  │ Metrics            │ │
                    │  │ Audit              │ │
                    │  │ Logging            │ │
                    │  │ Recovery           │ │
                    │  └───────────────────┘ │
                    └───────────┬───────────┘
                                │
              ┌─────────────────┼──────────────────┐
              │                 │                  │
    ┌─────────▼──────┐  ┌──────▼───────┐  ┌──────▼───────┐
    │  Event Bus     │  │ Registry     │  │  Plugin      │
    │  (Worker Pool) │  │ Bundle       │  │  Service     │
    │  Async Dispatch│  │ ┌──────────┐ │  │  Init/Host   │
    │  Bounded Queue │  │ │ Decoder  │ │  └──────────────┘
    │  Pattern Match │  │ │ Loader   │ │
    └────────────────┘  │ │ Provider │ │
                        │ └──────────┘ │
                        └──────────────┘
```

### Public Facade: `config.Config`

Created with `config.New(ctx, opts...)`. Provides:

| Service | Methods | Description |
|---------|---------|-------------|
| **QueryService** | `Get`, `Has`, `GetAll`, `Explain`, `Search` | Read-only access to merged config |
| **MutationService** | `Set`, `Delete`, `BatchSet` | Write operations through the Command Pipeline |
| **RuntimeService** | `Reload`, `Snapshot`, `Restore`, `Close`, `SetNamespace` | Lifecycle and state management |
| **PluginService** | `Subscribe`, `WatchPattern` | Event subscription and plugin integration |

### Bounded Services (Internal)

- **QueryService** — Immutable reads from the merged state
- **MutationService** — Routes all writes through the Command Pipeline
- **RuntimeService** — Manages reload cycles, snapshots, and graceful shutdown
- **PluginService** — Manages plugin lifecycle and event subscriptions

### Command Pipeline Middleware

All mutations (`Set`, `Delete`, `Reload`, `Bind`, `Close`) route through:

1. **CorrelationID** — Auto-generates trace IDs for request tracking
2. **Tracing** — Distributed tracing integration
3. **Metrics** — Per-operation recording (count, duration, errors)
4. **Audit** — Immutable audit log entries for compliance
5. **Logging** — Structured `slog` logging
6. **Recovery** — Catches panics and converts to `AppError`

### Event Bus

Worker-pool async dispatcher with:
- Configurable worker count and bounded queue size
- Pattern-based subscriptions (`WatchPattern("server.*", handler)`)
- 7 typed events: `Create`, `Update`, `Delete`, `Reload`, `Error`, `Watch`, `Audit`
- Automatic unsubscribe via returned cancellation functions

### Registry Bundle

Dependency-injected container holding:
- **Decoder Registry** — Maps file extensions to decoders (`.json`, `.yaml`, `.toml`, `.env`)
- **Loader Registry** — Maps loader names to loader instances
- **Provider Registry** — Maps provider names to config providers

---

## Configuration Files

| File | Format | Purpose |
|------|--------|---------|
| `yml_config.yml` | YAML | Base application configuration (lowest file priority) |
| `json_config.json` | JSON | Staging environment overrides |
| `toml_config.toml` | TOML | Local development overrides (highest file priority) |
| `configdata.go` | Go | Hard-coded YAML/JSON/TOML config strings for self-contained demos |
| `main.go` | Go | Comprehensive example demonstrating all 28 features |

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "github.com/os-gomod/config/v2"
    "github.com/os-gomod/config/v2/internal/domain/layer"
    "github.com/os-gomod/config/v2/internal/domain/value"
)

func main() {
    // Create in-memory data
    data := make(map[string]value.Value)
    data["app.name"] = value.New("my-app")
    data["server.port"] = value.New(8080)
    memLayer := layer.NewStaticLayer("defaults", data, layer.WithPriority(10))

    // Create config with functional options
    cfg, err := config.New(context.Background(),
        config.WithLayer(memLayer),
        config.WithDeltaReload(true),
    )
    if err != nil { panic(err) }
    defer cfg.Close(context.Background())

    // Read values
    if v, ok := cfg.Get("app.name"); ok {
        fmt.Printf("App: %s\n", v.String())
    }
}
```

**Run the full example:**

```bash
go run main.go
```

---

## All 28 Features

### 1. Multi-Format File Loading

Load configuration from JSON, TOML, YAML, in-memory maps, and environment variables simultaneously. Each source becomes a layer with a configurable priority.

```go
package main

import (
    "context"
    "github.com/os-gomod/config/v2"
    "github.com/os-gomod/config/v2/internal/domain/layer"
    "github.com/os-gomod/config/v2/internal/domain/value"
    "github.com/os-gomod/config/v2/internal/loader"
)

func main() {
    ctx := context.Background()

    // Helper: convert raw map to value.Value map
    memoryData := func(raw map[string]any) map[string]value.Value {
        data := make(map[string]value.Value, len(raw))
        for k, v := range raw {
            data[k] = value.New(v)
        }
        return data
    }

    var layers []*layer.Layer

    // (a) Memory defaults — priority 10
    defaults := memoryData(map[string]any{
        "app.name":    "default-app",
        "server.port": 3000,
    })
    layers = append(layers, layer.NewStaticLayer("memory-defaults", defaults, layer.WithPriority(10)))

    // (b) JSON config file — priority 30
    jsonLoader := loader.NewFileLoader("json-config", []string{"config.json"}, nil)
    if jsonData, err := jsonLoader.Load(ctx); err == nil && len(jsonData) > 0 {
        layers = append(layers, layer.NewStaticLayer("json-file", jsonData, layer.WithPriority(30)))
    }

    // (c) YAML config file — priority 40
    yamlLoader := loader.NewFileLoader("yaml-config", []string{"config.yml"}, nil)
    if yamlData, err := yamlLoader.Load(ctx); err == nil && len(yamlData) > 0 {
        layers = append(layers, layer.NewStaticLayer("yaml-file", yamlData, layer.WithPriority(40)))
    }

    // (d) Environment variables — priority 50 (highest)
    envLoader := loader.NewEnvLoader("env-vars", loader.WithEnvPrefix("APP_"), loader.WithEnvPriority(50))
    if envData, err := envLoader.Load(ctx); err == nil && len(envData) > 0 {
        layers = append(layers, layer.NewStaticLayer("env-layer", envData, layer.WithPriority(50)))
    }

    cfg, err := config.New(ctx, config.WithLayers(layers...))
    if err != nil { panic(err) }
    defer cfg.Close(ctx)
}
```

> **v2 Note:** `loader.NewFileLoader` auto-detects format by file extension via the `DecoderRegistry`. Environment variable loading now uses `loader.NewEnvLoader` with `WithEnvPrefix` and `WithEnvPriority` options.

---

### 2. Functional Options

All configuration is done via `config.New(ctx, opts...)` functional options — no Builder pattern.

```go
cfg, err := config.New(ctx,
    config.WithLayer(memLayer),              // Add a configuration layer
    config.WithLayers(layer1, layer2, ...),  // Add multiple layers at once
    config.WithMaxWorkers(4),                // Max concurrent reload workers
    config.WithDeltaReload(true),            // Skip unchanged layers on reload
    config.WithStrictReload(true),           // Fail on reload errors instead of continuing
    config.WithDebounce(300*time.Millisecond), // Debounce rapid reload triggers
    config.WithBusWorkers(16),               // Event bus worker pool size
    config.WithBusQueueSize(2048),           // Event bus bounded queue size
    config.WithNamespace("tenant.acme."),    // Multi-tenancy namespace prefix
    config.WithRecorder(recorder),           // Observability recorder
    config.WithTracer(tracer),               // Distributed tracing
    config.WithLogger(slog.Default()),        // Structured logger
    config.WithRegistryBundle(bundle),       // Dependency-injected registries
    config.WithOnReloadError(func(err error) { // Background reload error callback
        log.Printf("reload error: %v", err)
    }),
    config.WithPlugins(myPlugin),            // Plugin instances
)
```

> **v2 Note:** Replaces the v1 Builder pattern entirely. All options are composable and type-safe. The `ctx` parameter enables context propagation into all services.

---

### 3. Struct Binding

Map configuration keys to Go structs using `config:"key"` struct tags.

```go
type AppConfig struct {
    App      AppSection      `config:"app"`
    Server   ServerSection   `config:"server"`
    Database DatabaseSection `config:"database"`
}

type AppSection struct {
    Name    string `config:"name"    validate:"required"`
    Version string `config:"version" validate:"required"`
    Env     string `config:"env"     validate:"required"`
    Debug   bool   `config:"debug"`
}

type ServerSection struct {
    Host      string        `config:"host" validate:"required"`
    Port      int           `config:"port" validate:"min=1,max=65535"`
    ReadTimeout time.Duration `config:"read_timeout"`
    EnableTLS bool          `config:"enable_tls"`
    RateLimit int           `config:"rate_limit"`
}

var appCfg AppConfig
if err := cfg.Bind(ctx, &appCfg); err != nil {
    log.Fatalf("bind error: %v", err)
}
fmt.Printf("App: %s:%d\n", appCfg.Server.Host, appCfg.Server.Port)
```

> **v2 Note:** `Bind` routes through the Command Pipeline as a pass-through. For manual binding, use `cfg.GetAll()` which returns `map[string]value.Value` and access values via typed methods (`.String()`, `.Int()`, `.Bool()`, etc.).

---

### 4. Validation

Built-in validation using `go-playground/validator` with custom tags: `duration`, `port`, `absurl`, `envprefix`.

```go
v := validator.New()

// Validate a struct
valid := AppSection{Name: "svc", Version: "1.0", Env: "production"}
if err := v.Validate(ctx, valid); err != nil {
    log.Printf("validation failed: %v", err)
}

// Invalid struct — missing required fields
invalid := AppSection{Name: "", Version: "1.0", Env: "invalid_env"}
if err := v.Validate(ctx, invalid); err != nil {
    log.Printf("caught invalid: %v", err) // Name is required
}

// Validate via Config object
var target AppSection
if err := cfg.Validate(ctx, &target); err != nil {
    log.Printf("config validation error: %v", err)
}
```

> **v2 Note:** The `validator.New()` constructor returns a ready-to-use validator with custom tag registrations for `duration`, `port`, `absurl`, and `envprefix`. The `Config.Validate()` method binds first, then validates.

---

### 5. Layered Loading with Priorities

Configuration layers are merged by priority — higher priority wins when keys overlap.

```go
// Base layer (priority 10)
baseData := memoryData(map[string]any{
    "db.host":  "localhost",
    "db.port":  5432,
    "db.name":  "mydb",
})
baseLayer := layer.NewStaticLayer("base", baseData, layer.WithPriority(10))

// Override layer (priority 30)
overrideData := memoryData(map[string]any{
    "db.host": "prod-db.internal",
})
overrideLayer := layer.NewStaticLayer("override", overrideData, layer.WithPriority(30))

// Env layer (priority 50)
envData := memoryData(map[string]any{
    "db.port": 6432,
})
envLayer := layer.NewStaticLayer("env-override", envData, layer.WithPriority(50))

cfg, _ := config.New(ctx,
    config.WithLayer(baseLayer),
    config.WithLayer(overrideLayer),
    config.WithLayer(envLayer),
)

fmt.Println(cfg.Get("db.host"))  // "prod-db.internal" (from override, priority=30)
fmt.Println(cfg.Get("db.port"))  // 6432 (from env, priority=50)
fmt.Println(cfg.Get("db.name"))  // "mydb" (from base, no override)
```

**Merge order:** `env(50) > override(30) > base(10)`

> **v2 Note:** Layers are sorted internally by priority at construction time. `layer.NewStaticLayer` accepts `layer.WithPriority(N)` as a functional option.

---

### 6. Event System

Bounded async event dispatcher with pattern-based subscriptions and typed events.

```go
memData := memoryData(map[string]any{"counter": 0})
cfg, _ := config.New(ctx, config.WithLayer(memData))

// Subscribe to ALL events (returns unsubscribe function)
unsubAll := cfg.Subscribe(func(_ context.Context, evt domainevent.Event) error {
    fmt.Printf("[EVENT] type=%s key=%s old=%v new=%v\n",
        evt.EventType, evt.Key, evt.OldValue.Raw(), evt.NewValue.Raw())
    return nil
})

// Subscribe by key pattern (exact key or dot-segment glob)
unsubPattern := cfg.WatchPattern("counter", func(_ context.Context, evt domainevent.Event) error {
    fmt.Printf("[PATTERN] key=%s type=%s new=%v\n", evt.Key, evt.EventType, evt.NewValue.Raw())
    return nil
})

// Trigger events
cfg.Set(ctx, "counter", 1)  // fires Update event
cfg.Set(ctx, "counter", 2)  // fires Update event

// Unsubscribe
unsubAll()
unsubPattern()
cfg.Set(ctx, "counter", 3)  // no events fired

// Create events with rich metadata
evt := domainevent.New(domainevent.TypeUpdate, "demo.key",
    domainevent.WithTraceID("trace-abc-123"),
    domainevent.WithLabels(map[string]string{"region": "us-east-1"}),
    domainevent.WithMetadata(map[string]any{"source": "cli"}),
)
```

**Typed Events:** `Create`, `Update`, `Delete`, `Reload`, `Error`, `Watch`, `Audit`

> **v2 Note:** Events are dispatched asynchronously via a worker pool. The `Subscribe` and `WatchPattern` methods return unsubscribe functions for clean teardown.

---

### 7. Typed Interceptors

v2 replaces v1 hooks with **typed interceptors** — each operation type has its own interface with dedicated request/response types.

```go
// Set Interceptor
setInterceptor := &interceptor.SetFunc{
    BeforeFn: func(_ context.Context, req *interceptor.SetRequest) error {
        if strings.Contains(req.Key, "secret") {
            log.Printf("auditing access to key=%s", req.Key)
        }
        return nil
    },
    AfterFn: func(_ context.Context, req *interceptor.SetRequest, res *interceptor.SetResponse) error {
        log.Printf("set key=%s created=%v", res.Key, res.Created)
        return nil
    },
}

// Delete Interceptor
deleteInterceptor := &interceptor.DeleteFunc{
    BeforeFn: func(_ context.Context, req *interceptor.DeleteRequest) error {
        log.Printf("deleting key=%s", req.Key)
        return nil
    },
    AfterFn: func(_ context.Context, req *interceptor.DeleteRequest, res *interceptor.DeleteResponse) error {
        log.Printf("deleted key=%s found=%v", res.Key, res.Found)
        return nil
    },
}

// Reload Interceptor
reloadInterceptor := &interceptor.ReloadFunc{
    BeforeFn: func(_ context.Context, req *interceptor.ReloadRequest) error {
        log.Printf("reloading layers=%v", req.LayerNames)
        return nil
    },
    AfterFn: func(_ context.Context, req *interceptor.ReloadRequest, res *interceptor.ReloadResponse) error {
        log.Printf("reload changed=%v events=%d", res.Changed, len(res.Events))
        return nil
    },
}

// Chain manages ordered execution
chain := interceptor.NewChain()
chain.AddSetInterceptor(setInterceptor)
chain.AddDeleteInterceptor(deleteInterceptor)
chain.AddReloadInterceptor(reloadInterceptor)
fmt.Printf("Total interceptors: %d\n", chain.TotalCount())
```

**Interceptor Types:**

| Type | Request | Response |
|------|---------|----------|
| `SetFunc` | `SetRequest{Key, Value}` | `SetResponse{Key, Created}` |
| `DeleteFunc` | `DeleteRequest{Key}` | `DeleteResponse{Key, Found}` |
| `ReloadFunc` | `ReloadRequest{LayerNames}` | `ReloadResponse{Changed, Events}` |
| `BindFunc` | `BindRequest{Target}` | `BindResponse{Keys}` |
| `CloseFunc` | `CloseRequest` | `CloseResponse` |

> **v2 Note:** Each `*Func` struct is a functional adapter — implement only the closures you need. The `Chain` manages ordered execution and provides per-type counts.

---

### 8. Schema Generation

Generate JSON Schema from Go structs for documentation, validation UIs, or IDE integration.

```go
gen := schema.New()

sch, err := gen.Generate(AppConfig{})
if err != nil {
    log.Fatalf("schema generation error: %v", err)
}

b, _ := json.MarshalIndent(sch, "", "  ")
fmt.Printf("Generated Schema (%d bytes):\n%s\n", len(b), string(b))
```

> **v2 Note:** `schema.New()` creates a generator that respects `config` and `validate` struct tags to produce accurate JSON Schema with type constraints, required fields, and validation rules.

---

### 9. Snapshot / Restore

Capture and restore the entire configuration state. Secrets are automatically redacted in snapshots.

```go
cfg, _ := config.New(ctx, config.WithLayer(memLayer))

// Mutate state
cfg.Set(ctx, "a", 2)
cfg.Set(ctx, "b", "world")

// Take snapshot (secrets are redacted)
snap := cfg.Snapshot()
fmt.Printf("Snapshot keys=%d, a=%v\n", len(snap), snap["a"].Raw())

// Mutate further
cfg.Set(ctx, "a", 99)
cfg.Delete(ctx, "b")
cfg.Delete(ctx, "c")
fmt.Printf("After mutation: a=%v, b exists=%v\n", mustGet(cfg, "a"), cfg.Has("b"))

// Restore from snapshot (reverts all changes)
cfg.Restore(snap)
fmt.Printf("After restore: a=%v, b=%v, c=%v\n",
    mustGet(cfg, "a"), mustGet(cfg, "b"), mustGet(cfg, "c"))
```

> **v2 Note:** Snapshots return `map[string]value.Value` — an immutable copy. Secrets (keys containing `password`, `secret`, `token`, `key`) are automatically redacted to `*****`.

---

### 10. Command Pipeline

All mutating operations (`Set`, `Delete`, `Reload`, `Bind`, `Close`) route through a middleware chain.

```go
cfg, _ := config.New(ctx,
    config.WithLayer(memLayer),
    config.WithStrictReload(true),
    config.WithDeltaReload(true),
    config.WithOnReloadError(func(err error) {
        log.Printf("background reload error: %v", err)
    }),
)

// All mutations flow through the pipeline
cfg.Set(ctx, "pipeline.test", "updated via pipeline")
cfg.BatchSet(ctx, map[string]any{"pipeline.a": 1, "pipeline.b": 2})
cfg.Delete(ctx, "pipeline.b")

result, err := cfg.Reload(ctx)
if err != nil {
    log.Printf("reload error: %v", err)
} else {
    fmt.Printf("Reload: changed=%v, hasErrors=%v\n", result.Changed, result.HasErrors())
}
```

**Built-in Middleware (in order):**

| Middleware | Purpose |
|-----------|---------|
| CorrelationID | Auto-generates trace IDs for request tracking |
| Tracing | Distributed tracing integration |
| Metrics | Per-operation recording (count, duration, errors) |
| Audit | Immutable audit log entries |
| Logging | Structured `slog` logging |
| Recovery | Catches panics and converts to `AppError` |

> **v2 Note:** The pipeline is created internally by `config.New()` and cannot be directly customized. Observability options (`WithRecorder`, `WithTracer`, `WithLogger`) configure pipeline behavior.

---

### 11. Explain (Key Provenance)

Trace any configuration key to its source, priority, and value.

```go
cfg, _ := config.New(ctx, config.WithLayer(memLayer))

fmt.Println(cfg.Explain("db.host"))
// Output: key="db.host" value="prod-db.internal" source="override" priority=30

fmt.Println(cfg.Explain("nonexistent"))
// Output: key="nonexistent" not found
```

> **v2 Note:** `Explain` returns a formatted string containing the key name, current value, source layer name, and priority. Useful for debugging configuration conflicts.

---

### 12. Profiles

Named configuration presets with layer definitions, options, and parent inheritance.

```go
// Create a development profile
devLayers := []profile.LayerDef{
    {Name: "dev-memory", Type: "memory", Source: "inline", Priority: 10, Enabled: true},
}
devProfile := profile.NewProfile("development", devLayers, map[string]any{
    "app.env":     "development",
    "db.host":     "localhost",
    "server.port": 8080,
    "app.debug":   true,
    profile.OptNamespace:    "dev.",
    profile.OptStrictReload: false,
})
devProfile.Description = "Local development settings"

// Access typed options
fmt.Println(devProfile.OptionString(profile.OptNamespace, ""))
fmt.Println(devProfile.OptionBool(profile.OptStrictReload, true))

// Profile registry
reg := profile.NewRegistry()
reg.Register(devProfile)
fmt.Println("Registered profiles:", reg.List())

// Resolve profile
resolved, err := reg.Resolve("development")
if err != nil {
    log.Fatalf("resolve error: %v", err)
}
fmt.Println("Resolved:", resolved.String())
```

> **v2 Note:** Profiles support parent inheritance — a profile can inherit layers and options from a base profile. Resolution merges the profile chain from parent to child.

---

### 13. Feature Flags

Boolean, Percentage (0-100 rollout), and Variant (A/B/n) flags with rule-driven evaluation.

```go
// Define flags
flagData := map[string]featureflag.Flag{
    "new_ui": {
        Key:      "new_ui",
        Enabled:  true,
        FlagType: featureflag.FlagTypeBoolean,
    },
    "beta_rollout": {
        Key:        "beta_rollout",
        Enabled:    true,
        FlagType:   featureflag.FlagTypePercentage,
        Percentage: 30,  // 30% of users
    },
    "experiment": {
        Key:      "experiment",
        Enabled:  true,
        FlagType: featureflag.FlagTypeVariant,
        Variants: map[string]any{"control": nil, "treatment": nil, "v2": nil},
        Default:  "control",
    },
}

// Implement ConfigProvider interface
type staticFlagProvider struct {
    flags map[string]featureflag.Flag
}
func (p *staticFlagProvider) GetFlags() map[string]featureflag.Flag { return p.flags }
func (p *staticFlagProvider) GetFlag(key string) *featureflag.Flag {
    if f, ok := p.flags[key]; ok { return &f }
    return nil
}

provider := &staticFlagProvider{flags: flagData}
engine := featureflag.NewEngine(provider)

// Boolean flag
if engine.Bool("new_ui", nil) {
    fmt.Println("new_ui: ENABLED")
}

// Percentage rollout — deterministic per-user
users := []string{"alice", "bob", "charlie", "dave", "eve", "frank"}
for _, user := range users {
    evalCtx := featureflag.NewEvalContext(user)
    eval := engine.Evaluate("beta_rollout", evalCtx)
    fmt.Printf("  user '%s' -> matched=%v reason=%s\n", user, eval.Matched, eval.Reason)
}

// Variant experiment
for _, user := range users {
    evalCtx := featureflag.NewEvalContext(user)
    eval := engine.Evaluate("experiment", evalCtx)
    if eval.Matched {
        fmt.Printf("  user '%s' -> variant=%s\n", user, eval.VariantKey)
    }
}

// List all flags
fmt.Println("All flags:", engine.ListFlags())
```

| Flag Type | Use Case | Evaluation |
|-----------|----------|------------|
| `Boolean` | Simple on/off toggles | `engine.Bool(key, ctx)` |
| `Percentage` | Gradual rollouts (0-100) | Deterministic hash per entity |
| `Variant` | A/B/n experiments | Deterministic variant assignment |

> **v2 Note:** Feature flags use a `ConfigProvider` interface for pluggable flag sources. `NewEvalContext(entityID)` enables user/tenant-scoped evaluation with deterministic results.

---

### 14. Multi-Tenancy

Namespace-prefixed key isolation for multi-tenant applications.

```go
// Create config with tenant namespace
cfg, err := config.New(ctx,
    config.WithLayer(memLayer),
    config.WithNamespace("tenant.acme."),
)
if err != nil { panic(err) }
defer cfg.Close(ctx)

// Set tenant-scoped keys (namespace is auto-prefixed)
cfg.Set(ctx, "db.host", "acme-db.internal")

// Switch namespace at runtime
cfg.SetNamespace(ctx, "tenant.globex.")
cfg.Set(ctx, "db.host", "globex-db.internal")

// Read from specific namespace
v, ok := cfg.Get("db.host")
fmt.Printf("Current tenant db.host: %s\n", v.String())
```

> **v2 Note:** `config.WithNamespace("tenant.acme.")` sets the default namespace. `cfg.SetNamespace(ctx, "tenant.globex.")` switches at runtime. All `Set`/`Get`/`Has`/`Delete` operations are namespace-prefixed.

---

### 15. Observability

Per-operation recording with callback-based and metrics-based approaches.

```go
import "github.com/os-gomod/config/v2/internal/observability"

// Callback recorder — captures every operation for testing/debugging
recorder := observability.NewCallbackRecorder()
recorder.OnOperation(func(op observability.Operation) {
    fmt.Printf("[%s] key=%s duration=%v error=%v\n",
        op.Type, op.Key, op.Duration, op.Error)
})

// Use with Config
cfg, _ := config.New(ctx,
    config.WithLayer(memLayer),
    config.WithRecorder(recorder),
)

// Metrics recorder — exposes Prometheus-style counters
metrics := observability.NewMetrics()

// No-op recorder for production (zero overhead)
nop := observability.Nop()
_ = nop // use when no observability is needed
```

> **v2 Note:** `WithRecorder` injects the recorder into the Command Pipeline middleware. All operations (`Set`, `Delete`, `Reload`, `Bind`) are automatically recorded.

---

### 16. Secure Storage

Pluggable secret storage backends with caching.

```go
import "github.com/os-gomod/config/v2/internal/secure"

// Store interface
type Store interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    Delete(ctx context.Context, key string) error
}

// Vault integration
vaultStore := secure.VaultStore{
    Address: "https://vault.internal:8200",
    Token:   "vault-token",
    Prefix:  "secret/data/app",
}

// KMS integration
kmsStore := secure.KMSStore{
    Region: "us-east-1",
    KeyID:  "alias/config-secrets",
}

// Cached wrapper — reduces external calls
cachedStore := secure.NewCachedStore(vaultStore, 5*time.Minute)
secret, err := cachedStore.Get(ctx, "database.password")
```

> **v2 Note:** `secure.Store` is an interface — implement custom backends by providing `Get`, `Set`, and `Delete` methods. `NewCachedStore` wraps any store with TTL-based caching.

---

### 17. Plugin System

Extend configuration with plugins using the `service.Plugin` interface.

```go
import "github.com/os-gomod/config/v2/internal/service"

// Implement the Plugin interface
type greetingPlugin struct{}

func (greetingPlugin) Name() string { return "greeting-plugin" }

func (greetingPlugin) Init(host service.PluginHost) error {
    // PluginHost provides typed methods for extending the config:
    //   host.RegisterLoader(name, loader)
    //   host.RegisterProvider(name, provider)
    //   host.RegisterDecoder(ext, decoder)
    //   host.RegisterValidator(name, validator)
    //   host.Subscribe(handler)
    fmt.Println("greeting-plugin initialized successfully")
    return nil
}

// Register plugin during construction
cfg, _ := config.New(ctx,
    config.WithLayer(memLayer),
    config.WithPlugins(greetingPlugin{}),
)
```

**PluginHost Methods:**

| Method | Description |
|--------|-------------|
| `RegisterLoader(name, loader)` | Register a custom configuration loader |
| `RegisterProvider(name, provider)` | Register a configuration provider |
| `RegisterDecoder(ext, decoder)` | Register a file format decoder |
| `RegisterValidator(name, validator)` | Register a custom validator |
| `Subscribe(handler)` | Subscribe to configuration events |

> **v2 Note:** Plugins receive a `PluginHost` during `Init()` that provides typed access to all extension points. This replaces v1's manual registration calls.

---

### 18. Watcher / Debounce

File system watching with configurable debounce intervals.

```go
import "github.com/os-gomod/config/v2/internal/watcher"

// WatchManager monitors files for changes
wm := watcher.NewWatchManager()

// Debounce rapid file changes
debouncer := watcher.NewDebounce(300 * time.Millisecond)

// Full manager with debounce
mgr := watcher.NewManager(
    watcher.WithDebounce(300*time.Millisecond),
    watcher.WithPollInterval(1*time.Second),
)

// Watch a directory
mgr.Watch("/etc/app/config", func(event watcher.Event) {
    fmt.Printf("file changed: %s\n", event.Path)
})
```

> **v2 Note:** `watcher.NewDebounce(interval)` coalesces rapid file system events. The watcher integrates with the reload pipeline — file changes trigger debounced reloads.

---

### 19. Value Types

Typed value access with type conversion, checksums, redaction, and secret detection.

```go
import "github.com/os-gomod/config/v2/internal/domain/value"

// Create values
v1 := value.New("hello")
v2 := value.NewInMemory()         // empty value
v3 := value.FromRaw(42)           // from raw any

// Typed accessors
v := value.New(8080)
v.String()     // "8080"
v.Int()        // 8080
v.Bool()       // false (zero value)
v.Float64()    // 8080.0
v.Duration()   // parsed duration string
v.Slice()      // slice value
v.Map()        // map value
v.Raw()        // underlying any
v.Type()       // type name string
v.Source()     // source layer name

// Generic type access
var port int
if err := v.As(&port); err == nil {
    fmt.Printf("port=%d\n", port)
}

// Checksum for change detection
fmt.Println("checksum:", v.Checksum())

// Redaction (for snapshots)
if v.IsSecret() {
    fmt.Println("*****")  // redacted
}

// Secret detection (keys containing password, secret, token, key)
secretVal := value.New("my-password-123")
fmt.Println(secretVal.IsSecret())  // true
```

> **v2 Note:** `value.Value` is the core type used throughout the library. All configuration values are stored as `value.Value` in layers and snapshots.

---

### 20. AppError Handling

22 error codes with Severity, Retryable flag, CorrelationID, Operation, Key, Source, and StackTrace.

```go
import apperrors "github.com/os-gomod/config/v2/internal/domain/errors"

// Fluent builder pattern
err := apperrors.Build(
    apperrors.CodeConfigNotFound,
    "configuration key not found",
    apperrors.WithSeverity(apperrors.SeverityHigh),
    apperrors.WithOperation("config.Get"),
    apperrors.WithKey("database.host"),
    apperrors.WithSource("memory-defaults"),
).Wrap(fmt.Errorf("layer not loaded"))

// Access error properties
if appErr, ok := apperrors.AsAppError(err); ok {
    fmt.Println("Code:", appErr.Code())              // e.g., "config_not_found"
    fmt.Println("Message:", appErr.Error())          // human-readable message
    fmt.Println("Severity:", appErr.Severity())      // Low, Medium, High, Critical
    fmt.Println("Retryable:", appErr.Retryable())    // true/false
    fmt.Println("CorrelationID:", appErr.CorrelationID()) // trace ID
    fmt.Println("Operation:", appErr.Operation())    // e.g., "config.Get"
    fmt.Println("Key:", appErr.Key())                // e.g., "database.host"
    fmt.Println("Source:", appErr.Source())          // e.g., "memory-defaults"
    fmt.Println("StackTrace:", appErr.StackTrace())  // stack trace string
}

// Check error codes
if apperrors.IsCode(err, apperrors.CodeConfigNotFound) {
    // handle missing config
}

// Severity levels
const (
    SeverityLow      = "low"
    SeverityMedium   = "medium"
    SeverityHigh     = "high"
    SeverityCritical = "critical"
)
```

**Selected Error Codes:**

| Code | Description |
|------|-------------|
| `CodeConfigNotFound` | Configuration key not found |
| `CodeValidationFailed` | Struct validation failed |
| `CodeLayerLoadError` | Failed to load a layer |
| `CodeDecodeError` | Failed to decode config data |
| `CodeReloadError` | Configuration reload failed |
| `CodePluginError` | Plugin initialization failed |
| `CodePermissionDenied` | Access denied to secure store |

> **v2 Note:** All errors from the library implement the `AppError` interface. The `Build` function uses functional options for a fluent API. `Recovery` middleware converts panics to `AppError` with `SeverityCritical`.

---

### 21. Environment Variable Loader

Load configuration from environment variables with prefix filtering and priority control.

```go
import "github.com/os-gomod/config/v2/internal/loader"

// Create env loader with prefix (e.g., APP_SERVER_HOST -> server.host)
envLoader := loader.NewEnvLoader("env-vars",
    loader.WithEnvPrefix("APP_"),     // strip APP_ prefix
    loader.WithEnvPriority(50),       // layer priority
)

// Load environment variables as a layer
data, err := envLoader.Load(ctx)
if err != nil {
    log.Fatalf("env load error: %v", err)
}

// Use as a layer
envLayer := layer.NewStaticLayer("env-layer", data, layer.WithPriority(50))
cfg, _ := config.New(ctx, config.WithLayer(envLayer))
```

**Environment Variable Mapping:**

| Environment Variable | Config Key |
|---------------------|------------|
| `APP_SERVER_HOST` | `server.host` |
| `APP_SERVER_PORT` | `server.port` |
| `APP_DB_PASSWORD` | `db.password` |

> **v2 Note:** `WithEnvPrefix("APP_")` strips the prefix and converts to lowercase dot-notation. `WithEnvPriority(N)` sets the layer merge priority.

---

### 22. Decoder Registry

Maps file extensions to format-specific decoders. Ships with built-in support for YAML, JSON, TOML, and Env.

```go
import "github.com/os-gomod/config/v2/internal/decoder"

// Default registry (pre-registered: .json, .yaml, .yml, .toml, .env)
reg := decoder.NewDefaultRegistry()

// Lookup decoder by extension
dec, ok := reg.ForExtension(".json")
if ok {
    data, err := dec.Decode(jsonBytes)
}

dec, ok = reg.ForExtension(".yaml")
dec, ok = reg.ForExtension(".toml")
dec, ok = reg.ForExtension(".env")

// Register custom decoder
reg.Register(".custom", myCustomDecoder{})
```

**Built-in Decoders:**

| Extension | Format | Dependencies |
|-----------|--------|-------------|
| `.json` | JSON | `encoding/json` |
| `.yaml`, `.yml` | YAML | `gopkg.in/yaml.v3` |
| `.toml` | TOML | `github.com/BurntSushi/toml` |
| `.env` | Environment | Built-in parser |

> **v2 Note:** The `DecoderRegistry` is part of the `RegistryBundle` and is injected via `config.WithRegistryBundle()`. Custom decoders implement the `decoder.Decoder` interface.

---

### 23. BatchSet

Atomically set multiple keys in a single operation through the Command Pipeline.

```go
cfg, _ := config.New(ctx, config.WithLayer(memLayer))

// Set multiple keys atomically
err := cfg.BatchSet(ctx, map[string]any{
    "server.host":     "0.0.0.0",
    "server.port":     9090,
    "database.driver": "postgres",
    "database.host":   "db.internal",
    "database.port":   5432,
    "cache.enabled":   true,
})
if err != nil {
    log.Printf("batch set error: %v", err)
}

// BatchSet fires a single composite event, not individual events per key
```

> **v2 Note:** `BatchSet` routes through the Command Pipeline as a single command. All keys are set atomically, and a single batch event is published.

---

### 24. Delta Reload

Skip unchanged layers during reload using content checksums.

```go
cfg, _ := config.New(ctx,
    config.WithLayer(jsonLayer),
    config.WithLayer(yamlLayer),
    config.WithDeltaReload(true),    // enable delta reload
    config.WithStrictReload(true),   // fail on any layer error
)

// Reload only reloads layers whose content has changed (checksum comparison)
result, err := cfg.Reload(ctx)
if err != nil {
    log.Printf("reload error: %v", err)
}
fmt.Printf("changed=%v, hasErrors=%v\n", result.Changed, result.HasErrors())

// Handle background reload errors
cfg2, _ := config.New(ctx,
    config.WithLayer(memLayer),
    config.WithDeltaReload(true),
    config.WithOnReloadError(func(err error) {
        log.Printf("background reload error: %v", err)
    }),
)
```

> **v2 Note:** `WithDeltaReload(true)` computes checksums for each layer on reload. Layers with unchanged checksums are skipped, reducing unnecessary re-processing and event noise.

---

### 25. Event Bus (Direct)

Direct access to the underlying event bus for custom pub/sub patterns.

```go
import "github.com/os-gomod/config/v2/internal/eventbus"

// Create event bus with custom configuration
bus := eventbus.NewBus(
    eventbus.WithWorkerCount(8),    // worker pool size
    eventbus.WithQueueSize(1024),   // bounded queue capacity
)

// Subscribe to events
unsub := bus.Subscribe(func(ctx context.Context, evt domainevent.Event) error {
    fmt.Printf("bus event: type=%s key=%s\n", evt.EventType, evt.Key)
    return nil
})

// Publish async (queued)
bus.Publish(ctx, domainevent.New(domainevent.TypeUpdate, "my.key"))

// Publish sync (waits for all handlers)
err := bus.PublishSync(ctx, domainevent.New(domainevent.TypeUpdate, "my.key"))

// Unsubscribe
unsub()
```

> **v2 Note:** The event bus uses a worker pool with a bounded queue. If the queue is full, `Publish` blocks until space is available. Use `PublishSync` when you need to wait for all handlers to complete.

---

### 26. Registry Bundle

Dependency-injected container holding Decoder, Loader, and Provider registries.

```go
import "github.com/os-gomod/config/v2/internal/registry"

// Default bundle (pre-populated with standard registries)
bundle := registry.NewDefaultBundle()

// Access individual registries
decRegistry := bundle.DecoderRegistry()   // maps extensions -> decoders
ldrRegistry := bundle.LoaderRegistry()     // maps names -> loaders
provRegistry := bundle.ProviderRegistry()  // maps names -> providers

// Use with Config
cfg, _ := config.New(ctx,
    config.WithLayer(memLayer),
    config.WithRegistryBundle(bundle),
)

// Or create custom bundle and register components
customBundle := registry.NewBundle()
customBundle.SetDecoderRegistry(decoder.NewDefaultRegistry())
```

| Registry | Purpose | Methods |
|----------|---------|---------|
| **DecoderRegistry** | Format decoders | `ForExtension(ext)`, `Register(ext, decoder)` |
| **LoaderRegistry** | Config loaders | `Get(name)`, `Register(name, loader)` |
| **ProviderRegistry** | Config providers | `Get(name)`, `Register(name, provider)` |

> **v2 Note:** Replaces v1's global/package-level registries with dependency injection. The bundle is passed via `config.WithRegistryBundle()` and shared across all services.

---

### 27. Audit System

Immutable audit log entries for compliance and security tracking.

```go
import domainevent "github.com/os-gomod/config/v2/internal/domain/event"

// Create audit entries with 7 action types
entry := domainevent.NewAuditEntry(
    domainevent.AuditActionSet,       // action
    "database.password",              // key
    "env-layer",                      // source
    "admin-user",                     // actor
)

// Audit Actions:
//   AuditActionCreate  — new key created
//   AuditActionRead    — key was read
//   AuditActionUpdate  — key value changed
//   AuditActionDelete  — key was deleted
//   AuditActionReload  — configuration reloaded
//   AuditActionAccess  — access attempt (authorized/unauthorized)
//   AuditActionExport  — configuration exported/snapshotted

// Create audit event from entry
evt := domainevent.NewAuditEvent(entry)

// Publish to event bus
bus.Publish(ctx, evt)
```

> **v2 Note:** Audit entries are created automatically by the Audit middleware in the Command Pipeline. Manual audit entries can be created for custom operations. All audit events include timestamp, actor, action, key, and source.

---

### 28. Backoff Strategy

Configurable exponential backoff with jitter for retry logic.

```go
import "github.com/os-gomod/config/v2/internal/backoff"

// Create backoff strategy
b := backoff.New(
    backoff.WithInitial(100*time.Millisecond),  // initial delay
    backoff.WithMax(10*time.Second),            // maximum delay cap
    backoff.WithFactor(2.0),                    // exponential multiplier
    backoff.WithJitter(true),                   // add random jitter
)

// Create stopper with max attempts
stopper := backoff.NewStopper(b, 5)

// Retry loop
for stopper.Continue() {
    err := doSomething()
    if err == nil {
        break // success
    }
    if stopper.IsStopped() {
        log.Fatalf("max retries exceeded")
    }
    stopper.Wait(ctx) // waits with backoff + jitter
}
fmt.Printf("attempts=%d\n", stopper.Attempts())
```

**Backoff Parameters:**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `Initial` | 100ms | Initial retry delay |
| `Max` | 10s | Maximum delay cap |
| `Factor` | 2.0 | Exponential multiplier |
| `Jitter` | false | Add random jitter to prevent thundering herd |

> **v2 Note:** The `Stopper` provides `Continue()`, `IsStopped()`, `Wait(ctx)`, and `Attempts()` for clean retry loop control. Used internally for reload and watcher retry logic.

---

## v1 to v2 Migration Guide

### Construction

```go
// v1 — Builder Pattern
cfg := config.NewBuilder().
    WithLoader(jsonLoader).
    WithLayer(memoryLayer).
    Build()

// v2 — Functional Options
cfg, err := config.New(ctx,
    config.WithLayer(memoryLayer),
    config.WithDeltaReload(true),
)
```

### Hooks to Interceptors

```go
// v1 — Generic hooks
builder.BeforeSet(func(key string, val any) error { ... })
builder.AfterSet(func(key string, val any) { ... })

// v2 — Typed Interceptors
setInterceptor := &interceptor.SetFunc{
    BeforeFn: func(_ context.Context, req *interceptor.SetRequest) error { ... },
    AfterFn:  func(_ context.Context, req *interceptor.SetRequest, res *interceptor.SetResponse) error { ... },
}
chain.AddSetInterceptor(setInterceptor)
```

### Plugin Registration

```go
// v1 — Manual registration
cfg.RegisterLoader("my-loader", loader)

// v2 — Plugin interface with PluginHost
type myPlugin struct{}
func (myPlugin) Name() string { return "my-plugin" }
func (myPlugin) Init(host service.PluginHost) error {
    host.RegisterLoader("my-loader", loader)
    return nil
}
cfg, _ := config.New(ctx, config.WithPlugins(myPlugin{}))
```

### Error Handling

```go
// v1 — fmt.Errorf
return fmt.Errorf("failed to load config: %w", err)

// v2 — AppError with full context
return apperrors.Build(
    apperrors.CodeLayerLoadError,
    "failed to load config",
    apperrors.WithSeverity(apperrors.SeverityHigh),
    apperrors.WithRetryable(true),
).Wrap(err)
```

### Global Registries to Dependency Injection

```go
// v1 — Package-level singletons
decoder.Register(".json", jsonDecoder)

// v2 — RegistryBundle injection
bundle := registry.NewDefaultBundle()
bundle.DecoderRegistry().Register(".json", jsonDecoder)
cfg, _ := config.New(ctx, config.WithRegistryBundle(bundle))
```

### Direct Operations to Command Pipeline

```go
// v1 — Direct method calls
cfg.Set("key", "value")

// v2 — All mutations route through pipeline
cfg.Set(ctx, "key", "value")     // -> CorrelationID -> Tracing -> Metrics -> Audit -> Logging -> Recovery
cfg.BatchSet(ctx, map[string]any{"k1": "v1", "k2": "v2"})
result, _ := cfg.Reload(ctx)
```

---

## Feature Summary Table

| # | Feature | Key Types/Functions | v2 Highlight |
|---|---------|-------------------|--------------|
| 1 | Multi-Format File Loading | `loader.NewFileLoader`, `loader.NewEnvLoader`, `layer.NewStaticLayer` | Auto-detect by extension |
| 2 | Functional Options | `config.New(ctx, opts...)` | 15+ composable options |
| 3 | Struct Binding | `cfg.Bind(ctx, &target)` | Pipeline pass-through |
| 4 | Validation | `validator.New()`, custom tags | `duration`, `port`, `absurl`, `envprefix` |
| 5 | Layered Loading | `layer.WithPriority(N)` | Deterministic merge order |
| 6 | Event System | `cfg.Subscribe`, `cfg.WatchPattern` | 7 typed events, bounded dispatch |
| 7 | Typed Interceptors | `interceptor.SetFunc`, `interceptor.Chain` | Per-operation request/response |
| 8 | Schema Generation | `schema.New().Generate(struct{})` | JSON Schema from structs |
| 9 | Snapshot / Restore | `cfg.Snapshot()`, `cfg.Restore(data)` | Automatic secret redaction |
| 10 | Command Pipeline | Built-in middleware chain | 6 middleware layers |
| 11 | Explain | `cfg.Explain(key)` | Value, source, priority |
| 12 | Profiles | `profile.NewProfile`, `profile.NewRegistry` | Parent inheritance |
| 13 | Feature Flags | `featureflag.Engine`, `NewEvalContext` | Boolean, Percentage, Variant |
| 14 | Multi-Tenancy | `config.WithNamespace`, `cfg.SetNamespace` | Runtime namespace switching |
| 15 | Observability | `observability.NewCallbackRecorder` | Per-operation recording |
| 16 | Secure Storage | `secure.VaultStore`, `secure.KMSStore` | Pluggable + cached |
| 17 | Plugin System | `service.Plugin`, `PluginHost` | 5 host extension methods |
| 18 | Watcher / Debounce | `watcher.NewWatchManager`, `watcher.NewDebounce` | Coalesced file events |
| 19 | Value Types | `value.New`, `value.As[T]` | Type-safe, checksum, redaction |
| 20 | AppError | `apperrors.Build`, `WithSeverity` | 22 codes, 4 severity levels |
| 21 | Env Loader | `loader.NewEnvLoader` | Prefix stripping, priority |
| 22 | Decoder Registry | `decoder.NewDefaultRegistry` | JSON, YAML, TOML, Env |
| 23 | BatchSet | `cfg.BatchSet(ctx, map)` | Atomic multi-key set |
| 24 | Delta Reload | `config.WithDeltaReload(true)` | Checksum-based skip |
| 25 | Event Bus (Direct) | `eventbus.NewBus` | Worker pool, bounded queue |
| 26 | Registry Bundle | `registry.NewDefaultBundle` | DI container |
| 27 | Audit System | `domainevent.NewAuditEntry` | 7 audit actions |
| 28 | Backoff Strategy | `backoff.New`, `backoff.NewStopper` | Exponential + jitter |

---

## Running the Example

```bash
# Clone and run
go run main.go

# With environment variable overrides
APP_SERVER_PORT=9999 APP_APP_ENV=production go run main.go
```

The example runs fully self-contained using in-memory defaults. When `json_config.json`, `toml_config.toml`, and `yml_config.yml` are present in the same directory, they are loaded automatically at their respective priorities.

---

## License

See the [os-gomod/config](https://github.com/os-gomod/config) repository for license information.
