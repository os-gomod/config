package snapshot

import "time"

// Branch represents a named sequence of snapshots sharing a common base.
type Branch struct {
	name      string
	baseID    uint64
	snapshots []*Snapshot
	created   time.Time
}

// Name returns the branch name.
func (b *Branch) Name() string { return b.name }

// BaseID returns the snapshot ID that serves as the branch base.
func (b *Branch) BaseID() uint64 { return b.baseID }

// Created returns the time the branch was created.
func (b *Branch) Created() time.Time { return b.created }

// Len returns the number of snapshots in the branch.
func (b *Branch) Len() int { return len(b.snapshots) }

// Current returns the most recent snapshot in the branch, or nil if empty.
func (b *Branch) Current() *Snapshot {
	if len(b.snapshots) == 0 {
		return nil
	}
	return b.snapshots[len(b.snapshots)-1]
}

// History returns a copy of the snapshot list.
func (b *Branch) History() []*Snapshot {
	out := make([]*Snapshot, len(b.snapshots))
	copy(out, b.snapshots)
	return out
}

// Append adds a snapshot to the branch.
func (b *Branch) Append(s *Snapshot) {
	b.snapshots = append(b.snapshots, s)
}
