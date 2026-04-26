// Package layer provides domain types for configuration layers — the fundamental
// unit of config composition. Each Layer wraps a data source (file, env, vault, etc.)
// with its own priority, health tracking, and error handling.
package layer

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// Loadable
// ---------------------------------------------------------------------------

// Loadable is the interface that all config data sources must implement.
// The Load method reads configuration data from the source and returns it
// as a map of Values.
type Loadable interface {
	// Load reads the configuration from the source and returns the data.
	// Returns an error if the source is unavailable or data is malformed.
	Load() (map[string]value.Value, error)
	// Name returns a human-readable name for this source.
	Name() string
}

// ---------------------------------------------------------------------------
// HealthStatus
// ---------------------------------------------------------------------------

// HealthStatus represents the health of a configuration layer.
type HealthStatus struct {
	Healthy              bool      `json:"healthy"`               // Whether the layer is operational.
	LastError            string    `json:"last_error"`            // Last error message (empty if healthy).
	LastCheck            time.Time `json:"last_check"`            // When the last health check occurred.
	ConsecutiveSuccesses int       `json:"consecutive_successes"` // Consecutive successful loads.
	ConsecutiveFailures  int       `json:"consecutive_failures"`  // Consecutive failed loads.
}

func (h HealthStatus) String() string {
	status := "healthy"
	if !h.Healthy {
		status = "unhealthy"
	}
	var b strings.Builder
	b.WriteString(status)
	if h.LastError != "" {
		b.WriteString(" err=")
		b.WriteString(h.LastError)
	}
	if !h.LastCheck.IsZero() {
		b.WriteString(" last_check=")
		b.WriteString(h.LastCheck.Format(time.RFC3339))
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Layer
// ---------------------------------------------------------------------------

// Layer represents a single configuration source layer in the merge hierarchy.
// Layers are composed in priority order to form the final configuration state.
type Layer struct {
	name      string                 // Human-readable name.
	source    Loadable               // The data source (may be nil for static layers).
	data      map[string]value.Value // Cached data from last successful load.
	priority  int                    // Merge priority (higher = overrides lower).
	enabled   bool                   // Whether this layer is active.
	readonly  bool                   // If true, the layer's data cannot be modified.
	mu        sync.RWMutex           // Protects data and health.
	health    HealthStatus           // Current health status.
	loadCount int64                  // Number of times this layer has been loaded.
}

// LayerOption configures a Layer during construction.
type LayerOption func(*Layer)

// WithPriority sets the merge priority of the layer.
func WithPriority(p int) LayerOption {
	return func(l *Layer) {
		l.priority = p
	}
}

// WithEnabled sets whether the layer is initially enabled.
func WithEnabled(enabled bool) LayerOption {
	return func(l *Layer) {
		l.enabled = enabled
	}
}

// WithReadonly sets whether the layer is read-only.
func WithReadonly(readonly bool) LayerOption {
	return func(l *Layer) {
		l.readonly = readonly
	}
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// NewLayer creates a new Layer with the given name and data source.
func NewLayer(name string, source Loadable, opts ...LayerOption) *Layer {
	l := &Layer{
		name:     name,
		source:   source,
		data:     make(map[string]value.Value),
		priority: 0,
		enabled:  true,
		readonly: false,
		health: HealthStatus{
			Healthy:   true,
			LastCheck: time.Now().UTC(),
		},
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// NewStaticLayer creates a Layer with pre-loaded static data (no source).
func NewStaticLayer(name string, data map[string]value.Value, opts ...LayerOption) *Layer {
	l := NewLayer(name, nil, opts...)
	l.data = value.Copy(data)
	return l
}

// NewInMemoryLayer creates a layer for programmatic config with given priority.
func NewInMemoryLayer(name string, priority int, opts ...LayerOption) *Layer {
	return NewLayer(name, nil, append(opts, WithPriority(priority))...)
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

// Load loads or reloads the layer's data from its source.
// If the layer is disabled, has no source, or is read-only with existing data,
// it returns the current data without loading.
func (l *Layer) Load() (map[string]value.Value, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.loadCount++

	if !l.enabled {
		return value.Copy(l.data), nil
	}

	if l.source == nil {
		l.recordSuccess()
		return value.Copy(l.data), nil
	}

	data, err := l.source.Load()
	if err != nil {
		l.recordFailure(err)
		return nil, l.wrapLoadError(err)
	}

	l.data = data
	l.recordSuccess()
	return value.Copy(l.data), nil
}

// ---------------------------------------------------------------------------
// Read accessors
// ---------------------------------------------------------------------------

// Name returns the layer name.
func (l *Layer) Name() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.name
}

// Priority returns the merge priority.
func (l *Layer) Priority() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.priority
}

// Enabled returns whether the layer is active.
func (l *Layer) Enabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// Readonly returns whether the layer is read-only.
func (l *Layer) Readonly() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.readonly
}

// Data returns a copy of the layer's current data.
func (l *Layer) Data() map[string]value.Value {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return value.Copy(l.data)
}

// DataUnsafe returns the layer's data without copying (for read-only iteration).
func (l *Layer) DataUnsafe() map[string]value.Value {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.data
}

// Health returns the current health status.
func (l *Layer) Health() HealthStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.health
}

// LoadCount returns the number of times this layer has been loaded.
func (l *Layer) LoadCount() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.loadCount
}

// Len returns the number of keys in the layer.
func (l *Layer) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.data)
}

