// Package main demonstrates every feature of the config library in a single
// runnable program. Each section is self-contained and prints its results so
// you can follow along by reading the output.
//
// Features covered:
//  1. Builder pattern        — NewBuilder, File, Env, Memory, Watch, Validate, Recorder, Plugin
//  2. Functional options     — config.New with WithLoader, WithValidator, WithRecorder, etc.
//  3. Struct binding         — Bind, MustBind, tag-based mapping, defaults, nested structs
//  4. Validation             — validator.New, built-in tags, custom validation tags
//  5. Event system           — Subscribe, OnChange, event types, trace IDs, labels
//  6. Hooks                  — Before/After hooks for reload, set, delete, close
//  7. Schema generation      — JSON Schema from struct types
//  8. Snapshot / Restore     — save and restore config state
//  9. Operations             — atomic, reversible set/delete/bind operations
//  10. Explain                — key provenance tracing
//  11. Layered loading        — MemoryLoader, EnvLoader, FileLoader, priority merging
//  12. Profiles               — MemoryProfile, FileProfile, EnvProfile
//  13. Observability          — AtomicMetrics recorder, NopRecorder
//  14. Secure storage         — AESGCMEncryptor, SecureStore, SecureSource
//  15. Plugin system          — custom Plugin implementation
//  16. Watcher / debounce     — pattern-based change watching
//  17. Core Engine            — direct engine usage, Get, GetAll, Keys, Version, Has, Len
//  18. Value types            — type inference, accessors, As[T] generic
//  19. Error handling         — ConfigError codes, IsCode, Wrap
//  20. testutil               — zero-I/O test helper
//  21. YAML file loading      — load config.yaml with full production-style config
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	playground "github.com/go-playground/validator/v10"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/value"
	_errors "github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/hooks"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/observability"
	"github.com/os-gomod/config/plugin"
	"github.com/os-gomod/config/profile"
	"github.com/os-gomod/config/schema"
	"github.com/os-gomod/config/secure"
	"github.com/os-gomod/config/validator"
	"github.com/os-gomod/config/watcher"
)

// ---------------------------------------------------------------------------
// Domain structs — simple (used in demos 1-20)
// ---------------------------------------------------------------------------

// AppConfig is the top-level application configuration struct. Struct tags
// drive binding ("config"), defaults ("default"), and validation ("validate").
type AppConfig struct {
	App     AppSection     `config:"app"`
	DB      DBSection      `config:"db"`
	Server  ServerSection  `config:"server"`
	Feature FeatureSection `config:"feature"`
}

type AppSection struct {
	Name    string `config:"name"    default:"my-service"  validate:"required"`
	Version string `config:"version" default:"1.0.0"       validate:"required"`
	Env     string `config:"env"     default:"development" validate:"required,oneof_ci=development staging production"`
}

type DBSection struct {
	Host     string        `config:"host"      default:"localhost" validate:"required"`
	Port     int           `config:"port"      default:"5432"      validate:"required,min=1,max=65535"`
	Name     string        `config:"name"      default:"mydb"      validate:"required"`
	Timeout  time.Duration `config:"timeout"   default:"30s"`
	MaxConns int           `config:"max_conns" default:"10"        validate:"min=1"`
}

type ServerSection struct {
	Addr         string `config:"addr"          default:":8080" validate:"required"`
	ReadTimeout  int    `config:"read_timeout"  default:"30"`
	WriteTimeout int    `config:"write_timeout" default:"30"`
	EnableTLS    bool   `config:"enable_tls"    default:"false"`
}

type FeatureSection struct {
	DarkMode bool `config:"dark_mode" default:"false"`
	Beta     bool `config:"beta"      default:"false"`
	MaxItems int  `config:"max_items" default:"100"   validate:"min=1"`
}

// ---------------------------------------------------------------------------
// Domain structs — full production YAML config (config.yaml)
// ---------------------------------------------------------------------------

// ProductionConfig mirrors the full config.yaml structure. Every section of
// the YAML file is represented as a nested struct with appropriate struct
// tags for binding, defaults, and validation.
type ProductionConfig struct {
	App          AppConfigYAML          `config:"app"`
	Server       ServerConfigYAML       `config:"server"`
	Database     DatabaseConfigYAML     `config:"database"`
	Redis        RedisConfigYAML        `config:"redis"`
	Cache        CacheConfigYAML        `config:"cache"`
	Logging      LoggingConfigYAML      `config:"logging"`
	Features     FeaturesConfigYAML     `config:"features"`
	Monitoring   MonitoringConfigYAML   `config:"monitoring"`
	Integrations IntegrationsConfigYAML `config:"integrations"`
}

// AppConfigYAML represents the app section from config.yaml.
type AppConfigYAML struct {
	Name        string `config:"name"        validate:"required"`
	Version     string `config:"version"     validate:"required"`
	Environment string `config:"environment" validate:"required,oneof_ci=development staging production"`
	Debug       bool   `config:"debug"`
}

// TLSSection is a reusable nested TLS configuration block.
type TLSSection struct {
	Enabled bool `config:"enabled"`
}

// ServerConfigYAML represents the server section from config.yaml.
type ServerConfigYAML struct {
	Host            string        `config:"host"             default:"0.0.0.0" validate:"required"`
	Port            int           `config:"port"             default:"8080"    validate:"required,min=1,max=65535"`
	ReadTimeout     time.Duration `config:"read_timeout"     default:"30s"`
	WriteTimeout    time.Duration `config:"write_timeout"    default:"30s"`
	IdleTimeout     time.Duration `config:"idle_timeout"     default:"120s"`
	MaxHeaderBytes  int           `config:"max_header_bytes" default:"1048576"`
	TLS             TLSSection    `config:"tls"`
	RateLimit       int           `config:"rate_limit"       default:"100"`
	GracefulTimeout time.Duration `config:"graceful_timeout" default:"15s"`
}

// DatabaseConfigYAML represents the database section from config.yaml.
type DatabaseConfigYAML struct {
	Driver          string        `config:"driver"            default:"postgres"     validate:"required,oneof_ci=postgres mysql sqlite cockroach"`
	Host            string        `config:"host"              default:"localhost"    validate:"required"`
	Port            int           `config:"port"              default:"5432"         validate:"required,min=1,max=65535"`
	Name            string        `config:"name"              default:"mydb"         validate:"required"`
	User            string        `config:"user"              default:"app_user"`
	MaxOpenConns    int           `config:"max_open_conns"    default:"25"           validate:"min=1"`
	MaxIdleConns    int           `config:"max_idle_conns"    default:"10"           validate:"min=1"`
	ConnMaxLifetime time.Duration `config:"conn_max_lifetime" default:"5m"`
	MigrationPath   string        `config:"migration_path"    default:"./migrations"`
}

// RedisConfigYAML represents the redis section from config.yaml.
type RedisConfigYAML struct {
	Addresses    []string      `config:"addresses"`
	DB           int           `config:"db"             default:"0"`
	PoolSize     int           `config:"pool_size"      default:"10"`
	MinIdleConns int           `config:"min_idle_conns" default:"5"`
	DialTimeout  time.Duration `config:"dial_timeout"   default:"5s"`
	ReadTimeout  time.Duration `config:"read_timeout"   default:"3s"`
	WriteTimeout time.Duration `config:"write_timeout"  default:"3s"`
	TLS          TLSSection    `config:"tls"`
}

