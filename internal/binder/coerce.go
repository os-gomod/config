package binder

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/errors"
	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// Coercer
// ---------------------------------------------------------------------------

// Coercer converts configuration Values to target Go types using reflection.
// It handles all common types: bool, string, int, float64, duration,
// slices, maps, and nested structs.
type Coercer struct{}

// NewCoercer creates a new Coercer.
func NewCoercer() *Coercer {
	return &Coercer{}
}

// Coerce converts a Value to the target reflect.Type. Returns the coerced
// reflect.Value ready for assignment to a struct field.
//
//nolint:gocyclo,cyclop // high complexity is inherent to exhaustive type coercion
func (c *Coercer) Coerce(v value.Value, target reflect.Type) (reflect.Value, error) {
	if target == nil {
		return reflect.Value{}, errors.New(errors.CodeTypeMismatch, "target type must not be nil")
	}

	if target.Kind() == reflect.Ptr {
		return c.coercePointer(v, target)
	}

	return c.coerceDirect(v, target)
}

func (c *Coercer) coercePointer(v value.Value, target reflect.Type) (reflect.Value, error) {
	if v.IsZero() {
		return reflect.Zero(target), nil
	}

	elemType := target.Elem()
	elem, err := c.Coerce(v, elemType)
	if err != nil {
		return reflect.Value{}, err
	}

	ptr := reflect.New(elemType)
	ptr.Elem().Set(elem)
	return ptr, nil
}

func (c *Coercer) coerceDirect(v value.Value, target reflect.Type) (reflect.Value, error) {
	switch target.Kind() {
	case reflect.Bool:
		return c.coerceBool(v)
	case reflect.String:
		return c.coerceString(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return c.coerceSigned(v, target)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return c.coerceUnsigned(v, target)
	case reflect.Float32, reflect.Float64:
		return c.coerceFloat(v, target)
	case reflect.Interface:
		return reflect.ValueOf(v.Raw()), nil
	case reflect.Slice:
		return c.coerceSlice(v)
	case reflect.Map:
		return c.coerceMap(v, target)
	case reflect.Struct:
		return c.coerceStruct(v, target)
	default:
		return reflect.Value{}, errors.New(errors.CodeTypeMismatch,
			fmt.Sprintf("unsupported target type: %v", target))
	}
}

func (c *Coercer) coerceBool(v value.Value) (reflect.Value, error) {
	b, err := c.ToBool(v)
	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(b), nil
}

func (c *Coercer) coerceString(v value.Value) (reflect.Value, error) {
	s, err := c.ToString(v)
	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(s), nil
}

func (c *Coercer) coerceSigned(v value.Value, target reflect.Type) (reflect.Value, error) {
	if target.Kind() == reflect.Int64 && isDurationType(target) {
		d, err := c.ToDuration(v)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(d), nil
	}

	i, err := c.ToInt(v)
	if err != nil {
		return reflect.Value{}, err
	}

	switch target.Kind() {
	case reflect.Int:
		return reflect.ValueOf(i), nil
	case reflect.Int8:
		return reflect.ValueOf(int8(i)), nil
	case reflect.Int16:
		return reflect.ValueOf(int16(i)), nil
	case reflect.Int32:
		return reflect.ValueOf(int32(i)), nil
	case reflect.Int64:
		return reflect.ValueOf(v.Int64()), nil
	default:
		return reflect.Value{}, errors.New(errors.CodeTypeMismatch,
			fmt.Sprintf("unsupported signed target type: %v", target))
	}
}

func (c *Coercer) coerceUnsigned(v value.Value, target reflect.Type) (reflect.Value, error) {
	i, err := c.ToInt(v)
	if err != nil {
		return reflect.Value{}, err
	}

	switch target.Kind() {
	case reflect.Uint:
		return reflect.ValueOf(uint(i)), nil
	case reflect.Uint8:
		return reflect.ValueOf(uint8(i)), nil
	case reflect.Uint16:
		return reflect.ValueOf(uint16(i)), nil
	case reflect.Uint32:
		return reflect.ValueOf(uint32(i)), nil
	case reflect.Uint64:
		return reflect.ValueOf(uint64(i)), nil
	default:
		return reflect.Value{}, errors.New(errors.CodeTypeMismatch,
			fmt.Sprintf("unsupported unsigned target type: %v", target))
	}
}

func (c *Coercer) coerceFloat(v value.Value, target reflect.Type) (reflect.Value, error) {
	f, err := c.ToFloat64(v)
	if err != nil {
		return reflect.Value{}, err
	}

	if target.Kind() == reflect.Float32 {
		return reflect.ValueOf(float32(f)), nil
	}

	return reflect.ValueOf(f), nil
}

func (c *Coercer) coerceSlice(v value.Value) (reflect.Value, error) {
	slice, err := c.ToSlice(v)
	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(slice), nil
}

func (c *Coercer) coerceMap(v value.Value, target reflect.Type) (reflect.Value, error) {
	if target.Key().Kind() != reflect.String {
		return reflect.Value{}, errors.New(errors.CodeTypeMismatch,
			fmt.Sprintf("unsupported map key type: %v", target.Key()))
	}

	m, err := c.ToMap(v)
	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(m), nil
}

func (c *Coercer) coerceStruct(v value.Value, target reflect.Type) (reflect.Value, error) {
	if isDurationType(target) {
		d, err := c.ToDuration(v)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(d), nil
	}

	if isTimeType(target) {
		return reflect.ValueOf(v.Time()), nil
	}

	return reflect.Value{}, errors.New(errors.CodeTypeMismatch,
		fmt.Sprintf("binding to struct type %v requires nested binding", target))
}

// ToBool converts a Value to bool.
func (c *Coercer) ToBool(v value.Value) (bool, error) {
	if v.IsZero() {
		return false, nil
	}
	return v.Bool(), nil
}

// ToString converts a Value to string.
func (c *Coercer) ToString(v value.Value) (string, error) {
	if v.IsZero() {
		return "", nil
	}
	return v.String(), nil
}

// ToInt converts a Value to int.
func (c *Coercer) ToInt(v value.Value) (int, error) {
	if v.IsZero() {
		return 0, nil
	}
	return v.Int(), nil
}

// ToFloat64 converts a Value to float64.
func (c *Coercer) ToFloat64(v value.Value) (float64, error) {
	if v.IsZero() {
		return 0, nil
	}
	return v.Float64(), nil
}

// ToDuration converts a Value to time.Duration.
func (c *Coercer) ToDuration(v value.Value) (time.Duration, error) {
	if v.IsZero() {
		return 0, nil
	}

	// If already a duration, return directly.
	if d, ok := v.Raw().(time.Duration); ok {
		return d, nil
	}

	// Try parsing as a string duration.
	s := v.String()
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Try parsing as a numeric value (nanoseconds).
	i, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return time.Duration(i), nil
	}

	// Try parsing as numeric float seconds.
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return time.Duration(f * float64(time.Second)), nil
	}

	return 0, errors.New(errors.CodeTypeMismatch,
		fmt.Sprintf("cannot convert %q to duration", s))
}

