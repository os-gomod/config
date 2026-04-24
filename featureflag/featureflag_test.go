package featureflag

import (
	"context"
	"fmt"
	"testing"

	"github.com/os-gomod/config/core/value"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements ConfigProvider for testing.
type mockProvider struct {
	data map[string]value.Value
}

func (m *mockProvider) Get(key string) (value.Value, bool) {
	v, ok := m.data[key]
	return v, ok
}

func newMockProvider(data map[string]any) *mockProvider {
	m := &mockProvider{data: make(map[string]value.Value)}
	for k, v := range data {
		m.data[k] = value.NewInMemory(v)
	}
	return m
}

// ------------------------------------------------------------------
// Evaluate with percentage flags
// ------------------------------------------------------------------
func TestEvaluate_PercentageFlag(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.rollout": 50,
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "rollout", &EvalContext{Identifier: "user-1"})
	assert.Equal(t, FlagTypePercentage, eval.Flag.Type)
	assert.Equal(t, 50, eval.Flag.Percentage)
}

func TestEvaluate_PercentageFlag_Zero(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.zero_pct": 0,
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "zero_pct", &EvalContext{Identifier: "user-1"})
	assert.Equal(t, FlagTypePercentage, eval.Flag.Type)
	assert.Equal(t, 0, eval.Flag.Percentage)
	assert.False(t, eval.Enabled)
}

func TestEvaluate_PercentageFlag_OverHundred(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.over_100": "150",
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "over_100", &EvalContext{Identifier: "user-1"})
	// 150 > 100 gets clamped in evaluatePercentage, not in detectFlagType
	// detectFlagType detects "150" as percentage (Atoi succeeds and is in 0-100 range? no, 150>100)
	// Actually detectFlagType tries string parsing: Atoi("150")=150 but >100, so falls to default boolean
	assert.Equal(t, FlagTypeBoolean, eval.Flag.Type)
}

func TestEvaluate_PercentageFlag_Negative(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.neg_pct": -10,
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "neg_pct", &EvalContext{Identifier: "user-1"})
	assert.Equal(t, 0, eval.Flag.Percentage) // clamped
	assert.False(t, eval.Enabled)
}

// ------------------------------------------------------------------
// Evaluate with variant flags
// ------------------------------------------------------------------
func TestEvaluate_VariantFlag_SingleVariant(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.single_variant": "control",
	})
	eng := NewEngine(prov, "feature.")

	// "control" without commas is detected as boolean (default)
	eval := eng.Evaluate(context.Background(), "single_variant", nil)
	assert.False(t, eval.Enabled) // "control" parsed as boolean: false
}

func TestEvaluate_VariantFlag_Empty(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.empty_variants": "",
	})
	eng := NewEngine(prov, "feature.")

	// Empty string: detectFlagType falls to boolean, evaluateBoolean returns false
	eval := eng.Evaluate(context.Background(), "empty_variants", nil)
	assert.False(t, eval.Enabled)
}

func TestEvaluate_VariantFlag_NoIdentifier(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.no_id_variant": "a,b,c",
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "no_id_variant", nil)
	assert.True(t, eval.Enabled)
	assert.Equal(t, "a", eval.Variant) // defaults to first
	assert.Equal(t, "variant_no_identifier", eval.MatchedRule)
}

func TestEvaluate_VariantFlag_TwoVariants(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.two_variants": "blue,green",
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "two_variants", &EvalContext{Identifier: "user-42"})
	assert.True(t, eval.Enabled)
	assert.Contains(t, []string{"blue", "green"}, eval.Variant)

	// Same identifier should be deterministic
	eval2 := eng.Evaluate(context.Background(), "two_variants", &EvalContext{Identifier: "user-42"})
	assert.Equal(t, eval.Variant, eval2.Variant)
}

// ------------------------------------------------------------------
// Evaluate with unknown flag
// ------------------------------------------------------------------
func TestEvaluate_UnknownFlag(t *testing.T) {
	prov := newMockProvider(map[string]any{})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "unknown", nil)
	assert.False(t, eval.Enabled)
	assert.Equal(t, "not_found", eval.MatchedRule)
	assert.Equal(t, FlagTypeBoolean, eval.Flag.Type)
}

// ------------------------------------------------------------------
// IsEnabledFor with different identifiers
// ------------------------------------------------------------------
func TestIsEnabledFor_DifferentIdentifiers(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.pct": 50,
	})
	eng := NewEngine(prov, "feature.")

	ctx := context.Background()

	// Check with identifier (deterministic)
	pct50 := &EvalContext{Identifier: "user-50"}
	result := eng.IsEnabledFor(ctx, "pct", pct50)
	// The result depends on the hash, just verify it doesn't panic
	_ = result
}

