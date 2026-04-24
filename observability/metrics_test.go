package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTest = errors.New("test error")

func TestNopRecorder(t *testing.T) {
	r := NopRecorder{}
	ctx := context.Background()

	// All methods should not panic
	r.RecordLoad(ctx, "source", time.Second, nil)
	r.RecordLoad(ctx, "source", time.Second, errTest)
	r.RecordReload(ctx, time.Second, 5, nil)
	r.RecordReload(ctx, time.Second, 5, errTest)
	r.RecordSet(ctx, "key", time.Second, nil)
	r.RecordSet(ctx, "key", time.Second, errTest)
	r.RecordBatchSet(ctx, time.Second, nil)
	r.RecordBatchSet(ctx, time.Second, errTest)
	r.RecordDelete(ctx, "key", time.Second, nil)
	r.RecordDelete(ctx, "key", time.Second, errTest)
	r.RecordBind(ctx, time.Second, nil)
	r.RecordBind(ctx, time.Second, errTest)
	r.RecordHook(ctx, "hook", time.Second, nil)
	r.RecordHook(ctx, "hook", time.Second, errTest)
	r.RecordLayerLoad(ctx, "layer", time.Second, 10, nil)
	r.RecordLayerLoad(ctx, "layer", time.Second, 10, errTest)
	r.RecordValidation(ctx, time.Second, nil)
	r.RecordValidation(ctx, time.Second, errTest)
	r.RecordWatchEvent(ctx, "source")
	r.RecordSecretRedacted(ctx, "source")
	r.RecordConfigChangeEvent(ctx, "type", "source")
}

func TestNopFunction(t *testing.T) {
	r := Nop()
	if r == nil {
		t.Fatal("expected non-nil recorder")
	}
}

// ------------------------------------------------------------------
// OTelRecorder - nil meter/tracer (error paths)
// ------------------------------------------------------------------
func TestOTelRecorderNilMeter(t *testing.T) {
	_, err := NewOTelRecorder(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil meter")
	}
}

func TestOTelRecorderNilTracer(t *testing.T) {
	// We can't easily create a real meter in a test without OTel SDK,
	// so we test the nil tracer error path
	_, err := NewOTelRecorder(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "meter")
}

// ------------------------------------------------------------------
// AtomicMetrics - all Record methods
// ------------------------------------------------------------------
func TestAtomicMetrics_AllRecordMethods(t *testing.T) {
	m := &AtomicMetrics{}
	ctx := context.Background()

	// RecordLoad with success
	m.RecordLoad(ctx, "file", time.Millisecond, nil)
	assert.Equal(t, int64(1), m.Loads.Load())
	assert.Equal(t, int64(0), m.LoadErrors.Load())

	// RecordLoad with error
	m.RecordLoad(ctx, "file", time.Millisecond, errTest)
	assert.Equal(t, int64(2), m.Loads.Load())
	assert.Equal(t, int64(1), m.LoadErrors.Load())
	assert.Equal(t, int64(1), m.Errors.Load())

	// RecordReload with success
	m.RecordReload(ctx, 50*time.Millisecond, 10, nil)
	assert.Equal(t, int64(1), m.Reloads.Load())
	assert.Equal(t, int64(10), m.KeyCount.Load())
	assert.Equal(t, int64(50*time.Millisecond), m.LastReloadDuration.Load())

	// RecordReload with error
	m.RecordReload(ctx, 10*time.Millisecond, 0, errTest)
	assert.Equal(t, int64(2), m.Reloads.Load())
	assert.Equal(t, int64(1), m.ReloadErrors.Load())

	// RecordSet
	m.RecordSet(ctx, "key", time.Millisecond, nil)
	assert.Equal(t, int64(1), m.Sets.Load())
	m.RecordSet(ctx, "key", time.Millisecond, errTest)
	assert.Equal(t, int64(1), m.SetErrors.Load())

	// RecordBatchSet
	m.RecordBatchSet(ctx, time.Millisecond, nil)
	assert.Equal(t, int64(1), m.BatchSets.Load())
	m.RecordBatchSet(ctx, time.Millisecond, errTest)
	assert.Equal(t, int64(1), m.BatchSetErrors.Load())

	// RecordDelete
	m.RecordDelete(ctx, "key", time.Millisecond, nil)
	assert.Equal(t, int64(1), m.Deletes.Load())
	m.RecordDelete(ctx, "key", time.Millisecond, errTest)
	assert.Equal(t, int64(1), m.DeleteErrors.Load())

	// RecordBind
	m.RecordBind(ctx, time.Millisecond, nil)
	assert.Equal(t, int64(1), m.Binds.Load())

	// RecordHook
	m.RecordHook(ctx, "before_reload", time.Millisecond, nil)
	assert.Equal(t, int64(1), m.HookCalls.Load())
	m.RecordHook(ctx, "after_reload", time.Millisecond, errTest)
	assert.Equal(t, int64(1), m.HookErrors.Load())

	// RecordLayerLoad
	m.RecordLayerLoad(ctx, "layer1", 10*time.Millisecond, 5, nil)
	assert.Equal(t, int64(1), m.LayerLoads.Load())
	m.RecordLayerLoad(ctx, "layer2", 5*time.Millisecond, 0, errTest)
	assert.Equal(t, int64(1), m.LayerLoadErrors.Load())

	// RecordValidation
	m.RecordValidation(ctx, time.Millisecond, nil)
	assert.Equal(t, int64(1), m.ValidationCalls.Load())
	m.RecordValidation(ctx, time.Millisecond, errTest)
	assert.Equal(t, int64(1), m.ValidationErrors.Load())

	// RecordWatchEvent
	m.RecordWatchEvent(ctx, "nats")
	assert.Equal(t, int64(1), m.WatchEvents.Load())

	// RecordSecretRedacted
	m.RecordSecretRedacted(ctx, "explain")
	assert.Equal(t, int64(1), m.SecretsRedacted.Load())

	// RecordConfigChangeEvent
	m.RecordConfigChangeEvent(ctx, "create", "file")
	assert.Equal(t, int64(1), m.ConfigChangeEvents.Load())

	// Verify TotalDuration accumulated
	assert.True(t, m.TotalDuration.Load() > 0, "expected total duration > 0")
}

