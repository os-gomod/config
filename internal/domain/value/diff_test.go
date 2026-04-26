package value

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DiffType
// ---------------------------------------------------------------------------

func TestDiffType_String(t *testing.T) {
	tests := []struct {
		dt   DiffType
		want string
	}{
		{DiffNone, "none"},
		{DiffCreated, "created"},
		{DiffUpdated, "updated"},
		{DiffDeleted, "deleted"},
		{DiffType(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.dt.String())
		})
	}
}

// ---------------------------------------------------------------------------
// ComputeDiff
// ---------------------------------------------------------------------------

func TestComputeDiff_NoChanges(t *testing.T) {
	old := map[string]Value{
		"a": New(1),
		"b": New(2),
	}
	new_ := map[string]Value{
		"a": New(1),
		"b": New(2),
	}

	events := ComputeDiff(old, new_)
	assert.Empty(t, events)
}

func TestComputeDiff_AdditionsOnly(t *testing.T) {
	old := map[string]Value{
		"a": New(1),
	}
	new_ := map[string]Value{
		"a": New(1),
		"b": New(2),
		"c": New(3),
	}

	events := ComputeDiff(old, new_)
	// Should have 2 created events for "b" and "c"
	created := filterEvents(events, DiffCreated)
	assert.Len(t, created, 2)
	assert.Empty(t, filterEvents(events, DiffUpdated))
	assert.Empty(t, filterEvents(events, DiffDeleted))

	keys := eventKeys(created)
	assert.Contains(t, keys, "b")
	assert.Contains(t, keys, "c")
}

func TestComputeDiff_DeletionsOnly(t *testing.T) {
	old := map[string]Value{
		"a": New(1),
		"b": New(2),
	}
	new_ := map[string]Value{
		"a": New(1),
	}

	events := ComputeDiff(old, new_)
	deleted := filterEvents(events, DiffDeleted)
	assert.Len(t, deleted, 1)
	assert.Equal(t, "b", deleted[0].Key)
	assert.Empty(t, filterEvents(events, DiffCreated))
	assert.Empty(t, filterEvents(events, DiffUpdated))
}

func TestComputeDiff_UpdatesOnly(t *testing.T) {
	old := map[string]Value{
		"a": New(1),
		"b": New(2),
	}
	new_ := map[string]Value{
		"a": New(10),
		"b": New(2),
	}

	events := ComputeDiff(old, new_)
	updated := filterEvents(events, DiffUpdated)
	assert.Len(t, updated, 1)
	assert.Equal(t, "a", updated[0].Key)
	assert.Equal(t, 1, updated[0].Old.Int())
	assert.Equal(t, 10, updated[0].New.Int())
	assert.Empty(t, filterEvents(events, DiffCreated))
	assert.Empty(t, filterEvents(events, DiffDeleted))
}

func TestComputeDiff_MixedChanges(t *testing.T) {
	old := map[string]Value{
		"a": New(1),
		"b": New(2),
		"c": New(3),
	}
	new_ := map[string]Value{
		"a": New(10), // updated
		"c": New(3),  // unchanged
		// b deleted
		"d": New(4), // created
	}

	events := ComputeDiff(old, new_)

	created := filterEvents(events, DiffCreated)
	updated := filterEvents(events, DiffUpdated)
	deleted := filterEvents(events, DiffDeleted)

	assert.Len(t, created, 1)
	assert.Equal(t, "d", created[0].Key)

	assert.Len(t, updated, 1)
	assert.Equal(t, "a", updated[0].Key)

	assert.Len(t, deleted, 1)
	assert.Equal(t, "b", deleted[0].Key)
}

