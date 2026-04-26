# Migration Guide: v1 → v2

## Overview

This is **not** a cosmetic version bump. The v2 release is a deep architectural
transformation of `os-gomod/config`:

- The monolithic `Config` god-object has been decomposed into four focused
  services behind a backward-compatible facade.
- All global registries have been eliminated in favour of explicit dependency
  injection.
- The untyped hook system has been replaced with compile-time-safe typed
  interceptors.
- Every mutating lifecycle operation (Set, Delete, Reload, Bind) now routes
  through a composable **Command Pipeline** that centralises tracing, metrics,
  audit, recovery, and correlation IDs — eliminating per-method orchestration
  duplication.
- The event bus has been rewritten with a bounded worker-pool model, replacing
  the goroutine-per-subscriber approach that could cause unbounded memory growth.
- Errors now carry structured metadata (codes, severity, correlation IDs,
  retryability) via the `AppError` interface.

Despite these internal changes, the **public API surface of the `Config` facade
is largely backward-compatible**: `Get`, `Set`, `Delete`, `BatchSet`, `Reload`,
`Bind`, `Subscribe`, `WatchPattern`, `Close`, and the options pattern all work
as before.

---

## Breaking Changes

### 1. Import Path Change

All imports must use the `/v2` suffix:

```go
// v1
import "github.com/os-gomod/config"

// v2
import "github.com/os-gomod/config/v2"
```

If you use any internal packages (not recommended), their paths have changed
too:

| v1                                     | v2                                                  |
|----------------------------------------|-----------------------------------------------------|
| `github.com/os-gomod/config/event`    | `github.com/os-gomod/config/v2/internal/domain/event` |
| `github.com/os-gomod/config/loader`   | `github.com/os-gomod/config/v2/internal/loader`      |
| `github.com/os-gomod/config/decoder`  | `github.com/os-gomod/config/v2/internal/decoder`     |
| `github.com/os-gomod/config/provider` | `github.com/os-gomod/config/v2/internal/provider`    |

### 2. Global Registries Removed

**v1** used package-level global registries that could be mutated from anywhere
in the program:

```go
// v1 — global state
loader.MustRegister("file", file.NewFactory)
decoder.MustRegister(yaml.NewDecoder())
```

**v2** requires explicit injection. Create a `registry.Bundle` and pass it
through the options pattern:

```go
// v2 — explicit dependency injection
bundle := registry.NewDefaultBundle()
bundle.Loader.Register("file", file.NewFactory)
bundle.Decoder.Register(yaml.NewDecoder())

cfg, err := config.New(ctx,
    config.WithRegistryBundle(bundle),
    config.WithLayers(myLayers...),
)
```

The `registry.NewDefaultBundle()` convenience function pre-loads standard
decoders (YAML, JSON, TOML, Env) and provides empty loader and provider
registries. If you don't pass a bundle, `config.New` creates one for you
automatically — so simple setups need no changes.

### 3. Hook System → Typed Interceptors

**v1** used untyped hook functions registered by string keys:

```go
// v1
hooksManager.Register(event.HookBeforeSet, hooks.New("auth", 0, func(ctx, hctx) error {
    // validate auth...
    return nil
}))
```

**v2** provides compile-time-safe interceptor interfaces with dedicated
request/response types per operation:

```go
// v2
type AuthInterceptor struct{}

func (i *AuthInterceptor) BeforeSet(ctx context.Context, req *interceptor.SetRequest) error {
    // req.Key, req.Value are fully typed
    if req.Key == "" {
        return errors.New("key is required")
    }
    return nil
}

func (i *AuthInterceptor) AfterSet(ctx context.Context, req *interceptor.SetRequest, res *interceptor.SetResponse) error {
    // res.Key, res.OldValue, res.NewValue, res.Created are fully typed
    log.Printf("key %q set, created=%v", res.Key, res.Created)
    return nil
}
```

The interceptor chain is injected via the `Config` constructor. For simple
cases, use functional adapters:

```go
// v2 — functional adapter (no struct required)
chain := interceptor.NewChain()
chain.AddSetInterceptor(&interceptor.SetFunc{
    BeforeFn: func(ctx context.Context, req *interceptor.SetRequest) error {
        if req.Key == "" {
            return errors.New("key is required")
        }
        return nil
    },
})
```

**Hook migration mapping:**

