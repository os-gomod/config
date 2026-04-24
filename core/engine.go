// Package core implements the configuration engine, layer management, and
// internal value manipulation primitives used by the top-level config package.
// It provides the [Engine] type for atomic state management and concurrent
// layer reloading, as well as the [Layer] type for resilient, circuit-breaker-
// protected configuration source loading.
package core

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// Engine is the core configuration engine that manages layers, state,
// and mutation operations. It is safe for concurrent use.
//
// Reads are lock-free via atomic.Pointer to the state. Writes are serialized
// with a mutex. Layers are loaded concurrently during reload.
type Engine struct {
	closable       *common.Closable
	mu             sync.RWMutex
	layers         []*Layer
	state          atomic.Pointer[value.State]
	version        atomic.Uint64
	maxWorkers     int
	deltaReload    bool
	batchedReload  bool
	cacheTTL       time.Duration
	layerChecksums map[string]string
}

// New creates a new Engine with the given options.
func New(opts ...Option) *Engine {
	e := &Engine{
		closable:       common.NewClosable(),
		layers:         make([]*Layer, 0, 8),
		maxWorkers:     8,
		layerChecksums: make(map[string]string),
	}
	for _, opt := range opts {
		opt(e)
	}
	e.sortLayers()
	e.state.Store(value.NewState(nil, 0))
	return e
}

// Get returns the value for the given key. The second return value reports
// whether the key exists.
func (e *Engine) Get(key string) (value.Value, bool) {
	return e.state.Load().Get(key)
}

// GetAll returns a defensive copy of all key-value pairs in the current state.
func (e *Engine) GetAll() map[string]value.Value {
	return e.state.Load().GetAll()
}

// GetAllUnsafe returns the raw state data map without copying.
// The caller must not modify the returned map.
func (e *Engine) GetAllUnsafe() map[string]value.Value {
	return e.state.Load().GetAllUnsafe()
}

// Has reports whether the given key exists in the current state.
func (e *Engine) Has(key string) bool {
	return e.state.Load().Has(key)
}

// Keys returns all keys in the current state, sorted lexicographically.
func (e *Engine) Keys() []string {
	return e.state.Load().Keys()
}

// Version returns the current state version number, incremented on each mutation.
func (e *Engine) Version() uint64 {
	return e.state.Load().Version()
}

// State returns the current state pointer.
func (e *Engine) State() *value.State {
	return e.state.Load()
}

// Len returns the number of keys in the current state.
func (e *Engine) Len() int { return e.state.Load().Len() }

// IsClosed reports whether the engine has been closed.
func (e *Engine) IsClosed() bool          { return e.closable.IsClosed() }
func (e *Engine) BatchedReload() bool     { return e.batchedReload }
func (e *Engine) CacheTTL() time.Duration { return e.cacheTTL }

// Done returns a channel that is closed when the engine is closed.
func (e *Engine) Done() <-chan struct{} { return e.closable.Done() }

// Layers returns a copy of the current layer list, sorted by priority.
func (e *Engine) Layers() []*Layer {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Layer, len(e.layers))
	copy(out, e.layers)
	return out
}

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

// Set sets a single key in the state. It returns a create or update event.
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

// BatchSet sets multiple keys in the state atomically within a single lock acquisition.
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

// Delete removes a key from the state. It returns a delete event if the key existed.
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

// SetState replaces the entire state with the given data, incrementing the version.
func (e *Engine) SetState(data map[string]value.Value) {
	copied := value.Copy(data)
	e.mu.Lock()
	ver := e.version.Add(1)
	e.state.Store(value.NewState(copied, ver))
	e.mu.Unlock()
}

// ReloadResult contains the outcome of a reload operation, including
// any generated events, per-layer errors, and the merge plan.
type ReloadResult struct {
	Events    []event.Event
	LayerErrs []LayerError
	MergePlan value.MergePlan
}

// HasErrors reports whether any layer errors occurred during reload.
func (r *ReloadResult) HasErrors() bool {
	return len(r.LayerErrs) > 0
}

// Reload loads all enabled layers concurrently, merges their data by
// priority, and computes a diff against the previous state. If delta
// reload is enabled, unchanged layers (by checksum) are skipped.
func (e *Engine) Reload(ctx context.Context) (ReloadResult, error) {
	if e.IsClosed() {
		return ReloadResult{}, errors.ErrClosed
	}
	layers := e.enabledLayers()
	results := e.loadLayers(ctx, layers)
	maps, errs := collect(results)

	// Delta optimization: skip unchanged layers
	if e.deltaReload {
		var changedMaps []map[string]value.Value
		for _, r := range results {
			if r.err != nil {
				continue // skip failed layers
			}
			chk := value.ComputeChecksum(r.data)
			if e.getLayerChecksum(r.name) == chk {
				continue // unchanged
			}
			e.setLayerChecksum(r.name, chk)
			changedMaps = append(changedMaps, r.data)
		}
		// If no layers changed, return empty result (no-op)
		if len(changedMaps) == 0 {
			return ReloadResult{}, nil
		}
		maps = changedMaps
	}

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

// AddLayer adds a new layer to the engine and re-sorts by priority.
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

func (e *Engine) sortLayers() {
	sort.SliceStable(e.layers, func(i, j int) bool {
		if e.layers[i].priority != e.layers[j].priority {
			return e.layers[i].priority > e.layers[j].priority
		}
		return e.layers[i].name < e.layers[j].name
	})
}

// Close closes all layers and marks the engine as closed.
func (e *Engine) Close(ctx context.Context) error {
	e.mu.RLock()
	layers := make([]*Layer, len(e.layers))
	copy(layers, e.layers)
	e.mu.RUnlock()
	for _, l := range layers {
		_ = l.Close(ctx)
	}
	return e.closable.Close(ctx)
}

// loadResult holds the outcome of loading a single layer, including the
// loaded data, any error, and the layer name for error attribution.
type loadResult struct {
	data map[string]value.Value
	err  error
	name string
}

// enabledLayers returns all layers that are currently enabled, protected by
// a read lock.
func (e *Engine) enabledLayers() []*Layer {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Layer, 0, len(e.layers))
	for _, l := range e.layers {
		if l.IsEnabled() {
			out = append(out, l)
		}
	}
	return out
}

// loadLayers loads all given layers concurrently, respecting the engine's
// maxWorkers semaphore. Results are returned in the same order as the input.
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

func (e *Engine) getLayerChecksum(name string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.layerChecksums[name]
}

func (e *Engine) setLayerChecksum(name, chk string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.layerChecksums[name] = chk
}

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

// applySet sets a key in the given data map and returns an appropriate event
// (Create or Update). Returns nil if the value is unchanged.
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

// applyBatchSet sets multiple keys in the given data map and returns
// individual Create/Update events for each changed key.
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

// applyDelete removes a key from the given data map and returns a Delete
// event. Returns nil if the key did not exist.
func applyDelete(d map[string]value.Value, key string) []event.Event {
	oldVal, exists := d[key]
	if !exists {
		return nil
	}
	delete(d, key)
	return []event.Event{event.NewDeleteEvent(key, oldVal)}
}
