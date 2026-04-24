package testutil

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ------------------------------------------------------------------
// ConfigBuilder
// ------------------------------------------------------------------
func TestConfigBuilder_WithMemory(t *testing.T) {
	e := NewBuilder().WithMemory(map[string]any{"a": 1}).Build()
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	e.Close(testCtx())
}

func TestConfigBuilder_WithValue(t *testing.T) {
	e := NewBuilder().WithValue("k", "v").Build()
	// Build() creates an engine but doesn't call Reload(),
	// so we need to reload first to get data
	_, _ = e.Reload(testCtx())
	if !e.Has("k") {
		t.Error("expected key k to exist after reload")
	}
	e.Close(testCtx())
}

func TestConfigBuilder_BuildWithLayers(t *testing.T) {
	e := NewBuilder().BuildWithLayers()
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	e.Close(testCtx())
}

// ------------------------------------------------------------------
// GenerateNKeys
// ------------------------------------------------------------------
func TestGenerateNKeys(t *testing.T) {
	m := GenerateNKeys(10)
	if len(m) != 10 {
		t.Errorf("expected 10 keys, got %d", len(m))
	}
	if m["key.0000"] != "value.0000" {
		t.Errorf("expected key.0000=value.0000, got %v", m["key.0000"])
	}
}

func TestGenerateNKeys_Zero(t *testing.T) {
	m := GenerateNKeys(0)
	if len(m) != 0 {
		t.Errorf("expected 0 keys, got %d", len(m))
	}
}

// ------------------------------------------------------------------
// GenerateNValues
// ------------------------------------------------------------------
func TestGenerateNValues(t *testing.T) {
	m := GenerateNValues(5)
	if len(m) != 5 {
		t.Errorf("expected 5 values, got %d", len(m))
	}
}

func TestGenerateNValues_Zero(t *testing.T) {
	m := GenerateNValues(0)
	if len(m) != 0 {
		t.Errorf("expected 0 values, got %d", len(m))
	}
}

// ------------------------------------------------------------------
// ChaosInjector
// ------------------------------------------------------------------
func TestChaosInjector_MaxFailures(t *testing.T) {
	c := NewChaosInjector(3, 0)
	assert.True(t, c.ShouldFail(), "call 1")
	assert.True(t, c.ShouldFail(), "call 2")
	assert.True(t, c.ShouldFail(), "call 3")
	assert.False(t, c.ShouldFail(), "call 4")
	assert.Equal(t, int64(3), c.FailCount())
	assert.Equal(t, int64(4), c.CallCount())
}

func TestChaosInjector_FailRate(t *testing.T) {
	c := NewChaosInjector(0, 0.5)
	total := 1000
	fails := 0
	for i := 0; i < total; i++ {
		if c.ShouldFail() {
			fails++
		}
	}
	if fails < 300 || fails > 700 {
		t.Errorf("expected ~50%% failures, got %d/%d", fails, total)
	}
}

func TestChaosInjector_NoFailures(t *testing.T) {
	c := NewChaosInjector(0, 0)
	for i := 0; i < 100; i++ {
		if c.ShouldFail() {
			t.Error("expected no failures")
		}
	}
}

func TestChaosInjector_Reset(t *testing.T) {
	c := NewChaosInjector(5, 0)
	c.ShouldFail()
	c.ShouldFail()
	assert.Equal(t, int64(2), c.CallCount())
	assert.Equal(t, int64(2), c.FailCount())

	c.Reset()
	assert.Equal(t, int64(0), c.CallCount())
	assert.Equal(t, int64(0), c.FailCount())
}

// ------------------------------------------------------------------
// AssertNoError / AssertError / AssertEqual
// ------------------------------------------------------------------
func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil, "should not error")
}

func TestAssertError(t *testing.T) {
	AssertError(t, fmt.Errorf("test"), "expected error")
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, 42, 42, "should be equal")
}

// ------------------------------------------------------------------
// GoldenComparer
// ------------------------------------------------------------------
func TestGoldenComparer(t *testing.T) {
	gc := NewGoldenComparer(t, "/tmp/golden", false)
	if gc == nil {
		t.Fatal("expected non-nil GoldenComparer")
	}
}

func TestNewGoldenComparer_Update(t *testing.T) {
	gc := NewGoldenComparer(t, "/tmp/golden", true)
	if gc == nil {
		t.Fatal("expected non-nil GoldenComparer")
	}
}

// ------------------------------------------------------------------
// Concurrency test for ConfigBuilder
// ------------------------------------------------------------------
func TestConfigBuilder_ConcurrentBuilds(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := NewBuilder().WithValue("k", "v").Build()
			e.Close(testCtx())
		}()
	}
	wg.Wait()
}

// ------------------------------------------------------------------
// test helper
// ------------------------------------------------------------------
func testCtx() context.Context {
	return context.Background()
}
