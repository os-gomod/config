package value

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Value creation
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		v := New("hello")
		assert.Equal(t, "hello", v.Raw())
		assert.Equal(t, TypeString, v.Type())
		assert.Equal(t, SourceUnknown, v.Source())
		assert.Equal(t, 0, v.Priority())
	})

	t.Run("int", func(t *testing.T) {
		v := New(42)
		assert.Equal(t, 42, v.Raw())
		assert.Equal(t, TypeInt, v.Type())
	})

	t.Run("nil", func(t *testing.T) {
		v := New(nil)
		assert.Nil(t, v.Raw())
		assert.Equal(t, TypeUnknown, v.Type())
	})

	t.Run("bool", func(t *testing.T) {
		v := New(true)
		assert.Equal(t, TypeBool, v.Type())
	})

	t.Run("float64", func(t *testing.T) {
		v := New(3.14)
		assert.Equal(t, TypeFloat64, v.Type())
	})
}

func TestNewInMemory(t *testing.T) {
	v := NewInMemory("test", 10)
	assert.Equal(t, "test", v.Raw())
	assert.Equal(t, SourceMemory, v.Source())
	assert.Equal(t, 10, v.Priority())
	assert.Equal(t, TypeString, v.Type())
}

func TestFromRaw(t *testing.T) {
	v := FromRaw("hello", TypeString, SourceFile, 5)
	assert.Equal(t, "hello", v.Raw())
	assert.Equal(t, TypeString, v.Type())
	assert.Equal(t, SourceFile, v.Source())
	assert.Equal(t, 5, v.Priority())
}

// ---------------------------------------------------------------------------
// Type inference
// ---------------------------------------------------------------------------

func TestInferType(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want Type
	}{
		{"string", "hello", TypeString},
		{"int", 42, TypeInt},
		{"int64", int64(42), TypeInt64},
		{"float64", 3.14, TypeFloat64},
		{"bool", true, TypeBool},
		{"duration", time.Minute, TypeDuration},
		{"time", time.Now(), TypeTime},
		{"slice_any", []any{1, 2}, TypeSlice},
		{"slice_string", []string{"a"}, TypeSlice},
		{"slice_int", []int{1}, TypeSlice},
		{"map_any", map[string]any{}, TypeMap},
		{"map_string", map[string]string{}, TypeMap},
		{"bytes", []byte("hello"), TypeBytes},
		{"nil", nil, TypeUnknown},
		{"unknown_struct", struct{}{}, TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.raw).InferType()
			assert.Equal(t, tt.want, got)
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
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.String())
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
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.src.String())
		})
	}
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

func TestString(t *testing.T) {
	t.Run("string_value", func(t *testing.T) {
		v := New("hello")
		assert.Equal(t, "hello", v.String())
	})

	t.Run("nil", func(t *testing.T) {
		v := New(nil)
		assert.Equal(t, "", v.String())
	})

	t.Run("int_value", func(t *testing.T) {
		v := New(42)
		assert.Equal(t, "42", v.String())
	})
}

func TestBool(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want bool
	}{
		{"bool_true", true, true},
		{"bool_false", false, false},
		{"string_true", "true", true},
		{"string_false", "false", false},
		{"string_1", "1", true},
		{"string_invalid", "yes", false},
		{"int_nonzero", 1, true},
		{"int_zero", 0, false},
		{"int64_nonzero", int64(1), true},
		{"float64_nonzero", 1.0, true},
		{"float64_zero", 0.0, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, New(tt.raw).Bool())
		})
	}
}

func TestInt(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want int
	}{
		{"int", 42, 42},
		{"int64", int64(42), 42},
		{"float64", 42.7, 42},
		{"string_valid", "42", 42},
		{"string_invalid", "abc", 0},
		{"bool_true", true, 1},
		{"bool_false", false, 0},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, New(tt.raw).Int())
		})
	}
}

