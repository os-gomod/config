package secure

import (
	"context"
	"log/slog"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// SecureSource wraps a SecureStore as a loader.Loader so that secrets can be
// consumed through the standard layer stack.
type SecureSource struct {
	*common.Closable
	store    *SecureStore
	priority int
	logger   *slog.Logger
}

var _ interface {
	Load(context.Context) (map[string]value.Value, error)
	Watch(context.Context) (<-chan event.Event, error)
	Priority() int
	String() string
	Close(context.Context) error
} = (*SecureSource)(nil)

// NewSecureSource creates a SecureSource backed by the given SecureStore.
func NewSecureSource(store *SecureStore, opts ...Option) *SecureSource {
	o := defaultSecureOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return &SecureSource{
		Closable: common.NewClosable(),
		store:    store,
		priority: o.priority,
		logger:   o.logger,
	}
}

// Load implements loader.Loader. It delegates to the underlying SecureStore.
func (s *SecureSource) Load(_ context.Context) (map[string]value.Value, error) {
	if s.IsClosed() {
		return nil, ErrClosed
	}
	return s.store.Load(nil)
}

// Watch implements loader.Loader. SecureSource does not support watching;
// it always returns (nil, nil).
func (s *SecureSource) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Priority returns the merge priority of this source.
func (s *SecureSource) Priority() int { return s.priority }

// String returns a human-readable identifier for this source.
func (s *SecureSource) String() string { return "secure" }

// Close implements loader.Loader.
func (s *SecureSource) Close(ctx context.Context) error { return s.Closable.Close(ctx) }
