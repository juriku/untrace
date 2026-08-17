package gitignore

import "testing"

var matchSink bool

// Match runs once per walked entry, so it must stay allocation-free even
// against a large pattern set.
func TestMatchAllocatesNothing(t *testing.T) {
	m := NewMatcher(patterns(500))
	path := []string{"src", "internal", "main.go"}

	got := testing.AllocsPerRun(5, func() { matchSink = m.Match(path, false) })
	if got > 0 {
		t.Errorf("%v allocations, want 0", got)
	}
}
