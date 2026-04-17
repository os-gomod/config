package value

import (
	"testing"
)

func TestMerge(t *testing.T) {
	t.Run("no maps returns empty", func(t *testing.T) {
		result, plan := Merge()
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d keys", len(result))
		}
		if plan.TotalKeys != 0 {
			t.Errorf("expected 0 total keys, got %d", plan.TotalKeys)
		}
	})

	t.Run("single map", func(t *testing.T) {
		m := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 10),
			"b": New("2", TypeString, SourceMemory, 20),
		}
		result, plan := Merge(m)
		if len(result) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(result))
		}
		if plan.TotalKeys != 2 {
			t.Errorf("expected TotalKeys=2, got %d", plan.TotalKeys)
		}
		if len(plan.Order) != 2 {
			t.Errorf("expected Order length 2, got %d", len(plan.Order))
		}
	})

	t.Run("higher priority wins", func(t *testing.T) {
		m1 := map[string]Value{
			"key": New("low", TypeString, SourceMemory, 10),
		}
		m2 := map[string]Value{
			"key": New("high", TypeString, SourceFile, 50),
		}
		result, _ := Merge(m1, m2)
		if result["key"].String() != "high" {
			t.Errorf("expected 'high', got %q", result["key"].String())
		}
	})

	t.Run("same priority keeps first seen", func(t *testing.T) {
		m1 := map[string]Value{
			"key": New("first", TypeString, SourceMemory, 10),
		}
		m2 := map[string]Value{
			"key": New("second", TypeString, SourceFile, 10),
		}
		result, _ := Merge(m1, m2)
		if result["key"].String() != "first" {
			t.Errorf("expected 'first' (first seen), got %q", result["key"].String())
		}
	})

	t.Run("keys from all maps present", func(t *testing.T) {
		m1 := map[string]Value{"a": New("1", TypeString, SourceMemory, 10)}
		m2 := map[string]Value{"b": New("2", TypeString, SourceMemory, 20)}
		m3 := map[string]Value{"c": New("3", TypeString, SourceMemory, 30)}

		result, _ := Merge(m1, m2, m3)
		if len(result) != 3 {
			t.Fatalf("expected 3 keys, got %d", len(result))
		}
	})

	t.Run("order is sorted", func(t *testing.T) {
		m := map[string]Value{
			"z": New("1", TypeString, SourceMemory, 0),
			"a": New("2", TypeString, SourceMemory, 0),
			"m": New("3", TypeString, SourceMemory, 0),
		}
		_, plan := Merge(m)
		for i := 1; i < len(plan.Order); i++ {
			if plan.Order[i] < plan.Order[i-1] {
				t.Errorf("order not sorted: %v", plan.Order)
			}
		}
	})

	t.Run("overridden keys tracked", func(t *testing.T) {
		m1 := map[string]Value{
			"key": New("low", TypeString, SourceMemory, 10),
		}
		m2 := map[string]Value{
			"key": New("high", TypeString, SourceFile, 50),
		}
		_, plan := Merge(m1, m2)
		if len(plan.OverriddenKeys) != 1 {
			t.Fatalf("expected 1 overridden key, got %d", len(plan.OverriddenKeys))
		}
		if plan.OverriddenKeys[0] != "key" {
			t.Errorf("expected overridden key 'key', got %q", plan.OverriddenKeys[0])
		}
	})

	t.Run("hash is computed", func(t *testing.T) {
		m := map[string]Value{"a": New("1", TypeString, SourceMemory, 0)}
		_, plan := Merge(m)
		if len(plan.Hash) != 64 {
			t.Errorf("expected 64-char hex hash, got %d", len(plan.Hash))
		}
	})
}

func TestMergeWithLayerNames(t *testing.T) {
	t.Run("tracks layer attribution", func(t *testing.T) {
		m1 := map[string]Value{
			"db.host": New("localhost", TypeString, SourceFile, 50),
		}
		m2 := map[string]Value{
			"db.port": New("5432", TypeString, SourceMemory, 20),
		}
		names := []string{"file", "memory"}

		result, plan := MergeWithLayerNames([]map[string]Value{m1, m2}, names)

		if len(result) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(result))
		}
		if plan.LayerNames == nil {
			t.Fatal("expected LayerNames to be set")
		}
		if len(plan.Layers["file"]) != 1 || plan.Layers["file"][0] != "db.host" {
			t.Errorf("expected file layer to have db.host, got %v", plan.Layers["file"])
		}
		if len(plan.Layers["memory"]) != 1 || plan.Layers["memory"][0] != "db.port" {
			t.Errorf("expected memory layer to have db.port, got %v", plan.Layers["memory"])
		}
	})

	t.Run("nil names defaults to empty string", func(t *testing.T) {
		m1 := map[string]Value{"a": New("1", TypeString, SourceMemory, 10)}
		result, plan := MergeWithLayerNames([]map[string]Value{m1}, nil)
		if len(result) != 1 {
			t.Fatalf("expected 1 key, got %d", len(result))
		}
		// Layer name should be empty string for index beyond names slice
		keys, ok := plan.Layers[""]
		if !ok || len(keys) != 1 {
			t.Errorf("expected empty-name layer, got %v", plan.Layers)
		}
	})
}

