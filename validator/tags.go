package validator

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	gvalidator "github.com/go-playground/validator/v10"
)

// registerBuiltinTags registers config-specific custom validation tags.
func registerBuiltinTags(v *gvalidator.Validate) {
	_ = v.RegisterValidation("required_env", requiredEnvTag)
	_ = v.RegisterValidation("oneof_ci", oneofCITag)
	_ = v.RegisterValidation("duration", durationTag)
	_ = v.RegisterValidation("filepath", filepathTag)
	_ = v.RegisterValidation("urlhttp", urlHTTPTag)
}

// requiredEnvTag validates that the field is non-zero AND the environment
// variable named after the field is non-empty.
func requiredEnvTag(fl gvalidator.FieldLevel) bool {
	if fl.Field().IsZero() {
		return false
	}
	envName := fl.FieldName()
	return os.Getenv(envName) != ""
}

// oneofCITag validates that the field value is one of the specified values,
// using case-insensitive comparison.
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

// durationTag validates that the field is a valid time.Duration string.
func durationTag(fl gvalidator.FieldLevel) bool {
	s := fl.Field().String()
	_, err := time.ParseDuration(s)
	return err == nil
}

// filepathTag validates that the field is a non-empty, valid file path.
func filepathTag(fl gvalidator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return false
	}
	return filepath.Clean(s) != ""
}

// urlHTTPTag validates that the field parses as an HTTP or HTTPS URL.
func urlHTTPTag(fl gvalidator.FieldLevel) bool {
	s := fl.Field().String()
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
