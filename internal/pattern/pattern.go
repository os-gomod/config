// Package pattern provides glob-style pattern matching for config key subscriptions.
package pattern

// Match reports whether key matches the given glob pattern.
// The pattern supports '*' (match any sequence) and '?' (match single character).
// An empty pattern or "*" matches every key.
func Match(key, pat string) bool {
	if pat == "*" || pat == "" {
		return true
	}
	return matchGlob(key, pat)
}

// matchGlob implements a DP-based glob matcher supporting '*' and '?' wildcards.
func matchGlob(s, pattern string) bool {
	m, n := len(s), len(pattern)
	prev := make([]bool, n+1)
	curr := make([]bool, n+1)
	prev[0] = true
	for j := 1; j <= n; j++ {
		if pattern[j-1] == '*' {
			prev[j] = prev[j-1]
		}
	}
	for i := 1; i <= m; i++ {
		curr[0] = false
		for j := 1; j <= n; j++ {
			switch pattern[j-1] {
			case '*':
				curr[j] = prev[j] || curr[j-1]
			case '?', s[i-1]:
				curr[j] = prev[j-1]
			default:
				curr[j] = false
			}
		}
		prev, curr = curr, prev
	}
	return prev[n]
}
