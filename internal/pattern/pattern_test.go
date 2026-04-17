package pattern

import "testing"

func TestMatch(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		tests := []struct {
			key, pat string
			want     bool
		}{
			{"app.name", "app.name", true},
			{"app.name", "app.port", false},
			{"db.host", "db.host", true},
			{"", "", true},          // empty key and empty pattern
			{"", "*", true},         // empty key, star pattern
			{"anything", "", true},  // any key, empty pattern
			{"anything", "*", true}, // any key, star pattern
		}
		for _, tt := range tests {
			if got := Match(tt.key, tt.pat); got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.key, tt.pat, got, tt.want)
			}
		}
	})

	t.Run("wildcard matches everything", func(t *testing.T) {
		keys := []string{"a", "a.b", "a.b.c", "db.host", ""}
		for _, key := range keys {
			if !Match(key, "*") {
				t.Errorf("Match(%q, '*') should be true", key)
			}
		}
	})

	t.Run("empty pattern matches everything", func(t *testing.T) {
		keys := []string{"a", "a.b", "db.host", "anything.at.all"}
		for _, key := range keys {
			if !Match(key, "") {
				t.Errorf("Match(%q, '') should be true", key)
			}
		}
	})
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name string
		key  string
		pat  string
		want bool
	}{
		// Wildcard tests
		{"star matches any prefix", "db.host", "db.*", true},
		{"star matches empty suffix", "db.", "db.*", true},
		{"star matches multi-segment", "db.host.port", "db.*", true},
		{"star at start", "host.db", "*.db", true},
		{"star at end", "db.", "db.*", true},
		{"star matches everything", "anything.goes.here", "*", true},
		{"double star", "a.b.c", "a.*.c", true},
		{"triple star", "a.b.c.d", "a.*.*.d", true},

		// Question mark tests
		{"question mark single char", "db.a", "db.?", true},
		{"question mark not multi char", "db.ab", "db.?", false},
		{"question mark not empty", "db.", "db.?", false},
		{"question mark matches dot", "db.", "db?", true},
		{"multiple question marks", "a.b.c", "?.?.?", true}, // each ? matches one char
		{"question mark at start", "x.y", "?.y", true},      // ? matches 'x'

		// Mixed wildcards
		{"mixed star and question", "abc123", "abc*", true},
		{"prefix with star", "app.config.name", "app.*.name", true},
		{"prefix with star no match", "app.config.port", "app.*.name", false},

		// No wildcards (exact match)
		{"exact match", "app.name", "app.name", true},
		{"exact no match", "app.name", "app.port", false},

		// Edge cases
		{"empty key empty pattern", "", "", true},
		{"empty key non-empty pattern", "", "a", false},
		{"non-empty key empty pattern", "a", "", true},
		{"star pattern empty key", "", "*", true},
		{"single char key single star", "a", "*", true},
		{"pattern longer than key", "a", "ab", false},
		{"key longer than pattern", "ab", "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.key, tt.pat); got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.key, tt.pat, got, tt.want)
			}
		})
	}
}

func TestMatch_EventBusStyle(t *testing.T) {
	// Test patterns as they'd be used in the event bus
	tests := []struct {
		key  string
		pat  string
		want bool
	}{
		// Database patterns
		{"db.host", "db.*", true},
		{"db.port", "db.*", true},
		{"db.connection.pool", "db.*", true},

		// App patterns
		{"app.config.name", "app.*", true},
		{"app.config.port", "app.*", true},
		{"app.server.host", "app.*", true},

		// Nested glob
		{"app.server.host", "app.*.host", true},
		{"app.server.port", "app.*.host", false},
		{"app.config.host", "app.*.host", true},

		// Single char patterns
		{"log.a", "log.?", true},
		{"log.b", "log.?", true},
		{"log.ab", "log.?", false},
		{"log.1", "log.?", true},
	}
	for _, tt := range tests {
		t.Run(tt.key+"/"+tt.pat, func(t *testing.T) {
			if got := Match(tt.key, tt.pat); got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.key, tt.pat, got, tt.want)
			}
		})
	}
}
