package value_test

import (
	"testing"

	"github.com/os-gomod/config/core/value"
)

func TestNewStatePopulatesChecksum(t *testing.T) {
	data := map[string]value.Value{
		"key": value.NewInMemory("val"),
	}
	s := value.NewState(data, 1)
	if s.Checksum() == "" {
		t.Fatal("NewState should populate checksum")
	}
	if s.Version() != 1 {
		t.Fatalf("Version: got %d, want 1", s.Version())
	}
}

func TestNewStateNilData(t *testing.T) {
	s := value.NewState(nil, 0)
	if s.Checksum() == "" {
		t.Fatal("NewState with nil data should still have a checksum")
	}
}

func TestStateGetHasKeysLen(t *testing.T) {
	data := map[string]value.Value{
		"a": value.NewInMemory(1),
		"b": value.NewInMemory(2),
	}
	s := value.NewState(data, 1)

	v, ok := s.Get("a")
	if !ok || v.Raw() != 1 {
		t.Fatal("Get failed for existing key")
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("Get should return false for missing key")
	}
	if !s.Has("a") {
		t.Fatal("Has should return true for existing key")
	}
	if s.Has("missing") {
		t.Fatal("Has should return false for missing key")
	}
	keys := s.Keys()
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("Keys: got %v", keys)
	}
	if s.Len() != 2 {
		t.Fatalf("Len: got %d, want 2", s.Len())
	}
}

func TestGetAllReturnsCopy(t *testing.T) {
	data := map[string]value.Value{
		"key": value.NewInMemory("val"),
	}
	s := value.NewState(data, 1)
	all := s.GetAll()
	all["key"] = value.NewInMemory("changed")
	if s.Get("key"); data["key"].Raw() != "val" {
		t.Fatal("GetAll should return a copy; mutation should not affect original")
	}
}

func TestGetAllUnsafeReturnsSameMapReference(t *testing.T) {
	data := map[string]value.Value{
		"key": value.NewInMemory("val"),
	}
	s := value.NewState(data, 1)
	unsafe := s.GetAllUnsafe()
	if unsafe["key"].Raw() != "val" {
		t.Fatal("GetAllUnsafe should return the same map reference")
	}
}

func TestEqualSameChecksum(t *testing.T) {
	data := map[string]value.Value{
		"key": value.NewInMemory("val"),
	}
	s1 := value.NewState(data, 1)
	s2 := value.NewState(data, 2)
	if !s1.Equal(s2) {
		t.Fatal("states with same data should have same checksum and be Equal")
	}
}

func TestEqualNilStates(t *testing.T) {
	var s1 *value.State
	var s2 *value.State
	if !s1.Equal(s2) {
		t.Fatal("two nil states should be equal")
	}
	s3 := value.NewState(nil, 0)
	if s1.Equal(s3) {
		t.Fatal("nil state should not equal non-nil state")
	}
}

func TestDiffEvents(t *testing.T) {
	old := map[string]value.Value{
		"a": value.NewInMemory(1),
		"b": value.NewInMemory(2),
	}
	s := value.NewState(old, 1)

	newData := map[string]value.Value{
		"a": value.NewInMemory(10), // updated
		"c": value.NewInMemory(3),  // created
		// b deleted
	}
	events := s.DiffEvents(newData)
	if len(events) != 3 {
		t.Fatalf("expected 3 diff events, got %d", len(events))
	}

	byType := make(map[value.DiffType]int)
	for _, e := range events {
		byType[e.Type]++
	}
	if byType[value.DiffUpdated] != 1 || byType[value.DiffCreated] != 1 ||
		byType[value.DiffDeleted] != 1 {
		t.Fatalf("expected 1 update, 1 create, 1 delete; got %v", byType)
	}
}
