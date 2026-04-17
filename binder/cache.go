package binder

import (
	"reflect"

	"github.com/os-gomod/config/internal/keyutil"
)

// fieldInfo holds pre-computed reflection metadata for a struct field,
// used by the binder to efficiently map config keys to struct fields.
type fieldInfo struct {
	index      int
	configKey  string
	defaultVal string
	kind       reflect.Kind
	fieldType  reflect.Type
	isNested   bool
}

// resolveFields returns field metadata for the given type, using a sync.Map cache
// to avoid redundant reflection on repeated binds of the same type.
func resolveFields(t reflect.Type, tagName string) []fieldInfo {
	if cached, ok := fieldCache.Load(t); ok {
		return cached.([]fieldInfo)
	}
	fields := buildFields(t, tagName)
	actual, _ := fieldCache.LoadOrStore(t, fields)
	return actual.([]fieldInfo)
}

// buildFields extracts fieldInfo for all exported fields of the struct type.
// Unexported fields (those with a PkgPath) are skipped.
// The config key is determined by the "config" tag, falling back to snake_case
// conversion of the field name.
func buildFields(t reflect.Type, tagName string) []fieldInfo {
	var fields []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
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
		if field.Type.String() == "time.Duration" {
			fi.isNested = false
		}
		fields = append(fields, fi)
	}
	return fields
}
