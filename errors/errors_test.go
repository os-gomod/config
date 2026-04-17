package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCode_String(t *testing.T) {
	tests := []struct {
		code Code
		want string
	}{
		{CodeUnknown, "unknown"},
		{CodeNotFound, "not_found"},
		{CodeTypeMismatch, "type_mismatch"},
		{CodeInvalidFormat, "invalid_format"},
		{CodeParseError, "parse_error"},
		{CodeValidation, "validation_error"},
		{CodeSource, "source_error"},
		{CodeCrypto, "crypto_error"},
		{CodeWatch, "watch_error"},
		{CodeBind, "bind_error"},
		{CodeContextCanceled, "context_canceled"},
		{CodeTimeout, "timeout"},
		{CodeInvalidConfig, "invalid_config"},
		{CodeAlreadyExists, "already_exists"},
		{CodePermissionDenied, "permission_denied"},
		{CodeNotImplemented, "not_implemented"},
		{CodeConnection, "connection_error"},
		{CodeConflict, "conflict"},
		{CodeInternal, "internal_error"},
		{CodeClosed, "closed"},
		{Code(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.code.String(); got != tt.want {
				t.Errorf("Code.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("creates error with code and message", func(t *testing.T) {
		e := New(CodeNotFound, "key not found")
		if e.Code != CodeNotFound {
			t.Fatalf("expected CodeNotFound, got %d", e.Code)
		}
		if e.Message != "key not found" {
			t.Fatalf("expected 'key not found', got %q", e.Message)
		}
		if e.Stack() == "" {
			t.Fatal("expected non-empty stack")
		}
	})
}

func TestNewf(t *testing.T) {
	e := Newf(CodeValidation, "field %s is required", "name")
	if e.Message != "field name is required" {
		t.Fatalf("unexpected message: %q", e.Message)
	}
	if e.Code != CodeValidation {
		t.Fatalf("expected CodeValidation, got %d", e.Code)
	}
}

func TestWrap(t *testing.T) {
	t.Run("wraps non-nil error", func(t *testing.T) {
		cause := fmt.Errorf("db connection failed")
		e := Wrap(cause, CodeConnection, "database error")
		if e == nil {
			t.Fatal("expected non-nil error")
		}
		if e.Code != CodeConnection {
			t.Fatalf("expected CodeConnection, got %d", e.Code)
		}
		if e.Cause != cause {
			t.Fatal("expected cause to be set")
		}
	})

	t.Run("wraps nil returns nil", func(t *testing.T) {
		e := Wrap(nil, CodeUnknown, "test")
		if e != nil {
			t.Fatal("expected nil for nil cause")
		}
	})
}

func TestConfigError_WithKey(t *testing.T) {
	e := New(CodeNotFound, "not found").WithKey("app.port")
	if e.Key != "app.port" {
		t.Fatalf("expected key 'app.port', got %q", e.Key)
	}
	// Original should not be modified
	original := New(CodeNotFound, "not found")
	_ = original.WithKey("other")
	if original.Key != "" {
		t.Fatal("WithKey should not modify original")
	}
}

func TestConfigError_WithSource(t *testing.T) {
	e := New(CodeSource, "error").WithSource("consul")
	if e.Source != "consul" {
		t.Fatalf("expected source 'consul', got %q", e.Source)
	}
}

func TestConfigError_WithOperation(t *testing.T) {
	e := New(CodeSource, "error").WithOperation("load")
	if e.Operation != "load" {
		t.Fatalf("expected operation 'load', got %q", e.Operation)
	}
}

func TestConfigError_WithPath(t *testing.T) {
	e := New(CodeInvalidConfig, "error").WithPath("config.yaml")
	if e.Path != "config.yaml" {
		t.Fatalf("expected path 'config.yaml', got %q", e.Path)
	}
}

func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ConfigError
		want string
	}{
		{
			name: "message only",
			err:  New(CodeUnknown, "something went wrong"),
			want: "something went wrong",
		},
		{
			name: "with operation",
			err:  New(CodeSource, "failed").WithOperation("load"),
			want: "load: failed",
		},
		{
			name: "with key",
			err:  New(CodeNotFound, "not found").WithKey("app.port"),
			want: "not found [key=app.port]",
		},
		{
			name: "with source",
			err:  New(CodeSource, "error").WithSource("consul"),
			want: "error [source=consul]",
		},
		{
			name: "with path",
			err:  New(CodeInvalidConfig, "error").WithPath("/etc/config.yaml"),
			want: "error [path=/etc/config.yaml]",
		},
		{
			name: "with cause",
			err:  Wrap(fmt.Errorf("conn refused"), CodeConnection, "connection error"),
			want: "connection error: conn refused",
		},
		{
			name: "with all fields",
			err:  Wrap(fmt.Errorf("timeout"), CodeTimeout, "request failed").WithOperation("load").WithKey("k").WithSource("src").WithPath("/p"),
			want: "load: request failed [key=k] [source=src] [path=/p]: timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	e := Wrap(cause, CodeUnknown, "wrapper")
	if !errors.Is(e, cause) {
		t.Fatal("expected errors.Is to match cause")
	}
}

func TestConfigError_Is(t *testing.T) {
	t.Run("same code matches", func(t *testing.T) {
		e1 := New(CodeNotFound, "msg1")
		e2 := New(CodeNotFound, "msg2")
		if !errors.Is(e1, e2) {
			t.Fatal("errors.Is should match same code")
		}
	})

	t.Run("different code does not match", func(t *testing.T) {
		e1 := New(CodeNotFound, "msg1")
		e2 := New(CodeValidation, "msg2")
		if errors.Is(e1, e2) {
			t.Fatal("errors.Is should not match different code")
		}
	})
}

func TestConfigError_Stack(t *testing.T) {
	e := New(CodeUnknown, "test")
	stack := e.Stack()
	if stack == "" {
		t.Fatal("expected non-empty stack")
	}
	// Stack should contain at least one frame
	if !strings.Contains(stack, "\n") {
		t.Fatal("stack should contain multiple lines")
	}
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  *ConfigError
		code Code
		msg  string
	}{
		{"ErrClosed", ErrClosed, CodeClosed, "config is closed"},
		{"ErrNotFound", ErrNotFound, CodeNotFound, "key not found"},
		{"ErrTypeMismatch", ErrTypeMismatch, CodeTypeMismatch, "type mismatch"},
		{"ErrInvalidKey", ErrInvalidKey, CodeInvalidConfig, "invalid key"},
		{"ErrNotImplemented", ErrNotImplemented, CodeNotImplemented, "not implemented"},
		{"ErrPermission", ErrPermission, CodePermissionDenied, "permission denied"},
		{"ErrDecryptFailed", ErrDecryptFailed, CodeCrypto, "decryption failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("expected code %d, got %d", tt.code, tt.err.Code)
			}
			if tt.err.Message != tt.msg {
				t.Errorf("expected message %q, got %q", tt.msg, tt.err.Message)
			}
		})
	}
}

