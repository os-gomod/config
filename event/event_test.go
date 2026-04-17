package event

import (
	"errors"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
)

func TestType_String(t *testing.T) {
	tests := []struct {
		typ  Type
		want string
	}{
		{TypeCreate, "create"},
		{TypeUpdate, "update"},
		{TypeDelete, "delete"},
		{TypeReload, "reload"},
		{TypeError, "error"},
		{TypeWatch, "watch"},
		{Type(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("Type(%d).String() = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("basic event creation", func(t *testing.T) {
		evt := New(TypeCreate, "app.name")
		if evt.Type != TypeCreate {
			t.Errorf("expected TypeCreate, got %d", evt.Type)
		}
		if evt.Key != "app.name" {
			t.Errorf("expected key 'app.name', got %q", evt.Key)
		}
		if evt.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("with options", func(t *testing.T) {
		evt := New(TypeUpdate, "app.port",
			WithTraceID("trace-123"),
			WithSource(value.SourceFile),
			WithLabel("env", "production"),
		)
		if evt.TraceID != "trace-123" {
			t.Errorf("expected trace ID 'trace-123', got %q", evt.TraceID)
		}
		if evt.Source != value.SourceFile {
			t.Errorf("expected SourceFile, got %d", evt.Source)
		}
		if evt.Labels == nil || evt.Labels["env"] != "production" {
			t.Error("expected label env=production")
		}
	})
}

func TestNewCreateEvent(t *testing.T) {
	newVal := value.New("localhost", value.TypeString, value.SourceFile, 10)
	evt := NewCreateEvent("db.host", newVal)

	if evt.Type != TypeCreate {
		t.Errorf("expected TypeCreate, got %d", evt.Type)
	}
	if evt.Key != "db.host" {
		t.Errorf("expected key 'db.host', got %q", evt.Key)
	}
	if !evt.NewValue.Equal(newVal) {
		t.Error("NewValue mismatch")
	}
	if !evt.OldValue.IsZero() {
		t.Error("expected OldValue to be zero")
	}
}

func TestNewUpdateEvent(t *testing.T) {
	oldVal := value.New("localhost", value.TypeString, value.SourceFile, 10)
	newVal := value.New("127.0.0.1", value.TypeString, value.SourceFile, 20)
	evt := NewUpdateEvent("db.host", oldVal, newVal)

	if evt.Type != TypeUpdate {
		t.Errorf("expected TypeUpdate, got %d", evt.Type)
	}
	if evt.Key != "db.host" {
		t.Errorf("expected key 'db.host', got %q", evt.Key)
	}
	if !evt.OldValue.Equal(oldVal) {
		t.Error("OldValue mismatch")
	}
	if !evt.NewValue.Equal(newVal) {
		t.Error("NewValue mismatch")
	}
}

func TestNewDeleteEvent(t *testing.T) {
	oldVal := value.New("localhost", value.TypeString, value.SourceFile, 10)
	evt := NewDeleteEvent("db.host", oldVal)

	if evt.Type != TypeDelete {
		t.Errorf("expected TypeDelete, got %d", evt.Type)
	}
	if evt.Key != "db.host" {
		t.Errorf("expected key 'db.host', got %q", evt.Key)
	}
	if !evt.OldValue.Equal(oldVal) {
		t.Error("OldValue mismatch")
	}
	if !evt.NewValue.IsZero() {
		t.Error("expected NewValue to be zero")
	}
}

func TestDiffEventsFromResult(t *testing.T) {
	tests := []struct {
		name     string
		input    []value.DiffEvent
		wantLen  int
		wantType []Type
	}{
		{
			name: "mixed diff events",
			input: []value.DiffEvent{
				{Type: value.DiffCreated, Key: "new.key", NewValue: value.New("v", value.TypeString, value.SourceMemory, 0)},
				{Type: value.DiffUpdated, Key: "upd.key", OldValue: value.New("a", value.TypeString, value.SourceMemory, 0), NewValue: value.New("b", value.TypeString, value.SourceMemory, 0)},
				{Type: value.DiffDeleted, Key: "del.key", OldValue: value.New("c", value.TypeString, value.SourceMemory, 0)},
			},
			wantLen:  3,
			wantType: []Type{TypeCreate, TypeUpdate, TypeDelete},
		},
		{
			name:     "empty input",
			input:    nil,
			wantLen:  0,
			wantType: nil,
		},
		{
			name:     "empty slice",
			input:    []value.DiffEvent{},
			wantLen:  0,
			wantType: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := DiffEventsFromResult(tt.input)
			if len(events) != tt.wantLen {
				t.Fatalf("expected %d events, got %d", tt.wantLen, len(events))
			}
			for i, typ := range tt.wantType {
				if events[i].Type != typ {
					t.Errorf("event[%d].Type = %d, want %d", i, events[i].Type, typ)
				}
			}
		})
	}
}

func TestNewDiffEvents(t *testing.T) {
	t.Run("detects creates, updates, and deletes", func(t *testing.T) {
		valKeep := value.New("same", value.TypeString, value.SourceMemory, 0)
		valUpdateOld := value.New("old", value.TypeString, value.SourceMemory, 0)
		valUpdateNew := value.New("new", value.TypeString, value.SourceMemory, 0)
		valDel := value.New("del", value.TypeString, value.SourceMemory, 0)
		valCreate := value.New("create", value.TypeString, value.SourceMemory, 0)

		oldData := map[string]value.Value{
			"keep":   valKeep,
			"update": valUpdateOld,
			"delete": valDel,
		}
		newData := map[string]value.Value{
			"keep":   valKeep,
			"update": valUpdateNew,
			"create": valCreate,
		}

		events := NewDiffEvents(oldData, newData)
		if len(events) != 3 {
			t.Fatalf("expected 3 events, got %d: %+v", len(events), events)
		}

		// Verify types
		typeCount := map[Type]int{}
		for _, e := range events {
			typeCount[e.Type]++
		}
		if typeCount[TypeCreate] != 1 {
			t.Errorf("expected 1 create, got %d", typeCount[TypeCreate])
		}
		if typeCount[TypeUpdate] != 1 {
			t.Errorf("expected 1 update, got %d", typeCount[TypeUpdate])
		}
		if typeCount[TypeDelete] != 1 {
			t.Errorf("expected 1 delete, got %d", typeCount[TypeDelete])
		}
	})

	t.Run("no changes returns empty", func(t *testing.T) {
		valA := value.New("alpha", value.TypeString, value.SourceMemory, 0)
		oldData := map[string]value.Value{"k": valA}
		newData := map[string]value.Value{"k": valA}

		events := NewDiffEvents(oldData, newData)
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("all new keys returns creates", func(t *testing.T) {
		newData := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 0),
			"b": value.New("2", value.TypeString, value.SourceMemory, 0),
		}
		events := NewDiffEvents(nil, newData)
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		for _, e := range events {
			if e.Type != TypeCreate {
				t.Errorf("expected all creates, got %d", e.Type)
			}
		}
	})

	t.Run("all deleted returns deletes", func(t *testing.T) {
		oldData := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 0),
			"b": value.New("2", value.TypeString, value.SourceMemory, 0),
		}
		events := NewDiffEvents(oldData, nil)
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		for _, e := range events {
			if e.Type != TypeDelete {
				t.Errorf("expected all deletes, got %d", e.Type)
			}
		}
	})

	t.Run("passes options to events", func(t *testing.T) {
		oldData := map[string]value.Value{
			"key": value.New("old", value.TypeString, value.SourceMemory, 0),
		}
		newData := map[string]value.Value{
			"key": value.New("new", value.TypeString, value.SourceMemory, 0),
		}

		events := NewDiffEvents(oldData, newData, WithTraceID("abc"))
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].TraceID != "abc" {
			t.Errorf("expected trace ID 'abc', got %q", events[0].TraceID)
		}
	})
}

