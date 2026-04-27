package event

import (
	"fmt"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// AuditAction
// ---------------------------------------------------------------------------

// AuditAction describes what kind of audit event occurred.
type AuditAction string

const (
	AuditActionConfigLoad     AuditAction = "config_load"
	AuditActionConfigChange   AuditAction = "config_change"
	AuditActionSecretAccess   AuditAction = "secret_access"
	AuditActionSecretRedacted AuditAction = "secret_redacted"
	AuditActionSourceError    AuditAction = "source_error"
	AuditActionReload         AuditAction = "reload"
	AuditActionWatch          AuditAction = "watch"
)

// ---------------------------------------------------------------------------
// AuditEntry
// ---------------------------------------------------------------------------

// AuditEntry is an immutable record of an auditable config operation.
// It is designed for structured logging, compliance trails, and SIEM ingestion.
type AuditEntry struct {
	Timestamp time.Time         // When the action occurred.
	Action    AuditAction       // What happened.
	Key       string            // The config key affected (may be empty for bulk ops).
	OldValue  string            // Redacted old value (always redacted).
	NewValue  string            // Redacted new value (always redacted).
	Source    string            // Config source (file, env, vault, etc.).
	TraceID   string            // Distributed trace ID.
	Actor     string            // Who or what initiated the action (service, user, process).
	Result    string            // "success" or "failure".
	Reason    string            // Human-readable description of why.
	Labels    map[string]string // Additional categorization labels.
	Error     string            // Error message if the operation failed.
	Redacted  bool              // Whether secret values were redacted in this entry.
}

// ---------------------------------------------------------------------------
// Recorder interface
// ---------------------------------------------------------------------------

// Recorder is the interface that audit consumers must implement.
// Each method receives a pre-constructed AuditEntry.
type Recorder interface {
	// RecordSecretRedacted is called when a secret value was detected and redacted.
	RecordSecretRedacted(entry AuditEntry)
	// RecordConfigChangeEvent is called when a configuration key changes.
	RecordConfigChangeEvent(entry AuditEntry)
}

// ---------------------------------------------------------------------------
// AuditEntry constructors
// ---------------------------------------------------------------------------

// NewAuditEntry creates a new AuditEntry with sensible defaults.
//
//nolint:gocritic // AuditEntry is immutable; copying is intentional for builder pattern
func NewAuditEntry(action AuditAction, key, source, actor string) AuditEntry {
	return AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Key:       key,
		Source:    source,
		Actor:     actor,
		Result:    "success",
		Redacted:  true, // always redact by default
	}
}

// WithError sets the error on an AuditEntry and marks it as failed.
//
//nolint:gocritic // AuditEntry is immutable; copying is intentional for builder pattern
func (e AuditEntry) WithError(err error) AuditEntry {
	e.Error = err.Error()
	e.Result = "failure"
	return e
}

// WithReason sets the human-readable reason.
//
//nolint:gocritic // AuditEntry is immutable; copying is intentional for builder pattern
func (e AuditEntry) WithReason(reason string) AuditEntry {
	e.Reason = reason
	return e
}

// WithTraceID sets the distributed trace ID.
//
//nolint:gocritic // AuditEntry is immutable; copying is intentional for builder pattern
func (e AuditEntry) WithTraceID(traceID string) AuditEntry {
	e.TraceID = traceID
	return e
}

// WithLabels adds labels to the audit entry.
//
//nolint:gocritic // AuditEntry is immutable; copying is intentional for builder pattern
func (e AuditEntry) WithLabels(labels map[string]string) AuditEntry {
	e.Labels = labels
	return e
}

// WithValues sets the old and new values on the audit entry.
// The values are always converted to strings and redacted if the key is a secret.
//
//nolint:gocritic // AuditEntry is immutable; copying is intentional for builder pattern
func (e AuditEntry) WithValues(oldVal, newVal value.Value) AuditEntry {
	oldStr := fmt.Sprint(oldVal.Raw())
	newStr := fmt.Sprint(newVal.Raw())

	if value.IsSecret(e.Key) {
		if oldStr != "" {
			oldStr = "***REDACTED***"
		}
		if newStr != "" {
			newStr = "***REDACTED***"
		}
		e.Redacted = true
	}

	e.OldValue = oldStr
	e.NewValue = newStr
	return e
}

// NewAuditEvent creates an Event of TypeAudit from an AuditEntry.
//
//nolint:gocritic // Event is immutable; copying is intentional for builder pattern
func NewAuditEvent(entry AuditEntry) Event {
	metadata := map[string]any{
		"action":   string(entry.Action),
		"actor":    entry.Actor,
		"result":   entry.Result,
		"redacted": entry.Redacted,
	}
	if entry.Reason != "" {
		metadata["reason"] = entry.Reason
	}
	if entry.Error != "" {
		metadata["error"] = entry.Error
	}

	opts := []Option{
		WithSource(entry.Source),
		WithTraceID(entry.TraceID),
		WithMetadata(metadata),
		WithLabels(entry.Labels),
		WithTimestamp(entry.Timestamp),
	}

	return New(TypeAudit, entry.Key, opts...)
}

// ---------------------------------------------------------------------------
// AuditRecorder
// ---------------------------------------------------------------------------

// AuditRecorder wraps a Recorder with convenience methods for creating
// and dispatching audit entries.
type AuditRecorder struct {
	recorder Recorder
	actor    string
}

// NewAuditRecorder creates an AuditRecorder that records to the given Recorder
// with the specified actor name.
func NewAuditRecorder(recorder Recorder, actor string) *AuditRecorder {
	return &AuditRecorder{
		recorder: recorder,
		actor:    actor,
	}
}

// RecordSecretRedacted records that a secret value was redacted.
func (r *AuditRecorder) RecordSecretRedacted(key, source string) {
	if r.recorder == nil {
		return
	}
	entry := NewAuditEntry(AuditActionSecretRedacted, key, source, r.actor).
		WithReason("secret value detected and redacted")
	r.recorder.RecordSecretRedacted(entry)
}

// RecordConfigChange records a configuration change event.
func (r *AuditRecorder) RecordConfigChange(key, source string, oldVal, newVal value.Value) {
	if r.recorder == nil {
		return
	}
	entry := NewAuditEntry(AuditActionConfigChange, key, source, r.actor).
		WithValues(oldVal, newVal).
		WithReason("configuration value changed")
	r.recorder.RecordConfigChangeEvent(entry)
}

// RecordConfigLoad records a successful config load.
func (r *AuditRecorder) RecordConfigLoad(source string, keyCount int) {
	if r.recorder == nil {
		return
	}
	entry := NewAuditEntry(AuditActionConfigLoad, "", source, r.actor).
		WithReason(fmt.Sprintf("loaded %d keys", keyCount))
	r.recorder.RecordConfigChangeEvent(entry)
}

// RecordSourceError records a source error.
func (r *AuditRecorder) RecordSourceError(source, key string, err error) {
	if r.recorder == nil {
		return
	}
	entry := NewAuditEntry(AuditActionSourceError, key, source, r.actor).
		WithError(err).
		WithReason("source operation failed")
	r.recorder.RecordConfigChangeEvent(entry)
}

// Recorder returns the underlying Recorder.
func (r *AuditRecorder) Recorder() Recorder {
	return r.recorder
}
