// Package event provides domain event types for configuration change tracking.
// Events are immutable value objects carrying rich context about what changed
// and when, supporting secret redaction for safe logging and audit trails.
package event

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// Observer is a function that receives configuration events.
// It is called by the event bus when a matching event is published.
// Returning a non-nil error signals delivery failure (may trigger retry).
type Observer func(ctx context.Context, evt Event) error

// ---------------------------------------------------------------------------
// EventType
// ---------------------------------------------------------------------------

// EventType classifies the kind of configuration event.
type EventType int

const (
	TypeCreate EventType = iota // A new config key was created.
	TypeUpdate                  // An existing key was updated.
	TypeDelete                  // A key was deleted.
	TypeReload                  // A config source was reloaded.
	TypeError                   // An error occurred during an operation.
	TypeWatch                   // A file or source watch event fired.
	TypeAudit                   // An audit-worthy event.
)

func (t EventType) String() string {
	switch t {
	case TypeCreate:
		return "create"
	case TypeUpdate:
		return "update"
	case TypeDelete:
		return "delete"
	case TypeReload:
		return "reload"
	case TypeError:
		return "error"
	case TypeWatch:
		return "watch"
	case TypeAudit:
		return "audit"
	default:
		return fmt.Sprintf("unknown(%d)", int(t))
	}
}

// ---------------------------------------------------------------------------
// Event
// ---------------------------------------------------------------------------

// Event represents a single configuration change event with rich metadata.
// Events are immutable after creation.
type Event struct {
	EventType EventType         // The type of event.
	Key       string            // The config key affected (may be empty for bulk events).
	OldValue  value.Value       // The previous value (zero if created).
	NewValue  value.Value       // The new value (zero if deleted).
	Timestamp time.Time         // When the event occurred.
	Source    string            // Name of the config source (file, env, vault, etc.).
	TraceID   string            // Distributed trace ID for correlation.
	Labels    map[string]string // Key-value labels for filtering/categorization.
	Metadata  map[string]any    // Additional metadata.
	Error     error             // Associated error, if any.
}

// ---------------------------------------------------------------------------
// Option pattern
// ---------------------------------------------------------------------------

// Option configures an Event during construction.
type Option func(*Event)

// WithSource sets the event source.
func WithSource(src string) Option {
	return func(e *Event) {
		e.Source = src
	}
}

// WithTraceID sets the trace ID.
func WithTraceID(traceID string) Option {
	return func(e *Event) {
		e.TraceID = traceID
	}
}

// WithLabels sets the event labels.
func WithLabels(labels map[string]string) Option {
	return func(e *Event) {
		e.Labels = labels
	}
}

// WithMetadata sets the event metadata.
func WithMetadata(metadata map[string]any) Option {
	return func(e *Event) {
		e.Metadata = metadata
	}
}

// WithError sets the event error.
func WithError(err error) Option {
	return func(e *Event) {
		e.Error = err
	}
}

