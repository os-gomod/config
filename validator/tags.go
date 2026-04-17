package validator

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	gvalidator "github.com/go-playground/validator/v10"
)

// registerBuiltinTags registers all built-in custom validation tags
// with the validator instance.
func registerBuiltinTags(v *gvalidator.Validate) {
	_ = v.RegisterValidation("required_env", requiredEnvTag)
	_ = v.RegisterValidation("oneof_ci", oneofCITag)
	_ = v.RegisterValidation("duration", durationTag)
	_ = v.RegisterValidation("filepath", filepathTag)
	_ = v.RegisterValidation("urlhttp", urlHTTPTag)
}

// requiredEnvTag validates that an environment variable with the field name exists.
// The field value must be non-zero.
func requiredEnvTag(fl gvalidator.FieldLevel) bool {
	if fl.Field().IsZero() {
		return false
	}
	envName := fl.FieldName()
	return os.Getenv(envName) != ""
}

// oneofCITag performs a case-insensitive one-of validation.
// The param is a space-separated list of allowed values.
func oneofCITag(fl gvalidator.FieldLevel) bool {
	param := fl.Param()
	if param == "" {
		return false
	}
	vals := strings.Split(param, " ")
	fieldVal := fl.Field().String()
	for _, v := range vals {
		if strings.EqualFold(fieldVal, v) {
			return true
		}
	}
	return false
}

// durationTag validates that the field value is a valid Go duration string
// (e.g., "30s", "1h30m", "500ms").
func durationTag(fl gvalidator.FieldLevel) bool {
	s := fl.Field().String()
	_, err := time.ParseDuration(s)
	return err == nil
}

// filepathTag validates that the field value is a non-empty string that
// can be cleaned by filepath.Clean.
func filepathTag(fl gvalidator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return false
	}
	return filepath.Clean(s) != ""
}

// urlHTTPTag validates that the field value is a URL with an "http" or "https" scheme.
func urlHTTPTag(fl gvalidator.FieldLevel) bool {
	s := fl.Field().String()
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
