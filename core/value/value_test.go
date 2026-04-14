package value_test

import (
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
)

func TestNewCreatesValueWithCorrectFields(t *testing.T) {
	tests := []struct {
		name     string
		raw      any
		typ      value.Type
		src      value.Source
		priority int
	}{
		{"string", "hello", value.TypeString, value.SourceMemory, 100},
		{"int", 42, value.TypeInt, value.SourceFile, 10},
		{"bool", true, value.TypeBool, value.SourceEnv, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := value.New(tt.raw, tt.typ, tt.src, tt.priority)
			if v.Raw() != tt.raw {
				t.Fatalf("Raw: got %v, want %v", v.Raw(), tt.raw)
			}
			if v.Type() != tt.typ {
				t.Fatalf("Type: got %v, want %v", v.Type(), tt.typ)
			}
			if v.Source() != tt.src {
				t.Fatalf("Source: got %v, want %v", v.Source(), tt.src)
			}
			if v.Priority() != tt.priority {
				t.Fatalf("Priority: got %d, want %d", v.Priority(), tt.priority)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	v1 := value.New("hello", value.TypeString, value.SourceMemory, 10)
	v2 := value.New("hello", value.TypeString, value.SourceFile, 20)
	v3 := value.New("world", value.TypeString, value.SourceMemory, 10)
	v4 := value.New(42, value.TypeInt, value.SourceMemory, 10)

	if !v1.Equal(v2) {
		t.Fatal("same raw and type should be equal regardless of source/priority")
	}
	if v1.Equal(v3) {
		t.Fatal("different raw should not be equal")
	}
	if v1.Equal(v4) {
		t.Fatal("different types should not be equal")
	}
}

func TestInferType(t *testing.T) {
	tests := []struct {
		raw any
		typ value.Type
	}{
		{"hello", value.TypeString},
		{42, value.TypeInt},
		{int64(42), value.TypeInt64},
		{3.14, value.TypeFloat64},
		{true, value.TypeBool},
		{[]any{1, 2}, value.TypeSlice},
		{map[string]any{"a": 1}, value.TypeMap},
		{time.Second, value.TypeDuration},
		{time.Now(), value.TypeTime},
		{[]byte("hi"), value.TypeBytes},
		{complex(1, 2), value.TypeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.typ.String(), func(t *testing.T) {
			got := value.InferType(tt.raw)
			if got != tt.typ {
				t.Fatalf("InferType(%T): got %v, want %v", tt.raw, got, tt.typ)
			}
		})
	}
}

func TestCopyProducesIndependentMap(t *testing.T) {
	original := map[string]value.Value{
		"key": value.NewInMemory("val"),
	}
	copied := value.Copy(original)
	copied["key"] = value.NewInMemory("changed")
	if original["key"].Raw() != "val" {
		t.Fatal("Copy should produce an independent map")
	}
}

func TestCopyNilMap(t *testing.T) {
	copied := value.Copy(nil)
	if len(copied) != 0 {
		t.Fatal("Copy(nil) should return empty map")
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]value.Value{
		"c": value.NewInMemory(3),
		"a": value.NewInMemory(1),
		"b": value.NewInMemory(2),
	}
	keys := value.SortedKeys(m)
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Fatalf("SortedKeys: got %v", keys)
	}
}

func TestSortedKeysNilMap(t *testing.T) {
	keys := value.SortedKeys(nil)
	if keys != nil {
		t.Fatalf("SortedKeys(nil): got %v, want nil", keys)
	}
}

func TestComputeChecksum(t *testing.T) {
	data1 := map[string]value.Value{
		"a": value.NewInMemory("1"),
		"b": value.NewInMemory("2"),
	}
	data2 := map[string]value.Value{
		"a": value.NewInMemory("1"),
		"b": value.NewInMemory("2"),
	}
	data3 := map[string]value.Value{
		"a": value.NewInMemory("1"),
		"b": value.NewInMemory("3"),
	}

	cs1 := value.ComputeChecksum(data1)
	cs2 := value.ComputeChecksum(data2)
	cs3 := value.ComputeChecksum(data3)

	if cs1 != cs2 {
		t.Fatal("same data should produce same checksum")
	}
	if cs1 == cs3 {
		t.Fatal("different data should produce different checksum")
	}
}

func TestComputeChecksumNil(t *testing.T) {
	cs := value.ComputeChecksum(nil)
	if len(cs) != 64 {
		t.Fatalf("nil checksum should be 64 chars, got %d", len(cs))
	}
}

func TestNewInMemory(t *testing.T) {
	v := value.NewInMemory("test")
	if v.Source() != value.SourceMemory {
		t.Fatal("NewInMemory should set SourceMemory")
	}
	if v.Priority() != 100 {
		t.Fatal("NewInMemory should set priority 100")
	}
	if v.Type() != value.TypeString {
		t.Fatal("NewInMemory should infer type")
	}
}

func TestFromRaw(t *testing.T) {
	v := value.FromRaw(42)
	if v.Source() != value.SourceUnknown {
		t.Fatal("FromRaw should set SourceUnknown")
	}
	if v.Type() != value.TypeInt {
		t.Fatal("FromRaw should infer type")
	}
}

func TestValueAccessors(t *testing.T) {
	t.Run("Bool", func(t *testing.T) {
		v := value.NewInMemory(true)
		b, ok := v.Bool()
		if !ok || !b {
			t.Fatal("Bool accessor failed")
		}
	})

	t.Run("Int64", func(t *testing.T) {
		v := value.NewInMemory(int64(42))
		n, ok := v.Int64()
		if !ok || n != 42 {
			t.Fatal("Int64 accessor failed")
		}
	})

	t.Run("Float64", func(t *testing.T) {
		v := value.NewInMemory(3.14)
		f, ok := v.Float64()
		if !ok || f != 3.14 {
			t.Fatal("Float64 accessor failed")
		}
	})

	t.Run("Slice", func(t *testing.T) {
		v := value.NewInMemory([]any{1, 2, 3})
		s, ok := v.Slice()
		if !ok || len(s) != 3 {
			t.Fatal("Slice accessor failed")
		}
	})
}