func TestInt64(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want int64
	}{
		{"int", 42, int64(42)},
		{"int64", int64(42), int64(42)},
		{"float64", 42.7, int64(42)},
		{"string_valid", "42", int64(42)},
		{"string_invalid", "abc", int64(0)},
		{"nil", nil, int64(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, New(tt.raw).Int64())
		})
	}
}

func TestFloat64(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want float64
	}{
		{"float64", 3.14, 3.14},
		{"int", 42, 42.0},
		{"int64", int64(42), 42.0},
		{"string_valid", "3.14", 3.14},
		{"string_invalid", "abc", 0.0},
		{"bool_true", true, 1.0},
		{"nil", nil, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, New(tt.raw).Float64())
		})
	}
}

func TestDuration(t *testing.T) {
	t.Run("duration", func(t *testing.T) {
		d := 5 * time.Minute
		v := New(d)
		assert.Equal(t, d, v.Duration())
	})

	t.Run("int64", func(t *testing.T) {
		v := New(int64(5000000000)) // 5 seconds in ns
		assert.Equal(t, 5*time.Second, v.Duration())
	})

	t.Run("string_valid", func(t *testing.T) {
		v := New("5s")
		assert.Equal(t, 5*time.Second, v.Duration())
	})

	t.Run("string_invalid", func(t *testing.T) {
		v := New("not a duration")
		assert.Equal(t, time.Duration(0), v.Duration())
	})

	t.Run("nil", func(t *testing.T) {
		v := New(nil)
		assert.Equal(t, time.Duration(0), v.Duration())
	})
}

func TestTime(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("time_value", func(t *testing.T) {
		v := New(now)
		got := v.Time()
		assert.True(t, got.Equal(now))
	})

	t.Run("rfc3339", func(t *testing.T) {
		s := now.Format(time.RFC3339)
		v := New(s)
		got := v.Time()
		assert.True(t, got.Equal(now))
	})

	t.Run("date_only", func(t *testing.T) {
		v := New("2024-01-15")
		got := v.Time()
		assert.Equal(t, 2024, got.Year())
		assert.Equal(t, time.January, got.Month())
		assert.Equal(t, 15, got.Day())
	})

	t.Run("nil", func(t *testing.T) {
		v := New(nil)
		assert.True(t, v.Time().IsZero())
	})

	t.Run("invalid_string", func(t *testing.T) {
		v := New("not a date")
		assert.True(t, v.Time().IsZero())
	})
}

func TestSlice(t *testing.T) {
	t.Run("[]any", func(t *testing.T) {
		v := New([]any{1, "two", true})
		got := v.Slice()
		require.Len(t, got, 3)
		assert.Equal(t, 1, got[0])
		assert.Equal(t, "two", got[1])
	})

	t.Run("[]string", func(t *testing.T) {
		v := New([]string{"a", "b"})
		got := v.Slice()
		require.Len(t, got, 2)
		assert.Equal(t, "a", got[0])
		assert.Equal(t, "b", got[1])
	})

	t.Run("[]int", func(t *testing.T) {
		v := New([]int{1, 2, 3})
		got := v.Slice()
		require.Len(t, got, 3)
		assert.Equal(t, 1, got[0])
	})

	t.Run("[]float64", func(t *testing.T) {
		v := New([]float64{1.1, 2.2})
		got := v.Slice()
		require.Len(t, got, 2)
	})

	t.Run("[]bool", func(t *testing.T) {
		v := New([]bool{true, false})
		got := v.Slice()
		require.Len(t, got, 2)
		assert.Equal(t, true, got[0])
	})

	t.Run("nil", func(t *testing.T) {
		v := New(nil)
		assert.Nil(t, v.Slice())
	})

	t.Run("non_slice", func(t *testing.T) {
		v := New("hello")
		assert.Nil(t, v.Slice())
	})
}

