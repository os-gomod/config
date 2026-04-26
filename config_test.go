package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/layer"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/loader"
)

// ---------------------------------------------------------------------------
// New config creation
// ---------------------------------------------------------------------------

func TestNew_Config(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	defer cfg.Close(context.Background())

	// Should be able to call basic operations
	assert.Equal(t, 0, cfg.Len())
	assert.Empty(t, cfg.Keys())
}

// ---------------------------------------------------------------------------
// Get / Set / Delete
// ---------------------------------------------------------------------------

func TestConfig_Get_Nonexistent(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	_, ok := cfg.Get("nonexistent")
	assert.False(t, ok)
}

func TestConfig_SetAndGet(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	err = cfg.Set(context.Background(), "host", "localhost")
	require.NoError(t, err)

	v, ok := cfg.Get("host")
	require.True(t, ok)
	assert.Equal(t, "localhost", v.String())
}

func TestConfig_Delete(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	err = cfg.Set(context.Background(), "temp", "value")
	require.NoError(t, err)

	err = cfg.Delete(context.Background(), "temp")
	require.NoError(t, err)

	_, ok := cfg.Get("temp")
	assert.False(t, ok)
}

func TestConfig_Has(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	assert.False(t, cfg.Has("missing"))

	err = cfg.Set(context.Background(), "exists", "yes")
	require.NoError(t, err)
	assert.True(t, cfg.Has("exists"))
}

// ---------------------------------------------------------------------------
// GetAll
// ---------------------------------------------------------------------------

func TestConfig_GetAll(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	err = cfg.Set(context.Background(), "a", 1)
	require.NoError(t, err)
	err = cfg.Set(context.Background(), "b", 2)
	require.NoError(t, err)

	all := cfg.GetAll()
	assert.Len(t, all, 2)
	assert.Equal(t, 1, all["a"].Int())
	assert.Equal(t, 2, all["b"].Int())
}

// ---------------------------------------------------------------------------
// BatchSet
// ---------------------------------------------------------------------------

func TestConfig_BatchSet(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	err = cfg.BatchSet(context.Background(), map[string]any{
		"x": 1,
		"y": "hello",
		"z": true,
	})
	require.NoError(t, err)

	vx, _ := cfg.Get("x")
	assert.Equal(t, 1, vx.Int())
	vy, _ := cfg.Get("y")
	assert.Equal(t, "hello", vy.String())
	vz, _ := cfg.Get("z")
	assert.True(t, vz.Bool())
}

// ---------------------------------------------------------------------------
// Reload
// ---------------------------------------------------------------------------

func TestConfig_Reload(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	result, err := cfg.Reload(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.HasErrors())
}

// ---------------------------------------------------------------------------
// Subscribe
// ---------------------------------------------------------------------------

func TestConfig_Subscribe(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	receivedKey := make(chan string, 1)
	unsub := cfg.Subscribe(func(ctx context.Context, evt event.Event) error {
		select {
		case receivedKey <- evt.Key:
		default:
		}
		return nil
	})
	require.NotNil(t, unsub)

	// Publish an event by setting a value
	err = cfg.Set(context.Background(), "test.key", "value")
	require.NoError(t, err)

	select {
	case key := <-receivedKey:
		assert.Equal(t, "test.key", key)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subscription callback")
	}

	unsub()
}

// ---------------------------------------------------------------------------
// Keys / Len
// ---------------------------------------------------------------------------

func TestConfig_Keys(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	cfg.Set(context.Background(), "z", 1)
	cfg.Set(context.Background(), "a", 2)
	cfg.Set(context.Background(), "m", 3)

	keys := cfg.Keys()
	assert.Equal(t, []string{"a", "m", "z"}, keys)
}

func TestConfig_Len(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	assert.Equal(t, 0, cfg.Len())

	cfg.Set(context.Background(), "a", 1)
	assert.Equal(t, 1, cfg.Len())

	cfg.Set(context.Background(), "b", 2)
	assert.Equal(t, 2, cfg.Len())
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestConfig_Close(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)

	err = cfg.Close(context.Background())
	require.NoError(t, err)
}

func TestConfig_Close_Idempotent(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)

	err = cfg.Close(context.Background())
	require.NoError(t, err)
	err = cfg.Close(context.Background())
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------------

func TestConfig_Snapshot(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	cfg.Set(context.Background(), "password", "secret123")
	cfg.Set(context.Background(), "host", "localhost")

	snap := cfg.Snapshot()
	require.NotNil(t, snap)
	assert.Equal(t, "localhost", snap["host"].String())
	// Secrets should be redacted in snapshot
	assert.Equal(t, "***REDACTED***", snap["password"].String())
}

// ---------------------------------------------------------------------------
// Plugins
// ---------------------------------------------------------------------------

func TestConfig_Plugins_Empty(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	assert.Empty(t, cfg.Plugins())
}

// ---------------------------------------------------------------------------
// Namespace
// ---------------------------------------------------------------------------

func TestConfig_Namespace(t *testing.T) {
	cfg, err := New(context.Background(), WithNamespace("app."))
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	assert.Equal(t, "app.", cfg.Namespace())
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestConfig_Validate(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	err = cfg.Validate(context.Background(), struct{}{})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------------

func TestConfig_Restore(t *testing.T) {
	cfg, err := New(context.Background())
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	cfg.Set(context.Background(), "original", 1)
	vOrig, _ := cfg.Get("original")
	assert.Equal(t, 1, vOrig.Int())

	newData := map[string]value.Value{
		"restored": value.New(42),
	}
	cfg.Restore(newData)
	vRestored, _ := cfg.Get("restored")
	assert.Equal(t, 42, vRestored.Int())
}

func TestConfig_FileLayerOverridesMemoryDefaultsWithFlattenedKeys(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
app:
  name: from-file
server:
  port: 9090
`), 0o600))

	defaults := layer.NewStaticLayer("memory-defaults", map[string]value.Value{
		"app.name":    value.NewInMemory("from-memory", 10),
		"server.port": value.NewInMemory(3000, 10),
	}, layer.WithPriority(10))

	fileData, err := loader.NewFileLoader("yaml-config", []string{path}, nil).Load(context.Background())
	require.NoError(t, err)

	fileLayer := layer.NewStaticLayer("yaml-file", fileData, layer.WithPriority(30))

	cfg, err := New(context.Background(), WithLayers(defaults, fileLayer))
	require.NoError(t, err)
	defer cfg.Close(context.Background())

	appName, ok := cfg.Get("app.name")
	require.True(t, ok)
	assert.Equal(t, "from-file", appName.Raw())
	assert.Equal(t, 30, appName.Priority())

	port, ok := cfg.Get("server.port")
	require.True(t, ok)
	assert.Equal(t, 9090, port.Raw())
	assert.Equal(t, 30, port.Priority())

	explain := cfg.Explain("app.name")
	assert.Contains(t, explain, `source=yaml-file`)
	assert.Contains(t, explain, `priority=30`)
	assert.True(t, strings.Contains(explain, `value=from-file`), explain)
}
