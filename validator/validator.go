// Package validator provides a tag-driven, extensible struct validation engine
// for use after config binding. It is backed by go-playground/validator/v10
// and augmented with config-specific custom tags defined in tags.go.
package validator

import (
	"context"
	"fmt"
	"strings"

	gvalidator "github.com/go-playground/validator/v10"

	_errors "github.com/os-gomod/config/errors"
)

// Validator validates a bound config struct.
type Validator interface {
	// Validate checks v against all registered validation rules.
	// Returns *ValidationErrors if any field fails; nil on success.
	Validate(ctx context.Context, v any) error
}

// Engine is the default Validator backed by go-playground/validator/v10.
// It is safe for concurrent use after construction.
type Engine struct {
	v *gvalidator.Validate
}

var _ Validator = (*Engine)(nil)

// New returns an Engine with all built-in custom tags pre-registered.
func New(opts ...Option) *Engine {
	v := gvalidator.New()
	registerBuiltinTags(v)
	for _, opt := range opts {
		opt(v)
	}
	return &Engine{v: v}
}

// RegisterValidation adds a named validation function to the engine.
func (e *Engine) RegisterValidation(tag string, fn gvalidator.Func) error {
	if e == nil || e.v == nil {
		return _errors.New(_errors.CodeValidation, "validator engine is not initialized")
	}
	return e.v.RegisterValidation(tag, fn)
}

// Option configures an Engine.
type Option func(*gvalidator.Validate)

// WithCustomTag registers a named validation function.
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

// Validate implements Validator. It validates the struct v against all
// registered rules, returning *ValidationErrors on failure.
func (e *Engine) Validate(_ context.Context, v any) error {
	if err := e.v.Struct(v); err != nil {
		if ve, ok := err.(gvalidator.ValidationErrors); ok {
			return translateErrors(ve)
		}
		return _errors.Wrap(err, _errors.CodeValidation, "validation failed")
	}
	return nil
}

// ValidationErrors is returned when one or more fields fail validation.
type ValidationErrors struct {
	// Fields contains one entry per failing validation constraint.
	Fields []FieldError
}

// Error implements the error interface.
func (ve *ValidationErrors) Error() string {
	if len(ve.Fields) == 0 {
		return "validation failed"
	}
	var sb strings.Builder
	sb.WriteString("validation failed: ")
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

// AsError returns ve if len(Fields) > 0, otherwise nil.
// This allows callers to write: return ve.AsError().
func (ve *ValidationErrors) AsError() error {
	if len(ve.Fields) > 0 {
		return ve
	}
	return nil
}

// FieldError describes a single field validation failure.
type FieldError struct {
	// Field is the struct field name that failed validation.
	Field string
	// Tag is the failing validation tag.
	Tag string
	// Value is the offending value.
	Value any
	// Message is a human-readable description of the failure.
	Message string
}

// translateErrors converts go-playground/validator errors to ValidationErrors.
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
