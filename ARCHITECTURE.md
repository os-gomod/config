# Architecture

## Design Principles

### 1. Centralise Orchestration — Pipeline Pattern
Every mutating lifecycle operation (Set, Delete, Reload, Bind) routes through
a single **Command Pipeline**. Cross-cutting concerns — tracing, metrics, audit,
recovery, correlation IDs — are middleware, not per-method boilerplate.
This eliminates the orchestration duplication that made v1's `Config` struct
difficult to maintain.

### 2. Isolate Domains — Clean Architecture Boundaries
Internal packages are organised by domain, not by technical layer. The `value`,
`event`, `errors`, and `layer` packages form a stable domain core with zero
external dependencies. Infrastructure packages (`eventbus`, `pipeline`,
`loader`, `provider`, `binder`) depend on the domain, never the reverse.

### 3. Eliminate Globals — Dependency Injection
All registries (loader, decoder, provider) are instance-based. The
`registry.Bundle` holds them and is injected via the options pattern. There
are no `var Default*` package-level variables. This makes the library safe for
testing, multi-tenant use, and concurrent initialisation.

### 4. Make Concurrency Explicit — Bounded Worker Pools
The event bus uses a fixed-size worker pool (default 32) processing events
from a bounded queue (default 4096). This replaces the v1 goroutine-per-subscriber
model that could cause unbounded memory growth under load. When the queue is
full, publishers receive a backpressure signal (`ErrQueueFull`).

### 5. Explicit Extension Points — Typed Interceptors
Each lifecycle operation has a dedicated interceptor interface with typed
request/response structs. Interceptors are chain-ordered, concurrency-safe,
and support both struct and functional adapter patterns. This replaces the
untyped string-keyed hook system from v1.

---

## Package Structure

```
github.com/os-gomod/config/v2
│
├── config.go                  # Public facade — delegates to services
├── options.go                 # Public options for Config construction
│
└── internal/
    │
    ├── domain/                # Pure domain types (zero external deps)
    │   ├── value/             # Value, Type, Source, State, Merge, Diff, Checksum
    │   ├── event/             # Event, EventType, Observer, AuditEntry, Redaction
    │   ├── errors/            # AppError interface, error codes, Severity, Builder
    │   ├── layer/             # Layer, CircuitBreaker, LayerError
    │   └── featureflag/       # Feature flag types
    │
    ├── eventbus/              # Bounded worker-pool event bus
    │   ├── bus.go             # Bus — publish, subscribe, pattern matching
    │   ├── worker.go          # Worker — retry, panic recovery
    │   └── options.go         # Bus options
    │
    ├── pipeline/              # Command Pipeline — the central orchestration
    │   ├── pipeline.go        # Pipeline, Command, Result, Handler, Middleware
    │   ├── middleware.go       # Built-in middleware (Tracing, Metrics, Audit, Logging, Recovery)
    │   └── options.go         # Pipeline options
    │
    ├── service/               # Bounded services behind the facade
    │   ├── engine.go          # Engine interface + CoreEngine implementation
    │   ├── query.go           # QueryService — read-only access
    │   ├── mutation.go        # MutationService — Set/Delete/BatchSet via pipeline
    │   ├── runtime.go         # RuntimeService — Reload/Bind/Close via pipeline
    │   └── plugin.go          # PluginService — plugin registration and lifecycle
    │
    ├── interceptor/           # Typed interceptors for lifecycle hooks
    │   ├── interceptor.go     # Set, Delete, Reload, Bind, Close interceptor interfaces
    │   ├── chain.go           # Chain — ordered execution with RWMutex
    │   └── funcs.go           # Functional adapters (SetFunc, DeleteFunc, etc.)
    │
    ├── registry/              # Dependency injection
    │   └── bundle.go          # Bundle — Loader, Decoder, Provider registries
    │
    ├── loader/                # Config source loaders
    │   ├── loader.go          # Loader interface
    │   ├── file.go            # File system loader (YAML, JSON, TOML, HCL, etc.)
    │   ├── env.go             # Environment variable loader
    │   ├── memory.go          # In-memory loader (for testing)
    │   ├── registry.go        # Loader registry
    │   └── errors.go          # Loader-specific errors
    │
    ├── decoder/               # Content decoders
    │   ├── decoder.go         # Decoder interface
    │   ├── decoders.go        # YAML, JSON, TOML, HCL, Properties decoders
    │   └── env.go             # Environment variable decoder
    │
    ├── provider/              # Remote config providers
    │   ├── provider.go        # Provider interface + registry
    │   ├── consul/            # HashiCorp Consul provider
    │   ├── etcd/              # etcd provider
    │   └── nats/              # NATS provider
    │
    ├── binder/                # Struct binding
    │   ├── binder.go          # Binder interface
    │   ├── cache.go           # Binding cache
    │   └── coerce.go          # Type coercion helpers
    │
    ├── validator/             # Validation
    │   ├── validator.go       # Validator interface
    │   └── tags.go            # Struct tag parsing
    │
    ├── schema/                # JSON Schema generation
    │   ├── schema.go          # Schema types
    │   └── generator.go       # Go type → JSON Schema converter
    │
    ├── pattern/               # Glob-style pattern matching
    │   └── pattern.go         # Match(key, pattern) — dot-segment wildcards
    │
    ├── watcher/               # File/source watching
    │   ├── watcher.go         # Watcher interface
    │   ├── debounce.go        # Debounce logic
    │   └── manager.go         # Watch manager
    │
    ├── secure/                # Secrets management
    │   ├── store.go           # Secret store interface
    │   ├── vault.go           # HashiCorp Vault integration
    │   └── kms.go             # KMS integration
    │
    ├── observability/         # Metrics, traces, audit
    │   ├── recorder.go        # Recorder interface, Nop, MultiRecorder, FuncRecorder
    │   ├── metrics.go         # Metrics helpers
    │   └── trace.go           # Trace helpers
    │
    ├── backoff/               # Retry strategies
    │   └── backoff.go         # Exponential backoff
    │
    └── profile/               # Configuration profiles
        ├── profile.go         # Profile types
        └── options.go         # Profile options
```

