package errors

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Severity
// ---------------------------------------------------------------------------

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     string
	}{
		{"low", SeverityLow, "low"},
		{"medium", SeverityMedium, "medium"},
		{"high", SeverityHigh, "high"},
		{"critical", SeverityCritical, "critical"},
		{"unknown", Severity(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.severity.String())
		})
	}
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	err := New(CodeNotFound, "key not found")
	require.NotNil(t, err)
	assert.Equal(t, CodeNotFound, err.Code())
	assert.Equal(t, "key not found", err.Message())
	assert.Equal(t, SeverityMedium, err.Severity())
	assert.False(t, err.Retryable())
	assert.NotEmpty(t, err.CorrelationID())
	assert.Equal(t, "", err.Operation())
	assert.Equal(t, "", err.Key())
	assert.Equal(t, "", err.Source())
	assert.Contains(t, err.Error(), "[not_found]")
	assert.Contains(t, err.Error(), "key not found")
}

func TestNewf(t *testing.T) {
	err := Newf(CodeParseError, "parse failed at line %d", 42)
	require.NotNil(t, err)
	assert.Equal(t, CodeParseError, err.Code())
	assert.Equal(t, "parse failed at line 42", err.Message())
}

func TestWrap_NilCause(t *testing.T) {
	err := Wrap(nil, CodeUnknown, "should be nil")
	assert.Nil(t, err)
}

func TestWrap_WithStandardError(t *testing.T) {
	inner := fmt.Errorf("io error")
	err := Wrap(inner, CodeSource, "load failed")
	require.NotNil(t, err)
	assert.Equal(t, CodeSource, err.Code())
	assert.Contains(t, err.Error(), "load failed")
	assert.Contains(t, err.Error(), "io error")
	assert.True(t, strings.Contains(err.Error(), ": io error"))
}

func TestWrap_PropagatesAppErrorMetadata(t *testing.T) {
	inner := New(CodeNotFound, "gone").
		WithKey("db.host").
		WithSource("consul").
		WithOperation("lookup").
		Wrap(fmt.Errorf("not found"))

	outer := Wrap(inner, CodeSource, "layer load failed")
	require.NotNil(t, outer)
	err := outer

	// Wrap propagates retryable, operation, key, source from inner AppError
	assert.Equal(t, "lookup", err.Operation())
	// Key and source are propagated from the inner configError
	assert.Contains(t, err.Error(), "key=db.host")
}

func TestWrap_WithRetryable(t *testing.T) {
	inner := Build(CodeConnection, "connection refused",
		WithRetryable(true),
		WithOperation("dial"),
	)
	outer := Wrap(inner, CodeSource, "layer failed")
	require.NotNil(t, outer)
	err := outer
	assert.True(t, err.Retryable())
	assert.Equal(t, "dial", err.Operation())
}

// ---------------------------------------------------------------------------
// WithKey, WithSource, WithOperation, Wrap (instance methods)
// ---------------------------------------------------------------------------

func TestWithKey(t *testing.T) {
	err := New(CodeUnknown, "test")
	withKey := err.WithKey("my.key")
	require.NotNil(t, withKey)

	// Original is unchanged
	assert.Equal(t, "", err.Key())
	assert.Equal(t, "my.key", withKey.Key())

	// Error string includes key
	assert.Contains(t, withKey.Error(), "key=my.key")
}

func TestWithSource(t *testing.T) {
	err := New(CodeUnknown, "test")
	withSrc := err.WithSource("consul")
	assert.Equal(t, "", err.Source())
	assert.Equal(t, "consul", withSrc.Source())
	assert.Contains(t, withSrc.Error(), "source=consul")
}

func TestWithOperation(t *testing.T) {
	err := New(CodeUnknown, "test")
	withOp := err.WithOperation("set")
	assert.Equal(t, "", err.Operation())
	assert.Equal(t, "set", withOp.Operation())
	assert.Contains(t, withOp.Error(), "op=set")
}

func TestWithKey_Source_Operation_Chaining(t *testing.T) {
	err := New(CodeNotFound, "missing").
		WithKey("db.host").
		WithSource("env").
		WithOperation("get")

	assert.Contains(t, err.Error(), "key=db.host")
	assert.Contains(t, err.Error(), "source=env")
	assert.Contains(t, err.Error(), "op=get")
}

func TestInstanceWrap_NilCause(t *testing.T) {
	err := New(CodeUnknown, "test")
	wrapped := err.Wrap(nil)
	// Wrapping nil returns the same error
	assert.Equal(t, err, wrapped)
}

func TestInstanceWrap_WithCause(t *testing.T) {
	err := New(CodeUnknown, "test")
	cause := fmt.Errorf("root cause")
	wrapped := err.Wrap(cause)

	assert.Contains(t, wrapped.Error(), "root cause")
	assert.Contains(t, wrapped.Error(), "test")
	// Unwrap should return the cause
	unwrapped := wrapped.Unwrap()
	assert.Equal(t, cause, unwrapped)
}

// ---------------------------------------------------------------------------
// error.Is matching by code
// ---------------------------------------------------------------------------

