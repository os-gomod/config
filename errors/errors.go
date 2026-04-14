// Package errors defines typed config errors with error codes and stack traces.
package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Code represents a numeric error category used to classify config errors.
type Code uint16

const (
	// CodeUnknown indicates an unclassified error.
	CodeUnknown Code = iota
	// CodeNotFound indicates a requested key was not found.
	CodeNotFound
	// CodeTypeMismatch indicates a value could not be converted to the requested type.
	CodeTypeMismatch
	// CodeInvalidFormat indicates data is in an unexpected format.
	CodeInvalidFormat
	// CodeParseError indicates a parsing failure.
	CodeParseError
	// CodeValidation indicates a validation rule was violated.
	CodeValidation
	// CodeSource indicates a source-level failure.
	CodeSource
	// CodeCrypto indicates a cryptographic operation failure.
	CodeCrypto
	// CodeWatch indicates a watch/subscription failure.
	CodeWatch
	// CodeBind indicates a binding failure.
	CodeBind
	// CodeContextCanceled indicates the operation was canceled via context.
	CodeContextCanceled
	// CodeTimeout indicates the operation exceeded its deadline.
	CodeTimeout
	// CodeInvalidConfig indicates the configuration is structurally invalid.
	CodeInvalidConfig
	// CodeAlreadyExists indicates a duplicate key or resource.
	CodeAlreadyExists
	// CodePermissionDenied indicates insufficient permissions.
	CodePermissionDenied
	// CodeNotImplemented indicates the operation is not supported.
	CodeNotImplemented
	// CodeConnection indicates a network or connection failure.
	CodeConnection
	// CodeConflict indicates a state conflict.
	CodeConflict
	// CodeInternal indicates an internal invariant violation.
	CodeInternal
	// CodeClosed indicates the config instance is closed.
	CodeClosed
)

var codeStrings = [...]string{
	CodeUnknown:          "unknown",
	CodeNotFound:         "not_found",
	CodeTypeMismatch:     "type_mismatch",
	CodeInvalidFormat:    "invalid_format",
	CodeParseError:       "parse_error",
	CodeValidation:       "validation_error",
	CodeSource:           "source_error",
	CodeCrypto:           "crypto_error",
	CodeWatch:            "watch_error",
	CodeBind:             "bind_error",
	CodeContextCanceled:  "context_canceled",
	CodeTimeout:          "timeout",
	CodeInvalidConfig:    "invalid_config",
	CodeAlreadyExists:    "already_exists",
	CodePermissionDenied: "permission_denied",
	CodeNotImplemented:   "not_implemented",
	CodeConnection:       "connection_error",
	CodeConflict:         "conflict",
	CodeInternal:         "internal_error",
	CodeClosed:           "closed",
}

// String returns the human-readable name for the error code.
func (c Code) String() string {
	if int(c) < len(codeStrings) && codeStrings[c] != "" {
		return codeStrings[c]
	}
	return "unknown"
}

// ConfigError is a structured error type that carries a machine-readable code,
// a human-readable message, optional context fields, and a captured stack trace.
type ConfigError struct {
	Code      Code
	Message   string
	Key       string
	Source    string
	Operation string
	Path      string
	Cause     error
	stack     []uintptr
}

// New creates a ConfigError with the given code and message, capturing a stack trace.
func New(code Code, msg string) *ConfigError {
	return &ConfigError{Code: code, Message: msg, stack: captureStack(2)}
}

// Newf creates a ConfigError with a formatted message, capturing a stack trace.
func Newf(code Code, format string, args ...any) *ConfigError {
	return New(code, fmt.Sprintf(format, args...))
}

// Wrap creates a ConfigError that wraps cause with the given code and message.
// If cause is nil, Wrap returns nil.
func Wrap(cause error, code Code, msg string) *ConfigError {
	if cause == nil {
		return nil
	}
	return &ConfigError{Code: code, Message: msg, Cause: cause, stack: captureStack(2)}
}

