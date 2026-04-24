package featureflag

import (
	"context"
	"fmt"
	"testing"

	"github.com/os-gomod/config/core/value"
)

func generateFlagData(n int) map[string]any {
	data := make(map[string]any, n)
	for i := 0; i < n; i++ {
		data[fmt.Sprintf("flag.%d", i)] = true
	}
	return data
}

// BenchmarkIsEnabled_Boolean benchmarks boolean flag evaluation across 1 000
// flags.  Each iteration evaluates a different flag by cycling through the
// flag index.
func BenchmarkIsEnabled_Boolean(b *testing.B) {
	data := generateFlagData(1000)
	prov := &mockProvider{data: make(map[string]value.Value)}
	for k, v := range data {
		prov.data[k] = value.NewInMemory(v)
	}
	eng := NewEngine(prov, "flag.")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.IsEnabled(ctx, fmt.Sprintf("%d", i%1000))
	}
}

// BenchmarkEvaluate_Percentage benchmarks percentage-based rollout evaluation
// for 50 % rollout.  Each iteration uses a different user identifier so the
// hash computation varies.
func BenchmarkEvaluate_Percentage(b *testing.B) {
	prov := &mockProvider{data: map[string]value.Value{
		"flag.rollout": value.NewInMemory(50),
	}}
	eng := NewEngine(prov, "flag.")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evalCtx := &EvalContext{Identifier: fmt.Sprintf("user-%d", i)}
		eng.IsEnabledFor(ctx, "rollout", evalCtx)
	}
}

// BenchmarkEvaluate_Variant benchmarks variant (A/B test) evaluation across
// a four-variant experiment.  Each iteration uses a different user identifier.
func BenchmarkEvaluate_Variant(b *testing.B) {
	prov := &mockProvider{data: map[string]value.Value{
		"flag.experiment": value.NewInMemory("control,treatment_a,treatment_b,treatment_c"),
	}}
	eng := NewEngine(prov, "flag.")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evalCtx := &EvalContext{Identifier: fmt.Sprintf("user-%d", i)}
		eng.Evaluate(ctx, "experiment", evalCtx)
	}
}

// BenchmarkIsEnabled_CacheFriendly benchmarks repeated evaluation of the
// SAME boolean flag (all iterations hit flag.0).  This measures the
// pure overhead of map lookup and type detection.
func BenchmarkIsEnabled_CacheFriendly(b *testing.B) {
	prov := newMockProvider(map[string]any{
		"flag.cached": true,
	})
	eng := NewEngine(prov, "flag.")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.IsEnabled(ctx, "cached")
	}
}

// BenchmarkEvaluate_Missing benchmarks evaluation of a non-existent flag.
// This exercises the fast-path return when Get returns (zero, false).
func BenchmarkEvaluate_Missing(b *testing.B) {
	prov := newMockProvider(map[string]any{})
	eng := NewEngine(prov, "flag.")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.Evaluate(ctx, "nonexistent", nil)
	}
}
