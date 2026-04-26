// Package service contains the core logic for configuration state management and querying.
package service

import (
	"context"
	"fmt"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/layer"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// Engine is the internal interface for state mutation and querying.
type Engine interface {
	Get(key string) (value.Value, bool)
	GetAll() map[string]value.Value
	Has(key string) bool
	State() *value.State
	Version() uint64
	Len() int
	Keys() []string
	Set(ctx context.Context, key string, raw any) (event.Event, error)
	Delete(ctx context.Context, key string) (event.Event, error)
	BatchSet(ctx context.Context, kv map[string]any) ([]event.Event, error)
	SetState(data map[string]value.Value)
	Reload(ctx context.Context) ([]event.Event, []layer.LayerError, error)
	Close(ctx context.Context) error
	AddLayer(l *layer.Layer) error
}

// ReloadResult holds the outcome of a reload operation.
type ReloadResult struct {
	Events    []event.Event
	LayerErrs []layer.LayerError
	Changed   bool
}

func (r *ReloadResult) HasErrors() bool { return len(r.LayerErrs) > 0 }

// QueryService provides read-only access to configuration state.
type QueryService struct {
	engine Engine
}

func NewQueryService(engine Engine) *QueryService {
	return &QueryService{engine: engine}
}

func (s *QueryService) Get(key string) (value.Value, bool) { return s.engine.Get(key) }
func (s *QueryService) GetAll() map[string]value.Value     { return s.engine.GetAll() }
func (s *QueryService) Has(key string) bool                { return s.engine.Has(key) }
func (s *QueryService) Version() uint64                    { return s.engine.Version() }
func (s *QueryService) Len() int                           { return s.engine.Len() }
func (s *QueryService) Keys() []string                     { return s.engine.Keys() }
func (s *QueryService) State() *value.State                { return s.engine.State() }

func (s *QueryService) Snapshot() map[string]value.Value {
	state := s.engine.State()
	if state == nil {
		return make(map[string]value.Value)
	}
	return state.RedactedCopy().GetAll()
}

func (s *QueryService) Explain(key string) string {
	v, ok := s.engine.Get(key)
	if !ok {
		return ""
	}
	displayVal := v.Raw()
	if value.IsSecret(key) {
		displayVal = "[REDACTED]"
	}
	source := v.Source().String()
	if sourceNamer, okEng := s.engine.(interface{ LayerSourceName(source string) string }); okEng {
		if layerName := sourceNamer.LayerSourceName(key); layerName != "" {
			source = layerName
		}
	}
	return fmt.Sprintf("key %q: value=%v, source=%s, priority=%d",
		key, displayVal, source, v.Priority())
}

// CoreEngine implements the Engine interface.
type CoreEngine struct {
	layers         []*layer.Layer
	state          *value.State
	version        int64
	maxWorkers     int
	deltaReload    bool
	layerChecksums map[string]string
	layerSources   map[string]string
	closed         bool
}

type CoreEngineOption func(*CoreEngine)

func WithMaxWorkers(n int) CoreEngineOption {
	return func(e *CoreEngine) {
		if n > 0 {
			e.maxWorkers = n
		}
	}
}

func WithDeltaReload(enabled bool) CoreEngineOption {
	return func(e *CoreEngine) { e.deltaReload = enabled }
}

func NewCoreEngine(layers []*layer.Layer, opts ...CoreEngineOption) *CoreEngine {
	e := &CoreEngine{
		layers:         layers,
		state:          value.NewState(nil),
		maxWorkers:     8,
		layerChecksums: make(map[string]string),
		layerSources:   make(map[string]string),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *CoreEngine) Get(key string) (value.Value, bool) {
	v := e.state.Get(key)
	return v, !v.IsZero()
}

func (e *CoreEngine) GetAll() map[string]value.Value { return e.state.GetAll() }
func (e *CoreEngine) Has(key string) bool            { return e.state.Has(key) }
func (e *CoreEngine) State() *value.State            { return e.state }
func (e *CoreEngine) Version() uint64                { return uint64(e.state.Version()) }
func (e *CoreEngine) Len() int                       { return e.state.Len() }
func (e *CoreEngine) Keys() []string                 { return e.state.Keys() }

func (e *CoreEngine) Set(ctx context.Context, key string, raw any) (event.Event, error) {
	_ = ctx
	if e.closed {
		return event.Event{}, errors.ErrClosed.WithOperation("engine.set")
	}
	newVal := value.NewInMemory(raw, 100)
	oldVal := e.state.Get(key)
	if !oldVal.IsZero() && oldVal.Equal(newVal) {
		return event.Event{}, nil
	}
	newData := e.state.GetAll()
	newData[key] = newVal
	e.layerSources[key] = "memory"
	e.version++
	e.state = value.NewStateWithVersion(newData, e.version)
	if !oldVal.IsZero() {
		return event.NewUpdateEvent(key, oldVal, newVal), nil
	}
	return event.NewCreateEvent(key, newVal), nil
}

func (e *CoreEngine) Delete(ctx context.Context, key string) (event.Event, error) {
	_ = ctx
	if e.closed {
		return event.Event{}, errors.ErrClosed.WithOperation("engine.delete")
	}
	oldVal := e.state.Get(key)
	if oldVal.IsZero() {
		return event.Event{}, nil
	}
	newData := e.state.GetAll()
	delete(newData, key)
	delete(e.layerSources, key)
	e.version++
	e.state = value.NewStateWithVersion(newData, e.version)
	return event.NewDeleteEvent(key, oldVal), nil
}

func (e *CoreEngine) BatchSet(ctx context.Context, kv map[string]any) ([]event.Event, error) {
	_ = ctx
	if e.closed {
		return nil, errors.ErrClosed.WithOperation("engine.batch_set")
	}
	if len(kv) == 0 {
		return nil, nil
	}
	newData := e.state.GetAll()
	var events []event.Event
	for k, v := range kv {
		newVal := value.NewInMemory(v, 100)
		oldVal := e.state.Get(k)
		if existing, ok := newData[k]; ok {
			oldVal = existing
		}
		if !oldVal.IsZero() && oldVal.Equal(newVal) {
			continue
		}
		newData[k] = newVal
		e.layerSources[k] = "memory"
		if !oldVal.IsZero() {
			events = append(events, event.NewUpdateEvent(k, oldVal, newVal))
		} else {
			events = append(events, event.NewCreateEvent(k, newVal))
		}
	}
	if len(events) == 0 {
		return nil, nil
	}
	e.version++
	e.state = value.NewStateWithVersion(newData, e.version)
	return events, nil
}

func (e *CoreEngine) SetState(data map[string]value.Value) {
	e.layerSources = make(map[string]string, len(data))
	for key, v := range data {
		e.layerSources[key] = v.Source().String()
	}
	e.version++
	e.state = value.NewStateWithVersion(value.Copy(data), e.version)
}

func (e *CoreEngine) Reload(ctx context.Context) ([]event.Event, []layer.LayerError, error) {
	_ = ctx

	if e.closed {
		return nil, nil, errors.ErrClosed.WithOperation("engine.reload")
	}

	maps, layerNames, layerPriorities, layerSources, layerErrs := e.loadLayers()

	merged, plan := value.MergeWithLayerNames(maps, layerNames)

	for _, mut := range plan.Mutations {
		if mut.Kind == value.MutationSet {
			key := mut.Key
			winningLayer := mut.Source

			layerSources[key] = winningLayer

			if priority, ok := layerPriorities[winningLayer]; ok {
				if origVal, okMerge := merged[key]; okMerge {
					merged[key] = value.FromRaw(
						origVal.Raw(),
						origVal.Type(),
						origVal.Source(),
						priority,
					)
				}
			}
		}
	}

	oldData := e.state.GetAll()
	diffs := value.ComputeDiff(oldData, merged)
	events := event.NewDiffEvents(diffs, "reload")

	e.version++
	e.state = value.NewStateWithVersion(merged, e.version)
	e.layerSources = layerSources

	return events, layerErrs, nil
}

func (e *CoreEngine) LayerSourceName(key string) string {
	return e.layerSources[key]
}

func (e *CoreEngine) AddLayer(l *layer.Layer) error {
	if e.closed {
		return errors.ErrClosed.WithOperation("engine.add_layer")
	}
	e.layers = append(e.layers, l)
	return nil
}

func (e *CoreEngine) Close(ctx context.Context) error {
	_ = ctx
	e.closed = true
	return nil
}

func (e *CoreEngine) loadLayers() (
	[]map[string]value.Value,
	[]string,
	map[string]int,
	map[string]string,
	[]layer.LayerError,
) {
	layerPriorities := make(map[string]int)
	layerSources := make(map[string]string)

	var (
		maps       []map[string]value.Value
		layerNames []string
		layerErrs  []layer.LayerError
	)

	for _, l := range e.layers {
		if !l.Enabled() {
			continue
		}

		data, err := l.Load()
		if err != nil {
			layerErrs = append(layerErrs, layer.LayerError{
				LayerName: l.Name(),
				Err:       err,
			})
			continue
		}

		if e.deltaReload {
			chk := value.NewInMemory(fmt.Sprintf("%v", data), 0).ComputeChecksum()
			if e.layerChecksums[l.Name()] == chk {
				continue
			}
			e.layerChecksums[l.Name()] = chk
		}

		maps = append(maps, data)
		layerNames = append(layerNames, l.Name())
		layerPriorities[l.Name()] = l.Priority()
	}

	return maps, layerNames, layerPriorities, layerSources, layerErrs
}
