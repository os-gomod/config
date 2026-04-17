package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/core/circuit"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
)

// stubLoadable implements Loadable for testing.
type stubLoadable struct {
	data map[string]value.Value
	err  error
}

func (s *stubLoadable) Load(_ context.Context) (map[string]value.Value, error) {
	if s.err != nil {
		return nil, s.err
	}
	return value.Copy(s.data), nil
}

func (s *stubLoadable) Close(_ context.Context) error { return nil }

// swappableLoadable is a stubLoadable whose data field can be swapped.
type swappableLoadable struct {
	stubLoadable
}

// countingLoadable counts Load calls.
type countingLoadable struct {
	data  map[string]value.Value
	count *atomic.Int64
}

func (c *countingLoadable) Load(_ context.Context) (map[string]value.Value, error) {
	c.count.Add(1)
	time.Sleep(10 * time.Millisecond)
	return value.Copy(c.data), nil
}

func (c *countingLoadable) Close(_ context.Context) error { return nil }

func TestNewEngine(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		e := New()
		if e == nil {
			t.Fatal("expected non-nil engine")
		}
		if e.IsClosed() {
			t.Fatal("new engine should not be closed")
		}
		if e.Len() != 0 {
			t.Fatalf("expected 0 keys, got %d", e.Len())
		}
		if e.Version() != 0 {
			t.Fatalf("expected version 0, got %d", e.Version())
		}
		layers := e.Layers()
		if len(layers) != 0 {
			t.Fatalf("expected 0 layers, got %d", len(layers))
		}
	})

	t.Run("with layers option", func(t *testing.T) {
		l := NewLayer("test")
		e := New(WithLayer(l))
		layers := e.Layers()
		if len(layers) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(layers))
		}
		if layers[0].name != "test" {
			t.Fatalf("expected layer name 'test', got %q", layers[0].name)
		}
	})

	t.Run("with multiple layers option", func(t *testing.T) {
		l1 := NewLayer("a")
		l2 := NewLayer("b")
		e := New(WithLayers(l1, l2))
		if len(e.Layers()) != 2 {
			t.Fatalf("expected 2 layers, got %d", len(e.Layers()))
		}
	})

	t.Run("with max workers option", func(t *testing.T) {
		e := New(WithMaxWorkers(4))
		if e.maxWorkers != 4 {
			t.Fatalf("expected maxWorkers 4, got %d", e.maxWorkers)
		}
	})

	t.Run("with max workers zero ignored", func(t *testing.T) {
		e := New(WithMaxWorkers(0))
		if e.maxWorkers != 8 {
			t.Fatalf("expected default maxWorkers 8, got %d", e.maxWorkers)
		}
	})

	t.Run("with negative max workers ignored", func(t *testing.T) {
		e := New(WithMaxWorkers(-1))
		if e.maxWorkers != 8 {
			t.Fatalf("expected default maxWorkers 8, got %d", e.maxWorkers)
		}
	})
}

