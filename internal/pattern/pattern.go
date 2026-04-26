// Package pattern provides lightweight glob-style matching for event keys.
//
// Supported patterns:
//   - "*"            matches everything (catch-all)
//   - "prefix.*"     matches keys starting with "prefix."
//   - "exact.key"    matches only that exact key
//   - "app.*.config" matches "app.db.config", "app.cache.config", etc.
//   - "*.changed"    matches "config.changed", "secrets.changed", etc.
package pattern

import "strings"

// Match checks if a key matches a pattern using simple glob rules.
// The only wildcard character is '*' which matches exactly one
// dot-separated segment (not an arbitrary substring).
func Match(key, pat string) bool {
	if pat == "*" || pat == "" {
		return true
	}
	if key == pat {
		return true
	}

	// Split both on '.'
	keyParts := strings.Split(key, ".")
	patParts := strings.Split(pat, ".")

	return globMatch(keyParts, patParts)
}

// globMatch checks if key segments match pattern segments.
// '*' in the pattern matches exactly one segment.
func globMatch(keyParts, patParts []string) bool {
	if len(keyParts) != len(patParts) {
		return false
	}

	for i := range patParts {
		if patParts[i] == "*" {
			continue
		}
		if patParts[i] != keyParts[i] {
			return false
		}
	}
	return true
}
