package loader

import (
	"errors"
	"testing"

	configerrors "github.com/os-gomod/config/errors"
)

func TestErrClosed(t *testing.T) {
	if ErrClosed == nil {
		t.Fatal("ErrClosed should not be nil")
	}
	var ce *configerrors.ConfigError
	if !errors.As(ErrClosed, &ce) {
		t.Error("ErrClosed should be a ConfigError")
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Fatal("ErrNotFound should not be nil")
	}
	var ce *configerrors.ConfigError
	if !errors.As(ErrNotFound, &ce) {
		t.Error("ErrNotFound should be a ConfigError")
	}
}

func TestErrNotSupported(t *testing.T) {
	if ErrNotSupported == nil {
		t.Fatal("ErrNotSupported should not be nil")
	}
}

func TestWrapErrors(t *testing.T) {
	result := WrapErrors(nil, "source", "op")
	if result != nil {
		t.Error("WrapErrors with nil error should return nil")
	}

	result = WrapErrors(errors.New("test"), "my-source", "my-op")
	if result == nil {
		t.Fatal("WrapErrors should return non-nil for non-nil error")
	}
}
