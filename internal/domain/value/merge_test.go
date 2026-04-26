package value

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Merge
// ---------------------------------------------------------------------------

func TestMerge(t *testing.T) {
	t.Run("single_map", func(t *testing.T) {
		m := map[string]Value{"a": New(1), "b": New(2)}
		result, plan := Merge(m)
		assert.Equal(t, m, result)
		assert.Len(t, plan.Mutations, 2)
	})

	t.Run("no_maps", func(t *testing.T) {
		result, plan := Merge()
		assert.Empty(t, result)
		assert.Empty(t, plan.Mutations)
		assert.Empty(t, plan.Layers)
	})

	t.Run("later_overrides_earlier", func(t *testing.T) {
		m1 := map[string]Value{"a": New(1), "b": New(2)}
		m2 := map[string]Value{"b": New(20), "c": New(3)}

		result, plan := Merge(m1, m2)
		assert.Equal(t, 1, result["a"].Int())
		assert.Equal(t, 20, result["b"].Int()) // overridden
		assert.Equal(t, 3, result["c"].Int())

		assert.Len(t, plan.Mutations, 4)
		assert.Len(t, plan.Layers, 2)
	})

	t.Run("three_maps_priority", func(t *testing.T) {
		m1 := map[string]Value{"a": New(1)}
		m2 := map[string]Value{"a": New(2)}
		m3 := map[string]Value{"a": New(3)}

		result, _ := Merge(m1, m2, m3)
		assert.Equal(t, 3, result["a"].Int())
	})

	t.Run("same_value_no_duplicate_mutation", func(t *testing.T) {
		m1 := map[string]Value{"a": New(1)}
		m2 := map[string]Value{"a": New(1)} // same value

		result, plan := Merge(m1, m2)
		assert.Equal(t, 1, result["a"].Int())
		// Only one mutation since the second map has the same value
		assert.Len(t, plan.Mutations, 1)
	})
}

// ---------------------------------------------------------------------------
// MergeWithLayerNames
// ---------------------------------------------------------------------------

func TestMergeWithLayerNames(t *testing.T) {
	t.Run("with_names", func(t *testing.T) {
		m1 := map[string]Value{"a": New(1)}
		m2 := map[string]Value{"b": New(2)}

		result, plan := MergeWithLayerNames([]map[string]Value{m1, m2}, []string{"defaults", "override"})

		assert.Equal(t, 1, result["a"].Int())
		assert.Equal(t, 2, result["b"].Int())
		assert.Equal(t, []string{"defaults", "override"}, plan.Layers)
		assert.Equal(t, "defaults", plan.Mutations[0].Source)
		assert.Equal(t, "override", plan.Mutations[1].Source)
	})

	t.Run("nil_names", func(t *testing.T) {
		m1 := map[string]Value{"a": New(1)}
		result, plan := MergeWithLayerNames([]map[string]Value{m1}, nil)

		assert.Equal(t, 1, result["a"].Int())
		require.Len(t, plan.Layers, 1)
		assert.Equal(t, "unnamed-\x30", plan.Layers[0])
	})

	t.Run("names_shorter_than_maps", func(t *testing.T) {
		m1 := map[string]Value{"a": New(1)}
		m2 := map[string]Value{"b": New(2)}

		_, plan := MergeWithLayerNames([]map[string]Value{m1, m2}, []string{"layer1"})

		require.Len(t, plan.Layers, 2)
		assert.Equal(t, "layer1", plan.Layers[0])
		assert.Equal(t, "unnamed-\x31", plan.Layers[1])
	})
}

// ---------------------------------------------------------------------------
// ApplyDelta
// ---------------------------------------------------------------------------

func TestApplyDelta(t *testing.T) {
	base := map[string]Value{
		"a": New(1),
		"b": New(2),
		"c": New(3),
	}

	t.Run("set_mutation", func(t *testing.T) {
		mutations := []Mutation{
			NewSetMutation("a", New(10), "test"),
		}
		result := ApplyDelta(base, mutations)
		assert.Equal(t, 10, result["a"].Int())
		assert.Equal(t, 2, result["b"].Int())
		assert.Equal(t, 3, result["c"].Int())
		// Base is not modified
		assert.Equal(t, 1, base["a"].Int())
	})

	t.Run("delete_mutation", func(t *testing.T) {
		mutations := []Mutation{
			NewDeleteMutation("b", "test"),
		}
		result := ApplyDelta(base, mutations)
		assert.Equal(t, 1, result["a"].Int())
		_, exists := result["b"]
		assert.False(t, exists)
		assert.Equal(t, 3, result["c"].Int())
	})

	t.Run("mixed_mutations", func(t *testing.T) {
		mutations := []Mutation{
			NewSetMutation("a", New(100), "test"),
			NewDeleteMutation("c", "test"),
			NewSetMutation("d", New(4), "test"),
		}
		result := ApplyDelta(base, mutations)
		assert.Equal(t, 100, result["a"].Int())
		assert.Equal(t, 2, result["b"].Int())
		_, exists := result["c"]
		assert.False(t, exists)
		assert.Equal(t, 4, result["d"].Int())
	})

	t.Run("nil_base", func(t *testing.T) {
		result := ApplyDelta(nil, []Mutation{NewSetMutation("x", New(1), "test")})
		assert.Equal(t, 1, result["x"].Int())
	})

	t.Run("empty_mutations", func(t *testing.T) {
		result := ApplyDelta(base, nil)
		assert.Equal(t, base, result)
	})
}