// ------------------------------------------------------------------
// Snapshot
// ------------------------------------------------------------------
func TestAtomicMetrics_Snapshot(t *testing.T) {
	m := &AtomicMetrics{}
	m.RecordLoad(context.Background(), "file", 10*time.Millisecond, nil)
	m.RecordReload(context.Background(), 50*time.Millisecond, 5, nil)
	m.RecordSecretRedacted(context.Background(), "loader")
	m.RecordConfigChangeEvent(context.Background(), "db.host", "file")
	m.RecordWatchEvent(context.Background(), "nats")
	m.RecordValidation(context.Background(), time.Millisecond, nil)

	snap := m.Snapshot()

	// Verify all expected keys exist
	expectedKeys := []string{
		"loads", "reloads", "sets", "batch_sets", "deletes",
		"binds", "hook_calls", "errors", "load_errors", "reload_errors",
		"set_errors", "batch_set_errors", "delete_errors", "hook_errors",
		"layer_loads", "layer_load_errors", "events_published",
		"events_dispatched", "total_duration_ns", "last_reload_ns",
		"last_state_version", "key_count", "validation_calls",
		"validation_errors", "watch_events", "secrets_redacted",
		"config_change_events",
	}
	for _, k := range expectedKeys {
		_, ok := snap[k]
		assert.True(t, ok, "snapshot should contain key %q", k)
	}

	// Verify some values
	assert.Equal(t, int64(1), snap["loads"])
	assert.Equal(t, int64(1), snap["reloads"])
	assert.Equal(t, int64(1), snap["secrets_redacted"])
	assert.Equal(t, int64(1), snap["config_change_events"])
	assert.Equal(t, int64(1), snap["watch_events"])
	assert.Equal(t, int64(1), snap["validation_calls"])
	assert.Equal(t, int64(0), snap["validation_errors"])
}

// ------------------------------------------------------------------
// Reset
// ------------------------------------------------------------------
func TestAtomicMetrics_Reset(t *testing.T) {
	m := &AtomicMetrics{}
	m.RecordLoad(context.Background(), "file", time.Millisecond, nil)
	m.RecordReload(context.Background(), time.Millisecond, 5, nil)
	m.RecordSecretRedacted(context.Background(), "src")

	m.Reset()

	snap := m.Snapshot()
	for _, v := range snap {
		assert.Equal(t, int64(0), v, "expected all counters to be 0 after reset")
	}
}

// ------------------------------------------------------------------
// NopRecorder NewMethods
// ------------------------------------------------------------------
func TestNopRecorder_NewMethods(t *testing.T) {
	r := NopRecorder{}

	// These should not panic
	assert.NotPanics(t, func() {
		r.RecordSecretRedacted(context.Background(), "source")
	})
	assert.NotPanics(t, func() {
		r.RecordConfigChangeEvent(context.Background(), "type", "source")
	})

	// Also verify existing methods don't panic
	assert.NotPanics(t, func() {
		r.RecordLoad(context.Background(), "file", time.Millisecond, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordReload(context.Background(), time.Millisecond, 0, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordSet(context.Background(), "key", time.Millisecond, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordBatchSet(context.Background(), time.Millisecond, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordDelete(context.Background(), "key", time.Millisecond, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordBind(context.Background(), time.Millisecond, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordHook(context.Background(), "hook", time.Millisecond, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordLayerLoad(context.Background(), "layer", time.Millisecond, 0, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordValidation(context.Background(), time.Millisecond, nil)
	})
	assert.NotPanics(t, func() {
		r.RecordWatchEvent(context.Background(), "source")
	})
}

// ------------------------------------------------------------------
// LastStateVersion overflow handling in Snapshot
// ------------------------------------------------------------------
func TestAtomicMetrics_Snapshot_LastStateVersion(t *testing.T) {
	m := &AtomicMetrics{}
	// Set a value that exceeds int64 max
	m.LastStateVersion.Store(^uint64(0)) // max uint64
	snap := m.Snapshot()
	// Should be clamped to max int64
	assert.Equal(t, int64(^uint64(0)>>1), snap["last_state_version"])
}