---

## Command Pipeline

The pipeline is the central innovation of v2. Every mutating lifecycle
operation is represented as a `Command` value object and executed through a
composable middleware chain.

```
                    ┌─────────────────────┐
                    │    Config.Set()     │  (public API)
                    └─────────┬───────────┘
                              │
                    ┌─────────▼───────────┐
                    │  BeforeSet          │  (typed interceptor)
                    │  interceptor chain  │
                    └─────────┬───────────┘
                              │
              ┌───────────────▼───────────────┐
              │        PIPELINE               │
              │                               │
              │  ┌─────────────────────────┐  │
              │  │ RecoveryMiddleware      │  │  ← panic recovery
              │  └───────────┬─────────────┘  │
              │  ┌───────────▼─────────────┐  │
              │  │ CorrelationIDMiddleware │  │  ← distributed tracing ID
              │  └───────────┬─────────────┘  │
              │  ┌───────────▼─────────────┐  │
              │  │ LoggingMiddleware       │  │  ← structured log entry
              │  └───────────┬─────────────┘  │
              │  ┌───────────▼─────────────┐  │
              │  │ TracingMiddleware       │  │  ← OpenTelemetry span
              │  └───────────┬─────────────┘  │
              │  ┌───────────▼─────────────┐  │
              │  │ MetricsMiddleware       │  │  ← Prometheus/OTel metric
              │  └───────────┬─────────────┘  │
              │  ┌───────────▼─────────────┐  │
              │  │ AuditMiddleware         │  │  ← audit trail (mutating ops)
              │  └───────────┬─────────────┘  │
              │  ┌───────────▼─────────────┐  │
              │  │ Executor (cmd.Execute)  │  │  ← actual work
              │  └─────────────────────────┘  │
              │                               │
              └───────────────┬───────────────┘
                              │
                    ┌─────────▼───────────┐
                    │  bus.Publish()      │  (emit events)
                    └─────────┬───────────┘
                              │
                    ┌─────────▼───────────┐
                    │  AfterSet           │  (typed interceptor)
                    │  interceptor chain  │
                    └─────────────────────┘
```