func TestIsEnabledFor_NilContext(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.pct": 50,
	})
	eng := NewEngine(prov, "feature.")

	// Without identifier, percentage flags default to enabled if > 0
	result := eng.IsEnabledFor(context.Background(), "pct", nil)
	assert.True(t, result)
}

// ------------------------------------------------------------------
// EvalContext with attributes
// ------------------------------------------------------------------
func TestEvalContext(t *testing.T) {
	ec := &EvalContext{
		Identifier: "user-123",
		Attributes: map[string]string{
			"country": "US",
			"tier":    "premium",
		},
	}
	assert.Equal(t, "user-123", ec.Identifier)
	assert.Equal(t, "US", ec.Attributes["country"])
	assert.Equal(t, "premium", ec.Attributes["tier"])
}

// ------------------------------------------------------------------
// detectFlagType with various key suffixes
// ------------------------------------------------------------------
func TestDetectFlagType_EnabledSuffix(t *testing.T) {
	v := value.NewInMemory("true")
	ft := detectFlagType(v, "feature.dark_mode.enabled")
	assert.Equal(t, FlagTypeBoolean, ft)
}

func TestDetectFlagType_PctSuffix(t *testing.T) {
	v := value.NewInMemory("50")
	ft := detectFlagType(v, "feature.rollout.pct")
	assert.Equal(t, FlagTypePercentage, ft)
}

func TestDetectFlagType_PercentSuffix(t *testing.T) {
	v := value.NewInMemory("50")
	ft := detectFlagType(v, "feature.rollout.percent")
	assert.Equal(t, FlagTypePercentage, ft)
}

func TestDetectFlagType_PercentageSuffix(t *testing.T) {
	v := value.NewInMemory("50")
	ft := detectFlagType(v, "feature.rollout.percentage")
	assert.Equal(t, FlagTypePercentage, ft)
}

func TestDetectFlagType_VariantsSuffix(t *testing.T) {
	v := value.NewInMemory("a,b,c")
	ft := detectFlagType(v, "feature.experiment.variants")
	assert.Equal(t, FlagTypeVariant, ft)
}

func TestDetectFlagType_VariantSuffix(t *testing.T) {
	v := value.NewInMemory("a,b")
	ft := detectFlagType(v, "feature.test.variant")
	assert.Equal(t, FlagTypeVariant, ft)
}

func TestDetectFlagType_AutoDetectBool(t *testing.T) {
	v := value.NewInMemory(true)
	ft := detectFlagType(v, "feature.something")
	assert.Equal(t, FlagTypeBoolean, ft)
}

func TestDetectFlagType_AutoDetectPercentage(t *testing.T) {
	v := value.NewInMemory(75)
	ft := detectFlagType(v, "feature.something")
	assert.Equal(t, FlagTypePercentage, ft)
}

func TestDetectFlagType_AutoDetectVariant(t *testing.T) {
	v := value.NewInMemory("a,b,c")
	ft := detectFlagType(v, "feature.something")
	assert.Equal(t, FlagTypeVariant, ft)
}

func TestDetectFlagType_Default(t *testing.T) {
	v := value.NewInMemory("some_string")
	ft := detectFlagType(v, "feature.something")
	assert.Equal(t, FlagTypeBoolean, ft) // default
}

// ------------------------------------------------------------------
// splitVariants edge cases
// ------------------------------------------------------------------
func TestSplitVariants_Single(t *testing.T) {
	assert.Equal(t, []string{"a"}, splitVariants("a"))
}

func TestSplitVariants_Multiple(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, splitVariants("a,b,c"))
}

func TestSplitVariants_WithSpaces(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, splitVariants(" a , b , c "))
}

func TestSplitVariants_Empty(t *testing.T) {
	assert.Nil(t, splitVariants(""))
}

func TestSplitVariants_WhitespaceOnly(t *testing.T) {
	assert.Nil(t, splitVariants("   "))
}

func TestSplitVariants_TrailingCommas(t *testing.T) {
	result := splitVariants("a,,b,")
	assert.Equal(t, []string{"a", "b"}, result)
}

// ------------------------------------------------------------------
// hashIdentifier determinism
// ------------------------------------------------------------------
func TestHashIdentifier_Deterministic(t *testing.T) {
	for i := 0; i < 100; i++ {
		h1 := hashIdentifier("user-123", "flag-a")
		h2 := hashIdentifier("user-123", "flag-a")
		assert.Equal(t, h1, h2)
	}
}

