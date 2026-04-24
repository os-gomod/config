package observability

import (
	"testing"
	"time"
)

func TestGenerateTraceID(t *testing.T) {
	id := GenerateTraceID()
	if len(id) != 32 {
		t.Errorf("expected 32-char hex string, got %d chars: %q", len(id), id)
	}

	// Should be valid hex
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex char: %c", c)
			break
		}
	}

	// Should produce different IDs on successive calls
	id2 := GenerateTraceID()
	if id == id2 {
		t.Error("trace IDs should be unique (very unlikely to be equal)")
	}
}

func TestNewTraceContext(t *testing.T) {
	tc := NewTraceContext("test-op")
	if tc == nil {
		t.Fatal("expected non-nil TraceContext")
	}
	if tc.Operation != "test-op" {
		t.Errorf("expected operation 'test-op', got %q", tc.Operation)
	}
	if tc.TraceID == "" {
		t.Error("expected non-empty TraceID")
	}
	if tc.StartTime.IsZero() {
		t.Error("expected non-zero StartTime")
	}
	if tc.Labels == nil {
		t.Error("expected non-nil Labels map")
	}
}

func TestTraceContextElapsed(t *testing.T) {
	tc := NewTraceContext("test")
	time.Sleep(10 * time.Millisecond)
	elapsed := tc.Elapsed()
	if elapsed < 10*time.Millisecond {
		t.Errorf("expected elapsed >= 10ms, got %v", elapsed)
	}
}
