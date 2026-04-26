// Package featureflag provides a domain-level feature flag evaluation engine.
// Flags can be boolean, variant (A/B), percentage-based, or rule-driven.
// The engine evaluates flags against a context containing user identifiers,
// attributes, and custom properties.
package featureflag

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// FlagType
// ---------------------------------------------------------------------------

// FlagType describes the type of feature flag.
type FlagType int

const (
	FlagTypeBoolean    FlagType = iota // Boolean on/off flag.
	FlagTypeVariant                    // Multi-variant flag (A/B/n testing).
	FlagTypePercentage                 // Percentage-based rollout (0-100).
)

func (ft FlagType) String() string {
	switch ft {
	case FlagTypeBoolean:
		return "boolean"
	case FlagTypeVariant:
		return "variant"
	case FlagTypePercentage:
		return "percentage"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Flag
// ---------------------------------------------------------------------------

// Flag defines a single feature flag with its configuration.
type Flag struct {
	Key         string            // Unique flag key.
	Name        string            // Human-readable name.
	Description string            // Flag description.
	Enabled     bool              // Global enabled state.
	FlagType    FlagType          // Type of flag.
	Variants    map[string]any    // Variant configurations (for variant/percentage flags).
	Default     any               // Default value when no rules match.
	Rules       []Rule            // Ordered evaluation rules.
	Percentage  float64           // Percentage rollout (0-100) for percentage flags.
	CreatedAt   time.Time         // When the flag was created.
	UpdatedAt   time.Time         // When the flag was last updated.
	Labels      map[string]string // Labels for categorization.
	Tags        []string          // Tags.
}

// ---------------------------------------------------------------------------
// Rule
// ---------------------------------------------------------------------------

// Rule is a single evaluation rule that can target specific segments.
type Rule struct {
	Priority   int         // Rule priority (lower = evaluated first).
	VariantKey string      // The variant to serve if this rule matches.
	Percent    float64     // Percentage of traffic this rule applies to (0-100).
	Conditions []Condition // All conditions must match (AND logic).
	Segment    string      // Named segment this rule targets.
}

// Condition is a single predicate within a rule.
type Condition struct {
	Attribute string      // The attribute to evaluate (e.g., "email", "country").
	Operator  ConditionOp // The comparison operator.
	Value     any         // The value to compare against.
}

// ConditionOp enumerates the supported comparison operators.
type ConditionOp int

const (
	OpEqual              ConditionOp = iota // ==
	OpNotEqual                              // !=
	OpContains                              // string contains
	OpNotContains                           // string does not contain
	OpGreaterThan                           // >
	OpLessThan                              // <
	OpGreaterThanOrEqual                    // >=
	OpLessThanOrEqual                       // <=
	OpIn                                    // value in list
	OpNotIn                                 // value not in list
	OpStartsWith                            // string prefix
	OpEndsWith                              // string suffix
	OpRegex                                 // regex match (value is pattern string)
	OpExists                                // attribute exists (Value ignored)
	OpNotExists                             // attribute does not exist
)

func (op ConditionOp) String() string {
	switch op {
	case OpEqual:
		return "=="
	case OpNotEqual:
		return "!="
	case OpContains:
		return "contains"
	case OpNotContains:
		return "not_contains"
	case OpGreaterThan:
		return ">"
	case OpLessThan:
		return "<"
	case OpGreaterThanOrEqual:
		return ">="
	case OpLessThanOrEqual:
		return "<="
	case OpIn:
		return "in"
	case OpNotIn:
		return "not_in"
	case OpStartsWith:
		return "starts_with"
	case OpEndsWith:
		return "ends_with"
	case OpRegex:
		return "regex"
	case OpExists:
		return "exists"
	case OpNotExists:
		return "not_exists"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Evaluation
// ---------------------------------------------------------------------------

// Evaluation is the result of evaluating a feature flag.
type Evaluation struct {
	FlagKey     string        // The flag key that was evaluated.
	VariantKey  string        // The selected variant (empty for boolean flags).
	Value       any           // The resolved value.
	Reason      EvalReason    // Why this value was selected.
	MatchedRule *Rule         // The rule that matched (nil if default).
	Timestamp   time.Time     // When the evaluation occurred.
	Matched     bool          // Whether the flag matched / is enabled.
	Duration    time.Duration // How long the evaluation took.
}

// EvalReason describes why a particular evaluation result was produced.
type EvalReason string

const (
	ReasonEnabled        EvalReason = "enabled"         // Flag is globally enabled (boolean).
	ReasonDisabled       EvalReason = "disabled"        // Flag is globally disabled.
	ReasonDefault        EvalReason = "default"         // No rules matched, returned default.
	ReasonRuleMatch      EvalReason = "rule_match"      // A specific rule matched.
	ReasonPercentage     EvalReason = "percentage"      // Percentage-based rollout matched.
	ReasonVariantMatch   EvalReason = "variant_match"   // Variant was selected.
	ReasonError          EvalReason = "error"           // An error occurred during evaluation.
	ReasonFlagNotFound   EvalReason = "flag_not_found"  // The flag does not exist.
	ReasonTargetingMatch EvalReason = "targeting_match" // Targeting conditions matched.
	ReasonFallthrough    EvalReason = "fallthrough"     // Fell through all rules.
)

func (r EvalReason) String() string { return string(r) }

// ---------------------------------------------------------------------------
// EvalContext
// ---------------------------------------------------------------------------

// EvalContext provides the context needed for flag evaluation.
type EvalContext struct {
	Identifier  string            // Unique identifier (user ID, session ID, etc.).
	Email       string            // User email.
	Country     string            // User country code.
	Attributes  map[string]any    // Custom attributes for rule evaluation.
	Labels      map[string]string // Labels for filtering.
	Anonymous   bool              // Whether the user is anonymous.
	Environment string            // Current environment (dev, staging, production).
	Timestamp   time.Time         // Evaluation timestamp.
}

// NewEvalContext creates an EvalContext with sensible defaults.
func NewEvalContext(identifier string) *EvalContext {
	return &EvalContext{
		Identifier: identifier,
		Attributes: make(map[string]any),
		Labels:     make(map[string]string),
		Timestamp:  time.Now().UTC(),
	}
}

// WithEmail sets the email on the context.
func (c *EvalContext) WithEmail(email string) *EvalContext {
	c.Email = email
	return c
}

// WithCountry sets the country on the context.
func (c *EvalContext) WithCountry(country string) *EvalContext {
	c.Country = country
	return c
}

// WithAttribute sets a custom attribute.
func (c *EvalContext) WithAttribute(key string, val any) *EvalContext {
	c.Attributes[key] = val
	return c
}

// WithAttributes sets multiple custom attributes.
func (c *EvalContext) WithAttributes(attrs map[string]any) *EvalContext {
	for k, v := range attrs {
		c.Attributes[k] = v
	}
	return c
}

// WithEnvironment sets the environment.
func (c *EvalContext) WithEnvironment(env string) *EvalContext {
	c.Environment = env
	return c
}

// ---------------------------------------------------------------------------
// ConfigProvider
// ---------------------------------------------------------------------------

// ConfigProvider is the interface for reading flag configurations from a
// config data source.
type ConfigProvider interface {
	// GetFlags returns all configured feature flags.
	GetFlags() map[string]Flag
	// GetFlag returns a single flag by key, or nil if not found.
	GetFlag(key string) *Flag
}

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// Engine evaluates feature flags against contexts.
// It is safe for concurrent use.
type Engine struct {
	provider ConfigProvider
	mu       sync.RWMutex
}

// NewEngine creates a new feature flag Engine.
func NewEngine(provider ConfigProvider) *Engine {
	return &Engine{
		provider: provider,
	}
}

// SetProvider replaces the config provider (thread-safe).
func (e *Engine) SetProvider(provider ConfigProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.provider = provider
}

// ---------------------------------------------------------------------------
// Evaluation methods
// ---------------------------------------------------------------------------

// Bool evaluates a boolean flag and returns true/false.
func (e *Engine) Bool(key string, ctx *EvalContext) bool {
	eval := e.Evaluate(key, ctx)
	if eval.Value == nil {
		return false
	}
	b, ok := eval.Value.(bool)
	if !ok {
		return false
	}
	return b
}

// Variant evaluates a variant flag and returns the selected variant key.
func (e *Engine) Variant(key string, ctx *EvalContext) string {
	eval := e.Evaluate(key, ctx)
	return eval.VariantKey
}

// String evaluates a flag and returns its value as a string.
func (e *Engine) String(key string, ctx *EvalContext, fallback string) string {
	eval := e.Evaluate(key, ctx)
	if eval.Value == nil {
		return fallback
	}
	s, ok := eval.Value.(string)
	if !ok {
		return fallback
	}
	return s
}

// Int evaluates a flag and returns its value as an int, or the fallback.
func (e *Engine) Int(key string, ctx *EvalContext, fallback int) int {
	eval := e.Evaluate(key, ctx)
	if eval.Value == nil {
		return fallback
	}
	switch v := eval.Value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return fallback
		}
		return i
	default:
		return fallback
	}
}

// Float64 evaluates a flag and returns its value as a float64, or the fallback.
func (e *Engine) Float64(key string, ctx *EvalContext, fallback float64) float64 {
	eval := e.Evaluate(key, ctx)
	if eval.Value == nil {
		return fallback
	}
	switch v := eval.Value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fallback
		}
		return f
	default:
		return fallback
	}
}