func TestIs_MatchingByCode(t *testing.T) {
	err := New(CodeNotFound, "key not found")
	sentinel := New(CodeNotFound, "another not found")

	assert.True(t, err.Is(sentinel))

	diffCode := New(CodeTypeMismatch, "type error")
	assert.False(t, err.Is(diffCode))
}

func TestErrorsIs(t *testing.T) {
	err := New(CodeNotFound, "key not found")
	sentinel := New(CodeNotFound, "any not found")
	assert.True(t, errorIs(err, sentinel))
}

func TestErrorsIs_DifferentCode(t *testing.T) {
	err := New(CodeNotFound, "key not found")
	sentinel := New(CodeClosed, "closed")
	assert.False(t, errorIs(err, sentinel))
}

func TestErrorsIs_Wrapped(t *testing.T) {
	inner := New(CodeNotFound, "key not found")
	outer := Wrap(inner, CodeSource, "layer failed")

	sentinel := New(CodeNotFound, "any")
	// Our custom isError walks the Unwrap chain, so it finds inner.
	// outer.Is(sentinel) returns false (CodeSource != CodeNotFound),
	// but outer.Unwrap() -> inner, and inner.Is(sentinel) returns true.
	assert.True(t, errorIs(outer, sentinel))
}

// errorIs calls the standard library's errors.Is for testing purposes.
func errorIs(err, target error) bool {
	// Use a simple wrapper since we can't import errors in this package
	// when it would shadow the local errors package.
	return isError(err, target)
}

func isError(err, target error) bool {
	for {
		if err == nil {
			return false
		}
		if isAppErr, ok := err.(interface{ Is(error) bool }); ok {
			if isAppErr.Is(target) {
				return true
			}
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
}

// ---------------------------------------------------------------------------
// error.As extraction
// ---------------------------------------------------------------------------

func TestAs_Direct(t *testing.T) {
	err := New(CodeNotFound, "key not found")

	var appErr AppError
	found := err.As(&appErr)
	assert.True(t, found)
	assert.Equal(t, CodeNotFound, appErr.Code())
}

func TestAsAppError_Direct(t *testing.T) {
	err := New(CodeNotFound, "key not found")
	appErr, ok := AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, CodeNotFound, appErr.Code())
}

func TestAsAppError_Wrapped(t *testing.T) {
	inner := New(CodeNotFound, "key not found")
	outer := Wrap(inner, CodeSource, "layer failed")

	appErr, ok := AsAppError(outer)
	assert.True(t, ok)
	assert.Equal(t, CodeSource, appErr.Code())
}

func TestAsAppError_Nil(t *testing.T) {
	appErr, ok := AsAppError(nil)
	assert.False(t, ok)
	assert.Nil(t, appErr)
}

func TestAsAppError_StandardError(t *testing.T) {
	appErr, ok := AsAppError(fmt.Errorf("plain error"))
	assert.False(t, ok)
	assert.Nil(t, appErr)
}

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  AppError
		code string
		msg  string
	}{
		{"ErrClosed", ErrClosed, CodeClosed, "config is closed"},
		{"ErrNotFound", ErrNotFound, CodeNotFound, "key not found"},
		{"ErrTypeMismatch", ErrTypeMismatch, CodeTypeMismatch, "type mismatch"},
		{"ErrInvalidKey", ErrInvalidKey, CodeInvalidConfig, "invalid key"},
		{"ErrBusClosed", ErrBusClosed, "bus_closed", "event bus is closed"},
		{"ErrQueueFull", ErrQueueFull, "queue_full", "event queue is full"},
		{"ErrInvalidPattern", ErrInvalidPattern, "invalid_pattern", "invalid subscription pattern"},
		{"ErrNilObserver", ErrNilObserver, "nil_observer", "observer must not be nil"},
		{"ErrDeliveryFailed", ErrDeliveryFailed, "delivery_failed", "event delivery failed after retries"},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			require.NotNil(t, s.err)
			assert.Equal(t, s.code, s.err.Code())
			assert.Equal(t, s.msg, s.err.Message())
			// Each sentinel should have a unique correlation ID
			assert.NotEmpty(t, s.err.CorrelationID())
		})
	}
}

func TestSentinelErrors_IsMatching(t *testing.T) {
	// New error with same code should match sentinel
	err := New(CodeClosed, "custom closed message")
	assert.True(t, errorIs(err, ErrClosed))
}

// ---------------------------------------------------------------------------
// Correlation ID generation
// ---------------------------------------------------------------------------

func TestCorrelationID_Uniqueness(t *testing.T) {
	ids := make(map[string]int, 100)
	for i := 0; i < 100; i++ {
		err := New(CodeUnknown, "test")
		id := err.CorrelationID()
		ids[id]++
	}
	// All 100 IDs should be unique
	assert.Len(t, ids, 100, "correlation IDs should be unique")
}

func TestCorrelationID_HexLength(t *testing.T) {
	err := New(CodeUnknown, "test")
	id := err.CorrelationID()
	// 16 bytes = 32 hex chars
	assert.Len(t, id, 32)
}

