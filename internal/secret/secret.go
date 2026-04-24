// Package secret provides utilities for identifying and redacting sensitive
// configuration values. It is used throughout the config system to prevent
// accidental leakage of passwords, tokens, API keys, and other secrets in
// logs, events, Explain() output, and snapshots.
//
// A key is considered a secret if its final path segment (after the last ".")
// matches one of the well-known secret suffixes:
//
//	.password, .token, .secret, .key, .apikey, .private_key,
//	.credentials, .passwd, .auth
//
// This is a conservative allowlist approach: any key ending with these
// suffixes will be redacted regardless of its value, preventing secrets
// from appearing in diagnostic output.
package secret

import "strings"

// RedactMask is the string used to replace secret values in output.
const RedactMask = "[REDACTED]"

// secretSuffixes are well-known secret suffixes (lowercase).
var secretSuffixes = [...]string{
	".password",
	".token",
	".secret",
	".key",
	".apikey",
	".private_key",
	".credentials",
	".passwd",
	".auth",
}

// IsSecret reports whether the given configuration key likely contains
// a sensitive value. The check is case-insensitive and matches the
// final suffix of the key.
//
// Examples:
//
//	IsSecret("db.password")     → true
//	IsSecret("auth_token")      → true (matches .token)
//	IsSecret("api.key")         → true
//	IsSecret("database.host")   → false
//	IsSecret("server.port")     → false
//	IsSecret("private_key_path") → true
func IsSecret(key string) bool {
	if key == "" {
		return false
	}
	lower := strings.ToLower(key)
	for _, suffix := range secretSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// RedactValue returns the original value unless the key is a secret,
// in which case it returns RedactMask. This is the single point of
// redaction logic used by all output paths (Explain, events, snapshots,
// log structured fields).
//
// Usage:
//
//	val := cfg.Get("db.host")
//	fmt.Printf("host=%s", secret.RedactValue("db.host", val.String()))
func RedactValue(key, val string) string {
	if IsSecret(key) {
		return RedactMask
	}
	return val
}
