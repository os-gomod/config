package observability_test

import (
	"testing"
	"time"

	"github.com/os-gomod/config/observability"
)

func TestGenerateTraceID(t *testing.T) {
	id := observability.GenerateTraceID()
	if len(id) != 32 {
		t.Errorf("expected 32-char hex string, got %d chars: %q", len(id), id)
	}
	// Must be valid hex.
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character in trace ID: %c", c)
			break
		}
	}
}

func TestGenerateTraceIDUnique(t *testing.T) {
	id1 := observability.GenerateTraceID()
	id2 := observability.GenerateTraceID()
	if id1 == id2 {
		t.Error("two successive TraceIDs should differ")
	}
}

func TestNewTraceContext(t *testing.T) {
	tc := observability.NewTraceContext("reload")
	if tc.TraceID == "" {
		t.Error("TraceID must not be empty")
	}
	if tc.Operation != "reload" {
		t.Errorf("Operation: expected reload, got %q", tc.Operation)
	}
	if tc.StartTime.IsZero() {
		t.Error("StartTime must not be zero")
	}
	if tc.Labels == nil {
		t.Error("Labels map must be initialised")
	}
}

func TestTraceContextElapsed(t *testing.T) {
	tc := observability.NewTraceContext("test")
	elapsed := tc.Elapsed()
	if elapsed < 0 {
		t.Errorf("Elapsed must be non-negative, got %v", elapsed)
	}
	// Small duration — just ensure it's reasonable.
	if elapsed > time.Second {
		t.Errorf("Elapsed too large for immediate call: %v", elapsed)
	}
}