### Command Structure

```go
type Command struct {
    Name      string                   // human-readable label
    Operation string                   // "set", "delete", "reload", "bind", "batch_set"
    Key       string                   // target key (for single-key ops)
    Value     any                      // value to set
    Values    map[string]any           // values for batch_set
    Execute   func(ctx) (Result, error) // actual implementation
}

type Result struct {
    Events     []event.Event  // domain events generated
    Duration   time.Duration  // execution time
    Skipped    bool           // short-circuited by middleware?
    SkipReason string         // why it was skipped
}
```

### Middleware Interface

```go
type Middleware func(next Handler) Handler
type Handler   func(ctx context.Context, cmd Command) (Result, error)
```

Middleware wraps the next handler, performing pre/post processing. Middleware
is composed left-to-right: the first registered is the outermost wrapper.

### Built-in Middleware

| Middleware               | Purpose                                        |
|--------------------------|-------------------------------------------------|
| `RecoveryMiddleware`     | Recovers panics → `AppError` (SeverityCritical) |
| `CorrelationIDMiddleware`| Ensures a correlation ID exists in context       |
| `LoggingMiddleware`      | Structured `slog` entry/exit logging            |
| `TracingMiddleware`      | OpenTelemetry span per command                  |
| `MetricsMiddleware`      | Records operation duration and success/failure  |
| `AuditMiddleware`        | Records audit entries for mutating operations   |

---

## Service Boundaries

```
┌──────────────────────────────────────────────────────┐
│                     Config (facade)                  │
├──────────────┬───────────────┬──────────┬────────────┤
│ QueryService │MutationService│RuntimeSvc│PluginSvc   │
│              │               │          │            │
│ Get()        │ Set()         │ Reload() │ Register() │
│ GetAll()     │ Delete()      │ Bind()   │ Plugins()  │
│ Has()        │ BatchSet()    │ Close()  │            │
│ Keys()       │ Validate()    │          │            │
│ Version()    │ Restore()     │          │            │
│ Len()        │ SetNamespace()│          │            │
│ Snapshot()   │ Namespace()   │          │            │
│ Explain()    │               │          │            │
│ Schema()     │               │          │            │
│ Bus()        │               │          │            │
└──────┬───────┴───────┬───────┴────┬─────┴─────┬──────┘
       │               │            │           │
       │          ┌────▼────┐       │     ┌─────▼─────┐
       │          │Pipeline │       │     │ Registry  │
       │          └────┬────┘       │     │  Bundle   │
       │               │            │     │           │
       └───────┬───────┘            │     │  Loader   │
               │                    │     │  Decoder  │
        ┌──────▼──────┐             │     │  Provider │
        │ CoreEngine  │             │     └───────────┘
        │             │             │
        │ • State     │             │
        │ • Layers    │             │
        │ • Version   │             │
        │ • Merge     │             │
        └─────────────┘             │
                                    │
                           ┌────────▼────────┐
                           │  EventBus       │
                           │  (worker pool)  │
                           └─────────────────┘
```

### QueryService
Read-only access to configuration state. Does **not** route through the pipeline
because reads are side-effect-free and require no tracing, metrics, or audit.

### MutationService
Handles all config mutations (Set, Delete, BatchSet). Every operation:
1. Runs `BeforeSet` / `BeforeDelete` interceptors
2. Routes through the pipeline (tracing, metrics, audit)
3. Executes the actual mutation on the engine
4. Publishes domain events to the event bus
5. Runs `AfterSet` / `AfterDelete` interceptors

### RuntimeService
Manages lifecycle operations (Reload, Bind, Close). Same interceptor → pipeline →
execute → publish → interceptor flow as MutationService.

### PluginService
Manages plugin registration and lifecycle. Plugins receive a `PluginHost`
interface that exposes scoped registration capabilities (add loaders, providers,
decoders, subscribe to events).

---

## Event Bus

### Worker Pool Model

