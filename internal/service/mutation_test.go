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
// Stub engine for testing
// ---------------------------------------------------------------------------

type stubEngine struct {
	data    map[string]value.Value
	version uint64
	closed  bool
}

func newStubEngine(data map[string]value.Value) *stubEngine {
	if data == nil {
		data = make(map[string]value.Value)
	}
	return &stubEngine{
		data: value.Copy(data),
	}
}

func (e *stubEngine) Get(key string) (value.Value, bool) {
	v, ok := e.data[key]
	return v, ok
}

func (e *stubEngine) GetAll() map[string]value.Value {
	return value.Copy(e.data)
}

func (e *stubEngine) Has(key string) bool {
	_, ok := e.data[key]
	return ok
}

func (e *stubEngine) State() *value.State {
	return value.NewState(e.data)
}

func (e *stubEngine) Version() uint64 { return e.version }

func (e *stubEngine) Len() int { return len(e.data) }

func (e *stubEngine) Keys() []string { return value.SortedKeys(e.data) }

func (e *stubEngine) Set(ctx context.Context, key string, raw any) (event.Event, error) {
	newVal := value.NewInMemory(raw, 100)
	oldVal, exists := e.data[key]
	e.data[key] = newVal
	e.version++
	if exists {
		return event.NewUpdateEvent(key, oldVal, newVal), nil
	}
	return event.NewCreateEvent(key, newVal), nil
}

func (e *stubEngine) Delete(ctx context.Context, key string) (event.Event, error) {
	oldVal, exists := e.data[key]
	if !exists {
		return event.Event{}, nil
	}
	delete(e.data, key)
	e.version++
	return event.NewDeleteEvent(key, oldVal), nil
}

func (e *stubEngine) BatchSet(ctx context.Context, kv map[string]any) ([]event.Event, error) {
	var events []event.Event
	for k, v := range kv {
		evt, err := e.Set(ctx, k, v)
		if err != nil {
			return nil, err
		}
		if evt.EventType == event.TypeCreate || evt.EventType == event.TypeUpdate {
			events = append(events, evt)
		}
	}
	return events, nil
}

func (e *stubEngine) SetState(data map[string]value.Value) {
	e.data = value.Copy(data)
	e.version++
}

func (e *stubEngine) Reload(ctx context.Context) ([]event.Event, []layer.LayerError, error) {
	return nil, nil, nil
}

func (e *stubEngine) Close(ctx context.Context) error {
	e.closed = true
	return nil
}

func (e *stubEngine) AddLayer(l *layer.Layer) error { return nil }

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestMutationService(engine *stubEngine, opts ...MutationServiceOption) *MutationService {
	bus := eventbus.NewBus(eventbus.WithWorkerCount(1), eventbus.WithQueueSize(100))
	pipe := pipeline.New()
	chain := interceptor.NewChain()
	return NewMutationService(pipe, engine, bus, chain, observability.Nop(), opts...)
}

func newTestMutationServiceWithChain(engine *stubEngine, chain *interceptor.Chain) *MutationService {
	bus := eventbus.NewBus(eventbus.WithWorkerCount(1), eventbus.WithQueueSize(100))
	pipe := pipeline.New()
	return NewMutationService(pipe, engine, bus, chain, observability.Nop())
}

// ---------------------------------------------------------------------------
// Set operation
// ---------------------------------------------------------------------------

func TestMutationService_Set(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestMutationService(engine)

	err := svc.Set(context.Background(), "host", "localhost")
	require.NoError(t, err)

	v, ok := engine.Get("host")
	assert.True(t, ok)
	assert.Equal(t, "localhost", v.String())
}

func TestMutationService_Set_Update(t *testing.T) {
	engine := newStubEngine(map[string]value.Value{
		"port": value.New(8080),
	})
	svc := newTestMutationService(engine)

	err := svc.Set(context.Background(), "port", 9090)
	require.NoError(t, err)

	v, _ := engine.Get("port")
	assert.Equal(t, 9090, v.Int())
}

// ---------------------------------------------------------------------------
// Delete operation
// ---------------------------------------------------------------------------

func TestMutationService_Delete(t *testing.T) {
	engine := newStubEngine(map[string]value.Value{
		"temp": value.New("value"),
	})
	svc := newTestMutationService(engine)

	err := svc.Delete(context.Background(), "temp")
	require.NoError(t, err)

	_, ok := engine.Get("temp")
	assert.False(t, ok)
}

