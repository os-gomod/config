// Package featureflag provides a feature flag engine built on top of the
// config package. It evaluates boolean, percentage, and targeting rules
// using configuration values, enabling progressive rollouts, A/B testing,
// and environment-specific feature toggles with zero additional infrastructure.
//
// Feature flags are stored as config values under a configurable prefix
// (default: "feature."). Each flag is evaluated lazily on access, so
// changes to the underlying config are immediately reflected without
// requiring any flag-specific reload or cache invalidation.
//
// Example usage:
//
//	cfg, _ := config.New(ctx,
//	    config.WithLoader(memoryLoader),
//	)
//	engine := featureflag.NewEngine(cfg)
//
//	// Boolean flag: feature.new_ui = true
//	if engine.IsEnabled(ctx, "new_ui") {
//	    // render new UI
//	}
//
//	// Percentage flag: feature.beta_rollout = 30 (30% of users)
//	if engine.IsEnabledFor(ctx, "beta_rollout", featureflag.WithIdentifier("user-123")) {
//	    // enable for this user (deterministic based on identifier)
//	}
package featureflag

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/os-gomod/config/core/value"
)

// FlagType represents the type of a feature flag.
type FlagType int

const (
	// FlagTypeBoolean is a simple on/off flag.
	FlagTypeBoolean FlagType = iota
	// FlagTypePercentage enables the flag for a percentage of identifiers.
	FlagTypePercentage
	// FlagTypeVariant returns one of multiple string variants (A/B testing).
	FlagTypeVariant
)

// String returns a human-readable representation of the flag type.
func (t FlagType) String() string {
	switch t {
	case FlagTypeBoolean:
		return "boolean"
	case FlagTypePercentage:
		return "percentage"
	case FlagTypeVariant:
		return "variant"
	default:
		return "unknown"
	}
}

// Flag holds the configuration for a single feature flag.
// It is populated from config values and evaluated lazily.
type Flag struct {
	// Name is the flag name (without the prefix).
	Name string
	// Type is the evaluation type (boolean, percentage, variant).
	Type FlagType
	// Enabled is the raw boolean value (for FlagTypeBoolean).
	Enabled bool
	// Percentage is the rollout percentage 0-100 (for FlagTypePercentage).
	Percentage int
	// Variants is the list of possible variants (for FlagTypeVariant).
	// Weights are optional and default to equal distribution.
	Variants []string
	// Default is the default value when the flag is not configured.
	Default any
	// Description is a human-readable description of the flag.
	Description string
	// Tags are optional labels for categorization and querying.
	Tags map[string]string
}

// Evaluation holds the result of evaluating a feature flag.
type Evaluation struct {
	// Flag is the flag that was evaluated.
	Flag *Flag
	// Enabled reports whether the flag is active for the given context.
	Enabled bool
	// Variant is the selected variant (for FlagTypeVariant).
	Variant string
	// Source is the config value source that provided the flag.
	Source value.Source
	// MatchedRule describes which rule caused the enable/disable decision.
	MatchedRule string
}

// TargetingRule defines a condition for flag evaluation.
type TargetingRule struct {
	// Attribute is the identifier attribute to match against (e.g., "country", "email", "tier").
	Attribute string
	// Operator is the comparison operator: "eq", "neq", "in", "not_in", "contains", "prefix", "suffix".
	Operator string
	// Values are the comparison values.
	Values []string
	// Enable is true to enable the flag when the rule matches, false to disable.
	Enable bool
}

// EvalContext holds contextual information for flag evaluation.
type EvalContext struct {
	// Identifier is a stable identifier for percentage-based rollouts
	// (e.g., user ID, session ID). Required for FlagTypePercentage.
	Identifier string
	// Attributes are arbitrary key-value pairs for targeting rules.
	Attributes map[string]string
}