// ToSlice converts a Value to []any.
func (c *Coercer) ToSlice(v value.Value) ([]any, error) {
	if v.IsZero() {
		return nil, nil
	}
	slice := v.Slice()
	if slice == nil {
		// Try parsing a comma-separated string.
		s := v.String()
		if s != "" {
			parts := strings.Split(s, ",")
			result := make([]any, len(parts))
			for i, p := range parts {
				result[i] = strings.TrimSpace(p)
			}
			return result, nil
		}
		return nil, errors.New(errors.CodeTypeMismatch,
			fmt.Sprintf("cannot convert %T to slice", v.Raw()))
	}
	return slice, nil
}

// ToMap converts a Value to map[string]any.
func (c *Coercer) ToMap(v value.Value) (map[string]any, error) {
	if v.IsZero() {
		//nolint:nilnil // a nil map is the zero-value representation for an absent config map
		return nil, nil
	}
	m := v.Map()
	if m == nil {
		return nil, errors.New(errors.CodeTypeMismatch,
			fmt.Sprintf("cannot convert %T to map", v.Raw()))
	}
	return m, nil
}

// ToStruct binds a Value's raw map data to a target struct using reflection.
func (c *Coercer) ToStruct(v value.Value, target any) error {
	if target == nil {
		return errors.New(errors.CodeInvalidConfig, "target must not be nil")
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New(errors.CodeInvalidConfig, "target must be a non-nil pointer")
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errors.New(errors.CodeInvalidConfig, "target must point to a struct")
	}

	m := v.Map()
	if m == nil {
		return errors.New(errors.CodeTypeMismatch, "value is not a map")
	}

	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		fieldVal := rv.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		// Determine key from tag or field name.
		key := strings.ToLower(field.Name)
		if tag := field.Tag.Get("config"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				key = parts[0]
			}
		}

		rawVal, exists := m[key]
		if !exists {
			continue
		}

		wrapped := value.New(rawVal)
		coerced, err := c.Coerce(wrapped, field.Type)
		if err != nil {
			return errors.Wrap(err, errors.CodeBind,
				fmt.Sprintf("failed to bind map key %q to field %s", key, field.Name)).
				WithKey(key)
		}

		fieldVal.Set(coerced)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isDurationType returns true if the type is time.Duration.
func isDurationType(t reflect.Type) bool {
	var d time.Duration
	return t == reflect.TypeOf(d)
}

// isTimeType returns true if the type is time.Time.
func isTimeType(t reflect.Type) bool {
	var tm time.Time
	return t == reflect.TypeOf(tm)
}