```
                    Publishers
                        │
                  ┌─────▼──────┐
                  │   Bus      │
                  │  Publish() │
                  └─────┬──────┘
                        │ matched(key)
                        │
                  ┌─────▼──────────┐
                  │  dispatchJob   │  {event, subscribers[]}
                  └─────┬──────────┘
                        │
                  ┌─────▼──────────┐
                  │  Queue (cap)   │  bounded channel
                  │  default: 4096 │
                  └─────┬──────────┘
                        │
        ┌───────────────┼───────────────┐
        │               │               │
   ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
   │ Worker 0│    │ Worker 1│ ...│ Worker N│    (default N=32)
   │         │    │         │    │         │
   │ deliver │    │ deliver │    │ deliver │
   │ + retry │    │ + retry │    │ + retry │
   └────┬────┘    └────┬────┘    └────┬────┘
        │               │               │
   ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
   │  obs 1  │    │  obs K  │    │  obs M  │
   └─────────┘    └─────────┘    └─────────┘
```

### Backpressure
When the queue is full, `Publish` returns `ErrQueueFull` immediately.
Publishers must decide how to handle this (retry, drop, buffer).

### Retry
Each worker retries failed deliveries up to `RetryCount` times with exponential
backoff (`base * 2^(attempt-1)`). Default: no retries.

### Panic Safety
Every observer invocation is wrapped in `recover()`. Panics are converted
to delivery errors and forwarded to the `PanicHandler`.

### Pattern Matching
Subscribers register with glob patterns using dot-separated segments:

| Pattern          | Matches                              |
|------------------|--------------------------------------|
| `""` or `"*"`    | All events (catch-all)               |
| `"database.host"`| Exact match only                     |
| `"database.*"`   | `database.host`, `database.port`, … |
| `"app.*.config"` | `app.db.config`, `app.cache.config`  |
| `"*.changed"`    | `config.changed`, `secrets.changed`  |

---

## Error Contracts

### AppError Interface

Every error returned by library operations implements `AppError`:

```go
type AppError interface {
    error
    Code() string           // Machine-readable: "not_found", "closed", …
    Message() string        // Human-readable description
    Retryable() bool        // Should the caller retry?
    Severity() Severity     // low, medium, high, critical
    CorrelationID() string  // 16-byte hex ID for tracing
    Operation() string      // "set", "reload", "bind", …
    WithKey(key string) AppError
    WithOperation(op string) AppError
    WithSource(src string) AppError
    Wrap(cause error) AppError
}
```

### Severity Levels

| Level     | Meaning                                     |
|-----------|---------------------------------------------|
| `Low`     | Informational, non-blocking                  |
| `Medium`  | Degraded behaviour (default)                 |
| `High`    | Significant failure, partial degradation     |
| `Critical`| Data loss or service unavailability          |

### Error Codes

| Code                | When                                         |
|---------------------|----------------------------------------------|
| `not_found`         | Key does not exist                           |
| `type_mismatch`     | Value type incompatible                      |
| `validation_error`  | Struct validation failed                     |
| `source_error`      | Layer, loader, or provider failure           |
| `closed`            | Config or bus has been shut down             |
| `pipeline_error`    | Pipeline execution panicked or failed        |
| `interceptor_error` | Interceptor returned an error                |
| `queue_full`        | Event bus queue capacity exceeded            |
| `bus_closed`        | Event bus is shut down                       |
| `delivery_failed`   | All delivery retry attempts exhausted        |
| `invalid_config`    | Invalid key or configuration                 |
| `already_exists`    | Duplicate registration                       |
| `not_implemented`   | Feature not yet available                    |
| `crypto_error`      | Encryption/decryption failure                |
| `watch_error`       | File watch failure                          |
| `bind_error`        | Struct binding failure                       |
| `connection_error`  | Network connectivity issue                   |

### Builder Pattern

```go
err := errors.Build(
    errors.CodeNotFound,
    "key not found",
    errors.WithSeverity(errors.SeverityLow),
    errors.WithRetryable(true),
    errors.WithOperation("get"),
).WithKey("database.host")
```

### Extraction

