package loader

import (
	"context"
	"sync"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
)

// MemoryLoader provides an in-memory configuration source. It is primarily
// useful for tests, defaults, and programmatic configuration.
//
// Data can be set at creation via WithMemoryData and updated at runtime
// via the Update method. All values are sourced from memory with a default
// priority of 20.
//
// Example:
//
//	ml := loader.NewMemoryLoader(
//	    loader.WithMemoryData(map[string]any{"db.host": "localhost"}),
//	    loader.WithMemoryPriority(50),
//	)
type MemoryLoader struct {
	*Base
	mu   sync.RWMutex
	data map[string]value.Value
}

var _ Loader = (*MemoryLoader)(nil)

// MemoryOption configures a MemoryLoader.
type MemoryOption func(*MemoryLoader)

// WithMemoryData sets the initial data for the MemoryLoader.
// Values are converted to value.Value with SourceMemory.
func WithMemoryData(data map[string]any) MemoryOption {
	return func(m *MemoryLoader) {
		converted := make(map[string]value.Value, len(data))
		for k, v := range data {
			converted[k] = value.New(v, value.InferType(v), value.SourceMemory, m.Priority())
		}
		m.data = converted
	}
}

// WithMemoryPriority sets the priority for the MemoryLoader's values.
// If data already exists, all existing values are re-created with the new priority.
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

// NewMemoryLoader creates a new MemoryLoader with the given options.
// Default priority is 20.
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

// Load returns a copy of the current in-memory data.
func (m *MemoryLoader) Load(_ context.Context) (map[string]value.Value, error) {
	if m.IsClosed() {
		return nil, ErrClosed
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return value.Copy(m.data), nil
}

// Update replaces the in-memory data with the given map.
// Values are converted to value.Value with SourceMemory.
func (m *MemoryLoader) Update(data map[string]any) {
	converted := make(map[string]value.Value, len(data))
	for k, v := range data {
		converted[k] = value.New(v, value.InferType(v), value.SourceMemory, m.Priority())
	}
	m.mu.Lock()
	m.data = converted
	m.mu.Unlock()
}

// Watch returns nil since MemoryLoader does not support change watching.
func (m *MemoryLoader) Watch(_ context.Context) (<-chan event.Event, error) { return nil, nil }

// Close releases resources held by the MemoryLoader.
func (m *MemoryLoader) Close(ctx context.Context) error { return m.Base.Close(ctx) }
