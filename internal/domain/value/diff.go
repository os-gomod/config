package value

// ---------------------------------------------------------------------------
// DiffType
// ---------------------------------------------------------------------------

// DiffType classifies the kind of change between two values.
type DiffType int

const (
	DiffNone    DiffType = iota // No change
	DiffCreated                 // Key added
	DiffUpdated                 // Key value changed
	DiffDeleted                 // Key removed
)

func (d DiffType) String() string {
	switch d {
	case DiffNone:
		return "none"
	case DiffCreated:
		return "created"
	case DiffUpdated:
		return "updated"
	case DiffDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// DiffEvent
// ---------------------------------------------------------------------------

// DiffEvent represents a single detected change between old and new state.
type DiffEvent struct {
	Key    string   // The config key that changed.
	Type   DiffType // What kind of change.
	Old    Value    // The old value (zero Value if created).
	New    Value    // The new value (zero Value if deleted).
	OldSrc Source   // Source of the old value.
	NewSrc Source   // Source of the new value.
}

// ---------------------------------------------------------------------------
// DiffResult
// ---------------------------------------------------------------------------

// DiffResult holds the full set of differences between two maps of Values.
type DiffResult struct {
	Created []DiffEvent // Keys that appeared.
	Updated []DiffEvent // Keys whose values changed.
	Deleted []DiffEvent // Keys that were removed.
}

// HasChanges returns true if there is at least one difference.
func (r *DiffResult) HasChanges() bool {
	if r == nil {
		return false
	}
	return len(r.Created) > 0 || len(r.Updated) > 0 || len(r.Deleted) > 0
}

// Total returns the total number of detected changes.
func (r *DiffResult) Total() int {
	if r == nil {
		return 0
	}
	return len(r.Created) + len(r.Updated) + len(r.Deleted)
}

// AllEvents returns a flat slice of all diff events in Created, Updated, Deleted order.
func (r *DiffResult) AllEvents() []DiffEvent {
	if r == nil {
		return nil
	}
	out := make([]DiffEvent, 0, len(r.Created)+len(r.Updated)+len(r.Deleted))
	out = append(out, r.Created...)
	out = append(out, r.Updated...)
	out = append(out, r.Deleted...)
	return out
}

// ---------------------------------------------------------------------------
// ComputeDiff
// ---------------------------------------------------------------------------

// ComputeDiff computes the per-key diff events between old and new maps.
// It returns a flat list of DiffEvent (no grouping).
func ComputeDiff(old, new_ map[string]Value) []DiffEvent {
	var events []DiffEvent

	// Detect created and updated.
	for key, newVal := range new_ {
		oldVal, exists := old[key]
		if !exists {
			events = append(events, DiffEvent{
				Key:    key,
				Type:   DiffCreated,
				New:    newVal,
				NewSrc: newVal.Source(),
			})
		} else if !oldVal.Equal(newVal) {
			events = append(events, DiffEvent{
				Key:    key,
				Type:   DiffUpdated,
				Old:    oldVal,
				New:    newVal,
				OldSrc: oldVal.Source(),
				NewSrc: newVal.Source(),
			})
		}
	}

	// Detect deleted.
	for key, oldVal := range old {
		if _, exists := new_[key]; !exists {
			events = append(events, DiffEvent{
				Key:    key,
				Type:   DiffDeleted,
				Old:    oldVal,
				OldSrc: oldVal.Source(),
			})
		}
	}

	return events
}

// ---------------------------------------------------------------------------
// ComputeDiffResult
// ---------------------------------------------------------------------------

// ComputeDiffResult computes the full DiffResult between old and new maps,
// grouping events by type (created, updated, deleted).
func ComputeDiffResult(old, new_ map[string]Value) DiffResult {
	events := ComputeDiff(old, new_)
	result := DiffResult{
		Created: make([]DiffEvent, 0),
		Updated: make([]DiffEvent, 0),
		Deleted: make([]DiffEvent, 0),
	}

	for _, e := range events {
		switch e.Type {
		case DiffCreated:
			result.Created = append(result.Created, e)
		case DiffUpdated:
			result.Updated = append(result.Updated, e)
		case DiffDeleted:
			result.Deleted = append(result.Deleted, e)
		}
	}

	return result
}
