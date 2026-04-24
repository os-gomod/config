package value

// State is an immutable snapshot of the configuration at a specific version.
// It stores the full key-value map along with a monotonically increasing
// version number and a SHA-256 checksum for fast equality comparisons.
//
// State instances should be treated as read-only. Use [NewStateCopy] to
// create a new State that owns its data independently.
type State struct {
	data     map[string]Value
	version  uint64
	checksum string
}

// NewState creates a new [State] from the given data map and version. The
// checksum is computed automatically. The data map is stored directly
// (not copied); callers must ensure it is not mutated externally.
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

// NewStateCopy creates a new [State] from a deep copy of the given data map.
// This is the preferred constructor when the caller does not control the
// lifetime of the source map.
func NewStateCopy(data map[string]Value, ver uint64) *State {
	return NewState(Copy(data), ver)
}

// Get returns the [Value] for the given key and whether the key exists.
func (s *State) Get(key string) (Value, bool) {
	v, ok := s.data[key]
	return v, ok
}

// GetAll returns a defensive copy of all key-value pairs in the state.
func (s *State) GetAll() map[string]Value {
	return Copy(s.data)
}

// GetAllUnsafe returns the raw underlying map without copying. Callers must
// not mutate the returned map.
func (s *State) GetAllUnsafe() map[string]Value {
	return s.data
}

// Has reports whether the given key exists in the state.
func (s *State) Has(key string) bool { _, ok := s.data[key]; return ok }

// Data returns a defensive copy of the underlying data map.
func (s *State) Data() map[string]Value { return Copy(s.data) }

// DataUnsafe returns the raw underlying map without copying. Callers must
// not mutate the returned map.
func (s *State) DataUnsafe() map[string]Value { return s.data }

// Version returns the monotonically increasing version number of this state.
func (s *State) Version() uint64 { return s.version }

// Len returns the number of keys in the state.
func (s *State) Len() int { return len(s.data) }

// Checksum returns the SHA-256 checksum of the state's data, computed at
// construction time. It can be used for fast equality comparisons without
// inspecting individual values.
func (s *State) Checksum() string { return s.checksum }

// Keys returns all keys in the state, sorted lexicographically.
func (s *State) Keys() []string {
	return SortedKeys(s.data)
}

// Equal reports whether two states have the same checksum, which is a fast
// proxy for structural equality. Nil states are handled gracefully.
func (s *State) Equal(other *State) bool {
	if s == nil && other == nil {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	return s.checksum == other.checksum
}

// DiffEvents computes the [DiffEvent] entries between this state's data and
// the given new data map. This is useful for determining what changed during
// a reload or mutation.
func (s *State) DiffEvents(newData map[string]Value) []DiffEvent {
	return ComputeDiff(s.data, newData)
}

// RedactedCopy returns a new State with all secret values replaced by
// [REDACTED]. The version and checksum are recomputed for the redacted
// data. The original State is not modified.
//
// This is used by Config.Snapshot() and logging to prevent accidental
// secret leakage in diagnostic output.
func (s *State) RedactedCopy() *State {
	redacted := RedactMap(s.data)
	return NewState(redacted, s.version)
}
