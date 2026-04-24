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
	configerrors "github.com/os-gomod/config/errors"
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

type AppConfig struct {
	App     AppSection     `config:"app"`
	DB      DBSection      `config:"db"`
	Server  ServerSection  `config:"server"`
	Feature FeatureSection `config:"feature"`
}
type AppSection struct {
	Name    string `config:"name"     validate:"required"`
	Version string `config:"version"  validate:"required"`
	Env     string `config:"env"      validate:"required,oneof_ci=development staging production"`
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
type AppConfigYAML struct {
	Name        string `config:"name"        validate:"required"`
	Version     string `config:"version"     validate:"required"`
	Environment string `config:"environment" validate:"required,oneof_ci=development staging production"`
	Debug       bool   `config:"debug"`
}
type TLSSection struct {
	Enabled bool `config:"enabled"`
}
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
type CacheConfigYAML struct {
	Type            string        `config:"type"             default:"memory" validate:"oneof_ci=memory redis memcached"`
	TTL             time.Duration `config:"ttl"              default:"10m"`
	MaxSize         int           `config:"max_size"         default:"10000"  validate:"min=0"`
	CleanupInterval time.Duration `config:"cleanup_interval" default:"5m"`
}
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
type FeaturesConfigYAML struct {
	Metrics      FeatureToggleYAML `config:"metrics"`
	Tracing      FeatureToggleYAML `config:"tracing"`
	Profiling    FeatureToggleYAML `config:"profiling"`
	Swagger      FeatureToggleYAML `config:"swagger"`
	GraphQL      FeatureToggleYAML `config:"graphql"`
	Experimental ExperimentalYAML  `config:"experimental"`
	Maintenance  MaintenanceYAML   `config:"maintenance"`
}
type FeatureToggleYAML struct {
	Enabled bool `config:"enabled"`
}
type ExperimentalYAML struct {
	API bool `config:"api"`
}
type MaintenanceYAML struct {
	Mode bool `config:"mode"`
}
type MonitoringConfigYAML struct {
	Prometheus PrometheusConfigYAML `config:"prometheus"`
	Health     HealthConfigYAML     `config:"health"`
	Alerts     AlertsConfigYAML     `config:"alerts"`
}
type PrometheusConfigYAML struct {
	Enabled   bool   `config:"enabled"`
	Path      string `config:"path"      default:"/metrics"`
	Port      int    `config:"port"      default:"9090"     validate:"min=1,max=65535"`
	Namespace string `config:"namespace" default:"myapp"`
}
type HealthConfigYAML struct {
	Enabled       bool          `config:"enabled"`
	Path          string        `config:"path"           default:"/health"`
	Detailed      bool          `config:"detailed"`
	CheckInterval time.Duration `config:"check_interval" default:"30s"`
}
type AlertsConfigYAML struct {
	Enabled    bool   `config:"enabled"`
	WebhookURL string `config:"webhook_url"`
	Channel    string `config:"channel"`
	Threshold  int    `config:"threshold"   default:"3" validate:"min=1"`
}
type IntegrationsConfigYAML struct {
	Kafka     KafkaConfigYAML     `config:"kafka"`
	S3        S3ConfigYAML        `config:"s3"`
	Slack     SlackConfigYAML     `config:"slack"`
	PagerDuty PagerDutyConfigYAML `config:"pagerduty"`
}
type KafkaConfigYAML struct {
	Brokers []string `config:"brokers"`
	Topic   string   `config:"topic"`
	GroupID string   `config:"group_id"`
	Version string   `config:"version"`
}
type S3ConfigYAML struct {
	Bucket   string `config:"bucket"`
	Region   string `config:"region"`
	Endpoint string `config:"endpoint"`
}
type SlackConfigYAML struct {
	Channel  string `config:"channel"`
	Username string `config:"username"`
}
type PagerDutyConfigYAML struct {
	Severity string `config:"severity" validate:"oneof_ci=critical high low"`
}

func demoBuilder() {
	fmt.Println("\n=== 1. Builder Pattern ===")
	cfg, err := config.NewBuilder().
		WithContext(context.Background()).
		Memory(map[string]any{
			"app.name":    "demo-service",
			"app.version": "2.0.0",
			"app.env":     "production",
			"db.host":     "db.prod.internal",
			"db.port":     5432,
		}).
		ValidateWith(validator.New()).
		Build()
	if err != nil {
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

func demoOptions() {
	fmt.Println("\n=== 2. Functional Options ===")
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
	var quick AppConfig
	cfg2 := config.NewBuilder().
		Memory(map[string]any{"app.name": "quick", "app.version": "1.0", "app.env": "development"}).
		MustBind(context.Background(), &quick)
	defer cfg2.Close(context.Background())
	fmt.Printf("  MustBind name = %s\n", quick.App.Name)
}

func demoValidation() {
	fmt.Println("\n=== 4. Validation ===")
	v := validator.New()
	valid := AppSection{Name: "svc", Version: "1.0", Env: "production"}
	if err := v.Validate(context.Background(), valid); err != nil {
		fmt.Printf("  Valid struct rejected: %v\n", err)
	} else {
		fmt.Println("  Valid struct passed validation")
	}
	invalid := AppSection{Name: "", Version: "1.0", Env: "invalid"}
	if err := v.Validate(context.Background(), invalid); err != nil {
		fmt.Printf("  Invalid struct caught: %v\n", err)
	}

	v2 := validator.New(
		validator.WithCustomTag("even", func(fl playground.FieldLevel) bool {
			return fl.Field().Int()%2 == 0
		}),
	)
	fmt.Println("  Custom 'even' tag registered (available for struct tags)")
	_ = v2
}

func demoEvents() {
	fmt.Println("\n=== 5. Event System ===")
	cfg := config.NewBuilder().
		Memory(map[string]any{"counter": 0}).
		MustBuild()
	defer cfg.Close(context.Background())
	var delivered sync.WaitGroup
	delivered.Add(4)
	unsubAll := cfg.Subscribe(func(_ context.Context, evt event.Event) error {
		defer delivered.Done()
		fmt.Printf("  [ALL] type=%s key=%s\n", evt.Type, evt.Key)
		return nil
	})
	unsubPattern := cfg.OnChange("counter*", func(_ context.Context, evt event.Event) error {
		defer delivered.Done()
		fmt.Printf("  [PATTERN] key=%s old=%v new=%v\n", evt.Key, evt.OldValue, evt.NewValue)
		return nil
	})
	_ = cfg.Set(context.Background(), "counter", 1)
	_ = cfg.Set(context.Background(), "counter", 2)
	waitForWaitGroup(&delivered, time.Second)
	unsubAll()
	unsubPattern()
	_ = cfg.Set(context.Background(), "counter", 3)
	fmt.Println("  (events after unsubscribe are silent)")
	evt := event.New(event.TypeUpdate, "demo.key",
		event.WithTraceID("abc-123"),
		event.WithLabel("region", "us-east-1"),
		event.WithMetadata("source", "cli"),
	)
	fmt.Printf("  Event traceID=%s labels=%v\n", evt.TraceID, evt.Labels)
}

func demoHooks() {
	fmt.Println("\n=== 6. Hooks ===")
	hookMgr := hooks.NewManager()
	hookMgr.Register(event.HookBeforeReload, hooks.New("log-reload", 10,
		func(_ context.Context, hctx *hooks.Context) error {
			fmt.Printf("  [HOOK] Before reload at %v\n", hctx.StartTime.Format(time.RFC3339))
			return nil
		},
	))
	hookMgr.Register(event.HookAfterSet, hooks.New("audit-set", 20,
		func(_ context.Context, hctx *hooks.Context) error {
			fmt.Printf("  [HOOK] After set key=%s value=%v\n", hctx.Key, hctx.NewValue)
			return nil
		},
	))
	_ = hookMgr.Execute(context.Background(), event.HookBeforeReload,
		&hooks.Context{Operation: "reload", StartTime: time.Now()})
	_ = hookMgr.Execute(context.Background(), event.HookAfterSet,
		&hooks.Context{Operation: "set", Key: "demo", NewValue: 42})
	fmt.Printf("  Hook count: before-reload=%d, after-set=%d\n",
		hookMgr.Count(event.HookBeforeReload), hookMgr.Count(event.HookAfterSet))
}

func demoSchema() {
	fmt.Println("\n=== 7. Schema Generation ===")
	gen := schema.New(
		schema.WithTitle("AppConfig"),
		schema.WithDescription("Application configuration schema"),
	)
	sch, err := gen.Generate(AppConfig{})
	if err != nil {
		fmt.Printf("  Generate error: %v\n", err)
		return
	}
	b, _ := json.MarshalIndent(sch, "", "  ")
	preview := string(b)
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	fmt.Printf("  Generated JSON Schema (%d bytes):\n  %s\n", len(b), preview)
}

func demoSnapshotRestore() {
	fmt.Println("\n=== 8. Snapshot / Restore ===")
	cfg := config.NewBuilder().
		Memory(map[string]any{"a": 1, "b": "hello"}).
		MustBuild()
	defer cfg.Close(context.Background())
	_ = cfg.Set(context.Background(), "a", 2)
	snap := cfg.Snapshot()
	fmt.Printf("  Snapshot keys=%d, a=%v, b=%v\n", len(snap), snap["a"], snap["b"])
	_ = cfg.Set(context.Background(), "a", 99)
	_ = cfg.Delete(context.Background(), "b")
	fmt.Printf("  After mutation: a=%v, b exists=%v\n", mustGet(cfg, "a"), cfg.Has("b"))
	cfg.Restore(snap)
	fmt.Printf("  After restore: a=%v, b exists=%v\n", mustGet(cfg, "a"), cfg.Has("b"))
}

func demoOperations() {
	fmt.Println("\n=== 9. Operations ===")
	cfg := config.NewBuilder().
		Memory(map[string]any{"x": "original"}).
		MustBuild()
	defer cfg.Close(context.Background())
	setOp := config.NewSetOperation("x", "updated")
	if err := config.ApplyOperation(context.Background(), cfg, setOp); err != nil {
		fmt.Printf("  Apply error: %v\n", err)
	}
	fmt.Printf("  After set: x=%v\n", mustGet(cfg, "x"))
	ops := []config.Operation{
		config.NewSetOperation("y", "added"),
		config.NewSetOperation("z", "added"),
		config.NewBindOperation(&AppConfig{}),
	}
	if err := config.ApplyOperations(context.Background(), cfg, ops); err != nil {
		fmt.Printf("  Batch error: %v\n", err)
	}
	fmt.Printf("  After batch: y=%v, z=%v\n", mustGet(cfg, "y"), mustGet(cfg, "z"))
	delOp := config.NewDeleteOperation("y")
	_ = config.ApplyOperation(context.Background(), cfg, delOp)
	fmt.Printf("  After delete: y exists=%v\n", cfg.Has("y"))
}

func demoExplain() {
	fmt.Println("\n=== 10. Explain (Key Provenance) ===")
	cfg := config.NewBuilder().
		MemoryWithPriority(map[string]any{"db.host": "memory-host"}, 50).
		MustBuild()
	defer cfg.Close(context.Background())
	fmt.Printf("  Explain db.host: %s\n", cfg.Explain("db.host"))
	fmt.Printf("  Explain missing: %q\n", cfg.Explain("nonexistent"))
}

func demoLayeredLoading() {
	fmt.Println("\n=== 11. Layered Loading ===")
	base := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"db.host": "localhost",
			"db.port": 5432,
		}),
		loader.WithMemoryPriority(10),
	)
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
	fmt.Printf("  db.host = %v (should be prod-db.internal)\n", mustGet(cfg, "db.host"))
	fmt.Printf("  db.port = %v (only in base, so base wins)\n", mustGet(cfg, "db.port"))
}

