// Package event provides a typed event system for configuration lifecycle operations.
// Events represent changes such as key creation, updates, deletion, reloads, errors,
// and watch notifications. They carry metadata including old/new values, timestamps,
// tracing IDs, and optional labels for filtering and observability.
//
// Events are published through a Bus and can be subscribed to using glob patterns.
// This package also provides factory functions for common event types and diff-based
// event generation from value maps.
package event

import (
	"time"

	"github.com/os-gomod/config/core/value"
)

// Type represents the category of a configuration event.
type Type uint8

const (
	// TypeCreate indicates a new configuration key was created.
	TypeCreate Type = iota
	// TypeUpdate indicates an existing configuration key's value changed.
	TypeUpdate
	// TypeDelete indicates a configuration key was removed.
	TypeDelete
	// TypeReload indicates the entire configuration was reloaded.
	TypeReload
	// TypeError indicates an error occurred during a configuration operation.
	TypeError
	// TypeWatch indicates a filesystem or remote watch event was triggered.
	TypeWatch
)

// typeNames maps Type constants to their string representations.
var typeNames = [...]string{
	TypeCreate: "create",
	TypeUpdate: "update",
	TypeDelete: "delete",
	TypeReload: "reload",
	TypeError:  "error",
	TypeWatch:  "watch",
}

// String returns the human-readable name of the event type.
// Returns "unknown" for undefined types.
func (t Type) String() string {
	if int(t) < len(typeNames) {
		return typeNames[t]
	}
	return "unknown"
}

// Event represents a single configuration lifecycle event. It carries the event type,
// the affected key, old and new values, a timestamp, the value source, an optional
// trace ID for distributed tracing, and arbitrary labels and metadata.
type Event struct {
	Type      Type
	Key       string
	OldValue  value.Value
	NewValue  value.Value
	Timestamp time.Time
	Source    value.Source
	TraceID   string
	Labels    map[string]string
	Metadata  map[string]any
	Error     error
}

// New creates a new Event with the given type and key, applying any provided options.
// The timestamp is automatically set to the current time.
//
// Example:
//
//	evt := event.New(event.TypeCreate, "db.host",
//	    event.WithSource(value.SourceFile),
//	    event.WithTraceID("abc123"),
//	)
func New(typ Type, key string, opts ...Option) Event {
	e := Event{Type: typ, Key: key, Timestamp: time.Now()}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// newValueEvent creates a new event with both old and new values populated.
func newValueEvent(typ Type, key string, oldVal, newVal value.Value, opts ...Option) Event {
	evt := New(typ, key, opts...)
	evt.OldValue = oldVal
	evt.NewValue = newVal
	return evt
}

// NewCreateEvent creates a TypeCreate event for the given key and new value.
func NewCreateEvent(key string, newVal value.Value, opts ...Option) Event {
	return newValueEvent(TypeCreate, key, value.Value{}, newVal, opts...)
}

// NewUpdateEvent creates a TypeUpdate event for the given key with old and new values.
func NewUpdateEvent(key string, oldVal, newVal value.Value, opts ...Option) Event {
	return newValueEvent(TypeUpdate, key, oldVal, newVal, opts...)
}

// NewDeleteEvent creates a TypeDelete event for the given key with the old value.
func NewDeleteEvent(key string, oldVal value.Value, opts ...Option) Event {
	return newValueEvent(TypeDelete, key, oldVal, value.Value{}, opts...)
}

// DiffEventsFromResult converts a slice of value.DiffEvent into a slice of Event.
// Each diff event is mapped to the corresponding create, update, or delete event type.
func DiffEventsFromResult(diffEvents []value.DiffEvent, opts ...Option) []Event {
	events := make([]Event, 0, len(diffEvents))
	for _, de := range diffEvents {
		switch de.Type {
		case value.DiffCreated:
			events = append(events, NewCreateEvent(de.Key, de.NewValue, opts...))
		case value.DiffUpdated:
			events = append(events, NewUpdateEvent(de.Key, de.OldValue, de.NewValue, opts...))
		case value.DiffDeleted:
			events = append(events, NewDeleteEvent(de.Key, de.OldValue, opts...))
		}
	}
	return events
}

// NewDiffEvents computes the differences between two configuration data maps and
// returns the corresponding create, update, and delete events. Keys are processed
// in sorted order for deterministic output.
func NewDiffEvents(old, newData map[string]value.Value, opts ...Option) []Event {
	events := make([]Event, 0, 8)
	for _, k := range value.SortedKeys(old) {
		ov := old[k]
		if nv, exists := newData[k]; exists {
			if !ov.Equal(nv) {
				events = append(events, NewUpdateEvent(k, ov, nv, opts...))
			}
		} else {
			events = append(events, NewDeleteEvent(k, ov, opts...))
		}
	}
	for _, k := range value.SortedKeys(newData) {
		if _, exists := old[k]; !exists {
			events = append(events, NewCreateEvent(k, newData[k], opts...))
		}
	}
	return events
}