func TestMutationService_Delete_Nonexistent(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestMutationService(engine)

	err := svc.Delete(context.Background(), "nonexistent")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// BatchSet operation
// ---------------------------------------------------------------------------

func TestMutationService_BatchSet(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestMutationService(engine)

	err := svc.BatchSet(context.Background(), map[string]any{
		"a": 1,
		"b": "hello",
		"c": true,
	})
	require.NoError(t, err)

	v, _ := engine.Get("a")
	assert.Equal(t, 1, v.Int())

	v, _ = engine.Get("b")
	assert.Equal(t, "hello", v.String())

	v, _ = engine.Get("c")
	assert.True(t, v.Bool())
}

func TestMutationService_BatchSet_Empty(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestMutationService(engine)

	err := svc.BatchSet(context.Background(), nil)
	require.NoError(t, err)
}

func TestMutationService_BatchSet_Overwrite(t *testing.T) {
	engine := newStubEngine(map[string]value.Value{
		"a": value.New(1),
	})
	svc := newTestMutationService(engine)

	err := svc.BatchSet(context.Background(), map[string]any{"a": 99})
	require.NoError(t, err)

	v, _ := engine.Get("a")
	assert.Equal(t, 99, v.Int())
}

// ---------------------------------------------------------------------------
// Namespace prefix
// ---------------------------------------------------------------------------

func TestMutationService_Namespace(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestMutationService(engine, WithMutationNamespace("app."))

	err := svc.Set(context.Background(), "host", "localhost")
	require.NoError(t, err)

	// Key should be prefixed with namespace
	_, ok := engine.Get("app.host")
	assert.True(t, ok)
	_, ok = engine.Get("host")
	assert.False(t, ok)
}

func TestMutationService_Namespace_Delete(t *testing.T) {
	engine := newStubEngine(map[string]value.Value{
		"app.temp": value.New("value"),
	})
	svc := newTestMutationService(engine, WithMutationNamespace("app."))

	err := svc.Delete(context.Background(), "temp")
	require.NoError(t, err)

	_, ok := engine.Get("app.temp")
	assert.False(t, ok)
}

func TestMutationService_Namespace_BatchSet(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestMutationService(engine, WithMutationNamespace("ns."))

	err := svc.BatchSet(context.Background(), map[string]any{"a": 1, "b": 2})
	require.NoError(t, err)

	_, ok := engine.Get("ns.a")
	assert.True(t, ok)
	_, ok = engine.Get("ns.b")
	assert.True(t, ok)
}

func TestMutationService_NamespaceAccessor(t *testing.T) {
	svc := newTestMutationService(newStubEngine(nil), WithMutationNamespace("app."))
	assert.Equal(t, "app.", svc.Namespace())
}

// ---------------------------------------------------------------------------
// Interceptor integration
// ---------------------------------------------------------------------------

func TestMutationService_BeforeSetInterceptor_Error(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	chain.AddSetInterceptor(&interceptor.SetFunc{
		BeforeFn: func(ctx context.Context, req *interceptor.SetRequest) error {
			return errors.New(errors.CodeValidation, "key is required")
		},
	})

	svc := newTestMutationServiceWithChain(engine, chain)

	err := svc.Set(context.Background(), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before-set interceptor failed")
	// Value should NOT be set
	_, ok := engine.Get("")
	assert.False(t, ok)
}

func TestMutationService_AfterSetInterceptor(t *testing.T) {
	engine := newStubEngine(nil)
	chain := interceptor.NewChain()

	var afterCalled bool
	chain.AddSetInterceptor(&interceptor.SetFunc{
		AfterFn: func(ctx context.Context, req *interceptor.SetRequest, res *interceptor.SetResponse) error {
			afterCalled = true
			assert.True(t, res.Created)
			return nil
		},
	})

	svc := newTestMutationServiceWithChain(engine, chain)

	err := svc.Set(context.Background(), "newkey", "value")
	require.NoError(t, err)
	assert.True(t, afterCalled)
}

func TestMutationService_BeforeDeleteInterceptor_Error(t *testing.T) {
	engine := newStubEngine(map[string]value.Value{"key": value.New("val")})
	chain := interceptor.NewChain()

	chain.AddDeleteInterceptor(&interceptor.DeleteFunc{
		BeforeFn: func(ctx context.Context, req *interceptor.DeleteRequest) error {
			return errors.New(errors.CodePermissionDenied, "cannot delete")
		},
	})

	svc := newTestMutationServiceWithChain(engine, chain)

	err := svc.Delete(context.Background(), "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before-delete interceptor failed")
	// Key should still exist
	_, ok := engine.Get("key")
	assert.True(t, ok)
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestMutationService_Validate(t *testing.T) {
	svc := newTestMutationService(newStubEngine(nil))
	err := svc.Validate(context.Background(), struct{}{})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------------

func TestMutationService_Restore(t *testing.T) {
	engine := newStubEngine(nil)
	svc := newTestMutationService(engine)

	newData := map[string]value.Value{
		"a": value.New(1),
		"b": value.New(2),
	}
	svc.Restore(newData)

	v, ok := engine.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v.Int())
}
