package pipeline

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/config/v2/internal/domain/event"
)

// ---------------------------------------------------------------------------
// Pipeline creation
// ---------------------------------------------------------------------------

func TestNew_Pipeline(t *testing.T) {
	p := New()
	require.NotNil(t, p)
	assert.Equal(t, 0, p.MiddlewareCount())
}

func TestNew_WithMiddleware(t *testing.T) {
	var order []string
	mw1 := func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			order = append(order, "mw1-before")
			res, err := next(ctx, cmd)
			order = append(order, "mw1-after")
			return res, err
		}
	}
	mw2 := func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			order = append(order, "mw2-before")
			res, err := next(ctx, cmd)
			order = append(order, "mw2-after")
			return res, err
		}
	}

	p := New(WithMiddleware(mw1, mw2))
	assert.Equal(t, 2, p.MiddlewareCount())

	_, err := p.Run(context.Background(), Command{
		Execute: func(ctx context.Context) (Result, error) {
			order = append(order, "execute")
			return Result{}, nil
		},
	})
	require.NoError(t, err)

	// First middleware is outermost: before runs first, after runs last
	assert.Equal(t, []string{
		"mw1-before", "mw2-before", "execute", "mw2-after", "mw1-after",
	}, order)
}

// ---------------------------------------------------------------------------
// Command execution
// ---------------------------------------------------------------------------

func TestRun_BasicExecution(t *testing.T) {
	p := New()

	result, err := p.Run(context.Background(), Command{
		Name:      "test-cmd",
		Operation: "set",
		Key:       "timeout",
		Execute: func(ctx context.Context) (Result, error) {
			return Result{Events: []event.Event{event.New(event.TypeCreate, "timeout")}}, nil
		},
	})
	require.NoError(t, err)
	assert.Len(t, result.Events, 1)
	assert.False(t, result.Skipped)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestRun_NoExecute(t *testing.T) {
	p := New()

	result, err := p.Run(context.Background(), Command{})
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Equal(t, "no execute function provided", result.SkipReason)
}

// ---------------------------------------------------------------------------
// Error propagation
// ---------------------------------------------------------------------------

func TestRun_ErrorPropagation(t *testing.T) {
	p := New()

	_, err := p.Run(context.Background(), Command{
		Operation: "set",
		Execute: func(ctx context.Context) (Result, error) {
			return Result{}, assert.AnError
		},
	})
	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestRun_MiddlewareError(t *testing.T) {
	errMW := func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			return Result{}, assert.AnError
		}
	}

	p := New(WithMiddleware(errMW))
	_, err := p.Run(context.Background(), Command{
		Execute: func(ctx context.Context) (Result, error) {
			return Result{}, nil
		},
	})
	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

// ---------------------------------------------------------------------------
// Recovery from panics
// ---------------------------------------------------------------------------

func TestRecoveryMiddleware(t *testing.T) {
	p := New(WithMiddleware(RecoveryMiddleware()))

	_, err := p.Run(context.Background(), Command{
		Name:      "panic-cmd",
		Operation: "set",
		Execute: func(ctx context.Context) (Result, error) {
			panic("test panic")
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panic recovered")
	assert.Contains(t, err.Error(), "panic-cmd")
}

// ---------------------------------------------------------------------------
// Correlation ID injection
// ---------------------------------------------------------------------------

func TestCorrelationIDMiddleware(t *testing.T) {
	p := New(WithMiddleware(CorrelationIDMiddleware()))

	var capturedID string
	_, err := p.Run(context.Background(), Command{
		Operation: "set",
		Execute: func(ctx context.Context) (Result, error) {
			capturedID = CorrelationIDFromContext(ctx)
			return Result{}, nil
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, capturedID)
	assert.Contains(t, capturedID, "set-")
}

func TestCorrelationIDFromContext_Empty(t *testing.T) {
	id := CorrelationIDFromContext(context.Background())
	assert.Empty(t, id)
}

func TestContextWithCorrelationID(t *testing.T) {
	ctx := ContextWithCorrelationID(context.Background(), "test-id")
	assert.Equal(t, "test-id", CorrelationIDFromContext(ctx))
}

func TestCorrelationIDMiddleware_PreservesExisting(t *testing.T) {
	p := New(WithMiddleware(CorrelationIDMiddleware()))

	ctx := ContextWithCorrelationID(context.Background(), "existing-id")
	var capturedID string

	_, err := p.Run(ctx, Command{
		Execute: func(ctx context.Context) (Result, error) {
			capturedID = CorrelationIDFromContext(ctx)
			return Result{}, nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "existing-id", capturedID, "should preserve existing correlation ID")
}

// ---------------------------------------------------------------------------
// Use (dynamic middleware registration)
// ---------------------------------------------------------------------------

func TestUse_DynamicRegistration(t *testing.T) {
	p := New()
	assert.Equal(t, 0, p.MiddlewareCount())

	var called atomic.Bool
	mw := func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			called.Store(true)
			return next(ctx, cmd)
		}
	}

	p.Use(mw)
	assert.Equal(t, 1, p.MiddlewareCount())

	_, err := p.Run(context.Background(), Command{
		Execute: func(ctx context.Context) (Result, error) {
			return Result{}, nil
		},
	})
	require.NoError(t, err)
	assert.True(t, called.Load())
}

// ---------------------------------------------------------------------------
// MetricsRecorder integration
// ---------------------------------------------------------------------------

func TestMetricsRecorder_Called(t *testing.T) {
	var recorded atomic.Int32
	recorder := &testMetricsRecorder{
		onRecord: func(ctx context.Context, op string, duration time.Duration, err error) {
			recorded.Add(1)
		},
	}

	p := New(WithMetricsRecorder(recorder))

	_, _ = p.Run(context.Background(), Command{
		Operation: "set",
		Execute: func(ctx context.Context) (Result, error) {
			return Result{}, nil
		},
	})
	assert.Equal(t, int32(1), recorded.Load())
}

// ---------------------------------------------------------------------------
// Command fields
// ---------------------------------------------------------------------------

func TestCommand_Fields(t *testing.T) {
	cmd := Command{
		Name:      "batch-set",
		Operation: "batch_set",
		Key:       "key",
		Value:     "value",
		Values:    map[string]any{"a": 1, "b": 2},
	}
	assert.Equal(t, "batch-set", cmd.Name)
	assert.Equal(t, "batch_set", cmd.Operation)
	assert.Equal(t, "key", cmd.Key)
	assert.Equal(t, "value", cmd.Value)
	assert.Len(t, cmd.Values, 2)
}

// ---------------------------------------------------------------------------
// Result fields
// ---------------------------------------------------------------------------

func TestResult_AppendEvents(t *testing.T) {
	r := &Result{}
	assert.Empty(t, r.Events)

	r.Events = append(r.Events, event.New(event.TypeCreate, "a"))
	r.Events = append(r.Events, event.New(event.TypeUpdate, "b"))
	assert.Len(t, r.Events, 2)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type testMetricsRecorder struct {
	onRecord func(ctx context.Context, op string, duration time.Duration, err error)
}

func (t *testMetricsRecorder) RecordOperation(ctx context.Context, op string, duration time.Duration, err error) {
	if t.onRecord != nil {
		t.onRecord(ctx, op, duration, err)
	}
}