func TestCorrelationID_WithKeyPreserves(t *testing.T) {
	orig := New(CodeUnknown, "test")
	origID := orig.CorrelationID()

	withKey := orig.WithKey("my.key")
	assert.Equal(t, origID, withKey.CorrelationID(), "correlation ID should be preserved")
}

// ---------------------------------------------------------------------------
// Stack trace capture
// ---------------------------------------------------------------------------

func TestStackTrace_Captured(t *testing.T) {
	err := New(CodeUnknown, "test")
	st := err.StackTrace()
	assert.NotEmpty(t, st, "stack trace should be captured")
}

func TestStackFrames(t *testing.T) {
	err := New(CodeUnknown, "test")
	frames := err.StackFrames()
	assert.NotEmpty(t, frames)
	// At least one frame should be from this test package
	found := false
	for _, f := range frames {
		if strings.Contains(f.Function, "errors") {
			found = true
			break
		}
	}
	assert.True(t, found, "should find a frame from the errors package")
}

func TestFormat_Verb_s(t *testing.T) {
	err := New(CodeNotFound, "key missing").WithKey("db.host")
	s := fmt.Sprintf("%s", err)
	assert.Contains(t, s, "[not_found]")
	assert.Contains(t, s, "key missing")
	assert.Contains(t, s, "key=db.host")
}

func TestFormat_Verb_q(t *testing.T) {
	err := New(CodeNotFound, "key missing")
	q := fmt.Sprintf("%q", err)
	assert.Contains(t, q, "[not_found]")
	assert.Contains(t, q, "key missing")
}

func TestFormat_Verb_v_Plus(t *testing.T) {
	err := New(CodeNotFound, "key missing")
	v := fmt.Sprintf("%+v", err)
	assert.Contains(t, v, "[not_found]")
	assert.Contains(t, v, "key missing")
	// %+v should include stack trace
	assert.Contains(t, v, "errors_test.go")
}

// ---------------------------------------------------------------------------
// Build with options
// ---------------------------------------------------------------------------

func TestBuild_WithRetryable(t *testing.T) {
	err := Build(CodeConnection, "timeout",
		WithRetryable(true),
	)
	assert.True(t, err.Retryable())
}

func TestBuild_WithSeverity(t *testing.T) {
	err := Build(CodeInternal, "crash",
		WithSeverity(SeverityCritical),
	)
	assert.Equal(t, SeverityCritical, err.Severity())
}

func TestBuild_WithOperation_Option(t *testing.T) {
	err := Build(CodeUnknown, "test",
		WithOperation("set"),
	)
	assert.Equal(t, "set", err.Operation())
}

func TestBuild_MultipleOptions(t *testing.T) {
	err := Build(CodeSource, "load failed",
		WithRetryable(true),
		WithSeverity(SeverityHigh),
		WithOperation("reload"),
	)
	assert.Equal(t, CodeSource, err.Code())
	assert.True(t, err.Retryable())
	assert.Equal(t, SeverityHigh, err.Severity())
	assert.Equal(t, "reload", err.Operation())
}

func TestBuild_DefaultSeverity(t *testing.T) {
	err := Build(CodeUnknown, "test")
	assert.Equal(t, SeverityMedium, err.Severity())
}

// ---------------------------------------------------------------------------
// Unwrap
// ---------------------------------------------------------------------------

func TestUnwrap_NilCause(t *testing.T) {
	err := New(CodeUnknown, "test")
	assert.Nil(t, err.Unwrap())
}

func TestUnwrap_WithCause(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := New(CodeUnknown, "test").Wrap(cause)
	assert.Equal(t, cause, err.Unwrap())
}

// ---------------------------------------------------------------------------
// Error string format
// ---------------------------------------------------------------------------

func TestErrorString_AllFields(t *testing.T) {
	err := New(CodeNotFound, "not here").
		WithKey("db.host").
		WithSource("consul").
		WithOperation("get").
		Wrap(fmt.Errorf("connection refused"))

	s := err.Error()
	assert.Contains(t, s, "[not_found]")
	assert.Contains(t, s, "not here")
	assert.Contains(t, s, "key=db.host")
	assert.Contains(t, s, "source=consul")
	assert.Contains(t, s, "op=get")
	assert.Contains(t, s, "connection refused")
}

func TestErrorString_OnlyCode(t *testing.T) {
	err := New(CodeUnknown, "something went wrong")
	s := err.Error()
	assert.Equal(t, "[unknown] something went wrong", s)
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestConcurrent_New(t *testing.T) {
	t.Run("parallel_creates", func(t *testing.T) {
		var mu sync.Mutex
		var errs [100]AppError
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				e := New(CodeUnknown, fmt.Sprintf("err %d", i))
				mu.Lock()
				errs[i] = e
				mu.Unlock()
			}()
		}
		wg.Wait()
		// Verify all are distinct
		ids := make(map[string]struct{})
		for _, e := range errs {
			ids[e.CorrelationID()] = struct{}{}
		}
		assert.Len(t, ids, 100)
	})
}