func demoProfiles() {
	fmt.Println("\n=== 12. Profiles ===")
	devProfile := profile.MemoryProfile("development", map[string]any{
		"app.env":     "development",
		"db.host":     "localhost",
		"server.addr": ":8080",
	}, 10)
	fmt.Printf("  Profile name = %s, layers = %d\n", devProfile.Name, len(devProfile.Layers))
	fmt.Printf("  Layer[0] name=%s priority=%d\n",
		devProfile.Layers[0].Name, devProfile.Layers[0].Priority)
	eng := core.New()
	if err := devProfile.Apply(eng); err != nil {
		fmt.Printf("  Apply error: %v\n", err)
	}
	fmt.Println("  Profile applied to engine successfully")
	ep := profile.EnvProfile("env-overlay", "MYAPP_", 40)
	fmt.Printf("  EnvProfile name=%s, layer[0].source=%v\n", ep.Name, ep.Layers[0].Source)
}

func demoObservability() {
	fmt.Println("\n=== 13. Observability ===")
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
	_ = cfg.Set(context.Background(), "key", "new-val")
	_, _ = cfg.Reload(context.Background())
	snap := metrics.Snapshot()
	fmt.Printf("  Reloads=%d, Sets=%d, Errors=%d\n",
		snap["reloads"], snap["sets"], snap["errors"])
	nop := observability.Nop()
	fmt.Printf("  NopRecorder type: %T\n", nop)
}

