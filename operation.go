package config

import (
	"context"

	"github.com/os-gomod/config/core/value"
)

// Operation represents a reversible configuration operation. Each operation
// carries a name, an Apply function that performs the change, and an optional
// Rollback function that reverts it.
//
// Operations are used with [ApplyOperations] to execute a sequence of
// configuration changes that can be rolled back atomically if any step fails.
type Operation struct {
	Name string
	// Apply performs the configuration mutation. It receives the context and
	// the [Config] instance, and returns an error if the operation fails.
	Apply func(ctx context.Context, c *Config) error
	// Rollback reverses the effect of Apply. If nil, the operation is
	// considered non-rollbackable.
	Rollback func(ctx context.Context, c *Config) error
}

// ApplyOperation applies a single [Operation] to the given [Config]. It is
// a convenience wrapper around calling op.Apply directly.
func ApplyOperation(ctx context.Context, c *Config, op Operation) error {
	return op.Apply(ctx, c)
}

// ApplyOperations applies a sequence of [Operation] values to the given
// [Config]. If any operation fails, all previously applied operations are
// rolled back in reverse order. The returned error corresponds to the first
// operation that failed.
func ApplyOperations(ctx context.Context, c *Config, ops []Operation) error {
	applied := make([]int, 0, len(ops))
	for i, op := range ops {
		if err := op.Apply(ctx, c); err != nil {
			for j := len(applied) - 1; j >= 0; j-- {
				if rollbackOp := ops[applied[j]]; rollbackOp.Rollback != nil {
					_ = rollbackOp.Rollback(ctx, c)
				}
			}
			return err
		}
		applied = append(applied, i)
	}
	return nil
}

// NewSetOperation creates an [Operation] that sets a key to the given raw
// value. On rollback, the previous value is restored, or the key is deleted
// if it did not exist before.
func NewSetOperation(key string, raw any) Operation {
	var oldVal value.Value
	var existed bool
	return Operation{
		Name: "set:" + key,
		Apply: func(ctx context.Context, c *Config) error {
			if v, ok := c.Get(key); ok {
				oldVal = v
				existed = true
			}
			return c.Set(ctx, key, raw)
		},
		Rollback: func(ctx context.Context, c *Config) error {
			if !existed {
				return c.Delete(ctx, key)
			}
			return c.Set(ctx, key, oldVal.Raw())
		},
	}
}

// NewDeleteOperation creates an [Operation] that deletes a key. On rollback,
// the previous value is restored if the key existed before deletion.
func NewDeleteOperation(key string) Operation {
	var oldVal value.Value
	var existed bool
	return Operation{
		Name: "delete:" + key,
		Apply: func(ctx context.Context, c *Config) error {
			if v, ok := c.Get(key); ok {
				oldVal = v
				existed = true
			}
			return c.Delete(ctx, key)
		},
		Rollback: func(ctx context.Context, c *Config) error {
			if existed {
				return c.Set(ctx, key, oldVal.Raw())
			}
			return nil
		},
	}
}

// NewBindOperation creates an [Operation] that binds the current configuration
// state to the given target struct. Rollback is a no-op because struct binding
// is not reversible.
func NewBindOperation(target any) Operation {
	return Operation{
		Name: "bind",
		Apply: func(ctx context.Context, c *Config) error {
			return c.Bind(ctx, target)
		},
		Rollback: func(_ context.Context, _ *Config) error {
			return nil
		},
	}
}
