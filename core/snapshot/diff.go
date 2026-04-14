package snapshot

import (
	"strconv"
	"strings"

	"github.com/os-gomod/config/core/value"
)

// ChangeType classifies the kind of change between two snapshots.
type ChangeType int

const (
	// ChangeNone indicates no change.
	ChangeNone ChangeType = iota
	// ChangeAdded indicates a key was added.
	ChangeAdded
	// ChangeModified indicates a key's value changed.
	ChangeModified
	// ChangeDeleted indicates a key was removed.
	ChangeDeleted
)

var changeTypeNames = [...]string{
	ChangeNone:     "none",
	ChangeAdded:    "added",
	ChangeModified: "modified",
	ChangeDeleted:  "deleted",
}

// String returns the human-readable name of the ChangeType.
func (ct ChangeType) String() string {
	if int(ct) < len(changeTypeNames) {
		return changeTypeNames[ct]
	}
	return "unknown"
}

// Diff describes a single key-level change between two snapshots.
type Diff struct {
	Key      string
	Type     ChangeType
	OldValue value.Value
	NewValue value.Value
}

// DiffResult aggregates the set of changes between two snapshots.
type DiffResult struct {
	Diffs     []Diff
	Added     int
	Modified  int
	Deleted   int
	Unchanged int
}

// HasChanges reports whether the diff contains any changes.
func (r *DiffResult) HasChanges() bool { return len(r.Diffs) > 0 }

// Summary returns a human-readable summary of the diff.
func (r *DiffResult) Summary() string {
	var parts []string
	if r.Added > 0 {
		parts = append(parts, pluralize(r.Added, "addition", "additions"))
	}
	if r.Modified > 0 {
		parts = append(parts, pluralize(r.Modified, "modification", "modifications"))
	}
	if r.Deleted > 0 {
		parts = append(parts, pluralize(r.Deleted, "deletion", "deletions"))
	}
	if len(parts) == 0 {
		return "no changes"
	}
	return strings.Join(parts, ", ")
}

// Compare computes the diff between two snapshots.
func Compare(old, newSnap *Snapshot) *DiffResult {
	result := &DiffResult{Diffs: make([]Diff, 0)}
	var oldData, newData map[string]value.Value
	if old != nil {
		oldData = old.data
	}
	if newSnap != nil {
		newData = newSnap.data
	}
	diffMaps(result, oldData, newData)
	return result
}

// diffMaps populates the DiffResult by comparing two value maps.
func diffMaps(result *DiffResult, old, newData map[string]value.Value) {
	for _, k := range value.SortedKeys(old) {
		ov := old[k]
		nv, exists := newData[k]
		result.record(k, ov, nv, true, exists)
	}
	for _, k := range value.SortedKeys(newData) {
		if _, exists := old[k]; !exists {
			result.record(k, value.Value{}, newData[k], false, true)
		}
	}
}

// record classifies and appends a Diff entry.
func (r *DiffResult) record(key string, ov, nv value.Value, oldExists, newExists bool) {
	switch {
	case !oldExists && newExists:
		r.Diffs = append(r.Diffs, Diff{Key: key, Type: ChangeAdded, NewValue: nv})
		r.Added++
	case oldExists && !newExists:
		r.Diffs = append(r.Diffs, Diff{Key: key, Type: ChangeDeleted, OldValue: ov})
		r.Deleted++
	case oldExists && newExists:
		if !ov.Equal(nv) {
			r.Diffs = append(
				r.Diffs,
				Diff{Key: key, Type: ChangeModified, OldValue: ov, NewValue: nv},
			)
			r.Modified++
		} else {
			r.Unchanged++
		}
	}
}

// pluralize returns a count with the appropriate singular or plural noun.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(n) + " " + plural
}
