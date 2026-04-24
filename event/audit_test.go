package event

import (
	"context"
	"testing"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/observability"
)

func TestNewAuditEntry(t *testing.T) {
	entry := NewAuditEntry("set", "key", "user", "trace-123")
	if entry.Operation != "set" {
		t.Errorf("expected operation 'set', got %s", entry.Operation)
	}
	if entry.Key != "key" {
		t.Errorf("expected key 'key', got %s", entry.Key)
	}
	if entry.Actor != "user" {
		t.Errorf("expected actor 'user', got %s", entry.Actor)
	}
	if entry.TraceID != "trace-123" {
		t.Errorf("expected traceID 'trace-123', got %s", entry.TraceID)
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if entry.Metadata == nil {
		t.Error("expected non-nil metadata")
	}
}

func TestNewAuditEvent_SecretRedaction(t *testing.T) {
	entry := NewAuditEntry("set", "db.password", "user", "trace-1")
	entry.OldValue = value.NewInMemory("oldsecret")
	entry.NewValue = value.NewInMemory("newsecret")
	evt := NewAuditEvent(entry)
	if evt.OldValue.String() != "[REDACTED]" {
		t.Errorf("expected old value redacted, got %s", evt.OldValue.String())
	}
	if evt.NewValue.String() != "[REDACTED]" {
		t.Errorf("expected new value redacted, got %s", evt.NewValue.String())
	}
}

func TestNewAuditEvent_NonSecret(t *testing.T) {
	entry := NewAuditEntry("set", "app.name", "user", "trace-1")
	entry.OldValue = value.NewInMemory("old")
	entry.NewValue = value.NewInMemory("new")
	evt := NewAuditEvent(entry)
	if evt.OldValue.String() != "old" {
		t.Errorf("expected old value unchanged, got %s", evt.OldValue.String())
	}
	if evt.NewValue.String() != "new" {
		t.Errorf("expected new value unchanged, got %s", evt.NewValue.String())
	}
}

func TestAuditRecorder_Nil(t *testing.T) {
	rec := NewAuditRecorder(nil)
	// Should not panic with nil recorder
	rec.RecordAudit(context.Background(), NewAuditEntry("set", "key", "user", "trace"))
}

func TestAuditRecorder_Nop(t *testing.T) {
	rec := NewAuditRecorder(observability.Nop())
	rec.RecordAudit(context.Background(), NewAuditEntry("set", "db.password", "user", "trace"))
	rec.RecordAudit(context.Background(), NewAuditEntry("set", "app.name", "user", "trace"))
	// Should not panic
}