// Evaluate performs the full evaluation of a feature flag.
func (e *Engine) Evaluate(key string, ctx *EvalContext) Evaluation {
	start := time.Now()

	e.mu.RLock()
	provider := e.provider
	e.mu.RUnlock()

	if provider == nil {
		return Evaluation{
			FlagKey:   key,
			Reason:    ReasonError,
			Timestamp: time.Now().UTC(),
			Duration:  time.Since(start),
		}
	}

	flag := provider.GetFlag(key)
	if flag == nil {
		return Evaluation{
			FlagKey:   key,
			Reason:    ReasonFlagNotFound,
			Timestamp: time.Now().UTC(),
			Duration:  time.Since(start),
		}
	}

	return e.evaluateFlag(flag, ctx, start)
}

// EvaluateAll evaluates all flags and returns a map of results.
func (e *Engine) EvaluateAll(ctx *EvalContext) map[string]Evaluation {
	start := time.Now()

	e.mu.RLock()
	provider := e.provider
	e.mu.RUnlock()

	if provider == nil {
		return nil
	}

	flags := provider.GetFlags()
	results := make(map[string]Evaluation, len(flags))
	for key, flag := range flags {
		results[key] = e.evaluateFlag(&flag, ctx, start)
	}
	return results
}

// ListFlags returns all registered flag keys.
func (e *Engine) ListFlags() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.provider == nil {
		return nil
	}
	flags := e.provider.GetFlags()
	keys := make([]string, 0, len(flags))
	for k := range flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------------------
