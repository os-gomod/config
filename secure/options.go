package secure

import (
	"log/slog"

	configerrors "github.com/os-gomod/config/errors"
)

// secureOptions holds configuration options for secure package components.
type secureOptions struct {
	logger   *slog.Logger
	priority int
}

// Option is a functional option for configuring secure package components.
type Option func(*secureOptions)

// WithLogger sets the structured logger for secure components.
func WithLogger(l *slog.Logger) Option {
	return func(o *secureOptions) { o.logger = l }
}

// WithPriority sets the value priority for secure components.
func WithPriority(p int) Option {
	return func(o *secureOptions) { o.priority = p }
}

// defaultSecureOptions returns the default secure options with
// slog.Default() logger and priority 50.
func defaultSecureOptions() secureOptions {
	return secureOptions{
		logger:   slog.Default(),
		priority: 50,
	}
}

// ErrNotImplemented is returned by stub provider implementations.
var ErrNotImplemented = configerrors.ErrNotImplemented

// ErrClosed is returned when operating on a closed component.
var ErrClosed = configerrors.ErrClosed
