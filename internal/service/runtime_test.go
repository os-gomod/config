package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/layer"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/eventbus"
	"github.com/os-gomod/config/v2/internal/interceptor"
	"github.com/os-gomod/config/v2/internal/observability"
	"github.com/os-gomod/config/v2/internal/pipeline"
)

// ---------------------------------------------------------------------------
// Test helpers (reuse stubEngine from mutation_test.go since same package)
// ---------------------------------------------------------------------------

func newTestRuntimeService(engine Engine, opts ...RuntimeServiceOption) *RuntimeService {
	bus := eventbus.NewBus(eventbus.WithWorkerCount(1), eventbus.WithQueueSize(100))
	pipe := pipeline.New()
	chain := interceptor.NewChain()
	return NewRuntimeService(pipe, engine, bus, chain, observability.Nop(), opts...)
}

func newTestRuntimeServiceWithChain(engine Engine, chain *interceptor.Chain) *RuntimeService {
	bus := eventbus.NewBus(eventbus.WithWorkerCount(1), eventbus.WithQueueSize(100))
	pipe := pipeline.New()
	return NewRuntimeService(pipe, engine, bus, chain, observability.Nop())
}

// ---------------------------------------------------------------------------
// Reload operation
// ---------------------------------------------------------------------------

func TestRuntimeService_Reload(t *testing.T) {
	engine := newStubEngine(map[string]value.Value{
		"a": value.New(1),
	})
	svc := newTestRuntimeService(engine)

	result, err := svc.Reload(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRuntimeService_Reload_WithEvents(t *testing.T) {
	// Use a reload-producing engine
	engine := &reloadStubEngine{
		stubEngine: newStubEngine(nil),
		events: []event.Event{
			event.New(event.TypeUpdate, "key", event.WithSource("test")),
		},
	}
	svc := newTestRuntimeService(engine)

	result, err := svc.Reload(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Changed)
}

func TestRuntimeService_Reload_BeforeInterceptor_Error(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	chain.AddReloadInterceptor(&interceptor.ReloadFunc{
		BeforeFn: func(ctx context.Context, req *interceptor.ReloadRequest) error {
			return errors.New(errors.CodePermissionDenied, "reload blocked")
		},
	})

	svc := newTestRuntimeServiceWithChain(engine, chain)

	_, err := svc.Reload(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before-reload interceptor failed")
}

func TestRuntimeService_Reload_AfterInterceptor(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	var afterCalled bool
	chain.AddReloadInterceptor(&interceptor.ReloadFunc{
		AfterFn: func(ctx context.Context, req *interceptor.ReloadRequest, res *interceptor.ReloadResponse) error {
			afterCalled = true
			return nil
		},
	})

	svc := newTestRuntimeServiceWithChain(engine, chain)

	_, err := svc.Reload(context.Background())
	require.NoError(t, err)
	assert.True(t, afterCalled)
}

func TestRuntimeService_Reload_OnReloadError(t *testing.T) {
	engine := &errorReloadStubEngine{stubEngine: newStubEngine(nil)}

	var capturedErr error
	svc := newTestRuntimeService(engine, WithOnReloadError(func(err error) {
		capturedErr = err
	}))

	_, err := svc.Reload(context.Background())
	require.Error(t, err)
	assert.NotNil(t, capturedErr)
}

// ---------------------------------------------------------------------------
// Bind operation
// ---------------------------------------------------------------------------

func TestRuntimeService_Bind(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestRuntimeService(engine)

	var target struct{}
	err := svc.Bind(context.Background(), &target)
	require.NoError(t, err)
}

func TestRuntimeService_Bind_BeforeInterceptor_Error(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	chain.AddBindInterceptor(&interceptor.BindFunc{
		BeforeFn: func(ctx context.Context, req *interceptor.BindRequest) error {
			return errors.New(errors.CodeValidation, "invalid target")
		},
	})

	svc := newTestRuntimeServiceWithChain(engine, chain)

	err := svc.Bind(context.Background(), &struct{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before-bind interceptor failed")
}

func TestRuntimeService_Bind_AfterInterceptor(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	var afterCalled bool
	chain.AddBindInterceptor(&interceptor.BindFunc{
		AfterFn: func(ctx context.Context, req *interceptor.BindRequest, res *interceptor.BindResponse) error {
			afterCalled = true
			return nil
		},
	})

	svc := newTestRuntimeServiceWithChain(engine, chain)

	err := svc.Bind(context.Background(), &struct{}{})
	require.NoError(t, err)
	assert.True(t, afterCalled)
}

// ---------------------------------------------------------------------------
// Close operation
// ---------------------------------------------------------------------------

func TestRuntimeService_Close(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestRuntimeService(engine)

	err := svc.Close(context.Background())
	require.NoError(t, err)
	assert.True(t, engine.closed)
}

func TestRuntimeService_Close_BeforeInterceptor_Error(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	chain.AddCloseInterceptor(&interceptor.CloseFunc{
		BeforeFn: func(ctx context.Context, req *interceptor.CloseRequest) error {
			return errors.New(errors.CodeInternal, "close blocked")
		},
	})

	svc := newTestRuntimeServiceWithChain(engine, chain)

	err := svc.Close(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before-close interceptor failed")
}

func TestRuntimeService_Close_AfterInterceptor(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	var afterCalled bool
	chain.AddCloseInterceptor(&interceptor.CloseFunc{
		AfterFn: func(ctx context.Context, req *interceptor.CloseRequest, res *interceptor.CloseResponse) error {
			afterCalled = true
			return nil
		},
	})

	svc := newTestRuntimeServiceWithChain(engine, chain)

	err := svc.Close(context.Background())
	require.NoError(t, err)
	assert.True(t, afterCalled)
}

func TestRuntimeService_Close_ClosesBus(t *testing.T) {
	engine := newStubEngine(nil)
	bus := eventbus.NewBus(eventbus.WithWorkerCount(1), eventbus.WithQueueSize(100))
	pipe := pipeline.New()
	chain := interceptor.NewChain()

	svc := NewRuntimeService(pipe, engine, bus, chain, observability.Nop())

	err := svc.Close(context.Background())
	require.NoError(t, err)

	// Bus should be closed — publishing should fail
	publishErr := bus.Publish(context.Background(), &event.Event{Key: "test"})
	require.Error(t, publishErr)
	assert.Contains(t, publishErr.Error(), "bus is closed")
}

// ---------------------------------------------------------------------------
// ReloadResult
// ---------------------------------------------------------------------------

func TestReloadResult_HasErrors(t *testing.T) {
	t.Run("no_errors", func(t *testing.T) {
		r := ReloadResult{}
		assert.False(t, r.HasErrors())
	})

	t.Run("with_errors", func(t *testing.T) {
		r := ReloadResult{
			LayerErrs: []layer.LayerError{
				{LayerName: "file", Err: errors.New("io error", "read failed")},
			},
		}
		assert.True(t, r.HasErrors())
	})
}

// ---------------------------------------------------------------------------
// Stub engines for specific test scenarios
// ---------------------------------------------------------------------------

// reloadStubEngine returns events on Reload.
type reloadStubEngine struct {
	*stubEngine
	events []event.Event
}

func (e *reloadStubEngine) Reload(ctx context.Context) ([]event.Event, []layer.LayerError, error) {
	return e.events, nil, nil
}

// errorReloadStubEngine returns an error on Reload.
type errorReloadStubEngine struct {
	*stubEngine
}

func (e *errorReloadStubEngine) Reload(ctx context.Context) ([]event.Event, []layer.LayerError, error) {
	return nil, nil, errors.New(errors.CodeSource, "reload failed")
}
