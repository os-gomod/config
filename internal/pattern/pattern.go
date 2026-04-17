// Package pattern provides glob-style pattern matching for configuration keys.
// It supports the * (matches any sequence) and ? (matches single character) wildcards.
package pattern

// Match reports whether the key matches the given pattern using glob-style matching.
// A pattern of "*" or "" matches all keys.
//
// Examples:
//
//	pattern.Match("db.host", "db.*")     // true
//	pattern.Match("db.host", "*.host")   // true
//	pattern.Match("db.host", "db.h?st")  // true
//	pattern.Match("db.host", "server.*") // false
func Match(key, pat string) bool {
	if pat == "*" || pat == "" {
		return true
	}
	return matchGlob(key, pat)
}

// matchGlob implements glob matching using dynamic programming.
// It supports '*' (matches zero or more characters) and '?' (matches exactly one).
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
