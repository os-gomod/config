package secure

import (
	"context"
	"log/slog"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/internal/common"
)

// Source wraps a Store as a configuration source that can be
// used as a loader layer. It decrypts all values on Load and provides
// them as typed string values with SourceVault.
type Source struct {
	*common.Closable
	store    *Store
	priority int
	logger   *slog.Logger
}

var _ interface {
	Load(context.Context) (map[string]value.Value, error)
	Watch(context.Context) (<-chan event.Event, error)
	Priority() int
	String() string
	Close(context.Context) error
} = (*Source)(nil)

// NewSource creates a new Source backed by the given Store.
func NewSource(store *Store, opts ...Option) *Source {
	o := defaultSecureOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return &Source{
		Closable: common.NewClosable(),
		store:    store,
		priority: o.priority,
		logger:   o.logger,
	}
}

// Load decrypts all values from the underlying Store and returns them
// as a map of value.Value. Returns ErrClosed if the source has been closed.
func (s *Source) Load(_ context.Context) (map[string]value.Value, error) {
	if s.IsClosed() {
		return nil, ErrClosed
	}
	return s.store.Load(nil)
}

// Watch returns nil since Source does not support change watching.
func (s *Source) Watch(_ context.Context) (<-chan event.Event, error) {
	return nil, nil
}

// Priority returns the priority of this secure source's values.
func (s *Source) Priority() int { return s.priority }

// String returns "secure".
func (s *Source) String() string { return "secure" }

// Close releases resources held by the Source.
func (s *Source) Close(ctx context.Context) error { return s.Closable.Close(ctx) }
