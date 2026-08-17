package gitignore

import (
	"fmt"
	"testing"
)

// Match walks the pattern list from the end, so a path matching nothing is the
// worst case: it visits every pattern before returning false.
func patterns(n int) []Pattern {
	out := make([]Pattern, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ParsePattern(fmt.Sprintf("build%03d/", i), nil))
		out = append(out, ParsePattern(fmt.Sprintf("*.ext%03d", i), nil))
	}
	return out
}

func benchMatch(b *testing.B, m Matcher, path []string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(path, false)
	}
}

func BenchmarkMatchFewPatterns(b *testing.B) {
	benchMatch(b, NewMatcher(patterns(5)), []string{"src", "main.go"})
}

func BenchmarkMatchManyPatterns(b *testing.B) {
	benchMatch(b, NewMatcher(patterns(500)), []string{"src", "main.go"})
}

func BenchmarkMatchDeepPath(b *testing.B) {
	path := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "main.go"}
	benchMatch(b, NewMatcher(patterns(500)), path)
}

// Paired with BenchmarkMatchManyPatterns over the same list: an early hit
// returns without walking the rest.
func BenchmarkMatchEarlyHit(b *testing.B) {
	benchMatch(b, NewMatcher(patterns(500)), []string{"build499", "main.go"})
}

func BenchmarkNewMatcher(b *testing.B) {
	ps := patterns(50)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewMatcher(ps)
	}
}

func BenchmarkParsePattern(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ParsePattern("**/node_modules/*.log", []string{"src"})
	}
}