func TestHashIdentifier_DifferentInputs(t *testing.T) {
	h1 := hashIdentifier("user-1", "flag-a")
	h2 := hashIdentifier("user-2", "flag-a")
	h3 := hashIdentifier("user-1", "flag-b")
	assert.NotEqual(t, h1, h2, "different identifiers should produce different hashes")
	assert.NotEqual(t, h1, h3, "different flags should produce different hashes")
	assert.NotEqual(t, h2, h3, "different combinations should produce different hashes")
}

func TestHashIdentifier_EmptyStrings(t *testing.T) {
	h := hashIdentifier("", "")
	// Should still produce a valid hash without panicking
	assert.True(t, h >= 0)
}

// ------------------------------------------------------------------
// FlagType String
// ------------------------------------------------------------------
func TestFlagType_String(t *testing.T) {
	assert.Equal(t, "boolean", FlagTypeBoolean.String())
	assert.Equal(t, "percentage", FlagTypePercentage.String())
	assert.Equal(t, "variant", FlagTypeVariant.String())
	assert.Equal(t, "unknown", FlagType(99).String())
}

// ------------------------------------------------------------------
// IsEnabled
// ------------------------------------------------------------------
func TestIsEnabled_BooleanFlag(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.new_ui":    true,
		"feature.old_ui":    false,
		"feature.dark_mode": "true",
		"feature.missing":   nil,
	})
	eng := NewEngine(prov, "feature.")

	assert.True(t, eng.IsEnabled(context.Background(), "new_ui"))
	assert.False(t, eng.IsEnabled(context.Background(), "old_ui"))
	assert.True(t, eng.IsEnabled(context.Background(), "dark_mode"))
	assert.False(t, eng.IsEnabled(context.Background(), "missing"))
	assert.False(t, eng.IsEnabled(context.Background(), "nonexistent"))
}

// ------------------------------------------------------------------
// IsEnabledFor_PercentageFlag
// ------------------------------------------------------------------
func TestIsEnabledFor_PercentageFlag(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.rollout": 50,
	})
	eng := NewEngine(prov, "feature.")

	ctx := context.Background()
	evalCtx := &EvalContext{Identifier: "user-123"}

	result1 := eng.IsEnabledFor(ctx, "rollout", evalCtx)
	result2 := eng.IsEnabledFor(ctx, "rollout", evalCtx)
	assert.Equal(t, result1, result2)

	results := make(map[bool]int)
	for i := 0; i < 100; i++ {
		uid := EvalContext{Identifier: fmt.Sprintf("user-%d", i)}
		results[eng.IsEnabledFor(ctx, "rollout", &uid)]++
	}
	assert.True(t, results[true] > 20)
	assert.True(t, results[false] > 20)
}

// ------------------------------------------------------------------
// Evaluate_VariantFlag
// ------------------------------------------------------------------
func TestEvaluate_VariantFlag(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.experiment": "control,treatment_a,treatment_b",
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "experiment", &EvalContext{Identifier: "user-42"})
	assert.True(t, eval.Enabled)
	assert.Contains(t, []string{"control", "treatment_a", "treatment_b"}, eval.Variant)

	eval2 := eng.Evaluate(context.Background(), "experiment", &EvalContext{Identifier: "user-42"})
	assert.Equal(t, eval.Variant, eval2.Variant)
}

// ------------------------------------------------------------------
// Evaluate_NotFound
// ------------------------------------------------------------------
func TestEvaluate_NotFound(t *testing.T) {
	prov := newMockProvider(map[string]any{})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "nonexistent", nil)
	assert.False(t, eval.Enabled)
	assert.Equal(t, "not_found", eval.MatchedRule)
}

// ------------------------------------------------------------------
// Evaluate_StringPercentage
// ------------------------------------------------------------------
func TestEvaluate_StringPercentage(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.rollout_str": "75",
	})
	eng := NewEngine(prov, "feature.")

	eval := eng.Evaluate(context.Background(), "rollout_str", nil)
	require.NotNil(t, eval.Flag)
	assert.Equal(t, FlagTypePercentage, eval.Flag.Type)
	assert.Equal(t, 75, eval.Flag.Percentage)
}

// ------------------------------------------------------------------
// Engine_CustomPrefix
// ------------------------------------------------------------------
func TestEngine_CustomPrefix(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"ff.my_flag": true,
	})
	eng := NewEngine(prov, "ff.")
	assert.True(t, eng.IsEnabled(context.Background(), "my_flag"))
}

// ------------------------------------------------------------------
// NewEngine default prefix
// ------------------------------------------------------------------
func TestNewEngine_DefaultPrefix(t *testing.T) {
	prov := newMockProvider(map[string]any{
		"feature.flag": true,
	})
	eng := NewEngine(prov, "")
	assert.True(t, eng.IsEnabled(context.Background(), "flag"))
}
