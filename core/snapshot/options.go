package snapshot

import "time"

// Option configures a Snapshot during creation.
type Option func(*Snapshot)

// WithLabel sets the human-readable label on a Snapshot.
func WithLabel(label string) Option {
	return func(s *Snapshot) { s.label = label }
}

// WithParent sets the parent snapshot ID on a Snapshot.
func WithParent(parentID uint64) Option {
	return func(s *Snapshot) { s.parent = parentID }
}

// WithTimestamp overrides the Snapshot creation timestamp.
func WithTimestamp(t time.Time) Option {
	return func(s *Snapshot) { s.timestamp = t }
}

// WithMetadata adds a key-value pair to the Snapshot metadata.
func WithMetadata(key string, val any) Option {
	return func(s *Snapshot) {
		if s.metadata == nil {
			s.metadata = make(map[string]any)
		}
		s.metadata[key] = val
	}
}
