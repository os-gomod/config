package config

import (
	"context"

	"github.com/os-gomod/config/core/value"
)

// Operation represents a named, reversible configuration change that can be
// applied and rolled back atomically. Operations are useful for multi-step
// configuration updates that must either succeed entirely or leave the state
// unchanged.
type Operation struct {
	// Name is a human-readable identifier for this operation.
	Name string
	// Apply is the function that applies the configuration change.
	Apply func(ctx context.Context, c *Config) error
	// Rollback is the function that reverses the configuration change.
	// It is called when a subsequent operation in a batch fails.
	Rollback func(ctx context.Context, c *Config) error
}

// ApplyOperation executes the operation's Apply function.
func ApplyOperation(ctx context.Context, c *Config, op Operation) error {
	return op.Apply(ctx, c)
}

// ApplyOperations executes a batch of operations in order. If any operation
// fails, all previously applied operations are rolled back in reverse order.
func ApplyOperations(ctx context.Context, c *Config, ops []Operation) error {
	var applied []int
	for i, op := range ops {
		if err := op.Apply(ctx, c); err != nil {
			// Roll back in reverse order.
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

// NewSetOperation creates an operation that sets a key.
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

// NewDeleteOperation creates an operation that deletes a key.
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

// NewBindOperation creates an operation that binds config values to a target struct.
func NewBindOperation(target any) Operation {
	return Operation{
		Name: "bind",
		Apply: func(ctx context.Context, c *Config) error {
			return c.Bind(ctx, target)
		},
		Rollback: func(_ context.Context, _ *Config) error {
			// Bind operations have no meaningful rollback.
			return nil
		},
	}
}
