// Package errors provides a structured error type for the configuration library.
// ConfigError carries a machine-readable error Code, human-readable Message,
// optional Key, Source, Operation, Path context, a wrapped Cause error, and
// a captured stack trace for debugging.
//
// Error codes are defined as constants and support error matching via errors.Is
// and the IsCode helper function.
package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Code is a machine-readable error category for configuration errors.
type Code uint16

const (
	// CodeUnknown indicates an uncategorized error.
	CodeUnknown Code = iota
	// CodeNotFound indicates a requested key or resource was not found.
	CodeNotFound
	// CodeTypeMismatch indicates a value could not be converted to the requested type.
	CodeTypeMismatch
	// CodeInvalidFormat indicates data has an invalid format.
	CodeInvalidFormat
	// CodeParseError indicates a parsing failure.
	CodeParseError
	// CodeValidation indicates a validation rule was violated.
	CodeValidation
	// CodeSource indicates an error from a configuration source (file, remote, etc.).
	CodeSource
	// CodeCrypto indicates an encryption or decryption failure.
	CodeCrypto
	// CodeWatch indicates an error during file or remote watching.
	CodeWatch
	// CodeBind indicates a failure to bind a value to a struct field.
	CodeBind
	// CodeContextCanceled indicates the operation was cancelled via context.
	CodeContextCanceled
	// CodeTimeout indicates an operation exceeded its deadline.
	CodeTimeout
	// CodeInvalidConfig indicates invalid configuration parameters.
	CodeInvalidConfig
	// CodeAlreadyExists indicates a duplicate registration.
	CodeAlreadyExists
	// CodePermissionDenied indicates insufficient permissions.
	CodePermissionDenied
	// CodeNotImplemented indicates an operation is not yet supported.
	CodeNotImplemented
	// CodeConnection indicates a network connectivity error.
	CodeConnection
	// CodeConflict indicates a version or state conflict.
	CodeConflict
	// CodeInternal indicates an unexpected internal error.
	CodeInternal
	// CodeClosed indicates an operation on a closed resource.
	CodeClosed
)

// codeStrings maps Code constants to their string representations.
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

// String returns the human-readable name of the error code.
func (c Code) String() string {
	if int(c) < len(codeStrings) && codeStrings[c] != "" {
		return codeStrings[c]
	}
	return "unknown"
}

// ConfigError is a structured error with a code, message, context fields,
// optional cause, and captured stack trace. It implements the error interface
// and supports errors.Is matching by code.
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

// New creates a new ConfigError with the given code and message.
// A stack trace is captured at the point of creation.
func New(code Code, msg string) *ConfigError {
	return &ConfigError{Code: code, Message: msg, stack: captureStack(2)}
}

// Newf creates a new ConfigError with a formatted message.
func Newf(code Code, format string, args ...any) *ConfigError {
	return New(code, fmt.Sprintf(format, args...))
}

// Wrap creates a new ConfigError that wraps an existing cause error.
// Returns nil if cause is nil.
func Wrap(cause error, code Code, msg string) *ConfigError {
	if cause == nil {
		return nil
	}
	return &ConfigError{Code: code, Message: msg, Cause: cause, stack: captureStack(2)}
}

// WithKey returns a copy of the error with the Key field set.
func (e *ConfigError) WithKey(key string) *ConfigError {
	c := *e
	c.Key = key
	return &c
}

// WithSource returns a copy of the error with the Source field set.
func (e *ConfigError) WithSource(src string) *ConfigError {
	c := *e
	c.Source = src
	return &c
}

// WithOperation returns a copy of the error with the Operation field set.
func (e *ConfigError) WithOperation(op string) *ConfigError {
	c := *e
	c.Operation = op
	return &c
}

// WithPath returns a copy of the error with the Path field set.
func (e *ConfigError) WithPath(path string) *ConfigError {
	c := *e
	c.Path = path
	return &c
}

// Error returns a human-readable error string including the operation, message,
// and any context fields.
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

// Unwrap returns the underlying cause error for errors.Unwrap support.
func (e *ConfigError) Unwrap() error { return e.Cause }

// Is implements errors.Is matching by Code.
func (e *ConfigError) Is(target error) bool {
	var t *ConfigError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// Stack returns a formatted stack trace captured at the point of error creation.
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

// Common error sentinels.

var (
	// ErrClosed is returned when operating on a closed resource.
	ErrClosed = &ConfigError{Code: CodeClosed, Message: "config is closed"}
	// ErrNotFound is returned when a requested key does not exist.
	ErrNotFound = &ConfigError{Code: CodeNotFound, Message: "key not found"}
	// ErrTypeMismatch is returned when a value's type does not match expectations.
	ErrTypeMismatch = &ConfigError{Code: CodeTypeMismatch, Message: "type mismatch"}
	// ErrInvalidKey is returned when a configuration key is invalid.
	ErrInvalidKey = &ConfigError{Code: CodeInvalidConfig, Message: "invalid key"}
	// ErrNotImplemented is returned when an operation is not yet supported.
	ErrNotImplemented = &ConfigError{Code: CodeNotImplemented, Message: "not implemented"}
	// ErrPermission is returned when access is denied.
	ErrPermission = &ConfigError{Code: CodePermissionDenied, Message: "permission denied"}
	// ErrDecryptFailed is returned when decryption fails.
	ErrDecryptFailed = &ConfigError{Code: CodeCrypto, Message: "decryption failed"}
)

// WrapSource is a convenience function that wraps an error as a CodeSource error
// with source name and operation context. Returns nil if err is nil.
func WrapSource(err error, sourceName, operation string) error {
	if err == nil {
		return nil
	}
	return Wrap(err, CodeSource, "source error").
		WithSource(sourceName).
		WithOperation(operation)
}

// IsCode reports whether err (or any wrapped error) is a ConfigError with the given code.
func IsCode(err error, code Code) bool {
	var ce *ConfigError
	if errors.As(err, &ce) {
		return ce.Code == code
	}
	return false
}

// captureStack captures a stack trace, skipping the specified number of frames.
func captureStack(skip int) []uintptr {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+2, pcs)
	return pcs[:n]
}