| v1 Hook                  | v2 Interceptor                                           |
|--------------------------|----------------------------------------------------------|
| `HookBeforeSet`          | `SetInterceptor.BeforeSet`                               |
| `HookAfterSet`           | `SetInterceptor.AfterSet`                                |
| `HookBeforeDelete`       | `DeleteInterceptor.BeforeDelete`                         |
| `HookAfterDelete`        | `DeleteInterceptor.AfterDelete`                          |
| `HookBeforeReload`       | `ReloadInterceptor.BeforeReload`                         |
| `HookAfterReload`        | `ReloadInterceptor.AfterReload`                          |
| `HookBeforeBind`         | `BindInterceptor.BeforeBind`                             |
| `HookAfterBind`          | `BindInterceptor.AfterBind`                              |
| `HookBeforeClose`        | `CloseInterceptor.BeforeClose`                           |
| `HookAfterClose`         | `CloseInterceptor.AfterClose`                            |

### 4. Error Handling

**v1** returned simple `errors.ConfigError` values.

**v2** returns the `errors.AppError` interface with rich metadata:

```go
// v2
type AppError interface {
    error
    Code() string           // e.g., "not_found", "type_mismatch"
    Message() string        // human-readable description
    Retryable() bool        // should the caller retry?
    Severity() Severity     // low, medium, high, critical
    CorrelationID() string  // for distributed tracing
    Operation() string      // which operation caused the error
    WithKey(key string) AppError
    WithOperation(op string) AppError
    WithSource(src string) AppError
    Wrap(cause error) AppError
}
```

Update your error handling:

```go
// v1
if err != nil {
    if cerr, ok := err.(*errors.ConfigError); ok {
        log.Printf("config error: %s", cerr)
    }
}

// v2
if err != nil {
    if appErr, ok := errors.AsAppError(err); ok {
        log.Printf("[%s] %s (op=%s, severity=%s, retryable=%v)",
            appErr.Code(), appErr.Message(),
            appErr.Operation(), appErr.Severity(), appErr.Retryable())
    }
}
```

Common error codes:

| Code                 | Meaning                          |
|----------------------|----------------------------------|
| `not_found`          | Key does not exist               |
| `type_mismatch`      | Value type does not match        |
| `validation_error`   | Validation failed                |
| `source_error`       | Layer/loader/provider failure    |
| `closed`             | Config has been shut down        |
| `pipeline_error`     | Pipeline execution failure       |
| `interceptor_error`  | Interceptor returned an error    |
| `queue_full`         | Event bus back-pressure drop     |
| `bus_closed`         | Event bus is shut down           |

### 5. Event Bus

**v1** used `event.Bus` with goroutine-per-subscriber delivery.

**v2** uses `eventbus.Bus` with a bounded worker pool and backpressure:

```go
// v1
bus := event.NewBus()
bus.Subscribe("config.changed", myObserver)

// v2
bus := eventbus.NewBus(
    eventbus.WithWorkerCount(32),
    eventbus.WithQueueSize(4096),
    eventbus.WithRetryCount(3),
)
bus.Subscribe("config.changed", myObserver)
```

Key differences:

- **Bounded concurrency**: fixed worker pool (default 32), not one goroutine per subscriber.
- **Backpressure**: when the queue is full, `Publish` returns `ErrQueueFull` instead of spawning unbounded goroutines.
- **Retry**: configurable per-subscriber retry with exponential backoff.
- **Pattern matching**: `Subscribe` accepts glob patterns (`"prefix.*"`, `"app.*.config"`).
- **Graceful shutdown**: `Close()` drains the queue and waits for all workers.

### 6. Config Struct Split

**v1** had a single `Config` struct with 15+ fields and responsibilities.

**v2** keeps the same `Config` facade but delegates internally to four bounded
services:

```
Config (public facade)
├── QueryService    — read-only: Get, GetAll, Has, Keys, Version, Snapshot, Explain
├── MutationService — writes:    Set, Delete, BatchSet, Restore
├── RuntimeService  — lifecycle: Reload, Bind, Close
└── PluginService   — plugins:   Register, Plugins
```

If you previously accessed internal fields directly, you must use the public
methods instead. The internal fields are no longer exported.

### 7. Pipeline-Based Orchestration

**v1** had per-method orchestration logic duplicated across each lifecycle
method (manual tracing spans, metric recording, audit logging, hook execution).

**v2** centralises all cross-cutting concerns in the Pipeline:

```go
// v2 — cross-cutting concerns are middleware, not per-method boilerplate
pipe := pipeline.New(
    pipeline.WithRecoveryMiddleware(),    // panic recovery
    pipeline.WithCorrelationIDMiddleware(), // distributed tracing IDs
    pipeline.WithTracingMiddleware(tracer), // OpenTelemetry spans
    pipeline.WithMetricsMiddleware(recorder), // Prometheus/OTel metrics
    pipeline.WithLoggingMiddleware(logger),   // structured logging
    pipeline.WithAuditMiddleware(auditRec),   // audit trail
)
```

