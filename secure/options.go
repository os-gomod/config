package secure

import (
	"log/slog"

	configerrors "github.com/os-gomod/config/errors"
)

// secureOptions holds configuration shared across SecureStore, SecureSource,
// VaultProvider, and KMSProvider.
type secureOptions struct {
	logger   *slog.Logger
	priority int
}

// Option configures a secure type.
type Option func(*secureOptions)

// WithLogger sets the structured logger used for stub warnings.
// The default is slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(o *secureOptions) { o.logger = l }
}

// WithPriority sets the merge priority for the secure source.
func WithPriority(p int) Option {
	return func(o *secureOptions) { o.priority = p }
}

// defaultSecureOptions returns secureOptions with sensible defaults.
func defaultSecureOptions() secureOptions {
	return secureOptions{
		logger:   slog.Default(),
		priority: 50,
	}
}

// ErrNotImplemented is the sentinel error returned by stub providers.
var ErrNotImplemented = configerrors.ErrNotImplemented

// ErrClosed is the sentinel error returned when operating on a closed secure source.
var ErrClosed = configerrors.ErrClosed
