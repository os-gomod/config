package value

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("string value", func(t *testing.T) {
		v := New("hello", TypeString, SourceFile, 10)
		if v.String() != "hello" {
			t.Errorf("expected 'hello', got %q", v.String())
		}
		if v.Type() != TypeString {
			t.Errorf("expected TypeString, got %d", v.Type())
		}
		if v.Source() != SourceFile {
			t.Errorf("expected SourceFile, got %d", v.Source())
		}
		if v.Priority() != 10 {
			t.Errorf("expected priority 10, got %d", v.Priority())
		}
	})

	t.Run("int value", func(t *testing.T) {
		v := New(42, TypeInt, SourceMemory, 50)
		if v.Type() != TypeInt {
			t.Errorf("expected TypeInt, got %d", v.Type())
		}
	})

	t.Run("bool value", func(t *testing.T) {
		v := New(true, TypeBool, SourceEnv, 0)
		if v.Type() != TypeBool {
			t.Errorf("expected TypeBool, got %d", v.Type())
		}
	})

	t.Run("nil raw value", func(t *testing.T) {
		v := New(nil, TypeUnknown, SourceUnknown, 0)
		if !v.IsZero() {
			t.Error("expected IsZero() to be true for nil raw")
		}
		if v.String() != "" {
			t.Errorf("expected empty string for nil, got %q", v.String())
		}
	})
}

func TestNewInMemory(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		v := NewInMemory("test")
		if v.Type() != TypeString {
			t.Errorf("expected TypeString, got %d", v.Type())
		}
		if v.Source() != SourceMemory {
			t.Errorf("expected SourceMemory, got %d", v.Source())
		}
		if v.Priority() != 100 {
			t.Errorf("expected priority 100, got %d", v.Priority())
		}
	})

	t.Run("int", func(t *testing.T) {
		v := NewInMemory(42)
		if v.Type() != TypeInt {
			t.Errorf("expected TypeInt, got %d", v.Type())
		}
	})

	t.Run("float64", func(t *testing.T) {
		v := NewInMemory(3.14)
		if v.Type() != TypeFloat64 {
			t.Errorf("expected TypeFloat64, got %d", v.Type())
		}
	})
}

func TestFromRaw(t *testing.T) {
	v := FromRaw("hello")
	if v.Type() != TypeString {
		t.Errorf("expected TypeString, got %d", v.Type())
	}
	if v.Source() != SourceUnknown {
		t.Errorf("expected SourceUnknown, got %d", v.Source())
	}
	if v.Priority() != 0 {
		t.Errorf("expected priority 0, got %d", v.Priority())
	}
}