// CacheConfigYAML represents the cache section from config.yaml.
type CacheConfigYAML struct {
	Type            string        `config:"type"             default:"memory" validate:"oneof_ci=memory redis memcached"`
	TTL             time.Duration `config:"ttl"              default:"10m"`
	MaxSize         int           `config:"max_size"         default:"10000"  validate:"min=0"`
	CleanupInterval time.Duration `config:"cleanup_interval" default:"5m"`
}

// LoggingConfigYAML represents the logging section from config.yaml.
type LoggingConfigYAML struct {
	Level      string            `config:"level"        default:"info"   validate:"oneof_ci=debug info warn error"`
	Format     string            `config:"format"       default:"json"   validate:"oneof_ci=json text"`
	Output     string            `config:"output"       default:"stdout" validate:"oneof_ci=stdout stderr file"`
	FilePath   string            `config:"file_path"`
	MaxSizeMB  int               `config:"max_size_mb"  default:"100"    validate:"min=1"`
	MaxBackups int               `config:"max_backups"  default:"5"      validate:"min=0"`
	MaxAgeDays int               `config:"max_age_days" default:"30"     validate:"min=1"`
	Compress   bool              `config:"compress"`
	Fields     map[string]string `config:"fields"`
}

// FeaturesConfigYAML represents the features section from config.yaml.
type FeaturesConfigYAML struct {
	Metrics      FeatureToggleYAML `config:"metrics"`
	Tracing      FeatureToggleYAML `config:"tracing"`
	Profiling    FeatureToggleYAML `config:"profiling"`
	Swagger      FeatureToggleYAML `config:"swagger"`
	GraphQL      FeatureToggleYAML `config:"graphql"`
	Experimental ExperimentalYAML  `config:"experimental"`
	Maintenance  MaintenanceYAML   `config:"maintenance"`
}

// FeatureToggleYAML represents a simple enabled/disabled feature toggle.
type FeatureToggleYAML struct {
	Enabled bool `config:"enabled"`
}

// ExperimentalYAML represents the experimental sub-section of features.
type ExperimentalYAML struct {
	API bool `config:"api"`
}

// MaintenanceYAML represents the maintenance sub-section of features.
type MaintenanceYAML struct {
	Mode bool `config:"mode"`
}

// MonitoringConfigYAML represents the monitoring section from config.yaml.
type MonitoringConfigYAML struct {
	Prometheus PrometheusConfigYAML `config:"prometheus"`
	Health     HealthConfigYAML     `config:"health"`
	Alerts     AlertsConfigYAML     `config:"alerts"`
}

// PrometheusConfigYAML represents the prometheus sub-section.
type PrometheusConfigYAML struct {
	Enabled   bool   `config:"enabled"`
	Path      string `config:"path"      default:"/metrics"`
	Port      int    `config:"port"      default:"9090"     validate:"min=1,max=65535"`
	Namespace string `config:"namespace" default:"myapp"`
}

// HealthConfigYAML represents the health sub-section.
type HealthConfigYAML struct {
	Enabled       bool          `config:"enabled"`
	Path          string        `config:"path"           default:"/health"`
	Detailed      bool          `config:"detailed"`
	CheckInterval time.Duration `config:"check_interval" default:"30s"`
}

// AlertsConfigYAML represents the alerts sub-section.
type AlertsConfigYAML struct {
	Enabled    bool   `config:"enabled"`
	WebhookURL string `config:"webhook_url"`
	Channel    string `config:"channel"`
	Threshold  int    `config:"threshold"   default:"3" validate:"min=1"`
}

// IntegrationsConfigYAML represents the integrations section from config.yaml.
type IntegrationsConfigYAML struct {
	Kafka     KafkaConfigYAML     `config:"kafka"`
	S3        S3ConfigYAML        `config:"s3"`
	Slack     SlackConfigYAML     `config:"slack"`
	PagerDuty PagerDutyConfigYAML `config:"pagerduty"`
}

// KafkaConfigYAML represents the kafka sub-section.
type KafkaConfigYAML struct {
	Brokers []string `config:"brokers"`
	Topic   string   `config:"topic"`
	GroupID string   `config:"group_id"`
	Version string   `config:"version"`
}

// S3ConfigYAML represents the s3 sub-section.
type S3ConfigYAML struct {
	Bucket   string `config:"bucket"`
	Region   string `config:"region"`
	Endpoint string `config:"endpoint"`
}

// SlackConfigYAML represents the slack sub-section.
type SlackConfigYAML struct {
	Channel  string `config:"channel"`
	Username string `config:"username"`
}

// PagerDutyConfigYAML represents the pagerduty sub-section.
type PagerDutyConfigYAML struct {
	Severity string `config:"severity" validate:"oneof_ci=critical high low"`
}

// ---------------------------------------------------------------------------
// 1. Builder Pattern
// ---------------------------------------------------------------------------

func demoBuilder() {
	fmt.Println("\n=== 1. Builder Pattern ===")

	// The Builder provides a fluent, method-chaining API for constructing a
	// Config. Each method returns the same Builder pointer so calls can be
	// chained. Memory data is the simplest way to supply values (no I/O).
	cfg, err := config.NewBuilder().
		WithContext(context.Background()).
		Memory(map[string]any{
			"app.name":    "demo-service",
			"app.version": "2.0.0",
			"app.env":     "production",
			"db.host":     "db.prod.internal",
			"db.port":     5432,
		}).
		Validate(validator.New()).
		Build()
	if err != nil {
		// Build may return a *ReloadWarning if non-strict and a layer fails;
		// treat that as non-fatal for this demo.
		if _, ok := err.(*config.ReloadWarning); !ok {
			fmt.Printf("  Build error: %v\n", err)
			return
		}
		fmt.Printf("  Build warning (non-fatal): %v\n", err)
	}
	defer cfg.Close(context.Background())

	fmt.Printf("  app.name    = %v\n", mustGet(cfg, "app.name"))
	fmt.Printf("  db.host     = %v\n", mustGet(cfg, "db.host"))
	fmt.Printf("  db.port     = %v\n", mustGet(cfg, "db.port"))
}

// ---------------------------------------------------------------------------
// 2. Functional Options
// ---------------------------------------------------------------------------

func demoOptions() {
	fmt.Println("\n=== 2. Functional Options ===")

	// config.New() accepts functional options. This is equivalent to the
	// Builder but uses the options pattern common in Go libraries.
	memLoader := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"server.addr":       ":9090",
			"server.enable_tls": true,
			"feature.dark_mode": true,
			"feature.max_items": 50,
		}),
		loader.WithMemoryPriority(100),
	)

	cfg, err := config.New(context.Background(),
		config.WithLoader(memLoader),
		config.WithValidator(validator.New()),
		config.WithMaxWorkers(4),
		config.WithDebounce(300*time.Millisecond),
	)
	if err != nil {
		fmt.Printf("  New error: %v\n", err)
		return
	}
	defer cfg.Close(context.Background())

	fmt.Printf("  server.addr       = %v\n", mustGet(cfg, "server.addr"))
	fmt.Printf("  server.enable_tls = %v\n", mustGet(cfg, "server.enable_tls"))
	fmt.Printf("  feature.dark_mode = %v\n", mustGet(cfg, "feature.dark_mode"))
}

// ---------------------------------------------------------------------------
// 3. Struct Binding
// ---------------------------------------------------------------------------