// WithKey returns a copy of the error with the key field set.
func (e *ConfigError) WithKey(key string) *ConfigError {
	c := *e
	c.Key = key
	return &c
}

// WithSource returns a copy of the error with the source field set.
func (e *ConfigError) WithSource(src string) *ConfigError {
	c := *e
	c.Source = src
	return &c
}

// WithOperation returns a copy of the error with the operation field set.
func (e *ConfigError) WithOperation(op string) *ConfigError {
	c := *e
	c.Operation = op
	return &c
}

// WithPath returns a copy of the error with the path field set.
func (e *ConfigError) WithPath(path string) *ConfigError {
	c := *e
	c.Path = path
	return &c
}

// Error formats the error into a human-readable string including operation,
// message, key, source, path, and cause.
func (e *ConfigError) Error() string {
	var sb strings.Builder
	if e.Operation != "" {
		sb.WriteString(e.Operation)
		sb.WriteString(": ")
	}
	sb.WriteString(e.Message)
	if e.Key != "" {
		sb.WriteString(" [key=")
		sb.WriteString(e.Key)
		sb.WriteByte(']')
	}
	if e.Source != "" {
		sb.WriteString(" [source=")
		sb.WriteString(e.Source)
		sb.WriteByte(']')
	}
	if e.Path != "" {
		sb.WriteString(" [path=")
		sb.WriteString(e.Path)
		sb.WriteByte(']')
	}
	if e.Cause != nil {
		sb.WriteString(": ")
		sb.WriteString(e.Cause.Error())
	}
	return sb.String()
}

// Unwrap returns the underlying cause for errors.Is/errors.As traversal.
func (e *ConfigError) Unwrap() error { return e.Cause }

// Is reports whether the target error matches this ConfigError by code.
func (e *ConfigError) Is(target error) bool {
	var t *ConfigError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// Stack returns a formatted stack trace captured at the point the error was created.
func (e *ConfigError) Stack() string {
	frames := runtime.CallersFrames(e.stack)
	var sb strings.Builder
	for {
		f, more := frames.Next()
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", f.Function, f.File, f.Line)
		if !more {
			break
		}
	}
	return sb.String()
}

// ErrClosed is the sentinel error returned when operating on a closed config instance.
var ErrClosed = &ConfigError{Code: CodeClosed, Message: "config is closed"}

// ErrNotFound is the sentinel error returned when a key is not found.
var ErrNotFound = &ConfigError{Code: CodeNotFound, Message: "key not found"}

// ErrTypeMismatch is the sentinel error returned on type conversion failure.
var ErrTypeMismatch = &ConfigError{Code: CodeTypeMismatch, Message: "type mismatch"}

// ErrInvalidKey is the sentinel error returned for malformed keys.
var ErrInvalidKey = &ConfigError{Code: CodeInvalidConfig, Message: "invalid key"}

// ErrNotImplemented is the sentinel error returned for unsupported operations.
var ErrNotImplemented = &ConfigError{Code: CodeNotImplemented, Message: "not implemented"}

// ErrPermission is the sentinel error returned on permission denial.
var ErrPermission = &ConfigError{Code: CodePermissionDenied, Message: "permission denied"}

// ErrDecryptFailed is the sentinel error returned on decryption failure.
var ErrDecryptFailed = &ConfigError{Code: CodeCrypto, Message: "decryption failed"}

// WrapSource wraps a source-level error with the source name and operation context.
func WrapSource(err error, sourceName, operation string) error {
	if err == nil {
		return nil
	}
	return Wrap(err, CodeSource, "source error").
		WithSource(sourceName).
		WithOperation(operation)
}

// IsCode reports whether err is a ConfigError with the given code.
func IsCode(err error, code Code) bool {
	var ce *ConfigError
	if errors.As(err, &ce) {
		return ce.Code == code
	}
	return false
}

// captureStack captures a stack trace, skipping the caller plus the skip offset.
// The skip+2 offset accounts for captureStack itself and runtime.Callers.
func captureStack(skip int) []uintptr {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+2, pcs)
	return pcs[:n]
}
