package value

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// NewState
// ---------------------------------------------------------------------------

func TestNewState(t *testing.T) {
	t.Run("with_data", func(t *testing.T) {
		data := map[string]Value{
			"a": New(1),
			"b": New("hello"),
		}
		s := NewState(data)

		assert.Equal(t, int64(0), s.Version())
		assert.NotEmpty(t, s.Checksum())
		assert.Equal(t, 2, s.Len())

		// Data should be a copy — modifying original doesn't affect state
		data["c"] = New(3)
		assert.Equal(t, 2, s.Len())
	})

	t.Run("nil_data", func(t *testing.T) {
		s := NewState(nil)
		assert.NotNil(t, s)
		assert.Equal(t, 0, s.Len())
		assert.Equal(t, int64(0), s.Version())
	})
}

// ---------------------------------------------------------------------------
// NewStateCopy
// ---------------------------------------------------------------------------

func TestNewStateCopy(t *testing.T) {
	data := map[string]Value{"a": New(1)}
	s1 := NewState(data)
	s2 := NewStateCopy(s1)

	assert.Equal(t, int64(1), s2.Version(), "version should be incremented")
	// Data should be the same even though version differs (Equal checks version too)
	assert.Equal(t, s1.GetAll(), s2.GetAll(), "data should be equal")

	// Modifying copy should not affect original
	s3 := s2.Set("b", New(2))
	assert.Equal(t, 1, s1.Len())
	assert.Equal(t, 2, s3.Len())
}

func TestNewStateCopy_Nil(t *testing.T) {
	s := NewStateCopy(nil)
	assert.NotNil(t, s)
	assert.Equal(t, 0, s.Len())
}

// ---------------------------------------------------------------------------
// NewStateWithVersion
// ---------------------------------------------------------------------------

func TestNewStateWithVersion(t *testing.T) {
	data := map[string]Value{"a": New(1)}
	s := NewStateWithVersion(data, 42)
	assert.Equal(t, int64(42), s.Version())
	assert.Equal(t, 1, s.Len())
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestState_Get(t *testing.T) {
	data := map[string]Value{
		"a": New(1),
		"b": New("hello"),
	}
	s := NewState(data)

	t.Run("existing_key", func(t *testing.T) {
		v := s.Get("a")
		assert.False(t, v.IsZero())
		assert.Equal(t, 1, v.Int())
	})

	t.Run("missing_key", func(t *testing.T) {
		v := s.Get("missing")
		assert.True(t, v.IsZero())
	})

	t.Run("nil_state", func(t *testing.T) {
		var nilState *State
		v := nilState.Get("anything")
		assert.True(t, v.IsZero())
	})
}

// ---------------------------------------------------------------------------
// GetAll / Has / Len / Keys / Checksum
// ---------------------------------------------------------------------------

func TestState_GetAll(t *testing.T) {
	data := map[string]Value{"a": New(1), "b": New(2)}
	s := NewState(data)

	all := s.GetAll()
	assert.Equal(t, 2, len(all))
	assert.Equal(t, 1, all["a"].Int())
	assert.Equal(t, 2, all["b"].Int())

	// Modifying returned copy doesn't affect state
	all["c"] = New(3)
	assert.Equal(t, 2, s.Len())
}

func TestState_Has(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1)})

	assert.True(t, s.Has("a"))
	assert.False(t, s.Has("b"))
}

func TestState_Has_Nil(t *testing.T) {
	var nilState *State
	assert.False(t, nilState.Has("a"))
}

func TestState_Len(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1), "b": New(2), "c": New(3)})
	assert.Equal(t, 3, s.Len())

	s2 := NewState(nil)
	assert.Equal(t, 0, s2.Len())
}

func TestState_Keys(t *testing.T) {
	s := NewState(map[string]Value{"c": New(3), "a": New(1), "b": New(2)})
	keys := s.Keys()
	assert.Equal(t, []string{"a", "b", "c"}, keys)
}