func demoBinding() {
	fmt.Println("\n=== 3. Struct Binding ===")

	cfg := config.NewBuilder().
		Memory(map[string]any{
			"app.name":          "binding-demo",
			"app.version":       "3.1.0",
			"app.env":           "staging",
			"db.host":           "staging-db.internal",
			"db.port":           5433,
			"db.name":           "stagingdb",
			"db.timeout":        "10s",
			"db.max_conns":      25,
			"server.addr":       ":8081",
			"feature.beta":      true,
			"feature.max_items": 200,
		}).
		MustBuild()
	defer cfg.Close(context.Background())

	var appCfg AppConfig
	if errBind := cfg.Bind(context.Background(), &appCfg); errBind != nil {
		fmt.Printf("  Bind error: %v\n", errBind)
		return
	}

	fmt.Printf("  App Name      = %s\n", appCfg.App.Name)
	fmt.Printf("  App Version   = %s\n", appCfg.App.Version)
	fmt.Printf("  DB Host:Port  = %s:%d\n", appCfg.DB.Host, appCfg.DB.Port)
	fmt.Printf("  DB Timeout    = %v\n", appCfg.DB.Timeout)
	fmt.Printf("  Server Addr   = %s\n", appCfg.Server.Addr)
	fmt.Printf("  Feature Beta  = %v\n", appCfg.Feature.Beta)
	fmt.Printf("  Feature MaxIt = %d\n", appCfg.Feature.MaxItems)

	// MustBind panics on error — use in init paths where failure is fatal.
	var quick AppConfig
	cfg2 := config.NewBuilder().
		Memory(map[string]any{"app.name": "quick", "app.version": "1.0", "app.env": "development"}).
		MustBind(context.Background(), &quick)
	defer cfg2.Close(context.Background())
	fmt.Printf("  MustBind name = %s\n", quick.App.Name)
}

// ---------------------------------------------------------------------------
// 4. Validation
// ---------------------------------------------------------------------------

func demoValidation() {
	fmt.Println("\n=== 4. Validation ===")

	// The validator package wraps go-playground/validator with custom tags
	// like "oneof_ci" (case-insensitive oneof), "required_env", "filepath",
	// "urlhttp", and "duration".
	v := validator.New()

	// Valid config.
	valid := AppSection{Name: "svc", Version: "1.0", Env: "production"}
	if err := v.Validate(context.Background(), valid); err != nil {
		fmt.Printf("  Valid struct rejected: %v\n", err)
	} else {
		fmt.Println("  Valid struct passed validation")
	}

	// Invalid: empty name (required) and bad env.
	invalid := AppSection{Name: "", Version: "1.0", Env: "invalid"}
	if err := v.Validate(context.Background(), invalid); err != nil {
		fmt.Printf("  Invalid struct caught: %v\n", err)
	}

	// Custom validation tag.
	v2 := validator.New(
		validator.WithCustomTag("even", func(fl playground.FieldLevel) bool {
			return fl.Field().Int()%2 == 0
		}),
	)
	fmt.Println("  Custom 'even' tag registered (available for struct tags)")
	_ = v2
}

// ---------------------------------------------------------------------------
// 5. Event System
// ---------------------------------------------------------------------------

func demoEvents() {
	fmt.Println("\n=== 5. Event System ===")

	cfg := config.NewBuilder().
		Memory(map[string]any{"counter": 0}).
		MustBuild()
	defer cfg.Close(context.Background())

	var delivered sync.WaitGroup
	delivered.Add(4)

	// Subscribe to ALL events (catch-all observer).
	unsubAll := cfg.Subscribe(func(_ context.Context, evt event.Event) error {
		defer delivered.Done()
		fmt.Printf("  [ALL] type=%s key=%s\n", evt.Type, evt.Key)
		return nil
	})

	// OnChange registers a pattern-based observer for keys matching "counter*".
	unsubPattern := cfg.OnChange("counter*", func(_ context.Context, evt event.Event) error {
		defer delivered.Done()
		fmt.Printf("  [PATTERN] key=%s old=%v new=%v\n", evt.Key, evt.OldValue, evt.NewValue)
		return nil
	})

	// Set triggers a TypeCreate event (first time) and TypeUpdate subsequently.
	_ = cfg.Set(context.Background(), "counter", 1)
	_ = cfg.Set(context.Background(), "counter", 2)
	waitForWaitGroup(&delivered, time.Second)

	// Unsubscribe.
	unsubAll()
	unsubPattern()

	// After unsubscribe, further changes are silent.
	_ = cfg.Set(context.Background(), "counter", 3)
	fmt.Println("  (events after unsubscribe are silent)")

	// Events can carry trace IDs and labels for observability.
	evt := event.New(event.TypeUpdate, "demo.key",
		event.WithTraceID("abc-123"),
		event.WithLabel("region", "us-east-1"),
		event.WithMetadata("source", "cli"),
	)
	fmt.Printf("  Event traceID=%s labels=%v\n", evt.TraceID, evt.Labels)
}

// ---------------------------------------------------------------------------
// 6. Hooks
// ---------------------------------------------------------------------------

func demoHooks() {
	fmt.Println("\n=== 6. Hooks ===")

	// Hooks run before/after lifecycle operations. They are registered on
	// the hooks.Manager with a HookType and a Hook implementation.
	hookMgr := hooks.NewManager()

	// Register a before-reload hook using the convenience constructor.
	hookMgr.Register(event.HookBeforeReload, hooks.New("log-reload", 10,
		func(_ context.Context, hctx *hooks.Context) error {
			fmt.Printf("  [HOOK] Before reload at %v\n", hctx.StartTime.Format(time.RFC3339))
			return nil
		},
	))

	// Register an after-set hook.
	hookMgr.Register(event.HookAfterSet, hooks.New("audit-set", 20,
		func(_ context.Context, hctx *hooks.Context) error {
			fmt.Printf("  [HOOK] After set key=%s value=%v\n", hctx.Key, hctx.NewValue)
			return nil
		},
	))

	// Execute hooks manually (the Config type does this internally).
	_ = hookMgr.Execute(context.Background(), event.HookBeforeReload,
		&hooks.Context{Operation: "reload", StartTime: time.Now()})
	_ = hookMgr.Execute(context.Background(), event.HookAfterSet,
		&hooks.Context{Operation: "set", Key: "demo", NewValue: 42})

	fmt.Printf("  Hook count: before-reload=%d, after-set=%d\n",
		hookMgr.Count(event.HookBeforeReload), hookMgr.Count(event.HookAfterSet))
}

// ---------------------------------------------------------------------------
// 7. Schema Generation
// ---------------------------------------------------------------------------

func demoSchema() {
	fmt.Println("\n=== 7. Schema Generation ===")

	// The schema package generates JSON Schema from Go struct types.
	// This is useful for documentation, validation, and code generation.
	gen := schema.New(
		schema.WithTitle("AppConfig"),
		schema.WithDescription("Application configuration schema"),
	)

	sch, err := gen.Generate(AppConfig{})
	if err != nil {
		fmt.Printf("  Generate error: %v\n", err)
		return
	}

	// Write the schema to JSON.
	b, _ := json.MarshalIndent(sch, "", "  ")
	preview := string(b)
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	fmt.Printf("  Generated JSON Schema (%d bytes):\n  %s\n", len(b), preview)

	// Schema.WriteTo streams to any io.Writer.
	// var buf bytes.Buffer
	// sch.WriteTo(&buf)
}

// ---------------------------------------------------------------------------
// 8. Snapshot / Restore
// ---------------------------------------------------------------------------

