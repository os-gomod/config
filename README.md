# README.md - Comprehensive Documentation

## Config - Multi-Layer Configuration Management for Go

[![Go Version](https://img.shields.io/badge/Go-1.26.1-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Config is a powerful, production-ready configuration management library for Go applications. It provides a layered configuration system with support for multiple sources, real-time updates, validation, event-driven architecture, and comprehensive observability.

---

## Table of Contents

1. [Overview](#overview)
2. [Core Features](#core-features)
3. [Quick Start](#quick-start)
4. [Architecture](#architecture)
5. [Installation](#installation)
6. [Configuration Sources](#configuration-sources)
7. [Layered Loading](#layered-loading)
8. [Struct Binding](#struct-binding)
9. [Validation](#validation)
10. [Events & Observability](#events--observability)
11. [Dynamic Updates](#dynamic-updates)
12. [Hooks System](#hooks-system)
13. [Profiles](#profiles)
14. [Plugins](#plugins)
15. [Snapshot & Restore](#snapshot--restore)
16. [Observability & Metrics](#observability--metrics)
17. [Error Handling](#error-handling)
18. [Operations & Transactions](#operations--transactions)
19. [Core Engine API](#core-engine-api)
20. [Schema Generation](#schema-generation)
21. [Secure Configuration](#secure-configuration)
22. [Remote Providers](#remote-providers)
23. [Best Practices](#best-practices)
24. [API Reference](#api-reference)

---

## Overview

Config is designed to handle complex configuration scenarios in modern distributed systems. It supports loading configuration from multiple sources (files, environment variables, remote KV stores), merging them with priority-based layering, providing real-time updates, and validating configuration against schemas.

### Key Capabilities

- **Multi-layer Configuration**: Combine configs from different sources with priority-based merging
- **Real-time Watching**: Automatic reload on configuration changes with debouncing
- **Event-Driven**: Subscribe to configuration changes with pattern matching
- **Validation**: Built-in validation with custom tags and schema generation
- **Observability**: Built-in metrics, tracing, and health checks
- **Plugin System**: Extensible architecture for custom loaders, providers, and validators
- **Remote Providers**: Support for Consul, etcd, NATS KV, and more
- **Secure Storage**: AES-GCM encryption for sensitive configuration data
- **Snapshot & Restore**: Versioned configuration snapshots with diff capabilities

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/os-gomod/config"
    "github.com/os-gomod/config/validator"
)

type AppConfig struct {
    App struct {
        Name    string `config:"name"    default:"myapp" validate:"required"`
        Version string `config:"version" default:"1.0.0"`
        Env     string `config:"env"     default:"development" validate:"oneof_ci=development staging production"`
    } `config:"app"`

    Server struct {
        Host string `config:"host" default:"0.0.0.0"`
        Port int    `config:"port" default:"8080" validate:"min=1,max=65535"`
    } `config:"server"`
}

func main() {
    // Create configuration with layered sources
    cfg, err := config.NewBuilder().
        WithContext(context.Background()).
        File("config.yaml").                   // Priority: 30
        Env("APP_").                           // Priority: 40 (higher priority)
        Validate(validator.New()).             // Enable validation
        Watch().                               // Enable hot reloading
        Build()
    if err != nil {
        log.Fatal(err)
    }
    defer cfg.Close(context.Background())

    // Bind to struct
    var appCfg AppConfig
    if err := cfg.Bind(context.Background(), &appCfg); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("App: %s v%s (%s)\n", appCfg.App.Name, appCfg.App.Version, appCfg.App.Env)
    fmt.Printf("Server: %s:%d\n", appCfg.Server.Host, appCfg.Server.Port)
}
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Application Code                               │
│                         (struct binding, direct access)                     │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Config (Public API)                            │
│  - Bind() / Get() / Set() / Delete() / Reload() / Snapshot() / Restore()    │
│  - OnChange() / Subscribe() / Explain() / Schema() / Validate()             │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
        ┌─────────────┬───────────────┼───────────────┬─────────────┐
        ▼             ▼               ▼               ▼             ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│   Events     │ │    Hooks     │ │   Watcher    │ │   Metrics    │ │   Plugins    │
│   (Bus)      │ │  (Manager)   │ │  (Manager)   │ │  (Recorder)  │ │ (Registry)   │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Core Engine                                    │
│  - State management (versioned)                                             │
│  - Layer priority merging                                                   │
│  - Circuit breakers for remote sources                                      │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
        ┌─────────────┬───────────────┼───────────────┬─────────────┐
        ▼             ▼               ▼               ▼             ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│     File     │ │     Env      │ │    Memory    │ │    Remote    │ │    Secure    │
│  (YAML/JSON/ │ │  (Variables) │ │   (Default)  │ │  (Consul/    │ │   (Vault/    │
│   TOML/HCL)  │ │              │ │              │ │   etcd/NATS) │ │    KMS)      │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
```

---

## Installation

```bash
go get github.com/os-gomod/config
```

For remote providers, use additional modules as needed:

```bash
go get github.com/os-gomod/config/provider/consul
go get github.com/os-gomod/config/provider/etcd
go get github.com/os-gomod/config/provider/nats
```

---

## Configuration Sources

### File Loader

Supports YAML, JSON, TOML, HCL, INI, and .env formats.

```go
import "github.com/os-gomod/config/loader"

// Basic usage
fileLoader := loader.NewFileLoader("config.yaml",
    loader.WithFilePriority(30),
)

// With custom decoder
jsonLoader := loader.NewFileLoader("config.json",
    loader.WithFileDecoder(decoder.NewJSONDecoder()),
    loader.WithFilePriority(30),
)

// With polling interval for watching
pollingLoader := loader.NewFileLoader("config.yaml",
    loader.WithFilePollInterval(5*time.Second),
)
```

### Environment Loader

```go
envLoader := loader.NewEnvLoader(
    loader.WithEnvPrefix("MYAPP"),
    loader.WithEnvPriority(40),
    loader.WithEnvKeyReplacer(func(key string) string {
        // Custom key transformation
        return strings.ToLower(strings.ReplaceAll(key, "_", "."))
    }),
)

// Environment variables like MYAPP_DB_HOST become db.host
```

### Memory Loader

```go
memoryLoader := loader.NewMemoryLoader(
    loader.WithMemoryData(map[string]any{
        "app.name": "myapp",
        "db.host":  "localhost",
    }),
    loader.WithMemoryPriority(20),
)

// Dynamic updates
memoryLoader.Update(map[string]any{
    "app.name": "updated-app",
})
```

### Custom Loader

```go
type CustomLoader struct {
    *loader.Base
}

func (c *CustomLoader) Load(ctx context.Context) (map[string]value.Value, error) {
    data := make(map[string]value.Value)
    // Custom loading logic
    data["custom.key"] = value.New("value", value.TypeString, value.SourceRemote, c.Priority())
    return data, nil
}

func (c *CustomLoader) Watch(ctx context.Context) (<-chan event.Event, error) {
    // Optional: implement watching
    return nil, nil
}
```

---

## Layered Loading

Layers are merged by priority (higher priority wins):

```go
cfg, err := config.NewBuilder().
    MemoryWithPriority(map[string]any{
        "app.name": "default-app",
        "db.host":  "localhost",
        "db.port":  5432,
    }, 10).                           // Priority 10 (lowest)
    FileWithPriority("config.yaml", 30). // Priority 30
    EnvWithPriority("APP_", 40).      // Priority 40 (highest)
    Build()
```

### Priority Rules

- Higher priority values override lower priority values
- Each layer contributes independently - missing keys from higher priority layers fall back to lower layers
- Keys from lower priority layers are retained unless overridden

### Layer Options

```go
import "github.com/os-gomod/config/core"

layer := core.NewLayer("my-layer",
    core.WithLayerPriority(50),
    core.WithLayerSource(loader),
    core.WithLayerTimeout(5*time.Second),
    core.WithLayerEnabled(true),
    core.WithLayerCircuitBreaker(circuit.BreakerConfig{
        Threshold:        5,
        Timeout:          30 * time.Second,
        SuccessThreshold: 2,
    }),
)
```

---

## Struct Binding

### Basic Binding

```go
type Config struct {
    App struct {
        Name    string `config:"name"    default:"myapp"`
        Version string `config:"version" default:"1.0.0"`
    } `config:"app"`

    Database struct {
        Host     string        `config:"host"     default:"localhost"`
        Port     int           `config:"port"     default:"5432"`
        Timeout  time.Duration `config:"timeout"  default:"30s"`
        Enabled  bool          `config:"enabled"  default:"true"`
    } `config:"database"`
}

var cfg Config
if err := configInstance.Bind(ctx, &cfg); err != nil {
    log.Fatal(err)
}
```

### Supported Types

- `string`, `int`, `int64`, `float64`, `bool`
- `time.Duration`, `time.Time`
- `[]string`, `[]int`, `[]any` (slices)
- `map[string]any`, `map[string]string`
- Nested structs
- Pointers to any of the above

### Custom Tag Name

```go
binder := binder.New(binder.WithTagName("custom"))
cfg := config.New(ctx, config.WithBinder(binder))
```

---

## Validation

### Built-in Validators

```go
import "github.com/os-gomod/config/validator"

v := validator.New()

type Config struct {
    Name     string `validate:"required"`
    Port     int    `validate:"min=1,max=65535"`
    Mode     string `validate:"oneof_ci=development staging production"`
    Timeout  string `validate:"duration"`
    URL      string `validate:"urlhttp"`
    FilePath string `validate:"filepath"`
}
```

### Custom Validation Tags

```go
v := validator.New(
    validator.WithCustomTag("even", func(fl validator.FieldLevel) bool {
        return fl.Field().Int()%2 == 0
    }),
    validator.WithCustomTag("username", func(fl validator.FieldLevel) bool {
        return len(fl.Field().String()) >= 3
    }),
)

// Use in struct
type MyConfig struct {
    EvenNumber int    `validate:"even"`
    Username   string `validate:"username"`
}
```

### Struct Level Validation

```go
v := validator.New(
    validator.WithStructLevel(func(sl validator.StructLevel) {
        cfg := sl.Current().Interface().(MyConfig)
        if cfg.StartPort > cfg.EndPort {
            sl.ReportError(cfg.EndPort, "EndPort", "end_port", "port_range", "")
        }
    }, MyConfig{}),
)
```

---

## Events & Observability

### Event Types

```go
const (
    TypeCreate  // New key created
    TypeUpdate  // Existing key updated
    TypeDelete  // Key deleted
    TypeReload  // Full configuration reload
    TypeError   // Error occurred
    TypeWatch   // Watch event triggered
)
```

### Subscribing to Events

```go
// Subscribe to all events
unsubscribe := cfg.Subscribe(func(ctx context.Context, evt event.Event) error {
    fmt.Printf("Event: %s key=%s\n", evt.Type, evt.Key)
    return nil
})
defer unsubscribe()

// Subscribe with pattern matching
unsubPattern := cfg.OnChange("database.*", func(ctx context.Context, evt event.Event) error {
    fmt.Printf("Database config changed: %s\n", evt.Key)
    return nil
})

// Pattern examples
cfg.OnChange("app.*", handler)      // All app.* keys
cfg.OnChange("db.*.host", handler)  // db.*.host pattern
cfg.OnChange("*", handler)          // All keys (same as Subscribe)
```

### Creating Custom Events

```go
evt := event.New(event.TypeUpdate, "custom.key",
    event.WithTraceID("trace-123"),
    event.WithLabel("region", "us-east-1"),
    event.WithMetadata("source", "manual"),
    event.WithError(nil),
)
```

---

## Dynamic Updates

### Setting Values

```go
// Single set
if err := cfg.Set(ctx, "app.name", "new-name"); err != nil {
    log.Printf("Failed to set: %v", err)
}

// Batch set
if err := cfg.BatchSet(ctx, map[string]any{
    "app.name":    "myapp",
    "app.version": "2.0.0",
    "db.host":     "prod-db",
}); err != nil {
    log.Printf("Batch set failed: %v", err)
}

// Delete
if err := cfg.Delete(ctx, "deprecated.key"); err != nil {
    log.Printf("Delete failed: %v", err)
}
```

### Reloading

```go
// Manual reload
result, err := cfg.Reload(ctx)
if err != nil {
    log.Printf("Reload failed: %v", err)
}
if result.HasErrors() {
    for _, layerErr := range result.LayerErrs {
        log.Printf("Layer %s error: %v", layerErr.Layer, layerErr.Err)
    }
}

// Automatic reload with debouncing (enabled via Watch())
cfg, _ := config.NewBuilder().
    Watch().
    WithDebounce(500 * time.Millisecond).
    Build()
```

---

## Hooks System

Hooks allow you to execute code at specific points in the configuration lifecycle.

### Hook Types

```go
const (
    HookBeforeReload
    HookAfterReload
    HookBeforeSet
    HookAfterSet
    HookBeforeDelete
    HookAfterDelete
    HookBeforeValidate
    HookAfterValidate
    HookBeforeClose
    HookAfterClose
)
```

### Implementing Hooks

```go
import "github.com/os-gomod/config/hooks"

// Hook function
auditHook := hooks.New("audit", 10, func(ctx context.Context, hctx *hooks.Context) error {
    log.Printf("[AUDIT] %s: key=%s, old=%v, new=%v",
        hctx.Operation, hctx.Key, hctx.OldValue, hctx.NewValue)
    return nil
})

// Register hook with config
cfg, _ := config.New(ctx,
    config.WithHook(HookAfterSet, auditHook),
    config.WithHook(HookBeforeReload, validationHook),
)
```

### Hook Context

```go
type Context struct {
    Operation   string          // "reload", "set", "delete", etc.
    Key         string          // Affected key
    Value       any             // New value
    OldValue    any             // Previous value
    NewValue    any             // New value (for updates)
    OldState    *value.State    // Previous full state
    NewState    *value.State    // New full state
    Metadata    map[string]any  // Custom metadata
    StartTime   time.Time       // Operation start time
    BatchValues map[string]any  // For batch operations
}
```

---

## Profiles

Profiles provide reusable configuration presets.

```go
import "github.com/os-gomod/config/profile"

// Memory profile
devProfile := profile.MemoryProfile("development", map[string]any{
    "app.env":     "development",
    "db.host":     "localhost",
    "log.level":   "debug",
}, 10)

// File profile
prodProfile := profile.FileProfile("production", "prod.yaml", 30)

// Env profile
overrideProfile := profile.EnvProfile("override", "OVERRIDE_", 40)

// Combine profiles
cfg, _ := config.NewBuilder().
    WithProfile(devProfile).
    WithProfile(prodProfile).
    WithProfile(overrideProfile).
    Build()
```

### Custom Profile

```go
customProfile := profile.New("custom",
    profile.LayerSpec{
        Name:     "custom-layer",
        Priority: 25,
        Source:   customLoader,
    },
    profile.LayerSpec{
        Name:     "fallback-layer",
        Priority: 10,
        Source:   fallbackLoader,
    },
)
```

---

## Plugins

The plugin system allows extending Config with custom loaders, providers, decoders, and validators.

### Plugin Interface

```go
type Plugin interface {
    Name() string
    Init(h Host) error
}

type Host interface {
    RegisterLoader(name string, f loader.Factory) error
    RegisterProvider(name string, f provider.Factory) error
    RegisterDecoder(d decoder.Decoder) error
    RegisterValidator(tag string, fn validator.Func) error
    Subscribe(obs event.Observer) func()
}
```

### Example Plugin

```go
type CustomPlugin struct{}

func (p *CustomPlugin) Name() string { return "custom-plugin" }

func (p *CustomPlugin) Init(h plugin.Host) error {
    // Register custom loader
    h.RegisterLoader("custom", func(cfg map[string]any) (loader.Loader, error) {
        return &CustomLoader{}, nil
    })

    // Register custom validator
    h.RegisterValidator("custom_tag", func(fl validator.FieldLevel) bool {
        return fl.Field().String() != ""
    })

    // Subscribe to events
    h.Subscribe(func(ctx context.Context, evt event.Event) error {
        fmt.Printf("Plugin received event: %s\n", evt.Type)
        return nil
    })

    return nil
}

// Use plugin
cfg, _ := config.New(ctx, config.WithPlugin(&CustomPlugin{}))
```

---

## Snapshot & Restore

```go
// Take a snapshot
snapshot := cfg.Snapshot()

// Modify config
cfg.Set(ctx, "app.name", "new-name")
cfg.Delete(ctx, "db.host")

// Restore from snapshot
cfg.Restore(snapshot)

// Compare snapshots
oldSnapshot := cfg.Snapshot()
cfg.Set(ctx, "app.version", "2.0.0")
diff := core.CompareSnapshots(oldSnapshot, cfg.Snapshot())
fmt.Printf("Changes: %s\n", diff.Summary()) // "1 addition, 1 modification"
```

### Core Snapshot Manager

```go
import "github.com/os-gomod/config/core/snapshot"

manager := snapshot.NewManager(100) // Keep last 100 snapshots

// Take versioned snapshot
s := manager.Take(engine.Version(), engine.GetAll(),
    snapshot.WithLabel("deployment-123"),
    snapshot.WithMetadata("user", "admin"),
)

// Retrieve snapshots
latest := manager.Latest()
byID := manager.Get(42)

// Create branch
branch := manager.CreateBranch("feature-x", s.ID())
branch.Append(snapshot.New(2, 2, newData))
```

---

## Observability & Metrics

### Atomic Metrics Recorder

```go
import "github.com/os-gomod/config/observability"

metrics := &observability.AtomicMetrics{}

cfg, _ := config.New(ctx,
    config.WithRecorder(metrics),
)

// Later, query metrics
stats := metrics.Snapshot()
fmt.Printf("Reloads: %d, Sets: %d, Errors: %d\n",
    stats["reloads"], stats["sets"], stats["errors"])
```

### OpenTelemetry Integration

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
    "github.com/os-gomod/config/observability"
)

meter := otel.Meter("myapp")
tracer := otel.Tracer("myapp")

otelRecorder, _ := observability.NewOTelRecorder(meter, tracer)

cfg, _ := config.New(ctx,
    config.WithRecorder(otelRecorder),
)
```

### Custom Recorder

```go
type MyRecorder struct{}

func (r *MyRecorder) RecordReload(ctx context.Context, dur time.Duration, keyCount int, err error) {
    // Custom metrics collection
}

func (r *MyRecorder) RecordSet(ctx context.Context, key string, dur time.Duration, err error) {
    // Custom logging
}

// Implement all Recorder interface methods...
```

---

## Error Handling

### Error Types

```go
import configerrors "github.com/os-gomod/config/errors"

// Built-in errors
configerrors.ErrClosed
configerrors.ErrNotFound
configerrors.ErrTypeMismatch
configerrors.ErrNotImplemented
configerrors.ErrPermission

// Error codes
const (
    CodeNotFound
    CodeValidation
    CodeSource
    CodeBind
    CodeTimeout
    // ... many more
)
```

### Working with Errors

```go
err := cfg.Set(ctx, "key", "value")
if err != nil {
    if configerrors.IsCode(err, configerrors.CodeNotFound) {
        fmt.Println("Key not found")
    }

    if ce, ok := err.(*configerrors.ConfigError); ok {
        fmt.Printf("Error: %s (code: %s, source: %s)\n",
            ce.Message, ce.Code, ce.Source)
        fmt.Printf("Stack trace:\n%s", ce.Stack())
    }
}

// Wrapping errors
err = configerrors.Wrap(originalErr, configerrors.CodeBind, "bind failed").
    WithKey("db.host").
    WithSource("yaml file").
    WithOperation("bind")
```

### Reload Warnings

```go
cfg, err := config.New(ctx, opts...)
if err != nil {
    if warning, ok := err.(*config.ReloadWarning); ok {
        fmt.Printf("Reload completed with %d warnings\n", len(warning.LayerErrors))
        for _, layerErr := range warning.LayerErrors {
            fmt.Printf("  Layer %s: %v\n", layerErr.Layer, layerErr.Err)
        }
        // Continue - config is still usable
    } else {
        // Fatal error
        panic(err)
    }
}
```

---

## Operations & Transactions

Operations allow atomic, rollbackable configuration changes.

```go
import "github.com/os-gomod/config"

// Create an operation
setOp := config.NewSetOperation("app.name", "new-name")
deleteOp := config.NewDeleteOperation("deprecated.key")
bindOp := config.NewBindOperation(&myConfig{})

// Apply single operation
if err := config.ApplyOperation(ctx, cfg, setOp); err != nil {
    log.Printf("Operation failed: %v", err)
}

// Apply multiple operations with rollback
ops := []config.Operation{
    config.NewSetOperation("db.host", "new-host"),
    config.NewSetOperation("db.port", 5433),
    config.NewSetOperation("app.version", "2.0.0"),
}

if err := config.ApplyOperations(ctx, cfg, ops); err != nil {
    // All successful operations are automatically rolled back
    log.Printf("Batch failed, rolled back: %v", err)
}
```

### Custom Operation

```go
customOp := config.Operation{
    Name: "migration",
    Apply: func(ctx context.Context, c *config.Config) error {
        // Apply changes
        return c.Set(ctx, "migration.version", "2")
    },
    Rollback: func(ctx context.Context, c *config.Config) error {
        // Rollback changes
        return c.Delete(ctx, "migration.version")
    },
}
```

---

## Core Engine API

For advanced use cases, you can work directly with the core engine.

```go
import "github.com/os-gomod/config/core"

// Create engine
engine := core.New(
    core.WithMaxWorkers(8),
    core.WithLayer(layer1),
    core.WithLayer(layer2),
)

// Basic operations
engine.Set(ctx, "key", "value")
engine.BatchSet(ctx, map[string]any{"a": 1, "b": 2})
engine.Delete(ctx, "key")

// Get values
val, ok := engine.Get("key")
all := engine.GetAll()
keys := engine.Keys()
has := engine.Has("key")
len := engine.Len()
version := engine.Version()

// Reload all layers
result, err := engine.Reload(ctx)
for _, evt := range result.Events {
    fmt.Printf("Event: %s\n", evt.Type)
}

// State management
state := engine.State()
engine.SetState(newData)

// Layer management
engine.AddLayer(newLayer)
layers := engine.Layers()

// Close
engine.Close(ctx)
```

### Circuit Breakers

```go
import "github.com/os-gomod/config/core/circuit"

breaker := circuit.New(circuit.BreakerConfig{
    Threshold:        5,              // Failures before opening
    Timeout:          30 * time.Second, // Time before half-open
    SuccessThreshold: 2,              // Successes to close
})

// Use with layer
layer := core.NewLayer("remote",
    core.WithLayerCircuitBreaker(circuit.DefaultConfig()),
    core.WithLayerSource(remoteProvider),
)

// Check layer health
if layer.IsHealthy() {
    // Layer is operational
}
status := layer.HealthStatus()
fmt.Printf("Healthy: %v, Failures: %d\n", status.Healthy, status.ConsecutiveFails)
```

---

## Schema Generation

Generate JSON Schema from your configuration structs.

```go
import "github.com/os-gomod/config/schema"

type MyConfig struct {
    App struct {
        Name    string `json:"name"    description:"Application name" validate:"required"`
        Version string `json:"version" default:"1.0.0"`
        Port    int    `json:"port"    validate:"min=1,max=65535"`
    } `json:"app"`
}

gen := schema.New(
    schema.WithTitle("My App Configuration"),
    schema.WithDescription("Complete configuration schema for My App"),
)

sch, err := gen.Generate(MyConfig{})
if err != nil {
    log.Fatal(err)
}

// Write to file
f, _ := os.Create("schema.json")
defer f.Close()
sch.WriteTo(f)

// Or marshal to JSON
data, _ := json.MarshalIndent(sch, "", "  ")
```

### Schema Tags

| Tag | Description | Example |
|-----|-------------|---------|
| `default` | Default value | `default:"localhost"` |
| `description` | Field description | `description:"Database hostname"` |
| `validate` | Validation rules | `validate:"required,min=1"` |
| `json` | JSON field name | `json:"db_host"` |

---

## Secure Configuration

### AES-GCM Encrypted Store

```go
import "github.com/os-gomod/config/secure"

// Generate or load encryption key (32 bytes for AES-256)
key := []byte("32-byte-secret-key-here-1234567890")

// Create encryptor
encryptor, err := secure.NewAESGCMEncryptor(key)
if err != nil {
    log.Fatal(err)
}

// Create secure store
store := secure.NewStore(encryptor,
    secure.WithPriority(60),
    secure.WithLogger(slog.Default()),
)

// Store secrets
store.Set("database.password", []byte("s3cr3t!"))
store.Set("api.key", []byte("sk-1234567890"))

// Retrieve secrets
password, _ := store.Get("database.password")
fmt.Printf("Password: %s\n", password) // s3cr3t!

// Use as config source
secureSource := secure.NewSource(store, secure.WithPriority(60))
cfg, _ := config.New(ctx, config.WithLoader(secureSource))
```

### Vault Provider (Stub - Extend for production)

```go
vaultProvider := secure.NewVaultProvider(secure.VaultConfig{
    Address:   "https://vault.example.com:8200",
    Token:     "hvs.xxx",
    MountPath: "secret",
    Namespace: "myapp",
})

cfg, _ := config.New(ctx, config.WithProvider(vaultProvider))
```

---

## Remote Providers

### Consul

```go
import "github.com/os-gomod/config/provider/consul"

consulProvider, err := consul.New(&consul.Config{
    Address:      "localhost:8500",
    Datacenter:   "dc1",
    Token:        "your-token",
    Prefix:       "myapp/config/",
    Priority:     50,
    PollInterval: 5 * time.Second, // Optional: poll instead of watch
    Timeout:      5 * time.Second,
})

cfg, _ := config.New(ctx,
    config.WithProvider(consulProvider),
    config.WithLoader(loader.NewFileLoader("base.yaml")), // Fallback
)
```

### etcd

```go
import "github.com/os-gomod/config/provider/etcd"

etcdProvider, err := etcd.New(&etcd.Config{
    Endpoints:    []string{"localhost:2379"},
    Username:     "user",
    Password:     "pass",
    Prefix:       "/myapp/config/",
    Priority:     50,
    PollInterval: 0, // Use watch mode (0 = watch, >0 = poll)
    Timeout:      5 * time.Second,
})
```

### NATS KV

```go
import "github.com/os-gomod/config/provider/nats"

natsProvider, err := nats.New(nats.Config{
    URL:          "nats://localhost:4222",
    Bucket:       "myapp_config",
    Priority:     50,
    PollInterval: 10 * time.Second,
    Timeout:      5 * time.Second,
})
```

---

## Best Practices

### 1. Configuration Structure

```go
// Good: Organized, typed configuration
type Config struct {
    Server   ServerConfig   `config:"server"`
    Database DatabaseConfig `config:"database"`
    Redis    RedisConfig    `config:"redis"`
}

// Bad: Flat, untyped configuration
type Config struct {
    ServerHost string
    ServerPort int
    DBHost     string
    DBPort     int
}
```

### 2. Validation

```go
// Always validate configuration on startup
cfg, err := config.NewBuilder().
    Validate(validator.New()).
    Build()
if err != nil {
    // Handle validation errors
}

// Use meaningful validation tags
type Config struct {
    Port     int    `validate:"min=1024,max=65535"`  // Specific range
    Mode     string `validate:"oneof_ci=dev staging prod"` // Explicit values
    Timeout  string `validate:"duration"`            // Format validation
}
```

### 3. Sensible Defaults

```go
type Config struct {
    Host    string `config:"host"    default:"localhost"`
    Port    int    `config:"port"    default:"8080"`
    Timeout int    `config:"timeout" default:"30"` // seconds
    Retries int    `config:"retries" default:"3"`
}
```

### 4. Layer Strategy

- **Lowest priority (10-20)**: Default/memory values
- **Medium priority (30-40)**: File-based configuration
- **Higher priority (50-60)**: Environment variables
- **Highest priority (70-100)**: Remote/override sources

### 5. Error Handling

```go
// Handle reload warnings gracefully
cfg, err := config.New(ctx, opts...)
if err != nil {
    if warning, ok := err.(*config.ReloadWarning); ok {
        log.Printf("Warning: %v", warning)
        // Continue - config is usable
    } else {
        log.Fatal(err)
    }
}
```

### 6. Graceful Shutdown

```go
// Always close config to clean up resources
defer cfg.Close(context.Background())

// Or with signal handling
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigChan
    cfg.Close(context.Background())
    os.Exit(0)
}()
```

---

## API Reference

### Config API

| Method | Description |
|--------|-------------|
| `New(ctx, opts...)` | Create new config instance |
| `MustNew(ctx, opts...)` | Panic on error |
| `Reload(ctx)` | Reload all layers |
| `Set(ctx, key, value)` | Set a configuration value |
| `BatchSet(ctx, kv)` | Set multiple values |
| `Delete(ctx, key)` | Delete a key |
| `Get(key)` | Get value by key |
| `Has(key)` | Check if key exists |
| `Keys()` | Get all keys |
| `Bind(ctx, target)` | Bind to struct |
| `Snapshot()` | Take configuration snapshot |
| `Restore(data)` | Restore from snapshot |
| `OnChange(pattern, obs)` | Subscribe with pattern |
| `Subscribe(obs)` | Subscribe to all events |
| `Explain(key)` | Get key provenance |
| `Schema(v)` | Generate JSON Schema |
| `Validate(ctx, target)` | Validate struct |
| `Plugins()` | List registered plugins |
| `Close(ctx)` | Close config |

### Builder API

| Method | Description |
|--------|-------------|
| `NewBuilder()` | Create new builder |
| `WithContext(ctx)` | Set context |
| `File(path)` | Add file loader (priority 30) |
| `FileWithPriority(path, p)` | Add file with priority |
| `Env(prefix)` | Add env loader (priority 40) |
| `EnvWithPriority(prefix, p)` | Add env with priority |
| `Memory(data)` | Add memory loader (priority 20) |
| `MemoryWithPriority(data, p)` | Add memory with priority |
| `Remote(name, cfg)` | Add remote provider |
| `Watch()` | Enable watching |
| `Validate(v)` | Set validator |
| `StrictReload()` | Fail on any layer error |
| `OnReloadError(fn)` | Set reload error handler |
| `Recorder(r)` | Set metrics recorder |
| `Plugin(p)` | Add plugin |
| `Build()` | Build config |
| `MustBuild()` | Build or panic |
| `Bind(ctx, target)` | Build and bind |
| `MustBind(ctx, target)` | Build and bind or panic |

### Options

| Option | Description |
|--------|-------------|
| `WithLayer(layer)` | Add core layer |
| `WithLoader(loader)` | Add loader as layer |
| `WithProvider(provider)` | Add provider as layer |
| `WithValidator(v)` | Set validator |
| `WithReloadErrorHandler(fn)` | Set error handler |
| `WithStrictReload()` | Enable strict mode |
| `WithRecorder(r)` | Set metrics recorder |
| `WithPlugin(p)` | Add plugin |
| `WithMaxWorkers(n)` | Set concurrency |
| `WithDebounce(d)` | Set debounce duration |

---

## License

MIT License - see LICENSE file for details.

---

**Maintained by [os-gomod](https://github.com/os-gomod)** | [Report Bug](https://github.com/os-gomod/config/issues) | [Request Feature](https://github.com/os-gomod/config/issues)
