// Package value provides typed, source-tracked configuration values, diff
// computation, priority-based merging, and immutable state management. It is
// the foundational data layer used by the core engine and higher-level
// configuration APIs.
package value

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"time"
)

// Type enumerates the supported value types for configuration values.
type Type uint8

const (
	// TypeUnknown represents a value whose type could not be inferred.
	TypeUnknown Type = iota
	// TypeString represents a string value.
	TypeString
	// TypeInt represents an int value.
	TypeInt
	// TypeInt64 represents an int64 value.
	TypeInt64
	// TypeFloat64 represents a float64 value.
	TypeFloat64
	// TypeBool represents a bool value.
	TypeBool
	// TypeDuration represents a time.Duration value.
	TypeDuration
	// TypeTime represents a time.Time value.
	TypeTime
	// TypeSlice represents a []any (slice) value.
	TypeSlice
	// TypeMap represents a map[string]any value.
	TypeMap
	// TypeBytes represents a []byte value.
	TypeBytes
)

// typeNames maps Type constants to human-readable names.
var typeNames = [...]string{
	TypeUnknown:  "unknown",
	TypeString:   "string",
	TypeInt:      "int",
	TypeInt64:    "int64",
	TypeFloat64:  "float64",
	TypeBool:     "bool",
	TypeDuration: "duration",
	TypeTime:     "time",
	TypeSlice:    "slice",
	TypeMap:      "map",
	TypeBytes:    "bytes",
}

// String returns the human-readable name of the Type.
func (t Type) String() string {
	if int(t) < len(typeNames) {
		return typeNames[t]
	}
	return "unknown"
}

// Source enumerates the origin of a configuration value, indicating which
// kind of configuration source produced it.
type Source uint8

const (
	// SourceUnknown indicates an unknown or unspecified source.
	SourceUnknown Source = iota
	// SourceFile indicates the value was loaded from a file.
	SourceFile
	// SourceEnv indicates the value was loaded from an environment variable.
	SourceEnv
	// SourceMemory indicates the value was set programmatically in memory.
	SourceMemory
	// SourceHTTP indicates the value was fetched via HTTP.
	SourceHTTP
	// SourceKubernetes indicates the value was loaded from a Kubernetes ConfigMap or Secret.
	SourceKubernetes
	// SourceVault indicates the value was loaded from HashiCorp Vault.
	SourceVault
	// SourceKMS indicates the value was loaded from a Key Management Service.
	SourceKMS
	// SourceRemote indicates the value was loaded from a generic remote provider.
	SourceRemote
)

// sourceNames maps Source constants to human-readable names.
var sourceNames = [...]string{
	SourceUnknown:    "unknown",
	SourceFile:       "file",
	SourceEnv:        "env",
	SourceMemory:     "memory",
	SourceHTTP:       "http",
	SourceKubernetes: "kubernetes",
	SourceVault:      "vault",
	SourceKMS:        "kms",
	SourceRemote:     "remote",
}

// String returns the human-readable name of the Source.
func (s Source) String() string {
	if int(s) < len(sourceNames) {
		return sourceNames[s]
	}
	return "unknown"
}

// Value is a typed, source-tracked configuration value. Each Value knows
// its raw Go value, inferred type, origin source, and merge priority.
// Values are immutable after creation.
type Value struct {
	raw      any
	typ      Type
	src      Source
	priority int
}

// New creates a new Value with the given raw value, type, source, and
// priority.
func New(raw any, typ Type, src Source, priority int) Value {
	return Value{raw: raw, typ: typ, src: src, priority: priority}
}

// NewInMemory creates a Value from a raw Go value, using [SourceMemory] as
// the source, priority 100, and an auto-inferred [Type].
func NewInMemory(raw any) Value {
	return Value{raw: raw, typ: InferType(raw), src: SourceMemory, priority: 100}
}

// FromRaw creates a Value from a raw Go value with an auto-inferred type and
// [SourceUnknown] as the source. Useful for constructing values when the
// source is not relevant.
func FromRaw(raw any) Value {
	return Value{raw: raw, typ: InferType(raw), src: SourceUnknown}
}

// Raw returns the underlying Go value stored in this Value.
func (v Value) Raw() any { return v.raw }

// Type returns the inferred [Type] of this Value.
func (v Value) Type() Type { return v.typ }

// Source returns the [Source] that produced this Value.
func (v Value) Source() Source { return v.src }

// Priority returns the merge priority of this Value. Higher values win
// during conflict resolution.
func (v Value) Priority() int { return v.priority }

// String returns a fmt.Sprint representation of the raw value, or an empty
// string if the value is nil.
func (v Value) String() string {
	if v.raw == nil {
		return ""
	}
	return fmt.Sprintf("%v", v.raw)
}