func TestState_Checksum(t *testing.T) {
	s1 := NewState(map[string]Value{"a": New(1)})
	s2 := NewState(map[string]Value{"a": New(1)})
	assert.Equal(t, s1.Checksum(), s2.Checksum())
}

func TestState_Version(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1)})
	assert.Equal(t, int64(0), s.Version())

	s2 := s.Set("b", New(2))
	assert.Equal(t, int64(1), s2.Version())
}

// ---------------------------------------------------------------------------
// Set (returns new State)
// ---------------------------------------------------------------------------

func TestState_Set(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1)})

	s2 := s.Set("b", New(2))
	assert.Equal(t, 1, s.Len(), "original unchanged")
	assert.Equal(t, 2, s2.Len(), "new state has new key")
	assert.Equal(t, int64(1), s2.Version())

	// Overwrite existing key
	s3 := s.Set("a", New(10))
	assert.Equal(t, 1, s3.Len())
	assert.Equal(t, 10, s3.Get("a").Int())
}

func TestState_Set_Nil(t *testing.T) {
	var nilState *State
	s := nilState.Set("a", New(1))
	assert.NotNil(t, s)
	assert.Equal(t, 1, s.Len())
}

// ---------------------------------------------------------------------------
// Delete (returns new State)
// ---------------------------------------------------------------------------

func TestState_Delete(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1), "b": New(2)})

	s2 := s.Delete("a")
	assert.Equal(t, 2, s.Len(), "original unchanged")
	assert.Equal(t, 1, s2.Len())
	assert.False(t, s2.Has("a"))
	assert.Equal(t, int64(1), s2.Version())
}

func TestState_Delete_Nonexistent(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1)})
	s2 := s.Delete("missing")
	assert.Equal(t, 1, s2.Len())
}

func TestState_Delete_Nil(t *testing.T) {
	var nilState *State
	s := nilState.Delete("a")
	assert.Nil(t, s)
}

// ---------------------------------------------------------------------------
// Merge (returns new State)
// ---------------------------------------------------------------------------

func TestState_Merge(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1), "b": New(2)})
	overlay := map[string]Value{"b": New(20), "c": New(3)}

	s2 := s.Merge(overlay)
	assert.Equal(t, 1, s.Get("a").Int(), "original unchanged")
	assert.Equal(t, 20, s2.Get("b").Int(), "overlay overrides")
	assert.Equal(t, 3, s2.Get("c").Int(), "new key added")
	assert.Equal(t, int64(1), s2.Version())
}

func TestState_Merge_Nil(t *testing.T) {
	var nilState *State
	overlay := map[string]Value{"a": New(1)}
	s := nilState.Merge(overlay)
	assert.Equal(t, 1, s.Len())
}

func TestState_Merge_EmptyOverlay(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1)})
	s2 := s.Merge(nil)
	assert.Equal(t, 1, s2.Len())
}

// ---------------------------------------------------------------------------
// DiffEvents
// ---------------------------------------------------------------------------

func TestState_DiffEvents(t *testing.T) {
	s1 := NewState(map[string]Value{"a": New(1), "b": New(2)})
	s2 := NewState(map[string]Value{"a": New(10), "c": New(3)})

	events := s1.DiffEvents(s2)
	require.Len(t, events, 3)

	created := filterDiffEvents(events, DiffCreated)
	updated := filterDiffEvents(events, DiffUpdated)
	deleted := filterDiffEvents(events, DiffDeleted)

	assert.Len(t, created, 1)
	assert.Equal(t, "c", created[0].Key)

	assert.Len(t, updated, 1)
	assert.Equal(t, "a", updated[0].Key)

	assert.Len(t, deleted, 1)
	assert.Equal(t, "b", deleted[0].Key)
}

