package loader

import (
	"context"
	"sync"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

// MemoryLoader holds config in memory. It is primarily used in tests
// and for runtime overrides injected by the application.
type MemoryLoader struct {
	*Base
	mu   sync.RWMutex
	data map[string]value.Value
}

var _ Loader = (*MemoryLoader)(nil)

// MemoryOption configures a MemoryLoader during creation.
type MemoryOption func(*MemoryLoader)

// WithMemoryData initializes the loader with the given data map.
// Values are converted to value.Value with inferred types and SourceMemory.
func WithMemoryData(data map[string]any) MemoryOption {
	return func(m *MemoryLoader) {
		converted := make(map[string]value.Value, len(data))
		for k, v := range data {
			converted[k] = value.New(v, value.InferType(v), value.SourceMemory, m.Priority())
		}
		m.data = converted
	}
}

// WithMemoryPriority sets the merge priority.
func WithMemoryPriority(p int) MemoryOption {
	return func(m *MemoryLoader) {
		m.SetPriority(p)
		if len(m.data) == 0 {
			return
		}
		updated := make(map[string]value.Value, len(m.data))
		for k, v := range m.data {
			updated[k] = value.New(v.Raw(), v.Type(), value.SourceMemory, p)
		}
		m.data = updated
	}
}

// NewMemoryLoader creates a MemoryLoader with the given options.
func NewMemoryLoader(opts ...MemoryOption) *MemoryLoader {
	m := &MemoryLoader{
		Base: NewBase("memory", "memory", 20),
		data: make(map[string]value.Value),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Load implements Loader. It returns a safe copy of the current data.
func (m *MemoryLoader) Load(_ context.Context) (map[string]value.Value, error) {
	if m.IsClosed() {
		return nil, ErrClosed
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return value.Copy(m.data), nil
}

// Update replaces the in-memory data atomically. Values are converted with
// inferred types and SourceMemory.
func (m *MemoryLoader) Update(data map[string]any) {
	converted := make(map[string]value.Value, len(data))
	for k, v := range data {
		converted[k] = value.New(v, value.InferType(v), value.SourceMemory, m.Priority())
	}
	m.mu.Lock()
	m.data = converted
	m.mu.Unlock()
}

// Watch implements Loader. MemoryLoader does not support watching;
// it always returns (nil, nil).
func (m *MemoryLoader) Watch(_ context.Context) (<-chan event.Event, error) { return nil, nil }

// Close implements Loader.
func (m *MemoryLoader) Close(ctx context.Context) error { return m.Base.Close(ctx) }
