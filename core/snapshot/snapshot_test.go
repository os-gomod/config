package snapshot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/os-gomod/config/core/value"
)

func TestNewSnapshot(t *testing.T) {
	t.Run("basic snapshot", func(t *testing.T) {
		data := map[string]value.Value{
			"key1": value.NewInMemory("v1"),
			"key2": value.NewInMemory(42),
		}
		s := New(1, 10, data)
		if s == nil {
			t.Fatal("expected non-nil snapshot")
		}
		if s.ID() != 1 {
			t.Fatalf("expected id 1, got %d", s.ID())
		}
		if s.Version() != 10 {
			t.Fatalf("expected version 10, got %d", s.Version())
		}
		if s.Len() != 2 {
			t.Fatalf("expected 2 entries, got %d", s.Len())
		}
		if s.Checksum() == "" {
			t.Fatal("expected non-empty checksum")
		}
		if s.Timestamp().IsZero() {
			t.Fatal("expected non-zero timestamp")
		}
	})

	t.Run("with label option", func(t *testing.T) {
		s := New(1, 1, nil, WithLabel("release-1.0"))
		if s.Label() != "release-1.0" {
			t.Fatalf("expected label 'release-1.0', got %q", s.Label())
		}
	})

	t.Run("with parent option", func(t *testing.T) {
		s := New(2, 2, nil, WithParent(1))
		if s.Parent() != 1 {
			t.Fatalf("expected parent 1, got %d", s.Parent())
		}
	})

	t.Run("with timestamp option", func(t *testing.T) {
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		s := New(1, 1, nil, WithTimestamp(ts))
		if !s.Timestamp().Equal(ts) {
			t.Fatalf("expected timestamp %v, got %v", ts, s.Timestamp())
		}
	})

	t.Run("with metadata option", func(t *testing.T) {
		s := New(1, 1, nil, WithMetadata("author", "test"))
		m := s.Metadata()
		if m["author"] != "test" {
			t.Fatalf("expected author 'test', got %v", m["author"])
		}
	})

	t.Run("copies input data", func(t *testing.T) {
		data := map[string]value.Value{
			"k": value.NewInMemory("v"),
		}
		s := New(1, 1, data)
		data["injected"] = value.NewInMemory("x")
		if _, ok := s.Get("injected"); ok {
			t.Fatal("snapshot should copy input data")
		}
	})
}

func TestSnapshot_Get(t *testing.T) {
	data := map[string]value.Value{
		"key": value.NewInMemory("value"),
	}
	s := New(1, 1, data)

	t.Run("existing key", func(t *testing.T) {
		v, ok := s.Get("key")
		if !ok {
			t.Fatal("expected key to exist")
		}
		if v.Raw() != "value" {
			t.Fatalf("expected 'value', got %v", v.Raw())
		}
	})

	t.Run("missing key", func(t *testing.T) {
		_, ok := s.Get("missing")
		if ok {
			t.Fatal("expected key to not exist")
		}
	})
}

func TestSnapshot_Keys(t *testing.T) {
	data := map[string]value.Value{
		"zebra":  value.NewInMemory("z"),
		"apple":  value.NewInMemory("a"),
		"banana": value.NewInMemory("b"),
	}
	s := New(1, 1, data)
	keys := s.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "apple" || keys[1] != "banana" || keys[2] != "zebra" {
		t.Fatalf("expected sorted keys, got %v", keys)
	}
}

func TestSnapshot_Data(t *testing.T) {
	data := map[string]value.Value{
		"k": value.NewInMemory("v"),
	}
	s := New(1, 1, data)
	returned := s.Data()
	returned["injected"] = value.NewInMemory("x")
	if _, ok := s.Get("injected"); ok {
		t.Fatal("Data() should return a copy")
	}
}

func TestSnapshot_Metadata(t *testing.T) {
	s := New(1, 1, nil)
	m := s.Metadata()
	if m == nil {
		t.Fatal("expected non-nil metadata")
	}
	// Mutating returned map should not affect snapshot
	m["injected"] = "x"
	if s.Metadata()["injected"] == "x" {
		t.Fatal("Metadata() should return a copy")
	}
}

func TestSnapshot_MarshalJSON(t *testing.T) {
	data := map[string]value.Value{
		"key": value.NewInMemory("value"),
	}
	s := New(1, 5, data, WithLabel("test"))
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(b), `"id":1`) {
		t.Fatalf("expected id in JSON, got: %s", string(b))
	}
	if !strings.Contains(string(b), `"version":5`) {
		t.Fatalf("expected version in JSON, got: %s", string(b))
	}
	if !strings.Contains(string(b), `"label":"test"`) {
		t.Fatalf("expected label in JSON, got: %s", string(b))
	}
}

