package fuzz

import (
	"encoding/json"
	"testing"

	"github.com/os-gomod/config/core/value"
)

// ---------------------------------------------------------------------------
// FuzzValueNew
// ---------------------------------------------------------------------------

func FuzzValueNew(f *testing.F) {
	// Seed with JSON-encoded raw values (valid fuzz type is []byte).
	seeds := []string{
		`"hello"`,
		`42`,
		`99`,
		`3.14`,
		`true`,
		`false`,
		`""`,
		`["a", "b"]`,
		`{"k": "v"}`,
		`"bytes"`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var raw any
		if err := json.Unmarshal(data, &raw); err != nil {
			return // skip invalid JSON
		}
		v := value.New(raw, value.InferType(raw), value.SourceMemory, 20)
		// Verify the value is usable — must not panic.
		_ = v.Raw()
		_ = v.Type()
		_ = v.Source()
		_ = v.Priority()
		_ = v.String()
	})
}

// ---------------------------------------------------------------------------
// FuzzValueEqual
// ---------------------------------------------------------------------------

func FuzzValueEqual(f *testing.F) {
	// Seed with pairs of same-type JSON values encoded as two byte slices.
	seedPairs := [][2]string{
		{`"hello"`, `"hello"`},
		{`"hello"`, `"world"`},
		{`42`, `42`},
		{`42`, `43`},
		{`true`, `true`},
		{`true`, `false`},
		{`""`, `""`},
		{`1.0`, `1.0`},
		{`1.0`, `2.0`},
	}
	for _, pair := range seedPairs {
		f.Add([]byte(pair[0]), []byte(pair[1]))
	}

	f.Fuzz(func(t *testing.T, dataA, dataB []byte) {
		var a, b any
		if err := json.Unmarshal(dataA, &a); err != nil {
			return
		}
		if err := json.Unmarshal(dataB, &b); err != nil {
			return
		}

		typA := value.InferType(a)
		typB := value.InferType(b)
		va := value.New(a, typA, value.SourceMemory, 20)
		vb := value.New(b, typB, value.SourceMemory, 20)

		eq := va.Equal(vb)
		eqRev := vb.Equal(va)

		// Symmetry: Equal should be symmetric.
		if eq != eqRev {
			t.Errorf("Equal not symmetric: a.Equal(b)=%v, b.Equal(a)=%v", eq, eqRev)
		}

		// If types differ, must not be equal.
		if typA != typB && eq {
			t.Errorf("values with different types should not be equal: %v vs %v", typA, typB)
		}
	})
}
