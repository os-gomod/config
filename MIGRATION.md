# Migration Guide

## From Viper

Viper and `config` solve similar problems but `config` is designed for production-grade microservices with stronger type safety, observability, and reliability guarantees.

### Key Differences

| Viper | config |
|-------|--------|
| `viper.New()` | `config.New(ctx, ...)` |
| `viper.SetDefault("key", val)` | Use `default:"val"` struct tag |
| `viper.Set("key", val)` | `cfg.Set(ctx, "key", val)` |
| `viper.Get("key")` | `cfg.Get("key")` (returns typed Value) |
| `viper.GetString("key")` | Use `cfg.Bind(ctx, &struct{})` |
| `viper.Unmarshal(&cfg)` | `cfg.Bind(ctx, &cfg)` |
| `viper.WatchConfig()` | `cfg.OnChange(pattern, fn)` |
| `viper.AddConfigPath()` | `config.WithLoader(loader.NewFile(path))` |
| `viper.AutomaticEnv()` | `config.WithLoader(loader.NewEnv(prefix))` |
| `viper.BindPFlag()` | Load flags into MemoryLoader |
| `viper.MergeConfigMap()` | Layer priority merge (automatic) |
| `viper.OnConfigChange()` | `cfg.OnChange(pattern, obs)` |
| `viper.ReadInConfig()` | Automatic on `config.New()` |
| Global state | Explicit Config instance |
| No circuit breaker | Per-source circuit breaker |
| No audit trail | Full audit logging |
| No secret redaction | Automatic redaction |

### Migration Example

**Viper:**
```go
v := viper.New()
v.SetConfigName("config")
v.SetConfigType("yaml")
v.AddConfigPath(".")
v.AutomaticEnv()
v.SetDefault("server.port", 8080)
v.ReadInConfig()
v.WatchConfig()

port := v.GetInt("server.port")
host := v.GetString("server.host")
```

**config:**
```go
cfg, _ := config.New(ctx,
    config.WithLoader(loader.NewFile("config.yaml")),
    config.WithLoader(loader.NewEnv("APP")),
    config.WithStrictReload(),
)

type Server struct {
    Port int    `config:"server.port" default:"8080"`
    Host string `config:"server.host"`
}
var srv Server
_ = cfg.Bind(ctx, &srv)
// srv.Port == 8080 (from default or config)
// srv.Host == "0.0.0.0" (from config or env)
```

### Benefits of Migrating
- **Type safety**: Compile-time checked struct binding
- **Performance**: <50ms for 10K keys vs ~200ms in Viper
- **Reliability**: Circuit breaker prevents cascading failures
- **Observability**: Built-in OTel traces and metrics
- **Security**: Automatic secret redaction in all events
- **Compliance**: Full audit trail for SOC2/HIPAA

## From Koanf

### Key Differences

| Koanf | config |
|-------|--------|
| `koanf.New()` | `config.New(ctx, ...)` |
| `k.Load(file.Provider(), yaml.Parser())` | `config.WithLoader(loader.NewFile("config.yaml"))` |
| `k.Unmarshal("server", &cfg)` | `cfg.Bind(ctx, &cfg)` |
| `k.Watch()` | `cfg.OnChange(pattern, fn)` |
| Manual confmap merge | Automatic priority-based merge |
| No built-in observability | OTel + metrics |
| No circuit breaker | Per-source circuit breaker |

### Migration Example

**Koanf:**
```go
k := koanf.New(".")
k.Load(file.Provider("config.yaml"), yaml.Parser())
k.Load(env.Provider("APP_", ".", func(s string) string {
    return strings.Replace(strings.ToLower(s), "_", ".", -1)
}), nil)

var cfg ServerConfig
k.UnmarshalWithConf("server", &cfg, koanf.UnmarshalConf{Tag: "config"})
```

**config:**
```go
cfg, _ := config.New(ctx,
    config.WithLoader(loader.NewFile("config.yaml")),
    config.WithLoader(loader.NewEnv("APP", loader.WithKeyReplacer(".", "_"))),
)
var srv ServerConfig
_ = cfg.Bind(ctx, &srv)
```

## Migration Checklist

- [ ] Replace `viper.New()` / `koanf.New()` with `config.New(ctx, ...)`
- [ ] Replace `AddConfigPath`/`ReadInConfig` with `config.WithLoader(loader.NewFile(...))`
- [ ] Replace `AutomaticEnv` with `config.WithLoader(loader.NewEnv(...))`
- [ ] Replace `Get*` calls with struct binding via `cfg.Bind(ctx, &struct)`
- [ ] Replace `SetDefault` with `default:"val"` struct tags
- [ ] Replace `WatchConfig`/`OnChange` with `cfg.OnChange(pattern, fn)`
- [ ] Add context.Context to all config operations
- [ ] Replace global viper instance with explicit Config dependency injection
- [ ] Add circuit breaker config for remote sources
- [ ] Enable audit logging for compliance requirements
- [ ] Add namespace isolation for multi-tenant deployments
