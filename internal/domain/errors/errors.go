// Package errors provides structured error types for the config library.
// All major flows return AppError instances with machine-readable codes,
// severity levels, correlation IDs, and rich context for tracing.
package errors

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ---------------------------------------------------------------------------
// Severity
// ---------------------------------------------------------------------------

// Severity indicates the seriousness of an error.
type Severity int

const (
	SeverityLow      Severity = iota // Low: informational, non-blocking
	SeverityMedium                   // Medium: degraded behavior
	SeverityHigh                     // High: significant failure
	SeverityCritical                 // Critical: data loss / unavailable
)

func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// ---------------------------------------------------------------------------
// Error codes
// ---------------------------------------------------------------------------

const (
	CodeUnknown          = "unknown"
	CodeNotFound         = "not_found"
	CodeTypeMismatch     = "type_mismatch"
	CodeInvalidFormat    = "invalid_format"
	CodeParseError       = "parse_error"
	CodeValidation       = "validation_error"
	CodeSource           = "source_error"
	CodeCrypto           = "crypto_error"
	CodeWatch            = "watch_error"
	CodeBind             = "bind_error"
	CodeContextCanceled  = "context_canceled"
	CodeTimeout          = "timeout"
	CodeInvalidConfig    = "invalid_config"
	CodeAlreadyExists    = "already_exists"
	CodePermissionDenied = "permission_denied"
	CodeNotImplemented   = "not_implemented"
	CodeConnection       = "connection_error"
	CodeConflict         = "conflict"
	CodeInternal         = "internal_error"
	CodeClosed           = "closed"
	CodeLayer            = "layer_error"
	CodePipeline         = "pipeline_error"
	CodeInterceptor      = "interceptor_error"
)

// ---------------------------------------------------------------------------
// AppError interface
// ---------------------------------------------------------------------------

// AppError is the structured error interface all major flows must return.
//
//nolint:interfacebloat // this is the package's stable structured error contract
type AppError interface {
	error
	// Code returns the machine-readable error code.
	Code() string
	// Message returns the human-readable error description.
	Message() string
	// Retryable returns true if the operation may succeed on retry.
	Retryable() bool
	// Severity returns the error severity level.
	Severity() Severity
	// CorrelationID returns a unique identifier for tracing.
	CorrelationID() string
	// Operation returns the operation that caused the error.
	Operation() string
	Key() string
	Source() string
	// WithKey returns a copy with an associated config key.
	WithKey(key string) AppError
	// WithOperation returns a copy with the operation name set.
	WithOperation(op string) AppError
	// WithSource returns a copy with an associated source name.
	WithSource(src string) AppError
	// Wrap returns a new AppError wrapping a cause.
	Wrap(cause error) AppError
	// Unwrap returns the underlying cause (supports errors.Unwrap).
	Unwrap() error
	// Is supports error.Is by matching on error code.
	Is(target error) bool
	// As supports error.As extraction.
	As(target any) bool
	// StackTrace returns the captured stack trace pointers.
	StackTrace() []uintptr
	// StackFrames returns the captured stack trace as runtime.Frames.
	StackFrames() []runtime.Frame
}

// ---------------------------------------------------------------------------
// configError — concrete implementation
// ---------------------------------------------------------------------------