func TestDiff(t *testing.T) {
	t.Run("compare identical snapshots", func(t *testing.T) {
		data := map[string]value.Value{"k": value.NewInMemory("v")}
		s1 := New(1, 1, data)
		s2 := New(2, 2, data)
		result := Compare(s1, s2)
		if result.HasChanges() {
			t.Fatal("expected no changes")
		}
		if result.Summary() != "no changes" {
			t.Fatalf("expected 'no changes', got %q", result.Summary())
		}
	})

	t.Run("compare added keys", func(t *testing.T) {
		s1 := New(1, 1, map[string]value.Value{"a": value.NewInMemory(1)})
		s2 := New(2, 2, map[string]value.Value{
			"a": value.NewInMemory(1),
			"b": value.NewInMemory(2),
		})
		result := Compare(s1, s2)
		if !result.HasChanges() {
			t.Fatal("expected changes")
		}
		if result.Added != 1 {
			t.Fatalf("expected 1 added, got %d", result.Added)
		}
	})

	t.Run("compare deleted keys", func(t *testing.T) {
		s1 := New(1, 1, map[string]value.Value{"a": value.NewInMemory(1), "b": value.NewInMemory(2)})
		s2 := New(2, 2, map[string]value.Value{"a": value.NewInMemory(1)})
		result := Compare(s1, s2)
		if result.Deleted != 1 {
			t.Fatalf("expected 1 deleted, got %d", result.Deleted)
		}
	})

	t.Run("compare modified keys", func(t *testing.T) {
		s1 := New(1, 1, map[string]value.Value{"k": value.NewInMemory("v1")})
		s2 := New(2, 2, map[string]value.Value{"k": value.NewInMemory("v2")})
		result := Compare(s1, s2)
		if result.Modified != 1 {
			t.Fatalf("expected 1 modified, got %d", result.Modified)
		}
	})

	t.Run("compare with nil", func(t *testing.T) {
		s2 := New(2, 2, map[string]value.Value{"k": value.NewInMemory("v")})
		result := Compare(nil, s2)
		if result.Added != 1 {
			t.Fatalf("expected 1 added from nil, got %d", result.Added)
		}
	})
}