// Bool attempts to extract a bool from the raw value. Returns the value and
// true if the assertion succeeds, or false otherwise.
func (v Value) Bool() (bool, bool) {
	val, ok := v.raw.(bool)
	return val, ok
}

// Int attempts to extract an int from the raw value. It also accepts int64
// and float64 values, truncating if necessary. Returns the value and true if
// the conversion succeeds.
func (v Value) Int() (int, bool) {
	switch val := v.raw.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	}
	return 0, false
}

// Int64 attempts to extract an int64 from the raw value. It also accepts int
// and float64 values, truncating if necessary. Returns the value and true if
// the conversion succeeds.
func (v Value) Int64() (int64, bool) {
	switch val := v.raw.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	}
	return 0, false
}

// Float64 attempts to extract a float64 from the raw value. It also accepts
// int and int64 values. Returns the value and true if the conversion
// succeeds.
func (v Value) Float64() (float64, bool) {
	switch val := v.raw.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return 0, false
}

// Duration attempts to extract a [time.Duration] from the raw value. It also
// accepts int64 (nanoseconds) and string (parsed via time.ParseDuration).
// Returns the duration and true if the conversion succeeds.
func (v Value) Duration() (time.Duration, bool) {
	switch val := v.raw.(type) {
	case time.Duration:
		return val, true
	case int64:
		return time.Duration(val), true
	case string:
		d, err := time.ParseDuration(val)
		return d, err == nil
	}
	return 0, false
}

// Time attempts to extract a [time.Time] from the raw value. Returns the
// time and true if the assertion succeeds.
func (v Value) Time() (time.Time, bool) {
	t, ok := v.raw.(time.Time)
	return t, ok
}

// Slice attempts to extract a []any from the raw value. Returns the slice
// and true if the assertion succeeds.
func (v Value) Slice() ([]any, bool) {
	s, ok := v.raw.([]any)
	return s, ok
}

// Map attempts to extract a map[string]any from the raw value. Returns the
// map and true if the assertion succeeds.
func (v Value) Map() (map[string]any, bool) {
	m, ok := v.raw.(map[string]any)
	return m, ok
}

// Bytes attempts to extract a []byte from the raw value. It also accepts
// string values, converting them to a byte slice. Returns the bytes and true
// if the conversion succeeds.
func (v Value) Bytes() ([]byte, bool) {
	switch val := v.raw.(type) {
	case []byte:
		return val, true
	case string:
		return []byte(val), true
	}
	return nil, false
}

// Equal reports whether two Values have the same type and deeply equal raw
// values (using [reflect.DeepEqual]).
func (v Value) Equal(other Value) bool {
	if v.typ != other.typ {
		return false
	}
	return reflect.DeepEqual(v.raw, other.raw)
}

// IsZero reports whether the Value has no raw data (nil).
func (v Value) IsZero() bool { return v.raw == nil }

// As is a generic helper that attempts to type-assert the raw value to T.
// Returns the value and true if the assertion succeeds, or the zero value
// of T and false otherwise.
//
//	var s string
//	s, ok := value.As[string](v)
func As[T any](v Value) (T, bool) {
	if v.raw == nil {
		var zero T
		return zero, false
	}
	res, ok := v.raw.(T)
	return res, ok
}

// InferType inspects the Go type of v and returns the corresponding [Type]
// constant. Unrecognized types return [TypeUnknown].
func InferType(v any) Type {
	switch v.(type) {
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
	case []any:
		return TypeSlice
	case map[string]any:
		return TypeMap
	case []byte:
		return TypeBytes
	default:
		return TypeUnknown
	}
}

// ComputeChecksum computes a deterministic SHA-256 checksum of the given
// value map. Keys are sorted before hashing to ensure consistent output
// regardless of map iteration order. A nil map produces the all-zeros hash.
func ComputeChecksum(data map[string]Value) string {
	if data == nil {
		return "0000000000000000000000000000000000000000000000000000000000000000"
	}
	keys := SortedKeys(data)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(data[k].String()))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Copy creates a shallow copy of the given value map. Both the map and its
// entries are copied to the new map. If src is nil, an empty map is returned.
func Copy(src map[string]Value) map[string]Value {
	if src == nil {
		return make(map[string]Value)
	}
	dst := make(map[string]Value, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// SortedKeys returns all keys in the map sorted in lexicographic order.
// Returns nil if the map is nil.
func SortedKeys(m map[string]Value) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// GetAs is a convenience function that retrieves a value from the state by
// key and attempts to type-assert it to T. Returns the value and true if the
// key exists and the assertion succeeds.
func GetAs[T any](s *State, key string) (T, bool) {
	v, ok := s.Get(key)
	if !ok {
		var zero T
		return zero, false
	}
	return As[T](v)
}
