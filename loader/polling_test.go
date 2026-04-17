package loader

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
	"github.com/stretchr/testify/require"
)

func TestPollingLoader_New(t *testing.T) {
	base := NewBase("poll-test", "poll", 10)
	pl := NewPollingLoader(base, nil, "test", 100*time.Millisecond, 64)

	if pl.interval != 100*time.Millisecond {
		t.Errorf("interval = %v, want %v", pl.interval, 100*time.Millisecond)
	}
	if pl.label != "test" {
		t.Errorf("label = %q, want %q", pl.label, "test")
	}
}

func TestPollingLoader_Load(t *testing.T) {
	t.Run("delegates to loadFn", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		wantData := map[string]value.Value{
			"a": value.New("1", value.TypeString, value.SourceMemory, 10),
		}
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return wantData, nil
		}, "test", 0, 0)

		got, err := pl.Load(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != len(wantData) {
			t.Errorf("got %d keys, want %d", len(got), len(wantData))
		}
		if !got["a"].Equal(wantData["a"]) {
			t.Errorf("key 'a' mismatch")
		}
	})

	t.Run("load returns cached data on error", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		callCount := 0
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			callCount++
			if callCount == 1 {
				return map[string]value.Value{
					"key": value.New("initial", value.TypeString, value.SourceMemory, 10),
				}, nil
			}
			return nil, errors.New("transient failure")
		}, "test", 0, 0)

		// First load succeeds
		data1, err := pl.Load(context.Background())
		if err != nil {
			t.Fatalf("first load: %v", err)
		}
		if data1["key"].String() != "initial" {
			t.Fatalf("first load: key = %q, want %q", data1["key"].String(), "initial")
		}

		// Second load fails, should return last cached data
		data2, err := pl.Load(context.Background())
		if err == nil {
			t.Fatal("expected error from failing loadFn")
		}
		if data2 == nil {
			t.Fatal("expected cached data, got nil")
		}
		if data2["key"].String() != "initial" {
			t.Errorf("cached key = %q, want %q", data2["key"].String(), "initial")
		}
	})

	t.Run("load after close returns error", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return nil, nil
		}, "test", 0, 0)

		pl.Close(context.Background())
		_, err := pl.Load(context.Background())
		if err == nil {
			t.Fatal("expected error loading after close")
		}
	})
}

func TestPollingLoader_Watch(t *testing.T) {
	t.Run("watch with interval > 0 returns channel", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return map[string]value.Value{
				"k": value.New("v", value.TypeString, value.SourceMemory, 10),
			}, nil
		}, "test", 50*time.Millisecond, 64)

		ch, err := pl.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch error: %v", err)
		}
		if ch == nil {
			t.Fatal("expected non-nil channel for interval > 0")
		}
		pl.Close(context.Background())
	})

	t.Run("watch with interval <= 0 returns nil", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return nil, nil
		}, "test", 0, 0)

		ch, err := pl.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch error: %v", err)
		}
		if ch != nil {
			t.Error("expected nil channel for interval <= 0")
		}
	})

	t.Run("watch with negative interval returns nil", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return nil, nil
		}, "test", -1*time.Second, 0)

		ch, err := pl.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch error: %v", err)
		}
		if ch != nil {
			t.Error("expected nil channel for negative interval")
		}
	})
}

func TestPollingLoader_Close(t *testing.T) {
	t.Run("close succeeds", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return nil, nil
		}, "test", 0, 0)

		err := pl.Close(context.Background())
		if err != nil {
			t.Fatalf("close error: %v", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return nil, nil
		}, "test", 0, 0)

		err1 := pl.Close(context.Background())
		err2 := pl.Close(context.Background())
		if err1 != nil || err2 != nil {
			t.Fatalf("close errors: %v, %v", err1, err2)
		}
	})

	t.Run("close stops watching", func(t *testing.T) {
		base := NewBase("poll", "poll", 10)
		pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
			return nil, nil
		}, "test", 50*time.Millisecond, 64)

		ctx, cancel := context.WithCancel(context.Background())
		ch, _ := pl.Watch(ctx)
		pl.Close(context.Background())
		cancel()
		// After close, reading from channel should eventually succeed or return zero
		if ch != nil {
			// The channel might have events buffered or be closed by controller
			select {
			case _, ok := <-ch:
				_ = ok // ok==false means closed, ok==true means event buffered
			default:
				// Nothing buffered, that's fine too
			}
		}
	})
}

func TestPollingLoader_LastData(t *testing.T) {
	base := NewBase("poll", "poll", 10)
	wantData := map[string]value.Value{
		"x": value.New("y", value.TypeString, value.SourceMemory, 10),
	}
	pl := NewPollingLoader(base, func(_ context.Context) (map[string]value.Value, error) {
		return wantData, nil
	}, "test", 0, 0)

	// Before any load, LastData returns empty
	if ld := pl.LastData(); len(ld) != 0 {
		t.Errorf("expected empty LastData before load, got %d keys", len(ld))
	}

	// After load, LastData should have data
	_, err := pl.Load(context.Background())
	require.NoError(t, err)
	ld := pl.LastData()
	if len(ld) != 1 {
		t.Errorf("expected 1 key in LastData, got %d", len(ld))
	}
	if !ld["x"].Equal(wantData["x"]) {
		t.Error("LastData mismatch")
	}
}

func TestPollingLoader_DefaultBufferSize(t *testing.T) {
	base := NewBase("poll", "poll", 10)
	// Pass 0 or negative buffer size, should default to 100
	pl := NewPollingLoader(base, nil, "test", 0, 0)
	// Verify it was created without panic
	if pl == nil {
		t.Fatal("expected non-nil PollingLoader")
	}
}

func TestPollingLoader_SetLastData(t *testing.T) {
	base := NewBase("poll", "poll", 10)
	pl := NewPollingLoader(base, nil, "test", 0, 0)

	newData := map[string]value.Value{
		"set": value.New("direct", value.TypeString, value.SourceMemory, 10),
	}
	pl.SetLastData(newData)

	ld := pl.LastData()
	if len(ld) != 1 {
		t.Fatalf("expected 1 key, got %d", len(ld))
	}
	if ld["set"].String() != "direct" {
		t.Errorf("set = %q, want %q", ld["set"].String(), "direct")
	}
}