// ---------------------------------------------------------------------------
// ComputeDeltaEvents
// ---------------------------------------------------------------------------

func TestComputeDeltaEvents(t *testing.T) {
	old := map[string]Value{"a": New(1), "b": New(2)}
	new_ := map[string]Value{"a": New(10), "c": New(3)}

	events := ComputeDeltaEvents(old, new_, "reload")

	require.Len(t, events, 3)

	// Count by kind
	sets, deletes := GroupMutationsByKind(events)
	assert.Len(t, sets, 2)    // "a" updated, "c" created
	assert.Len(t, deletes, 1) // "b" deleted

	// All should have source "reload"
	for _, m := range events {
		assert.Equal(t, "reload", m.Source)
	}
}

// ---------------------------------------------------------------------------
// MutationKind
// ---------------------------------------------------------------------------

func TestMutationKind_String(t *testing.T) {
	tests := []struct {
		kind MutationKind
		want string
	}{
		{MutationSet, "set"},
		{MutationDelete, "delete"},
		{MutationKind(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}

// ---------------------------------------------------------------------------
// NewSetMutation / NewDeleteMutation
// ---------------------------------------------------------------------------

func TestNewSetMutation(t *testing.T) {
	m := NewSetMutation("key", New(42), "source")
	assert.Equal(t, "key", m.Key)
	assert.Equal(t, MutationSet, m.Kind)
	assert.Equal(t, 42, m.Value.Int())
	assert.Equal(t, "source", m.Source)
}

func TestNewDeleteMutation(t *testing.T) {
	m := NewDeleteMutation("key", "source")
	assert.Equal(t, "key", m.Key)
	assert.Equal(t, MutationDelete, m.Kind)
	assert.True(t, m.Value.IsZero())
	assert.Equal(t, "source", m.Source)
}

// ---------------------------------------------------------------------------
// GroupMutationsByKind
// ---------------------------------------------------------------------------

func TestGroupMutationsByKind(t *testing.T) {
	mutations := []Mutation{
		NewSetMutation("a", New(1), "s"),
		NewSetMutation("b", New(2), "s"),
		NewDeleteMutation("c", "s"),
	}

	sets, deletes := GroupMutationsByKind(mutations)
	assert.Len(t, sets, 2)
	assert.Len(t, deletes, 1)
}

func TestGroupMutationsByKind_Empty(t *testing.T) {
	sets, deletes := GroupMutationsByKind(nil)
	assert.Empty(t, sets)
	assert.Empty(t, deletes)
}

// ---------------------------------------------------------------------------
// MutationKeys
// ---------------------------------------------------------------------------

func TestMutationKeys(t *testing.T) {
	mutations := []Mutation{
		NewSetMutation("c", New(1), "s"),
		NewSetMutation("a", New(1), "s"),
		NewDeleteMutation("b", "s"),
	}

	keys := MutationKeys(mutations)
	assert.Equal(t, []string{"a", "b", "c"}, keys)
}

func TestMutationKeys_Empty(t *testing.T) {
	assert.Empty(t, MutationKeys(nil))
}

// ---------------------------------------------------------------------------
// MergePlan
// ---------------------------------------------------------------------------

func TestMergePlan(t *testing.T) {
	m1 := map[string]Value{"a": New(1)}
	m2 := map[string]Value{"b": New(2)}

	_, plan := Merge(m1, m2)
	assert.Len(t, plan.Layers, 2)
	assert.Len(t, plan.Mutations, 2)
	assert.Equal(t, MutationSet, plan.Mutations[0].Kind)
	assert.Equal(t, MutationSet, plan.Mutations[1].Kind)
}

// ---------------------------------------------------------------------------
// MergeWithPriorityPlan
// ---------------------------------------------------------------------------

func TestMergeWithPriorityPlan(t *testing.T) {
	m1 := map[string]Value{"a": New(1)}
	result, plan := MergeWithPriorityPlan([]map[string]Value{m1}, []string{"layer1"})

	assert.Equal(t, 1, result["a"].Int())
	assert.Len(t, plan.Layers, 1)
	assert.Equal(t, "layer1", plan.Layers[0])
}