// Internal evaluation
// ---------------------------------------------------------------------------

func (e *Engine) evaluateFlag(flag *Flag, ctx *EvalContext, start time.Time) Evaluation {
	if !flag.Enabled {
		return Evaluation{
			FlagKey:   flag.Key,
			Reason:    ReasonDisabled,
			Value:     flag.Default,
			Timestamp: time.Now().UTC(),
			Duration:  time.Since(start),
			Matched:   false,
		}
	}

	switch flag.FlagType {
	case FlagTypeBoolean:
		return e.evaluateBoolean(flag, ctx, start)
	case FlagTypeVariant:
		return e.evaluateVariant(flag, ctx, start)
	case FlagTypePercentage:
		return e.evaluatePercentage(flag, ctx, start)
	default:
		return e.evaluateBoolean(flag, ctx, start)
	}
}

func (e *Engine) evaluateBoolean(flag *Flag, ctx *EvalContext, start time.Time) Evaluation {
	// Check rules first (sorted by priority).
	rules := sortedRules(flag.Rules)
	for _, rule := range rules {
		if e.matchesConditions(ctx, rule.Conditions) {
			val := flag.Default
			if rule.VariantKey != "" {
				if v, ok := flag.Variants[rule.VariantKey]; ok {
					val = v
				}
			}
			return Evaluation{
				FlagKey:     flag.Key,
				VariantKey:  rule.VariantKey,
				Value:       val,
				Reason:      ReasonRuleMatch,
				MatchedRule: &rule,
				Timestamp:   time.Now().UTC(),
				Duration:    time.Since(start),
				Matched:     true,
			}
		}
	}

	val := true // boolean flags default to true when enabled
	if flag.Default != nil {
		if b, ok := flag.Default.(bool); ok {
			val = b
		}
	}
	return Evaluation{
		FlagKey:   flag.Key,
		Value:     val,
		Reason:    ReasonEnabled,
		Timestamp: time.Now().UTC(),
		Duration:  time.Since(start),
		Matched:   val,
	}
}

