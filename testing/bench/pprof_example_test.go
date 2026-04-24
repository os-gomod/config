package bench

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"testing"

	"github.com/os-gomod/config"
	"github.com/os-gomod/config/loader"
)

// TestProfile_Reload demonstrates profiling a 10K key reload.
// Run with:
//
//	go test -run=TestProfile_Reload -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./bench/
func TestProfile_Reload(t *testing.T) {
	// CPU profiling
	f, err := os.Create("cpu.prof")
	if err != nil {
		t.Fatalf("create cpu.prof: %v", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		t.Fatalf("start cpu profile: %v", err)
	}
	defer func() {
		pprof.StopCPUProfile()
		f.Close()
	}()

	ctx := context.Background()
	data := make(map[string]any, 10000)
	for i := 0; i < 10000; i++ {
		data[fmt.Sprintf("key.%04d", i)] = fmt.Sprintf("value.%04d", i)
	}
	ml := loader.NewMemoryLoader(loader.WithMemoryData(data))
	cfg, err := config.New(ctx, config.WithLoader(ml))
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	for i := 0; i < 1000; i++ {
		_, err := cfg.Reload(ctx)
		if err != nil {
			t.Fatalf("reload %d: %v", i, err)
		}
	}
	if err := cfg.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
}