func TestMap(t *testing.T) {
	t.Run("map[string]any", func(t *testing.T) {
		v := New(map[string]any{"a": 1})
		got := v.Map()
		require.NotNil(t, got)
		assert.Equal(t, 1, got["a"])
	})

	t.Run("map[string]string", func(t *testing.T) {
		v := New(map[string]string{"a": "b"})
		got := v.Map()
		require.NotNil(t, got)
		assert.Equal(t, "b", got["a"])
	})

	t.Run("nil", func(t *testing.T) {
		v := New(nil)
		assert.Nil(t, v.Map())
	})

	t.Run("non_map", func(t *testing.T) {
		v := New("hello")
		assert.Nil(t, v.Map())
	})
}

func TestBytes(t *testing.T) {
	t.Run("[]byte", func(t *testing.T) {
		b := []byte("hello")
		v := New(b)
		got := v.Bytes()
		assert.Equal(t, b, got)
	})

	t.Run("string", func(t *testing.T) {
		v := New("hello")
		got := v.Bytes()
		assert.Equal(t, []byte("hello"), got)
	})

	t.Run("nil", func(t *testing.T) {
		v := New(nil)
		assert.Nil(t, v.Bytes())
	})

	t.Run("non_bytes", func(t *testing.T) {
		v := New(42)
		assert.Nil(t, v.Bytes())
	})
}

// ---------------------------------------------------------------------------
// Equal
// ---------------------------------------------------------------------------

func TestEqual(t *testing.T) {
	tests := []struct {
		name  string
		a, b  Value
		equal bool
	}{
		{
			"same_string",
			New("hello"), New("hello"), true,
		},
		{
			"diff_string",
			New("hello"), New("world"), false,
		},
		{
			"diff_type",
			New("hello"), New(42), false,
		},
		{
			"diff_source",
			FromRaw("x", TypeString, SourceFile, 0),
			FromRaw("x", TypeString, SourceEnv, 0),
			false,
		},
		{
			"diff_priority",
			FromRaw("x", TypeString, SourceMemory, 1),
			FromRaw("x", TypeString, SourceMemory, 2),
			false,
		},
		{
			"same_int",
			New(42), New(42), true,
		},
		{
			"same_float64",
			New(3.14), New(3.14), true,
		},
		{
			"same_bool",
			New(true), New(true), true,
		},
		{
			"same_map",
			New(map[string]any{"a": 1}), New(map[string]any{"a": 1}), true,
		},
		{
			"diff_map",
			New(map[string]any{"a": 1}), New(map[string]any{"a": 2}), false,
		},
		{
			"same_slice",
			New([]any{1, 2}), New([]any{1, 2}), true,
		},
		{
			"diff_slice",
			New([]any{1, 2}), New([]any{1, 3}), false,
		},
		{
			"same_bytes",
			New([]byte("abc")), New([]byte("abc")), true,
		},
		{
			"diff_bytes",
			New([]byte("abc")), New([]byte("abd")), false,
		},
		{
			"same_duration",
			New(5 * time.Minute), New(5 * time.Minute), true,
		},
		{
			"nil_nil",
			New(nil), New(nil), true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.equal, tt.a.Equal(tt.b))
		})
	}
}

// ---------------------------------------------------------------------------
// IsZero
// ---------------------------------------------------------------------------

func TestIsZero(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		zero bool
	}{
		{"nil", nil, true},
		{"empty_string", "", true},
		{"nonempty_string", "hello", false},
		{"zero_int", 0, true},
		{"nonzero_int", 1, false},
		{"zero_int64", int64(0), true},
		{"nonzero_int64", int64(1), false},
		{"zero_float64", 0.0, true},
		{"nonzero_float64", 1.0, false},
		{"false_bool", false, true},
		{"true_bool", true, false},
		{"zero_duration", time.Duration(0), true},
		{"nonzero_duration", time.Minute, false},
		{"zero_time", time.Time{}, true},
		{"nonzero_time", time.Now(), false},
		{"empty_slice", []any{}, true},
		{"nonempty_slice", []any{1}, false},
		{"empty_map", map[string]any{}, true},
		{"nonempty_map", map[string]any{"a": 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.zero, New(tt.raw).IsZero())
		})
	}
}

// ---------------------------------------------------------------------------
// Generic As[T]
// ---------------------------------------------------------------------------