func (e *Engine) evaluateVariant(flag *Flag, ctx *EvalContext, start time.Time) Evaluation {
	// Check rules first.
	rules := sortedRules(flag.Rules)
	for _, rule := range rules {
		if !e.matchesConditions(ctx, rule.Conditions) {
			continue
		}
		variantKey := rule.VariantKey
		if variantKey == "" {
			variantKey = selectVariant(ctx.Identifier, flag.Variants)
		}
		val, ok := flag.Variants[variantKey]
		if !ok {
			// Fall through to default.
			break
		}
		return Evaluation{
			FlagKey:     flag.Key,
			VariantKey:  variantKey,
			Value:       val,
			Reason:      ReasonVariantMatch,
			MatchedRule: &rule,
			Timestamp:   time.Now().UTC(),
			Duration:    time.Since(start),
			Matched:     true,
		}
	}

	// Fall through: select variant by hash of identifier.
	variantKey := selectVariant(ctx.Identifier, flag.Variants)
	val, ok := flag.Variants[variantKey]
	if !ok {
		return Evaluation{
			FlagKey:   flag.Key,
			Value:     flag.Default,
			Reason:    ReasonDefault,
			Timestamp: time.Now().UTC(),
			Duration:  time.Since(start),
			Matched:   false,
		}
	}

	return Evaluation{
		FlagKey:    flag.Key,
		VariantKey: variantKey,
		Value:      val,
		Reason:     ReasonVariantMatch,
		Timestamp:  time.Now().UTC(),
		Duration:   time.Since(start),
		Matched:    true,
	}
}

func (e *Engine) evaluatePercentage(flag *Flag, ctx *EvalContext, start time.Time) Evaluation {
	percentage := flag.Percentage
	if percentage <= 0 {
		return Evaluation{
			FlagKey:   flag.Key,
			Value:     flag.Default,
			Reason:    ReasonPercentage,
			Timestamp: time.Now().UTC(),
			Duration:  time.Since(start),
			Matched:   false,
		}
	}

	if percentage >= 100 {
		return Evaluation{
			FlagKey:   flag.Key,
			Value:     true,
			Reason:    ReasonPercentage,
			Timestamp: time.Now().UTC(),
			Duration:  time.Since(start),
			Matched:   true,
		}
	}

	hash := hashIdentifier(ctx.Identifier, flag.Key)
	bucket := float64(hash%10000) / 100.0

	if bucket < percentage {
		return Evaluation{
			FlagKey:   flag.Key,
			Value:     true,
			Reason:    ReasonPercentage,
			Timestamp: time.Now().UTC(),
			Duration:  time.Since(start),
			Matched:   true,
		}
	}

	return Evaluation{
		FlagKey:   flag.Key,
		Value:     flag.Default,
		Reason:    ReasonPercentage,
		Timestamp: time.Now().UTC(),
		Duration:  time.Since(start),
		Matched:   false,
	}
}

// ---------------------------------------------------------------------------
// Condition matching
// ---------------------------------------------------------------------------

func (e *Engine) matchesConditions(ctx *EvalContext, conditions []Condition) bool {
	if len(conditions) == 0 {
		return true // no conditions = matches everything
	}

	for _, cond := range conditions {
		attrVal := resolveAttribute(ctx, cond.Attribute)
		if !evaluateCondition(attrVal, cond) {
			return false
		}
	}
	return true
}

// resolveAttribute extracts an attribute value from the context.
func resolveAttribute(ctx *EvalContext, attr string) any {
	switch strings.ToLower(attr) {
	case "identifier", "id", "userid", "user_id":
		return ctx.Identifier
	case "email":
		return ctx.Email
	case "country", "region":
		return ctx.Country
	case "anonymous":
		return ctx.Anonymous
	case "environment", "env":
		return ctx.Environment
	default:
		if ctx.Attributes != nil {
			if v, ok := ctx.Attributes[attr]; ok {
				return v
			}
			// Try case-insensitive lookup.
			lower := strings.ToLower(attr)
			for k, v := range ctx.Attributes {
				if strings.EqualFold(k, lower) {
					return v
				}
			}
		}
		return nil
	}
}

// evaluateCondition evaluates a single condition.
func evaluateCondition(attrVal any, cond Condition) bool {
	// Handle existence operators.
	switch cond.Operator {
	case OpExists:
		return attrVal != nil
	case OpNotExists:
		return attrVal == nil
	}

	if attrVal == nil {
		return false
	}

	attrStr := fmt.Sprint(attrVal)
	condStr := fmt.Sprint(cond.Value)

	switch cond.Operator {
	case OpEqual:
		return attrStr == condStr
	case OpNotEqual:
		return attrStr != condStr
	case OpContains:
		return strings.Contains(attrStr, condStr)
	case OpNotContains:
		return !strings.Contains(attrStr, condStr)
	case OpGreaterThan:
		return numericCompare(attrVal, cond.Value) > 0
	case OpLessThan:
		return numericCompare(attrVal, cond.Value) < 0
	case OpGreaterThanOrEqual:
		return numericCompare(attrVal, cond.Value) >= 0
	case OpLessThanOrEqual:
		return numericCompare(attrVal, cond.Value) <= 0
	case OpIn:
		return valueInList(attrStr, cond.Value)
	case OpNotIn:
		return !valueInList(attrStr, cond.Value)
	case OpStartsWith:
		return strings.HasPrefix(attrStr, condStr)
	case OpEndsWith:
		return strings.HasSuffix(attrStr, condStr)
	case OpRegex:
		// Simple regex: just check if pattern appears in the value.
		// For production, use regexp package. Here we do substring match
		// as a safe default.
		return strings.Contains(attrStr, condStr)
	default:
		return false
	}
}

