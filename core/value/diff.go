package value

// DiffType classifies the kind of change between two value maps.
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

// DiffEvent describes a single change between two value maps.
type DiffEvent struct {
	Type     DiffType
	Key      string
	OldValue Value
	NewValue Value
}

// DiffResult aggregates the set of changes between two value maps.
type DiffResult struct {
	Events    []DiffEvent
	Created   int
	Updated   int
	Deleted   int
	Unchanged int
}

// HasChanges reports whether the diff contains any events.
func (r *DiffResult) HasChanges() bool { return len(r.Events) > 0 }

// ComputeDiff computes the diff events between oldMap and newMap.
// Keys present only in oldMap produce DiffDeleted events; keys present only
// in newMap produce DiffCreated events; keys present in both with different
// values produce DiffUpdated events.
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

// ComputeDiffResult computes the diff and returns a structured DiffResult with counts.
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