func demoSnapshotRestore() {
	fmt.Println("\n=== 8. Snapshot / Restore ===")

	cfg := config.NewBuilder().
		Memory(map[string]any{"a": 1, "b": "hello"}).
		MustBuild()
	defer cfg.Close(context.Background())

	_ = cfg.Set(context.Background(), "a", 2)

	// Snapshot captures the entire state.
	snap := cfg.Snapshot()
	fmt.Printf("  Snapshot keys=%d, a=%v, b=%v\n", len(snap), snap["a"], snap["b"])

	// Mutate state.
	_ = cfg.Set(context.Background(), "a", 99)
	_ = cfg.Delete(context.Background(), "b")
	fmt.Printf("  After mutation: a=%v, b exists=%v\n", mustGet(cfg, "a"), cfg.Has("b"))

	// Restore from snapshot.
	cfg.Restore(snap)
	fmt.Printf("  After restore: a=%v, b exists=%v\n", mustGet(cfg, "a"), cfg.Has("b"))
}

// ---------------------------------------------------------------------------
// 9. Operations (Atomic, Reversible)
// ---------------------------------------------------------------------------

func demoOperations() {
	fmt.Println("\n=== 9. Operations ===")

	cfg := config.NewBuilder().
		Memory(map[string]any{"x": "original"}).
		MustBuild()
	defer cfg.Close(context.Background())

	// Individual operation.
	setOp := config.NewSetOperation("x", "updated")
	if err := config.ApplyOperation(context.Background(), cfg, setOp); err != nil {
		fmt.Printf("  Apply error: %v\n", err)
	}
	fmt.Printf("  After set: x=%v\n", mustGet(cfg, "x"))

	// Batch operations with automatic rollback on failure.
	ops := []config.Operation{
		config.NewSetOperation("y", "added"),
		config.NewSetOperation("z", "added"),
		config.NewBindOperation(&AppConfig{}), // bind is a no-op rollback
	}
	if err := config.ApplyOperations(context.Background(), cfg, ops); err != nil {
		fmt.Printf("  Batch error: %v\n", err)
	}
	fmt.Printf("  After batch: y=%v, z=%v\n", mustGet(cfg, "y"), mustGet(cfg, "z"))

	// Delete operation with rollback.
	delOp := config.NewDeleteOperation("y")
	_ = config.ApplyOperation(context.Background(), cfg, delOp)
	fmt.Printf("  After delete: y exists=%v\n", cfg.Has("y"))
}

// ---------------------------------------------------------------------------
// 10. Explain (Key Provenance)
// ---------------------------------------------------------------------------

func demoExplain() {
	fmt.Println("\n=== 10. Explain (Key Provenance) ===")

	// When multiple layers contribute values for the same key, Explain shows
	// which layer won and what the value is.
	cfg := config.NewBuilder().
		MemoryWithPriority(map[string]any{"db.host": "memory-host"}, 50).
		MustBuild()
	defer cfg.Close(context.Background())

	fmt.Printf("  Explain db.host: %s\n", cfg.Explain("db.host"))
	fmt.Printf("  Explain missing: %q\n", cfg.Explain("nonexistent"))
}

// ---------------------------------------------------------------------------
// 11. Layered Loading (Priority Merging)
// ---------------------------------------------------------------------------

func demoLayeredLoading() {
	fmt.Println("\n=== 11. Layered Loading ===")

	// When multiple layers define the same key, the layer with the highest
	// priority wins. This is how environment overrides work: env (priority 40)
	// beats file (priority 30) beats memory (priority 20).
	//
	// Priority levels by convention:
	//   10 — defaults / base
	//   20 — memory overrides
	//   30 — file config
	//   40 — environment variables
	//   50+ — remote / secrets

	// Layer 1: base defaults.
	base := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"db.host": "localhost",
			"db.port": 5432,
		}),
		loader.WithMemoryPriority(10),
	)

	// Layer 2: file-like overrides (higher priority).
	override := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"db.host": "prod-db.internal",
		}),
		loader.WithMemoryPriority(30),
	)

	cfg, err := config.New(context.Background(),
		config.WithLoader(base),
		config.WithLoader(override),
	)
	if err != nil {
		fmt.Printf("  New error: %v\n", err)
		return
	}
	defer cfg.Close(context.Background())

	// db.host should come from the override layer (priority 30 > 10).
	fmt.Printf("  db.host = %v (should be prod-db.internal)\n", mustGet(cfg, "db.host"))
	fmt.Printf("  db.port = %v (only in base, so base wins)\n", mustGet(cfg, "db.port"))
}

// ---------------------------------------------------------------------------
// 12. Profiles
// ---------------------------------------------------------------------------

func demoProfiles() {
	fmt.Println("\n=== 12. Profiles ===")

	// Profiles bundle a named set of layers for a particular environment.
	// They are convenient for switching between dev/staging/prod configurations.
	devProfile := profile.MemoryProfile("development", map[string]any{
		"app.env":     "development",
		"db.host":     "localhost",
		"server.addr": ":8080",
	}, 10)

	fmt.Printf("  Profile name = %s, layers = %d\n", devProfile.Name, len(devProfile.Layers))
	fmt.Printf("  Layer[0] name=%s priority=%d\n",
		devProfile.Layers[0].Name, devProfile.Layers[0].Priority)

	// Profiles can also be created from files or env:
	//   profile.FileProfile("production", "/etc/app/config.yaml", 30)
	//   profile.EnvProfile("production", "APP_", 40)

	// A profile can be applied directly to a core.Engine:
	eng := core.New()
	if err := devProfile.Apply(eng); err != nil {
		fmt.Printf("  Apply error: %v\n", err)
	}
	fmt.Println("  Profile applied to engine successfully")

	// EnvProfile example (no actual env vars needed for demo).
	ep := profile.EnvProfile("env-overlay", "MYAPP_", 40)
	fmt.Printf("  EnvProfile name=%s, layer[0].source=%v\n", ep.Name, ep.Layers[0].Source)
}

// ---------------------------------------------------------------------------
// 13. Observability (Metrics & Recorder)
// ---------------------------------------------------------------------------

func demoObservability() {
	fmt.Println("\n=== 13. Observability ===")

	// AtomicMetrics is a zero-dependency recorder that uses atomic counters.
	// It is safe for concurrent use and ideal for exposing metrics in /debug
	// endpoints or pulling snapshots for monitoring.
	metrics := &observability.AtomicMetrics{}

	cfg, err := config.New(context.Background(),
		config.WithLoader(loader.NewMemoryLoader(
			loader.WithMemoryData(map[string]any{"key": "val"}),
		)),
		config.WithRecorder(metrics),
	)
	if err != nil {
		fmt.Printf("  New error: %v\n", err)
		return
	}
	defer cfg.Close(context.Background())

	// Perform some operations to populate metrics.
	_ = cfg.Set(context.Background(), "key", "new-val")
	_, _ = cfg.Reload(context.Background())

	snap := metrics.Snapshot()
	fmt.Printf("  Reloads=%d, Sets=%d, Errors=%d\n",
		snap["reloads"], snap["sets"], snap["errors"])

	// NopRecorder is the default when no recorder is set.
	nop := observability.Nop()
	fmt.Printf("  NopRecorder type: %T\n", nop)

	// The OTelRecorder integrates with OpenTelemetry:
	//   otelRec, _ := observability.NewOTelRecorder(meter, tracer)
	//   config.New(ctx, config.WithRecorder(otelRec))
}

// ---------------------------------------------------------------------------
// 14. Secure Storage
// ---------------------------------------------------------------------------

