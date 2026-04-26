package pattern

import "testing"

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		pat  string
		want bool
	}{
		{name: "catch all empty", key: "counter", pat: "", want: true},
		{name: "catch all star", key: "counter", pat: "*", want: true},
		{name: "exact key", key: "counter", pat: "counter", want: true},
		{name: "suffix star is not prefix glob", key: "counter", pat: "counter*", want: false},
		{name: "create star is not event type prefix", key: "create", pat: "create*", want: false},
		{name: "segment wildcard matches one segment", key: "app.db.config", pat: "app.*.config", want: true},
		{name: "segment wildcard must preserve segment count", key: "app.config", pat: "app.*.config", want: false},
		{name: "suffix segment wildcard", key: "config.changed", pat: "*.changed", want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Match(tt.key, tt.pat); got != tt.want {
				t.Fatalf("Match(%q, %q) = %v, want %v", tt.key, tt.pat, got, tt.want)
			}
		})
	}
}
