package loader

import (
	"fmt"

	"github.com/os-gomod/config/v2/internal/domain/errors"
)

// WrapErrors wraps an error with loader-specific context using the domain
// AppError type. This provides structured error codes, severity levels,
// correlation IDs, and source metadata for consistent error handling across
// the loader layer.
func WrapErrors(err error, name, operation string) error {
	if err == nil {
		return nil
	}

	// If the error is already an AppError, enrich it with source info.
	if appErr, ok := errors.AsAppError(err); ok {
		return appErr.WithSource(name)
	}

	// Otherwise, wrap it in a new AppError with full context.
	return errors.Build(errors.CodeSource,
		fmt.Sprintf("loader %q: %s failed", name, operation),
		errors.WithOperation(operation)).Wrap(err)
}
