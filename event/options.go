package event

import "github.com/os-gomod/config/core/value"

// Option configures an Event during creation.
type Option func(*Event)

// WithTraceID sets the TraceID field on the Event.
func WithTraceID(id string) Option { return func(e *Event) { e.TraceID = id } }

// WithError sets the Error field on the Event.
func WithError(err error) Option { return func(e *Event) { e.Error = err } }

// WithSource sets the Source field on the Event.
func WithSource(src value.Source) Option { return func(e *Event) { e.Source = src } }

// WithLabel adds a single label key-value pair to the Event.
func WithLabel(key, val string) Option {
	return func(e *Event) {
		if e.Labels == nil {
			e.Labels = make(map[string]string)
		}
		e.Labels[key] = val
	}
}

// WithLabels merges the given labels into the Event's Labels map.
func WithLabels(labels map[string]string) Option {
	return func(e *Event) {
		if e.Labels == nil {
			e.Labels = make(map[string]string, len(labels))
		}
		for k, v := range labels {
			e.Labels[k] = v
		}
	}
}

// WithMetadata adds a single metadata key-value pair to the Event.
func WithMetadata(key string, val any) Option {
	return func(e *Event) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]any)
		}
		e.Metadata[key] = val
	}
}
