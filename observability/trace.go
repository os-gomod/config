package observability

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// GenerateTraceID generates a random 128-bit trace ID encoded as a 32-character
// hex string. Falls back to a time+PID-based ID if crypto/rand fails.
func GenerateTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%016x%016x",
			uint64(time.Now().UnixNano()),
			uint64(os.Getpid()),
		)
	}
	return hex.EncodeToString(b[:])
}

// TraceContext carries tracing metadata for a configuration operation.
// It includes a trace ID, operation name, start time, and arbitrary labels.
type TraceContext struct {
	TraceID   string
	Operation string
	StartTime time.Time
	Labels    map[string]string
}

// NewTraceContext creates a new TraceContext with a random trace ID and the
// current time as the start time.
func NewTraceContext(operation string) *TraceContext {
	return &TraceContext{
		TraceID:   GenerateTraceID(),
		Operation: operation,
		StartTime: time.Now(),
		Labels:    make(map[string]string),
	}
}

// Elapsed returns the duration since the trace context was created.
func (t *TraceContext) Elapsed() time.Duration {
	return time.Since(t.StartTime)
}
