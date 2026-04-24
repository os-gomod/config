package config

import (
	"context"
	"testing"

	"github.com/os-gomod/config/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespace_Isolation(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"app.name":               "myapp",
			"app.port":               8080,
			"tenant.acme.app.name":   "acme-app",
			"tenant.acme.app.port":   9090,
			"tenant.globex.app.name": "globex-app",
		}),
		loader.WithMemoryPriority(50),
	)

	// Without namespace
	cfg1, err := New(ctx, WithLoader(ml))
	require.NoError(t, err)

	v, ok := cfg1.Get("app.name")
	assert.True(t, ok)
	assert.Equal(t, "myapp", v.String())
	assert.Equal(t, "", cfg1.Namespace())

	// With namespace "tenant.acme."
	cfg2, err := New(ctx, WithLoader(ml), WithNamespace("tenant.acme."))
	require.NoError(t, err)
	assert.Equal(t, "tenant.acme.", cfg2.Namespace())

	v, ok = cfg2.Get("app.name")
	assert.True(t, ok)
	assert.Equal(t, "acme-app", v.String())

	v, ok = cfg2.Get("app.port")
	assert.True(t, ok)
	assert.Equal(t, "9090", v.String())

	// With namespace "tenant.globex."
	cfg3, err := New(ctx, WithLoader(ml), WithNamespace("tenant.globex."))
	require.NoError(t, err)
	assert.Equal(t, "tenant.globex.", cfg3.Namespace())

	v, ok = cfg3.Get("app.name")
	assert.True(t, ok)
	assert.Equal(t, "globex-app", v.String())
}

func TestSetNamespace_RuntimeSwitch(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"tenant.a.key": "value-a",
			"tenant.b.key": "value-b",
		}),
		loader.WithMemoryPriority(50),
	)

	cfg, err := New(ctx, WithLoader(ml), WithNamespace("tenant.a."))
	require.NoError(t, err)

	v, ok := cfg.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value-a", v.String())

	// Switch namespace at runtime
	err = cfg.SetNamespace(ctx, "tenant.b.")
	require.NoError(t, err)
	assert.Equal(t, "tenant.b.", cfg.Namespace())

	v, ok = cfg.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value-b", v.String())
}

func TestNamespace_EmptyString(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"app.host": "localhost",
		}),
		loader.WithMemoryPriority(50),
	)

	cfg, err := New(ctx, WithLoader(ml), WithNamespace(""))
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Namespace())

	v, ok := cfg.Get("app.host")
	assert.True(t, ok)
	assert.Equal(t, "localhost", v.String())
}

func TestNamespace_Has(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"tenant.acme.feature.flag": true,
		}),
		loader.WithMemoryPriority(50),
	)

	cfg, err := New(ctx, WithLoader(ml), WithNamespace("tenant.acme."))
	require.NoError(t, err)

	assert.True(t, cfg.Has("feature.flag"))
	assert.False(t, cfg.Has("nonexistent.key"))
}

func TestNamespace_SameNamespaceNoReload(t *testing.T) {
	ctx := context.Background()
	ml := loader.NewMemoryLoader(
		loader.WithMemoryData(map[string]any{
			"tenant.acme.key": "val",
		}),
		loader.WithMemoryPriority(50),
	)

	cfg, err := New(ctx, WithLoader(ml), WithNamespace("tenant.acme."))
	require.NoError(t, err)

	// Setting same namespace should be a no-op (no error, no reload)
	err = cfg.SetNamespace(ctx, "tenant.acme.")
	require.NoError(t, err)
	assert.Equal(t, "tenant.acme.", cfg.Namespace())
}
