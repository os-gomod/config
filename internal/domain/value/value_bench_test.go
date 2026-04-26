package value

import (
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Pre-built test data (initialised once per benchmark via benchInit helpers)
// ---------------------------------------------------------------------------

// benchRawValues holds representative raw values of every supported type.
var benchRawValues = []any{
	nil,
	"hello world",
	"",
	42,
	int64(9223372036854775807),
	3.14159265358979,
	true,
	false,
	time.Duration(5 * time.Minute),
	time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
	[]any{"a", "b", "c", 1, 2, 3},
	[]string{"alpha", "bravo", "charlie"},
	[]int{10, 20, 30},
	[]float64{1.1, 2.2, 3.3},
	[]bool{true, false, true},
	[]byte("raw bytes payload"),
	map[string]any{
		"database": map[string]any{
			"host":     "localhost",
			"port":     5432,
			"password": "s3cret",
			"enabled":  true,
		},
		"cache": map[string]any{
			"ttl":   "30s",
			"max":   1000,
			"nodes": []any{"node1", "node2", "node3"},
		},
		"logging": map[string]any{
			"level": "info",
			"tags":  map[string]any{"service": "api", "env": "production"},
		},
	},
}

// benchLargeMap creates a map with n keys of mixed types for scale benchmarks.
func benchLargeMap(n int) map[string]Value {
	m := make(map[string]Value, n)
	for i := 0; i < n; i++ {
		switch i % 6 {
		case 0:
			m[fmt.Sprintf("string.key.%d", i)] = New(fmt.Sprintf("value-%d", i))
		case 1:
			m[fmt.Sprintf("int.key.%d", i)] = New(i)
		case 2:
			m[fmt.Sprintf("bool.key.%d", i)] = New(i%2 == 0)
		case 3:
			m[fmt.Sprintf("float.key.%d", i)] = New(float64(i) * 1.5)
		case 4:
			m[fmt.Sprintf("duration.key.%d", i)] = New(time.Duration(i) * time.Second)
		case 5:
			m[fmt.Sprintf("map.key.%d", i)] = New(map[string]any{
				"nested": fmt.Sprintf("deep-%d", i),
				"count":  i,
			})
		}
	}
	return m
}

// benchMutations creates a mix of set and delete mutations for ApplyDelta benchmarks.
func benchMutations(n int) []Mutation {
	muts := make([]Mutation, 0, n)
	for i := 0; i < n; i++ {
		if i%3 == 0 {
			muts = append(muts, NewDeleteMutation(fmt.Sprintf("delete.key.%d", i), "bench"))
		} else {
			muts = append(muts, NewSetMutation(
				fmt.Sprintf("set.key.%d", i),
				New(fmt.Sprintf("new-value-%d", i)),
				"bench",
			))
		}
	}
	return muts
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	for _, raw := range benchRawValues {
		b.Run(fmt.Sprintf("%T", raw), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = New(raw)
			}
		})
	}
}

func BenchmarkInferType(b *testing.B) {
	b.ReportAllocs()
	for _, raw := range benchRawValues {
		b.Run(fmt.Sprintf("%T", raw), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = inferType(raw)
			}
		})
	}
}

func BenchmarkComputeChecksum(b *testing.B) {
	b.ReportAllocs()

	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		data := benchLargeMap(size)
		// Pre-build Values outside the loop for pure checksum measurement.
		b.Run(fmt.Sprintf("map_%dkeys", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = ComputeMapChecksum(data)
			}
		})
	}

	// Also benchmark per-Value checksums for each type.
	for _, raw := range benchRawValues {
		if raw == nil {
			continue
		}
		v := New(raw)
		b.Run(fmt.Sprintf("value_%T", raw), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = v.ComputeChecksum()
			}
		})
	}
}

func BenchmarkCopy(b *testing.B) {
	b.ReportAllocs()

	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		data := benchLargeMap(size)
		b.Run(fmt.Sprintf("map_%dkeys", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Copy(data)
			}
		})
	}
}

