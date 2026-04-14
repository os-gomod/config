package observability

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// GenerateTraceID returns a cryptographically random 32-character hex string
// suitable for use as a distributed trace identifier.
// On crypto/rand failure (extremely unlikely), it falls back to a nanosecond
// timestamp XOR'd with the process ID to guarantee a non-empty return value.
func GenerateTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%016x%016x",
			uint64(time.Now().UnixNano()),
			uint64(os.Getpid()), //nolint:gosec // fallback only; not security-sensitive
		)
	}
	return hex.EncodeToString(b[:])
}

// TraceContext carries distributed tracing information for a config operation.
type TraceContext struct {
	// TraceID is the unique identifier for this trace.
	TraceID string
	// Operation is the name of the config operation being traced.
	Operation string
	// StartTime records when the operation began.
	StartTime time.Time
	// Labels carries optional key-value pairs for the trace.
	Labels map[string]string
}

// NewTraceContext returns a TraceContext with a freshly generated TraceID.
func NewTraceContext(operation string) *TraceContext {
	return &TraceContext{
		TraceID:   GenerateTraceID(),
		Operation: operation,
		StartTime: time.Now(),
		Labels:    make(map[string]string),
	}
}

// Elapsed returns the duration since StartTime.
func (t *TraceContext) Elapsed() time.Duration {
	return time.Since(t.StartTime)
}