// numericCompare compares two values numerically.
// Returns 0 if equal, <0 if a < b, >0 if a > b.
func numericCompare(a, b any) int {
	af := value.NumericCoerce(a)
	bf := value.NumericCoerce(b)
	if math.IsNaN(af) || math.IsNaN(bf) {
		return -1 // non-numeric values sort before numbers
	}
	switch {
	case af < bf:
		return -1
	case af > bf:
		return 1
	default:
		return 0
	}
}

// valueInList checks if a string value is in a list.
func valueInList(val, list any) bool {
	switch l := list.(type) {
	case []string:
		s := fmt.Sprint(val)
		for _, item := range l {
			if item == s {
				return true
			}
		}
	case []any:
		s := fmt.Sprint(val)
		for _, item := range l {
			if fmt.Sprint(item) == s {
				return true
			}
		}
	case string:
		// Treat comma-separated string as a list.
		s := fmt.Sprint(val)
		for _, item := range strings.Split(l, ",") {
			if strings.TrimSpace(item) == s {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Flag type detection
// ---------------------------------------------------------------------------

// detectFlagType determines the FlagType from a raw config value.
func detectFlagType(raw any) FlagType {
	switch v := raw.(type) {
	case bool:
		return FlagTypeBoolean
	case float64:
		if v >= 0 && v <= 100 {
			return FlagTypePercentage
		}
		return FlagTypeBoolean
	case string:
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			return FlagTypePercentage
		}
		if v == "true" || v == "false" {
			return FlagTypeBoolean
		}
		return FlagTypeVariant
	case map[string]any:
		if len(v) > 0 {
			return FlagTypeVariant
		}
		return FlagTypeBoolean
	default:
		return FlagTypeBoolean
	}
}

// DetectFlagType is the exported version of detectFlagType.
func DetectFlagType(raw any) FlagType {
	return detectFlagType(raw)
}

// ---------------------------------------------------------------------------
// Hashing
// ---------------------------------------------------------------------------

// hashIdentifier returns a deterministic hash in [0, 10000) from identifier and salt.
func hashIdentifier(identifier, salt string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(identifier))
	if salt != "" {
		h.Write([]byte(salt))
	}
	return h.Sum32() % 10000
}

// ---------------------------------------------------------------------------
// Variant selection
// ---------------------------------------------------------------------------

// selectVariant deterministically selects a variant based on the identifier hash.
func selectVariant(identifier string, variants map[string]any) string {
	if len(variants) == 0 {
		return ""
	}

	keys := make([]string, 0, len(variants))
	for k := range variants {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 1 {
		return keys[0]
	}

	// Split the hash space evenly among variants.
	hash := hashIdentifier(identifier, "")
	bucketSize := 10000 / len(keys)
	idx := hash / uint32(bucketSize)
	if int(idx) >= len(keys) {
		idx = uint32(len(keys) - 1)
	}
	return keys[idx]
}

// sortedRules returns rules sorted by priority (ascending).
func sortedRules(rules []Rule) []Rule {
	if len(rules) <= 1 {
		return rules
	}
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	return sorted
}

// ---------------------------------------------------------------------------
// Flag from config value
// ---------------------------------------------------------------------------

// FlagFromValue creates a Flag from a config Value.
func FlagFromValue(key string, val value.Value) Flag {
	ft := detectFlagType(val.Raw())
	flag := Flag{
		Key:       key,
		FlagType:  ft,
		Enabled:   true,
		Variants:  make(map[string]any),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	switch ft {
	case FlagTypeBoolean:
		flag.Default = val.Bool()
	case FlagTypeVariant:
		if m := val.Map(); m != nil {
			for k, v := range m {
				flag.Variants[k] = v
			}
		}
		flag.Default = val.Raw()
	case FlagTypePercentage:
		flag.Percentage = val.Float64()
		flag.Default = true
	}

	return flag
}