func demoSecure() {
	fmt.Println("\n=== 14. Secure Storage ===")

	// Create an AES-256-GCM encryptor (key must be 32 bytes).
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	enc, err := secure.NewAESGCMEncryptor(key)
	if err != nil {
		fmt.Printf("  Encryptor error: %v\n", err)
		return
	}

	// SecureStore encrypts values at rest in memory.
	store := secure.NewSecureStore(enc, secure.WithPriority(60))

	// Store a secret.
	if setErr := store.Set("database.password", []byte("s3cr3t!")); setErr != nil {
		fmt.Printf("  Set error: %v\n", setErr)
		return
	}

	// Retrieve and decrypt.
	plain, err := store.Get("database.password")
	if err != nil {
		fmt.Printf("  Get error: %v\n", err)
		return
	}
	fmt.Printf("  Decrypted secret: %s\n", string(plain))
	fmt.Printf("  Has key: %v, Keys: %v\n", store.Has("database.password"), store.Keys())

	// SecureSource wraps a SecureStore as a loader.Loader so secrets can be
	// layered into the config engine like any other data source.
	src := secure.NewSecureSource(store, secure.WithPriority(60))
	fmt.Printf("  SecureSource priority=%d string=%s\n", src.Priority(), src.String())

	// Use the secure source as a loader layer.
	cfg, err := config.New(context.Background(), config.WithLoader(src))
	if err != nil {
		fmt.Printf("  Config with SecureSource error: %v\n", err)
		return
	}
	defer cfg.Close(context.Background())

	fmt.Printf("  database.password from config = %v\n", mustGet(cfg, "database.password"))
}

// ---------------------------------------------------------------------------
// 15. Plugin System
// ---------------------------------------------------------------------------

// demoPlugin is a custom Plugin that registers a custom validation tag.
type demoPlugin struct{}

func (demoPlugin) Name() string { return "demo-plugin" }
func (demoPlugin) Init(h plugin.Host) error {
	fmt.Println("  Plugin Init() called — registering custom validator tag")
	return h.RegisterValidator("demo_tag", func(fl playground.FieldLevel) bool {
		return fl.Field().String() != ""
	})
}

func demoPluginSystem() {
	fmt.Println("\n=== 15. Plugin System ===")

	p := &demoPlugin{}
	type pluginConfig struct {
		App struct {
			Name string `config:"name" validate:"demo_tag"`
		} `config:"app"`
	}

	cfg, err := config.NewBuilder().
		Memory(map[string]any{"app.name": "plugin-demo", "app.version": "1.0", "app.env": "development"}).
		Validate(validator.New()).
		Plugin(p).
		Build()
	if err != nil {
		if _, ok := err.(*config.ReloadWarning); !ok {
			fmt.Printf("  Build error: %v\n", err)
			return
		}
	}
	defer cfg.Close(context.Background())

	var bound pluginConfig
	if errBind := cfg.Bind(context.Background(), &bound); errBind != nil {
		fmt.Printf("  Plugin validation failed: %v\n", errBind)
		return
	}
	fmt.Printf("  Registered plugins: %v\n", cfg.Plugins())
	fmt.Printf("  Plugin validator accepted app.name=%s\n", bound.App.Name)
}

// ---------------------------------------------------------------------------
// 16. Watcher / Debounce
// ---------------------------------------------------------------------------

func demoWatcher() {
	fmt.Println("\n=== 16. Watcher / Debounce ===")

	// The watcher.Manager coordinates change notifications with debouncing
	// to batch rapid changes into a single reload.
	mgr := watcher.NewManager()

	// Subscribe to changes.
	mgr.Subscribe(func(_ context.Context, change watcher.Change) {
		fmt.Printf("  [CHANGE] type=%s key=%s\n", change.Type, change.Key)
	})

	// Notify of a change.
	mgr.Notify(context.Background(), watcher.Change{
		Type:      watcher.ChangeModified,
		Key:       "db.host",
		OldValue:  value.NewInMemory("localhost"),
		NewValue:  value.NewInMemory("prod-db.internal"),
		Timestamp: time.Now(),
	})

	// TriggerReload debounces: calling it multiple times within the debounce
	// window results in only one callback execution.
	callCount := 0
	mgr.TriggerReload(100*time.Millisecond, func() {
		callCount++
	})
	mgr.TriggerReload(100*time.Millisecond, func() {
		callCount++
	})
	time.Sleep(200 * time.Millisecond)
	fmt.Printf("  Debounced reloads executed: %d (batched from 2 triggers)\n", callCount)

	mgr.StopDebouncer()

	// PatternWatcher filters events by key pattern.
	pw := watcher.NewPatternWatcher("db.*", func(_ context.Context, evt event.Event) error {
		fmt.Printf("  [PATTERN WATCHER] key=%s\n", evt.Key)
		return nil
	})
	_ = pw.Observe(context.Background(), event.New(event.TypeUpdate, "db.host"))
	_ = pw.Observe(context.Background(), event.New(event.TypeUpdate, "app.name")) // filtered out
}

// ---------------------------------------------------------------------------
// 17. Core Engine (Direct Usage)
// ---------------------------------------------------------------------------

func demoEngine() {
	fmt.Println("\n=== 17. Core Engine ===")

	// The core.Engine can be used directly for maximum control, bypassing
	// the Config facade. This is useful in libraries that only need the
	// merge/reload semantics without binding or events.
	eng := core.New(core.WithMaxWorkers(4))

	// Add a layer.
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{"k1": "v1", "k2": 42}),
		loader.WithMemoryPriority(10),
	)
	layer := core.NewLayer("mem", core.WithLayerSource(ml), core.WithLayerPriority(10))
	_ = eng.AddLayer(layer)

	// Initial load.
	result, err := eng.Reload(context.Background())
	if err != nil {
		fmt.Printf("  Reload error: %v\n", err)
		return
	}
	fmt.Printf("  Reload: events=%d, errors=%v\n", len(result.Events), result.HasErrors())

	// Read operations.
	v, ok := eng.Get("k1")
	fmt.Printf("  Get k1: ok=%v value=%v\n", ok, v)
	fmt.Printf("  Has k2: %v\n", eng.Has("k2"))
	fmt.Printf("  Keys: %v\n", eng.Keys())
	fmt.Printf("  Len: %d\n", eng.Len())
	fmt.Printf("  Version: %d\n", eng.Version())

	// Write operations return events.
	evt, err := eng.Set(context.Background(), "k3", "new")
	if err != nil {
		fmt.Printf("  Set error: %v\n", err)
	}
	fmt.Printf("  Set event: type=%s key=%s\n", evt.Type, evt.Key)

	events, err := eng.BatchSet(context.Background(), map[string]any{"k4": "a", "k5": "b"})
	if err != nil {
		fmt.Printf("  BatchSet error: %v\n", err)
	}
	fmt.Printf("  BatchSet events: %d\n", len(events))

	delEvt, err := eng.Delete(context.Background(), "k3")
	if err != nil {
		fmt.Printf("  Delete error: %v\n", err)
	}
	fmt.Printf("  Delete event: type=%s key=%s\n", delEvt.Type, delEvt.Key)

	// Layer health.
	fmt.Printf("  Layer healthy: %v\n", layer.IsHealthy())
	hs := layer.HealthStatus()
	fmt.Printf("  Health: healthy=%v fails=%d\n", hs.Healthy, hs.ConsecutiveFails)

	// Layer enable/disable.
	layer.Disable()
	fmt.Printf("  Layer enabled after Disable: %v\n", layer.IsEnabled())
	layer.Enable()
	fmt.Printf("  Layer enabled after Enable: %v\n", layer.IsEnabled())

	_ = eng.Close(context.Background())
	fmt.Printf("  Engine closed: %v\n", eng.IsClosed())
}

// ---------------------------------------------------------------------------
// 18. Value Types
// ---------------------------------------------------------------------------

