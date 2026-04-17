// Package keyutil provides key manipulation utilities for flattening nested maps,
// normalizing key casing, and converting between naming conventions.
//
// The FlattenMap function is the single source of truth for recursive map
// flattening, used by both the decoder package (with lowercase=true) and
// the provider packages (preserving original casing).
package keyutil

import (
	"strings"
	"unicode"
)

// FlattenMap recursively flattens a nested map[string]any into a dot-separated
// flat map. Original key casing is preserved. Nil maps return an empty map.
//
// Example:
//
//	input := map[string]any{"db": map[string]any{"host": "localhost", "port": 5432}}
//	output := keyutil.FlattenMap(input)
//	// output["db.host"] = "localhost"
//	// output["db.port"] = 5432
func FlattenMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(m))
	flattenInto(result, m, "", false)
	return result
}

// FlattenMapLower recursively flattens a nested map[string]any into a dot-separated
// flat map with all keys lowercased. This is used by the decoder package to normalize
// configuration keys regardless of source format casing.
//
// Example:
//
//	input := map[string]any{"Server": map[string]any{"Host": "localhost"}}
//	output := keyutil.FlattenMapLower(input)
//	// output["server.host"] = "localhost"
func FlattenMapLower(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(m))
	flattenInto(result, m, "", true)
	return result
}

// flattenInto is the internal recursive flattening implementation.
// When lowercase is true, all keys are lowercased during flattening.
func flattenInto(dst, src map[string]any, prefix string, lowercase bool) {
	for k, v := range src {
		key := k
		if lowercase {
			key = strings.ToLower(k)
		}
		if prefix != "" {
			key = prefix + "." + key
		}
		switch val := v.(type) {
		case map[string]any:
			flattenInto(dst, val, key, lowercase)
		case map[any]any:
			converted := make(map[string]any, len(val))
			for mk, mv := range val {
				if ks, ok := mk.(string); ok {
					converted[ks] = mv
				}
			}
			flattenInto(dst, converted, key, lowercase)
		default:
			dst[key] = v
		}
	}
}

// FlattenProviderKey transforms a hierarchical key from a remote provider
// (e.g. consul/etcd path-style keys) into a dot-separated flat key.
// It strips the prefix, replaces "/" with ".", lowercases, and trims dots.
//
// Example:
//
//	key := keyutil.FlattenProviderKey("config/db/host", "config/")
//	// key == "db.host"
func FlattenProviderKey(key, prefix string) string {
	key = strings.TrimPrefix(key, prefix)
	key = strings.ReplaceAll(key, "/", ".")
	key = strings.ToLower(key)
	key = strings.Trim(key, ".")
	return key
}

// ToSnakeCase converts a CamelCase or camelCase string to snake_case.
// It handles sequences of uppercase letters correctly (e.g., "HTTPServer" -> "http_server").
//
// Example:
//
//	keyutil.ToSnakeCase("HTTPServer")  // "http_server"
//	keyutil.ToSnakeCase("dbHost")      // "db_host"
//	keyutil.ToSnakeCase("simple")      // "simple"
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

// NormalizeKey returns the key in lowercase with leading/trailing whitespace removed.
func NormalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