func TestAs_Generic(t *testing.T) {
	t.Run("correct_type", func(t *testing.T) {
		v := New("hello")
		s, ok := As[string](v)
		assert.True(t, ok)
		assert.Equal(t, "hello", s)
	})

	t.Run("wrong_type", func(t *testing.T) {
		v := New("hello")
		i, ok := As[int](v)
		assert.False(t, ok)
		assert.Equal(t, 0, i)
	})

	t.Run("nil_value", func(t *testing.T) {
		v := New(nil)
		s, ok := As[string](v)
		assert.False(t, ok)
		assert.Equal(t, "", s)
	})

	t.Run("int_type", func(t *testing.T) {
		v := New(42)
		i, ok := As[int](v)
		assert.True(t, ok)
		assert.Equal(t, 42, i)
	})
}

// ---------------------------------------------------------------------------
// ComputeChecksum
// ---------------------------------------------------------------------------

func TestComputeChecksum(t *testing.T) {
	t.Run("same_value_same_checksum", func(t *testing.T) {
		v1 := New("hello")
		v2 := New("hello")
		assert.Equal(t, v1.ComputeChecksum(), v2.ComputeChecksum())
	})

	t.Run("diff_value_diff_checksum", func(t *testing.T) {
		v1 := New("hello")
		v2 := New("world")
		assert.NotEqual(t, v1.ComputeChecksum(), v2.ComputeChecksum())
	})

	t.Run("nil_checksum", func(t *testing.T) {
		v := New(nil)
		assert.NotEmpty(t, v.ComputeChecksum())
	})

	t.Run("map_deterministic", func(t *testing.T) {
		v1 := New(map[string]any{"b": 2, "a": 1})
		v2 := New(map[string]any{"a": 1, "b": 2})
		assert.Equal(t, v1.ComputeChecksum(), v2.ComputeChecksum())
	})

	t.Run("float64_checksum", func(t *testing.T) {
		v := New(3.14)
		chk := v.ComputeChecksum()
		assert.NotEmpty(t, chk)
		// Same float should produce same checksum
		v2 := New(3.14)
		assert.Equal(t, chk, v2.ComputeChecksum())
	})
}

func TestComputeMapChecksum(t *testing.T) {
	t.Run("same_map_same_checksum", func(t *testing.T) {
		m1 := map[string]Value{"a": New(1), "b": New(2)}
		m2 := map[string]Value{"a": New(1), "b": New(2)}
		assert.Equal(t, ComputeMapChecksum(m1), ComputeMapChecksum(m2))
	})

	t.Run("different_order_same_checksum", func(t *testing.T) {
		m1 := map[string]Value{"b": New(2), "a": New(1)}
		m2 := map[string]Value{"a": New(1), "b": New(2)}
		assert.Equal(t, ComputeMapChecksum(m1), ComputeMapChecksum(m2))
	})

	t.Run("nil_map", func(t *testing.T) {
		chk := ComputeMapChecksum(nil)
		assert.NotEmpty(t, chk)
	})

	t.Run("empty_map", func(t *testing.T) {
		chk := ComputeMapChecksum(map[string]Value{})
		assert.NotEmpty(t, chk)
	})
}

// ---------------------------------------------------------------------------
// Copy
// ---------------------------------------------------------------------------

func TestCopy(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, Copy(nil))
	})

	t.Run("copies_data", func(t *testing.T) {
		orig := map[string]Value{"a": New(1)}
		cp := Copy(orig)
		assert.Equal(t, orig, cp)

		// Modify copy should not affect original
		cp["b"] = New(2)
		assert.NotContains(t, orig, "b")
	})
}

// ---------------------------------------------------------------------------
// SortedKeys
// ---------------------------------------------------------------------------

func TestSortedKeys(t *testing.T) {
	t.Run("sorted", func(t *testing.T) {
		data := map[string]Value{"c": New(3), "a": New(1), "b": New(2)}
		keys := SortedKeys(data)
		assert.Equal(t, []string{"a", "b", "c"}, keys)
	})

	t.Run("nil", func(t *testing.T) {
		assert.Empty(t, SortedKeys(nil))
	})

	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, SortedKeys(map[string]Value{}))
	})
}