func demoSecure() {
	fmt.Println("\n=== 14. Secure Storage ===")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	enc, err := secure.NewAESGCMEncryptor(key)
	if err != nil {
		fmt.Printf("  Encryptor error: %v\n", err)
		return
	}
	store := secure.NewStore(enc, secure.WithPriority(60))
	if setErr := store.Set("database.password", []byte("s3cr3t!")); setErr != nil {
		fmt.Printf("  Set error: %v\n", setErr)
		return
	}
	plain, err := store.Get("database.password")
	if err != nil {
		fmt.Printf("  Get error: %v\n", err)
		return
	}
	fmt.Printf("  Decrypted secret: %s\n", string(plain))
	fmt.Printf("  Has key: %v, Keys: %v\n", store.Has("database.password"), store.Keys())
	src := secure.NewSource(store, secure.WithPriority(60))
	fmt.Printf("  Source priority=%d string=%s\n", src.Priority(), src.String())
	cfg, err := config.New(context.Background(), config.WithLoader(src))
	if err != nil {
		fmt.Printf("  Config with Source error: %v\n", err)
		return
	}
	defer cfg.Close(context.Background())
	fmt.Printf("  database.password from config = %v\n", mustGet(cfg, "database.password"))
}

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
		Validate().
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

func demoWatcher() {
	fmt.Println("\n=== 16. Watcher / Debounce ===")
	mgr := watcher.NewManager()
	mgr.Subscribe(func(_ context.Context, change watcher.Change) {
		fmt.Printf("  [CHANGE] type=%s key=%s\n", change.Type, change.Key)
	})
	mgr.Notify(context.Background(), &watcher.Change{
		Type:      watcher.ChangeModified,
		Key:       "db.host",
		OldValue:  value.NewInMemory("localhost"),
		NewValue:  value.NewInMemory("prod-db.internal"),
		Timestamp: time.Now(),
	})
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
	pw := watcher.NewPatternWatcher("db.*", func(_ context.Context, evt event.Event) error {
		fmt.Printf("  [PATTERN WATCHER] key=%s\n", evt.Key)
		return nil
	})
	evt1 := event.New(event.TypeUpdate, "db.host")
	_ = pw.Observe(context.Background(), &evt1)
	evt2 := event.New(event.TypeUpdate, "app.name")
	_ = pw.Observe(context.Background(), &evt2)
}