// ConfigProvider is an interface for reading configuration values.
// It is implemented by config.Config.
type ConfigProvider interface {
	Get(key string) (value.Value, bool)
}

// Engine evaluates feature flags using configuration values.
// It reads flags from the config system with a configurable prefix
// and supports boolean, percentage, and variant evaluation modes.
type Engine struct {
	provider ConfigProvider
	prefix   string
}

// NewEngine creates a new feature flag engine backed by the given config.
// Flags are read from config keys prefixed with the given prefix.
// If prefix is empty, defaults to "feature.".
func NewEngine(provider ConfigProvider, prefix string) *Engine {
	if prefix == "" {
		prefix = "feature."
	}
	return &Engine{
		provider: provider,
		prefix:   prefix,
	}
}

// resolveKey constructs the full config key from a flag name.
func (e *Engine) resolveKey(name string) string {
	return e.prefix + name
}

// IsEnabled reports whether a boolean flag is enabled.
// Returns false if the flag is not found or not a valid boolean.
func (e *Engine) IsEnabled(ctx context.Context, name string) bool {
	eval := e.Evaluate(ctx, name, nil)
	return eval.Enabled
}

// IsEnabledFor reports whether a flag is enabled for the given evaluation context.
// The EvalContext provides the identifier for percentage-based rollouts
// and attributes for targeting rules.
func (e *Engine) IsEnabledFor(ctx context.Context, name string, evalCtx *EvalContext) bool {
	return e.Evaluate(ctx, name, evalCtx).Enabled
}

// Evaluate performs a full evaluation of the named flag.
// It returns an Evaluation with the result and metadata about the decision.
func (e *Engine) Evaluate(_ context.Context, name string, evalCtx *EvalContext) Evaluation {
	key := e.resolveKey(name)
	v, ok := e.provider.Get(key)
	if !ok {
		return Evaluation{
			Flag:        &Flag{Name: name, Type: FlagTypeBoolean, Enabled: false},
			Enabled:     false,
			MatchedRule: "not_found",
		}
	}

	// Detect flag type from the value
	flagType := detectFlagType(v, key)
	flag := &Flag{
		Name:    name,
		Type:    flagType,
		Default: false,
	}

	switch flagType {
	case FlagTypeBoolean:
		return e.evaluateBoolean(flag, v)
	case FlagTypePercentage:
		return e.evaluatePercentage(flag, v, evalCtx)
	case FlagTypeVariant:
		return e.evaluateVariant(flag, v, evalCtx)
	default:
		return Evaluation{
			Flag:        flag,
			Enabled:     false,
			MatchedRule: "unknown_type",
		}
	}
}

// evaluateBoolean handles boolean flag evaluation.
func (e *Engine) evaluateBoolean(flag *Flag, v value.Value) Evaluation {
	enabled, ok := v.Bool()
	if !ok {
		// Try string parsing
		s := strings.ToLower(v.String())
		enabled = s == "true" || s == "1" || s == "yes" || s == "on"
	}
	flag.Enabled = enabled
	return Evaluation{
		Flag:        flag,
		Enabled:     enabled,
		Source:      v.Source(),
		MatchedRule: "boolean_value",
	}
}