func TestComputeDiff_NilMaps(t *testing.T) {
	t.Run("nil_old", func(t *testing.T) {
		events := ComputeDiff(nil, map[string]Value{"a": New(1)})
		created := filterEvents(events, DiffCreated)
		assert.Len(t, created, 1)
	})

	t.Run("nil_new", func(t *testing.T) {
		events := ComputeDiff(map[string]Value{"a": New(1)}, nil)
		deleted := filterEvents(events, DiffDeleted)
		assert.Len(t, deleted, 1)
	})

	t.Run("both_nil", func(t *testing.T) {
		events := ComputeDiff(nil, nil)
		assert.Empty(t, events)
	})
}

func TestComputeDiff_SameValueDifferentPriority(t *testing.T) {
	old := map[string]Value{
		"a": FromRaw("x", TypeString, SourceFile, 0),
	}
	new_ := map[string]Value{
		"a": FromRaw("x", TypeString, SourceMemory, 10),
	}

	events := ComputeDiff(old, new_)
	// Different priority means values are not equal, so this should show as updated
	updated := filterEvents(events, DiffUpdated)
	assert.Len(t, updated, 1)
}

func TestComputeDiff_SourceTracking(t *testing.T) {
	old := map[string]Value{
		"key": FromRaw("old", TypeString, SourceFile, 0),
	}
	new_ := map[string]Value{
		"key": FromRaw("new", TypeString, SourceEnv, 0),
	}

	events := ComputeDiff(old, new_)
	require.Len(t, events, 1)
	assert.Equal(t, DiffUpdated, events[0].Type)
	assert.Equal(t, SourceFile, events[0].OldSrc)
	assert.Equal(t, SourceEnv, events[0].NewSrc)
}

// ---------------------------------------------------------------------------
// ComputeDiffResult
// ---------------------------------------------------------------------------

func TestComputeDiffResult_NoChanges(t *testing.T) {
	old := map[string]Value{"a": New(1)}
	new_ := map[string]Value{"a": New(1)}

	result := ComputeDiffResult(old, new_)
	assert.False(t, result.HasChanges())
	assert.Equal(t, 0, result.Total())
	assert.Empty(t, result.Created)
	assert.Empty(t, result.Updated)
	assert.Empty(t, result.Deleted)
}

func TestComputeDiffResult_Mixed(t *testing.T) {
	old := map[string]Value{
		"a": New(1),
		"b": New(2),
	}
	new_ := map[string]Value{
		"a": New(10),
		"c": New(3),
	}

	result := ComputeDiffResult(old, new_)
	assert.True(t, result.HasChanges())
	assert.Equal(t, 3, result.Total())

	assert.Len(t, result.Created, 1)
	assert.Equal(t, "c", result.Created[0].Key)

	assert.Len(t, result.Updated, 1)
	assert.Equal(t, "a", result.Updated[0].Key)

	assert.Len(t, result.Deleted, 1)
	assert.Equal(t, "b", result.Deleted[0].Key)
}

func TestDiffResult_HasChanges_Nil(t *testing.T) {
	var r *DiffResult
	assert.False(t, r.HasChanges())
	assert.Equal(t, 0, r.Total())
}

func TestDiffResult_AllEvents(t *testing.T) {
	result := DiffResult{
		Created: []DiffEvent{{Key: "a", Type: DiffCreated}},
		Updated: []DiffEvent{{Key: "b", Type: DiffUpdated}},
		Deleted: []DiffEvent{{Key: "c", Type: DiffDeleted}},
	}

	all := result.AllEvents()
	assert.Len(t, all, 3)
	assert.Equal(t, "a", all[0].Key)
	assert.Equal(t, "b", all[1].Key)
	assert.Equal(t, "c", all[2].Key)
}

func TestDiffResult_AllEvents_Nil(t *testing.T) {
	var r *DiffResult
	assert.Nil(t, r.AllEvents())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func filterEvents(events []DiffEvent, dt DiffType) []DiffEvent {
	var out []DiffEvent
	for _, e := range events {
		if e.Type == dt {
			out = append(out, e)
		}
	}
	return out
}

func eventKeys(events []DiffEvent) []string {
	keys := make([]string, len(events))
	for i, e := range events {
		keys[i] = e.Key
	}
	return keys
}