Every command (Set, Delete, Reload, Bind) routes through the same pipeline.
Custom middleware can be added at construction time via options or later via
`pipe.Use()`.

---

## Non-Breaking Changes

The following public APIs work identically in v1 and v2:

- `config.New(ctx, opts...)` — same signature, same behaviour
- `cfg.Get(key)`, `cfg.GetAll()`, `cfg.Has(key)` — unchanged
- `cfg.Set(ctx, key, value)`, `cfg.Delete(ctx, key)`, `cfg.BatchSet(ctx, kv)` — unchanged
- `cfg.Reload(ctx)`, `cfg.Bind(ctx, target)` — unchanged
- `cfg.Subscribe(observer)` — unchanged (catch-all subscription)
- `cfg.WatchPattern(pattern, observer)` — new in v2, replaces manual pattern filtering
- `cfg.Close(ctx)` — unchanged
- Options pattern (`config.WithLayers`, `config.WithLogger`, etc.) — preserved and extended

---

## Step-by-Step Migration

### Step 1: Update Import Paths

```bash
# Find and replace all imports
find . -name '*.go' -exec sed -i 's|"github.com/os-gomod/config"|"github.com/os-gomod/config/v2"|g' {} +
```

### Step 2: Replace Global Registry Usage

If you used global registries, create a `Bundle`:

```go
// Before (v1)
loader.MustRegister("file", file.NewFactory)

// After (v2)
bundle := registry.NewDefaultBundle()
bundle.Loader.Register("file", file.NewFactory)
cfg, err := config.New(ctx,
    config.WithRegistryBundle(bundle),
    config.WithLayers(...),
)
```

If you used only the default registries, **no change is needed** —
`config.New` creates a `NewDefaultBundle()` automatically.

### Step 3: Replace Hooks with Typed Interceptors

```go
// Before (v1)
hooksManager.Register(event.HookBeforeSet, myHook)

// After (v2) — using functional adapter
chain := interceptor.NewChain()
chain.AddSetInterceptor(&interceptor.SetFunc{
    BeforeFn: func(ctx context.Context, req *interceptor.SetRequest) error {
        // your validation logic
        return nil
    },
})
```

Or for richer type safety, implement the interface:

```go
// After (v2) — typed struct
type MyValidator struct{}
func (v *MyValidator) BeforeSet(ctx context.Context, req *interceptor.SetRequest) error {
    if len(req.Key) == 0 {
        return errors.New("key must not be empty")
    }
    return nil
}
func (v *MyValidator) AfterSet(ctx context.Context, req *interceptor.SetRequest, res *interceptor.SetResponse) error {
    return nil
}
```

### Step 4: Update Error Handling

```go
// Before (v1)
if err != nil {
    log.Printf("config error: %v", err)
}

// After (v2)
if err != nil {
    if appErr, ok := errors.AsAppError(err); ok {
        log.Printf("[%s] %s (severity=%s, retryable=%v)",
            appErr.Code(), appErr.Message(), appErr.Severity(), appErr.Retryable())
    } else {
        log.Printf("unexpected error: %v", err)
    }
}
```

### Step 5: Configure Pipeline Middleware (Optional)

If you previously had custom tracing, metrics, or logging integrated with
each lifecycle method, migrate them to pipeline middleware:

```go
cfg, err := config.New(ctx,
    config.WithLogger(logger),
    config.WithTracer(tracer),
    config.WithRecorder(recorder),
    // These automatically register the corresponding middleware
)
```

### Step 6: Update Event Subscription Pattern Matching

```go
// Before (v1) — manual filtering in observer
cfg.Subscribe(func(ctx context.Context, evt event.Event) error {
    if strings.HasPrefix(evt.Key, "database.") {
        // handle
    }
    return nil
})

// After (v2) — use glob patterns
cfg.WatchPattern("database.*", func(ctx context.Context, evt event.Event) error {
    // handle — only receives matching events
    return nil
})
```

---

## Quick Compatibility Checklist

- [ ] Update `go.mod` to `github.com/os-gomod/config/v2`
- [ ] Update all import paths
- [ ] Replace `loader.MustRegister` / `decoder.MustRegister` with `bundle.Register`
- [ ] Replace `event.Hook*` with `interceptor.*Interceptor`
- [ ] Update error type assertions to use `errors.AsAppError()`
- [ ] If using `event.Bus` directly, switch to `eventbus.Bus`
- [ ] If accessing internal Config fields, switch to public methods
- [ ] Run `go vet ./...` and `go build ./...`
- [ ] Run your test suite