// Has returns true if the key exists in the layer's data.
func (l *Layer) Has(key string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.data[key]
	return ok
}

// Get returns the value for the given key, or a zero Value if not found.
func (l *Layer) Get(key string) value.Value {
	l.mu.RLock()
	defer l.mu.RUnlock()
	v, ok := l.data[key]
	if !ok {
		return value.Value{}
	}
	return v
}

// Keys returns the sorted keys of the layer's data.
func (l *Layer) Keys() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return value.SortedKeys(l.data)
}

// ---------------------------------------------------------------------------
// Write accessors
// ---------------------------------------------------------------------------

// Set adds or updates a key in the layer's data. Returns an error if the layer
// is read-only.
func (l *Layer) Set(key string, val value.Value) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.readonly {
		return fmt.Errorf("layer %q is read-only", l.name)
	}
	l.data[key] = val
	return nil
}

// SetMany adds or updates multiple keys in the layer's data.
func (l *Layer) SetMany(data map[string]value.Value) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.readonly {
		return fmt.Errorf("layer %q is read-only", l.name)
	}
	for k, v := range data {
		l.data[k] = v
	}
	return nil
}

// Delete removes a key from the layer's data. Returns an error if read-only.
func (l *Layer) Delete(key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.readonly {
		return fmt.Errorf("layer %q is read-only", l.name)
	}
	delete(l.data, key)
	return nil
}

// ---------------------------------------------------------------------------
// State modifiers
// ---------------------------------------------------------------------------

// SetEnabled enables or disables the layer.
func (l *Layer) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// SetPriority changes the layer's merge priority.
func (l *Layer) SetPriority(p int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.priority = p
}

// SetReadonly changes the layer's read-only flag.
func (l *Layer) SetReadonly(readonly bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.readonly = readonly
}

// ReplaceData replaces the layer's data entirely with the given map.
func (l *Layer) ReplaceData(data map[string]value.Value) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data = value.Copy(data)
}

// Clear removes all data from the layer.
func (l *Layer) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data = make(map[string]value.Value)
}

// ---------------------------------------------------------------------------
// Health tracking
// ---------------------------------------------------------------------------

func (l *Layer) recordSuccess() {
	l.health.Healthy = true
	l.health.LastError = ""
	l.health.LastCheck = time.Now().UTC()
	l.health.ConsecutiveSuccesses++
	l.health.ConsecutiveFailures = 0
}

func (l *Layer) recordFailure(err error) {
	l.health.Healthy = false
	l.health.LastError = err.Error()
	l.health.LastCheck = time.Now().UTC()
	l.health.ConsecutiveFailures++
	l.health.ConsecutiveSuccesses = 0
}

func (l *Layer) wrapLoadError(err error) error {
	return fmt.Errorf("layer %q load failed: %w", l.name, err)
}

// ---------------------------------------------------------------------------
// LayerError
// ---------------------------------------------------------------------------

// LayerError represents an error that occurred during layer operations.
type LayerError struct {
	LayerName string // Name of the layer.
	Operation string // Operation that failed (load, set, delete, etc.).
	Err       error  // Underlying error.
}

// Error implements error.
func (e *LayerError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("layer %q: %s: %s", e.LayerName, e.Operation, e.Err.Error())
	}
	return fmt.Sprintf("layer %q: %s", e.LayerName, e.Err.Error())
}

// Unwrap implements errors.Unwrap.
func (e *LayerError) Unwrap() error {
	return e.Err
}

// Equal returns true if two layers have identical data.
func (l *Layer) Equal(other *Layer) bool {
	if l == nil && other == nil {
		return true
	}
	if l == nil || other == nil {
		return false
	}

	l.mu.RLock()
	other.mu.RLock()
	defer l.mu.RUnlock()
	defer other.mu.RUnlock()

	if l.name != other.name || l.priority != other.priority || l.enabled != other.enabled {
		return false
	}
	if len(l.data) != len(other.data) {
		return false
	}
	for k, v := range l.data {
		ov, ok := other.data[k]
		if !ok || !v.Equal(ov) {
			return false
		}
	}
	return true
}

// String returns a human-readable summary.
func (l *Layer) String() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	status := "enabled"
	if !l.enabled {
		status = "disabled"
	}
	return fmt.Sprintf("Layer{name=%q, priority=%d, keys=%d, %s, src=%v}",
		l.name, l.priority, len(l.data), status, l.source != nil)
}
