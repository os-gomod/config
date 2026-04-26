// Package validator provides infrastructure for validating configuration
// structs after binding. It wraps go-playground/validator/v10 with the
// domain error system. All instances are created via constructors —
// NO global validators.
package validator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gvalidator "github.com/go-playground/validator/v10"

	apperrors "github.com/os-gomod/config/v2/internal/domain/errors"
)

// ---------------------------------------------------------------------------
// Validator interface
// ---------------------------------------------------------------------------

// Validator validates configuration structs after binding.
type Validator interface {
	// Validate validates the target struct. Returns a structured error
	// containing all validation failures.
	Validate(ctx context.Context, target any) error
	// RegisterValidation registers a custom validation function for the given tag.
	RegisterValidation(tag string, fn gvalidator.Func) error
}

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// Engine is the concrete implementation of Validator. It wraps
// go-playground/validator with domain-specific error formatting.
// Each Engine instance is independent — NO global state.
type Engine struct {
	validate *gvalidator.Validate
}

// New creates a new validation Engine with default settings.
// Custom validation tags are NOT registered automatically; use
// RegisterDefaultTags to add them.
func New() *Engine {
	return &Engine{
		validate: gvalidator.New(),
	}
}

// NewWithTags creates a new validation Engine with default custom tags registered.
func NewWithTags() *Engine {
	e := New()
	RegisterDefaultTags(e)
	return e
}

// Validate validates the target struct using go-playground/validator.
// All validation errors are collected and wrapped in a single domain AppError.
func (e *Engine) Validate(ctx context.Context, target any) error {
	select {
	case <-ctx.Done():
		return apperrors.New(apperrors.CodeContextCanceled, "validation cancelled").
			WithOperation("validate")
	default:
	}

	if target == nil {
		return apperrors.New(apperrors.CodeInvalidConfig, "validation target must not be nil").
			WithOperation("validate")
	}

	err := e.validate.Struct(target)
	if err == nil {
		return nil
	}

	// Handle validation errors.
	validationErrs, ok := apperrors.AsConfigError[gvalidator.ValidationErrors](err)
	if !ok {
		// Not a validation error — wrap as internal error.
		return apperrors.Wrap(err, apperrors.CodeInternal,
			"unexpected validation error").
			WithOperation("validate")
	}

	// Collect all validation errors into a single message.
	var msgs []string
	for _, ve := range validationErrs {
		msgs = append(msgs, formatValidationError(ve))
	}

	return apperrors.New(apperrors.CodeValidation,
		"validation failed: "+strings.Join(msgs, "; ")).
		WithOperation("validate")
}

// RegisterValidation registers a custom validation function for the given tag.
func (e *Engine) RegisterValidation(tag string, fn gvalidator.Func) error {
	if tag == "" {
		return apperrors.New(apperrors.CodeInvalidConfig, "validation tag must not be empty")
	}
	if fn == nil {
		return apperrors.New(apperrors.CodeInvalidConfig,
			fmt.Sprintf("validation function for tag %q must not be nil", tag))
	}
	return e.validate.RegisterValidation(tag, fn)
}

// RegisterStructValidation registers a struct-level validation function.
func (e *Engine) RegisterStructValidation(fn gvalidator.StructLevelFunc, target any) {
	e.validate.RegisterStructValidation(fn, target)
}

// ValidateValue validates a single field value against a tag.
func (e *Engine) ValidateValue(field string, value any, tag string) error {
	err := e.validate.VarWithValue(field, value, tag)
	if err == nil {
		return nil
	}
	var ve gvalidator.ValidationErrors
	if errors.As(err, &ve) {
		return apperrors.New(apperrors.CodeValidation,
			formatValidationError(ve[0])).
			WithKey(field).
			WithOperation("validate_value")
	}
	return apperrors.Wrap(err, apperrors.CodeValidation,
		fmt.Sprintf("validate value for %q failed", field)).
		WithKey(field)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// formatValidationError formats a single validation error into a human-readable
// string with field, tag, and value context.
func formatValidationError(ve gvalidator.FieldError) string {
	var b strings.Builder

	field := ve.Field()
	ns := ve.Namespace()
	if ns != "" {
		// Use the short name (last segment of namespace).
		parts := strings.Split(ns, ".")
		if len(parts) > 0 {
			field = parts[len(parts)-1]
		}
	}

	b.WriteString(field)
	b.WriteString(": ")
	b.WriteString(formatTag(ve.Tag()))

	// Include the parameter if present.
	if ve.Param() != "" {
		b.WriteString("=")
		b.WriteString(ve.Param())
	}

	// Include the actual value for better debugging.
	rv := ve.Value()
	if rv != nil {
		b.WriteString(" (got: ")
		fmt.Fprintf(&b, "%v", rv)
		b.WriteString(")")
	}

	return b.String()
}

// formatTag converts a validator tag to a more readable description.
func formatTag(tag string) string {
	switch tag {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "url":
		return "must be a valid URL"
	case "min":
		return "minimum value violated"
	case "max":
		return "maximum value violated"
	case "len":
		return "length requirement violated"
	case "gt":
		return "must be greater than"
	case "gte":
		return "must be greater than or equal to"
	case "lt":
		return "must be less than"
	case "lte":
		return "must be less than or equal to"
	case "oneof":
		return "must be one of the allowed values"
	case "uuid":
		return "must be a valid UUID"
	case "ip":
		return "must be a valid IP address"
	case "hostname":
		return "must be a valid hostname"
	case "port":
		return "must be a valid port number"
	default:
		return "failed validation (" + tag + ")"
	}
}
