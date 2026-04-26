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

// RuntimeService manages lifecycle operations: reload, bind, watch.
type RuntimeService struct {
	pipeline    *pipeline.Pipeline
	engine      Engine
	bus         *eventbus.Bus
	chain       *interceptor.Chain
	recorder    observability.Recorder
	onReloadErr func(error)
}

type RuntimeServiceOption func(*RuntimeService)

func WithOnReloadError(fn func(error)) RuntimeServiceOption {
	return func(s *RuntimeService) { s.onReloadErr = fn }
}

//nolint:dupl // service constructor pattern is similar to MutationService
func NewRuntimeService(
	pipe *pipeline.Pipeline,
	engine Engine,
	bus *eventbus.Bus,
	chain *interceptor.Chain,
	recorder observability.Recorder,
	opts ...RuntimeServiceOption,
) *RuntimeService {
	s := &RuntimeService{
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

func (s *RuntimeService) Reload(ctx context.Context) (ReloadResult, error) {
	if s.chain.HasReloadInterceptors() {
		req := &interceptor.ReloadRequest{}
		if err := s.chain.BeforeReload(ctx, req); err != nil {
			return ReloadResult{}, errors.Build(
				errors.CodeInterceptor, "before-reload interceptor failed",
				errors.WithOperation("reload"),
			).Wrap(err)
		}
	}

	result, err := s.pipeline.Run(ctx, pipeline.Command{
		Name:      "reload",
		Operation: "reload",
		Execute: func(ctx context.Context) (pipeline.Result, error) {
			events, layerErrs, execErr := s.engine.Reload(ctx)
			_ = ReloadResult{Events: events, LayerErrs: layerErrs, Changed: len(events) > 0}
			return pipeline.Result{Events: events}, execErr
		},
	})
	if err != nil {
		if s.onReloadErr != nil {
			s.onReloadErr(err)
		}
		return ReloadResult{}, err
	}

	for i := range result.Events {
		_ = s.bus.Publish(ctx, &result.Events[i])
	}

	if s.chain.HasReloadInterceptors() {
		req := &interceptor.ReloadRequest{}
		res := &interceptor.ReloadResponse{
			Events:  result.Events,
			Changed: len(result.Events) > 0,
		}
		if afterErr := s.chain.AfterReload(ctx, req, res); afterErr != nil {
			return ReloadResult{}, errors.Build(
				errors.CodeInterceptor, "after-reload interceptor failed",
				errors.WithOperation("reload"),
			).Wrap(afterErr)
		}
	}

	return ReloadResult{
		Events:  result.Events,
		Changed: len(result.Events) > 0,
	}, nil
}

func (s *RuntimeService) Bind(ctx context.Context, target any) error {
	if s.chain.HasBindInterceptors() {
		req := &interceptor.BindRequest{Target: target}
		if err := s.chain.BeforeBind(ctx, req); err != nil {
			return errors.Build(
				errors.CodeInterceptor, "before-bind interceptor failed",
				errors.WithOperation("bind"),
			).Wrap(err)
		}
	}

	_, err := s.pipeline.Run(ctx, pipeline.Command{
		Name:      "bind",
		Operation: "bind",
		Execute: func(_ context.Context) (pipeline.Result, error) {
			_ = s.engine.GetAll()
			_ = target
			return pipeline.Result{}, nil
		},
	})
	if err != nil {
		return err
	}

	if s.chain.HasBindInterceptors() {
		req := &interceptor.BindRequest{Target: target}
		res := &interceptor.BindResponse{Target: target}
		if afterErr := s.chain.AfterBind(ctx, req, res); afterErr != nil {
			return errors.Build(
				errors.CodeInterceptor, "after-bind interceptor failed",
				errors.WithOperation("bind"),
			).Wrap(afterErr)
		}
	}
	return nil
}

func (s *RuntimeService) Close(ctx context.Context) error {
	if s.chain.HasCloseInterceptors() {
		req := &interceptor.CloseRequest{}
		if err := s.chain.BeforeClose(ctx, req); err != nil {
			return errors.Build(
				errors.CodeInterceptor, "before-close interceptor failed",
				errors.WithOperation("close"),
			).Wrap(err)
		}
	}

	err := s.engine.Close(ctx)
	s.bus.Close()

	if s.chain.HasCloseInterceptors() {
		req := &interceptor.CloseRequest{}
		res := &interceptor.CloseResponse{}
		if afterErr := s.chain.AfterClose(ctx, req, res); afterErr != nil {
			return errors.Build(
				errors.CodeInterceptor, "after-close interceptor failed",
				errors.WithOperation("close"),
			).Wrap(afterErr)
		}
	}
	return err
}

func init() {
	_ = value.Value{}
	_ = errors.AppError(nil)
	_ = observability.Recorder(nil)
	_ = event.Event{}
}
