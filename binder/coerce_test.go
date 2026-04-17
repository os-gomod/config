package binder

import (
	"reflect"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	configerrors "github.com/os-gomod/config/errors"
)

func TestCoerceDuration(t *testing.T) {
	t.Run("string to duration", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()
		handled, err := coerceDuration(fv, "5s")
		if !handled {
			t.Fatal("expected handled=true")
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d != 5*time.Second {
			t.Fatalf("expected 5s, got %v", d)
		}
	})

	t.Run("int64 to duration", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()
		handled, err := coerceDuration(fv, int64(1000000000))
		if !handled || err != nil {
			t.Fatal("unexpected result")
		}
		if d != time.Second {
			t.Fatalf("expected 1s, got %v", d)
		}
	})

	t.Run("float64 to duration", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()
		handled, err := coerceDuration(fv, float64(5000000000))
		if !handled || err != nil {
			t.Fatal("unexpected result")
		}
		if d != 5*time.Second {
			t.Fatalf("expected 5s, got %v", d)
		}
	})

	t.Run("int to duration", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()
		handled, err := coerceDuration(fv, 42)
		if !handled || err != nil {
			t.Fatal("unexpected result")
		}
		if d != 42 {
			t.Fatalf("expected 42ns, got %v", d)
		}
	})

	t.Run("invalid duration string", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()
		handled, err := coerceDuration(fv, "not-a-duration")
		if !handled {
			t.Fatal("expected handled=true for duration field")
		}
		if err == nil {
			t.Fatal("expected error for invalid duration")
		}
	})

	t.Run("non-duration field returns false", func(t *testing.T) {
		var s string
		fv := reflect.ValueOf(&s).Elem()
		handled, _ := coerceDuration(fv, "5s")
		if handled {
			t.Fatal("expected handled=false for non-duration field")
		}
	})

	t.Run("invalid type for duration", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()
		handled, err := coerceDuration(fv, []int{1, 2, 3})
		if !handled {
			t.Fatal("expected handled=true")
		}
		if err == nil {
			t.Fatal("expected error for invalid type")
		}
	})
}

func TestCoerceString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", "hello"},
		{"[]byte", []byte("world"), "world"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"nil", nil, "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s string
			fv := reflect.ValueOf(&s).Elem()
			coerceString(fv, tt.input)
			if s != tt.want {
				t.Errorf("got %q, want %q", s, tt.want)
			}
		})
	}
}

func TestCoerceInt(t *testing.T) {
	t.Run("from int value", func(t *testing.T) {
		var i int
		fv := reflect.ValueOf(&i).Elem()
		v := value.NewInMemory(int64(42))
		err := coerceInt(fv, v, int64(42))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 42 {
			t.Fatalf("expected 42, got %d", i)
		}
	})

	t.Run("from string", func(t *testing.T) {
		var i int
		fv := reflect.ValueOf(&i).Elem()
		v := value.NewInMemory("123")
		err := coerceInt(fv, v, "123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 123 {
			t.Fatalf("expected 123, got %d", i)
		}
	})

	t.Run("from invalid string", func(t *testing.T) {
		var i int
		fv := reflect.ValueOf(&i).Elem()
		v := value.NewInMemory("abc")
		err := coerceInt(fv, v, "abc")
		if err == nil {
			t.Fatal("expected error for invalid string")
		}
	})
}

func TestCoerceUint(t *testing.T) {
	t.Run("from int64", func(t *testing.T) {
		var u uint
		fv := reflect.ValueOf(&u).Elem()
		v := value.NewInMemory(int64(42))
		coerceUint(fv, v)
		if u != 42 {
			t.Fatalf("expected 42, got %d", u)
		}
	})

	t.Run("negative int64 is ignored", func(t *testing.T) {
		var u uint
		fv := reflect.ValueOf(&u).Elem()
		v := value.NewInMemory(int64(-1))
		coerceUint(fv, v)
		if u != 0 {
			t.Fatalf("expected 0 for negative, got %d", u)
		}
	})
}

func TestCoerceFloat(t *testing.T) {
	t.Run("from float64 value", func(t *testing.T) {
		var f float64
		fv := reflect.ValueOf(&f).Elem()
		v := value.NewInMemory(3.14)
		err := coerceFloat(fv, v, 3.14)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f != 3.14 {
			t.Fatalf("expected 3.14, got %f", f)
		}
	})

	t.Run("from string", func(t *testing.T) {
		var f float64
		fv := reflect.ValueOf(&f).Elem()
		v := value.NewInMemory("2.71")
		err := coerceFloat(fv, v, "2.71")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f != 2.71 {
			t.Fatalf("expected 2.71, got %f", f)
		}
	})

	t.Run("from invalid string", func(t *testing.T) {
		var f float64
		fv := reflect.ValueOf(&f).Elem()
		v := value.NewInMemory("not-a-float")
		err := coerceFloat(fv, v, "not-a-float")
		if err == nil {
			t.Fatal("expected error for invalid string")
		}
	})
}

func TestCoerceBool(t *testing.T) {
	t.Run("from bool value", func(t *testing.T) {
		var b bool
		fv := reflect.ValueOf(&b).Elem()
		v := value.NewInMemory(true)
		err := coerceBool(fv, v, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Fatal("expected true")
		}
	})

	t.Run("from string true", func(t *testing.T) {
		var b bool
		fv := reflect.ValueOf(&b).Elem()
		v := value.NewInMemory("true")
		err := coerceBool(fv, v, "true")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Fatal("expected true")
		}
	})

	t.Run("from string 1", func(t *testing.T) {
		var b bool
		fv := reflect.ValueOf(&b).Elem()
		v := value.NewInMemory("1")
		err := coerceBool(fv, v, "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Fatal("expected true")
		}
	})

	t.Run("from string false", func(t *testing.T) {
		var b bool
		fv := reflect.ValueOf(&b).Elem()
		v := value.NewInMemory("false")
		err := coerceBool(fv, v, "false")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b {
			t.Fatal("expected false")
		}
	})

	t.Run("from invalid string", func(t *testing.T) {
		var b bool
		fv := reflect.ValueOf(&b).Elem()
		v := value.NewInMemory("yes")
		err := coerceBool(fv, v, "yes")
		if err == nil {
			t.Fatal("expected error for invalid bool string")
		}
	})
}

func TestCoerceFallback(t *testing.T) {
	t.Run("assignable type", func(t *testing.T) {
		var s string
		fv := reflect.ValueOf(&s).Elem()
		err := coerceFallback(fv, "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "hello" {
			t.Fatalf("expected 'hello', got %q", s)
		}
	})

	t.Run("convertible type", func(t *testing.T) {
		var i int32
		fv := reflect.ValueOf(&i).Elem()
		err := coerceFallback(fv, int64(42))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 42 {
			t.Fatalf("expected 42, got %d", i)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		var s string
		fv := reflect.ValueOf(&s).Elem()
		err := coerceFallback(fv, nil)
		if err == nil {
			t.Fatal("expected error for nil")
		}
		var ce *configerrors.ConfigError
		if !configerrors.IsCode(err, configerrors.CodeBind) {
			t.Fatal("expected CodeBind error")
		}
		_ = ce
	})

	t.Run("incompatible type", func(t *testing.T) {
		var i int
		fv := reflect.ValueOf(&i).Elem()
		err := coerceFallback(fv, "not-an-int")
		if err == nil {
			t.Fatal("expected error for incompatible type")
		}
	})
}
