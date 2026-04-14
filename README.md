# github.com/os-gomod/config

A production-grade configuration management library for Go with layered loading, real-time watching, schema validation, plugin system, and comprehensive observability.

## Features

### Core Capabilities
- **Layered Configuration** - Multiple sources with priority-based merging
- **Real-time Watching** - File polling, NATS KV, Consul KV, etcd v3, KMS, Vault
- **Struct Binding** - Type-safe binding with validation and default values
- **Event System** - Publish/Subscribe for configuration changes with pattern matching
- **Schema Generation** - JSON Schema generation from struct tags

### Resilience & Reliability
- **Circuit Breaker** - Prevents cascading failures with automatic recovery
- **Health Checking** - Built-in health checks for all sources
- **Fallback Data** - Uses last good data when source is unavailable
- **Graceful Degradation** - Continues operating when individual layers fail

### Security
- **Secure Storage** - AES-GCM encryption for sensitive values
- **Vault Integration** - HashiCorp Vault support (stub, ready for implementation)
- **KMS Integration** - Cloud KMS support (stub, ready for implementation)

### Validation
- **Custom Validators** - Built-in validators: `required_env`, `oneof_ci`, `duration`, `filepath`, `urlhttp`
- **Extensible** - Register custom validation tags via plugin system
- **Struct Validation** - go-playground/validator v10 integration

### Observability
- **Structured Logging** - slog integration
- **Prometheus Metrics** - Counter and histogram metrics via OpenTelemetry
- **OpenTelemetry Tracing** - Distributed tracing support
- **Atomic Metrics** - Zero-allocation metrics for high-performance scenarios

### Developer Experience
- **Builder Pattern** - Fluent API for configuration setup
- **Functional Options** - Alternative configuration approach
- **Profiles** - Pre-configured profiles for common scenarios
- **Snapshot/Restore** - Save and restore configuration state
- **Operations** - Atomic operations with rollback support

## Installation

```bash
go get github.com/os-gomod/config
```

## Quick Start

### Basic Usage with Builder

```go
package main

import (
    "context"
    "fmt"
    "github.com/os-gomod/config"
    "github.com/os-gomod/config/validator"
)

type AppConfig struct {
    App struct {
        Name    string `config:"name"    default:"myapp"  validate:"required"`
        Version string `config:"version" default:"1.0.0"  validate:"required"`
        Env     string `config:"env"     default:"dev"    validate:"oneof=dev staging prod"`
    } `config:"app"`
    Server struct {
        Addr string `config:"addr" default:":8080" validate:"required"`
        Port int    `config:"port" default:"8080" validate:"min=1,max=65535"`
    } `config:"server"`
}

func main() {
    ctx := context.Background()

    // Build configuration with multiple sources
    cfg, err := config.NewBuilder().
        WithContext(ctx).
        Memory(map[string]any{
            "app.name":    "my-service",
            "app.version": "2.0.0",
            "app.env":     "production",
            "server.port": 9090,
        }).
        Env("MYAPP_").
        File("config.yaml").
        Validate(validator.New()).
        Build()
    if err != nil {
        panic(err)
    }
    defer cfg.Close(ctx)

    // Bind to struct
    var appCfg AppConfig
    if err := cfg.Bind(ctx, &appCfg); err != nil {
        panic(err)
    }

    fmt.Printf("App: %s v%s (%s)\n", appCfg.App.Name, appCfg.App.Version, appCfg.App.Env)
    fmt.Printf("Server: %s:%d\n", appCfg.Server.Addr, appCfg.Server.Port)
}
```

### Layered Configuration with Priority

```go
// Layers with different priorities
base := loader.NewMemoryLoader(
    loader.WithMemoryData(map[string]any{
        "db.host": "localhost",
        "db.port": 5432,
    }),
    loader.WithMemoryPriority(10), // Lower priority
)

override := loader.NewMemoryLoader(
    loader.WithMemoryData(map[string]any{
        "db.host": "prod-db.internal",
    }),
    loader.WithMemoryPriority(30), // Higher priority wins
)

cfg, _ := config.New(ctx,
    config.WithLoader(base),
    config.WithLoader(override),
)

// db.host = "prod-db.internal" (from override)
// db.port = 5432 (only in base)
```

