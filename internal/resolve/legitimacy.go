package resolve

import "unicode"

// Scripts whose rendering depends on ZWJ and ZWNJ: in these, a joiner is
// orthography rather than a hidden marker.
var joiningScripts = []*unicode.RangeTable{
	unicode.Arabic, unicode.Syriac, unicode.Thaana, unicode.Mongolian,
	unicode.Devanagari, unicode.Bengali, unicode.Gurmukhi, unicode.Gujarati,
	unicode.Oriya, unicode.Tamil, unicode.Telugu, unicode.Kannada,
	unicode.Malayalam, unicode.Sinhala, unicode.Myanmar, unicode.Khmer,
	unicode.Tibetan, unicode.Javanese, unicode.Balinese,
}

const (
	wavingBlackFlag = 0x1F3F4

	// Variation Selectors Supplement, registered only for ideographic
	// variation sequences.
	ivsFirst = 0xE0100
	ivsLast  = 0xE01EF
)

var (
	cjkScripts = []*unicode.RangeTable{
		unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul,
		unicode.Bopomofo,
	}
	arabicScripts = []*unicode.RangeTable{unicode.Arabic, unicode.Syriac, unicode.Thaana}
	greekScripts  = []*unicode.RangeTable{unicode.Greek}
	rtlScripts    = []*unicode.RangeTable{unicode.Arabic, unicode.Hebrew, unicode.Syriac, unicode.Thaana}
)

// U+3002 is a full stop in Japanese, not a stylised ASCII period.
var scriptPunctuation = map[rune]scriptGroup{
	0x060C: groupArabic,
	0x037E: groupGreek,
	0x3000: groupCJK,
	0x3002: groupCJK,
	0xFE50: groupCJK,
	0xFE52: groupCJK,
	0xFE55: groupCJK,
	0xFE56: groupCJK,
	0xFE63: groupCJK,
	0xFF01: groupCJK,
	0xFF07: groupCJK,
	0xFF0C: groupCJK,
	0xFF0E: groupCJK,
	0xFF0F: groupCJK,
	0xFF1A: groupCJK,
	0xFF1B: groupCJK,
	0xFF1F: groupCJK,
}

// Legitimate reports whether runes[i] is doing its designed Unicode job rather
// than hiding data, and why. A legitimate occurrence is not reported at all.
func Legitimate(runes []rune, i int, census Census) (bool, string) {
	r := runes[i]
	prev := at(runes, i-1)
	next := at(runes, i+1)

	switch {
	case r == 0xFEFF:
		if i == 0 {
			return true, "byte order mark at start of file"
		}

	case r == 0xFE0F || r == 0xFE0E:
		if takesVariationSelector(prev) {
			return true, "variation selector on a symbol"
		}

	case r >= 0xFE00 && r <= 0xFE0D, r >= ivsFirst && r <= ivsLast:
		if isIdeograph(prev) {
			return true, "variation selector on an ideograph"
		}

	case r == 0x200D:
		if isEmojiish(prev) && isEmojiish(next) {
			return true, "emoji zero-width joiner sequence"
		}
		if inJoiningScript(prev) || inJoiningScript(next) {
			return true, "joiner required by the surrounding script"
		}

	case r == 0x200C:
		if inJoiningScript(prev) || inJoiningScript(next) {
			return true, "non-joiner required by the surrounding script"
		}

	case r >= 0x180B && r <= 0x180D:
		if unicode.Is(unicode.Mongolian, prev) {
			return true, "Mongolian free variation selector"
		}

	case isTagRune(r):
		if inFlagSequence(runes, i) {
			return true, "emoji tag sequence for a subdivision flag"
		}

	case scriptPunctuation[r] != groupNone:
		g := scriptPunctuation[r]
		if census.native(g) {
			return true, "punctuation belonging to a script this document is written in"
		}
		if nearestLetterInGroup(runes, i, g) {
			return true, "punctuation belonging to the script beside it"
		}

	// Overrides and embeddings, U+202A to U+202E, are deliberately absent: they
	// are the Trojan Source vector (CVE-2021-42574). UAX #9 recommends isolates
	// for mixing scripts, so only those can be legitimate here.
	case r >= 0x2066 && r <= 0x2069:
		if isolateWrapsRTL(runes, i) {
			return true, "bidi isolate around right-to-left text"
		}
	}

	return false, ""
}

// UAX #9 X8 terminates an isolate at the paragraph separator, so a partner on
// another line is no partner.
func isolateWrapsRTL(runes []rune, i int) bool {
	step := 1
	if runes[i] == 0x2069 {
		step = -1
	}

	depth := 1
	for j := i + step; j >= 0 && j < len(runes); j += step {
		switch {
		case runes[j] == '\n':
			return false
		case runes[j] >= 0x2066 && runes[j] <= 0x2068:
			depth += step
		case runes[j] == 0x2069:
			depth -= step
		}
		if depth == 0 {
			return spanHasRTL(runes, i, j)
		}
	}
	return false
}

func spanHasRTL(runes []rune, from, to int) bool {
	if from > to {
		from, to = to, from
	}
	for j := from; j <= to; j++ {
		if isAnyOf(runes[j], rtlScripts) {
			return true
		}
	}
	return false
}

func nearestLetterInGroup(runes []rune, i int, g scriptGroup) bool {
	for _, step := range []int{-1, 1} {
		for j := i + step; j >= 0 && j < len(runes); j += step {
			if !unicode.IsLetter(runes[j]) {
				continue
			}
			if groupOf(runes[j]) == g {
				return true
			}
			break
		}
	}
	return false
}

func isAnyOf(r rune, scripts []*unicode.RangeTable) bool {
	if r == 0 {
		return false
	}
	for _, t := range scripts {
		if unicode.Is(t, r) {
			return true
		}
	}
	return false
}

// Subdivision flags are U+1F3F4 followed by tag letters and a cancel tag, as in
// the Scotland flag. Walking back over the run finds the base.
func inFlagSequence(runes []rune, i int) bool {
	for j := i - 1; j >= 0; j-- {
		if isTagRune(runes[j]) {
			continue
		}
		return runes[j] == wavingBlackFlag
	}
	return false
}

func isTagRune(r rune) bool { return r >= 0xE0000 && r <= 0xE007F }

func at(runes []rune, i int) rune {
	if i < 0 || i >= len(runes) {
		return 0
	}
	return runes[i]
}

func takesVariationSelector(r rune) bool {
	if r == 0 {
		return false
	}
	if unicode.Is(unicode.S, r) {
		return true
	}
	// Keycap sequences are a digit, # or * followed by U+FE0F U+20E3.
	if (r >= '0' && r <= '9') || r == '#' || r == '*' {
		return true
	}
	return isIdeograph(r)
}

func isIdeograph(r rune) bool {
	return r != 0 && (unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r))
}

func isEmojiish(r rune) bool {
	if r == 0 {
		return false
	}
	if r >= 0x1F000 && r <= 0x1FAFF {
		return true
	}
	if r == 0xFE0F {
		return true
	}
	return unicode.Is(unicode.So, r)
}

func inJoiningScript(r rune) bool {
	if r == 0 {
		return false
	}
	for _, t := range joiningScripts {
		if unicode.Is(t, r) {
			return true
		}
	}
	return false
}
