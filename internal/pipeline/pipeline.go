// Package pipeline implements the Command Pipeline Pattern — the central
// innovation for routing all lifecycle actions (Set, Delete, Reload, Bind)
// through a single, composable middleware chain. No orchestration duplication.
package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/event"
)

// correlationIDKey is the context key for correlation IDs.
type correlationIDKey struct{}

// Command represents a discrete operation to execute through the pipeline.
// Commands are value objects — they describe WHAT to do, not HOW.
// The Execute field contains the actual implementation, allowing the
// pipeline to handle cross-cutting concerns uniformly.
type Command struct {
	// Name is a human-readable label for this command (e.g., "set-defaults").
	Name string

	// Operation classifies the command type for middleware routing decisions.
	// Recognized values: "set", "delete", "reload", "bind", "batch_set".
	Operation string

	// Key is the target key for single-key operations (set, delete).
	Key string

	// Value is the value to set (for set operations).
	Value any

	// Values is the map of key-value pairs for batch_set operations.
	Values map[string]any

	// Execute contains the actual operation implementation. The pipeline
	// calls this after all middleware has processed the command.
	Execute func(ctx context.Context) (Result, error)
}

// Result holds the outcome of a command execution.
// It is populated by the Execute function and enriched by middleware.
type Result struct {
	// Events contains domain events generated during command execution.
	Events []event.Event

	// Duration records how long the command took to execute.
	Duration time.Duration

	// Skipped indicates whether the command was short-circuited by middleware.
	Skipped bool

	// SkipReason explains why the command was skipped (if Skipped is true).
	SkipReason string
}

// Middleware intercepts command execution for cross-cutting concerns.
// Middleware wraps the next handler, performing pre/post processing.
// Middleware is composed left-to-right: first middleware wraps second, etc.
// The innermost handler is the actual command executor.
type Middleware func(next Handler) Handler

// Handler processes a command and returns a result.
type Handler func(ctx context.Context, cmd Command) (Result, error)

// Pipeline executes commands through a chain of middleware.
// It is the single entry point for all config lifecycle operations.
//
// Usage:
//
//	p := pipeline.New(
//	    pipeline.WithMiddleware(logging, recovery, tracing),
//	    pipeline.WithMetricsRecorder(myRecorder),
//	)
//	result, err := p.Run(ctx, pipeline.Command{
//	    Name:      "set-timeout",
//	    Operation: "set",
//	    Key:       "timeout",
//	    Value:     30,
//	    Execute:   func(ctx context.Context) (pipeline.Result, error) { ... },
//	})
type Pipeline struct {
	middleware []Middleware
	mu         sync.RWMutex

	metricsRecorder MetricsRecorder
	logger          structuredLogger
	correlator      Correlator
}

// structuredLogger abstracts the logging interface for testability.
type structuredLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// Correlator generates correlation IDs for request tracing.
type Correlator interface {
	NewID() string
}

// MetricsRecorder records operation metrics (duration, success/failure).
// Implementations should be safe for concurrent use.
type MetricsRecorder interface {
	RecordOperation(ctx context.Context, op string, duration time.Duration, err error)
}

// AuditRecorder records audit entries for mutating operations.
// It wraps the domain event.Recorder with pipeline-specific semantics.
// Implementations should be safe for concurrent use.
type AuditRecorder interface {
	RecordAudit(ctx context.Context, entry event.AuditEntry)
}

// noopRecorder is a safe no-op implementation of MetricsRecorder.
type noopRecorder struct{}

func (noopRecorder) RecordOperation(_ context.Context, _ string, _ time.Duration, _ error) {}

// noopLogger is a safe no-op implementation of structuredLogger.
type noopLogger struct{}

func (noopLogger) Info(_ string, _ ...any)  {}
func (noopLogger) Warn(_ string, _ ...any)  {}
func (noopLogger) Error(_ string, _ ...any) {}
func (noopLogger) Debug(_ string, _ ...any) {}

// noopCorrelator is a safe no-op Correlator that generates zero-value IDs.
type noopCorrelator struct{}

func (noopCorrelator) NewID() string { return "" }

// buildHandler chains all middleware around the final executor.
// If no middleware is registered, the executor is called directly.
func (p *Pipeline) buildHandler() Handler {
	// Start with the base executor handler
	base := p.executorHandler()

	// Wrap middleware in reverse order so first registered is outermost.
	// This means the first middleware's "before" logic runs first,
	// and its "after" logic runs last.
	p.mu.RLock()
	mws := make([]Middleware, len(p.middleware))
	copy(mws, p.middleware)
	p.mu.RUnlock()

	// Apply in reverse: last middleware wraps first, so first is outermost.
	for i := len(mws) - 1; i >= 0; i-- {
		base = mws[i](base)
	}

	return base
}

// executorHandler returns the base handler that actually executes the command.
func (p *Pipeline) executorHandler() Handler {
	return func(ctx context.Context, cmd Command) (Result, error) {
		if cmd.Execute == nil {
			return Result{Skipped: true, SkipReason: "no execute function provided"}, nil
		}

		start := time.Now()
		result, err := cmd.Execute(ctx)
		result.Duration = time.Since(start)

		// Record metrics if a recorder is configured.
		if p.metricsRecorder != nil {
			p.metricsRecorder.RecordOperation(ctx, cmd.Operation, result.Duration, err)
		}

		return result, err
	}
}

// New creates a Pipeline with the given options.
func New(opts ...Option) *Pipeline {
	p := &Pipeline{
		metricsRecorder: noopRecorder{},
		logger:          noopLogger{},
		correlator:      noopCorrelator{},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Run executes a command through all registered middleware.
// The middleware chain is rebuilt on each call to support dynamic
// middleware registration, but a read-lock is used for efficiency.
//
//nolint:gocritic // Command is immutable by design; value semantics prevent middleware side-effects
func (p *Pipeline) Run(ctx context.Context, cmd Command) (Result, error) {
	handler := p.buildHandler()
	return handler(ctx, cmd)
}

// Use adds middleware to the pipeline. Middleware is applied in order:
// the first middleware registered becomes the outermost wrapper,
// meaning its "before" logic runs first and "after" logic runs last.
func (p *Pipeline) Use(mw ...Middleware) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.middleware = append(p.middleware, mw...)
}

// MiddlewareCount returns the number of registered middleware.
func (p *Pipeline) MiddlewareCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.middleware)
}

// CorrelationIDFromContext extracts the correlation ID from the context.
// Returns empty string if no correlation ID is present.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey{}).(string); ok {
		return id
	}
	return ""
}

// ContextWithCorrelationID returns a new context with the given correlation ID.
func ContextWithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}