// configError implements AppError with full context capture.
type configError struct {
	code          string
	message       string
	severity      Severity
	retryable     bool
	correlationID string
	operation     string
	key           string
	source        string
	cause         error
	stack         []uintptr
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// New creates a new AppError with the given code and message.
func New(code, message string) AppError {
	return &configError{
		code:          code,
		message:       message,
		severity:      SeverityMedium,
		correlationID: newCorrelationID(),
		stack:         captureStack(3),
	}
}

// Newf creates a new AppError with a formatted message.
func Newf(code, format string, args ...any) AppError {
	return New(code, fmt.Sprintf(format, args...))
}

// Wrap creates a new AppError wrapping a cause error.
func Wrap(cause error, code, message string) AppError {
	if cause == nil {
		return nil
	}
	ae := &configError{
		code:          code,
		message:       message,
		severity:      SeverityMedium,
		correlationID: newCorrelationID(),
		cause:         cause,
		stack:         captureStack(3),
	}
	// Propagate retryable and metadata from wrapped AppError.
	if wrapped, ok := AsAppError(cause); ok {
		ae.retryable = wrapped.Retryable()
		ae.operation = wrapped.Operation()
		if ce, ok2 := wrapped.(*configError); ok2 {
			ae.key = ce.key
			ae.source = ce.source
		}
	}
	return ae
}

// ---------------------------------------------------------------------------
// AppError methods
// ---------------------------------------------------------------------------

func (e *configError) Error() string {
	var b strings.Builder
	if e.code != "" {
		b.WriteString("[")
		b.WriteString(e.code)
		b.WriteString("] ")
	}
	b.WriteString(e.message)
	if e.key != "" {
		b.WriteString(" (key=")
		b.WriteString(e.key)
		b.WriteString(")")
	}
	if e.source != "" {
		b.WriteString(" (source=")
		b.WriteString(e.source)
		b.WriteString(")")
	}
	if e.operation != "" {
		b.WriteString(" (op=")
		b.WriteString(e.operation)
		b.WriteString(")")
	}
	if e.cause != nil {
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}
	return b.String()
}

func (e *configError) Code() string          { return e.code }
func (e *configError) Message() string       { return e.message }
func (e *configError) Retryable() bool       { return e.retryable }
func (e *configError) Severity() Severity    { return e.severity }
func (e *configError) CorrelationID() string { return e.correlationID }
func (e *configError) Operation() string     { return e.operation }

// Key returns the config key associated with this error (unexported accessor
// used internally by Wrap propagation).
func (e *configError) Key() string { return e.key }

// Source returns the source name associated with this error (unexported accessor).
func (e *configError) Source() string { return e.source }

func (e *configError) WithKey(key string) AppError {
	cp := *e
	cp.key = key
	return &cp
}

func (e *configError) WithSource(src string) AppError {
	cp := *e
	cp.source = src
	return &cp
}

// WithOperation returns a copy of the error with the operation name set.
func (e *configError) WithOperation(op string) AppError {
	cp := *e
	cp.operation = op
	return &cp
}

func (e *configError) Wrap(cause error) AppError {
	if cause == nil {
		return e
	}
	cp := *e
	cp.cause = cause
	cp.stack = captureStack(3)
	return &cp
}

// ---------------------------------------------------------------------------
// error.Is / error.As support
// ---------------------------------------------------------------------------

// Is supports error.Is by matching on error code.
// If target is a string, it matches against the code.
// If target is an AppError, it matches on code equality.
func (e *configError) Is(target error) bool {
	switch t := target.(type) {
	case *configError:
		return e.code == t.code
	case AppError:
		return e.code == t.Code()
	default:
		return false
	}
}

// As supports error.As extraction — AppError targets are matched directly.
func (e *configError) As(target any) bool {
	if p, ok := target.(*AppError); ok {
		*p = e
		return true
	}
	return false
}

// AsConfigError supports error.As extraction for *configError targets.
func AsConfigError[T any](err error) (T, bool) {
	var target T
	ok := errors.As(err, &target)
	return target, ok
}

// ---------------------------------------------------------------------------
// Unwrap supports errors.Unwrap.
func (e *configError) Unwrap() error { return e.cause }

// ---------------------------------------------------------------------------
// Stack trace helpers
// ---------------------------------------------------------------------------

func (e *configError) StackTrace() []uintptr {
	return append([]uintptr(nil), e.stack...)
}

func (e *configError) StackFrames() []runtime.Frame {
	if len(e.stack) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(e.stack)
	var result []runtime.Frame
	for {
		frame, more := frames.Next()
		result = append(result, frame)
		if !more {
			break
		}
	}
	return result
}

// Format implements fmt.Formatter to print stack traces with %+v.
func (e *configError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%s\n", e.Error())
			frames := e.StackFrames()
			for _, f := range frames {
				fmt.Fprintf(s, "  %s\n    %s:%d\n", f.Function, f.File, f.Line)
			}
			return
		}
		fallthrough
	case 's':
		fmt.Fprintf(s, "%s", e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// captureStack captures the call stack, skipping skip frames.
func captureStack(skip int) []uintptr {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip, pcs)
	return pcs[:n]
}

// newCorrelationID generates a unique 16-byte hex correlation ID using crypto/rand.
func newCorrelationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Functional option setters (for builder-style construction)
// ---------------------------------------------------------------------------

// WithRetryable marks the error as retryable.
func WithRetryable(retryable bool) func(AppError) {
	return func(e AppError) {
		if ce, ok := e.(*configError); ok {
			ce.retryable = retryable
		}
	}
}

// WithSeverity sets the severity of the error.
func WithSeverity(s Severity) func(AppError) {
	return func(e AppError) {
		if ce, ok := e.(*configError); ok {
			ce.severity = s
		}
	}
}

// WithOperation sets the operation name on the error.
func WithOperation(op string) func(AppError) {
	return func(e AppError) {
		if ce, ok := e.(*configError); ok {
			ce.operation = op
		}
	}
}

// Build creates a new AppError and applies optional modifiers.
func Build(code, message string, opts ...func(AppError)) AppError {
	e := New(code, message)
	for _, o := range opts {
		o(e)
	}
	return e
}

// ---------------------------------------------------------------------------
// AsAppError extracts an AppError from a chain of wrapped errors.
// Returns the AppError and true if found, nil and false otherwise.
// ---------------------------------------------------------------------------

func AsAppError(err error) (AppError, bool) {
	if err == nil {
		return nil, false
	}
	// Direct type assertion first.
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	// Walk the unwrap chain.
	for unwrapped := err; unwrapped != nil; {
		var appErrr AppError
		if errors.As(unwrapped, &appErrr) {
			return appErrr, true
		}
		u, ok := unwrapped.(interface{ Unwrap() error })
		if !ok {
			break
		}
		unwrapped = u.Unwrap()
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Predefined sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrClosed       = New(CodeClosed, "config is closed")
	ErrNotFound     = New(CodeNotFound, "key not found")
	ErrTypeMismatch = New(CodeTypeMismatch, "type mismatch")
	ErrInvalidKey   = New(CodeInvalidConfig, "invalid key")

	// Event bus sentinel errors.
	ErrBusClosed      = New("bus_closed", "event bus is closed")
	ErrQueueFull      = New("queue_full", "event queue is full")
	ErrInvalidPattern = New("invalid_pattern", "invalid subscription pattern")
	ErrNilObserver    = New("nil_observer", "observer must not be nil")
	ErrDeliveryFailed = New("delivery_failed", "event delivery failed after retries")
)