func demoEngine() {
	fmt.Println("\n=== 17. Core Engine ===")
	eng := core.New(core.WithMaxWorkers(4))
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{"k1": "v1", "k2": 42}),
		loader.WithMemoryPriority(10),
	)
	layer := core.NewLayer("mem", core.WithLayerSource(ml), core.WithLayerPriority(10))
	_ = eng.AddLayer(layer)
	result, err := eng.Reload(context.Background())
	if err != nil {
		fmt.Printf("  Reload error: %v\n", err)
		return
	}
	fmt.Printf("  Reload: events=%d, errors=%v\n", len(result.Events), result.HasErrors())
	v, ok := eng.Get("k1")
	fmt.Printf("  Get k1: ok=%v value=%v\n", ok, v)
	fmt.Printf("  Has k2: %v\n", eng.Has("k2"))
	fmt.Printf("  Keys: %v\n", eng.Keys())
	fmt.Printf("  Len: %d\n", eng.Len())
	fmt.Printf("  Version: %d\n", eng.Version())
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
	fmt.Printf("  Layer healthy: %v\n", layer.IsHealthy())
	hs := layer.HealthStatus()
	fmt.Printf("  Health: healthy=%v fails=%d\n", hs.Healthy, hs.ConsecutiveFails)
	layer.Disable()
	fmt.Printf("  Layer enabled after Disable: %v\n", layer.IsEnabled())
	layer.Enable()
	fmt.Printf("  Layer enabled after Enable: %v\n", layer.IsEnabled())
	_ = eng.Close(context.Background())
	fmt.Printf("  Engine closed: %v\n", eng.IsClosed())
}