```go
if appErr, ok := errors.AsAppError(err); ok {
    log.Printf("[%s] %s", appErr.Code(), appErr.Message())
}
```

---

## Dependency Flow

```
                  ┌─────────┐
                  │ config  │  (public facade)
                  └────┬────┘
                       │
          ┌────────────┼────────────┐
          │            │            │
    ┌─────▼─────┐ ┌───▼────┐ ┌─────▼─────┐
    │  service  │ │ eventbus│ │ registry  │
    │           │ │        │ │           │
    │ query     │ │        │ │ bundle    │
    │ mutation  │ │        │ │           │
    │ runtime   │ │        │ │ loader    │
    │ plugin    │ │        │ │ decoder   │
    └─────┬─────┘ └───┬────┘ │ provider  │
          │           │      └───────────┘
          │     ┌─────▼─────┐
          │     │  pattern  │
          │     └───────────┘
          │
    ┌─────▼──────┐
    │  pipeline   │
    └─────┬──────┘
          │
    ┌─────▼──────┐       ┌──────────────┐
    │ interceptor │────▶│  domain/event │
    └─────────────┘      └──────────────┘
          │
    ┌─────▼──────┐
    │ observability│
    └─────┬──────┘
          │
    ┌─────▼──────────────────────┐
    │        domain core         │  (zero external dependencies)
    │                            │
    │  value  │  event  │  errors │
    │  layer  │  pattern│        │
    └────────────────────────────┘

 Dependency direction:  config → service → pipeline → domain
                                            service → eventbus → domain
                                            service → registry → loader/decoder/provider
                                            eventbus → pattern
```

**Key rules:**
- `domain/` packages import nothing from `internal/` or external packages.
- `service/` depends on `pipeline`, `eventbus`, `interceptor`, `domain`, and `registry`.
- `config/` (facade) depends only on `service`, `eventbus`, `interceptor`, `registry`, and `pipeline`.
- Infrastructure packages (`loader`, `provider`, `decoder`, `binder`, `watcher`) depend only on `domain/` types.

---

## Performance Characteristics

### Bounded Concurrency
Every concurrent resource is bounded at construction time:

| Resource              | Default  | Configurable via                          |
|-----------------------|----------|-------------------------------------------|
| Event bus workers     | 32       | `WithBusWorkers(n)`                       |
| Event bus queue       | 4096     | `WithBusQueueSize(n)`                     |
| Layer reload workers  | 8        | `WithMaxWorkers(n)`                       |
| Pipeline concurrency  | 1 (sync) | N/A (serial by design, parallel via caller) |

This means memory usage is **predictable and constant** regardless of load:
- Fixed worker goroutine count
- Fixed queue capacity
- No unbounded goroutine spawning

### Predictable Resource Use
- **State**: immutable snapshots; mutations create new `State` objects (copy-on-write).
- **Checksums**: SHA-256 computed on state transitions, not on every read.
- **Sorted keys**: computed on demand, cached only when explicitly requested.
- **Copy-on-write maps**: `Value.Copy()` and `State.GetAll()` allocate new maps,
  ensuring safe concurrent reads without locking reads.

### Pipeline Overhead
The pipeline adds minimal overhead — each middleware is a function call.
Benchmarks (`BenchmarkPipelineRunWithMiddleware`) show:
- 0 middleware: ~50ns per command
- 5 realistic middleware (recovery + correlation + tracing + metrics + logging):
  ~200-500ns per command depending on middleware implementation.

### Event Bus Throughput
- Async `Publish` is ~100-300ns per call (enqueue only, non-blocking).
- `PublishSync` scales linearly with subscriber count.
- Pattern matching is O(p) where p = number of registered patterns.

### Value Operations
- `Value.New()` with type inference: ~10-20ns per call.
- `Value.Copy()` for 1000-key map: ~5µs.
- `Value.Merge()` for 5 layers × 100 keys: ~15µs.
- `ComputeChecksum` for 1000-key map: ~100µs (SHA-256 over all values).
- `ComputeDiff` for 1000 keys with 10% changes: ~10µs.
