package validator

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	gvalidator "github.com/go-playground/validator/v10"
)

// RegisterDefaultTags registers custom validation tags that are useful
// for configuration validation. These extend go-playground/validator with
// config-specific rules.
//
// Registered tags:
//   - "duration"  — validates that a string is a valid Go duration
//   - "port"      — validates that a value is a valid TCP/UDP port (1-65535)
//   - "absurl"    — validates that a string is an absolute URL with a scheme
//   - "envprefix" — validates that a string looks like an env var prefix (uppercase, ends with _)
func RegisterDefaultTags(e *Engine) {
	_ = e.RegisterValidation("duration", validateDuration)
	_ = e.RegisterValidation("port", validatePort)
	_ = e.RegisterValidation("absurl", validateAbsURL)
	_ = e.RegisterValidation("envprefix", validateEnvPrefix)
}

// validateDuration checks that a field value is a valid Go duration string.
func validateDuration(fl gvalidator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "" {
		return true // use "required" for mandatory fields
	}
	_, err := time.ParseDuration(val)
	return err == nil
}

// validatePort checks that a field value is a valid port number (1-65535).
func validatePort(fl gvalidator.FieldLevel) bool {
	switch v := fl.Field().Interface().(type) {
	case int:
		return v >= 1 && v <= 65535
	case int64:
		return v >= 1 && v <= 65535
	case uint:
		return v >= 1 && v <= 65535
	case uint16:
		return v >= 1
	case string:
		port, err := net.DefaultResolver.LookupPort(context.Background(), "tcp", v)
		return err == nil && port >= 1
	default:
		return false
	}
}

// validateAbsURL checks that a string is an absolute URL with a scheme.
func validateAbsURL(fl gvalidator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "" {
		return true
	}
	u, err := url.Parse(val)
	if err != nil {
		return false
	}
	return u.Scheme != "" && (u.Host != "" || u.Path != "")
}

// validateEnvPrefix checks that a string looks like an environment variable prefix:
// uppercase, alphanumeric with underscores, typically ending with underscore.
func validateEnvPrefix(fl gvalidator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "" {
		return true
	}
	if len(val) < 2 {
		return false
	}
	for _, c := range val {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Additional custom validators (standalone functions)
// ---------------------------------------------------------------------------

// IsPort checks if a string is a valid port number.
func IsPort(val string) bool {
	port, err := net.DefaultResolver.LookupPort(context.Background(), "tcp", val)
	return err == nil && port >= 1
}

// IsAbsURL checks if a string is an absolute URL.
func IsAbsURL(val string) bool {
	u, err := url.Parse(val)
	if err != nil {
		return false
	}
	return u.Scheme != "" && (u.Host != "" || u.Path != "")
}

// IsEnvPrefix checks if a string is a valid environment variable prefix.
func IsEnvPrefix(val string) bool {
	if val == "" || len(val) < 2 {
		return false
	}
	return strings.ToUpper(val) == val && isAlphaNumericUnderscore(val)
}

func isAlphaNumericUnderscore(s string) bool {
	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}