func demoValues() {
	fmt.Println("\n=== 18. Value Types ===")
	v1 := value.NewInMemory("hello")
	fmt.Printf("  NewInMemory: raw=%v type=%s source=%s priority=%d\n",
		v1.Raw(), v1.Type(), v1.Source(), v1.Priority())
	v2 := value.New(42, value.TypeInt, value.SourceFile, 30)
	fmt.Printf("  New: raw=%v type=%s source=%s priority=%d\n",
		v2.Raw(), v2.Type(), v2.Source(), v2.Priority())
	raw, ok := value.As[string](v1)
	fmt.Printf("  As[string]: %q ok=%v\n", raw, ok)
	fmt.Printf("  InferType(3.14)=%s, InferType(true)=%s, InferType([]any{})=%s\n",
		value.InferType(3.14), value.InferType(true), value.InferType([]any{}))
	durVal := value.NewInMemory("5s")
	d, ok := durVal.Duration()
	fmt.Printf("  Duration from string: %v ok=%v\n", d, ok)
	v3 := value.NewInMemory("hello")
	fmt.Printf("  Equal: %v\n", v1.Equal(v3))
	data := map[string]value.Value{"a": v1, "b": v2}
	checksum := value.ComputeChecksum(data)
	if len(checksum) > 16 {
		checksum = checksum[:16] + "..."
	}
	fmt.Printf("  Checksum: %s\n", checksum)
	fmt.Printf("  SortedKeys: %v\n", value.SortedKeys(data))
	copied := value.Copy(data)
	fmt.Printf("  Copy: len=%d, same keys=%v\n", len(copied), copied["a"].Equal(v1))
	boolVal := value.NewInMemory(true)
	b, ok := boolVal.Bool()
	fmt.Printf("  Bool accessor: %v ok=%v\n", b, ok)
	intVal := value.New(99, value.TypeInt, value.SourceMemory, 0)
	n, ok := intVal.Int()
	fmt.Printf("  Int accessor: %v ok=%v\n", n, ok)
	floatVal := value.New(3.14, value.TypeFloat64, value.SourceMemory, 0)
	f, ok := floatVal.Float64()
	fmt.Printf("  Float64 accessor: %v ok=%v\n", f, ok)
	empty := value.Value{}
	fmt.Printf("  IsZero: %v\n", empty.IsZero())
}

