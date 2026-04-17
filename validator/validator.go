// Package validator provides configuration validation using the go-playground/validator
// library. It supports custom validation tags, struct-level validation, and
// built-in validation tags tailored for configuration values.
//
// Built-in validation tags:
//   - required_env: validates that an environment variable with the field name exists
//   - oneof_ci: case-insensitive one-of validation
//   - duration: validates that a string is a valid Go duration
//   - filepath: validates that a string is a valid file path
//   - urlhttp: validates that a string is an HTTP or HTTPS URL
package validator

import (
	"context"
	"fmt"
	"strings"

	gvalidator "github.com/go-playground/validator/v10"

	configerrors "github.com/os-gomod/config/errors"
)

// Validator is the interface for configuration validators.
type Validator interface {
	// Validate validates the given value and returns an error if validation fails.
	Validate(ctx context.Context, v any) error
}

// Engine wraps a go-playground/validator instance with custom validation tags
// and error translation. It implements the Validator interface.
type Engine struct {
	v *gvalidator.Validate
}

var _ Validator = (*Engine)(nil)

// New creates a new validation Engine with the given options.
// Built-in validation tags are registered automatically.
func New(opts ...Option) *Engine {
	v := gvalidator.New()
	registerBuiltinTags(v)
	for _, opt := range opts {
		opt(v)
	}
	return &Engine{v: v}
}

// RegisterValidation registers a custom validation function for the given tag.
// Returns an error if the engine is not initialized.
func (e *Engine) RegisterValidation(tag string, fn gvalidator.Func) error {
	if e == nil || e.v == nil {
		return configerrors.New(configerrors.CodeValidation, "validator engine is not initialized")
	}
	return e.v.RegisterValidation(tag, fn)
}

// Option configures a validation Engine.
type Option func(*gvalidator.Validate)

// WithCustomTag registers a custom validation tag function during engine creation.
// Errors during registration are silently ignored.
func WithCustomTag(tag string, fn gvalidator.Func) Option {
	return func(v *gvalidator.Validate) {
		_ = v.RegisterValidation(tag, fn)
	}
}

// WithStructLevel registers a struct-level validation function for the given types.
func WithStructLevel(fn gvalidator.StructLevelFunc, types ...any) Option {
	return func(v *gvalidator.Validate) {
		v.RegisterStructValidation(fn, types...)
	}
}

// Validate runs the validator against the given value (typically a struct pointer).
// Validation errors are translated into a ValidationErrors with per-field details.
func (e *Engine) Validate(_ context.Context, v any) error {
	if err := e.v.Struct(v); err != nil {
		if ve, ok := err.(gvalidator.ValidationErrors); ok {
			return translateErrors(ve)
		}
		return configerrors.Wrap(err, configerrors.CodeValidation, "❌ Validation failed")
	}
	return nil
}

// ValidationErrors aggregates multiple field validation errors into a single error.
// It implements the error interface.
type ValidationErrors struct {
	Fields []FieldError
}

// Error returns a human-readable string listing all field validation failures.
func (ve *ValidationErrors) Error() string {
	if len(ve.Fields) == 0 {
		return "❌ Validation failed"
	}
	var sb strings.Builder
	sb.WriteString("❌ Validation failed: ")
	for i, f := range ve.Fields {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(f.Field)
		sb.WriteString(" ")
		sb.WriteString(f.Tag)
		if f.Message != "" {
			sb.WriteString(": ")
			sb.WriteString(f.Message)
		}
	}
	return sb.String()
}

// AsError returns this ValidationErrors as an error if there are field errors,
// or nil if there are none.
func (ve *ValidationErrors) AsError() error {
	if len(ve.Fields) > 0 {
		return ve
	}
	return nil
}

// FieldError describes a single field validation failure.
type FieldError struct {
	Field   string
	Tag     string
	Value   any
	Message string
}

// translateErrors converts go-playground/validator validation errors into
// ValidationErrors with per-field details.
func translateErrors(errs gvalidator.ValidationErrors) *ValidationErrors {
	ve := &ValidationErrors{}
	for _, e := range errs {
		fe := FieldError{
			Field: e.Field(),
			Tag:   e.Tag(),
			Value: e.Value(),
		}
		fe.Message = fmt.Sprintf("field %s failed %s validation", e.Field(), e.Tag())
		ve.Fields = append(ve.Fields, fe)
	}
	return ve
}
