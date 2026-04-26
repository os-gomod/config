package observability

import (
	"crypto/rand"
	"encoding/hex"
)

// ---------------------------------------------------------------------------
// Trace ID generation
// ---------------------------------------------------------------------------

// GenerateTraceID generates a unique 128-bit (16-byte) trace ID encoded as
// a 32-character lowercase hex string. Uses crypto/rand for cryptographic
// uniqueness guarantees.
func GenerateTraceID() string {
	b := make([]byte, 16)
	// crypto/rand.Read always returns len(b) bytes and err == nil on supported
	// platforms, but we handle the error defensively.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateSpanID generates a unique 64-bit (8-byte) span ID encoded as a
// 16-character lowercase hex string. Suitable for use as a child span
// identifier in distributed tracing.
func GenerateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
