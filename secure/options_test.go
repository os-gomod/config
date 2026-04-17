package secure

import (
	"log/slog"
	"testing"
)

func TestWithLogger(t *testing.T) {
	logger := slog.Default()
	opt := WithLogger(logger)
	o := secureOptions{}
	opt(&o)
	if o.logger != logger {
		t.Fatal("expected logger to be set")
	}
}

func TestWithPriority(t *testing.T) {
	opt := WithPriority(99)
	o := secureOptions{}
	opt(&o)
	if o.priority != 99 {
		t.Fatalf("expected priority 99, got %d", o.priority)
	}
}

func TestDefaultSecureOptions(t *testing.T) {
	o := defaultSecureOptions()
	if o.logger == nil {
		t.Fatal("expected default logger")
	}
	if o.priority != 50 {
		t.Fatalf("expected default priority 50, got %d", o.priority)
	}
}
