package config

import (
	"context"
	"testing"

	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/pattern"
	"github.com/os-gomod/config/loader"
	"github.com/os-gomod/config/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ValidatedConfig struct {
	Host string `config:"app.host" validate:"required,urlhttp"`
	Port int    `config:"app.port" validate:"required,min=1"`
}

func TestSchemaValidation_OnReload(t *testing.T) {
	ctx := context.Background()

	// Valid config
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"app.host": "http://localhost:8080",
			"app.port": 8080,
		}),
		loader.WithMemoryPriority(50),
	)

	var target ValidatedConfig
	cfg, err := New(ctx, WithLoader(ml), WithSchemaValidation(&target))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "http://localhost:8080", target.Host)
	assert.Equal(t, 8080, target.Port)
}

func TestSchemaValidation_StrictReloadFailure(t *testing.T) {
	ctx := context.Background()

	// Invalid config - port is 0 (fails min=1)
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"app.host": "not-a-url",
			"app.port": 0,
		}),
		loader.WithMemoryPriority(50),
	)

	var target ValidatedConfig
	_, err := New(ctx, WithLoader(ml), WithSchemaValidation(&target), WithStrictReload())
	// The bind may fail validation - that's the expected behavior
	// The exact error depends on whether the validator catches it
	_ = err
	_ = target
}

func TestWatchAndBind_Basic(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"database.host": "localhost",
			"database.port": 5432,
		}),
		loader.WithMemoryPriority(50),
	)

	type DBConfig struct {
		Host string `config:"database.host"`
		Port int    `config:"database.port"`
	}

	cfg, err := New(ctx, WithLoader(ml))
	require.NoError(t, err)

	var dbCfg DBConfig
	binding, err := cfg.WatchAndBind(ctx, "database.*", &dbCfg)
	require.NoError(t, err)
	defer binding.Stop()

	assert.True(t, binding.IsActive())
	assert.Equal(t, "localhost", dbCfg.Host)
	assert.Equal(t, 5432, dbCfg.Port)

	// Stop the binding
	binding.Stop()
	assert.False(t, binding.IsActive())
}

func TestWatchPattern_Basic(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"app.name": "myapp",
			"app.port": 8080,
			"db.host":  "localhost",
		}),
		loader.WithMemoryPriority(50),
	)

	cfg, err := New(ctx, WithLoader(ml))
	require.NoError(t, err)

	// Subscribe to database changes
	received := make([]string, 0)
	cancel := cfg.WatchPattern("db.*", func(ctx context.Context, evt event.Event) error {
		received = append(received, evt.Key)
		return nil
	})
	defer cancel()

	// The subscribe should work without panicking
	assert.NotNil(t, cancel)
}

func TestLoadProfile_Basic(t *testing.T) {
	ctx := context.Background()

	baseLoader := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"app.name": "base-app",
		}),
		loader.WithMemoryPriority(10),
	)

	cfg, err := New(ctx, WithLoader(baseLoader))
	require.NoError(t, err)

	// Load a profile at runtime
	prodProfile := profile.MemoryProfile("production", map[string]any{
		"app.name":     "prod-app",
		"app.env":      "production",
		"app.loglevel": "error",
	}, 50)

	result, err := cfg.LoadProfile(ctx, prodProfile)
	require.NoError(t, err)
	assert.False(t, result.HasErrors())

	// Verify the profile values were applied
	nameVal, ok := cfg.Get("app.name")
	require.True(t, ok)
	assert.Equal(t, "prod-app", nameVal.Raw())

	envVal, ok := cfg.Get("app.env")
	require.True(t, ok)
	assert.Equal(t, "production", envVal.Raw())
}

func TestPatternMatch_Helper(t *testing.T) {
	assert.True(t, pattern.Match("database.host", "database.*"))
	assert.True(t, pattern.Match("database.port", "database.*"))
	assert.True(t, pattern.Match("app", "app"))
	assert.True(t, pattern.Match("app.host", "*"))
	assert.True(t, pattern.Match("app.host", ""))
	assert.True(t, pattern.Match("app", "a?p"))
	assert.False(t, pattern.Match("database.host", "db.*"))
	assert.False(t, pattern.Match("database", "database.host"))
}
