package value

// State is an immutable snapshot of the config key-value store at a given version.
// It is safe for concurrent read access after creation.
type State struct {
	data     map[string]Value
	version  uint64
	checksum string
}

// NewState creates a State from the given data map and version number.
// The checksum is computed from the data; nil data is treated as empty.
func NewState(data map[string]Value, ver uint64) *State {
	s := &State{
		data:    data,
		version: ver,
	}
	if data != nil {
		s.checksum = ComputeChecksum(data)
	} else {
		s.checksum = ComputeChecksum(nil)
	}
	return s
}

// NewStateCopy creates a State with a deep copy of the data map.
func NewStateCopy(data map[string]Value, ver uint64) *State {
	return NewState(Copy(data), ver)
}

// Get retrieves a Value by key. The second return indicates whether the key existed.
func (s *State) Get(key string) (Value, bool) {
	v, ok := s.data[key]
	return v, ok
}

// GetAll returns a safe copy of the data map. Mutations to the returned map
// do not affect the State.
func (s *State) GetAll() map[string]Value {
	return Copy(s.data)
}

// GetAllUnsafe returns the internal data map directly without copying.
// Callers must not modify the returned map; doing so corrupts the State.
// Use GetAll for a safe copy.
func (s *State) GetAllUnsafe() map[string]Value {
	return s.data
}

// Has reports whether the key exists in the state.
func (s *State) Has(key string) bool { _, ok := s.data[key]; return ok }

// Data returns a safe copy of the data map.
func (s *State) Data() map[string]Value { return Copy(s.data) }

// DataUnsafe returns the internal data map without copying.
func (s *State) DataUnsafe() map[string]Value { return s.data }

// Version returns the version number of this state snapshot.
func (s *State) Version() uint64 { return s.version }

// Len returns the number of keys in the state.
func (s *State) Len() int { return len(s.data) }

// Checksum returns the SHA-256 hex digest of the state data.
func (s *State) Checksum() string { return s.checksum }

// Keys returns all keys in the state in sorted order.
func (s *State) Keys() []string {
	return SortedKeys(s.data)
}

// Equal reports whether two States have the same checksum. This is a fast
// equality check that relies on checksum collision resistance.
func (s *State) Equal(other *State) bool {
	if s == nil && other == nil {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	return s.checksum == other.checksum
}

// DiffEvents computes the diff events between the current state data and newData.
func (s *State) DiffEvents(newData map[string]Value) []DiffEvent {
	return ComputeDiff(s.data, newData)
}