func TestDiffResult_Summary(t *testing.T) {
	tests := []struct {
		name string
		r    *DiffResult
		want string
	}{
		{"no changes", &DiffResult{}, "no changes"},
		{"1 addition", &DiffResult{Added: 1}, "1 addition"},
		{"2 additions", &DiffResult{Added: 2}, "2 additions"},
		{"1 modification", &DiffResult{Modified: 1}, "1 modification"},
		{"1 deletion", &DiffResult{Deleted: 1}, "1 deletion"},
		{"mixed", &DiffResult{Added: 1, Modified: 1, Deleted: 1}, "1 addition, 1 modification, 1 deletion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChangeType_String(t *testing.T) {
	tests := []struct {
		ct   ChangeType
		want string
	}{
		{ChangeNone, "none"},
		{ChangeAdded, "added"},
		{ChangeModified, "modified"},
		{ChangeDeleted, "deleted"},
		{ChangeType(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.want {
				t.Errorf("ChangeType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	t.Run("new version", func(t *testing.T) {
		v := NewVersion(1, 2, 3)
		if v.Major() != 1 || v.Minor() != 2 || v.Patch() != 3 {
			t.Fatal("expected 1.2.3")
		}
	})

	t.Run("string", func(t *testing.T) {
		v := NewVersion(1, 2, 3)
		if v.String() != "1.2.3" {
			t.Fatalf("expected '1.2.3', got %q", v.String())
		}
	})

	t.Run("with label", func(t *testing.T) {
		v := NewVersion(1, 0, 0).WithLabel("alpha")
		if v.String() != "1.0.0-alpha" {
			t.Fatalf("expected '1.0.0-alpha', got %q", v.String())
		}
		if v.Label() != "alpha" {
			t.Fatalf("expected label 'alpha', got %q", v.Label())
		}
	})

	t.Run("compare", func(t *testing.T) {
		tests := []struct {
			a, b Version
			want int
		}{
			{NewVersion(1, 0, 0), NewVersion(1, 0, 0), 0},
			{NewVersion(1, 0, 0), NewVersion(2, 0, 0), -1},
			{NewVersion(2, 0, 0), NewVersion(1, 0, 0), 1},
			{NewVersion(1, 1, 0), NewVersion(1, 2, 0), -1},
			{NewVersion(1, 2, 0), NewVersion(1, 1, 0), 1},
			{NewVersion(1, 1, 5), NewVersion(1, 1, 10), -1},
			{NewVersion(1, 1, 10), NewVersion(1, 1, 5), 1},
		}
		for _, tt := range tests {
			got := tt.a.Compare(tt.b)
			if got != tt.want {
				t.Errorf("%v.Compare(%v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		}
	})
}

func TestManager(t *testing.T) {
	t.Run("new manager with default max", func(t *testing.T) {
		m := NewManager(0)
		if m == nil {
			t.Fatal("expected non-nil manager")
		}
		if m.Latest() != nil {
			t.Fatal("latest should be nil for empty manager")
		}
	})

	t.Run("take snapshot", func(t *testing.T) {
		m := NewManager(10)
		data := map[string]value.Value{"k": value.NewInMemory("v")}
		s := m.Take(1, data)
		if s == nil {
			t.Fatal("expected non-nil snapshot")
		}
		if s.ID() != 1 {
			t.Fatalf("expected id 1, got %d", s.ID())
		}
	})

	t.Run("latest returns most recent", func(t *testing.T) {
		m := NewManager(10)
		m.Take(1, map[string]value.Value{"a": value.NewInMemory(1)})
		m.Take(2, map[string]value.Value{"b": value.NewInMemory(2)})
		latest := m.Latest()
		if latest == nil {
			t.Fatal("expected non-nil")
		}
		if latest.Version() != 2 {
			t.Fatalf("expected version 2, got %d", latest.Version())
		}
	})

	t.Run("get by id", func(t *testing.T) {
		m := NewManager(10)
		m.Take(1, nil)
		m.Take(2, nil)
		s := m.Get(1)
		if s == nil {
			t.Fatal("expected to find snapshot 1")
		}
		// check nil return for missing
		missing := m.Get(99)
		if missing != nil {
			t.Fatal("expected nil for missing id")
		}
	})

	t.Run("get missing returns nil", func(t *testing.T) {
		m := NewManager(10)
		if m.Get(99) != nil {
			t.Fatal("expected nil for missing id")
		}
	})

	t.Run("list returns all snapshots", func(t *testing.T) {
		m := NewManager(10)
		m.Take(1, nil)
		m.Take(2, nil)
		m.Take(3, nil)
		list := m.List()
		if len(list) != 3 {
			t.Fatalf("expected 3 snapshots, got %d", len(list))
		}
	})

	t.Run("max snapshots limit", func(t *testing.T) {
		m := NewManager(3)
		m.Take(1, nil)
		m.Take(2, nil)
		m.Take(3, nil)
		m.Take(4, nil)
		list := m.List()
		if len(list) != 3 {
			t.Fatalf("expected 3 snapshots (max), got %d", len(list))
		}
	})

	t.Run("create branch and retrieve", func(t *testing.T) {
		m := NewManager(10)
		b := m.CreateBranch("feature", 1)
		if b.Name() != "feature" {
			t.Fatalf("expected branch name 'feature', got %q", b.Name())
		}
		if b.BaseID() != 1 {
			t.Fatalf("expected base id 1, got %d", b.BaseID())
		}
		retrieved, ok := m.Branch("feature")
		if !ok {
			t.Fatal("expected branch to exist")
		}
		if retrieved.Name() != "feature" {
			t.Fatal("branch name mismatch")
		}
	})

	t.Run("branch not found", func(t *testing.T) {
		m := NewManager(10)
		_, ok := m.Branch("missing")
		if ok {
			t.Fatal("expected branch to not exist")
		}
	})

	t.Run("incrementing IDs", func(t *testing.T) {
		m := NewManager(10)
		s1 := m.Take(1, nil)
		s2 := m.Take(2, nil)
		if s2.ID() <= s1.ID() {
			t.Fatalf("expected incrementing IDs: %d <= %d", s2.ID(), s1.ID())
		}
	})
}

func TestVersionGraph(t *testing.T) {
	t.Run("add and retrieve node", func(t *testing.T) {
		g := NewVersionGraph()
		node := VersionNode{
			ID:        "1",
			Checksum:  "abc",
			Timestamp: time.Now(),
		}
		g.Add(&node)
		// Graph is internal, tested indirectly through Manager
	})
}

func TestBranch(t *testing.T) {
	t.Run("new branch", func(t *testing.T) {
		b := &Branch{name: "test", baseID: 1, created: time.Now()}
		if b.Name() != "test" {
			t.Fatalf("expected name 'test', got %q", b.Name())
		}
		if b.BaseID() != 1 {
			t.Fatalf("expected base id 1, got %d", b.BaseID())
		}
		if b.Len() != 0 {
			t.Fatalf("expected 0 snapshots, got %d", b.Len())
		}
		if b.Current() != nil {
			t.Fatal("expected nil current")
		}
	})

	t.Run("append and current", func(t *testing.T) {
		b := &Branch{name: "test", baseID: 1, created: time.Now()}
		s := New(1, 1, map[string]value.Value{"k": value.NewInMemory("v")})
		b.Append(s)
		if b.Len() != 1 {
			t.Fatalf("expected 1, got %d", b.Len())
		}
		if b.Current().ID() != 1 {
			t.Fatalf("expected id 1, got %d", b.Current().ID())
		}
	})

	t.Run("history returns copy", func(t *testing.T) {
		b := &Branch{name: "test", baseID: 1, created: time.Now()}
		s := New(1, 1, nil)
		b.Append(s)
		h := b.History()
		h[0] = nil
		if b.Current() == nil {
			t.Fatal("History should return a copy")
		}
	})
}
