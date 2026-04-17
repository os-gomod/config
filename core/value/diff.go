package value

// DiffType identifies the kind of change between two configuration states.
type DiffType uint8

const (
	// DiffNone indicates no change.
	DiffNone DiffType = iota
	// DiffCreated indicates a key was added.
	DiffCreated
	// DiffUpdated indicates a key's value changed.
	DiffUpdated
	// DiffDeleted indicates a key was removed.
	DiffDeleted
)

// String returns the human-readable name of the DiffType.
func (d DiffType) String() string {
	switch d {
	case DiffCreated:
		return "created"
	case DiffUpdated:
		return "updated"
	case DiffDeleted:
		return "deleted"
	default:
		return "none"
	}
}

// DiffEvent represents a single configuration change between two states. It
// captures the type of change, the affected key, and the old and new values.
type DiffEvent struct {
	// Type is the kind of change (created, updated, or deleted).
	Type DiffType
	// Key is the configuration key that changed.
	Key string
	// OldValue is the value before the change. Empty for DiffCreated events.
	OldValue Value
	// NewValue is the value after the change. Empty for DiffDeleted events.
	NewValue Value
}

// DiffResult holds the complete diff between two configuration states,
// including individual events and summary counts.
type DiffResult struct {
	// Events is the list of individual change events.
	Events []DiffEvent
	// Created is the number of keys that were added.
	Created int
	// Updated is the number of keys whose values changed.
	Updated int
	// Deleted is the number of keys that were removed.
	Deleted int
	// Unchanged is the number of keys that remained the same.
	Unchanged int
}

// HasChanges reports whether the diff contains any change events.
func (r *DiffResult) HasChanges() bool { return len(r.Events) > 0 }

// ComputeDiff computes the set of [DiffEvent] entries between two value maps.
// Keys present only in oldMap produce DiffDeleted events, keys present only
// in newMap produce DiffCreated events, and keys in both with unequal values
// produce DiffUpdated events. Results are sorted by key.
func ComputeDiff(oldMap, newMap map[string]Value) []DiffEvent {
	events := make([]DiffEvent, 0)
	for _, k := range SortedKeys(oldMap) {
		ov := oldMap[k]
		nv, exists := newMap[k]
		if !exists {
			events = append(events, DiffEvent{Type: DiffDeleted, Key: k, OldValue: ov})
		} else if !ov.Equal(nv) {
			events = append(events, DiffEvent{Type: DiffUpdated, Key: k, OldValue: ov, NewValue: nv})
		}
	}
	for _, k := range SortedKeys(newMap) {
		if _, exists := oldMap[k]; !exists {
			events = append(events, DiffEvent{Type: DiffCreated, Key: k, NewValue: newMap[k]})
		}
	}
	return events
}

// ComputeDiffResult computes the diff between two value maps and returns a
// [DiffResult] with both the individual events and summary counts (created,
// updated, deleted, unchanged).
func ComputeDiffResult(oldMap, newMap map[string]Value) *DiffResult {
	events := ComputeDiff(oldMap, newMap)
	r := &DiffResult{Events: events}
	for _, e := range events {
		switch e.Type {
		case DiffCreated:
			r.Created++
		case DiffUpdated:
			r.Updated++
		case DiffDeleted:
			r.Deleted++
		}
	}
	if oldMap != nil && newMap != nil {
		for _, k := range SortedKeys(oldMap) {
			if nv, ok := newMap[k]; ok {
				if oldMap[k].Equal(nv) {
					r.Unchanged++
				}
			}
		}
	}
	return r
}