### Real-time Configuration Watching

```go
// File watching with polling
fileLoader := loader.NewFileLoader("config.yaml",
    loader.WithFilePollInterval(5*time.Second),
)

cfg, _ := config.New(ctx, config.WithLoader(fileLoader))

// Subscribe to all changes
unsubscribe := cfg.Subscribe(func(ctx context.Context, evt event.Event) error {
    fmt.Printf("Config changed: %s %s = %v\n", evt.Type, evt.Key, evt.NewValue.Raw())
    return nil
})
defer unsubscribe()

// Pattern-based subscription
cfg.OnChange("db.*", func(ctx context.Context, evt event.Event) error {
    fmt.Printf("Database config changed: %s\n", evt.Key)
    return nil
})
```

### Environment Variables with Custom Prefix

```go
// Load from environment variables with prefix
envLoader := loader.NewEnvLoader(
    loader.WithEnvPrefix("MYAPP"),
    loader.WithEnvPriority(40),
)

// MYAPP_DB_HOST becomes db.host
cfg, _ := config.New(ctx, config.WithLoader(envLoader))

// Custom key transformation
envLoader := loader.NewEnvLoader(
    loader.WithEnvKeyReplacer(func(key string) string {
        // Keep original case, don't transform
        return key
    }),
)
```

### Remote Configuration Sources

```go
// NATS KV Provider
natsProvider, _ := nats.New(nats.Config{
    URL:    "nats://localhost:4222",
    Bucket: "config-bucket",
    Priority: 50,
    PollInterval: 10 * time.Second,
})

// Consul KV Provider
consulProvider, _ := consul.New(consul.Config{
    Address: "localhost:8500",
    Prefix:  "myapp/config/",
    Priority: 50,
    PollInterval: 10 * time.Second,
})

// etcd v3 Provider
etcdProvider, _ := etcd.New(etcd.Config{
    Endpoints: []string{"localhost:2379"},
    Prefix:    "/myapp/config/",
    Priority:  50,
})

cfg, _ := config.New(ctx,
    config.WithProvider(natsProvider),
    config.WithProvider(consulProvider),
    config.WithProvider(etcdProvider),
)
```

### Secure Configuration Storage

```go
import "github.com/os-gomod/config/secure"

// Create encrypted store
key := make([]byte, 32) // Use proper key management in production
encryptor, _ := secure.NewAESGCMEncryptor(key)
store := secure.NewSecureStore(encryptor, secure.WithPriority(60))

// Store sensitive values
store.Set("database.password", []byte("s3cr3t!"))
store.Set("api.secret", []byte("secret-key"))

// Use as config source
secureSource := secure.NewSecureSource(store)
cfg, _ := config.New(ctx, config.WithLoader(secureSource))

// Values are automatically decrypted on access
password, _ := cfg.Get("database.password")
fmt.Println(password.Raw()) // "s3cr3t!"
```

### Schema Generation & Validation

```go
type Config struct {
    Server struct {
        Host string `config:"host" validate:"required"`
        Port int    `config:"port" validate:"min=1,max=65535"`
    } `config:"server"`
}

// Generate JSON Schema
gen := schema.New(
    schema.WithTitle("AppConfig"),
    schema.WithDescription("Application configuration"),
)
sch, _ := gen.Generate(Config{})

// Export schema
sch.WriteTo(os.Stdout)

// Validate configuration
validator := validator.New()
if err := validator.Validate(ctx, &cfg); err != nil {
    // Handle validation errors
}
```

### Atomic Operations with Rollback

```go
// Create atomic operation
setOp := config.NewSetOperation("key", "new-value")
delOp := config.NewDeleteOperation("temporary-key")

// Apply single operation
if err := config.ApplyOperation(ctx, cfg, setOp); err != nil {
    // Handle error
}

// Apply multiple operations with automatic rollback
ops := []config.Operation{
    config.NewSetOperation("a", 1),
    config.NewSetOperation("b", 2),
    config.NewBindOperation(&myStruct),
}

if err := config.ApplyOperations(ctx, cfg, ops); err != nil {
    // All successful operations are rolled back automatically
}
```

