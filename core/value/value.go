// Package value defines the immutable Value type that carries a single config datum,
// its type, source, and merge priority.
package value

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"time"
)

// Type represents the inferred Go type of a config value.
type Type uint8

const (
	// TypeUnknown indicates the type could not be inferred.
	TypeUnknown Type = iota
	// TypeString indicates a string value.
	TypeString
	// TypeInt indicates an int value.
	TypeInt
	// TypeInt64 indicates an int64 value.
	TypeInt64
	// TypeFloat64 indicates a float64 value.
	TypeFloat64
	// TypeBool indicates a bool value.
	TypeBool
	// TypeDuration indicates a time.Duration value.
	TypeDuration
	// TypeTime indicates a time.Time value.
	TypeTime
	// TypeSlice indicates a []any value.
	TypeSlice
	// TypeMap indicates a map[string]any value.
	TypeMap
	// TypeBytes indicates a []byte value.
	TypeBytes
)

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

// Source represents the origin of a config value.
type Source uint8

const (
	// SourceUnknown indicates the source is unspecified.
	SourceUnknown Source = iota
	// SourceFile indicates the value originated from a file.
	SourceFile
	// SourceEnv indicates the value originated from an environment variable.
	SourceEnv
	// SourceMemory indicates the value was set in memory.
	SourceMemory
	// SourceHTTP indicates the value was fetched via HTTP.
	SourceHTTP
	// SourceKubernetes indicates the value originated from a Kubernetes ConfigMap or Secret.
	SourceKubernetes
	// SourceVault indicates the value was fetched from HashiCorp Vault.
	SourceVault
	// SourceKMS indicates the value was fetched from a KMS provider.
	SourceKMS
	// SourceRemote indicates the value was fetched from a remote config service.
	SourceRemote
)

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

// Value is an immutable container for a single config datum. It carries the raw
// value, its inferred type, its source, and its merge priority.
type Value struct {
	raw      any    // the underlying Go value
	typ      Type   // inferred type of raw
	src      Source // origin of the value
	priority int    // merge priority; higher values win
}

// New creates a Value with explicit type, source, and priority.
func New(raw any, typ Type, src Source, priority int) Value {
	return Value{raw: raw, typ: typ, src: src, priority: priority}
}

// NewInMemory creates a Value with SourceMemory, inferred type, and priority 100.
func NewInMemory(raw any) Value {
	return Value{raw: raw, typ: InferType(raw), src: SourceMemory, priority: 100}
}

// FromRaw creates a Value with SourceUnknown and inferred type.
func FromRaw(raw any) Value {
	return Value{raw: raw, typ: InferType(raw), src: SourceUnknown}
}

// Raw returns the underlying Go value.
func (v Value) Raw() any { return v.raw }

// Type returns the inferred Type of the value.
func (v Value) Type() Type { return v.typ }

// Source returns the Source of the value.
func (v Value) Source() Source { return v.src }

// Priority returns the merge priority of the value.
func (v Value) Priority() int { return v.priority }

// String returns a human-readable representation of the raw value.
func (v Value) String() string {
	if v.raw == nil {
		return ""
	}
	return fmt.Sprintf("%v", v.raw)
}

// Bool returns the value as a bool. The second return indicates whether the
// underlying value was actually a bool.
func (v Value) Bool() (bool, bool) {
	b, ok := v.raw.(bool)
	return b, ok
}

// Int returns the value as an int, with narrowing from int64 or float64.
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

// Int64 returns the value as an int64, with narrowing from int or float64.
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

// Float64 returns the value as a float64, with widening from int or int64.
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

// Duration returns the value as a time.Duration. It accepts time.Duration,
// int64 (nanoseconds), or a string parseable by time.ParseDuration.
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

// Time returns the value as a time.Time if the underlying value is a time.Time.
func (v Value) Time() (time.Time, bool) {
	t, ok := v.raw.(time.Time)
	return t, ok
}

// Slice returns the value as a []any if the underlying value is a slice.
func (v Value) Slice() ([]any, bool) {
	s, ok := v.raw.([]any)
	return s, ok
}

// Map returns the value as a map[string]any if the underlying value is a map.
func (v Value) Map() (map[string]any, bool) {
	m, ok := v.raw.(map[string]any)
	return m, ok
}

// Bytes returns the value as a []byte. String values are converted to []byte.
func (v Value) Bytes() ([]byte, bool) {
	switch val := v.raw.(type) {
	case []byte:
		return val, true
	case string:
		return []byte(val), true
	}
	return nil, false
}

// Equal reports whether two Values have the same type and raw value, as
// determined by reflect.DeepEqual.
func (v Value) Equal(other Value) bool {
	if v.typ != other.typ {
		return false
	}
	return reflect.DeepEqual(v.raw, other.raw)
}

// IsZero reports whether the Value's raw value is nil.
func (v Value) IsZero() bool { return v.raw == nil }

// As performs a type assertion on the Value's raw value, returning the
// typed result and whether the assertion succeeded.
func As[T any](v Value) (T, bool) {
	if v.raw == nil {
		var zero T
		return zero, false
	}
	res, ok := v.raw.(T)
	return res, ok
}

// InferType returns the Type that best describes the Go value v.
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

// ComputeChecksum returns a deterministic SHA-256 hex digest of the data map.
// Keys are sorted before hashing to ensure a stable output.
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

// Copy creates a shallow copy of the src map. Individual Value structs are
// copied by value (they are immutable), so the copy is safe for mutation.
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

// SortedKeys returns the keys of the map in lexicographic order.
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

// GetAs retrieves a value from State by key and performs a type assertion.
func GetAs[T any](s *State, key string) (T, bool) {
	v, ok := s.Get(key)
	if !ok {
		var zero T
		return zero, false
	}
	return As[T](v)
}
