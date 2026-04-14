// Package core provides the central Engine and Layer types that compose
// config layers, manage state, and support atomic mutations.
package core

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// Engine is the central config orchestrator. It owns an ordered list of Layers,
// a single atomic State, and a monotonically increasing version counter.
//
// Lock order: mu is always acquired before any Layer-level locks.
type Engine struct {
	closable   *common.Closable
	mu         sync.RWMutex // protects layers and state transitions
	layers     []*Layer
	state      atomic.Pointer[value.State]
	version    atomic.Uint64
	maxWorkers int // semaphore capacity for concurrent layer loads
}

// New creates an Engine with the given options. The Engine is initialized with
// an empty state at version 0. Layers added via WithLayer/WithLayers are
// sorted by descending priority.
func New(opts ...Option) *Engine {
	e := &Engine{
		closable:   common.NewClosable(),
		layers:     make([]*Layer, 0, 8),
		maxWorkers: 8,
	}
	for _, opt := range opts {
		opt(e)
	}
	e.sortLayers()
	e.state.Store(value.NewState(nil, 0))
	return e
}

// Get retrieves a Value by key. The second return indicates whether the key existed.
func (e *Engine) Get(key string) (value.Value, bool) {
	return e.state.Load().Get(key)
}

// GetAll returns a safe copy of the current state data.
func (e *Engine) GetAll() map[string]value.Value {
	return e.state.Load().GetAll()
}

// GetAllUnsafe returns the internal state data map without copying.
// Callers must not modify the returned map.
func (e *Engine) GetAllUnsafe() map[string]value.Value {
	return e.state.Load().GetAllUnsafe()
}

// Has reports whether a key exists in the current state.
func (e *Engine) Has(key string) bool {
	return e.state.Load().Has(key)
}

// Keys returns all keys in the current state in sorted order.
func (e *Engine) Keys() []string {
	return e.state.Load().Keys()
}

// Version returns the current state version.
func (e *Engine) Version() uint64 {
	return e.state.Load().Version()
}

// State returns the current State pointer.
func (e *Engine) State() *value.State {
	return e.state.Load()
}

// Len returns the number of keys in the current state.
func (e *Engine) Len() int { return e.state.Load().Len() }

// IsClosed reports whether the Engine has been closed.
func (e *Engine) IsClosed() bool { return e.closable.IsClosed() }

// Done returns a channel that is closed when the Engine is closed.
func (e *Engine) Done() <-chan struct{} { return e.closable.Done() }

// Layers returns a copy of the layer list.
func (e *Engine) Layers() []*Layer {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Layer, len(e.layers))
	copy(out, e.layers)
	return out
}

// applyMutation executes a mutation function under the write lock, updating
// the state atomically if any change events are produced.
func (e *Engine) applyMutation(
	fn func(map[string]value.Value) ([]event.Event, error),
) ([]event.Event, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cur := e.state.Load()
	next := value.Copy(cur.Data())
	events, err := fn(next)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	ver := e.version.Add(1)
	e.state.Store(value.NewState(next, ver))
	return events, nil
}

// Set sets a single key to the given raw value. It returns the change event
// describing the modification.
func (e *Engine) Set(_ context.Context, key string, raw any) (event.Event, error) {
	if e.IsClosed() {
		return event.Event{}, errors.ErrClosed
	}
	var evt event.Event
	_, err := e.applyMutation(func(d map[string]value.Value) ([]event.Event, error) {
		evts := applySet(d, key, raw)
		if len(evts) > 0 {
			evt = evts[0]
		}
		return evts, nil
	})
	return evt, err
}

// BatchSet sets multiple keys atomically. It returns the change events for
// all modifications.
func (e *Engine) BatchSet(_ context.Context, kv map[string]any) ([]event.Event, error) {
	if e.IsClosed() {
		return nil, errors.ErrClosed
	}
	if len(kv) == 0 {
		return nil, nil
	}
	return e.applyMutation(func(d map[string]value.Value) ([]event.Event, error) {
		return applyBatchSet(d, kv)
	})
}

// Delete removes a key from the state. It returns the change event for the
// deletion, or a zero event if the key did not exist.
func (e *Engine) Delete(_ context.Context, key string) (event.Event, error) {
	if e.IsClosed() {
		return event.Event{}, errors.ErrClosed
	}
	var evt event.Event
	_, err := e.applyMutation(func(d map[string]value.Value) ([]event.Event, error) {
		evts := applyDelete(d, key)
		if len(evts) > 0 {
			evt = evts[0]
		}
		return evts, nil
	})
	return evt, err
}

// SetState replaces the entire config state atomically.
// mu is held to serialize concurrent SetState and applyMutation calls.
// version.Add is safe to call with or without the lock, but holding mu here
// ensures that state and version are always updated within the same
// critical section, preventing observers from seeing a version increment
// before the corresponding state is visible.
func (e *Engine) SetState(data map[string]value.Value) {
	copied := value.Copy(data)
	e.mu.Lock()
	ver := e.version.Add(1)
	e.state.Store(value.NewState(copied, ver))
	e.mu.Unlock()
}

