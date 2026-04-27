package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// Local tracing interfaces (replaces go.opentelemetry.io/otel)
// ---------------------------------------------------------------------------

// StatusCode represents the status of a span.
type StatusCode int

const (
	StatusUnset StatusCode = iota
	StatusOK
	StatusError
)

// KeyValue is a key-value pair for span attributes.
type KeyValue struct {
	Key   string
	Value any
}

// String creates a string KeyValue attribute.
func String(key, val string) KeyValue {
	return KeyValue{Key: key, Value: val}
}

// Int64 creates an int64 KeyValue attribute.
func Int64(key string, val int64) KeyValue {
	return KeyValue{Key: key, Value: val}
}

// SpanStartConfig holds options for starting a span.
type SpanStartConfig struct {
	Attributes []KeyValue
	Timestamp  time.Time
}

// SpanStartOption is a function that modifies SpanStartConfig.
type SpanStartOption func(*SpanStartConfig)

// WithAttributes adds attributes to a span start config.
func WithAttributes(attrs ...KeyValue) SpanStartOption {
	return func(cfg *SpanStartConfig) {
		cfg.Attributes = append(cfg.Attributes, attrs...)
	}
}

// WithTimestamp sets the start timestamp for a span.
func WithTimestamp(t time.Time) SpanStartOption {
	return func(cfg *SpanStartConfig) {
		cfg.Timestamp = t
	}
}

// Span is the local interface for a trace span, replacing otel's trace.Span.
type Span interface {
	End()
	RecordError(err error)
	SetStatus(code StatusCode, msg string)
	SetAttributes(attrs ...KeyValue)
}

// Tracer is the local interface for a tracer, replacing otel's trace.Tracer.
type Tracer interface {
	Start(ctx context.Context, spanName string, opts ...SpanStartOption) (context.Context, Span)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isMutating checks whether an operation mutates config state.
func isMutating(op string) bool {
	for _, m := range []string{"set", "delete", "batch_set"} {
		if op == m {
			return true
		}
	}
	return false
}

// operationToAuditAction maps a pipeline operation string to an AuditAction.
func operationToAuditAction(op string) event.AuditAction {
	switch op {
	case "set", "batch_set":
		return event.AuditActionConfigChange
	case "delete":
		return event.AuditActionConfigChange
	case "reload":
		return event.AuditActionReload
	case "watch":
		return event.AuditActionWatch
	default:
		return event.AuditActionConfigChange
	}
}

// TracingMiddleware adds tracing to command execution using a local Tracer interface.
// Each command becomes a span with standard attributes for operation,
// key, and command name. Errors are recorded on the span.
//
// If the tracer is nil, the middleware is a no-op pass-through.
func TracingMiddleware(tracer Tracer) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			if tracer == nil {
				return next(ctx, cmd)
			}

			spanName := "config." + cmd.Operation
			ctx, span := tracer.Start(
				ctx,
				spanName,
				WithAttributes(
					String("service.name", "config"),
					String("config.operation", cmd.Operation),
					String("config.command", cmd.Name),
					String("config.key", cmd.Key),
				),
				WithTimestamp(time.Now()),
			)
			defer func() {
				span.End()
			}()

			result, err := next(ctx, cmd)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(StatusError, err.Error())

				// Enrich span with AppError details if available.
				if ae, ok := errors.AsAppError(err); ok {
					span.SetAttributes(
						String("config.error.code", ae.Code()),
						String("config.error.severity", ae.Severity().String()),
					)
				}
			} else {
				span.SetStatus(StatusOK, "")
				if result.Duration > 0 {
					span.SetAttributes(
						Int64("config.duration_ms", result.Duration.Milliseconds()),
					)
				}
			}

			return result, err
		}
	}
}

// MetricsMiddleware records operation metrics (duration, success/failure)
// using the provided MetricsRecorder. This middleware delegates timing
// to the recorder interface, keeping the middleware itself stateless.
//
// If the recorder is nil, the middleware is a no-op pass-through.
func MetricsMiddleware(recorder MetricsRecorder) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			if recorder == nil {
				return next(ctx, cmd)
			}

			start := time.Now()
			result, err := next(ctx, cmd)
			duration := time.Since(start)

			recorder.RecordOperation(ctx, cmd.Operation, duration, err)

			return result, err
		}
	}
}