func demoValues() {
	fmt.Println("\n=== 18. Value Types ===")

	// value.Value is an immutable container that carries the raw value,
	// its inferred type, source, and merge priority.
	v1 := value.NewInMemory("hello")
	fmt.Printf("  NewInMemory: raw=%v type=%s source=%s priority=%d\n",
		v1.Raw(), v1.Type(), v1.Source(), v1.Priority())

	v2 := value.New(42, value.TypeInt, value.SourceFile, 30)
	fmt.Printf("  New: raw=%v type=%s source=%s priority=%d\n",
		v2.Raw(), v2.Type(), v2.Source(), v2.Priority())

	// The generic As[T] function performs a type assertion.
	raw, ok := value.As[string](v1)
	fmt.Printf("  As[string]: %q ok=%v\n", raw, ok)

	// InferType deduces the Type from a Go value.
	fmt.Printf("  InferType(3.14)=%s, InferType(true)=%s, InferType([]any{})=%s\n",
		value.InferType(3.14), value.InferType(true), value.InferType([]any{}))

	// Duration values.
	durVal := value.NewInMemory("5s")
	d, ok := durVal.Duration()
	fmt.Printf("  Duration from string: %v ok=%v\n", d, ok)

	// Value equality.
	v3 := value.NewInMemory("hello")
	fmt.Printf("  Equal: %v\n", v1.Equal(v3))

	// Checksum and copy utilities.
	data := map[string]value.Value{"a": v1, "b": v2}
	checksum := value.ComputeChecksum(data)
	if len(checksum) > 16 {
		checksum = checksum[:16] + "..."
	}
	fmt.Printf("  Checksum: %s\n", checksum)
	fmt.Printf("  SortedKeys: %v\n", value.SortedKeys(data))

	copied := value.Copy(data)
	fmt.Printf("  Copy: len=%d, same keys=%v\n", len(copied), copied["a"].Equal(v1))

	// Bool, Int, Float64 accessors.
	boolVal := value.NewInMemory(true)
	b, ok := boolVal.Bool()
	fmt.Printf("  Bool accessor: %v ok=%v\n", b, ok)

	intVal := value.New(99, value.TypeInt, value.SourceMemory, 0)
	n, ok := intVal.Int()
	fmt.Printf("  Int accessor: %v ok=%v\n", n, ok)

	floatVal := value.New(3.14, value.TypeFloat64, value.SourceMemory, 0)
	f, ok := floatVal.Float64()
	fmt.Printf("  Float64 accessor: %v ok=%v\n", f, ok)

	// IsZero check.
	empty := value.Value{}
	fmt.Printf("  IsZero: %v\n", empty.IsZero())
}

// ---------------------------------------------------------------------------
// 19. Error Handling
// ---------------------------------------------------------------------------

func demoErrors() {
	fmt.Println("\n=== 19. Error Handling ===")

	// ConfigError carries structured error information: code, message, key,
	// source, operation, and path. It supports errors.Is and errors.As.
	err := _errors.New(_errors.CodeValidation, "port out of range").
		WithKey("db.port").
		WithSource("file").
		WithOperation("bind")

	fmt.Printf("  Error: %v\n", err)
	fmt.Printf("  IsCode(Validation): %v\n", _errors.IsCode(err, _errors.CodeValidation))

	// Wrap adds a cause.
	wrapped := _errors.Wrap(err, _errors.CodeBind, "bind failed")
	fmt.Printf("  Wrapped: %v\n", wrapped)
	fmt.Printf("  IsCode(Bind): %v\n", _errors.IsCode(wrapped, _errors.CodeBind))

	// Sentinel errors.
	fmt.Printf("  ErrClosed: %v\n", _errors.ErrClosed)
	fmt.Printf("  ErrNotFound: %v\n", _errors.ErrNotFound)
	fmt.Printf("  ErrNotImplemented: %v\n", _errors.ErrNotImplemented)

	// Stack trace.
	stackStr := err.Stack()
	if len(stackStr) > 40 {
		stackStr = stackStr[:40] + "..."
	}
	fmt.Printf("  Stack: %s\n", stackStr)
}

// ---------------------------------------------------------------------------
// 20. testutil (Zero-I/O Test Helper)
// ---------------------------------------------------------------------------

func demoTestutil() {
	fmt.Println("\n=== 20. testutil ===")

	// The testutil package provides Builder, a zero-I/O test helper for
	// constructing configs without touching the filesystem or environment.
	// It is designed for use in table-driven tests.
	//
	// Usage in tests:
	//   b := testutil.New(t)
	//   b.WithValues(map[string]any{"db.host": "test-host"})
	//   cfg := b.Build()
	//   var target MyConfig
	//   b.MustBind(&target)
	//
	// The Builder automatically cleans up any environment variables it sets
	// via t.Cleanup, so tests don't leak state.
	fmt.Println("  testutil.Builder — zero-I/O config builder for tests")
	fmt.Println(
		"  Methods: New(t), WithValues(kv), WithStruct(v), WithEnv(env), Build(), MustBind(target)",
	)
	fmt.Println("  (Not runnable outside of *testing.T — shown here for reference)")
}

// ---------------------------------------------------------------------------
// Bonus: EnvLoader with Key Replacer
// ---------------------------------------------------------------------------

func demoEnvLoader() {
	fmt.Println("\n=== Bonus: EnvLoader ===")

	// Set an environment variable for the demo.
	os.Setenv("MYAPP_DB_HOST", "env-db-host")
	defer os.Unsetenv("MYAPP_DB_HOST")

	// EnvLoader reads environment variables with a given prefix.
	// MYAPP_DB_HOST becomes db.host after stripping prefix and lowercasing.
	envLoader := loader.NewEnvLoader(
		loader.WithEnvPrefix("MYAPP_"),
		loader.WithEnvPriority(40),
	)

	cfg, err := config.New(context.Background(), config.WithLoader(envLoader))
	if err != nil {
		fmt.Printf("  New error: %v\n", err)
		return
	}
	defer cfg.Close(context.Background())

	fmt.Printf("  db.host from env = %v\n", mustGet(cfg, "db.host"))

	// The key replacer can customize how env var names map to config keys.
	upperReplacer := loader.NewEnvLoader(
		loader.WithEnvPrefix("MYAPP_"),
		loader.WithEnvKeyReplacer(func(s string) string { return s }),
	)
	fmt.Printf("  Custom key replacer loader created: %s\n", upperReplacer.String())
}

// ---------------------------------------------------------------------------
// Bonus: Decoder Registry
// ---------------------------------------------------------------------------

func demoDecoder() {
	fmt.Println("\n=== Bonus: Decoder Registry ===")

	// The decoder package provides a registry of decoders for common formats.
	// DefaultRegistry comes pre-loaded with YAML, JSON, dotenv, and INI.
	//
	// Usage:
	//   import "github.com/os-gomod/config/decoder"
	//   dec, _ := decoder.DefaultRegistry.ForExtension(".json")
	//   data, _ := dec.Decode([]byte(`{"key": "value"}`))
	//
	fmt.Println("  decoder.DefaultRegistry — pre-loaded with YAML, JSON, INI, dotenv decoders")
	fmt.Println("  Methods: ForExtension(ext), ForMediaType(mt), Register(d), Names()")
}

// ---------------------------------------------------------------------------
// 21. YAML File Loading (Full Production Config)
// ---------------------------------------------------------------------------

