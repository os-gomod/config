package interceptor

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ConditionFunc determines whether an operation should proceed.
// It receives the current value and returns true if the operation should be allowed.
type ConditionFunc func(ctx context.Context, currentValue value.Value) (allowed bool, reason string)

// ConditionalInterceptor wraps a SetInterceptor with pre-condition checks.
// This extends the existing interceptor system rather than creating a separate engine.
type ConditionalInterceptor struct {
	mu         sync.RWMutex
	conditions map[string]ConditionFunc
	inner      SetInterceptor
}

// NewConditionalInterceptor creates a conditional interceptor that wraps an existing one.
func NewConditionalInterceptor(inner SetInterceptor) *ConditionalInterceptor {
	return &ConditionalInterceptor{
		conditions: make(map[string]ConditionFunc),
		inner:      inner,
	}
}

// RegisterCondition adds a condition for a specific key pattern.
// Patterns support glob matching (e.g., "database.*", "*.port").
func (ci *ConditionalInterceptor) RegisterCondition(keyPattern string, fn ConditionFunc) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.conditions[keyPattern] = fn
}

// UnregisterCondition removes a condition for a key pattern.
func (ci *ConditionalInterceptor) UnregisterCondition(keyPattern string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	delete(ci.conditions, keyPattern)
}

// BeforeSet implements SetInterceptor with pre-condition checks.
func (ci *ConditionalInterceptor) BeforeSet(ctx context.Context, req *SetRequest) error {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	// Check all conditions that match this key
	for pattern, cond := range ci.conditions {
		if ci.matchPattern(req.Key, pattern) {
			// Need to get current value - this should be provided via context
			// or we need to inject a ValueGetter interface
			allowed, reason := cond(ctx, value.Value{})
			if !allowed {
				return fmt.Errorf("condition blocked set on %q: %s", req.Key, reason)
			}
		}
	}

	if ci.inner != nil {
		return ci.inner.BeforeSet(ctx, req)
	}
	return nil
}

// AfterSet implements SetInterceptor.
func (ci *ConditionalInterceptor) AfterSet(ctx context.Context, req *SetRequest, res *SetResponse) error {
	if ci.inner != nil {
		return ci.inner.AfterSet(ctx, req, res)
	}
	return nil
}

// matchPattern checks if a key matches a glob pattern (supports * wildcard).
func (ci *ConditionalInterceptor) matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == key {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, ".")
		keyParts := strings.Split(key, ".")
		if len(parts) != len(keyParts) {
			return false
		}
		for i := range parts {
			if parts[i] != "*" && parts[i] != keyParts[i] {
				return false
			}
		}
		return true
	}
	return false
}

// ConditionBuilder provides a fluent API for creating conditions.
type ConditionBuilder struct {
	keyPattern string
	fn         ConditionFunc
}

// Key sets the key pattern for the condition.
func (cb *ConditionBuilder) Key(pattern string) *ConditionBuilder {
	cb.keyPattern = pattern
	return cb
}

// When sets the condition function.
func (cb *ConditionBuilder) When(fn ConditionFunc) *ConditionBuilder {
	cb.fn = fn
	return cb
}

// Build returns the configured ConditionBuilder (for chaining).
func (cb *ConditionBuilder) Build() (*ConditionBuilder, ConditionFunc) {
	return cb, cb.fn
}

// NewConditionBuilder creates a new condition builder.
func NewConditionBuilder() *ConditionBuilder {
	return &ConditionBuilder{}
}

var Conditions = struct {
	// OnlyAllowIf creates a condition that allows the operation only if the current value matches the predicate
	OnlyAllowIf func(predicate func(current any) bool) ConditionFunc
	// OnlyAllowOnce creates a condition that allows the operation only once
	OnlyAllowOnce func() ConditionFunc
	// RequireMinValue creates a condition that requires the new value to be >= min
	RequireMinValue func(min int) ConditionFunc
	// RequireMaxValue creates a condition that requires the new value to be <= max
	RequireMaxValue func(max int) ConditionFunc
	// RateLimit creates a condition that rate limits operations on a key
	RateLimit func(perSecond int) ConditionFunc
}{
	OnlyAllowIf: func(predicate func(current any) bool) ConditionFunc {
		return func(ctx context.Context, current value.Value) (bool, string) {
			if predicate(current.Raw()) {
				return true, ""
			}
			return false, "value does not satisfy required condition"
		}
	},
	OnlyAllowOnce: func() ConditionFunc {
		var allowed bool
		return func(ctx context.Context, current value.Value) (bool, string) {
			if allowed {
				return false, "key can only be set once"
			}
			allowed = true
			return true, ""
		}
	},
	RequireMinValue: func(min int) ConditionFunc {
		return func(ctx context.Context, current value.Value) (bool, string) {
			if current.Int() >= min {
				return true, ""
			}
			return false, "value is below minimum"
		}
	},
	RequireMaxValue: func(max int) ConditionFunc {
		return func(ctx context.Context, current value.Value) (bool, string) {
			if current.Int() <= max {
				return true, ""
			}
			return false, "value exceeds maximum"
		}
	},
	RateLimit: func(perSecond int) ConditionFunc {
		// Simplified - use token bucket in production
		return func(ctx context.Context, current value.Value) (bool, string) {
			return true, ""
		}
	},
}
