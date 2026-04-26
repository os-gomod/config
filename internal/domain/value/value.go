// Package value provides core domain types for configuration values,
// including type enumeration, source tracking, priority ordering,
// secret detection, and checksum computation.
package value

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Type — value type enumeration
// ---------------------------------------------------------------------------

// Type represents the data type of a configuration value.
type Type int

const (
	TypeUnknown  Type = iota // Unknown / unspecified
	TypeString               // string
	TypeInt                  // int
	TypeInt64                // int64
	TypeFloat64              // float64
	TypeBool                 // bool
	TypeDuration             // time.Duration
	TypeTime                 // time.Time
	TypeSlice                // []any
	TypeMap                  // map[string]any
	TypeBytes                // []byte
)

func (t Type) String() string {
	switch t {
	case TypeString:
		return "string"
	case TypeInt:
		return "int"
	case TypeInt64:
		return "int64"
	case TypeFloat64:
		return "float64"
	case TypeBool:
		return "bool"
	case TypeDuration:
		return "duration"
	case TypeTime:
		return "time"
	case TypeSlice:
		return "slice"
	case TypeMap:
		return "map"
	case TypeBytes:
		return "bytes"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Source — value source enumeration
// ---------------------------------------------------------------------------

// Source represents where a configuration value originated.
type Source int

const (
	SourceUnknown    Source = iota // Unknown source
	SourceFile                     // File on disk (YAML, JSON, TOML, HCL, etc.)
	SourceEnv                      // Environment variable
	SourceMemory                   // Set programmatically
	SourceHTTP                     // HTTP endpoint (consul, etc.)
	SourceKubernetes               // Kubernetes ConfigMap / Secret
	SourceVault                    // HashiCorp Vault
	SourceKMS                      // Key Management Service
	SourceRemote                   // Generic remote source
)

func (s Source) String() string {
	switch s {
	case SourceFile:
		return "file"
	case SourceEnv:
		return "env"
	case SourceMemory:
		return "memory"
	case SourceHTTP:
		return "http"
	case SourceKubernetes:
		return "kubernetes"
	case SourceVault:
		return "vault"
	case SourceKMS:
		return "kms"
	case SourceRemote:
		return "remote"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Value
// ---------------------------------------------------------------------------

// Value represents a single configuration value with type, source, and priority metadata.
type Value struct {
	raw      any
	typ      Type
	src      Source
	priority int
}

// New creates a Value from a raw interface{} with inferred type and source.
func New(raw any) Value {
	v := Value{raw: raw, priority: 0}
	v.typ = inferType(raw)
	return v
}

// NewInMemory creates a Value set programmatically with the given priority.
func NewInMemory(raw any, priority int) Value {
	v := Value{raw: raw, src: SourceMemory, priority: priority}
	v.typ = inferType(raw)
	return v
}

// FromRaw creates a Value with an explicit type and source.
func FromRaw(raw any, typ Type, src Source, priority int) Value {
	return Value{
		raw:      raw,
		typ:      typ,
		src:      src,
		priority: priority,
	}
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

// Raw returns the underlying raw value.
func (v Value) Raw() any { return v.raw }

// Type returns the value type.
func (v Value) Type() Type { return v.typ }

// Source returns where the value originated.
func (v Value) Source() Source { return v.src }

// Priority returns the merge priority (higher wins).
func (v Value) Priority() int { return v.priority }

// String returns the value as a string, or "" if not a string.
func (v Value) String() string {
	if v.raw == nil {
		return ""
	}
	s, ok := v.raw.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v.raw)
}

// Bool returns the value as a bool.
func (v Value) Bool() bool {
	if v.raw == nil {
		return false
	}
	switch val := v.raw.(type) {
	case bool:
		return val
	case string:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false
		}
		return b
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
		return val != 0
	default:
		return false
	}
}

// Int returns the value as an int.
func (v Value) Int() int {
	if v.raw == nil {
		return 0
	}
	switch val := v.raw.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		i, err := strconv.Atoi(val)
		if err != nil {
			return 0
		}
		return i
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Int64 returns the value as an int64.
func (v Value) Int64() int64 {
	if v.raw == nil {
		return 0
	}
	switch val := v.raw.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case string:
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0
		}
		return i
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Float64 returns the value as a float64.
func (v Value) Float64() float64 {
	if v.raw == nil {
		return 0
	}
	switch val := v.raw.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0
		}
		return f
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Duration returns the value as a time.Duration.
func (v Value) Duration() time.Duration {
	if v.raw == nil {
		return 0
	}
	switch val := v.raw.(type) {
	case time.Duration:
		return val
	case int64:
		return time.Duration(val)
	case int:
		return time.Duration(val)
	case float64:
		return time.Duration(int64(val))
	case string:
		d, err := time.ParseDuration(val)
		if err != nil {
			return 0
		}
		return d
	default:
		return 0
	}
}

// Time returns the value as a time.Time.
func (v Value) Time() time.Time {
	if v.raw == nil {
		return time.Time{}
	}
	switch val := v.raw.(type) {
	case time.Time:
		return val
	case string:
		// Try RFC3339 first, then common layouts.
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			time.RFC1123,
			time.RFC1123Z,
			time.RFC850,
			time.RFC822,
			time.RFC822Z,
			time.DateTime,
			time.DateOnly,
			"2006-01-02 15:04:05",
			"2006-01-02",
		} {
			t, err := time.Parse(layout, val)
			if err == nil {
				return t
			}
		}
		return time.Time{}
	default:
		return time.Time{}
	}
}

// Slice returns the value as a []any, or nil if not a slice.
func (v Value) Slice() []any {
	if v.raw == nil {
		return nil
	}
	switch val := v.raw.(type) {
	case []any:
		return val
	case []string:
		result := make([]any, len(val))
		for i, s := range val {
			result[i] = s
		}
		return result
	case []int:
		result := make([]any, len(val))
		for i, n := range val {
			result[i] = n
		}
		return result
	case []float64:
		result := make([]any, len(val))
		for i, f := range val {
			result[i] = f
		}
		return result
	case []bool:
		result := make([]any, len(val))
		for i, b := range val {
			result[i] = b
		}
		return result
	default:
		return nil
	}
}

// Map returns the value as a map[string]any, or nil if not a map.
func (v Value) Map() map[string]any {
	if v.raw == nil {
		return nil
	}
	switch val := v.raw.(type) {
	case map[string]any:
		return val
	case map[string]string:
		result := make(map[string]any, len(val))
		for k, s := range val {
			result[k] = s
		}
		return result
	default:
		return nil
	}
}

// Bytes returns the value as []byte.
func (v Value) Bytes() []byte {
	if v.raw == nil {
		return nil
	}
	switch val := v.raw.(type) {
	case []byte:
		return val
	case string:
		return []byte(val)
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Comparison / utility
// ---------------------------------------------------------------------------

// Equal returns true if two Values are deeply equal in raw, type, source, and priority.
func (v Value) Equal(other Value) bool {
	return v.typ == other.typ &&
		v.src == other.src &&
		v.priority == other.priority &&
		deepEqual(v.raw, other.raw)
}

// deepEqual performs a deep comparison of two values without using reflect.DeepEqual
// for the core types we care about.
//
//nolint:gocyclo,cyclop // high complexity due to exhaustive type switch
func deepEqual(a, b any) bool {
	if equal, handled := deepEqualScalar(a, b); handled {
		return equal
	}

	switch av := a.(type) {
	case []byte:
		return deepEqualBytes(av, b)
	case map[string]any:
		return deepEqualMap(av, b)
	case []any:
		return deepEqualSlice(av, b)
	default:
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
}

//nolint:revive // the paired bools are intentionally positional: equal, handled
func deepEqualScalar(a, b any) (bool, bool) {
	if a == nil || b == nil {
		return a == nil && b == nil, true
	}

	switch av := a.(type) {
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv, true
	case int:
		bv, ok := b.(int)
		return ok && av == bv, true
	case int64:
		bv, ok := b.(int64)
		return ok && av == bv, true
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv, true
	case string:
		bv, ok := b.(string)
		return ok && av == bv, true
	case time.Duration:
		bv, ok := b.(time.Duration)
		return ok && av == bv, true
	case time.Time:
		bv, ok := b.(time.Time)
		return ok && av.Equal(bv), true
	default:
		return false, false
	}
}

func deepEqualBytes(a []byte, b any) bool {
	bv, ok := b.([]byte)
	if !ok || len(a) != len(bv) {
		return false
	}

	for i := range a {
		if a[i] != bv[i] {
			return false
		}
	}

	return true
}

func deepEqualMap(a map[string]any, b any) bool {
	bv, ok := b.(map[string]any)
	if !ok || len(a) != len(bv) {
		return false
	}

	for key, value := range a {
		other, exists := bv[key]
		if !exists || !deepEqual(value, other) {
			return false
		}
	}

	return true
}

func deepEqualSlice(a []any, b any) bool {
	bv, ok := b.([]any)
	if !ok || len(a) != len(bv) {
		return false
	}

	for i := range a {
		if !deepEqual(a[i], bv[i]) {
			return false
		}
	}

	return true
}

// IsZero returns true if the value has no meaningful content.
func (v Value) IsZero() bool {
	if v.raw == nil {
		return true
	}
	switch val := v.raw.(type) {
	case string:
		return val == ""
	case int, int64, float64:
		return val == 0 || val == int64(0) || val == float64(0)
	case bool:
		return !val
	case time.Duration:
		return val == 0
	case time.Time:
		return val.IsZero()
	case []any, []byte, []string, []int, []float64, []bool:
		return fmt.Sprintf("%v", val) == "[]" || fmt.Sprintf("%v", val) == ""
	case map[string]any, map[string]string:
		return fmt.Sprintf("%v", val) == "map[]" || fmt.Sprintf("%v", val) == ""
	default:
		return v.raw == nil
	}
}

// As attempts to cast the raw value to T. Returns the value and true on success,
// or the zero value and false on failure.
func As[T any](v Value) (T, bool) {
	t, ok := v.raw.(T)
	return t, ok
}

// InferType determines the Type of the raw value.
func (v Value) InferType() Type {
	return inferType(v.raw)
}

// inferType determines the Type for any value.
func inferType(raw any) Type {
	if raw == nil {
		return TypeUnknown
	}
	switch raw.(type) {
	case string:
		return TypeString
	case int:
		return TypeInt
	case int64:
		return TypeInt64
	case float64:
		return TypeFloat64
	case bool:
		return TypeBool
	case time.Duration:
		return TypeDuration
	case time.Time:
		return TypeTime
	case []any, []string, []int, []int64, []float64, []bool:
		return TypeSlice
	case map[string]any, map[string]string:
		return TypeMap
	case []byte:
		return TypeBytes
	default:
		// JSON number
		if _, ok := raw.(json.Number); ok {
			return TypeString // treat as string until explicitly parsed
		}
		return TypeUnknown
	}
}

// ---------------------------------------------------------------------------
// Checksum
// ---------------------------------------------------------------------------

// ComputeChecksum returns a SHA-256 hex checksum of the value's raw data.
func (v Value) ComputeChecksum() string {
	h := sha256.New()
	writeValue(h, v.raw)
	return hex.EncodeToString(h.Sum(nil))
}

// ComputeMapChecksum returns a SHA-256 hex checksum of a map of Values.
// Keys are sorted for deterministic output.
func ComputeMapChecksum(data map[string]Value) string {
	h := sha256.New()
	keys := SortedKeys(data)
	for _, k := range keys {
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte{0}) // null separator
		writeValue(h, data[k].raw)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

type byteWriter interface {
	Write(p []byte) (n int, err error)
}

func writeValue(h byteWriter, raw any) {
	if raw == nil {
		writeBytes(h, []byte("nil"))
		return
	}
	switch val := raw.(type) {
	case []byte:
		writeBytes(h, val)
	case bool:
		writeStringValue(h, strconv.FormatBool(val))
	case int:
		writeStringValue(h, strconv.FormatInt(int64(val), 10))
	case int64:
		writeStringValue(h, strconv.FormatInt(val, 10))
	case float64:
		writeFloatValue(h, val)
	case string:
		writeStringValue(h, val)
	case time.Duration:
		writeStringValue(h, val.String())
	case time.Time:
		writeStringValue(h, val.UTC().Format(time.RFC3339Nano))
	case map[string]any:
		writeMapValue(h, val)
	case []any:
		writeSliceValue(h, val)
	default:
		writeFallbackValue(h, val)
	}
}

func writeBytes(h byteWriter, data []byte) {
	_, _ = h.Write(data)
}

func writeStringValue(h byteWriter, s string) {
	writeBytes(h, []byte(s))
}

func writeFloatValue(h byteWriter, value float64) {
	formatted := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(formatted, ".") {
		formatted += ".0"
	}
	writeStringValue(h, formatted)
}

func writeMapValue(h byteWriter, data map[string]any) {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		writeStringValue(h, k)
		writeBytes(h, []byte{0})
		writeValue(h, data[k])
		writeBytes(h, []byte{0})
	}
}

func writeSliceValue(h byteWriter, items []any) {
	for _, item := range items {
		writeValue(h, item)
		writeBytes(h, []byte{0})
	}
}

func writeFallbackValue(h byteWriter, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		_, _ = fmt.Fprintf(h, "%v", value)
		return
	}

	writeBytes(h, encoded)
}

// ---------------------------------------------------------------------------
// Copy / SortedKeys
// ---------------------------------------------------------------------------

// Copy returns a shallow copy of a map[string]Value.
func Copy(data map[string]Value) map[string]Value {
	if data == nil {
		return nil
	}
	out := make(map[string]Value, len(data))
	for k, v := range data {
		out[k] = v
	}
	return out
}

// SortedKeys returns the sorted keys of a map[string]Value.
func SortedKeys(data map[string]Value) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------------------
// Secret detection and redaction
// ---------------------------------------------------------------------------

// secretPatterns lists key substrings that indicate a secret value.
var secretPatterns = []string{
	"password",
	"secret",
	"token",
	"api_key",
	"apikey",
	"private_key",
	"credential",
}

// IsSecret returns true if the key suggests a secret value.
func IsSecret(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range secretPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// Redact returns a copy of the Value with the raw value replaced by a redaction
// marker if the key indicates a secret. Non-secret keys are returned unchanged.
func (v Value) Redact(key string) Value {
	if !IsSecret(key) {
		return v
	}
	return Value{
		raw:      "***REDACTED***",
		typ:      TypeString,
		src:      v.src,
		priority: v.priority,
	}
}

// RedactMap returns a copy of the map with secret values redacted.
func RedactMap(data map[string]Value) map[string]Value {
	if data == nil {
		return nil
	}
	out := make(map[string]Value, len(data))
	for k, v := range data {
		out[k] = v.Redact(k)
	}
	return out
}

// ---------------------------------------------------------------------------
// Floating point helpers
// ---------------------------------------------------------------------------

// Float64FromAny converts any numeric type to float64.
func Float64FromAny(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return math.NaN(), false
	}
}

// NumericCoerce attempts to convert any value to float64.
func NumericCoerce(raw any) float64 {
	switch val := raw.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint64:
		return float64(val)
	case int32:
		return float64(val)
	case uint32:
		return float64(val)
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return math.NaN()
		}
		return f
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return math.NaN()
	}
}

// ---------------------------------------------------------------------------
// String representation
// ---------------------------------------------------------------------------

// GoString implements fmt.GoStringer for nicer debugging output.
func (v Value) GoString() string {
	return fmt.Sprintf("value.Value{raw: %#v, typ: %s, src: %s, priority: %d}",
		v.raw, v.typ, v.src, v.priority)
}