### Snapshot & Restore

```go
// Save current state
snapshot := cfg.Snapshot()

// Make changes
cfg.Set(ctx, "key", "new-value")
cfg.Delete(ctx, "other-key")

// Restore to previous state
cfg.Restore(snapshot)

// Check configuration provenance
fmt.Println(cfg.Explain("db.host"))
// Output: key "db.host": value=prod-db.internal, source=env, priority=40
```

### Plugin System

```go
type CustomValidatorPlugin struct{}

func (p CustomValidatorPlugin) Name() string { return "custom-validator" }

func (p CustomValidatorPlugin) Init(h plugin.Host) error {
    // Register custom validation tag
    return h.RegisterValidator("even", func(fl validator.FieldLevel) bool {
        return fl.Field().Int()%2 == 0
    })
}

// Use plugin
cfg, _ := config.NewBuilder().
    Memory(map[string]any{"port": 8080}).
    Plugin(&CustomValidatorPlugin{}).
    Build()
```

### Observability Integration

```go
// OpenTelemetry metrics
meter := otel.Meter("config")
tracer := otel.Tracer("config")
otelRecorder, _ := observability.NewOTelRecorder(meter, tracer)

// Atomic metrics (zero-allocation)
atomicMetrics := &observability.AtomicMetrics{}

cfg, _ := config.New(ctx,
    config.WithRecorder(otelRecorder),
    config.WithRecorder(atomicMetrics), // Chain multiple recorders
)

// Export metrics
metrics := atomicMetrics.Snapshot()
fmt.Printf("Reloads: %d, Sets: %d, Errors: %d\n",
    metrics["reloads"], metrics["sets"], metrics["errors"])
```

## Configuration Sources

| Source           | Description                     | Watch Support      |
| ---------------- | ------------------------------- | ------------------ |
| **File**         | YAML, JSON, TOML, ENV, INI, HCL | Polling            |
| **Environment**  | OS environment variables        | None               |
| **Memory**       | In-memory map                   | None               |
| **NATS KV**      | NATS JetStream key-value        | Watch / Polling    |
| **Consul KV**    | HashiCorp Consul                | Blocking / Polling |
| **etcd v3**      | etcd distributed KV             | Watch / Polling    |
| **Secure Store** | AES-GCM encrypted               | None               |
| **Vault**        | HashiCorp Vault (stub)          | None               |
| **KMS**          | Cloud KMS (stub)                | None               |

## Layer Priority

Configuration layers are merged by priority (higher priority wins):

```
Priority 100: Memory (default)
Priority 80:  File (default)
Priority 60:  Remote (NATS/Consul/etcd)
Priority 50:  Secure (Vault/KMS)
Priority 40:  Environment (default)
Priority 30:  Default values from struct tags
```

## Event Types

| Event        | Description               |
| ------------ | ------------------------- |
| `TypeCreate` | New key created           |
| `TypeUpdate` | Existing key updated      |
| `TypeDelete` | Key deleted               |
| `TypeReload` | Full configuration reload |
| `TypeError`  | Error occurred            |
| `TypeWatch`  | Watch event received      |

## Hook Points

| Hook               | Timing                |
| ------------------ | --------------------- |
| `HookBeforeReload` | Before full reload    |
| `HookAfterReload`  | After full reload     |
| `HookBeforeSet`    | Before setting a key  |
| `HookAfterSet`     | After setting a key   |
| `HookBeforeDelete` | Before deleting a key |
| `HookAfterDelete`  | After deleting a key  |
| `HookBeforeClose`  | Before closing        |
| `HookAfterClose`   | After closing         |


## Requirements

- Go 1.26.1 or higher
- Optional: NATS (for NATS KV provider)
- Optional: Consul (for Consul provider)
- Optional: etcd (for etcd provider)
- Optional: Prometheus/OpenTelemetry (for observability)

## License

MIT License

## Contributing

Contributions welcome! Please submit pull requests with:
- Comprehensive test coverage
- Updated documentation
- Adherence to existing code style

