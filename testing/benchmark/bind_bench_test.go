package benchmark

import (
	"context"
	"testing"

	"github.com/os-gomod/config/binder"
	"github.com/os-gomod/config/core/value"
)

// ---------------------------------------------------------------------------
// BenchmarkBindSimpleStruct
// ---------------------------------------------------------------------------

type simpleBenchStruct struct {
	Name  string `config:"name"`
	Count int    `config:"count"`
	Flag  bool   `config:"flag"`
}

func BenchmarkBindSimpleStruct(b *testing.B) {
	data := map[string]value.Value{
		"name":  value.New("simple", value.TypeString, value.SourceMemory, 10),
		"count": value.New(42, value.TypeInt, value.SourceMemory, 10),
		"flag":  value.New(true, value.TypeBool, value.SourceMemory, 10),
	}
	bnd := binder.New()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg simpleBenchStruct
		if err := bnd.Bind(ctx, data, &cfg); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// BenchmarkBindNestedStruct
// ---------------------------------------------------------------------------

type nestedBenchStruct struct {
	App    string            `config:"app"`
	Server nestedBenchServer `config:"server"`
	DB     nestedBenchDB     `config:"db"`
}

type nestedBenchServer struct {
	Host string `config:"host"`
	Port int    `config:"port"`
	TLS  bool   `config:"tls"`
}

type nestedBenchDB struct {
	Driver   string `config:"driver"`
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Password string `config:"password"`
}

func BenchmarkBindNestedStruct(b *testing.B) {
	data := map[string]value.Value{
		"app":         value.New("nested-app", value.TypeString, value.SourceMemory, 10),
		"server.host": value.New("0.0.0.0", value.TypeString, value.SourceMemory, 10),
		"server.port": value.New(443, value.TypeInt, value.SourceMemory, 10),
		"server.tls":  value.New(true, value.TypeBool, value.SourceMemory, 10),
		"db.driver":   value.New("postgres", value.TypeString, value.SourceMemory, 10),
		"db.host":     value.New("db.local", value.TypeString, value.SourceMemory, 10),
		"db.port":     value.New(5432, value.TypeInt, value.SourceMemory, 10),
		"db.password": value.New("secret", value.TypeString, value.SourceMemory, 10),
	}
	bnd := binder.New()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg nestedBenchStruct
		if err := bnd.Bind(ctx, data, &cfg); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// BenchmarkBindLargeStruct (30+ fields)
// ---------------------------------------------------------------------------

type largeBenchStruct struct {
	Field01 string `config:"field01"`
	Field02 int    `config:"field02"`
	Field03 bool   `config:"field03"`
	Field04 string `config:"field04"`
	Field05 int    `config:"field05"`
	Field06 bool   `config:"field06"`
	Field07 string `config:"field07"`
	Field08 int    `config:"field08"`
	Field09 bool   `config:"field09"`
	Field10 string `config:"field10"`
	Field11 int    `config:"field11"`
	Field12 bool   `config:"field12"`
	Field13 string `config:"field13"`
	Field14 int    `config:"field14"`
	Field15 bool   `config:"field15"`
	Field16 string `config:"field16"`
	Field17 int    `config:"field17"`
	Field18 bool   `config:"field18"`
	Field19 string `config:"field19"`
	Field20 int    `config:"field20"`
	Field21 bool   `config:"field21"`
	Field22 string `config:"field22"`
	Field23 int    `config:"field23"`
	Field24 bool   `config:"field24"`
	Field25 string `config:"field25"`
	Field26 int    `config:"field26"`
	Field27 bool   `config:"field27"`
	Field28 string `config:"field28"`
	Field29 int    `config:"field29"`
	Field30 bool   `config:"field30"`
	Field31 string `config:"field31"`
	Field32 int    `config:"field32"`
}

func buildLargeData() map[string]value.Value {
	// Alternate string, int, bool values for 32 fields.
	data := make(map[string]value.Value, 32)
	types := []value.Type{
		value.TypeString, value.TypeInt, value.TypeBool,
	}
	vals := []any{"str-val", 100, true}
	for i := 1; i <= 32; i++ {
		key := "field" + itoa2(i)
		idx := (i - 1) % 3
		data[key] = value.New(vals[idx], types[idx], value.SourceMemory, 10)
	}
	return data
}

func itoa2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return itoa2(n/10) + string(rune('0'+n%10))
}

func BenchmarkBindLargeStruct(b *testing.B) {
	data := buildLargeData()
	bnd := binder.New()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg largeBenchStruct
		if err := bnd.Bind(ctx, data, &cfg); err != nil {
			b.Fatal(err)
		}
	}
}
