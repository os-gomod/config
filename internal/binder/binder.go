// Package binder provides infrastructure for binding configuration Values
// to Go structs using reflection. It supports type coercion, caching, and
// optional validation via pluggable validators.
package binder

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// Validator interface
// ---------------------------------------------------------------------------

// Validator validates a bound struct after binding.
type Validator interface {
	Validate(ctx context.Context, target any) error
}

// nopValidator is a no-op validator used when no validator is configured.
type nopValidator struct{}

func (n *nopValidator) Validate(_ context.Context, _ any) error { return nil }

// ---------------------------------------------------------------------------
// Binder
// ---------------------------------------------------------------------------

// Binder binds configuration Values to Go structs using reflection.
// It traverses struct fields, matches by "config" tag or field name,
// and converts Values to the target field types using the Coercer.
type Binder struct {
	cache     *Cache
	coerce    *Coercer
	validator Validator
	tagName   string // struct tag name for config key mapping (default: "config")
}

// BinderOption configures a Binder during construction.
type BinderOption func(*Binder)

// WithCache sets the cache for binding results.
func WithCache(c *Cache) BinderOption {
	return func(b *Binder) {
		b.cache = c
	}
}

// WithCoercer sets the type coercer.
func WithCoercer(c *Coercer) BinderOption {
	return func(b *Binder) {
		b.coerce = c
	}
}

// WithValidator sets the struct validator.
func WithValidator(v Validator) BinderOption {
	return func(b *Binder) {
		b.validator = v
	}
}

// WithTagName sets the struct tag name used for config key mapping.
func WithTagName(tag string) BinderOption {
	return func(b *Binder) {
		if tag != "" {
			b.tagName = tag
		}
	}
}

// New creates a new Binder with the given options.
func New(opts ...BinderOption) *Binder {
	b := &Binder{
		cache:     nil,
		coerce:    NewCoercer(),
		validator: &nopValidator{},
		tagName:   "config",
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Bind binds configuration data to the target struct. The target must be a
// pointer to a struct. Fields are matched by the "config" tag (or field name
// if no tag is present) and Values are coerced to the field type.
//
// After binding, if a validator is configured, it is called to validate
// the struct.
func (b *Binder) Bind(ctx context.Context, data map[string]value.Value, target any) error {
	if target == nil {
		return errors.New(errors.CodeInvalidConfig, "bind target must not be nil")
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New(errors.CodeInvalidConfig,
			"bind target must be a non-nil pointer to a struct")
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errors.New(errors.CodeInvalidConfig,
			"bind target must point to a struct")
	}

	rt := rv.Type()

	for i := range rt.NumField() {
		field := rt.Field(i)
		fieldVal := rv.Field(i)

		// Skip unexported fields.
		if !fieldVal.CanSet() {
			continue
		}

		// Determine config key from tag or field name.
		configKey := b.fieldConfigKey(field)
		if configKey == "-" {
			continue // explicitly skipped
		}

		v, exists := data[configKey]
		if !exists {
			// Try nested path: "parent.child" -> check if field is a struct
			// and bind nested data.
			if fieldVal.Kind() == reflect.Struct && !isPrimitiveKind(fieldVal.Kind()) {
				nestedKey := field.Name
				if tag := field.Tag.Get(b.tagName); tag != "" {
					parts := strings.Split(tag, ",")
					if parts[0] != "" {
						nestedKey = parts[0]
					}
				}
				if err := b.bindNested(ctx, data, nestedKey, fieldVal); err != nil {
					return err
				}
			}
			continue
		}

		// Coerce the Value to the field type.
		coerced, err := b.coerce.Coerce(v, field.Type)
		if err != nil {
			return errors.Wrap(err, errors.CodeBind,
				fmt.Sprintf("failed to bind key %q to field %s.%s",
					configKey, rt.Name(), field.Name)).
				WithKey(configKey).
				WithOperation("bind.coerce")
		}

		fieldVal.Set(coerced)
	}

	// Run validation if configured.
	if b.validator != nil {
		if err := b.validator.Validate(ctx, target); err != nil {
			return errors.Wrap(err, errors.CodeValidation,
				"validation failed after binding").
				WithOperation("bind.validate")
		}
	}

	return nil
}

// bindNested handles binding nested struct fields with a key prefix.
func (b *Binder) bindNested(_ context.Context, data map[string]value.Value, prefix string, rv reflect.Value) error {
	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		fieldVal := rv.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		nestedKey := prefix + "." + b.fieldConfigKey(field)
		if nestedKey == prefix+"-" || nestedKey == prefix+"." {
			continue
		}

		v, exists := data[nestedKey]
		if !exists {
			continue
		}

		coerced, err := b.coerce.Coerce(v, field.Type)
		if err != nil {
			return errors.Wrap(err, errors.CodeBind,
				fmt.Sprintf("failed to bind nested key %q to field %s.%s",
					nestedKey, rt.Name(), field.Name)).
				WithKey(nestedKey).
				WithOperation("bind.nested_coerce")
		}

		fieldVal.Set(coerced)
	}
	return nil
}

// fieldConfigKey extracts the config key from a struct field's tag.
// Falls back to the lowercase field name if no tag is present.
//
//nolint:gocritic // fieldConfigKey is a simple helper that doesn't warrant the complexity of a full struct tag parser.
func (b *Binder) fieldConfigKey(field reflect.StructField) string {
	tag := field.Tag.Get(b.tagName)
	if tag == "" {
		return strings.ToLower(field.Name)
	}

	// Handle "key,opt1,opt2" format.
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		return strings.ToLower(field.Name)
	}
	return parts[0]
}

// isPrimitiveKind returns true for non-struct types that should not be
// recursed into during nested binding.
func isPrimitiveKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Struct:
		// Time is a struct but should be treated as a primitive.
		var zero time.Time
		return reflect.TypeOf(zero).Kind() == kind
	default:
		return true
	}
}