// AuditMiddleware records audit entries for mutating operations.
// Each set, delete, or batch_set operation produces an AuditEntry
// that is forwarded to the AuditRecorder.
//
// The middleware uses the domain event.AuditEntry type with sensible defaults:
// values are redacted for secret keys, timestamps are UTC, and the actor
// defaults to "pipeline".
//
// If the recorder is nil, the middleware is a no-op pass-through.
func AuditMiddleware(recorder AuditRecorder) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			if recorder == nil {
				return next(ctx, cmd)
			}

			// Execute the command first so we can record success/failure.
			result, err := next(ctx, cmd)

			// Only audit mutating operations.
			if !isMutating(cmd.Operation) {
				return result, err
			}

			action := operationToAuditAction(cmd.Operation)
			cid := CorrelationIDFromContext(ctx)

			// Build the audit entry using domain constructors.
			entry := event.NewAuditEntry(action, cmd.Key, "pipeline", "pipeline")

			// Set trace ID from correlation ID if present.
			if cid != "" {
				entry = entry.WithTraceID(cid)
			}

			// Add command name as a label.
			labels := map[string]string{
				"command": cmd.Name,
			}
			if cmd.Operation == "batch_set" {
				labels["batch_size"] = strconv.Itoa(len(cmd.Values))
			}
			entry = entry.WithLabels(labels)

			// Record error if the operation failed.
			if err != nil {
				entry = entry.WithError(err).WithReason("command execution failed")
			} else {
				entry = entry.WithReason("command executed successfully")
			}

			// Add context about the new value for set operations.
			if cmd.Operation == "set" && cmd.Value != nil {
				newVal := value.New(cmd.Value)
				entry = entry.WithValues(value.Value{}, newVal)
			}

			recorder.RecordAudit(ctx, entry)

			return result, err
		}
	}
}

// LoggingMiddleware adds structured logging for command execution.
// Logs entry, exit, duration, and errors at appropriate levels.
// Uses the standard library slog for zero-dependency structured logging.
//
// If the logger is nil, the middleware is a no-op pass-through.
func LoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			if logger == nil {
				return next(ctx, cmd)
			}

			// Build base log attributes.
			baseAttrs := []any{
				"operation", cmd.Operation,
				"command", cmd.Name,
			}
			if cmd.Key != "" {
				baseAttrs = append(baseAttrs, "key", cmd.Key)
			}

			logger.InfoContext(ctx, "pipeline: executing command", baseAttrs...)

			start := time.Now()
			result, err := next(ctx, cmd)
			duration := time.Since(start)

			// Build result log attributes.
			resultAttrs := make([]any, len(baseAttrs))
			copy(resultAttrs, baseAttrs)
			resultAttrs = append(resultAttrs,
				"duration_ms", duration.Milliseconds(),
				"events", len(result.Events),
			)
			if result.Skipped {
				resultAttrs = append(resultAttrs, "skipped", true, "skip_reason", result.SkipReason)
			}

			if err != nil {
				resultAttrs = append(resultAttrs, "error", err.Error())
				// Use AppError code if available.
				if ae, ok := errors.AsAppError(err); ok {
					resultAttrs = append(resultAttrs,
						"error_code", ae.Code(),
						"error_severity", ae.Severity().String(),
					)
				}
				logger.ErrorContext(ctx, "pipeline: command failed", resultAttrs...)
			} else {
				logger.InfoContext(ctx, "pipeline: command completed", resultAttrs...)
			}

			return result, err
		}
	}
}

// RecoveryMiddleware recovers from panics in command execution,
// converting them into AppError values. This prevents a single
// command failure from crashing the entire application.
//
//nolint:nonamedreturns // We want to name the return values for clarity in the deferred recovery function.
func RecoveryMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (result Result, err error) {
			// Defer recovery to catch panics in the entire handler chain.
			defer func() {
				if r := recover(); r != nil {
					// Convert panic to AppError.
					panicMsg := fmt.Sprintf("panic recovered in command %q (operation=%s): %v",
						cmd.Name, cmd.Operation, r)

					// Include stack trace in the error for debugging.
					stack := string(debug.Stack())
					appErr := errors.Build(
						errors.CodePipeline,
						panicMsg,
						errors.WithSeverity(errors.SeverityCritical),
					)

					result = Result{
						Skipped:    false,
						SkipReason: "",
						Events:     nil,
					}
					err = fmt.Errorf("%w\nstack:\n%s", appErr, stack)
				}
			}()

			return next(ctx, cmd)
		}
	}
}

// CorrelationIDMiddleware ensures a correlation ID exists in the context.
// If the context already has a correlation ID, it is preserved.
// Otherwise, a new one is generated from command metadata.
func CorrelationIDMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			// Check if correlation ID already exists.
			if CorrelationIDFromContext(ctx) == "" {
				id := generateCorrelationID(cmd)
				ctx = ContextWithCorrelationID(ctx, id)
			}

			return next(ctx, cmd)
		}
	}
}

// generateCorrelationID creates a correlation ID from command metadata.
//
//nolint:gocritic // Command is passed by value intentionally for immutability across middleware
func generateCorrelationID(cmd Command) string {
	return fmt.Sprintf("%s-%d", cmd.Operation, time.Now().UnixNano())
}
