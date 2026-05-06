package config

import (
	"context"
	"time"

	"github.com/os-gomod/config/v2/internal/audit"
	"github.com/os-gomod/config/v2/internal/interceptor"
	"github.com/os-gomod/config/v2/internal/service"
	"github.com/os-gomod/config/v2/internal/template"
	"github.com/os-gomod/config/v2/internal/version"
)

// =============================================================================
// Public Enhancement APIs
// =============================================================================

// NewHotReloader creates a hot reloader that integrates with the config's
// existing event bus and watcher system.
func (c *Config) NewHotReloader(debounceDuration time.Duration) *service.HotReloader {
	reloadFn := func(ctx context.Context) (int, error) {
		result, err := c.Reload(ctx)
		if err != nil {
			return 0, err
		}
		return len(result.Events), nil
	}

	cfg := service.DefaultHotReloadConfig()
	cfg.DebounceDuration = debounceDuration

	return service.NewHotReloader(reloadFn, cfg)
}

// NewHistoricalRecorder wraps the config's existing recorder with history capabilities.
func (c *Config) NewHistoricalRecorder(store audit.HistoryStore, maxSize int) *audit.HistoricalRecorder {
	// This requires access to the internal recorder - you'd need to expose it
	// or modify the config to hold a reference.
	// For now, this shows the integration pattern.
	return nil
}

// NewConditionalInterceptor creates a conditional interceptor for Set operations.
func (c *Config) NewConditionalInterceptor(inner interceptor.SetInterceptor) *interceptor.ConditionalInterceptor {
	return interceptor.NewConditionalInterceptor(inner)
}

// NewTemplateProcessor creates a template processor for config values.
func (c *Config) NewTemplateProcessor() *template.Processor {
	return template.NewProcessor(c)
}

// NewMigrationManager creates a version migration manager.
func (c *Config) NewMigrationManager(store version.VersionStore, currentVersion string) *version.Manager {
	return version.NewMigrationManager(store, currentVersion)
}

// =============================================================================
// Example Usage (in example/main.go)
// =============================================================================

/*
func ExampleEnhancements() {
	ctx := context.Background()

	// Create config as before
	data := make(map[string]value.Value)
	data["app.name"] = value.New("my-app")
	data["server.port"] = value.New(8080)
	memLayer := layer.NewStaticLayer("defaults", data, layer.WithPriority(10))

	cfg, err := New(ctx, WithLayer(memLayer))
	if err != nil { panic(err) }
	defer cfg.Close(ctx)

	// Enhancement 1: Hot Reloader (integrates with watcher system)
	reloader := cfg.NewHotReloader(500 * time.Millisecond)
	reloader.Start(ctx, eventCh) // eventCh from watcher.Manager
	defer reloader.Stop()

	// Enhancement 2: Conditional Interceptor (extends interceptor system)
	condInterceptor := cfg.NewConditionalInterceptor(nil)
	condInterceptor.RegisterCondition("server.port", interceptor.Conditions.RequireMinValue(1024))
	condInterceptor.RegisterCondition("server.port", interceptor.Conditions.RequireMaxValue(65535))

	// Enhancement 3: Template Processing (uses text/template)
	tpl := cfg.NewTemplateProcessor()
	tpl.RegisterFunc("uppercase", strings.ToUpper)
	result, _ := tpl.Process(ctx, "App name: {{.app.name | uppercase}}")
	fmt.Println(result)

	// Enhancement 4: Migration Manager
	store := version.NewInMemoryVersionStore()
	migrator := cfg.NewMigrationManager(store, "1.0.0")
	migrator.Register(version.Migration{
		FromVersion: "1.0.0",
		ToVersion:   "2.0.0",
		Upgrade: func(ctx context.Context, data map[string]value.Value) (map[string]value.Value, error) {
			data["schema_version"] = value.New("2.0.0")
			return data, nil
		},
	})
}
*/