func TestValue_String(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"float64", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.raw, InferType(tt.raw), SourceUnknown, 0)
			if got := v.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValue_Int(t *testing.T) {
	tests := []struct {
		name   string
		raw    any
		want   int
		wantOk bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(100), 100, true},
		{"float64", float64(3.7), 3, true},
		{"string", "not an int", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.raw, TypeUnknown, SourceUnknown, 0)
			got, ok := v.Int()
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("Int() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestValue_Int64(t *testing.T) {
	tests := []struct {
		name   string
		raw    any
		want   int64
		wantOk bool
	}{
		{"int64", int64(9223372036854775807), int64(9223372036854775807), true},
		{"int", 42, 42, true},
		{"float64", float64(3.7), 3, true},
		{"string", "not int64", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.raw, TypeUnknown, SourceUnknown, 0)
			got, ok := v.Int64()
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("Int64() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestValue_Float64(t *testing.T) {
	tests := []struct {
		name   string
		raw    any
		want   float64
		wantOk bool
	}{
		{"float64", 3.14, 3.14, true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"bool", true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.raw, TypeUnknown, SourceUnknown, 0)
			got, ok := v.Float64()
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("Float64() = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestValue_Bool(t *testing.T) {
	tests := []struct {
		name   string
		raw    any
		want   bool
		wantOk bool
	}{
		{"true", true, true, true},
		{"false", false, false, true},
		{"string", "true", false, false},
		{"int", 1, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.raw, TypeUnknown, SourceUnknown, 0)
			got, ok := v.Bool()
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("Bool() = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestInferType(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  Type
	}{
		{"string", "hello", TypeString},
		{"int", 42, TypeInt},
		{"int64", int64(100), TypeInt64},
		{"float64", 3.14, TypeFloat64},
		{"bool", true, TypeBool},
		{"duration", time.Hour, TypeDuration},
		{"time", time.Now(), TypeTime},
		{"slice", []any{}, TypeSlice},
		{"map", map[string]any{}, TypeMap},
		{"bytes", []byte("data"), TypeBytes},
		{"nil", nil, TypeUnknown},
		{"struct", struct{}{}, TypeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferType(tt.input); got != tt.want {
				t.Errorf("InferType(%T) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestType_String(t *testing.T) {
	tests := []struct {
		typ  Type
		want string
	}{
		{TypeUnknown, "unknown"},
		{TypeString, "string"},
		{TypeInt, "int"},
		{TypeInt64, "int64"},
		{TypeFloat64, "float64"},
		{TypeBool, "bool"},
		{TypeDuration, "duration"},
		{TypeTime, "time"},
		{TypeSlice, "slice"},
		{TypeMap, "map"},
		{TypeBytes, "bytes"},
		{Type(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("Type(%d).String() = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}

func TestSource_String(t *testing.T) {
	tests := []struct {
		src  Source
		want string
	}{
		{SourceUnknown, "unknown"},
		{SourceFile, "file"},
		{SourceEnv, "env"},
		{SourceMemory, "memory"},
		{SourceHTTP, "http"},
		{SourceKubernetes, "kubernetes"},
		{SourceVault, "vault"},
		{SourceKMS, "kms"},
		{SourceRemote, "remote"},
		{Source(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.src.String(); got != tt.want {
				t.Errorf("Source(%d).String() = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestValue_Equal(t *testing.T) {
	t.Run("equal values", func(t *testing.T) {
		v1 := New("hello", TypeString, SourceFile, 10)
		v2 := New("hello", TypeString, SourceFile, 10)
		if !v1.Equal(v2) {
			t.Error("expected equal values")
		}
	})

	t.Run("different types", func(t *testing.T) {
		v1 := New("42", TypeString, SourceFile, 10)
		v2 := New(42, TypeInt, SourceFile, 10)
		if v1.Equal(v2) {
			t.Error("expected different types to be unequal")
		}
	})

	t.Run("different raw values", func(t *testing.T) {
		v1 := New("hello", TypeString, SourceFile, 10)
		v2 := New("world", TypeString, SourceFile, 10)
		if v1.Equal(v2) {
			t.Error("expected different raw values to be unequal")
		}
	})

	t.Run("different sources but same raw and type", func(t *testing.T) {
		v1 := New("hello", TypeString, SourceFile, 10)
		v2 := New("hello", TypeString, SourceEnv, 10)
		if !v1.Equal(v2) {
			t.Error("expected equal - source doesn't affect equality")
		}
	})

	t.Run("different priorities but same raw and type", func(t *testing.T) {
		v1 := New("hello", TypeString, SourceFile, 10)
		v2 := New("hello", TypeString, SourceFile, 50)
		if !v1.Equal(v2) {
			t.Error("expected equal - priority doesn't affect equality")
		}
	})

	t.Run("zero values are equal", func(t *testing.T) {
		v1 := Value{}
		v2 := Value{}
		if !v1.Equal(v2) {
			t.Error("expected zero values to be equal")
		}
	})
}

func TestValue_Copy(t *testing.T) {
	t.Run("copy is independent", func(t *testing.T) {
		original := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 10),
			"b": New("2", TypeString, SourceMemory, 20),
		}
		copy := Copy(original)

		// Modify copy
		copy["c"] = New("3", TypeString, SourceMemory, 30)

		if len(original) != 2 {
			t.Errorf("expected original to have 2 keys, got %d", len(original))
		}
		if len(copy) != 3 {
			t.Errorf("expected copy to have 3 keys, got %d", len(copy))
		}
	})

	t.Run("copy values are shared (not deep copy)", func(t *testing.T) {
		original := map[string]Value{
			"a": New("1", TypeString, SourceMemory, 10),
		}
		copy := Copy(original)

		// Values are structs (value types), so they are already copies
		copy["a"] = New("changed", TypeString, SourceMemory, 10)
		if original["a"].String() == "changed" {
			t.Error("copy should be independent from original")
		}
	})

	t.Run("copy nil returns empty map", func(t *testing.T) {
		result := Copy(nil)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d keys", len(result))
		}
	})
}

func TestSortedKeys(t *testing.T) {
	t.Run("sorted output", func(t *testing.T) {
		m := map[string]Value{
			"zebra":  New("z", TypeString, SourceUnknown, 0),
			"apple":  New("a", TypeString, SourceUnknown, 0),
			"banana": New("b", TypeString, SourceUnknown, 0),
		}
		keys := SortedKeys(m)
		if len(keys) != 3 {
			t.Fatalf("expected 3 keys, got %d", len(keys))
		}
		if keys[0] != "apple" || keys[1] != "banana" || keys[2] != "zebra" {
			t.Errorf("expected sorted keys, got %v", keys)
		}
	})

	t.Run("nil map returns nil", func(t *testing.T) {
		keys := SortedKeys(nil)
		if keys != nil {
			t.Errorf("expected nil, got %v", keys)
		}
	})

	t.Run("empty map returns empty slice", func(t *testing.T) {
		keys := SortedKeys(map[string]Value{})
		if len(keys) != 0 {
			t.Errorf("expected empty slice, got %v", keys)
		}
	})
}

func TestValue_Raw(t *testing.T) {
	v := New("rawdata", TypeString, SourceMemory, 0)
	if v.Raw() != "rawdata" {
		t.Errorf("expected 'rawdata', got %v", v.Raw())
	}
}

func TestValue_Duration(t *testing.T) {
	t.Run("duration type", func(t *testing.T) {
		d := 5 * time.Second
		v := New(d, TypeDuration, SourceUnknown, 0)
		got, ok := v.Duration()
		if !ok || got != d {
			t.Errorf("Duration() = (%v, %v), want (%v, true)", got, ok, d)
		}
	})

	t.Run("int64 type", func(t *testing.T) {
		v := New(int64(5000000000), TypeUnknown, SourceUnknown, 0)
		got, ok := v.Duration()
		if !ok || got != 5*time.Second {
			t.Errorf("Duration() = (%v, %v), want (5s, true)", got, ok)
		}
	})

	t.Run("string type", func(t *testing.T) {
		v := New("5s", TypeUnknown, SourceUnknown, 0)
		got, ok := v.Duration()
		if !ok || got != 5*time.Second {
			t.Errorf("Duration() = (%v, %v), want (5s, true)", got, ok)
		}
	})

	t.Run("invalid string", func(t *testing.T) {
		v := New("not-a-duration", TypeUnknown, SourceUnknown, 0)
		_, ok := v.Duration()
		if ok {
			t.Error("expected false for invalid duration string")
		}
	})
}

func TestValue_Slice(t *testing.T) {
	t.Run("slice type", func(t *testing.T) {
		s := []any{1, "two", true}
		v := New(s, TypeSlice, SourceUnknown, 0)
		got, ok := v.Slice()
		if !ok || len(got) != 3 {
			t.Errorf("expected true with 3 elements")
		}
	})

	t.Run("non-slice type", func(t *testing.T) {
		v := New("not a slice", TypeUnknown, SourceUnknown, 0)
		_, ok := v.Slice()
		if ok {
			t.Error("expected false for non-slice")
		}
	})
}

func TestValue_Map(t *testing.T) {
	t.Run("map type", func(t *testing.T) {
		m := map[string]any{"key": "val"}
		v := New(m, TypeMap, SourceUnknown, 0)
		got, ok := v.Map()
		if !ok || got["key"] != "val" {
			t.Errorf("expected true with map")
		}
	})

	t.Run("non-map type", func(t *testing.T) {
		v := New("not a map", TypeUnknown, SourceUnknown, 0)
		_, ok := v.Map()
		if ok {
			t.Error("expected false for non-map")
		}
	})
}

func TestValue_Bytes(t *testing.T) {
	t.Run("bytes type", func(t *testing.T) {
		b := []byte("hello")
		v := New(b, TypeBytes, SourceUnknown, 0)
		got, ok := v.Bytes()
		if !ok || string(got) != "hello" {
			t.Error("expected true with bytes")
		}
	})

	t.Run("string converted to bytes", func(t *testing.T) {
		v := New("hello", TypeUnknown, SourceUnknown, 0)
		got, ok := v.Bytes()
		if !ok || string(got) != "hello" {
			t.Error("expected true with string converted to bytes")
		}
	})
}

func TestValue_Time(t *testing.T) {
	t.Run("time type", func(t *testing.T) {
		now := time.Now()
		v := New(now, TypeTime, SourceUnknown, 0)
		got, ok := v.Time()
		if !ok || !got.Equal(now) {
			t.Error("expected true with same time")
		}
	})

	t.Run("non-time type", func(t *testing.T) {
		v := New("not time", TypeUnknown, SourceUnknown, 0)
		_, ok := v.Time()
		if ok {
			t.Error("expected false for non-time")
		}
	})
}

func TestAs(t *testing.T) {
	t.Run("correct type", func(t *testing.T) {
		v := New("hello", TypeString, SourceUnknown, 0)
		got, ok := As[string](v)
		if !ok || got != "hello" {
			t.Error("expected to extract string")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		v := New(42, TypeInt, SourceUnknown, 0)
		_, ok := As[string](v)
		if ok {
			t.Error("expected false for wrong type")
		}
	})

	t.Run("nil value", func(t *testing.T) {
		v := New(nil, TypeUnknown, SourceUnknown, 0)
		got, ok := As[string](v)
		if ok || got != "" {
			t.Error("expected false and zero value for nil")
		}
	})
}

func TestComputeChecksum(t *testing.T) {
	t.Run("nil returns all-zeros hash", func(t *testing.T) {
		h := ComputeChecksum(nil)
		if len(h) != 64 {
			t.Errorf("expected 64-char hex string, got %d", len(h))
		}
	})

	t.Run("same data same hash", func(t *testing.T) {
		d1 := map[string]Value{"a": New("1", TypeString, SourceUnknown, 0)}
		d2 := map[string]Value{"a": New("1", TypeString, SourceUnknown, 0)}
		if ComputeChecksum(d1) != ComputeChecksum(d2) {
			t.Error("expected same checksum for same data")
		}
	})

	t.Run("different data different hash", func(t *testing.T) {
		d1 := map[string]Value{"a": New("1", TypeString, SourceUnknown, 0)}
		d2 := map[string]Value{"a": New("2", TypeString, SourceUnknown, 0)}
		if ComputeChecksum(d1) == ComputeChecksum(d2) {
			t.Error("expected different checksums for different data")
		}
	})
}
