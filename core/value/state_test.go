package value

import "testing"

func TestNewState(t *testing.T) {
	data := map[string]Value{"a": NewInMemory("1")}
	s := NewState(data, 1)
	if s.Version() != 1 {
		t.Errorf("expected version 1, got %d", s.Version())
	}
	if s.Len() != 1 {
		t.Errorf("expected len 1, got %d", s.Len())
	}
}

func TestState_Get(t *testing.T) {
	s := NewState(map[string]Value{"key": NewInMemory("val")}, 1)
	v, ok := s.Get("key")
	if !ok || v.String() != "val" {
		t.Error("expected key=val")
	}
	_, ok = s.Get("missing")
	if ok {
		t.Error("expected Get to fail for missing key")
	}
}

func TestState_GetAll(t *testing.T) {
	data := map[string]Value{"a": NewInMemory("1"), "b": NewInMemory("2")}
	s := NewState(data, 1)
	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestState_Has(t *testing.T) {
	s := NewState(map[string]Value{"key": NewInMemory("val")}, 1)
	if !s.Has("key") {
		t.Error("expected Has(key) = true")
	}
	if s.Has("missing") {
		t.Error("expected Has(missing) = false")
	}
}

func TestState_Keys(t *testing.T) {
	s := NewState(map[string]Value{"z": NewInMemory("1"), "a": NewInMemory("2")}, 1)
	keys := s.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestState_RedactedCopyBasic(t *testing.T) {
	data := map[string]Value{
		"app.name":    NewInMemory("myapp"),
		"db.password": NewInMemory("secret123"),
	}
	s := NewState(data, 1)
	rc := s.RedactedCopy()
	v, ok := rc.Get("db.password")
	if !ok || v.String() != "[REDACTED]" {
		t.Errorf("expected [REDACTED] for db.password, got %s", v.String())
	}
	v2, ok2 := rc.Get("app.name")
	if !ok2 || v2.String() != "myapp" {
		t.Errorf("expected myapp for app.name, got %s", v2.String())
	}
}

func TestState_Checksum(t *testing.T) {
	s := NewState(map[string]Value{"a": NewInMemory("1")}, 1)
	if s.Checksum() == "" {
		t.Error("expected non-empty checksum")
	}
}

func TestState_NilData(t *testing.T) {
	s := NewState(nil, 0)
	if s.Len() != 0 {
		t.Errorf("expected 0 len for nil data, got %d", s.Len())
	}
	if s.Has("any") {
		t.Error("expected Has to return false for nil data")
	}
}

func TestState_GetAllUnsafe(t *testing.T) {
	s := NewState(map[string]Value{"a": NewInMemory("1")}, 1)
	unsafe := s.GetAllUnsafe()
	if len(unsafe) != 1 {
		t.Errorf("expected 1 entry, got %d", len(unsafe))
	}
}

func TestState_Data(t *testing.T) {
	s := NewState(map[string]Value{"a": NewInMemory("1")}, 1)
	data := s.Data()
	if len(data) != 1 {
		t.Errorf("expected 1 entry, got %d", len(data))
	}
}

func TestState_DataUnsafe(t *testing.T) {
	s := NewState(map[string]Value{"a": NewInMemory("1")}, 1)
	data := s.DataUnsafe()
	if len(data) != 1 {
		t.Errorf("expected 1 entry, got %d", len(data))
	}
}

func TestState_Equal(t *testing.T) {
	s1 := NewState(map[string]Value{"a": NewInMemory("1")}, 1)
	s2 := NewState(map[string]Value{"a": NewInMemory("1")}, 2)
	if !s1.Equal(s2) {
		t.Error("expected equal states (same data)")
	}
	s3 := NewState(map[string]Value{"a": NewInMemory("2")}, 1)
	if s1.Equal(s3) {
		t.Error("expected unequal states")
	}
	s4 := NewState(nil, 0)
	s5 := (*State)(nil)
	if s4.Equal(s5) {
		t.Error("expected non-equal (one nil)")
	}
}

func TestState_DiffEvents(t *testing.T) {
	s := NewState(map[string]Value{"a": NewInMemory("1")}, 1)
	newData := map[string]Value{
		"b": NewInMemory("2"),
	}
	events := s.DiffEvents(newData)
	if len(events) != 2 { // a deleted, b created
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestNewStateCopy(t *testing.T) {
	data := map[string]Value{"a": NewInMemory("1")}
	s := NewStateCopy(data, 1)
	// Modify original
	data["a"] = NewInMemory("2")
	v, _ := s.Get("a")
	if v.String() != "1" {
		t.Error("NewStateCopy did not copy data")
	}
}