func demoYAMLFileLoading() {
	fmt.Println("\n=== 21. YAML File Loading (config.yaml) ===")

	// Resolve the config.yaml path relative to the example directory.
	// When running from the example/ directory, the file is at ./config.yaml.
	// When running from the module root, it is at example/config.yaml.
	yamlPath := findConfigFile("config.yaml")
	if yamlPath == "" {
		fmt.Println("  (config.yaml not found — skipping YAML file demo)")
		fmt.Println("  To run this demo, ensure example/config.yaml exists.")
		return
	}
	fmt.Printf("  Loading config from: %s\n", yamlPath)

	// Build a Config from the YAML file with memory defaults as a base layer.
	// Priority convention: memory(10) < file(30) < env(40).
	metrics := &observability.AtomicMetrics{}
	cfg, err := config.NewBuilder().
		WithContext(context.Background()).
		// Base layer: sensible defaults via memory (lowest priority).
		MemoryWithPriority(map[string]any{
			"app.name":        "default-app",
			"app.version":     "0.0.1",
			"app.environment": "development",
			"app.debug":       true,
			"server.host":     "127.0.0.1",
			"server.port":     3000,
			"database.driver": "sqlite",
			"cache.type":      "memory",
			"logging.level":   "info",
		}, 10).
		// File layer: the actual config.yaml (medium priority).
		FileWithPriority(yamlPath, 30).
		// Environment layer: overrides for containerized deployments.
		EnvWithPriority("APP_", 40).
		Validate(validator.New()).
		Recorder(metrics).
		OnReloadError(func(reloadErr error) {
			slog.Warn("yaml demo: reload error", "err", reloadErr)
		}).
		Build()
	if err != nil {
		if _, ok := err.(*config.ReloadWarning); !ok {
			fmt.Printf("  Build error: %v\n", err)
			return
		}
		fmt.Printf("  Build warning (non-fatal): %v\n", err)
	}
	defer cfg.Close(context.Background())

	// Bind the YAML config into the full production struct.
	var prodCfg ProductionConfig
	if bindErr := cfg.Bind(context.Background(), &prodCfg); bindErr != nil {
		fmt.Printf("  Bind error: %v\n", bindErr)
		return
	}

	// Print the bound configuration sections.
	fmt.Println("\n  --- App ---")
	fmt.Printf("  Name        = %s\n", prodCfg.App.Name)
	fmt.Printf("  Version     = %s\n", prodCfg.App.Version)
	fmt.Printf("  Environment = %s\n", prodCfg.App.Environment)
	fmt.Printf("  Debug       = %v\n", prodCfg.App.Debug)

	fmt.Println("\n  --- Server ---")
	fmt.Printf("  Host            = %s\n", prodCfg.Server.Host)
	fmt.Printf("  Port            = %d\n", prodCfg.Server.Port)
	fmt.Printf("  ReadTimeout     = %v\n", prodCfg.Server.ReadTimeout)
	fmt.Printf("  WriteTimeout    = %v\n", prodCfg.Server.WriteTimeout)
	fmt.Printf("  IdleTimeout     = %v\n", prodCfg.Server.IdleTimeout)
	fmt.Printf("  MaxHeaderBytes  = %d\n", prodCfg.Server.MaxHeaderBytes)
	fmt.Printf("  TLS Enabled     = %v\n", prodCfg.Server.TLS.Enabled)
	fmt.Printf("  RateLimit       = %d\n", prodCfg.Server.RateLimit)
	fmt.Printf("  GracefulTimeout = %v\n", prodCfg.Server.GracefulTimeout)

	fmt.Println("\n  --- Database ---")
	fmt.Printf("  Driver          = %s\n", prodCfg.Database.Driver)
	fmt.Printf("  Host:Port       = %s:%d\n", prodCfg.Database.Host, prodCfg.Database.Port)
	fmt.Printf("  Name            = %s\n", prodCfg.Database.Name)
	fmt.Printf("  User            = %s\n", prodCfg.Database.User)
	fmt.Printf("  MaxOpenConns    = %d\n", prodCfg.Database.MaxOpenConns)
	fmt.Printf("  MaxIdleConns    = %d\n", prodCfg.Database.MaxIdleConns)
	fmt.Printf("  ConnMaxLifetime = %v\n", prodCfg.Database.ConnMaxLifetime)
	fmt.Printf("  MigrationPath   = %s\n", prodCfg.Database.MigrationPath)

	fmt.Println("\n  --- Redis ---")
	fmt.Printf("  Addresses     = %v\n", prodCfg.Redis.Addresses)
	fmt.Printf("  DB            = %d\n", prodCfg.Redis.DB)
	fmt.Printf("  PoolSize      = %d\n", prodCfg.Redis.PoolSize)
	fmt.Printf("  MinIdleConns  = %d\n", prodCfg.Redis.MinIdleConns)
	fmt.Printf("  DialTimeout   = %v\n", prodCfg.Redis.DialTimeout)
	fmt.Printf("  ReadTimeout   = %v\n", prodCfg.Redis.ReadTimeout)
	fmt.Printf("  WriteTimeout  = %v\n", prodCfg.Redis.WriteTimeout)
	fmt.Printf("  TLS Enabled   = %v\n", prodCfg.Redis.TLS.Enabled)

	fmt.Println("\n  --- Cache ---")
	fmt.Printf("  Type            = %s\n", prodCfg.Cache.Type)
	fmt.Printf("  TTL             = %v\n", prodCfg.Cache.TTL)
	fmt.Printf("  MaxSize         = %d\n", prodCfg.Cache.MaxSize)
	fmt.Printf("  CleanupInterval = %v\n", prodCfg.Cache.CleanupInterval)

	fmt.Println("\n  --- Logging ---")
	fmt.Printf("  Level      = %s\n", prodCfg.Logging.Level)
	fmt.Printf("  Format     = %s\n", prodCfg.Logging.Format)
	fmt.Printf("  Output     = %s\n", prodCfg.Logging.Output)
	fmt.Printf("  FilePath   = %s\n", prodCfg.Logging.FilePath)
	fmt.Printf("  MaxSizeMB  = %d\n", prodCfg.Logging.MaxSizeMB)
	fmt.Printf("  MaxBackups = %d\n", prodCfg.Logging.MaxBackups)
	fmt.Printf("  MaxAgeDays = %d\n", prodCfg.Logging.MaxAgeDays)
	fmt.Printf("  Compress   = %v\n", prodCfg.Logging.Compress)
	fmt.Printf("  Fields     = %v\n", prodCfg.Logging.Fields)

	fmt.Println("\n  --- Features ---")
	fmt.Printf("  Metrics      enabled=%v\n", prodCfg.Features.Metrics.Enabled)
	fmt.Printf("  Tracing      enabled=%v\n", prodCfg.Features.Tracing.Enabled)
	fmt.Printf("  Profiling    enabled=%v\n", prodCfg.Features.Profiling.Enabled)
	fmt.Printf("  Swagger      enabled=%v\n", prodCfg.Features.Swagger.Enabled)
	fmt.Printf("  GraphQL      enabled=%v\n", prodCfg.Features.GraphQL.Enabled)
	fmt.Printf("  Experimental api=%v\n", prodCfg.Features.Experimental.API)
	fmt.Printf("  Maintenance  mode=%v\n", prodCfg.Features.Maintenance.Mode)

	fmt.Println("\n  --- Monitoring ---")
	fmt.Printf("  Prometheus  enabled=%v path=%s port=%d namespace=%s\n",
		prodCfg.Monitoring.Prometheus.Enabled,
		prodCfg.Monitoring.Prometheus.Path,
		prodCfg.Monitoring.Prometheus.Port,
		prodCfg.Monitoring.Prometheus.Namespace)
	fmt.Printf("  Health      enabled=%v path=%s detailed=%v interval=%v\n",
		prodCfg.Monitoring.Health.Enabled,
		prodCfg.Monitoring.Health.Path,
		prodCfg.Monitoring.Health.Detailed,
		prodCfg.Monitoring.Health.CheckInterval)
	fmt.Printf("  Alerts      enabled=%v channel=%s threshold=%d\n",
		prodCfg.Monitoring.Alerts.Enabled,
		prodCfg.Monitoring.Alerts.Channel,
		prodCfg.Monitoring.Alerts.Threshold)

	fmt.Println("\n  --- Integrations ---")
	fmt.Printf("  Kafka     brokers=%v topic=%s group=%s version=%s\n",
		prodCfg.Integrations.Kafka.Brokers,
		prodCfg.Integrations.Kafka.Topic,
		prodCfg.Integrations.Kafka.GroupID,
		prodCfg.Integrations.Kafka.Version)
	fmt.Printf("  S3        bucket=%s region=%s endpoint=%s\n",
		prodCfg.Integrations.S3.Bucket,
		prodCfg.Integrations.S3.Region,
		prodCfg.Integrations.S3.Endpoint)
	fmt.Printf("  Slack     channel=%s username=%s\n",
		prodCfg.Integrations.Slack.Channel,
		prodCfg.Integrations.Slack.Username)
	fmt.Printf("  PagerDuty severity=%s\n",
		prodCfg.Integrations.PagerDuty.Severity)

	// Explain provenance for a key (shows which layer provided the value).
	fmt.Println("\n  --- Explain ---")
	fmt.Printf("  app.name: %s\n", cfg.Explain("app.name"))
	fmt.Printf("  server.port: %s\n", cfg.Explain("server.port"))
	fmt.Printf("  database.host: %s\n", cfg.Explain("database.host"))

	// Generate schema from the production config struct.
	fmt.Println("\n  --- Schema ---")
	gen := schema.New(
		schema.WithTitle("ProductionConfig"),
		schema.WithDescription("Full production configuration schema from config.yaml"),
	)
	sch, schemaErr := gen.Generate(ProductionConfig{})
	if schemaErr != nil {
		fmt.Printf("  Schema generation error: %v\n", schemaErr)
	} else {
		b, _ := json.MarshalIndent(sch, "", "  ")
		fmt.Printf("  Generated JSON Schema for ProductionConfig (%d bytes)\n", len(b))
	}

	fmt.Println("\n  YAML file loading demo completed successfully")
}

