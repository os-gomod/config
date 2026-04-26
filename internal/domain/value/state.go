package value

import "sync"

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

// State represents an immutable snapshot of the configuration data at a point in time.
// It is safe for concurrent reads. For concurrent writes, use external synchronization
// or create new State copies.
type State struct {
	data     map[string]Value
	version  int64
	checksum string
	mu       sync.RWMutex // protects mutation access if needed
}

// NewState creates a new State from the given data map.
// The data map is copied; the caller's map is not retained.
func NewState(data map[string]Value) *State {
	copied := Copy(data)
	return &State{
		data:     copied,
		version:  0,
		checksum: ComputeMapChecksum(copied),
	}
}

// NewStateCopy creates a deep copy of an existing State with an incremented version.
func NewStateCopy(s *State) *State {
	if s == nil {
		return NewState(nil)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	copied := Copy(s.data)
	return &State{
		data:     copied,
		version:  s.version + 1,
		checksum: ComputeMapChecksum(copied),
	}
}

// NewStateWithVersion creates a State with a specific version number.
func NewStateWithVersion(data map[string]Value, version int64) *State {
	copied := Copy(data)
	return &State{
		data:     copied,
		version:  version,
		checksum: ComputeMapChecksum(copied),
	}
}

// ---------------------------------------------------------------------------
// Read accessors (all concurrency-safe)
// ---------------------------------------------------------------------------

// Get returns the Value for the given key. The returned Value may be a zero
// Value (IsZero() == true) if the key does not exist.
func (s *State) Get(key string) Value {
	if s == nil {
		return Value{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.data == nil {
		return Value{}
	}
	v, ok := s.data[key]
	if !ok {
		return Value{}
	}
	return v
}

// GetAll returns a copy of all key-value pairs.
func (s *State) GetAll() map[string]Value {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Copy(s.data)
}

// GetAllUnsafe returns the underlying data map directly. The caller MUST NOT
// modify this map. Use for read-only iteration performance.
func (s *State) GetAllUnsafe() map[string]Value {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Has returns true if the key exists in the state.
func (s *State) Has(key string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data[key]
	return ok
}

// Data returns a copy of the underlying data map (alias for GetAll).
func (s *State) Data() map[string]Value {
	return s.GetAll()
}

// DataUnsafe returns the underlying data map without copying (alias for GetAllUnsafe).
func (s *State) DataUnsafe() map[string]Value {
	return s.GetAllUnsafe()
}

// Version returns the state version counter.
func (s *State) Version() int64 {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// Len returns the number of keys in the state.
func (s *State) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Checksum returns the SHA-256 checksum of the state data.
func (s *State) Checksum() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.checksum
}

// Keys returns a sorted list of all keys in the state.
func (s *State) Keys() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SortedKeys(s.data)
}

// ---------------------------------------------------------------------------
// Comparison
// ---------------------------------------------------------------------------

// Equal returns true if two States have identical data, checksum, and version.
func (s *State) Equal(other *State) bool {
	if s == nil && other == nil {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	s.mu.RLock()
	other.mu.RLock()
	defer s.mu.RUnlock()
	defer other.mu.RUnlock()

	if s.version != other.version {
		return false
	}
	if s.checksum != other.checksum && s.checksum != "" && other.checksum != "" {
		return false
	}
	if len(s.data) != len(other.data) {
		return false
	}
	for k, sv := range s.data {
		ov, ok := other.data[k]
		if !ok || !sv.Equal(ov) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Diff
// ---------------------------------------------------------------------------

// DiffEvents computes the diff events between this state and another.
// Returns nil if both states are nil.
func (s *State) DiffEvents(other *State) []DiffEvent {
	if s == nil && other == nil {
		return nil
	}
	var oldData, newData map[string]Value
	if s != nil {
		oldData = s.GetAll()
	}
	if other != nil {
		newData = other.GetAll()
	}
	return ComputeDiff(oldData, newData)
}

// ---------------------------------------------------------------------------
// Redaction
// ---------------------------------------------------------------------------

// RedactedCopy returns a copy of the state with all secret values redacted.
func (s *State) RedactedCopy() *State {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	redacted := make(map[string]Value, len(s.data))
	for k, v := range s.data {
		redacted[k] = v.Redact(k)
	}
	return &State{
		data:     redacted,
		version:  s.version,
		checksum: ComputeMapChecksum(redacted),
	}
}

// ---------------------------------------------------------------------------
// Mutation (creates new State)
// ---------------------------------------------------------------------------

// Set returns a new State with the key set to the given Value.
func (s *State) Set(key string, val Value) *State {
	if s == nil {
		return NewState(map[string]Value{key: val})
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	newData := Copy(s.data)
	if newData == nil {
		newData = make(map[string]Value)
	}
	newData[key] = val
	return &State{
		data:     newData,
		version:  s.version + 1,
		checksum: ComputeMapChecksum(newData),
	}
}

// Delete returns a new State with the key removed.
func (s *State) Delete(key string) *State {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	newData := Copy(s.data)
	delete(newData, key)
	return &State{
		data:     newData,
		version:  s.version + 1,
		checksum: ComputeMapChecksum(newData),
	}
}

// Merge returns a new State with the given map merged on top.
// Existing keys are overwritten if present in the overlay.
func (s *State) Merge(overlay map[string]Value) *State {
	if s == nil {
		return NewState(overlay)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	newData := Copy(s.data)
	for k, v := range overlay {
		newData[k] = v
	}
	return &State{
		data:     newData,
		version:  s.version + 1,
		checksum: ComputeMapChecksum(newData),
	}
}