func demoErrors() {
	fmt.Println("\n=== 19. Error Handling ===")
	err := configerrors.New(configerrors.CodeValidation, "port out of range").
		WithKey("db.port").
		WithSource("file").
		WithOperation("bind")
	fmt.Printf("  Error: %v\n", err)
	fmt.Printf("  IsCode(Validation): %v\n", configerrors.IsCode(err, configerrors.CodeValidation))
	wrapped := configerrors.Wrap(err, configerrors.CodeBind, "bind failed")
	fmt.Printf("  Wrapped: %v\n", wrapped)
	fmt.Printf("  IsCode(Bind): %v\n", configerrors.IsCode(wrapped, configerrors.CodeBind))
	fmt.Printf("  ErrClosed: %v\n", configerrors.ErrClosed)
	fmt.Printf("  ErrNotFound: %v\n", configerrors.ErrNotFound)
	fmt.Printf("  ErrNotImplemented: %v\n", configerrors.ErrNotImplemented)
	stackStr := err.Stack()
	if len(stackStr) > 40 {
		stackStr = stackStr[:40] + "..."
	}
	fmt.Printf("  Stack: %s\n", stackStr)
}

func demoTestutil() {
	fmt.Println("\n=== 20. testutil ===")
	fmt.Println("  testutil.Builder — zero-I/O config builder for tests")
	fmt.Println(
		"  Methods: New(t), WithValues(kv), WithStruct(v), WithEnv(env), Build(), MustBind(target)",
	)
	fmt.Println("  (Not runnable outside of *testing.T — shown here for reference)")
}