func TestEventOptions(t *testing.T) {
	t.Run("WithTraceID", func(t *testing.T) {
		evt := New(TypeUpdate, "k", WithTraceID("tid-1"))
		if evt.TraceID != "tid-1" {
			t.Errorf("expected trace ID 'tid-1', got %q", evt.TraceID)
		}
	})

	t.Run("WithError", func(t *testing.T) {
		err := errors.New("something failed")
		evt := New(TypeError, "k", WithError(err))
		if evt.Error != err {
			t.Error("error mismatch")
		}
	})

	t.Run("WithSource", func(t *testing.T) {
		evt := New(TypeUpdate, "k", WithSource(value.SourceEnv))
		if evt.Source != value.SourceEnv {
			t.Errorf("expected SourceEnv, got %d", evt.Source)
		}
	})

	t.Run("WithLabel single", func(t *testing.T) {
		evt := New(TypeUpdate, "k", WithLabel("region", "us-east"))
		if evt.Labels == nil || evt.Labels["region"] != "us-east" {
			t.Error("label mismatch")
		}
	})

	t.Run("WithLabels multiple", func(t *testing.T) {
		evt := New(TypeUpdate, "k", WithLabels(map[string]string{
			"region": "us-east",
			"tier":   "prod",
		}))
		if evt.Labels == nil {
			t.Fatal("labels is nil")
		}
		if evt.Labels["region"] != "us-east" {
			t.Errorf("region = %q, want 'us-east'", evt.Labels["region"])
		}
		if evt.Labels["tier"] != "prod" {
			t.Errorf("tier = %q, want 'prod'", evt.Labels["tier"])
		}
	})

	t.Run("WithMetadata", func(t *testing.T) {
		evt := New(TypeUpdate, "k", WithMetadata("version", "2.0"))
		if evt.Metadata == nil || evt.Metadata["version"] != "2.0" {
			t.Error("metadata mismatch")
		}
	})

	t.Run("combined options", func(t *testing.T) {
		evt := New(TypeReload, "k",
			WithTraceID("combined"),
			WithSource(value.SourceHTTP),
			WithLabel("env", "test"),
			WithMetadata("count", 42),
			WithError(errors.New("test err")),
		)
		if evt.TraceID != "combined" {
			t.Error("trace ID mismatch")
		}
		if evt.Source != value.SourceHTTP {
			t.Error("source mismatch")
		}
		if evt.Labels["env"] != "test" {
			t.Error("label mismatch")
		}
		if evt.Metadata["count"] != 42 {
			t.Error("metadata mismatch")
		}
		if evt.Error == nil || evt.Error.Error() != "test err" {
			t.Error("error mismatch")
		}
	})
}

func TestEvent_Timestamp(t *testing.T) {
	before := time.Now()
	evt := New(TypeUpdate, "k")
	after := time.Now()

	if evt.Timestamp.Before(before) || evt.Timestamp.After(after) {
		t.Errorf("timestamp %v not between %v and %v", evt.Timestamp, before, after)
	}
}
