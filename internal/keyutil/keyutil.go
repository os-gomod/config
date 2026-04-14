// Package keyutil provides key normalisation helpers, including snake_case conversion.
package keyutil

import (
	"strings"
	"unicode"
)

// FlattenMap recursively flattens a nested map into dot-separated keys.
// Nested map[string]any and map[any]any values are traversed; all other
// values are stored as leaf entries.
func FlattenMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(m))
	flattenInto(result, m, "")
	return result
}

// flattenInto walks the source map, prefixing keys with the given path.
func flattenInto(dst, src map[string]any, prefix string) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flattenInto(dst, val, key)
		case map[any]any:
			converted := make(map[string]any, len(val))
			for mk, mv := range val {
				if ks, ok := mk.(string); ok {
					converted[ks] = mv
				}
			}
			flattenInto(dst, converted, key)
		default:
			dst[key] = v
		}
	}
}

// ToSnakeCase converts a CamelCase or mixedCaps string to snake_case.
// Adjacent uppercase runes are grouped, and underscores are inserted
// before a transition from lower to upper or upper-lower to upper.
func ToSnakeCase(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			if unicode.IsLower(prev) ||
				(unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// NormalizeKey lowercases and trims whitespace from a config key.
func NormalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
