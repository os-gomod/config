package loader

import _errors "github.com/os-gomod/config/errors"

var (
	// ErrClosed is returned when operating on a closed loader.
	ErrClosed = _errors.ErrClosed
	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = _errors.ErrNotFound
	// ErrNotSupported is returned when an operation is not supported by the loader.
	ErrNotSupported = _errors.New(_errors.CodeNotImplemented, "operation not supported")
)

// WrapErrors wraps an error with source name and operation context.
func WrapErrors(err error, sourceName, operation string) error {
	if err == nil {
		return nil
	}
	wrapped := _errors.Wrap(err, _errors.CodeSource, "source error")
	if wrapped != nil {
		wrapped = wrapped.WithSource(sourceName).WithOperation(operation)
	}
	return wrapped
}