// WithTimestamp sets the event timestamp.
func WithTimestamp(ts time.Time) Option {
	return func(e *Event) {
		e.Timestamp = ts
	}
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// New creates an Event with the given type and key, applying optional modifiers.
func New(eventType EventType, key string, opts ...Option) Event {
	e := Event{
		EventType: eventType,
		Key:       key,
		Timestamp: time.Now().UTC(),
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}

// NewCreateEvent creates a create event for a new config key.
func NewCreateEvent(key string, val value.Value, opts ...Option) Event {
	return New(TypeCreate, key, append(opts, func(e *Event) {
		e.NewValue = val
	})...)
}

// NewUpdateEvent creates an update event for a changed config key.
func NewUpdateEvent(key string, old, new_ value.Value, opts ...Option) Event {
	return New(TypeUpdate, key, append(opts, func(e *Event) {
		e.OldValue = old
		e.NewValue = new_
	})...)
}

// NewDeleteEvent creates a delete event for a removed config key.
func NewDeleteEvent(key string, old value.Value, opts ...Option) Event {
	return New(TypeDelete, key, append(opts, func(e *Event) {
		e.OldValue = old
	})...)
}

// NewReloadEvent creates a reload event for when a source is reloaded.
func NewReloadEvent(source string, opts ...Option) Event {
	return New(TypeReload, "", append(opts, WithSource(source))...)
}

// NewErrorEvent creates an error event.
func NewErrorEvent(key string, err error, opts ...Option) Event {
	return New(TypeError, key, append(opts, WithError(err))...)
}

// NewWatchEvent creates a watch event.
func NewWatchEvent(source, key string, opts ...Option) Event {
	return New(TypeWatch, key, append(opts, WithSource(source))...)
}

// ---------------------------------------------------------------------------
// Diff → Event conversion
// ---------------------------------------------------------------------------

// DiffEventsFromResult converts a DiffResult into a slice of Events.
// Each DiffEvent becomes a corresponding Event with the appropriate type.
func DiffEventsFromResult(result value.DiffResult, source string) []Event {
	if !result.HasChanges() {
		return nil
	}
	events := make([]Event, 0, result.Total())

	for _, d := range result.Created {
		events = append(events, NewCreateEvent(d.Key, d.New, WithSource(source)))
	}
	for _, d := range result.Updated {
		events = append(events, NewUpdateEvent(d.Key, d.Old, d.New, WithSource(source)))
	}
	for _, d := range result.Deleted {
		events = append(events, NewDeleteEvent(d.Key, d.Old, WithSource(source)))
	}

	return events
}

// NewDiffEvents creates events from a flat slice of DiffEvents.
func NewDiffEvents(diffs []value.DiffEvent, source string) []Event {
	if len(diffs) == 0 {
		return nil
	}
	events := make([]Event, 0, len(diffs))
	for _, d := range diffs {
		switch d.Type {
		case value.DiffCreated:
			events = append(events, NewCreateEvent(d.Key, d.New, WithSource(source)))
		case value.DiffUpdated:
			events = append(events, NewUpdateEvent(d.Key, d.Old, d.New, WithSource(source)))
		case value.DiffDeleted:
			events = append(events, NewDeleteEvent(d.Key, d.Old, WithSource(source)))
		}
	}
	return events
}

// ---------------------------------------------------------------------------
// Redaction
// ---------------------------------------------------------------------------

// RedactSecrets returns a copy of the Event with secret values redacted.
// The original Event is not modified.
func (e Event) RedactSecrets() Event {
	redacted := e // shallow copy
	if value.IsSecret(e.Key) {
		redacted.OldValue = e.OldValue.Redact(e.Key)
		redacted.NewValue = e.NewValue.Redact(e.Key)
	}

	// Redact secrets in metadata if it contains maps with sensitive keys.
	if e.Metadata != nil {
		newMeta := make(map[string]any, len(e.Metadata))
		for k, v := range e.Metadata {
			if value.IsSecret(k) {
				newMeta[k] = "***REDACTED***"
			} else {
				newMeta[k] = v
			}
		}
		redacted.Metadata = newMeta
	}

	return redacted
}

// RedactedDiffEvents returns a copy of the events slice with all secret values redacted.
func RedactedDiffEvents(events []Event) []Event {
	if events == nil {
		return nil
	}
	result := make([]Event, len(events))
	for i, e := range events {
		result[i] = e.RedactSecrets()
	}
	return result
}

// ---------------------------------------------------------------------------
// Event methods
// ---------------------------------------------------------------------------

// IsEmpty returns true if the event has no meaningful content.
func (e Event) IsEmpty() bool {
	return e.EventType == 0 && e.Key == "" && e.OldValue.IsZero() && e.NewValue.IsZero()
}

// String returns a human-readable summary of the event.
func (e Event) String() string {
	var b strings.Builder
	b.WriteString("[")
	b.WriteString(e.EventType.String())
	b.WriteString("]")
	if e.Key != "" {
		b.WriteString(" key=")
		b.WriteString(e.Key)
	}
	if !e.OldValue.IsZero() && e.EventType != TypeCreate {
		b.WriteString(" old=")
		fmt.Fprint(&b, e.OldValue.Raw())
	}
	if !e.NewValue.IsZero() && e.EventType != TypeDelete {
		b.WriteString(" new=")
		fmt.Fprint(&b, e.NewValue.Raw())
	}
	if e.Source != "" {
		b.WriteString(" src=")
		b.WriteString(e.Source)
	}
	if e.TraceID != "" {
		b.WriteString(" trace=")
		b.WriteString(e.TraceID)
	}
	if e.Error != nil {
		b.WriteString(" err=")
		b.WriteString(e.Error.Error())
	}
	b.WriteString(" @")
	b.WriteString(e.Timestamp.Format(time.RFC3339Nano))
	return b.String()
}

// ---------------------------------------------------------------------------
// Event filtering helpers
// ---------------------------------------------------------------------------

// FilterByType returns only events matching the given type.
func FilterByType(events []Event, eventType EventType) []Event {
	var filtered []Event
	for _, e := range events {
		if e.EventType == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// FilterByKey returns only events matching the given key.
func FilterByKey(events []Event, key string) []Event {
	var filtered []Event
	for _, e := range events {
		if e.Key == key {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// FilterBySource returns only events from the given source.
func FilterBySource(events []Event, source string) []Event {
	var filtered []Event
	for _, e := range events {
		if e.Source == source {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// FilterAfter returns only events that occurred after the given time.
func FilterAfter(events []Event, after time.Time) []Event {
	var filtered []Event
	for _, e := range events {
		if e.Timestamp.After(after) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