// evaluatePercentage handles percentage-based rollout evaluation.
// The flag value should be an integer 0-100.
// Deterministic routing is based on hashing the identifier.
func (e *Engine) evaluatePercentage(flag *Flag, v value.Value, evalCtx *EvalContext) Evaluation {
	pct, ok := v.Int()
	if !ok {
		// Try string parsing
		pct, _ = strconv.Atoi(strings.TrimSpace(v.String()))
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	flag.Percentage = pct

	if evalCtx == nil || evalCtx.Identifier == "" {
		// Without an identifier, percentage flags default to enabled if > 0
		// This allows server-side evaluation without user context
		return Evaluation{
			Flag:        flag,
			Enabled:     pct > 0,
			Source:      v.Source(),
			MatchedRule: "percentage_no_identifier",
		}
	}

	// Deterministic hash-based routing
	hash := hashIdentifier(evalCtx.Identifier, flag.Name)
	enabled := int(hash%100) < pct

	rule := fmt.Sprintf("percentage(%d%%): hash=%d", pct, hash%100)
	return Evaluation{
		Flag:        flag,
		Enabled:     enabled,
		Source:      v.Source(),
		MatchedRule: rule,
	}
}

// evaluateVariant handles variant-based flag evaluation (A/B testing).
// The flag value should be a comma-separated list of variant names.
// Deterministic routing selects one variant based on the identifier hash.
func (e *Engine) evaluateVariant(flag *Flag, v value.Value, evalCtx *EvalContext) Evaluation {
	raw := v.String()
	variants := splitVariants(raw)
	if len(variants) == 0 {
		return Evaluation{
			Flag:        flag,
			Enabled:     false,
			MatchedRule: "no_variants",
		}
	}
	flag.Variants = variants

	if len(variants) == 1 {
		return Evaluation{
			Flag:        flag,
			Enabled:     true,
			Variant:     variants[0],
			Source:      v.Source(),
			MatchedRule: "single_variant",
		}
	}

	if evalCtx == nil || evalCtx.Identifier == "" {
		return Evaluation{
			Flag:        flag,
			Enabled:     true,
			Variant:     variants[0],
			Source:      v.Source(),
			MatchedRule: "variant_no_identifier",
		}
	}

	// Deterministic selection based on identifier hash
	hash := hashIdentifier(evalCtx.Identifier, flag.Name)
	nVariants := uint32(len(variants)) //nolint:gosec // len is always positive for valid variants
	idx := int(hash % nVariants)

	return Evaluation{
		Flag:        flag,
		Enabled:     true,
		Variant:     variants[idx],
		Source:      v.Source(),
		MatchedRule: fmt.Sprintf("variant(%d/%d): hash=%d", idx, nVariants, hash%nVariants),
	}
}

// detectFlagType determines the flag type from its config value and key.
// Convention:
//   - Keys ending in ".enabled" or ".pct" or ".percent" → special handling
//   - Pure boolean values ("true"/"false"/bool) → FlagTypeBoolean
//   - Pure integer values 0-100 → FlagTypePercentage
//   - String values with commas → FlagTypeVariant
func detectFlagType(v value.Value, key string) FlagType {
	// Check for explicit type hint in key
	lower := strings.ToLower(key)
	if strings.HasSuffix(lower, ".enabled") {
		return FlagTypeBoolean
	}
	if strings.HasSuffix(lower, ".pct") || strings.HasSuffix(lower, ".percent") ||
		strings.HasSuffix(lower, ".percentage") {
		return FlagTypePercentage
	}
	if strings.HasSuffix(lower, ".variants") || strings.HasSuffix(lower, ".variant") {
		return FlagTypeVariant
	}

	// Auto-detect from value
	if _, ok := v.Bool(); ok {
		return FlagTypeBoolean
	}
	if i, ok := v.Int(); ok && i >= 0 && i <= 100 {
		return FlagTypePercentage
	}
	// Try parsing string values as integers for percentage detection
	s := v.String()
	if s != "" {
		if i, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && i >= 0 && i <= 100 {
			return FlagTypePercentage
		}
		if strings.Contains(s, ",") {
			return FlagTypeVariant
		}
	}

	// Default to boolean
	return FlagTypeBoolean
}

// hashIdentifier produces a deterministic uint32 hash from an identifier and flag name.
// The flag name is included to ensure different flags produce different distributions
// for the same identifier.
func hashIdentifier(identifier, flagName string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(flagName + ":" + identifier))
	return h.Sum32()
}

// splitVariants splits a comma-separated variant string into individual variants.
func splitVariants(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	variants := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			variants = append(variants, p)
		}
	}
	return variants
}
