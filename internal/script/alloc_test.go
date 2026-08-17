package script

import "testing"

var wordSink []Word

// Single-script text is the overwhelming majority of what gets scanned, and the
// census must reject it without allocating.
func TestSingleScriptTextAllocatesNothing(t *testing.T) {
	cases := []struct {
		name  string
		runes []rune
	}{
		{"pure latin", corpusPureLatin},
		{"pure cyrillic", corpusPureCyrillic},
		{"many short words", corpusManyShortWords},
		{"one long word", corpusOneLongWord},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := testing.AllocsPerRun(3, func() { wordSink = MixedWords(tc.runes) })
			if got > 0 {
				t.Errorf("%v allocations, want 0", got)
			}
		})
	}
}
