//go:build ignore

// full_example demonstrates the full feature set of the config library
// including multi-tenancy, feature flags, schema validation,
// delta reload, and watch-and-bind.
//
// Run from the project root:
//
//	go run example/full_example.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/featureflag"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/profile"
)

// AppConfig is the main application configuration with validation tags.
type AppConfig struct {
	Host     string `config:"app.host"     validate:"required"`
	Port     int    `config:"app.port"     validate:"required,min=1,max=65535"`
	LogLevel string `config:"app.log_level" validate:"required,oneof=debug info warn error"`
	Debug    bool   `config:"app.debug"`
}

func main() {
	ctx := context.Background()

	// --- Layer 1: File defaults ---
	fileLoader := loader.NewFileLoader("example/config.yaml",
		loader.WithFilePriority(10),
	)

	// --- Layer 2: Environment variables (override file) ---
	envLoader := loader.NewEnvLoader(
		loader.WithEnvPrefix("MYAPP"),
		loader.WithEnvPriority(40),
	)

	// --- Layer 3: Memory overrides (highest priority) ---
	memoryLoader := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"app.host":      "http://localhost:8080",
			"app.port":      8080,
			"app.log_level": "info",
			"app.debug":     false,
			// Feature flags
			"feature.new_ui":       true,
			"feature.beta_rollout": 25,
			"feature.experiment":   "control,treatment",
		}),
		loader.WithMemoryPriority(100),
	)

	// --- Create Config with enterprise features ---
	var appCfg AppConfig
	cfg, err := config.New(ctx,
		config.WithLoader(fileLoader),
		config.WithLoader(envLoader),
		config.WithLoader(memoryLoader),
		config.WithSchemaValidation(&appCfg),
		config.WithDeltaReload(),
		config.WithDebounce(200*time.Millisecond),
	)
	if err != nil {
		if cfg == nil {
			log.Fatalf("config init failed: %v", err)
		}
		log.Printf("config init warning: %v", err)
	}
	defer cfg.Close(ctx)

	// Explicitly bind when initial reload had layer warnings
	// (WithSchemaValidation skips binding on reload with errors).
	if err := cfg.Bind(ctx, &appCfg); err != nil {
		log.Printf("bind warning: %v", err)
	}

	fmt.Printf("Application: %s:%d (log: %s, debug: %v)\n",
		appCfg.Host, appCfg.Port, appCfg.LogLevel, appCfg.Debug)

	// --- Multi-tenancy example ---
	fmt.Println("\n--- Multi-Tenancy ---")
	tenantLoader := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"tenant.acme.app.host": "http://acme.internal:9090",
			"tenant.acme.app.port": 9090,
		}),
		loader.WithMemoryPriority(100),
	)
	nsCfg, err := config.New(ctx,
		config.WithLoader(tenantLoader),
		config.WithNamespace("tenant.acme."),
	)
	if err != nil {
		log.Printf("namespace config error: %v", err)
	} else {
		defer nsCfg.Close(ctx)
		if v, ok := nsCfg.Get("app.host"); ok {
			fmt.Printf("Tenant 'acme' host: %s\n", v.String())
		}
		if v, ok := nsCfg.Get("app.port"); ok {
			fmt.Printf("Tenant 'acme' port: %v\n", v.Raw())
		}
	}

	// --- Feature flags example ---
	fmt.Println("\n--- Feature Flags ---")
	ffEngine := featureflag.NewEngine(cfg, "feature.")

	// Boolean flag
	if ffEngine.IsEnabled(ctx, "new_ui") {
		fmt.Println("New UI is enabled")
	}

	// Percentage rollout
	users := []string{"alice", "bob", "charlie", "dave", "eve"}
	for _, user := range users {
		if ffEngine.IsEnabledFor(ctx, "beta_rollout",
			&featureflag.EvalContext{Identifier: user},
		) {
			fmt.Printf("  User '%s' is in beta rollout\n", user)
		}
	}

	// Variant (A/B test)
	eval := ffEngine.Evaluate(ctx, "experiment",
		&featureflag.EvalContext{Identifier: "session-123"},
	)
	if eval.Enabled {
		fmt.Printf("  Experiment variant: %s (rule: %s)\n", eval.Variant, eval.MatchedRule)
	}

	// --- Watch and Bind example ---
	fmt.Println("\n--- Watch & Bind ---")
	type DBConfig struct {
		Host string `config:"app.host"`
		Port int    `config:"app.port"`
	}
	var dbCfg DBConfig
	binding, err := cfg.WatchAndBind(ctx, "app.*", &dbCfg)
	if err != nil {
		log.Printf("watch-bind error: %v", err)
	} else {
		fmt.Printf("DB config: %s:%d\n", dbCfg.Host, dbCfg.Port)
		binding.Stop()
	}

	// --- Explain & Snapshot ---
	fmt.Println("\n--- Explain & Snapshot ---")
	fmt.Println(cfg.Explain("app.host"))

	snapshot := cfg.Snapshot()
	fmt.Printf("Snapshot has %d keys (secrets redacted)\n", len(snapshot))

	// --- Profile loading ---
	fmt.Println("\n--- Profile Loading ---")
	devOverride := profile.MemoryProfile("dev-override", map[string]any{
		"app.log_level": "debug",
		"app.debug":     true,
	}, 90)
	fmt.Printf("Profile: name=%s layers=%d\n", devOverride.Name, len(devOverride.Layers))
	fmt.Printf("  Layer[0]: name=%s priority=%d\n",
		devOverride.Layers[0].Name, devOverride.Layers[0].Priority)

	// Apply profile to a fresh engine
	if err := devOverride.Apply(cfg.Engine); err != nil {
		log.Printf("profile apply error: %v", err)
	} else {
		_, reloadErr := cfg.Reload(ctx)
		if reloadErr != nil {
			log.Printf("profile reload error: %v", reloadErr)
		} else {
			if v, ok := cfg.Get("app.log_level"); ok {
				fmt.Printf("  After profile: app.log_level = %v\n", v.Raw())
			}
			if v, ok := cfg.Get("app.debug"); ok {
				fmt.Printf("  After profile: app.debug = %v\n", v.Raw())
			}
		}
	}

	// --- Delta reload demo ---
	fmt.Println("\n--- Delta Reload ---")
	// A second reload with no changes should be a no-op (delta reload enabled)
	result, err := cfg.Reload(ctx)
	if err != nil {
		log.Printf("delta reload error: %v", err)
	} else {
		fmt.Printf("  Delta reload: events=%d\n", len(result.Events))
	}

	fmt.Println("\nAll enterprise features demonstrated successfully!")
}
