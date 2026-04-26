package loader

import (
	"context"
	"fmt"
	"sync"

	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// MemoryLoader
// ---------------------------------------------------------------------------

// MemoryLoader provides an in-memory configuration source for testing and
// programmatic config. It supports dynamic mutation via Set/Delete and
// emits change events through the Watch channel.
type MemoryLoader struct {
	*Base
	data map[string]value.Value
	mu   sync.RWMutex
	// watchCh holds subscriber channels for change notifications.
	watchMu     sync.Mutex
	subscribers []chan event.Event
}

// MemoryOption configures a MemoryLoader during construction.
type MemoryOption func(*MemoryLoader)

// WithMemoryData sets initial data for the loader.
func WithMemoryData(data map[string]any) MemoryOption {
	return func(m *MemoryLoader) {
		for k, v := range data {
			m.data[k] = value.New(v)
		}
	}
}

// WithMemoryPriority sets the loader priority.
func WithMemoryPriority(p int) MemoryOption {
	return func(m *MemoryLoader) {
		m.priority = p
	}
}

// NewMemoryLoader creates a new in-memory loader with the given initial data.
// The data map is deep-copied; the caller's map is not retained.
func NewMemoryLoader(name string, data map[string]any, opts ...MemoryOption) *MemoryLoader {
	m := &MemoryLoader{
		Base: NewBase(name, "memory", 0),
		data: make(map[string]value.Value),
	}

	for k, v := range data {
		m.data[k] = value.New(v)
	}

	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Load returns a copy of the current in-memory data.
func (m *MemoryLoader) Load(ctx context.Context) (map[string]value.Value, error) {
	if err := m.CheckClosed(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, m.WrapErr(ctx.Err(), "load")
	default:
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]value.Value, len(m.data))
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

// Watch returns a channel that receives events when the in-memory data changes.
func (m *MemoryLoader) Watch(ctx context.Context) (<-chan event.Event, error) {
	if err := m.CheckClosed(); err != nil {
		return nil, err
	}

	ch := make(chan event.Event, 16)

	m.watchMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.watchMu.Unlock()

	go func() {
		<-ctx.Done()
		m.removeSubscriber(ch)
		close(ch)
	}()

	return ch, nil
}

// Close implements Loader.Close and removes all subscribers.
func (m *MemoryLoader) Close(_ context.Context) error {
	_ = m.CloseBase()

	m.watchMu.Lock()
	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
	m.watchMu.Unlock()

	return nil
}

// Set adds or updates a key in the in-memory data and notifies watchers.
func (m *MemoryLoader) Set(key string, val any) {
	v := value.FromRaw(val, value.TypeUnknown, value.SourceMemory, m.priority)

	m.mu.Lock()
	old, existed := m.data[key]
	m.data[key] = v
	m.mu.Unlock()

	if existed {
		m.notify(event.NewUpdateEvent(key, old, v, event.WithSource(m.name)))
	} else {
		m.notify(event.NewCreateEvent(key, v, event.WithSource(m.name)))
	}
}

// Delete removes a key from the in-memory data and notifies watchers.
func (m *MemoryLoader) Delete(key string) {
	m.mu.Lock()
	old, existed := m.data[key]
	if !existed {
		m.mu.Unlock()
		return
	}
	delete(m.data, key)
	m.mu.Unlock()

	m.notify(event.NewDeleteEvent(key, old, event.WithSource(m.name)))
}

// Get returns the Value for the given key, or a zero Value if not found.
func (m *MemoryLoader) Get(key string) value.Value {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return value.Value{}
	}
	return v
}

// Len returns the number of keys in the in-memory data.
func (m *MemoryLoader) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Keys returns all keys in the in-memory data.
func (m *MemoryLoader) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return mapKeys(m.data)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// notify sends an event to all subscribers. Non-blocking; drops events
// if subscriber channels are full.
func (m *MemoryLoader) notify(evt event.Event) {
	m.watchMu.Lock()
	subs := make([]chan event.Event, len(m.subscribers))
	copy(subs, m.subscribers)
	m.watchMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
			// Drop event if channel is full to avoid blocking the writer.
		}
	}
}

// removeSubscriber removes a subscriber channel from the list.
func (m *MemoryLoader) removeSubscriber(ch chan event.Event) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()

	for i, sub := range m.subscribers {
		if sub == ch {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Batch operations
// ---------------------------------------------------------------------------

// SetMany adds or updates multiple keys at once and notifies watchers
// with a single reload event.
func (m *MemoryLoader) SetMany(data map[string]any) {
	m.mu.Lock()
	for k, v := range data {
		m.data[k] = value.FromRaw(v, value.TypeUnknown, value.SourceMemory, m.priority)
	}
	m.mu.Unlock()

	m.notify(event.NewReloadEvent(m.name,
		event.WithMetadata(map[string]any{
			"keys_count": len(data),
			"operation":  "batch_set",
		}),
	))
}

// Clear removes all keys and notifies watchers.
func (m *MemoryLoader) Clear() {
	m.mu.Lock()
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	m.data = make(map[string]value.Value)
	m.mu.Unlock()

	for _, k := range keys {
		m.notify(event.NewDeleteEvent(k, value.Value{}, event.WithSource(m.name)))
	}

	if len(keys) > 0 {
		m.notify(event.NewReloadEvent(m.name,
			event.WithMetadata(map[string]any{
				"keys_count": len(keys),
				"operation":  "clear",
			}),
		))
	}
}

// ---------------------------------------------------------------------------
// String helpers
// ---------------------------------------------------------------------------

// String returns a human-readable description of the memory loader.
func (m *MemoryLoader) String() string {
	m.mu.RLock()
	n := len(m.data)
	m.mu.RUnlock()
	return fmt.Sprintf("memory(%s, keys=%d)", m.name, n)
}