func TestEngine_Reload(t *testing.T) {
	t.Run("reload with no layers", func(t *testing.T) {
		e := New()
		result, err := e.Reload(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.HasErrors() {
			t.Fatal("expected no errors")
		}
		if len(result.Events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(result.Events))
		}
	})

	t.Run("reload with successful layers", func(t *testing.T) {
		data := map[string]value.Value{
			"key1": value.NewInMemory("value1"),
			"key2": value.NewInMemory(42),
		}
		l := NewLayer("test", WithLayerSource(&stubLoadable{data: data}))
		e := New(WithLayer(l))
		result, err := e.Reload(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.HasErrors() {
			t.Fatal("expected no errors")
		}
		if e.Len() != 2 {
			t.Fatalf("expected 2 keys after reload, got %d", e.Len())
		}
		v, ok := e.Get("key1")
		if !ok || v.Raw() != "value1" {
			t.Fatal("expected key1=value1")
		}
	})

	t.Run("reload with layer errors", func(t *testing.T) {
		l := NewLayer("fail", WithLayerSource(&stubLoadable{err: fmt.Errorf("load failed")}))
		e := New(WithLayer(l))
		result, err := e.Reload(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.HasErrors() {
			t.Fatal("expected layer errors")
		}
		if len(result.LayerErrs) != 1 {
			t.Fatalf("expected 1 layer error, got %d", len(result.LayerErrs))
		}
	})

	t.Run("reload generates diff events", func(t *testing.T) {
		swappable := &swappableLoadable{stubLoadable: stubLoadable{data: map[string]value.Value{"a": value.NewInMemory("v1")}}}
		l := NewLayer("test", WithLayerSource(swappable))
		e := New(WithLayer(l))
		_, _ = e.Reload(t.Context())

		// Update layer data and reload
		swappable.data = map[string]value.Value{
			"a": value.NewInMemory("v2"),
			"b": value.NewInMemory("new"),
		}
		result, err := e.Reload(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Events) != 2 {
			t.Fatalf("expected 2 events (update + create), got %d", len(result.Events))
		}
	})

	t.Run("reload when closed returns error", func(t *testing.T) {
		e := New()
		_ = e.Close(t.Context())
		_, err := e.Reload(t.Context())
		if err != errors.ErrClosed {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	})

	t.Run("reload merges layers by priority", func(t *testing.T) {
		highData := map[string]value.Value{
			"key": value.NewInMemory("high"),
		}
		lowData := map[string]value.Value{
			"key":  value.NewInMemory("low"),
			"key2": value.NewInMemory("only-low"),
		}
		lHigh := NewLayer("high", WithLayerPriority(100), WithLayerSource(&stubLoadable{data: highData}))
		lLow := NewLayer("low", WithLayerPriority(10), WithLayerSource(&stubLoadable{data: lowData}))
		e := New(WithLayer(lHigh), WithLayer(lLow))
		_, err := e.Reload(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v, ok := e.Get("key")
		if !ok || v.Raw() != "high" {
			t.Fatal("expected high-priority value to win")
		}
		_, ok = e.Get("key2")
		if !ok {
			t.Fatal("expected key2 from low layer")
		}
	})
}

func TestEngine_Set(t *testing.T) {
	t.Run("set new key", func(t *testing.T) {
		e := New()
		evt, err := e.Set(t.Context(), "key1", "value1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != event.TypeCreate {
			t.Fatalf("expected create event, got %s", evt.Type)
		}
		v, ok := e.Get("key1")
		if !ok || v.Raw() != "value1" {
			t.Fatal("expected key1=value1")
		}
	})

	t.Run("set updates existing key", func(t *testing.T) {
		e := New()
		_, _ = e.Set(t.Context(), "key1", "value1")
		evt, err := e.Set(t.Context(), "key1", "value2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != event.TypeUpdate {
			t.Fatalf("expected update event, got %s", evt.Type)
		}
	})

	t.Run("set same value is no-op", func(t *testing.T) {
		e := New()
		_, _ = e.Set(t.Context(), "key1", "value1")
		evt, err := e.Set(t.Context(), "key1", "value1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != event.Type(0) {
			t.Fatalf("expected no event, got %s", evt.Type)
		}
	})

	t.Run("set when closed returns error", func(t *testing.T) {
		e := New()
		_ = e.Close(t.Context())
		_, err := e.Set(t.Context(), "key1", "value1")
		if err != errors.ErrClosed {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	})

	t.Run("set increments version", func(t *testing.T) {
		e := New()
		v1 := e.Version()
		_, _ = e.Set(t.Context(), "key1", "value1")
		v2 := e.Version()
		if v2 <= v1 {
			t.Fatalf("expected version to increase: %d -> %d", v1, v2)
		}
	})
}

func TestEngine_Delete(t *testing.T) {
	t.Run("delete existing key", func(t *testing.T) {
		e := New()
		_, _ = e.Set(t.Context(), "key1", "value1")
		evt, err := e.Delete(t.Context(), "key1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != event.TypeDelete {
			t.Fatalf("expected delete event, got %s", evt.Type)
		}
		if _, ok := e.Get("key1"); ok {
			t.Fatal("expected key1 to be deleted")
		}
	})

	t.Run("delete non-existing key is no-op", func(t *testing.T) {
		e := New()
		evt, err := e.Delete(t.Context(), "missing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != event.Type(0) {
			t.Fatalf("expected no event, got %s", evt.Type)
		}
	})

	t.Run("delete when closed returns error", func(t *testing.T) {
		e := New()
		_ = e.Close(t.Context())
		_, err := e.Delete(t.Context(), "key1")
		if err != errors.ErrClosed {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	})
}

func TestEngine_BatchSet(t *testing.T) {
	t.Run("batch set multiple keys", func(t *testing.T) {
		e := New()
		kv := map[string]any{
			"a": 1,
			"b": "two",
			"c": true,
		}
		events, err := e.BatchSet(t.Context(), kv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 3 {
			t.Fatalf("expected 3 events, got %d", len(events))
		}
		if e.Len() != 3 {
			t.Fatalf("expected 3 keys, got %d", e.Len())
		}
	})

	t.Run("batch set empty map", func(t *testing.T) {
		e := New()
		events, err := e.BatchSet(t.Context(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if events != nil {
			t.Fatalf("expected nil events, got %v", events)
		}
	})

	t.Run("batch set when closed returns error", func(t *testing.T) {
		e := New()
		_ = e.Close(t.Context())
		_, err := e.BatchSet(t.Context(), map[string]any{"a": 1})
		if err != errors.ErrClosed {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	})
}

func TestEngine_Get(t *testing.T) {
	t.Run("get existing key", func(t *testing.T) {
		e := New()
		_, _ = e.Set(t.Context(), "key1", "value1")
		v, ok := e.Get("key1")
		if !ok {
			t.Fatal("expected key to exist")
		}
		if v.Raw() != "value1" {
			t.Fatalf("expected 'value1', got %v", v.Raw())
		}
	})

	t.Run("get missing key", func(t *testing.T) {
		e := New()
		_, ok := e.Get("missing")
		if ok {
			t.Fatal("expected key to not exist")
		}
	})
}

func TestEngine_GetAll(t *testing.T) {
	e := New()
	_, _ = e.Set(t.Context(), "a", 1)
	_, _ = e.Set(t.Context(), "b", 2)
	all := e.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(all))
	}
	// Verify it's a copy
	all["c"] = value.NewInMemory(3)
	if _, ok := e.Get("c"); ok {
		t.Fatal("GetAll should return a copy")
	}
}

func TestEngine_Has(t *testing.T) {
	e := New()
	if e.Has("missing") {
		t.Fatal("expected Has=false for missing key")
	}
	_, _ = e.Set(t.Context(), "key1", "value1")
	if !e.Has("key1") {
		t.Fatal("expected Has=true for existing key")
	}
}

func TestEngine_Keys(t *testing.T) {
	e := New()
	_, _ = e.Set(t.Context(), "b", 1)
	_, _ = e.Set(t.Context(), "a", 2)
	keys := e.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestEngine_State(t *testing.T) {
	e := New()
	_, _ = e.Set(t.Context(), "key1", "value1")
	state := e.State()
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.Len() != 1 {
		t.Fatalf("expected 1 key, got %d", state.Len())
	}
}

func TestEngine_Layers(t *testing.T) {
	t.Run("layers returns a copy", func(t *testing.T) {
		l := NewLayer("test")
		e := New(WithLayer(l))
		layers := e.Layers()
		if len(layers) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(layers))
		}
		layers[0] = nil
		if e.Layers()[0] == nil {
			t.Fatal("Layers() should return a copy")
		}
	})
}

func TestEngine_SortLayers(t *testing.T) {
	t.Run("sorts by priority descending", func(t *testing.T) {
		lLow := NewLayer("low", WithLayerPriority(10))
		lHigh := NewLayer("high", WithLayerPriority(100))
		e := New(WithLayer(lLow), WithLayer(lHigh))
		layers := e.Layers()
		if layers[0].Priority() < layers[1].Priority() {
			t.Fatal("layers should be sorted by priority descending")
		}
	})

	t.Run("sorts by name for same priority", func(t *testing.T) {
		lB := NewLayer("b", WithLayerPriority(50))
		lA := NewLayer("a", WithLayerPriority(50))
		e := New(WithLayer(lB), WithLayer(lA))
		layers := e.Layers()
		if layers[0].name > layers[1].name {
			t.Fatal("layers with same priority should be sorted by name ascending")
		}
	})
}

func TestEngine_AddLayer(t *testing.T) {
	t.Run("add layer after creation", func(t *testing.T) {
		e := New()
		l := NewLayer("added")
		err := e.AddLayer(l)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(e.Layers()) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(e.Layers()))
		}
	})

	t.Run("add layer when closed returns error", func(t *testing.T) {
		e := New()
		_ = e.Close(t.Context())
		err := e.AddLayer(NewLayer("closed"))
		if err != errors.ErrClosed {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	})
}

func TestEngine_Close(t *testing.T) {
	t.Run("close sets IsClosed", func(t *testing.T) {
		e := New()
		if e.IsClosed() {
			t.Fatal("should not be closed initially")
		}
		err := e.Close(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !e.IsClosed() {
			t.Fatal("should be closed after Close")
		}
	})

	t.Run("close sends on done channel", func(t *testing.T) {
		e := New()
		done := e.Done()
		select {
		case <-done:
			t.Fatal("done should not be closed initially")
		default:
		}
		_ = e.Close(t.Context())
		select {
		case <-done:
			// expected
		default:
			t.Fatal("done should be closed after Close")
		}
	})
}

func TestEngine_SetState(t *testing.T) {
	e := New()
	data := map[string]value.Value{
		"key1": value.NewInMemory("v1"),
		"key2": value.NewInMemory("v2"),
	}
	e.SetState(data)
	if e.Len() != 2 {
		t.Fatalf("expected 2 keys, got %d", e.Len())
	}
	// Verify data is copied
	data["key3"] = value.NewInMemory("v3")
	if _, ok := e.Get("key3"); ok {
		t.Fatal("SetState should copy data")
	}
}

func TestEngine_ConcurrentOperations(t *testing.T) {
	e := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			_, _ = e.Set(t.Context(), key, i)
		}(i)
	}
	wg.Wait()
	if e.Len() != 100 {
		t.Fatalf("expected 100 keys, got %d", e.Len())
	}
}

func TestEngine_ReloadConcurrent(t *testing.T) {
	var count atomic.Int64
	source := &countingLoadable{
		data:  map[string]value.Value{"key": value.NewInMemory("value")},
		count: &count,
	}

	layers := make([]*Layer, 10)
	for i := range layers {
		layers[i] = NewLayer(
			fmt.Sprintf("layer-%d", i),
			WithLayerSource(source),
			WithLayerTimeout(5*time.Second),
		)
	}
	e := New(WithLayers(layers...), WithMaxWorkers(2))
	_, err := e.Reload(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplySet(t *testing.T) {
	t.Run("create new key", func(t *testing.T) {
		d := make(map[string]value.Value)
		events := applySet(d, "key", "value")
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != event.TypeCreate {
			t.Fatalf("expected create event, got %s", events[0].Type)
		}
	})

	t.Run("update existing key", func(t *testing.T) {
		d := map[string]value.Value{"key": value.NewInMemory("old")}
		events := applySet(d, "key", "new")
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != event.TypeUpdate {
			t.Fatalf("expected update event, got %s", events[0].Type)
		}
	})

	t.Run("no event for same value", func(t *testing.T) {
		d := map[string]value.Value{"key": value.NewInMemory("same")}
		events := applySet(d, "key", "same")
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})
}

func TestApplyDelete(t *testing.T) {
	t.Run("delete existing key", func(t *testing.T) {
		d := map[string]value.Value{"key": value.NewInMemory("value")}
		events := applyDelete(d, "key")
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != event.TypeDelete {
			t.Fatalf("expected delete event, got %s", events[0].Type)
		}
		if _, ok := d["key"]; ok {
			t.Fatal("key should be deleted from map")
		}
	})

	t.Run("delete non-existing key", func(t *testing.T) {
		d := make(map[string]value.Value)
		events := applyDelete(d, "missing")
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})
}

func TestApplyBatchSet(t *testing.T) {
	t.Run("batch set creates new keys", func(t *testing.T) {
		d := make(map[string]value.Value)
		kv := map[string]any{"a": 1, "b": 2}
		events, err := applyBatchSet(d, kv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		if len(d) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(d))
		}
	})
}

func TestCollect(t *testing.T) {
	t.Run("collect successful results", func(t *testing.T) {
		results := []loadResult{
			{data: map[string]value.Value{"a": value.NewInMemory(1)}, name: "ok"},
			{data: map[string]value.Value{"b": value.NewInMemory(2)}, name: "ok2"},
		}
		maps, errs := collect(results)
		if len(errs) != 0 {
			t.Fatalf("expected no errors, got %d", len(errs))
		}
		if len(maps) != 2 {
			t.Fatalf("expected 2 maps, got %d", len(maps))
		}
	})

	t.Run("collect with errors", func(t *testing.T) {
		results := []loadResult{
			{data: map[string]value.Value{"a": value.NewInMemory(1)}, name: "ok"},
			{err: fmt.Errorf("failed"), name: "fail"},
		}
		maps, errs := collect(results)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if len(maps) != 1 {
			t.Fatalf("expected 1 map, got %d", len(maps))
		}
		if errs[0].Layer != "fail" {
			t.Fatalf("expected error layer 'fail', got %q", errs[0].Layer)
		}
	})
}

func TestReloadResult_HasErrors(t *testing.T) {
	tests := []struct {
		name   string
		result ReloadResult
		want   bool
	}{
		{"no errors", ReloadResult{}, false},
		{"with errors", ReloadResult{LayerErrs: []LayerError{{Layer: "x"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngine_EnabledLayers(t *testing.T) {
	t.Run("filters disabled layers", func(t *testing.T) {
		l1 := NewLayer("enabled")
		l2 := NewLayer("disabled", WithLayerEnabled(false))
		e := New(WithLayer(l1), WithLayer(l2))
		enabled := e.enabledLayers()
		if len(enabled) != 1 {
			t.Fatalf("expected 1 enabled layer, got %d", len(enabled))
		}
	})
}

func TestEngine_LayerWithCircuitBreaker(t *testing.T) {
	t.Run("layer uses custom circuit breaker", func(t *testing.T) {
		cfg := circuit.BreakerConfig{
			Threshold:        3,
			Timeout:          100 * time.Millisecond,
			SuccessThreshold: 1,
		}
		l := NewLayer("test", WithLayerCircuitBreaker(cfg))
		if l.CircuitBreaker() == nil {
			t.Fatal("expected circuit breaker")
		}
	})
}

func TestLayerError(t *testing.T) {
	err := LayerError{
		Layer: "test",
		Err:   fmt.Errorf("boom"),
	}
	msg := err.Error()
	if msg != "test: boom" {
		t.Fatalf("expected 'test: boom', got %q", msg)
	}
}
