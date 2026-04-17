package value

import (
	"testing"
)

func TestDiffType_String(t *testing.T) {
	tests := []struct {
		dt   DiffType
		want string
	}{
		{DiffNone, "none"},
		{DiffCreated, "created"},
		{DiffUpdated, "updated"},
		{DiffDeleted, "deleted"},
		{DiffType(99), "none"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.dt.String(); got != tt.want {
				t.Errorf("DiffType(%d).String() = %q, want %q", tt.dt, got, tt.want)
			}
		})
	}
}

func TestComputeDiff(t *testing.T) {
	t.Run("additions only", func(t *testing.T) {
		oldMap := map[string]Value{}
		newMap := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
			"b": New("2", TypeString, SourceMemory, 0),
		}

		events := ComputeDiff(oldMap, newMap)
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		for _, e := range events {
			if e.Type != DiffCreated {
				t.Errorf("expected DiffCreated, got %d for key %s", e.Type, e.Key)
			}
		}
	})

	t.Run("deletions only", func(t *testing.T) {
		oldMap := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
			"b": New("2", TypeString, SourceMemory, 0),
		}
		newMap := map[string]Value{}

		events := ComputeDiff(oldMap, newMap)
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		for _, e := range events {
			if e.Type != DiffDeleted {
				t.Errorf("expected DiffDeleted, got %d for key %s", e.Type, e.Key)
			}
		}
	})

	t.Run("updates only", func(t *testing.T) {
		oldMap := map[string]Value{
			"a": New("old", TypeString, SourceMemory, 0),
			"b": New("keep", TypeString, SourceMemory, 0),
		}
		newMap := map[string]Value{
			"a": New("new", TypeString, SourceMemory, 0),
			"b": New("keep", TypeString, SourceMemory, 0),
		}

		events := ComputeDiff(oldMap, newMap)
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != DiffUpdated {
			t.Errorf("expected DiffUpdated, got %d", events[0].Type)
		}
		if events[0].Key != "a" {
			t.Errorf("expected key 'a', got %q", events[0].Key)
		}
	})

	t.Run("mixed changes", func(t *testing.T) {
		oldMap := map[string]Value{
			"keep":   New("same", TypeString, SourceMemory, 0),
			"update": New("old", TypeString, SourceMemory, 0),
			"delete": New("val", TypeString, SourceMemory, 0),
		}
		newMap := map[string]Value{
			"keep":   New("same", TypeString, SourceMemory, 0),
			"update": New("new", TypeString, SourceMemory, 0),
			"create": New("val", TypeString, SourceMemory, 0),
		}

		events := ComputeDiff(oldMap, newMap)

		typeCount := map[DiffType]int{}
		for _, e := range events {
			typeCount[e.Type]++
		}

		if typeCount[DiffCreated] != 1 {
			t.Errorf("expected 1 created, got %d", typeCount[DiffCreated])
		}
		if typeCount[DiffUpdated] != 1 {
			t.Errorf("expected 1 updated, got %d", typeCount[DiffUpdated])
		}
		if typeCount[DiffDeleted] != 1 {
			t.Errorf("expected 1 deleted, got %d", typeCount[DiffDeleted])
		}
		if len(events) != 3 {
			t.Errorf("expected 3 total events, got %d", len(events))
		}
	})

	t.Run("no changes", func(t *testing.T) {
		oldMap := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}
		newMap := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 0),
		}

		events := ComputeDiff(oldMap, newMap)
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("both empty", func(t *testing.T) {
		events := ComputeDiff(map[string]Value{}, map[string]Value{})
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("nil maps", func(t *testing.T) {
		events := ComputeDiff(nil, nil)
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("type change counts as update", func(t *testing.T) {
		oldMap := map[string]Value{
			"k": New(42, TypeInt, SourceMemory, 0),
		}
		newMap := map[string]Value{
			"k": New("42", TypeString, SourceMemory, 0),
		}

		events := ComputeDiff(oldMap, newMap)
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != DiffUpdated {
			t.Errorf("expected DiffUpdated for type change, got %d", events[0].Type)
		}
	})
}

func TestComputeDiffResult(t *testing.T) {
	t.Run("counts all event types", func(t *testing.T) {
		oldMap := map[string]Value{
			"keep":   New("same", TypeString, SourceMemory, 0),
			"update": New("old", TypeString, SourceMemory, 0),
			"delete": New("val", TypeString, SourceMemory, 0),
		}
		newMap := map[string]Value{
			"keep":   New("same", TypeString, SourceMemory, 0),
			"update": New("new", TypeString, SourceMemory, 0),
			"create": New("val", TypeString, SourceMemory, 0),
		}

		r := ComputeDiffResult(oldMap, newMap)
		if r.Created != 1 {
			t.Errorf("expected Created=1, got %d", r.Created)
		}
		if r.Updated != 1 {
			t.Errorf("expected Updated=1, got %d", r.Updated)
		}
		if r.Deleted != 1 {
			t.Errorf("expected Deleted=1, got %d", r.Deleted)
		}
		if r.Unchanged != 1 {
			t.Errorf("expected Unchanged=1, got %d", r.Unchanged)
		}
		if !r.HasChanges() {
			t.Error("expected HasChanges to be true")
		}
	})

	t.Run("no changes", func(t *testing.T) {
		data := map[string]Value{
			"k": New("v", TypeString, SourceMemory, 0),
		}
		r := ComputeDiffResult(data, data)
		if r.HasChanges() {
			t.Error("expected HasChanges to be false")
		}
		if r.Unchanged != 1 {
			t.Errorf("expected Unchanged=1, got %d", r.Unchanged)
		}
	})

	t.Run("nil maps", func(t *testing.T) {
		r := ComputeDiffResult(nil, nil)
		if r.HasChanges() {
			t.Error("expected no changes for nil maps")
		}
		if r.Created != 0 || r.Deleted != 0 || r.Updated != 0 {
			t.Error("expected zero counts for nil maps")
		}
	})
}

func TestDiffEvent_Fields(t *testing.T) {
	evt := DiffEvent{
		Type:     DiffUpdated,
		Key:      "app.port",
		OldValue: New("8080", TypeString, SourceMemory, 0),
		NewValue: New("9090", TypeString, SourceMemory, 0),
	}
	if evt.Type != DiffUpdated {
		t.Errorf("expected Type DiffUpdated, got %v", evt.Type)
	}
	if evt.Key != "app.port" {
		t.Errorf("expected key 'app.port', got %q", evt.Key)
	}
	if evt.OldValue.String() != "8080" {
		t.Errorf("expected OldValue '8080', got %q", evt.OldValue.String())
	}
	if evt.NewValue.String() != "9090" {
		t.Errorf("expected NewValue '9090', got %q", evt.NewValue.String())
	}
}
