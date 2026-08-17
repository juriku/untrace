package resolve

import "unicode"

type scriptGroup int

const (
	groupNone scriptGroup = iota
	groupCJK
	groupArabic
	groupGreek
)

// A script must reach this share of the document's letters before its
// punctuation counts as native. One stray ideograph pasted into an English file
// does not make every fullwidth comma in it legitimate.
const nativeShare = 0.1

// NeedsCensus reports whether judging this rune consults the document census,
// so a caller can defer the pass until one is actually met.
func NeedsCensus(r rune) bool { return scriptPunctuation[r] != groupNone }

// Census counts letters per script group once per document, so a per-occurrence
// question costs a lookup rather than a scan.
type Census struct {
	cjk    int
	arabic int
	greek  int
	total  int
}

func NewCensus(runes []rune) Census {
	var c Census
	for _, r := range runes {
		if r < 0x80 {
			if unicode.IsLetter(r) {
				c.total++
			}
			continue
		}
		if !unicode.IsLetter(r) {
			continue
		}
		c.total++
		switch groupOf(r) {
		case groupCJK:
			c.cjk++
		case groupArabic:
			c.arabic++
		case groupGreek:
			c.greek++
		}
	}
	return c
}

func groupOf(r rune) scriptGroup {
	switch {
	case isAnyOf(r, cjkScripts):
		return groupCJK
	case isAnyOf(r, arabicScripts):
		return groupArabic
	case isAnyOf(r, greekScripts):
		return groupGreek
	}
	return groupNone
}

func (c Census) native(g scriptGroup) bool {
	var n int
	switch g {
	case groupCJK:
		n = c.cjk
	case groupArabic:
		n = c.arabic
	case groupGreek:
		n = c.greek
	default:
		return false
	}
	if n == 0 {
		return false
	}
	return float64(n) >= float64(c.total)*nativeShare
}
