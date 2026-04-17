package snapshot

import "time"

// Option is a functional option for configuring a Snapshot during creation.
type Option func(*Snapshot)

// WithLabel sets the snapshot label.
func WithLabel(label string) Option {
	return func(s *Snapshot) { s.label = label }
}

// WithParent sets the parent snapshot ID.
func WithParent(parentID uint64) Option {
	return func(s *Snapshot) { s.parent = parentID }
}

// WithTimestamp sets the snapshot timestamp.
func WithTimestamp(t time.Time) Option {
	return func(s *Snapshot) { s.timestamp = t }
}

// WithMetadata adds a key-value pair to the snapshot's metadata.
func WithMetadata(key string, val any) Option {
	return func(s *Snapshot) {
		if s.metadata == nil {
			s.metadata = make(map[string]any)
		}
		s.metadata[key] = val
	}
}
