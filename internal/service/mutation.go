package service

import (
	"context"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
	"github.com/os-gomod/config/v2/internal/eventbus"
	"github.com/os-gomod/config/v2/internal/interceptor"
	"github.com/os-gomod/config/v2/internal/observability"
	"github.com/os-gomod/config/v2/internal/pipeline"
)

// MutationService handles all config mutations.
// Every operation routes through the Pipeline — zero orchestration duplication.
type MutationService struct {
	pipeline  *pipeline.Pipeline
	engine    Engine
	bus       *eventbus.Bus
	chain     *interceptor.Chain
	recorder  observability.Recorder
	namespace string
}

type MutationServiceOption func(*MutationService)

func WithMutationNamespace(ns string) MutationServiceOption {
	return func(s *MutationService) { s.namespace = ns }
}

//nolint:dupl // service constructor pattern is similar to RuntimeService
func NewMutationService(
	pipe *pipeline.Pipeline,
	engine Engine,
	bus *eventbus.Bus,
	chain *interceptor.Chain,
	recorder observability.Recorder,
	opts ...MutationServiceOption,
) *MutationService {
	s := &MutationService{
		pipeline: pipe,
		engine:   engine,
		bus:      bus,
		chain:    chain,
		recorder: recorder,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *MutationService) resolveKey(key string) string {
	if s.namespace == "" {
		return key
	}
	return s.namespace + key
}

// Set sets a single config value through the pipeline.
func (s *MutationService) Set(ctx context.Context, key string, raw any) error {
	rkey := s.resolveKey(key)

	if s.chain.HasSetInterceptors() {
		req := &interceptor.SetRequest{Key: rkey, Value: raw}
		if err := s.chain.BeforeSet(ctx, req); err != nil {
			return errors.Build(
				errors.CodeInterceptor, "before-set interceptor failed",
				errors.WithOperation("set"), errors.WithOperation("key:"+rkey),
			).Wrap(err)
		}
	}

	result, err := s.pipeline.Run(ctx, pipeline.Command{
		Name:      "set",
		Operation: "set",
		Key:       rkey,
		Value:     raw,
		Execute: func(ctx context.Context) (pipeline.Result, error) {
			evt, execErr := s.engine.Set(ctx, rkey, raw)
			var events []event.Event
			if evt.EventType == event.TypeCreate || evt.EventType == event.TypeUpdate {
				events = append(events, evt)
			}
			return pipeline.Result{Events: events}, execErr
		},
	})
	if err != nil {
		return err
	}

	for i := range result.Events {
		_ = s.bus.Publish(ctx, &result.Events[i])
	}

	if s.chain.HasSetInterceptors() && len(result.Events) > 0 {
		req := &interceptor.SetRequest{Key: rkey, Value: raw}
		res := &interceptor.SetResponse{
			Key:      result.Events[0].Key,
			OldValue: result.Events[0].OldValue,
			NewValue: result.Events[0].NewValue,
			Created:  result.Events[0].EventType == event.TypeCreate,
		}
		if afterErr := s.chain.AfterSet(ctx, req, res); afterErr != nil {
			return errors.Build(
				errors.CodeInterceptor, "after-set interceptor failed",
				errors.WithOperation("set"), errors.WithOperation("key:"+rkey),
			).Wrap(afterErr)
		}
	}
	return nil
}

// Delete removes a config value through the pipeline.
func (s *MutationService) Delete(ctx context.Context, key string) error {
	rkey := s.resolveKey(key)

	if s.chain.HasDeleteInterceptors() {
		req := &interceptor.DeleteRequest{Key: rkey}
		if err := s.chain.BeforeDelete(ctx, req); err != nil {
			return errors.Build(
				errors.CodeInterceptor, "before-delete interceptor failed",
				errors.WithOperation("delete"), errors.WithOperation("key:"+rkey),
			).Wrap(err)
		}
	}

	result, err := s.pipeline.Run(ctx, pipeline.Command{
		Name:      "delete",
		Operation: "delete",
		Key:       rkey,
		Execute: func(ctx context.Context) (pipeline.Result, error) {
			evt, execErr := s.engine.Delete(ctx, rkey)
			var events []event.Event
			if evt.EventType == event.TypeDelete {
				events = append(events, evt)
			}
			return pipeline.Result{Events: events}, execErr
		},
	})
	if err != nil {
		return err
	}

	for i := range result.Events {
		_ = s.bus.Publish(ctx, &result.Events[i])
	}

	if s.chain.HasDeleteInterceptors() && len(result.Events) > 0 {
		req := &interceptor.DeleteRequest{Key: rkey}
		res := &interceptor.DeleteResponse{
			Key:      result.Events[0].Key,
			OldValue: result.Events[0].OldValue,
			Found:    true,
		}
		if afterErr := s.chain.AfterDelete(ctx, req, res); afterErr != nil {
			return errors.Build(
				errors.CodeInterceptor, "after-delete interceptor failed",
				errors.WithOperation("delete"), errors.WithOperation("key:"+rkey),
			).Wrap(afterErr)
		}
	}
	return nil
}

// BatchSet sets multiple config values atomically through the pipeline.
func (s *MutationService) BatchSet(ctx context.Context, kv map[string]any) error {
	if len(kv) == 0 {
		return nil
	}
	if s.namespace != "" {
		prefixed := make(map[string]any, len(kv))
		for k, v := range kv {
			prefixed[s.namespace+k] = v
		}
		kv = prefixed
	}

	result, err := s.pipeline.Run(ctx, pipeline.Command{
		Name:      "batch_set",
		Operation: "batch_set",
		Values:    kv,
		Execute: func(ctx context.Context) (pipeline.Result, error) {
			events, execErr := s.engine.BatchSet(ctx, kv)
			return pipeline.Result{Events: events}, execErr
		},
	})
	if err != nil {
		return err
	}

	for i := range result.Events {
		_ = s.bus.Publish(ctx, &result.Events[i])
	}
	return nil
}

// Validate validates a target struct.
func (s *MutationService) Validate(ctx context.Context, val any) error {
	_ = ctx
	_ = val
	return nil
}

// SetNamespace updates the namespace prefix.
func (s *MutationService) SetNamespace(ctx context.Context, ns string) error {
	s.namespace = ns
	_, _, err := s.engine.Reload(ctx)
	return err
}

func (s *MutationService) Namespace() string { return s.namespace }

func (s *MutationService) Restore(data map[string]value.Value) {
	s.engine.SetState(data)
}
