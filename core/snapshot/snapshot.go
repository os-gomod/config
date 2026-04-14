// Package snapshot provides point-in-time snapshots of config state with
// diffing, branching, and versioning support.
package snapshot

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/os-gomod/config/core/value"
)

// Snapshot is an immutable point-in-time capture of config state.
type Snapshot struct {
	id        uint64
	version   uint64
	data      map[string]value.Value
	timestamp time.Time
	label     string
	parent    uint64
	checksum  string
	metadata  map[string]any
}

// New creates a Snapshot with the given id, version, and data.
// The data map is deep-copied; the caller's map is not retained.
func New(id, version uint64, data map[string]value.Value, opts ...Option) *Snapshot {
	s := &Snapshot{
		id:        id,
		version:   version,
		data:      value.Copy(data),
		timestamp: time.Now(),
		metadata:  make(map[string]any),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.checksum = value.ComputeChecksum(s.data)
	return s
}

// ID returns the snapshot's unique identifier.
func (s *Snapshot) ID() uint64 { return s.id }

// Version returns the config version this snapshot was taken at.
func (s *Snapshot) Version() uint64 { return s.version }

// Timestamp returns the time at which the snapshot was created.
func (s *Snapshot) Timestamp() time.Time { return s.timestamp }

// Label returns the optional human-readable label.
func (s *Snapshot) Label() string { return s.label }

// Parent returns the ID of the parent snapshot, or 0 if this is a root.
func (s *Snapshot) Parent() uint64 { return s.parent }

// Checksum returns the SHA-256 hex digest of the snapshot data.
func (s *Snapshot) Checksum() string { return s.checksum }

// Len returns the number of keys in the snapshot.
func (s *Snapshot) Len() int { return len(s.data) }

// Data returns a safe copy of the snapshot data.
func (s *Snapshot) Data() map[string]value.Value { return value.Copy(s.data) }

// Metadata returns a copy of the snapshot metadata map.
func (s *Snapshot) Metadata() map[string]any { return copyMap(s.metadata) }

// Get retrieves a Value by key from the snapshot.
func (s *Snapshot) Get(key string) (value.Value, bool) {
	v, ok := s.data[key]
	return v, ok
}

// Keys returns all keys in the snapshot in sorted order.
func (s *Snapshot) Keys() []string {
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// MarshalJSON serializes the snapshot to JSON, converting Value entries to raw form.
func (s *Snapshot) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        uint64         `json:"id"`
		Version   uint64         `json:"version"`
		Data      map[string]any `json:"data"`
		Timestamp time.Time      `json:"timestamp"`
		Label     string         `json:"label,omitempty"`
		Parent    uint64         `json:"parent,omitempty"`
		Checksum  string         `json:"checksum"`
		Metadata  map[string]any `json:"metadata,omitempty"`
	}{
		ID:        s.id,
		Version:   s.version,
		Data:      valuesToRaw(s.data),
		Timestamp: s.timestamp,
		Label:     s.label,
		Parent:    s.parent,
		Checksum:  s.checksum,
		Metadata:  s.metadata,
	})
}

// valuesToRaw extracts the raw values from a Value map for JSON serialization.
func valuesToRaw(m map[string]value.Value) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v.Raw()
	}
	return result
}

// copyMap creates a shallow copy of a metadata map.
func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
