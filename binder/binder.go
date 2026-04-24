// Package binder provides configuration binding functionality that maps flat
// key-value configuration data to Go structs. It supports type coercion,
// nested struct binding via dot-separated keys, default values via struct tags,
// and optional validation integration.
//
// The default struct tag is "config", but can be customized. Keys are matched
// against the tag value or the snake_cased field name. Fields with a "default"
// tag receive that value when no matching config key exists.
//
// Example:
//
//	type Config struct {
//	    Host     string        `config:"db.host"`
//	    Port     int           `config:"db.port"`
//	    Timeout  time.Duration `config:"db.timeout" default:"30s"`
//	    Enabled  bool          `config:"enabled" default:"true"`
//	}
//	var cfg Config
//	binder := binder.New()
//	err := binder.Bind(ctx, data, &cfg)
package binder

import (
	"context"
	"reflect"
	"sync"

	"github.com/os-gomod/config/core/value"
	configerrors "github.com/os-gomod/config/errors"
	"github.com/os-gomod/config/validator"
)

// Binder is the interface for binding configuration data to target structs.
type Binder interface {
	// Bind maps configuration data to the target struct pointer.
	Bind(ctx context.Context, data map[string]value.Value, target any) error
}

// StructBinder binds flat key-value configuration data to Go structs using
// reflection. It supports nested structs via dot-separated keys, type coercion
// for all basic Go types, time.Duration parsing, slice and map binding,
// and optional validation via the validator package.
type StructBinder struct {
	tagName   string
	validator validator.Validator
}

var _ Binder = (*StructBinder)(nil)

// New creates a new StructBinder with the given options.
// By default, the struct tag "config" is used and no validation is performed.
func New(opts ...Option) *StructBinder {
	b := &StructBinder{tagName: "config"}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Option configures a StructBinder.
type Option func(*StructBinder)

// WithTagName sets the struct tag used for key mapping. Defaults to "config" if not specified.
func WithTagName(tag string) Option {
	return func(b *StructBinder) { b.tagName = tag }
}

// WithValidator sets a validator that is called after binding to validate
// the target struct.
func WithValidator(v validator.Validator) Option {
	return func(b *StructBinder) { b.validator = v }
}

// Bind maps configuration data to the target struct. The target must be a
// non-nil pointer to a struct. After binding, if a validator is configured,
// it is called to validate the populated struct.
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

// bindStruct recursively binds configuration values to struct fields.
// For nested struct fields without a matching config key, it recurses
// with the field name as a prefix.
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

// setField coerces a configuration value to the target field's type and sets it.
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

// fieldCache caches resolved field metadata keyed by reflect.Type to avoid
// redundant reflection on repeated binds of the same struct type.
var fieldCache sync.Map
