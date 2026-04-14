package core_test

import (
	"context"
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
)

func TestNewEngineStartsAtVersion0(t *testing.T) {
	e := core.New()
	if e.Version() != 0 {
		t.Fatalf("new engine should start at version 0, got %d", e.Version())
	}
}

func TestAddLayerAndLayers(t *testing.T) {
	e := core.New()
	l := core.NewLayer("test", core.WithLayerPriority(5))
	if err := e.AddLayer(l); err != nil {
		t.Fatalf("AddLayer failed: %v", err)
	}
	layers := e.Layers()
	if len(layers) != 1 || layers[0].Name() != "test" {
		t.Fatalf("Layers: got %v", layers)
	}
}

func TestSetGetRoundTrip(t *testing.T) {
	e := core.New()
	_, err := e.Set(context.Background(), "key", "val")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	v, ok := e.Get("key")
	if !ok {
		t.Fatal("Get should find key after Set")
	}
	if v.Raw() != "val" {
		t.Fatalf("Get: got %v, want val", v.Raw())
	}
}

func TestBatchSetAppliesAllKeys(t *testing.T) {
	e := core.New()
	kv := map[string]any{
		"a": 1,
		"b": "two",
		"c": true,
	}
	_, err := e.BatchSet(context.Background(), kv)
	if err != nil {
		t.Fatalf("BatchSet failed: %v", err)
	}
	for k := range kv {
		if _, ok := e.Get(k); !ok {
			t.Fatalf("key %q not found after BatchSet", k)
		}
	}
}

func TestDeleteRemovesKey(t *testing.T) {
	e := core.New()
	_, _ = e.Set(context.Background(), "key", "val")
	_, err := e.Delete(context.Background(), "key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, ok := e.Get("key"); ok {
		t.Fatal("key should not exist after Delete")
	}
}

// testSource is a Loadable implementation for testing.
type testSource struct {
	data map[string]value.Value
	err  error
}

func (s *testSource) Load(_ context.Context) (map[string]value.Value, error) {
	return s.data, s.err
}

func (s *testSource) Close(_ context.Context) error { return nil }

func TestReloadLoadsAllEnabledLayers(t *testing.T) {
	e := core.New()
	data := map[string]value.Value{
		"key": value.NewInMemory("from-layer"),
	}
	l := core.NewLayer("test",
		core.WithLayerSource(&testSource{data: data}),
	)
	_ = e.AddLayer(l)

	result, err := e.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	v, ok := e.Get("key")
	if !ok || v.Raw() != "from-layer" {
		t.Fatalf("after Reload: got %v, want from-layer", v.Raw())
	}
	if len(result.Events) == 0 && e.Len() > 0 {
		t.Fatal("Reload should produce events for new data")
	}
}

func TestReloadWithDisabledLayerSkipsIt(t *testing.T) {
	e := core.New()
	data := map[string]value.Value{
		"key": value.NewInMemory("should-not-appear"),
	}
	l := core.NewLayer("disabled",
		core.WithLayerSource(&testSource{data: data}),
		core.WithLayerEnabled(false),
	)
	_ = e.AddLayer(l)

	_, err := e.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if _, ok := e.Get("key"); ok {
		t.Fatal("disabled layer should not contribute data")
	}
}

func TestClosedEngineReturnsErrClosed(t *testing.T) {
	e := core.New()
	_ = e.Close(context.Background())

	_, err := e.Set(context.Background(), "key", "val")
	if !stderrors.Is(err, errors.ErrClosed) {
		t.Fatalf("Set on closed engine: got %v, want ErrClosed", err)
	}

	_, err = e.BatchSet(context.Background(), map[string]any{"k": "v"})
	if !stderrors.Is(err, errors.ErrClosed) {
		t.Fatalf("BatchSet on closed engine: got %v, want ErrClosed", err)
	}

	_, err = e.Delete(context.Background(), "key")
	if !stderrors.Is(err, errors.ErrClosed) {
		t.Fatalf("Delete on closed engine: got %v, want ErrClosed", err)
	}

	_, err = e.Reload(context.Background())
	if !stderrors.Is(err, errors.ErrClosed) {
		t.Fatalf("Reload on closed engine: got %v, want ErrClosed", err)
	}

	err = e.AddLayer(core.NewLayer("x"))
	if !stderrors.Is(err, errors.ErrClosed) {
		t.Fatalf("AddLayer on closed engine: got %v, want ErrClosed", err)
	}
}

func TestSetStateReplacesStateAtomically(t *testing.T) {
	e := core.New()
	_, _ = e.Set(context.Background(), "old", "value")

	newData := map[string]value.Value{
		"new": value.NewInMemory("data"),
	}
	e.SetState(newData)

	if _, ok := e.Get("old"); ok {
		t.Fatal("old key should not exist after SetState")
	}
	v, ok := e.Get("new")
	if !ok || v.Raw() != "data" {
		t.Fatal("new key should exist after SetState")
	}
}

func TestWithMaxWorkersIsRespected(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	// countingSource tracks how many loads run concurrently.
	countingSource := &countingLoadable{
		concurrent:    &concurrent,
		maxConcurrent: &maxConcurrent,
		loadDuration:  50 * time.Millisecond,
		data: map[string]value.Value{
			"key": value.NewInMemory("val"),
		},
	}

	e := core.New(core.WithMaxWorkers(2))
	for i := range 6 {
		l := core.NewLayer(
			"layer",
			core.WithLayerSource(countingSource),
			core.WithLayerPriority(i),
		)
		_ = e.AddLayer(l)
	}

	_, err := e.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	mc := maxConcurrent.Load()
	if mc > 2 {
		t.Fatalf("max concurrent loads should not exceed 2, got %d", mc)
	}
}

// countingLoadable tracks concurrent load calls.
type countingLoadable struct {
	concurrent    *atomic.Int32
	maxConcurrent *atomic.Int32
	loadDuration  time.Duration
	data          map[string]value.Value
	mu            sync.Mutex
}

func (s *countingLoadable) Load(_ context.Context) (map[string]value.Value, error) {
	cur := s.concurrent.Add(1)
	for {
		old := s.maxConcurrent.Load()
		if cur <= old || s.maxConcurrent.CompareAndSwap(old, cur) {
			break
		}
	}
	time.Sleep(s.loadDuration)
	s.concurrent.Add(-1)
	return s.data, nil
}

func (s *countingLoadable) Close(_ context.Context) error { return nil }

func TestReloadResultHasErrors(t *testing.T) {
	e := core.New()
	l := core.NewLayer("failing",
		core.WithLayerSource(&testSource{err: errors.New(errors.CodeSource, "fail")}),
	)
	_ = e.AddLayer(l)

	result, _ := e.Reload(context.Background())
	if !result.HasErrors() {
		t.Fatal("ReloadResult should report errors when a layer fails")
	}
}

func TestBatchSetEmptyReturnsNil(t *testing.T) {
	e := core.New()
	evts, err := e.BatchSet(context.Background(), nil)
	if err != nil {
		t.Fatalf("BatchSet(nil): %v", err)
	}
	if evts != nil {
		t.Fatal("BatchSet(nil) should return nil events")
	}
}