// ---------------------------------------------------------------------------
// Bonus: Full End-to-End Flow
// ---------------------------------------------------------------------------

func demoEndToEnd() {
	fmt.Println("\n=== Bonus: Full End-to-End Flow ===")

	// This demo combines multiple features into a realistic flow:
	//   1. Create a Config with memory defaults + env overrides + validation
	//   2. Bind to a struct
	//   3. Subscribe to changes
	//   4. Mutate and observe events
	//   5. Snapshot, mutate, restore
	//   6. Generate schema
	//   7. Explain key provenance
	//   8. Clean shutdown

	// Set env for override.
	os.Setenv("E2E_DB_HOST", "e2e-db.internal")
	defer os.Unsetenv("E2E_DB_HOST")

	// Build with multiple layers: memory base + env overlay.
	metrics := &observability.AtomicMetrics{}
	cfg, err := config.NewBuilder().
		WithContext(context.Background()).
		Memory(map[string]any{
			"app.name":     "e2e-demo",
			"app.version":  "1.0.0",
			"app.env":      "development",
			"db.host":      "localhost",
			"db.port":      5432,
			"db.name":      "mydb",
			"db.timeout":   "30s",
			"db.max_conns": 10,
			"server.addr":  ":8080",
			"feature.beta": false,
		}).
		EnvWithPriority("E2E_", 40).
		Validate(validator.New()).
		Recorder(metrics).
		OnReloadError(func(reloadErr error) {
			slog.Warn("e2e: reload error", "err", reloadErr)
		}).
		Build()
	if err != nil {
		fmt.Printf("  Build error: %v\n", err)
		return
	}
	defer cfg.Close(context.Background())

	// Subscribe to all events.
	eventCount := 0
	var mu sync.Mutex
	var delivered sync.WaitGroup
	delivered.Add(2)
	unsub := cfg.Subscribe(func(_ context.Context, _ event.Event) error {
		defer delivered.Done()
		mu.Lock()
		eventCount++
		mu.Unlock()
		return nil
	})

	// Bind to struct.
	var appCfg AppConfig
	if bindErr := cfg.Bind(context.Background(), &appCfg); bindErr != nil {
		fmt.Printf("  Bind error: %v\n", bindErr)
		return
	}
	fmt.Printf("  Bound: app=%s db=%s:%d\n", appCfg.App.Name, appCfg.DB.Host, appCfg.DB.Port)

	// Explain provenance.
	fmt.Printf("  Explain db.host: %s\n", cfg.Explain("db.host"))

	// Mutate and observe.
	_ = cfg.Set(context.Background(), "feature.beta", true)
	_ = cfg.Set(context.Background(), "app.version", "2.0.0")
	waitForWaitGroup(&delivered, time.Second)
	mu.Lock()
	fmt.Printf("  Events observed: %d\n", eventCount)
	mu.Unlock()

	// Snapshot + restore.
	snap := cfg.Snapshot()
	_ = cfg.Set(context.Background(), "app.version", "9.9.9")
	cfg.Restore(snap)
	fmt.Printf("  After restore, version = %v\n", mustGet(cfg, "app.version"))

	// Generate schema.
	sch, _ := cfg.Schema(AppConfig{})
	b, _ := json.MarshalIndent(sch, "", "  ")
	fmt.Printf("  Schema generated (%d bytes)\n", len(b))

	unsub()

	// Metrics snapshot.
	m := metrics.Snapshot()
	fmt.Printf(
		"  Metrics: binds=%d sets=%d reloads=%d\n",
		m["binds"],
		m["sets"],
		m["reloads"],
	)

	fmt.Println("  Full end-to-end flow completed successfully")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustGet(cfg *config.Config, key string) any {
	v, ok := cfg.Get(key)
	if !ok {
		return "<missing>"
	}
	return v.Raw()
}

func waitForWaitGroup(wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		return
	}
}

// findConfigFile searches for the named config file in the current directory
// and then in the example/ subdirectory relative to the working directory.
// Returns the absolute path if found, or an empty string.
func findConfigFile(name string) string {
	// Check current directory.
	if _, err := os.Stat(name); err == nil {
		if abs, absErr := filepath.Abs(name); absErr == nil {
			return abs
		}
		return name
	}
	// Check example/ subdirectory.
	candidate := filepath.Join("example", name)
	if _, err := os.Stat(candidate); err == nil {
		if abs, absErr := filepath.Abs(candidate); absErr == nil {
			return abs
		}
		return candidate
	}
	return ""
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	fmt.Println("==============================================================")
	fmt.Println("  github.com/os-gomod/config — Comprehensive Feature Demo")
	fmt.Println("==============================================================")

	demoBuilder()
	demoOptions()
	demoBinding()
	demoValidation()
	demoEvents()
	demoHooks()
	demoSchema()
	demoSnapshotRestore()
	demoOperations()
	demoExplain()
	demoLayeredLoading()
	demoProfiles()
	demoObservability()
	demoSecure()
	demoPluginSystem()
	demoWatcher()
	demoEngine()
	demoValues()
	demoErrors()
	demoTestutil()
	demoEnvLoader()
	demoDecoder()
	demoYAMLFileLoading()
	demoEndToEnd()

	fmt.Println("\nAll feature demos completed.")
}