func TestState_DiffEvents_NilStates(t *testing.T) {
	t.Run("both_nil", func(t *testing.T) {
		var s1, s2 *State
		events := s1.DiffEvents(s2)
		assert.Nil(t, events)
	})

	t.Run("nil_old", func(t *testing.T) {
		var s1 *State
		s2 := NewState(map[string]Value{"a": New(1)})
		events := s1.DiffEvents(s2)
		require.Len(t, events, 1)
		assert.Equal(t, DiffCreated, events[0].Type)
	})
}

// ---------------------------------------------------------------------------
// RedactedCopy
// ---------------------------------------------------------------------------

func TestState_RedactedCopy(t *testing.T) {
	data := map[string]Value{
		"host":     New("localhost"),
		"password": New("secret123"),
		"port":     New(5432),
	}
	s := NewState(data)
	redacted := s.RedactedCopy()

	assert.Equal(t, "localhost", redacted.Get("host").String())
	assert.Equal(t, "***REDACTED***", redacted.Get("password").String())
	assert.Equal(t, 5432, redacted.Get("port").Int())
	assert.Equal(t, s.Version(), redacted.Version())
	// Checksum should differ since secret values are changed
	assert.NotEqual(t, s.Checksum(), redacted.Checksum())
}

func TestState_RedactedCopy_Nil(t *testing.T) {
	var nilState *State
	assert.Nil(t, nilState.RedactedCopy())
}

// ---------------------------------------------------------------------------
// Equal
// ---------------------------------------------------------------------------

func TestState_Equal(t *testing.T) {
	t.Run("same_data", func(t *testing.T) {
		s1 := NewState(map[string]Value{"a": New(1)})
		s2 := NewState(map[string]Value{"a": New(1)})
		assert.True(t, s1.Equal(s2))
	})

	t.Run("different_data", func(t *testing.T) {
		s1 := NewState(map[string]Value{"a": New(1)})
		s2 := NewState(map[string]Value{"a": New(2)})
		assert.False(t, s1.Equal(s2))
	})

	t.Run("different_version", func(t *testing.T) {
		s1 := NewStateWithVersion(map[string]Value{"a": New(1)}, 1)
		s2 := NewStateWithVersion(map[string]Value{"a": New(1)}, 2)
		assert.False(t, s1.Equal(s2))
	})

	t.Run("both_nil", func(t *testing.T) {
		var s1, s2 *State
		assert.True(t, s1.Equal(s2))
	})

	t.Run("one_nil", func(t *testing.T) {
		var s1 *State
		s2 := NewState(nil)
		assert.False(t, s1.Equal(s2))
	})
}

// ---------------------------------------------------------------------------
// Data / DataUnsafe
// ---------------------------------------------------------------------------

func TestState_Data(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1)})
	d := s.Data()
	assert.Equal(t, 1, d["a"].Int())
}

func TestState_DataUnsafe(t *testing.T) {
	s := NewState(map[string]Value{"a": New(1)})
	d := s.DataUnsafe()
	assert.Equal(t, 1, d["a"].Int())
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func TestState_ConcurrentReads(t *testing.T) {
	s := NewState(map[string]Value{
		"a": New(1),
		"b": New("hello"),
		"c": New(true),
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Get("a")
			_ = s.GetAll()
			_ = s.Has("b")
			_ = s.Len()
			_ = s.Keys()
			_ = s.Checksum()
			_ = s.Version()
		}()
	}
	wg.Wait()
}

func TestState_ConcurrentWrites(t *testing.T) {
	s := NewState(nil)

	var wg sync.WaitGroup
	results := make(chan *State, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- s.Set("key", New(i))
		}(i)
	}
	wg.Wait()
	close(results)

	assert.Equal(t, 0, s.Len(), "base state should remain unchanged")
	for next := range results {
		require.NotNil(t, next)
		assert.Equal(t, 1, next.Len())
		assert.True(t, next.Has("key"))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func filterDiffEvents(events []DiffEvent, dt DiffType) []DiffEvent {
	var out []DiffEvent
	for _, e := range events {
		if e.Type == dt {
			out = append(out, e)
		}
	}
	return out
}
