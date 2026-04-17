package binder

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/os-gomod/config/core/value"
	configerrors "github.com/os-gomod/config/errors"
)

// coerceDuration attempts to coerce a raw value to time.Duration.
// Returns (true, nil) on success, (true, error) on coercion failure, or
// (false, nil) if the field is not a time.Duration.
func coerceDuration(fv reflect.Value, raw any) (bool, error) {
	if fv.Type().String() != "time.Duration" {
		return false, nil
	}
	switch val := raw.(type) {
	case string:
		d, err := time.ParseDuration(val)
		if err != nil {
			return true, configerrors.Wrap(err, configerrors.CodeBind, "invalid duration")
		}
		fv.SetInt(int64(d))
	case int64:
		fv.SetInt(val)
	case float64:
		fv.SetInt(int64(val))
	case int:
		fv.SetInt(int64(val))
	default:
		return true, configerrors.Newf(configerrors.CodeBind, "cannot coerce %T to time.Duration", raw)
	}
	return true, nil
}

// coerceString coerces a raw value to a string. Supports string, []byte, and
// fmt.Sprint fallback for other types.
func coerceString(fv reflect.Value, raw any) {
	switch val := raw.(type) {
	case string:
		fv.SetString(val)
	case []byte:
		fv.SetString(string(val))
	default:
		fv.SetString(fmt.Sprintf("%v", val))
	}
}

// coerceInt coerces a value to a signed integer. Tries Int64() first, then
// falls back to string parsing.
func coerceInt(fv reflect.Value, v value.Value, raw any) error {
	if i, ok := v.Int64(); ok {
		fv.SetInt(i)
		return nil
	}
	if s, ok := raw.(string); ok {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return configerrors.Wrap(err, configerrors.CodeBind, "parse int")
		}
		fv.SetInt(i)
	}
	return nil
}

// coerceUint coerces a value to an unsigned integer. Only succeeds if the
// Int64() value is non-negative.
func coerceUint(fv reflect.Value, v value.Value) {
	if i, ok := v.Int64(); ok && i >= 0 {
		fv.SetUint(uint64(i))
	}
}

// coerceFloat coerces a value to a floating-point number. Tries Float64() first,
// then falls back to string parsing.
func coerceFloat(fv reflect.Value, v value.Value, raw any) error {
	if f, ok := v.Float64(); ok {
		fv.SetFloat(f)
		return nil
	}
	if s, ok := raw.(string); ok {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return configerrors.Wrap(err, configerrors.CodeBind, "parse float")
		}
		fv.SetFloat(f)
	}
	return nil
}

// coerceBool coerces a value to a boolean. Tries Bool() first, then falls
// back to strconv.ParseBool for string values.
func coerceBool(fv reflect.Value, v value.Value, raw any) error {
	if bv, ok := v.Bool(); ok {
		fv.SetBool(bv)
		return nil
	}
	if s, ok := raw.(string); ok {
		parsed, err := strconv.ParseBool(s)
		if err != nil {
			return configerrors.Wrap(err, configerrors.CodeBind, "parse bool")
		}
		fv.SetBool(parsed)
	}
	return nil
}

// coerceSlice coerces a value to a slice by recursively coercing each element.
func coerceSlice(fv reflect.Value, v value.Value, b *StructBinder) error {
	slice, ok := v.Slice()
	if !ok {
		return nil
	}
	newSlice := reflect.MakeSlice(fv.Type(), 0, len(slice))
	for _, item := range slice {
		elem := reflect.New(fv.Type().Elem()).Elem()
		if err := b.setField(elem, value.FromRaw(item)); err != nil {
			return err
		}
		newSlice = reflect.Append(newSlice, elem)
	}
	fv.Set(newSlice)
	return nil
}

// coerceMap coerces a value to a map[string]T. The map key type must be string.
func coerceMap(fv reflect.Value, v value.Value, b *StructBinder) error {
	raw := v.Raw()
	if raw == nil {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return coerceFallback(fv, raw)
	}
	ft := fv.Type()
	if ft.Key().Kind() != reflect.String {
		return configerrors.Newf(configerrors.CodeBind, "map key must be string, got %s", ft.Key())
	}
	newMap := reflect.MakeMapWithSize(ft, len(m))
	for k, val := range m {
		elem := reflect.New(ft.Elem()).Elem()
		if err := b.setField(elem, value.FromRaw(val)); err != nil {
			return configerrors.Wrap(err, configerrors.CodeBind, "bind map element").WithKey(k)
		}
		newMap.SetMapIndex(reflect.ValueOf(k), elem)
	}
	fv.Set(newMap)
	return nil
}

// coerceFallback attempts direct assignment or type conversion as a last resort
// for unsupported field types.
func coerceFallback(fv reflect.Value, raw any) error {
	if raw == nil {
		return configerrors.Newf(configerrors.CodeBind, "cannot assign nil to %s", fv.Type())
	}
	val := reflect.ValueOf(raw)
	ft := fv.Type()
	if val.Type().AssignableTo(ft) {
		fv.Set(val)
		return nil
	}
	if val.Type().ConvertibleTo(ft) {
		fv.Set(val.Convert(ft))
		return nil
	}
	return configerrors.Newf(configerrors.CodeBind, "cannot assign %T to %s", raw, ft)
}
