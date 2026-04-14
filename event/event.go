// Package event defines the event types, bus, and lifecycle hook types used
// throughout the config framework.
package event

import (
	"time"

	"github.com/os-gomod/config/core/value"
)

// Type represents the kind of config event that occurred.
type Type uint8

const (
	// TypeCreate indicates a new config key was added.
	TypeCreate Type = iota
	// TypeUpdate indicates an existing config key's value changed.
	TypeUpdate
	// TypeDelete indicates a config key was removed.
	TypeDelete
	// TypeReload indicates a full configuration reload occurred.
	TypeReload
	// TypeError indicates an error occurred during a config operation.
	TypeError
	// TypeWatch indicates a watch event was triggered by a source change.
	TypeWatch
)

var typeNames = [...]string{
	TypeCreate: "create",
	TypeUpdate: "update",
	TypeDelete: "delete",
	TypeReload: "reload",
	TypeError:  "error",
	TypeWatch:  "watch",
}

// String returns the human-readable name of the event Type.
func (t Type) String() string {
	if int(t) < len(typeNames) {
		return typeNames[t]
	}
	return "unknown"
}

// Event represents a single configuration change or lifecycle event.
type Event struct {
	// Type is the kind of event.
	Type Type
	// Key is the config key affected by this event.
	Key string
	// OldValue is the value before the change (zero for create events).
	OldValue value.Value
	// NewValue is the value after the change (zero for delete events).
	NewValue value.Value
	// Timestamp is the time at which the event was created.
	Timestamp time.Time
	// Source identifies the origin of the event.
	Source value.Source
	// TraceID is an optional distributed tracing identifier.
	TraceID string
	// Labels are optional key-value pairs for event categorisation.
	Labels map[string]string
	// Metadata carries arbitrary structured data associated with the event.
	Metadata map[string]any
	// Error holds an error for TypeError events.
	Error error
}

// New creates an Event of the given type for the given key, applying any
// supplied options.
func New(typ Type, key string, opts ...Option) Event {
	e := Event{Type: typ, Key: key, Timestamp: time.Now()}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// newValueEvent creates an Event with old and new value fields set.
func newValueEvent(typ Type, key string, oldVal, newVal value.Value, opts ...Option) Event {
	evt := New(typ, key, opts...)
	evt.OldValue = oldVal
	evt.NewValue = newVal
	return evt
}

// NewCreateEvent creates a TypeCreate event for the given key and value.
func NewCreateEvent(key string, newVal value.Value, opts ...Option) Event {
	return newValueEvent(TypeCreate, key, value.Value{}, newVal, opts...)
}

// NewUpdateEvent creates a TypeUpdate event for the given key with old and new values.
func NewUpdateEvent(key string, oldVal, newVal value.Value, opts ...Option) Event {
	return newValueEvent(TypeUpdate, key, oldVal, newVal, opts...)
}

// NewDeleteEvent creates a TypeDelete event for the given key and its former value.
func NewDeleteEvent(key string, oldVal value.Value, opts ...Option) Event {
	return newValueEvent(TypeDelete, key, oldVal, value.Value{}, opts...)
}

// DiffEventsFromResult converts a slice of value.DiffEvent into Event values,
// applying the given options to each event.
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

// NewDiffEvents computes the diff between old and newData maps and returns
// a slice of Events (create, update, or delete) for each change.
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