func TestIsCode(t *testing.T) {
	t.Run("matches correct code", func(t *testing.T) {
		err := New(CodeNotFound, "not found")
		if !IsCode(err, CodeNotFound) {
			t.Fatal("expected IsCode to return true")
		}
	})

	t.Run("does not match different code", func(t *testing.T) {
		err := New(CodeNotFound, "not found")
		if IsCode(err, CodeValidation) {
			t.Fatal("expected IsCode to return false")
		}
	})

	t.Run("handles nil error", func(t *testing.T) {
		if IsCode(nil, CodeUnknown) {
			t.Fatal("expected false for nil error")
		}
	})

	t.Run("handles non-ConfigError", func(t *testing.T) {
		if IsCode(fmt.Errorf("plain"), CodeUnknown) {
			t.Fatal("expected false for non-ConfigError")
		}
	})
}

func TestWrapSource(t *testing.T) {
	t.Run("wraps with source and operation", func(t *testing.T) {
		cause := fmt.Errorf("connection refused")
		err := WrapSource(cause, "consul", "load")
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		var ce *ConfigError
		if !errors.As(err, &ce) {
			t.Fatal("expected ConfigError")
		}
		if ce.Code != CodeSource {
			t.Fatalf("expected CodeSource, got %d", ce.Code)
		}
		if ce.Source != "consul" {
			t.Fatalf("expected source 'consul', got %q", ce.Source)
		}
		if ce.Operation != "load" {
			t.Fatalf("expected operation 'load', got %q", ce.Operation)
		}
	})

	t.Run("nil cause returns nil", func(t *testing.T) {
		err := WrapSource(nil, "src", "op")
		if err != nil {
			t.Fatal("expected nil")
		}
	})
}

func TestConfigError_WithChaining(t *testing.T) {
	e := New(CodeSource, "base").
		WithKey("k").
		WithSource("src").
		WithOperation("op").
		WithPath("/p")
	msg := e.Error()
	if !strings.Contains(msg, "[key=k]") {
		t.Fatal("missing key")
	}
	if !strings.Contains(msg, "[source=src]") {
		t.Fatal("missing source")
	}
	if !strings.Contains(msg, "[path=/p]") {
		t.Fatal("missing path")
	}
	if !strings.Contains(msg, "op: ") {
		t.Fatal("missing operation")
	}
}
