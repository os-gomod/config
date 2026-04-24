package event

import (
	"context"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/observability"
)

// AuditEntry represents a single audit log entry for configuration mutations.
// It captures who changed what, when, and from where, providing a complete
// compliance trail for SOC2, HIPAA, and similar regulatory requirements.
type AuditEntry struct {
	Operation string            // "set", "delete", "bind", "reload"
	Key       string            // the config key affected
	Actor     string            // who initiated the change (user, service, system)
	TraceID   string            // distributed trace ID for correlation
	Timestamp time.Time         // when the change occurred
	OldValue  value.Value       // previous value (may be zero)
	NewValue  value.Value       // new value (may be zero for deletes)
	Metadata  map[string]string // arbitrary audit metadata (IP, request ID, etc.)
	Redacted  bool              // true if secrets were redacted
}

// NewAuditEntry creates an audit entry with the given operation, key, actor,
// and trace ID. The Timestamp is set to time.Now() and Metadata is initialized
// to an empty map.
func NewAuditEntry(operation, key, actor, traceID string, _ ...Option) AuditEntry {
	return AuditEntry{
		Operation: operation,
		Key:       key,
		Actor:     actor,
		TraceID:   traceID,
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}
}

// NewAuditEvent converts an AuditEntry into a publishable Event.
// Secret values are redacted before being included in the event.
func NewAuditEvent(entry AuditEntry) Event {
	// Redact secrets
	oldVal := entry.OldValue
	newVal := entry.NewValue
	redacted := false
	if value.IsSecret(entry.Key) {
		oldVal = oldVal.Redact(entry.Key)
		newVal = newVal.Redact(entry.Key)
		redacted = true
	}
	entry.OldValue = oldVal
	entry.NewValue = newVal

	evt := New(TypeAudit, entry.Key,
		WithTraceID(entry.TraceID),
		WithLabel("operation", entry.Operation),
		WithLabel("actor", entry.Actor),
	)
	evt.OldValue = oldVal
	evt.NewValue = newVal
	if entry.Metadata != nil {
		for k, v := range entry.Metadata {
			evt.Labels["audit_"+k] = v
		}
	}
	evt.Metadata = map[string]any{
		"audit_operation": entry.Operation,
		"audit_actor":     entry.Actor,
		"audit_timestamp": entry.Timestamp.Format(time.RFC3339Nano),
		"audit_redacted":  redacted,
	}
	return evt
}

// AuditRecorder wraps an observability.Recorder with audit-specific methods.
type AuditRecorder struct {
	recorder observability.Recorder
}

// NewAuditRecorder creates an AuditRecorder backed by the given observability recorder.
func NewAuditRecorder(r observability.Recorder) *AuditRecorder {
	return &AuditRecorder{recorder: r}
}

// RecordAudit logs an audit event. If the key is a secret, it increments
// the secret_redacted counter.
func (ar *AuditRecorder) RecordAudit(ctx context.Context, entry AuditEntry) {
	if ar.recorder == nil {
		return
	}
	if value.IsSecret(entry.Key) {
		ar.recorder.RecordSecretRedacted(ctx, "audit:"+entry.Operation)
	}
	ar.recorder.RecordConfigChangeEvent(ctx, "audit_"+entry.Operation, entry.Actor)
}
