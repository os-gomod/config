package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/event"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// noopMiddleware is a pass-through middleware that adds zero overhead
// beyond the function call itself.
func noopMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			return next(ctx, cmd)
		}
	}
}

// tracingLikeMiddleware simulates the overhead of a real tracing middleware:
// creates a span (simulated via time.Now), records attributes, and sets status.
func tracingLikeMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			// Simulate span creation and attribute recording.
			start := time.Now()
			_ = fmt.Sprintf("config.%s", cmd.Operation)
			_ = fmt.Sprintf("key=%s cmd=%s", cmd.Key, cmd.Name)

			result, err := next(ctx, cmd)

			_ = time.Since(start)
			if err != nil {
				_ = fmt.Sprintf("error: %v", err)
			}
			return result, err
		}
	}
}

// metricsLikeMiddleware simulates the overhead of a metrics middleware:
// times the operation and records duration.
func metricsLikeMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			start := time.Now()
			result, err := next(ctx, cmd)
			_ = time.Since(start)
			_ = cmd.Operation
			return result, err
		}
	}
}

// loggingLikeMiddleware simulates the overhead of structured logging:
// builds attribute slices and calls slog-like formatting.
func loggingLikeMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			attrs := []any{
				"operation", cmd.Operation,
				"command", cmd.Name,
				"key", cmd.Key,
			}
			_ = fmt.Sprintf("pipeline: executing command %v", attrs)

			start := time.Now()
			result, err := next(ctx, cmd)

			attrs = append(attrs, "duration_ms", time.Since(start).Milliseconds())
			if err != nil {
				attrs = append(attrs, "error", err.Error())
			}
			_ = fmt.Sprintf("pipeline: command completed %v", attrs)

			return result, err
		}
	}
}

// recoveryLikeMiddleware simulates the overhead of a recovery middleware
// with defer/recover.
func recoveryLikeMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (result Result, err error) {
			defer func() {
				if r := recover(); r != nil {
					result = Result{}
					err = fmt.Errorf("panic recovered: %v", r)
				}
			}()
			return next(ctx, cmd)
		}
	}
}

// benchCommand returns a simple command for benchmarking.
func benchCommand(name, operation, key string) Command {
	return Command{
		Name:      name,
		Operation: operation,
		Key:       key,
		Execute: func(ctx context.Context) (Result, error) {
			return Result{
				Events: []event.Event{
					event.New(event.TypeUpdate, key,
						event.WithSource("bench"),
					),
				},
			}, nil
		},
	}
}

// benchCommandNoop returns a command that does nothing (no event allocation).
func benchCommandNoop(name, operation, key string) Command {
	return Command{
		Name:      name,
		Operation: operation,
		Key:       key,
		Execute: func(ctx context.Context) (Result, error) {
			return Result{}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPipelineRun — base pipeline with no middleware
// ---------------------------------------------------------------------------

func BenchmarkPipelineRun(b *testing.B) {
	b.ReportAllocs()

	p := New()
	ctx := context.Background()

	b.Run("noop_execute", func(b *testing.B) {
		cmd := benchCommandNoop("bench", "get", "test.key")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = p.Run(ctx, cmd)
		}
	})

	b.Run("with_event", func(b *testing.B) {
		cmd := benchCommand("bench", "set", "test.key")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = p.Run(ctx, cmd)
		}
	})
}

// ---------------------------------------------------------------------------
// BenchmarkPipelineRunWithMiddleware — pipeline with varying middleware stacks
// ---------------------------------------------------------------------------

func BenchmarkPipelineRunWithMiddleware(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()

	cases := []struct {
		name string
		mws  []Middleware
	}{
		{
			name: "1_noop",
			mws:  []Middleware{noopMiddleware()},
		},
		{
			name: "3_noop",
			mws: []Middleware{
				noopMiddleware(),
				noopMiddleware(),
				noopMiddleware(),
			},
		},
		{
			name: "5_noop",
			mws: []Middleware{
				noopMiddleware(),
				noopMiddleware(),
				noopMiddleware(),
				noopMiddleware(),
				noopMiddleware(),
			},
		},
		{
			name: "realistic_4_mw",
			mws: []Middleware{
				recoveryLikeMiddleware(),
				correlationIDLikeMiddleware(),
				tracingLikeMiddleware(),
				metricsLikeMiddleware(),
			},
		},
		{
			name: "full_stack_5_mw",
			mws: []Middleware{
				recoveryLikeMiddleware(),
				correlationIDLikeMiddleware(),
				loggingLikeMiddleware(),
				tracingLikeMiddleware(),
				metricsLikeMiddleware(),
			},
		},
		{
			name: "full_stack_10_mw",
			mws: []Middleware{
				recoveryLikeMiddleware(),
				correlationIDLikeMiddleware(),
				noopMiddleware(),
				loggingLikeMiddleware(),
				tracingLikeMiddleware(),
				metricsLikeMiddleware(),
				noopMiddleware(),
				noopMiddleware(),
				noopMiddleware(),
				noopMiddleware(),
			},
		},
	}

	for _, tc := range cases {
		p := New(WithMiddleware(tc.mws...))

		b.Run("noop_execute/"+tc.name, func(b *testing.B) {
			cmd := benchCommandNoop("bench", "set", "test.key")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = p.Run(ctx, cmd)
			}
		})

		b.Run("with_event/"+tc.name, func(b *testing.B) {
			cmd := benchCommand("bench", "set", "test.key")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = p.Run(ctx, cmd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPipelineBuildHandler — measures middleware chain rebuild overhead
// ---------------------------------------------------------------------------

func BenchmarkPipelineBuildHandler(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name    string
		mwCount int
	}{
		{"1_middleware", 1},
		{"5_middleware", 5},
		{"10_middleware", 10},
	}

	for _, tc := range cases {
		p := New()
		for i := 0; i < tc.mwCount; i++ {
			p.Use(noopMiddleware())
		}

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = p.buildHandler()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPipelineParallel — concurrent pipeline execution
// ---------------------------------------------------------------------------

func BenchmarkPipelineParallel(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name string
		mws  []Middleware
	}{
		{
			name: "no_middleware",
			mws:  nil,
		},
		{
			name: "realistic_4_mw",
			mws: []Middleware{
				recoveryLikeMiddleware(),
				correlationIDLikeMiddleware(),
				tracingLikeMiddleware(),
				metricsLikeMiddleware(),
			},
		},
	}

	for _, tc := range cases {
		p := New(WithMiddleware(tc.mws...))
		cmd := benchCommandNoop("bench", "set", "test.key")

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				ctx := context.Background()
				for pb.Next() {
					_, _ = p.Run(ctx, cmd)
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers (pipeline-local)
// ---------------------------------------------------------------------------

// correlationIDLikeMiddleware simulates the correlation ID middleware overhead.
func correlationIDLikeMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, cmd Command) (Result, error) {
			if CorrelationIDFromContext(ctx) == "" {
				id := fmt.Sprintf("%s-%d", cmd.Operation, time.Now().UnixNano())
				ctx = ContextWithCorrelationID(ctx, id)
				_ = id
			}
			return next(ctx, cmd)
		}
	}
}
