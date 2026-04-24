package providerbase

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ------------------------------------------------------------------
// RemoteProvider creation
// ------------------------------------------------------------------
func TestRemoteProvider_New(t *testing.T) {
	rp := New(Config[int]{
		Name:     "test",
		Priority: 100,
		FetchFn: func(ctx context.Context, client int) (map[string]value.Value, error) {
			return map[string]value.Value{"k": value.NewInMemory("v")}, nil
		},
	})
	if rp == nil {
		t.Fatal("expected non-nil RemoteProvider")
	}
	assert.Equal(t, "test", rp.Name())
	assert.Equal(t, 100, rp.Priority())
	assert.Equal(t, "test", rp.String())
}

func TestRemoteProvider_New_StringFormat(t *testing.T) {
	rp := New(Config[string]{
		Name:         "test",
		StringFormat: "custom:test",
		FetchFn:      func(ctx context.Context, client string) (map[string]value.Value, error) { return nil, nil },
	})
	assert.Equal(t, "custom:test", rp.String())
}

func TestRemoteProvider_New_PanicsOnNilFetchFn(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil FetchFn")
		}
	}()
	New(Config[int]{})
}

// ------------------------------------------------------------------
// Lazy init
// ------------------------------------------------------------------
func TestRemoteProvider_EnsureClient_Lazy(t *testing.T) {
	initCalled := false
	rp := New(Config[string]{
		Name: "lazy",
		InitFn: func() (string, error) {
			initCalled = true
			return "initialized", nil
		},
		FetchFn: func(ctx context.Context, client string) (map[string]value.Value, error) {
			return map[string]value.Value{"client": value.NewInMemory(client)}, nil
		},
	})

	// Before any operation, init should not be called
	if initCalled {
		t.Error("InitFn should not be called eagerly")
	}

	// Load should trigger lazy init
	data, err := rp.Load(context.Background())
	require.NoError(t, err)
	require.True(t, initCalled, "InitFn should have been called")
	assert.Equal(t, "initialized", data["client"].String())
}

func TestRemoteProvider_EnsureClient_NoInitFn(t *testing.T) {
	rp := New(Config[int]{
		Name:    "noinit",
		FetchFn: func(ctx context.Context, client int) (map[string]value.Value, error) { return nil, nil },
	})

	_, err := rp.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no init function")
}

// ------------------------------------------------------------------
// Close
// ------------------------------------------------------------------
func TestRemoteProvider_Close(t *testing.T) {
	closedCalled := false
	rp := New(Config[string]{
		Name: "closeable",
		InitFn: func() (string, error) {
			return "client", nil
		},
		CloseFn: func(client string) error {
			closedCalled = true
			return nil
		},
		FetchFn: func(ctx context.Context, client string) (map[string]value.Value, error) {
			return nil, nil
		},
	})

	// Load to trigger init
	_, err := rp.Load(context.Background())
	require.NoError(t, err)

	err = rp.Close(context.Background())
	require.NoError(t, err)
	assert.True(t, closedCalled, "CloseFn should have been called")
}

func TestRemoteProvider_Close_NilCloseFn(t *testing.T) {
	rp := New(Config[int]{
		Name:    "noclose",
		FetchFn: func(ctx context.Context, client int) (map[string]value.Value, error) { return nil, nil },
	})

	err := rp.Close(context.Background())
	require.NoError(t, err)
}

func TestRemoteProvider_Close_DoubleClose(t *testing.T) {
	rp := New(Config[int]{
		Name:    "doubleclose",
		FetchFn: func(ctx context.Context, client int) (map[string]value.Value, error) { return nil, nil },
	})

	require.NoError(t, rp.Close(context.Background()))
	require.NoError(t, rp.Close(context.Background()))
}

// ------------------------------------------------------------------
// Health
// ------------------------------------------------------------------
func TestRemoteProvider_Health_NilHealthFn(t *testing.T) {
	rp := New(Config[int]{
		Name:    "nohealth",
		FetchFn: func(ctx context.Context, client int) (map[string]value.Value, error) { return nil, nil },
	})

	// With no InitFn, Health should fail (client not initialized)
	err := rp.Health(context.Background())
	require.Error(t, err)
}

// ------------------------------------------------------------------
// Watch
// ------------------------------------------------------------------
func TestRemoteProvider_Watch_NotSupported(t *testing.T) {
	rp := New(Config[int]{
		Name:    "nowatch",
		FetchFn: func(ctx context.Context, client int) (map[string]value.Value, error) { return nil, nil },
	})

	// No PollInterval, no NativeWatchFn -> nil
	ch, err := rp.Watch(context.Background())
	require.NoError(t, err)
	assert.Nil(t, ch)
}

// ------------------------------------------------------------------
// toBackoffConfig
// ------------------------------------------------------------------
func TestRetryConfig_ToBackoffConfig(t *testing.T) {
	rc := RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 50 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      1.5,
		JitterFactor:    0.3,
	}
	bc := rc.toBackoffConfig()
	assert.Equal(t, 3, bc.MaxRetries)
	assert.Equal(t, 50*time.Millisecond, bc.InitialInterval)
	assert.Equal(t, 5*time.Second, bc.MaxInterval)
	assert.Equal(t, 1.5, bc.Multiplier)
	assert.Equal(t, 0.3, bc.JitterFactor)
}