func demoYAMLFileLoading() {
	fmt.Println("\n=== 21. YAML File Loading (config.yaml) ===")
	yamlPath := findConfigFile("config.yaml")
	if yamlPath == "" {
		fmt.Println("  (config.yaml not found — skipping YAML file demo)")
		fmt.Println("  To run this demo, ensure example/config.yaml exists.")
		return
	}
	fmt.Printf("  Loading config from: %s\n", yamlPath)
	metrics := &observability.AtomicMetrics{}
	cfg, err := config.NewBuilder().
		WithContext(context.Background()).
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
		FileWithPriority(yamlPath, 30).
		EnvWithPriority("APP_", 40).
		Validate().
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
	var prodCfg ProductionConfig
	if bindErr := cfg.Bind(context.Background(), &prodCfg); bindErr != nil {
		fmt.Printf("  Bind error: %v\n", bindErr)
		return
	}
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
	fmt.Println("\n  --- Explain ---")
	fmt.Printf("  app.name: %s\n", cfg.Explain("app.name"))
	fmt.Printf("  server.port: %s\n", cfg.Explain("server.port"))
	fmt.Printf("  database.host: %s\n", cfg.Explain("database.host"))
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

func demoEnvLoader() {
	fmt.Println("\n=== Bonus: EnvLoader ===")
	os.Setenv("MYAPP_DB_HOST", "env-db-host")
	defer os.Unsetenv("MYAPP_DB_HOST")
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
	upperReplacer := loader.NewEnvLoader(
		loader.WithEnvPrefix("MYAPP_"),
		loader.WithEnvKeyReplacer(func(s string) string { return s }),
	)
	fmt.Printf("  Custom key replacer loader created: %s\n", upperReplacer.String())
}

func demoDecoder() {
	fmt.Println("\n=== Bonus: Decoder Registry ===")
	fmt.Println("  decoder.DefaultRegistry — pre-loaded with YAML, JSON, INI, dotenv decoders")
	fmt.Println("  Methods: ForExtension(ext), ForMediaType(mt), Register(d), Names()")
}

func demoBatchSet() {
	fmt.Println("\n=== Bonus: Batch Set ===")
	cfg := config.NewBuilder().
		Memory(map[string]any{}).
		MustBuild()
	defer cfg.Close(context.Background())
	if err := cfg.BatchSet(context.Background(), map[string]any{
		"app.name": "batch-demo",
		"app.env":  "production",
		"db.host":  "batch-db",
		"db.port":  3306,
	}); err != nil {
		fmt.Printf("  BatchSet error: %v\n", err)
		return
	}
	fmt.Printf("  Keys after batch: %v\n", cfg.Keys())
	fmt.Printf("  Len: %d\n", cfg.Len())
	fmt.Printf("  app.name=%s, db.host=%s, db.port=%d\n",
		mustGet(cfg, "app.name"),
		mustGet(cfg, "db.host"),
		mustGet(cfg, "db.port"),
	)
}

func demoConfigIntrospection() {
	fmt.Println("\n=== Bonus: Config Introspection ===")
	cfg := config.NewBuilder().
		Memory(map[string]any{
			"server.host": "0.0.0.0",
			"server.port": 8080,
			"db.host":     "localhost",
		}).
		MustBuild()
	defer cfg.Close(context.Background())
	fmt.Printf("  Has server.host: %v\n", cfg.Has("server.host"))
	fmt.Printf("  Has missing.key: %v\n", cfg.Has("missing.key"))
	fmt.Printf("  Keys: %v\n", cfg.Keys())
	fmt.Printf("  Len: %d\n", cfg.Len())
	fmt.Printf("  Version: %d\n", cfg.Version())
}

func demoEndToEnd() {
	fmt.Println("\n=== Bonus: Full End-to-End Flow ===")
	os.Setenv("E2E_DB_HOST", "e2e-db.internal")
	defer os.Unsetenv("E2E_DB_HOST")
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
		Validate().
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
	var appCfg AppConfig
	if bindErr := cfg.Bind(context.Background(), &appCfg); bindErr != nil {
		fmt.Printf("  Bind error: %v\n", bindErr)
		return
	}
	fmt.Printf("  Bound: app=%s db=%s:%d\n", appCfg.App.Name, appCfg.DB.Host, appCfg.DB.Port)
	fmt.Printf("  Explain db.host: %s\n", cfg.Explain("db.host"))
	_ = cfg.Set(context.Background(), "feature.beta", true)
	_ = cfg.Set(context.Background(), "app.version", "2.0.0")
	waitForWaitGroup(&delivered, time.Second)
	mu.Lock()
	fmt.Printf("  Events observed: %d\n", eventCount)
	mu.Unlock()
	snap := cfg.Snapshot()
	_ = cfg.Set(context.Background(), "app.version", "9.9.9")
	cfg.Restore(snap)
	fmt.Printf("  After restore, version = %v\n", mustGet(cfg, "app.version"))
	sch, _ := cfg.Schema(AppConfig{})
	b, _ := json.MarshalIndent(sch, "", "  ")
	fmt.Printf("  Schema generated (%d bytes)\n", len(b))
	unsub()
	m := metrics.Snapshot()
	fmt.Printf(
		"  Metrics: binds=%d sets=%d reloads=%d\n",
		m["binds"],
		m["sets"],
		m["reloads"],
	)
	fmt.Println("  Full end-to-end flow completed successfully")
}

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

func findConfigFile(name string) string {
	if _, err := os.Stat(name); err == nil {
		if abs, absErr := filepath.Abs(name); absErr == nil {
			return abs
		}
		return name
	}
	candidate := filepath.Join("example", name)
	if _, err := os.Stat(candidate); err == nil {
		if abs, absErr := filepath.Abs(candidate); absErr == nil {
			return abs
		}
		return candidate
	}
	return ""
}

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
	demoYAMLFileLoading()
	demoEnvLoader()
	demoDecoder()
	demoBatchSet()
	demoConfigIntrospection()
	demoEndToEnd()
	fmt.Println("\nAll feature demos completed.")
}
