// Package binder provides reflection-based struct binding for config values.
// It maps a flat dot-separated key-value store onto annotated Go structs,
// applying default values and running optional validation after binding.
package binder

import (
	"context"
	"reflect"
	"sync"

	"github.com/os-gomod/config/core/value"
	configerrors "github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/validator"
)

// Binder binds a flat key-value map to an annotated Go struct.
type Binder interface {
	// Bind populates target from data.
	// target must be a non-nil pointer to a struct.
	// Default tag values are applied for absent keys.
	// If a Validator was configured, it is called after all fields are set.
	Bind(ctx context.Context, data map[string]value.Value, target any) error
}

// StructBinder is the default Binder implementation.
// It uses reflection with a per-type field-map cache to eliminate repeated
// struct traversal on hot-reload paths.
//
// Lock ordering: cacheMu is the only lock held by StructBinder.
type StructBinder struct {
	tagName   string
	validator validator.Validator // nil = skip validation
}

var _ Binder = (*StructBinder)(nil)

// New returns a StructBinder.
func New(opts ...Option) *StructBinder {
	b := &StructBinder{tagName: "config"}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Option configures a StructBinder.
type Option func(*StructBinder)

// WithTagName sets the struct tag key used for config key lookup. Default: "config".
func WithTagName(tag string) Option {
	return func(b *StructBinder) { b.tagName = tag }
}

// WithValidator sets the validator called after successful binding.
// Pass nil to disable validation.
func WithValidator(v validator.Validator) Option {
	return func(b *StructBinder) { b.validator = v }
}

// Bind implements Binder.
// It resolves the field map from cache on the second and subsequent calls
// for the same reflect.Type, eliminating struct traversal overhead on
// hot-reload paths.
func (b *StructBinder) Bind(ctx context.Context, data map[string]value.Value, target any) error {
	if target == nil {
		return configerrors.New(configerrors.CodeBind, "target cannot be nil")
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return configerrors.New(configerrors.CodeBind, "target must be a non-nil pointer to struct")
	}
	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return configerrors.New(configerrors.CodeBind, "target must be a non-nil pointer to struct")
	}
	if err := b.bindStruct(elem, data, ""); err != nil {
		return err
	}
	if b.validator != nil {
		if err := b.validator.Validate(ctx, target); err != nil {
			return err
		}
	}
	return nil
}

// bindStruct walks the struct fields and sets values from data.
func (b *StructBinder) bindStruct(
	rv reflect.Value,
	data map[string]value.Value,
	prefix string,
) error {
	fields := resolveFields(rv.Type(), b.tagName)
	for _, fi := range fields {
		fv := rv.Field(fi.index)
		key := fi.configKey
		if prefix != "" {
			key = prefix + "." + fi.configKey
		}
		v, ok := data[key]
		if !ok {
			if fi.defaultVal != "" {
				v = value.FromRaw(fi.defaultVal)
				ok = true
			}
		}
		if !ok {
			if fi.isNested {
				if err := b.bindStruct(fv, data, key); err != nil {
					return err
				}
			}
			continue
		}
		if err := b.setField(fv, v); err != nil {
			return configerrors.Wrap(err, configerrors.CodeBind, "bind field").WithKey(key)
		}
	}
	return nil
}

// setField dispatches to the appropriate coerce function based on field kind.
func (b *StructBinder) setField(fv reflect.Value, v value.Value) error {
	if !fv.CanSet() {
		return nil
	}
	raw := v.Raw()
	if handled, err := coerceDuration(fv, raw); handled {
		return err
	}
	switch fv.Kind() {
	case reflect.String:
		coerceString(fv, raw)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return coerceInt(fv, v, raw)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		coerceUint(fv, v)
		return nil
	case reflect.Float32, reflect.Float64:
		return coerceFloat(fv, v, raw)
	case reflect.Bool:
		return coerceBool(fv, v, raw)
	case reflect.Slice:
		return coerceSlice(fv, v, b)
	case reflect.Map:
		return coerceMap(fv, v, b)
	default:
		return coerceFallback(fv, raw)
	}
}

// fieldCache stores []fieldInfo slices keyed by reflect.Type.
// sync.Map provides lock-free reads once a type is populated.
//
// This is the only permitted package-level variable in binder/.
var fieldCache sync.Map // map[reflect.Type][]fieldInfo