func TestMergeWithPriorityPlan(t *testing.T) {
	t.Run("same merge behavior without override tracking", func(t *testing.T) {
		m1 := map[string]Value{
			"key": New("low", TypeString, SourceMemory, 10),
		}
		m2 := map[string]Value{
			"key": New("high", TypeString, SourceFile, 50),
		}

		result, plan := MergeWithPriorityPlan(m1, m2)
		if result["key"].String() != "high" {
			t.Errorf("expected 'high', got %q", result["key"].String())
		}
		if len(plan.OverriddenKeys) != 0 {
			t.Errorf("expected no overridden keys, got %v", plan.OverriddenKeys)
		}
		if plan.TotalKeys != 1 {
			t.Errorf("expected 1 total key, got %d", plan.TotalKeys)
		}
	})

	t.Run("no maps returns empty", func(t *testing.T) {
		result, _ := MergeWithPriorityPlan()
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d keys", len(result))
		}
	})
}

func TestApplyDelta(t *testing.T) {
	t.Run("set mutations", func(t *testing.T) {
		base := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}
		mutations := []Mutation{
			NewSetMutation("b", New("2", TypeString, SourceMemory, 0)),
			NewSetMutation("a", New("updated", TypeString, SourceMemory, 0)),
		}

		result := ApplyDelta(base, mutations)
		if len(result) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(result))
		}
		if result["a"].String() != "updated" {
			t.Errorf("expected 'updated', got %q", result["a"].String())
		}
		if result["b"].String() != "2" {
			t.Errorf("expected '2', got %q", result["b"].String())
		}
	})

	t.Run("delete mutations", func(t *testing.T) {
		base := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
			"b": New("2", TypeString, SourceMemory, 0),
		}
		mutations := []Mutation{
			NewDeleteMutation("a"),
		}

		result := ApplyDelta(base, mutations)
		if len(result) != 1 {
			t.Fatalf("expected 1 key, got %d", len(result))
		}
		if _, exists := result["a"]; exists {
			t.Error("key 'a' should have been deleted")
		}
	})

	t.Run("base is not modified", func(t *testing.T) {
		base := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}
		mutations := []Mutation{
			NewSetMutation("b", New("2", TypeString, SourceMemory, 0)),
		}

		_ = ApplyDelta(base, mutations)
		if len(base) != 1 {
			t.Errorf("base was modified, expected 1 key, got %d", len(base))
		}
	})

	t.Run("nil base returns empty map", func(t *testing.T) {
		result := ApplyDelta(nil, []Mutation{})
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 keys, got %d", len(result))
		}
	})

	t.Run("empty mutations returns copy of base", func(t *testing.T) {
		base := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}
		result := ApplyDelta(base, nil)
		if len(result) != len(base) {
			t.Errorf("expected same length, got %d vs %d", len(result), len(base))
		}
	})
}

func TestComputeDeltaEvents(t *testing.T) {
	t.Run("set mutation creates diff events", func(t *testing.T) {
		base := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}
		mutations := []Mutation{
			NewSetMutation("a", New("updated", TypeString, SourceMemory, 0)),
			NewSetMutation("b", New("new", TypeString, SourceMemory, 0)),
		}

		events := ComputeDeltaEvents(base, mutations)
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}

		typeCount := map[DiffType]int{}
		for _, e := range events {
			typeCount[e.Type]++
		}
		if typeCount[DiffUpdated] != 1 {
			t.Errorf("expected 1 updated, got %d", typeCount[DiffUpdated])
		}
		if typeCount[DiffCreated] != 1 {
			t.Errorf("expected 1 created, got %d", typeCount[DiffCreated])
		}
	})

	t.Run("delete mutation creates diff event", func(t *testing.T) {
		base := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}
		mutations := []Mutation{
			NewDeleteMutation("a"),
		}

		events := ComputeDeltaEvents(base, mutations)
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != DiffDeleted {
			t.Errorf("expected DiffDeleted, got %d", events[0].Type)
		}
	})

	t.Run("no-op mutations produce no events", func(t *testing.T) {
		base := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}
		mutations := []Mutation{
			NewSetMutation("a", New("1", TypeString, SourceMemory, 0)),
		}

		events := ComputeDeltaEvents(base, mutations)
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})
}

func TestNewSetMutation(t *testing.T) {
	m := NewSetMutation("k", New("v", TypeString, SourceMemory, 0))
	if m.Kind != MutationSet {
		t.Errorf("expected MutationSet, got %d", m.Kind)
	}
	if m.Key != "k" {
		t.Errorf("expected key 'k', got %q", m.Key)
	}
	if m.Value.String() != "v" {
		t.Errorf("expected value 'v', got %q", m.Value.String())
	}
}

func TestNewDeleteMutation(t *testing.T) {
	m := NewDeleteMutation("k")
	if m.Kind != MutationDelete {
		t.Errorf("expected MutationDelete, got %d", m.Kind)
	}
	if m.Key != "k" {
		t.Errorf("expected key 'k', got %q", m.Key)
	}
}

func TestMerge_EmptyMapHandling(t *testing.T) {
	t.Run("merge with nil maps", func(t *testing.T) {
		m1 := map[string]Value{"a": New("1", TypeString, SourceMemory, 0)}
		result, _ := Merge(nil, m1, nil)
		if len(result) != 1 {
			t.Fatalf("expected 1 key, got %d", len(result))
		}
	})

	t.Run("merge with empty maps", func(t *testing.T) {
		m1 := map[string]Value{"a": New("1", TypeString, SourceMemory, 0)}
		result, _ := Merge(map[string]Value{}, m1)
		if len(result) != 1 {
			t.Fatalf("expected 1 key, got %d", len(result))
		}
	})
}
