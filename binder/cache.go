package binder

import (
	"reflect"
	"sync"

	"github.com/os-gomod/config/internal/keyutil"
)

// fieldInfo holds pre-computed binding metadata for a single struct field.
type fieldInfo struct {
	// index is the field index in the parent reflect.Type.
	index int
	// configKey is the resolved config key (tag value or snake_case name).
	configKey string
	// defaultVal is the "default" tag value; empty if absent.
	defaultVal string
	// kind is the cached kind for fast dispatch in coerce.go.
	kind reflect.Kind
	// fieldType is the full type, needed for struct/slice/map recursion.
	fieldType reflect.Type
	// isNested is true if fieldType.Kind() == reflect.Struct.
	isNested bool
}

// resolveFields returns the cached []fieldInfo for t, building and caching
// it on first access. tagName is the struct tag key (e.g. "config").
func resolveFields(t reflect.Type, tagName string) []fieldInfo {
	if cached, ok := fieldCache.Load(t); ok {
		return cached.([]fieldInfo)
	}
	fields := buildFields(t, tagName)
	actual, _ := fieldCache.LoadOrStore(t, fields)
	return actual.([]fieldInfo)
}

// buildFields constructs the field info slice for the given type.
func buildFields(t reflect.Type, tagName string) []fieldInfo {
	var fields []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue // skip unexported fields
		}
		configTag := field.Tag.Get(tagName)
		if configTag == "" {
			configTag = keyutil.ToSnakeCase(field.Name)
		}
		fi := fieldInfo{
			index:      i,
			configKey:  configTag,
			defaultVal: field.Tag.Get("default"),
			kind:       field.Type.Kind(),
			fieldType:  field.Type,
			isNested:   field.Type.Kind() == reflect.Struct,
		}
		// Special case: time.Duration is an int64, not a nested struct.
		if field.Type.String() == "time.Duration" {
			fi.isNested = false
		}
		fields = append(fields, fi)
	}
	return fields
}

// Suppress unused import warning.
var _ sync.Once
