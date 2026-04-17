package snapshot

import "time"

// Branch represents a named configuration branch that tracks a sequence of
// snapshots forked from a base snapshot. Branches allow experimenting with
// configuration changes without affecting the main history.
type Branch struct {
	name      string
	baseID    uint64
	snapshots []*Snapshot
	created   time.Time
}

// Name returns the branch name.
func (b *Branch) Name() string { return b.name }

// BaseID returns the ID of the snapshot this branch was created from.
func (b *Branch) BaseID() uint64 { return b.baseID }

// Created returns the time when the branch was created.
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

// History returns all snapshots in the branch in chronological order.
func (b *Branch) History() []*Snapshot {
	out := make([]*Snapshot, len(b.snapshots))
	copy(out, b.snapshots)
	return out
}

// Append adds a snapshot to the end of the branch history.
func (b *Branch) Append(s *Snapshot) {
	b.snapshots = append(b.snapshots, s)
}
