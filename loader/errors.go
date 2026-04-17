package loader

import configerrors "github.com/os-gomod/config/errors"

// Common error variables for the loader package.

var (
	// ErrClosed is returned when operating on a closed loader.
	ErrClosed = configerrors.ErrClosed
	// ErrNotFound is returned when a requested key or resource is not found.
	ErrNotFound = configerrors.ErrNotFound
	// ErrNotSupported is returned when an operation is not supported by the loader.
	ErrNotSupported = configerrors.New(configerrors.CodeNotImplemented, "operation not supported")
)

// WrapErrors wraps an error with source name and operation context using
// the errors package. Returns nil if err is nil.
func WrapErrors(err error, sourceName, operation string) error {
	if err == nil {
		return nil
	}
	wrapped := configerrors.Wrap(err, configerrors.CodeSource, "source error")
	if wrapped != nil {
		wrapped = wrapped.WithSource(sourceName).WithOperation(operation)
	}
	return wrapped
}
