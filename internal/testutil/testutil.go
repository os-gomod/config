package testutil

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/config/core"
	"github.com/os-gomod/config/core/value"
	"github.com/os-gomod/config/loader"
)

// ConfigBuilder provides a fluent API for building test configurations.
type ConfigBuilder struct {
	memoryData map[string]any
	layers     []*core.Layer
}

func NewBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		memoryData: make(map[string]any),
	}
}

func (b *ConfigBuilder) WithMemory(data map[string]any) *ConfigBuilder {
	for k, v := range data {
		b.memoryData[k] = v
	}
	return b
}

func (b *ConfigBuilder) WithValue(key string, val any) *ConfigBuilder {
	b.memoryData[key] = val
	return b
}

func (b *ConfigBuilder) Build() *core.Engine {
	mem := loader.NewMemoryLoader()
	if len(b.memoryData) > 0 {
		mem.Update(b.memoryData)
	}
	layer := core.NewLayer("test-memory",
		core.WithLayerSource(mem),
		core.WithLayerPriority(100),
	)
	return core.New(core.WithLayer(layer))
}

func (b *ConfigBuilder) BuildWithLayers() *core.Engine {
	if len(b.layers) > 0 {
		return core.New(core.WithLayers(b.layers...))
	}
	return b.Build()
}

// GenerateNKeys creates a map with n keys of the form "key.0001" -> "value.0001".
func GenerateNKeys(n int) map[string]any {
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("key.%04d", i)] = fmt.Sprintf("value.%04d", i)
	}
	return m
}

// GenerateNValues creates n Value entries.
func GenerateNValues(n int) map[string]value.Value {
	m := make(map[string]value.Value, n)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key.%04d", i)
		val := fmt.Sprintf("value.%04d", i)
		m[key] = value.NewInMemory(val)
	}
	return m
}

// GoldenFile helpers for golden file tests.
type GoldenComparer struct {
	t         *testing.T
	update    bool
	goldenDir string
}

func NewGoldenComparer(t *testing.T, goldenDir string, update bool) *GoldenComparer {
	return &GoldenComparer{t: t, update: update, goldenDir: goldenDir}
}

// ChaosInjector simulates failures for resilience testing.
type ChaosInjector struct {
	failCount   atomic.Int64
	maxFailures int64
	failRate    float64
	callCount   atomic.Int64
}

func NewChaosInjector(maxFailures int64, failRate float64) *ChaosInjector {
	return &ChaosInjector{
		maxFailures: maxFailures,
		failRate:    failRate,
	}
}

func (c *ChaosInjector) ShouldFail() bool {
	count := c.callCount.Add(1)
	if c.maxFailures > 0 && count <= c.maxFailures {
		c.failCount.Add(1)
		return true
	}
	if c.failRate > 0 && c.failRate < 1.0 {
		return float64(count%100) < c.failRate*100
	}
	return false
}

func (c *ChaosInjector) FailCount() int64 {
	return c.failCount.Load()
}

func (c *ChaosInjector) CallCount() int64 {
	return c.callCount.Load()
}

func (c *ChaosInjector) Reset() {
	c.failCount.Store(0)
	c.callCount.Store(0)
}

// RetryTest runs fn up to maxAttempts times, failing t if all attempts fail.
func RetryTest(t *testing.T, maxAttempts int, sleep time.Duration, fn func(t *testing.T) bool) {
	t.Helper()
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		t.Run(fmt.Sprintf("attempt_%d", i+1), func(t *testing.T) {
			if fn(t) {
				t.SkipNow() // success
			}
		})
		if t.Skipped() {
			return
		}
		lastErr = fmt.Errorf("attempt %d failed", i+1)
		if i < maxAttempts-1 {
			time.Sleep(sleep)
		}
	}
	if lastErr != nil && !t.Skipped() {
		t.Fatal(lastErr)
	}
}

// AssertNoError is a helper to check err is nil.
func AssertNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Helper()
		msg := "expected no error"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprint(msgAndArgs...)
		}
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError is a helper to check err is not nil.
func AssertError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		msg := "expected error"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprint(msgAndArgs...)
		}
		t.Fatal(msg)
	}
}

// AssertEqual compares two values.
func AssertEqual[T comparable](t *testing.T, got, want T, msgAndArgs ...any) {
	t.Helper()
	if got != want {
		msg := fmt.Sprintf("got %v, want %v", got, want)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprint(msgAndArgs...) + ": " + msg
		}
		t.Fatal(msg)
	}
}
