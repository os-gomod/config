package pipeline

import (
	"log/slog"
)

// Option configures a Pipeline during construction.
type Option func(*Pipeline)

// WithMiddleware adds middleware to the pipeline.
// Middleware is applied in registration order: first middleware wraps second, etc.
// This can be called multiple times; middleware accumulates.
func WithMiddleware(mw ...Middleware) Option {
	return func(p *Pipeline) {
		p.Use(mw...)
	}
}

// WithMetricsRecorder sets the MetricsRecorder for the pipeline.
// The recorder is used by the built-in MetricsMiddleware and by the
// base executor handler to record operation metrics.
// If not set, a no-op recorder is used.
func WithMetricsRecorder(r MetricsRecorder) Option {
	return func(p *Pipeline) {
		if r != nil {
			p.metricsRecorder = r
		}
	}
}

// WithAuditRecorder sets the AuditRecorder for the pipeline and automatically
// registers the AuditMiddleware. This is a convenience option that combines
// creating and registering the audit middleware in one call.
// If not set, audit logging is disabled.
func WithAuditRecorder(r AuditRecorder) Option {
	return func(p *Pipeline) {
		if r != nil {
			p.Use(AuditMiddleware(r))
		}
	}
}

// WithTracer sets a Tracer and automatically registers the TracingMiddleware.
// The tracer parameter must implement the pipeline.Tracer interface.
// If nil or not a Tracer, the middleware is a no-op pass-through.
func WithTracer(tracer any) Option {
	return func(p *Pipeline) {
		if t, ok := tracer.(Tracer); ok && t != nil {
			p.Use(TracingMiddleware(t))
		}
	}
}

// WithLogger sets the structured logger for the pipeline and automatically
// registers the LoggingMiddleware. This is a convenience option that
// combines creating and registering the logging middleware in one call.
func WithLogger(logger *slog.Logger) Option {
	return func(p *Pipeline) {
		if logger != nil {
			p.logger = slogLoggerAdapter{logger: logger}
			p.Use(LoggingMiddleware(logger))
		}
	}
}

// WithCorrelator sets the Correlator for generating correlation IDs.
// If not set, a no-op correlator is used that generates zero-value IDs.
func WithCorrelator(c Correlator) Option {
	return func(p *Pipeline) {
		if c != nil {
			p.correlator = c
		}
	}
}

// slogLoggerAdapter adapts *slog.Logger to the structuredLogger interface.
type slogLoggerAdapter struct {
	logger *slog.Logger
}

func (a slogLoggerAdapter) Info(msg string, args ...any) {
	a.logger.Info(msg, args...)
}

func (a slogLoggerAdapter) Warn(msg string, args ...any) {
	a.logger.Warn(msg, args...)
}

func (a slogLoggerAdapter) Error(msg string, args ...any) {
	a.logger.Error(msg, args...)
}

func (a slogLoggerAdapter) Debug(msg string, args ...any) {
	a.logger.Debug(msg, args...)
}
