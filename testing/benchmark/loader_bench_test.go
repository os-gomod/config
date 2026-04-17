package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/os-gomod/config/binder"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/event"
	"github.com/os-gomod/config/loader"
)

// ---------------------------------------------------------------------------
// Helper: build a map of N flat key-value pairs.
// ---------------------------------------------------------------------------

func buildValueMap(n int) map[string]value.Value {
	m := make(map[string]value.Value, n)
	for i := 0; i < n; i++ {
		m["key."+itoa(i)] = value.New(
			"value-"+itoa(i),
			value.TypeString,
			value.SourceMemory,
			20,
		)
	}
	return m
}

func buildAnyMap(n int) map[string]any {
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		m["key."+itoa(i)] = "value-" + itoa(i)
	}
	return m
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// ---------------------------------------------------------------------------
// BenchmarkMemoryLoaderLoad
// ---------------------------------------------------------------------------

func BenchmarkMemoryLoaderLoad(b *testing.B) {
	ml := loader.NewMemoryLoader(loader.WithMemoryData(buildAnyMap(1000)))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ml.Load(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// BenchmarkFileLoaderLoad
// ---------------------------------------------------------------------------

func BenchmarkFileLoaderLoad(b *testing.B) {
	// Create a temp YAML file with 500 keys.
	tmpDir := b.TempDir()
	yamlContent := generateYAML(500)
	path := filepath.Join(tmpDir, "bench.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		b.Fatal(err)
	}

	fl := loader.NewFileLoader(path, loader.WithFilePriority(30))
	ctx := context.Background()

	// Warm up once so the checksum cache is populated.
	_, _ = fl.Load(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := fl.Load(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func generateYAML(n int) string {
	var s string
	for i := 0; i < n; i++ {
		s += "key_" + itoa(i) + ": value_" + itoa(i) + "\n"
	}
	return s
}

// ---------------------------------------------------------------------------
// BenchmarkEventBusPublish
// ---------------------------------------------------------------------------

func BenchmarkEventBusPublish(b *testing.B) {
	ctx := context.Background()

	b.Run("10 subscribers", func(b *testing.B) {
		bus := event.NewBus()
		for i := 0; i < 10; i++ {
			bus.Subscribe("*", func(_ context.Context, _ event.Event) error { return nil })
		}
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bus.Publish(ctx, &evt)
		}
	})

	b.Run("100 subscribers", func(b *testing.B) {
		bus := event.NewBus()
		for i := 0; i < 100; i++ {
			bus.Subscribe("*", func(_ context.Context, _ event.Event) error { return nil })
		}
		evt := event.New(event.TypeCreate, "bench.key")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bus.Publish(ctx, &evt)
		}
	})
}

// ---------------------------------------------------------------------------
// BenchmarkValueCopy
// ---------------------------------------------------------------------------

func BenchmarkValueCopy(b *testing.B) {
	src := buildValueMap(1000)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = value.Copy(src)
	}
}

// ---------------------------------------------------------------------------
// BenchmarkMerge
// ---------------------------------------------------------------------------

func BenchmarkMerge(b *testing.B) {
	m1 := buildValueMap(500)
	m2 := buildValueMap(500)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		value.Merge(m1, m2)
	}
}

func BenchmarkMergeMany(b *testing.B) {
	maps := make([]map[string]value.Value, 5)
	for i := range maps {
		maps[i] = buildValueMap(200)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		value.Merge(maps...)
	}
}

// ---------------------------------------------------------------------------
// BenchmarkBindStruct
// ---------------------------------------------------------------------------

type benchConfig struct {
	Name    string `config:"name"`
	Port    int    `config:"port"`
	Enabled bool   `config:"enabled"`
	Host    string `config:"host"`
	Debug   bool   `config:"debug"`
	Timeout int    `config:"timeout"`
}

func BenchmarkBindStruct(b *testing.B) {
	data := map[string]value.Value{
		"name":    value.New("bench-app", value.TypeString, value.SourceMemory, 10),
		"port":    value.New(8080, value.TypeInt, value.SourceMemory, 10),
		"enabled": value.New(true, value.TypeBool, value.SourceMemory, 10),
		"host":    value.New("localhost", value.TypeString, value.SourceMemory, 10),
		"debug":   value.New(false, value.TypeBool, value.SourceMemory, 10),
		"timeout": value.New(30, value.TypeInt, value.SourceMemory, 10),
	}
	bnd := binder.New()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg benchConfig
		if err := bnd.Bind(ctx, data, &cfg); err != nil {
			b.Fatal(err)
		}
	}
}