// ---------------------------------------------------------------------------
// IsSecret
// ---------------------------------------------------------------------------

func TestIsSecret(t *testing.T) {
	tests := []struct {
		key    string
		secret bool
	}{
		{"password", true},
		{"db.password", true},
		{"PASSWORD", true},
		{"my_secret_key", true},
		{"api_token", true},
		{"app_api_key", true},
		{"user_apikey", true},
		{"private_key", true},
		{"tls.private_key", true},
		{"credential", true},
		{"database_url", false},
		{"host", false},
		{"port", false},
		{"timeout", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.secret, IsSecret(tt.key))
		})
	}
}

// ---------------------------------------------------------------------------
// Redact / RedactMap
// ---------------------------------------------------------------------------

func TestRedact(t *testing.T) {
	t.Run("secret_key", func(t *testing.T) {
		v := New("my-super-secret")
		redacted := v.Redact("db.password")
		assert.Equal(t, "***REDACTED***", redacted.String())
		assert.Equal(t, TypeString, redacted.Type())
	})

	t.Run("non_secret_key", func(t *testing.T) {
		v := New("hello")
		redacted := v.Redact("host")
		assert.Equal(t, "hello", redacted.String())
		assert.True(t, v.Equal(redacted))
	})
}

func TestRedactMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, RedactMap(nil))
	})

	t.Run("redacts_secrets", func(t *testing.T) {
		data := map[string]Value{
			"host":     New("localhost"),
			"password": New("secret123"),
			"port":     New(5432),
		}
		redacted := RedactMap(data)

		assert.Equal(t, "localhost", redacted["host"].String())
		assert.Equal(t, "***REDACTED***", redacted["password"].String())
		assert.Equal(t, 5432, redacted["port"].Int())
	})

	t.Run("preserves_source_and_priority", func(t *testing.T) {
		v := FromRaw("secret", TypeString, SourceVault, 10)
		data := map[string]Value{"api_key": v}
		redacted := RedactMap(data)

		assert.Equal(t, SourceVault, redacted["api_key"].Source())
		assert.Equal(t, 10, redacted["api_key"].Priority())
	})
}

// ---------------------------------------------------------------------------
// Float64FromAny
// ---------------------------------------------------------------------------

func TestFloat64FromAny(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want float64
		ok   bool
	}{
		{"float64", 3.14, 3.14, true},
		{"int", 42, 42.0, true},
		{"int64", int64(42), 42.0, true},
		{"uint", uint(42), 42.0, true},
		{"uint64", uint64(42), 42.0, true},
		{"uint32", uint32(42), 42.0, true},
		{"int32", int32(42), 42.0, true},
		{"string", "hello", math.NaN(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Float64FromAny(tt.raw)
			if tt.ok {
				assert.True(t, ok)
				assert.Equal(t, tt.want, got)
			} else {
				assert.False(t, ok)
				assert.True(t, math.IsNaN(got))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NumericCoerce
// ---------------------------------------------------------------------------

func TestNumericCoerce(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want float64
	}{
		{"float64", 3.14, 3.14},
		{"int", 42, 42.0},
		{"string_valid", "3.14", 3.14},
		{"string_invalid", "abc", math.NaN()},
		{"bool_true", true, 1.0},
		{"bool_false", false, 0.0},
		{"nil", nil, math.NaN()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NumericCoerce(tt.raw)
			if math.IsNaN(tt.want) {
				assert.True(t, math.IsNaN(got))
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GoString
// ---------------------------------------------------------------------------

func TestGoString(t *testing.T) {
	v := FromRaw("hello", TypeString, SourceFile, 5)
	s := v.GoString()
	assert.Contains(t, s, "value.Value{")
	assert.Contains(t, s, "hello")
	assert.Contains(t, s, "string")
	assert.Contains(t, s, "file")
	assert.Contains(t, s, "5")
}
