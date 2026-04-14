package binder

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/os-gomod/config/core/value"
	configerrors "github.com/os-gomod/config/errors"
)

// coerceDuration attempts to set fv to a time.Duration parsed from raw.
// Returns (true, err) if fv is a Duration field; (false, nil) otherwise.
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

// coerceString sets fv to a string representation of raw.
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

// coerceInt sets fv to an int64 parsed from raw or v.
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

// coerceUint sets fv to a uint64 from v if the stored value is a non-negative int.
func coerceUint(fv reflect.Value, v value.Value) {
	if i, ok := v.Int64(); ok && i >= 0 {
		fv.SetUint(uint64(i))
	}
}

// coerceFloat sets fv to a float64 parsed from raw or v.
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

// coerceBool sets fv to a bool parsed from raw or v.
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

// coerceSlice sets fv to a slice populated by recursively coercing each element.
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

// coerceMap sets fv to a map[string]T where each value is coerced recursively.
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

// coerceFallback attempts assignment or conversion as a last resort.
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
