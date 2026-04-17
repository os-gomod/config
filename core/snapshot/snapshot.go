// Package snapshot provides configuration state versioning and history management.
// It captures point-in-time snapshots of configuration data, computes diffs between
// versions, tracks version metadata, and supports branching for experimental configurations.
package snapshot

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/os-gomod/config/core/value"
)

// Snapshot represents a point-in-time capture of configuration state.
// It stores the configuration data, version number, timestamp, optional label,
// parent snapshot reference, content checksum, and arbitrary metadata.
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

// New creates a new Snapshot with the given ID, version, and data.
// The timestamp is automatically set to the current time, and a checksum
// is computed over the data. Options can be used to set label, parent, etc.
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

// ID returns the unique snapshot identifier.
func (s *Snapshot) ID() uint64 { return s.id }

// Version returns the configuration version number.
func (s *Snapshot) Version() uint64 { return s.version }

// Timestamp returns when the snapshot was taken.
func (s *Snapshot) Timestamp() time.Time { return s.timestamp }

// Label returns the optional snapshot label.
func (s *Snapshot) Label() string { return s.label }

// Parent returns the parent snapshot ID, or 0 if this is a root snapshot.
func (s *Snapshot) Parent() uint64 { return s.parent }

// Checksum returns the content checksum of the snapshot data.
func (s *Snapshot) Checksum() string { return s.checksum }

// Len returns the number of configuration keys in the snapshot.
func (s *Snapshot) Len() int { return len(s.data) }

// Data returns a copy of the snapshot's configuration data.
func (s *Snapshot) Data() map[string]value.Value { return value.Copy(s.data) }

// Metadata returns a copy of the snapshot's metadata.
func (s *Snapshot) Metadata() map[string]any { return copyMap(s.metadata) }

// Get retrieves a single value by key. Returns the value and whether it exists.
func (s *Snapshot) Get(key string) (value.Value, bool) {
	v, ok := s.data[key]
	return v, ok
}

// Keys returns all configuration keys in sorted order.
func (s *Snapshot) Keys() []string {
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// MarshalJSON returns a JSON representation of the snapshot including
// ID, version, data, timestamp, label, parent, checksum, and metadata.
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

// valuesToRaw converts a map of value.Value to a map of raw values.
func valuesToRaw(m map[string]value.Value) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v.Raw()
	}
	return result
}

// copyMap creates a shallow copy of a string-to-any map.
func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
