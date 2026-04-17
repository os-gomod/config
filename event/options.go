package event

import "github.com/os-gomod/config/core/value"

// Option is a functional option for configuring Event fields during creation.
type Option func(*Event)

// WithTraceID sets the trace ID on the event for distributed tracing correlation.
func WithTraceID(id string) Option { return func(e *Event) { e.TraceID = id } }

// WithError sets the error on the event, used for TypeError events.
func WithError(err error) Option { return func(e *Event) { e.Error = err } }

// WithSource sets the value source on the event (e.g., value.SourceFile, value.SourceEnv).
func WithSource(src value.Source) Option { return func(e *Event) { e.Source = src } }

// WithLabel adds a single key-value label to the event's Labels map.
// Labels can be used for categorizing and filtering events.
func WithLabel(key, val string) Option {
	return func(e *Event) {
		if e.Labels == nil {
			e.Labels = make(map[string]string)
		}
		e.Labels[key] = val
	}
}

// WithLabels merges a map of labels into the event's Labels map.
// Existing labels with the same key are overwritten.
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

// WithMetadata adds a single key-value pair to the event's Metadata map.
// Metadata carries arbitrary information alongside the event.
func WithMetadata(key string, val any) Option {
	return func(e *Event) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]any)
		}
		e.Metadata[key] = val
	}
}