func BenchmarkSortedKeys(b *testing.B) {
	b.ReportAllocs()

	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		data := benchLargeMap(size)
		b.Run(fmt.Sprintf("map_%dkeys", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = SortedKeys(data)
			}
		})
	}
}

func BenchmarkMerge(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name    string
		layers  int
		keysPer int
	}{
		{"2_layers_10_keys", 2, 10},
		{"2_layers_100_keys", 2, 100},
		{"5_layers_10_keys", 5, 10},
		{"5_layers_100_keys", 5, 100},
		{"10_layers_100_keys", 10, 100},
	}

	for _, tc := range cases {
		// Pre-build layer maps once.
		layers := make([]map[string]Value, tc.layers)
		for l := 0; l < tc.layers; l++ {
			layers[l] = benchLargeMap(tc.keysPer)
		}
		names := make([]string, tc.layers)
		for l := 0; l < tc.layers; l++ {
			names[l] = fmt.Sprintf("layer-%d", l)
		}

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = MergeWithLayerNames(layers, names)
			}
		})
	}
}

func BenchmarkApplyDelta(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name     string
		baseSize int
		mutCount int
	}{
		{"10_base_5_mutations", 10, 5},
		{"100_base_10_mutations", 100, 10},
		{"100_base_50_mutations", 100, 50},
		{"1000_base_100_mutations", 1000, 100},
	}

	for _, tc := range cases {
		base := benchLargeMap(tc.baseSize)
		muts := benchMutations(tc.mutCount)

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = ApplyDelta(base, muts)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Additional micro-benchmarks for core operations
// ---------------------------------------------------------------------------

func BenchmarkNewState(b *testing.B) {
	b.ReportAllocs()

	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		data := benchLargeMap(size)
		b.Run(fmt.Sprintf("map_%dkeys", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = NewState(data)
			}
		})
	}
}

func BenchmarkStateGet(b *testing.B) {
	b.ReportAllocs()

	data := benchLargeMap(1000)
	st := NewState(data)
	keys := SortedKeys(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = st.Get(keys[i%len(keys)])
	}
}

func BenchmarkStateSet(b *testing.B) {
	b.ReportAllocs()

	data := benchLargeMap(100)
	st := NewState(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st = st.Set(fmt.Sprintf("bench.key.%d", i), New(i))
	}
}

func BenchmarkStateMerge(b *testing.B) {
	b.ReportAllocs()

	data := benchLargeMap(100)
	st := NewState(data)
	overlay := benchLargeMap(50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st = st.Merge(overlay)
	}
}

func BenchmarkComputeDiff(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name      string
		baseSize  int
		changePct int // percentage of keys that differ
	}{
		{"100_keys_10pct_changed", 100, 10},
		{"100_keys_50pct_changed", 100, 50},
		{"1000_keys_10pct_changed", 1000, 10},
		{"1000_keys_50pct_changed", 1000, 50},
	}

	for _, tc := range cases {
		base := benchLargeMap(tc.baseSize)
		// Create a new map with some changed keys.
		newData := Copy(base)
		keys := SortedKeys(base)
		changeCount := tc.baseSize * tc.changePct / 100
		for j := 0; j < changeCount; j++ {
			newData[keys[j]] = New(fmt.Sprintf("changed-%d", j))
		}

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = ComputeDiff(base, newData)
			}
		})
	}
}

func BenchmarkIsSecret(b *testing.B) {
	b.ReportAllocs()

	keys := []string{
		"database.host",
		"database.password",
		"api.token",
		"api_key",
		"logging.level",
		"server.port",
		"auth.private_key",
		"vault.credential",
		"cache.ttl",
		"secret.sauce",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			_ = IsSecret(key)
		}
	}
}

func BenchmarkEqual(b *testing.B) {
	b.ReportAllocs()

	v1 := New(map[string]any{"a": 1, "b": "two", "c": true, "d": []any{1, 2, 3}})
	v2 := New(map[string]any{"a": 1, "b": "two", "c": true, "d": []any{1, 2, 3}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v1.Equal(v2)
	}
}