// ReloadResult holds the outcome of a Reload operation.
type ReloadResult struct {
	Events    []event.Event
	LayerErrs []LayerError
	MergePlan value.MergePlan
}

// HasErrors reports whether any layer reported an error during reload.
func (r ReloadResult) HasErrors() bool {
	return len(r.LayerErrs) > 0
}

// Reload loads all enabled layers concurrently, merges their results by
// priority, and replaces the engine state atomically.
func (e *Engine) Reload(ctx context.Context) (ReloadResult, error) {
	if e.IsClosed() {
		return ReloadResult{}, errors.ErrClosed
	}
	layers := e.enabledLayers()
	results := e.loadLayers(ctx, layers)
	maps, errs := collect(results)
	merged, plan := value.MergeWithPriorityPlan(maps...)
	e.mu.Lock()
	old := e.state.Load()
	ver := e.version.Add(1)
	e.state.Store(value.NewState(merged, ver))
	e.mu.Unlock()
	events := event.NewDiffEvents(old.Data(), merged)
	return ReloadResult{
		Events:    events,
		LayerErrs: errs,
		MergePlan: plan,
	}, nil
}

// AddLayer appends a Layer to the engine and re-sorts the layer list.
func (e *Engine) AddLayer(l *Layer) error {
	if e.IsClosed() {
		return errors.ErrClosed
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.layers = append(e.layers, l)
	e.sortLayers()
	return nil
}

// sortLayers sorts layers by descending priority, then ascending name for ties.
// Called under mu.Lock().
func (e *Engine) sortLayers() {
	sort.SliceStable(e.layers, func(i, j int) bool {
		if e.layers[i].priority != e.layers[j].priority {
			return e.layers[i].priority > e.layers[j].priority
		}
		return e.layers[i].name < e.layers[j].name
	})
}

// Close shuts down all layers and marks the engine as closed.
func (e *Engine) Close(ctx context.Context) error {
	e.mu.RLock()
	layers := make([]*Layer, len(e.layers))
	copy(layers, e.layers)
	e.mu.RUnlock()
	for _, l := range layers {
		_ = l.Close(ctx) // defer cleanup; error not propagated
	}
	return e.closable.Close(ctx)
}

// loadResult holds the outcome of loading a single layer.
type loadResult struct {
	data map[string]value.Value
	err  error
	name string
}

// enabledLayers returns a snapshot of all currently enabled layers.
func (e *Engine) enabledLayers() []*Layer {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []*Layer
	for _, l := range e.layers {
		if l.IsEnabled() {
			out = append(out, l)
		}
	}
	return out
}

// loadLayers loads the given layers concurrently, bounded by maxWorkers.
func (e *Engine) loadLayers(ctx context.Context, layers []*Layer) []loadResult {
	results := make([]loadResult, len(layers))
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.maxWorkers)
	for i, l := range layers {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, l *Layer) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := l.Load(ctx)
			results[i] = loadResult{
				data: data,
				err:  err,
				name: l.name,
			}
		}(i, l)
	}
	wg.Wait()
	return results
}

// collect separates load results into successful data maps and layer errors.
func collect(results []loadResult) ([]map[string]value.Value, []LayerError) {
	var maps []map[string]value.Value
	var errs []LayerError
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, LayerError{
				Layer: r.name,
				Err:   r.err,
			})
			continue
		}
		maps = append(maps, r.data)
	}
	return maps, errs
}

// applySet applies a single-key set operation, returning change events.
func applySet(d map[string]value.Value, key string, raw any) []event.Event {
	newVal := value.NewInMemory(raw)
	oldVal, exists := d[key]
	if exists && oldVal.Equal(newVal) {
		return nil
	}
	d[key] = newVal
	if exists {
		return []event.Event{event.NewUpdateEvent(key, oldVal, newVal)}
	}
	return []event.Event{event.NewCreateEvent(key, newVal)}
}

// applyBatchSet applies a batch of key-value pairs, returning change events.
func applyBatchSet(d map[string]value.Value, kv map[string]any) ([]event.Event, error) {
	var events []event.Event
	for k, v := range kv {
		newVal := value.NewInMemory(v)
		oldVal, exists := d[k]
		if exists && oldVal.Equal(newVal) {
			continue
		}
		d[k] = newVal
		if exists {
			events = append(events, event.NewUpdateEvent(k, oldVal, newVal))
		} else {
			events = append(events, event.NewCreateEvent(k, newVal))
		}
	}
	return events, nil
}

// applyDelete removes a key from the data map, returning a change event if the key existed.
func applyDelete(d map[string]value.Value, key string) []event.Event {
	oldVal, exists := d[key]
	if !exists {
		return nil
	}
	delete(d, key)
	return []event.Event{event.NewDeleteEvent(key, oldVal)}
}
