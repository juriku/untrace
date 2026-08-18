package resolve

import (
	"testing"

	"github.com/juriku/untrace/internal/markers"
)

// The two lists live in different packages because markers cannot import
// resolve. Drift between them is silent: a rune in one and not the other is
// either cleanable punctuation with no script rule, or a script rule for a
// character that is never looked up.
func TestScriptPunctuationMatchesTheNeverCleanedSet(t *testing.T) {
	for r := range scriptPunctuation {
		if !markers.NeverCleaned(r) {
			t.Errorf("U+%04X has a script rule but is still cleanable", r)
		}
		if _, ok := markers.Lookup(r, markers.Options{Typographic: true}); !ok {
			t.Errorf("U+%04X has a script rule but is not a marker", r)
		}
	}
}

const (
	greekQuestionMark = 0x037E
	arabicComma       = 0x060C
	ideographicStop   = 0x3002
	fullwidthQuestion = 0xFF1F
	ideographicSpace  = 0x3000
	rli               = 0x2067
	lri               = 0x2066
	pdi               = 0x2069
	rlo               = 0x202E
	pdf               = 0x202C
)

// Long enough that a few non-Latin letters stay under nativeShare.
const (
	englishLead = "the participant summarised the finding as "
	englishTail = " and the phrasing stuck in later commentary"
)

func englishAround(quote string) string { return englishLead + quote + englishTail }

func TestScriptPunctuationIsLegitimateInItsOwnScript(t *testing.T) {
	cases := []struct {
		name string
		text []rune
		at   int
		want bool
	}{
		{"ideographic stop after kana", []rune("です" + string(rune(ideographicStop))), 2, true},
		{"fullwidth question after kana", []rune("か" + string(rune(fullwidthQuestion))), 1, true},
		{"ideographic space between kanji", []rune("本" + string(rune(ideographicSpace)) + "語"), 1, true},
		{"arabic comma after arabic", []rune("مرحبا" + string(rune(arabicComma))), 5, true},
		{"greek question mark after greek", []rune("κάνεις" + string(rune(greekQuestionMark))), 6, true},

		{"ideographic stop in latin prose", []rune("end" + string(rune(ideographicStop)) + "next"), 3, false},
		{"arabic comma in latin prose", []rune("one" + string(rune(arabicComma)) + "two"), 3, false},
		{"greek question mark in latin prose", []rune("cost" + string(rune(greekQuestionMark)) + "x"), 4, false},
		{"fullwidth question in latin prose", []rune("really" + string(rune(fullwidthQuestion))), 6, false},

		{"quoted japanese inside english", []rune(englishAround("です" + string(rune(ideographicStop)))),
			len([]rune(englishLead)) + 2, true},
		{"quoted arabic inside english", []rune(englishAround("مرحبا" + string(rune(arabicComma)))),
			len([]rune(englishLead)) + 5, true},
		{"quoted greek inside english", []rune(englishAround("κάνεις" + string(rune(greekQuestionMark)))),
			len([]rune(englishLead)) + 6, true},

		{"ideographic stop away from the japanese it belongs to",
			[]rune("日本語 " + englishLead + string(rune(ideographicStop)) + englishTail),
			len([]rune("日本語 " + englishLead)), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := Legitimate(tc.text, tc.at, NewCensus(tc.text))
			if got != tc.want {
				t.Errorf("Legitimate = %v (%q), want %v", got, reason, tc.want)
			}
		})
	}
}

// Overrides are the Trojan Source vector (CVE-2021-42574) and must never be
// suppressed, however the surrounding text reads.
func TestBidiControlsLegitimacy(t *testing.T) {
	arabic := "مرحبا"

	cases := []struct {
		name string
		text []rune
		at   int
		want bool
	}{
		{"isolate wrapping arabic", []rune(string(rune(rli)) + arabic + string(rune(pdi))), 0, true},
		{"closing pdi of that pair", []rune(string(rune(rli)) + arabic + string(rune(pdi))), 6, true},
		{"nested isolates around arabic", []rune(string(rune(lri)) + string(rune(rli)) + arabic +
			string(rune(pdi)) + string(rune(pdi))), 0, true},

		{"unterminated isolate", []rune(string(rune(rli)) + arabic), 0, false},
		{"isolate wrapping latin only", []rune(string(rune(rli)) + "hello" + string(rune(pdi))), 0, false},
		{"override around arabic", []rune(string(rune(rlo)) + arabic + string(rune(pdf))), 0, false},
		{"pop formatting", []rune(string(rune(rlo)) + arabic + string(rune(pdf))), 6, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := Legitimate(tc.text, tc.at, NewCensus(tc.text))
			if got != tc.want {
				t.Errorf("Legitimate = %v (%q), want %v", got, reason, tc.want)
			}
		})
	}
}

// Emoji sequences below are from Unicode Emoji 17.0:
// https://unicode.org/Public/emoji/latest/emoji-zwj-sequences.txt
// https://unicode.org/Public/emoji/latest/emoji-sequences.txt
const (
	zwj       = 0x200D
	zwnj      = 0x200C
	vs15      = 0xFE0E
	vs16      = 0xFE0F
	vs1       = 0xFE00
	mongolFVS = 0x180B
	keycap    = 0x20E3
)

// A subdivision flag is U+1F3F4, then the CLDR region subtag lowercased with
// hyphens removed and each character offset by U+E0000, then U+E007F CANCEL TAG.
func tagFlag(region string) []rune {
	out := []rune{wavingBlackFlag}
	for _, c := range region {
		out = append(out, 0xE0000+c)
	}
	return append(out, 0xE007F)
}

func TestEmojiCarriersAreLegitimate(t *testing.T) {
	family := []rune{0x1F468, zwj, 0x1F469, zwj, 0x1F467, zwj, 0x1F466}
	healthWorker := []rune{0x1F468, 0x1F3FB, zwj, 0x2695, vs16}
	coupleWithHeart := []rune{0x1F469, 0x1F3FB, zwj, 0x2764, vs16, zwj, 0x1F468, 0x1F3FC}

	cases := []struct {
		name string
		text []rune
		at   int
	}{
		{"family first joiner", family, 1},
		{"family second joiner", family, 3},
		{"family third joiner", family, 5},
		{"joiner after a skin tone modifier", []rune{0x1F468, 0x1F3FB, zwj, 0x1F4BB}, 2},
		{"joiner before a bare symbol", healthWorker, 2},
		{"selector on that symbol", healthWorker, 4},
		{"joiner before a bare heart", coupleWithHeart, 2},
		{"selector on that heart", coupleWithHeart, 4},
		{"joiner after a selector", coupleWithHeart, 5},
		{"handshake with two skin tones", []rune{0x1FAF1, 0x1F3FB, zwj, 0x1FAF2, 0x1F3FC}, 2},
		{"gender-neutral role sequence", []rune{0x1F9D1, zwj, 0x1F680}, 1},
		{"gender-neutral role with a skin tone", []rune{0x1F9D1, 0x1F3FD, zwj, 0x1F3EB}, 2},
		{"people holding hands, first joiner", []rune{0x1F9D1, zwj, 0x1F91D, zwj, 0x1F9D1}, 1},
		{"people holding hands, second joiner", []rune{0x1F9D1, zwj, 0x1F91D, zwj, 0x1F9D1}, 3},
		{"rainbow flag", []rune{0x1F3F3, vs16, zwj, 0x1F308}, 2},
		{"pirate flag", []rune{wavingBlackFlag, zwj, 0x2620, vs16}, 1},
		{"eye in speech bubble", []rune{0x1F441, vs16, zwj, 0x1F5E8, vs16}, 2},
		{"keycap number sign", []rune{'#', vs16, keycap}, 1},
		{"keycap seven", []rune{'7', vs16, keycap}, 1},
		{"text presentation on a heart", []rune{0x2764, vs15}, 1},
		{"variation selector on an ideograph", []rune{0x8FBB, vs1}, 1},
		{"ideographic variation selector 17", []rune{0x8FBB, 0xE0100}, 1},
		{"ideographic variation selector 256", []rune{0x8FBB, 0xE01EF}, 1},
		{"ideographic variation selector on kana", []rune{0x30C4, 0xE0100}, 1},
		{"mongolian free variation selector", []rune{0x1826, mongolFVS}, 1},
		{"byte order mark at offset zero", []rune{0xFEFF, 'h', 'i'}, 0},
		{"persian non-joiner", []rune("م" + string(rune(zwnj)) + "ن"), 1},
		{"devanagari joiner", []rune("क" + string(rune(zwj)) + "ष"), 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legit, reason := Legitimate(tc.text, tc.at, NewCensus(tc.text))
			if !legit {
				t.Errorf("U+%04X at %d reported, want legitimate", tc.text[tc.at], tc.at)
			}
			if legit && reason == "" {
				t.Error("legitimate with no reason")
			}
		})
	}
}

func TestHiddenCarriersAreReported(t *testing.T) {
	cases := []struct {
		name string
		text []rune
		at   int
	}{
		{"joiner between latin letters", []rune{'a', zwj, 'b'}, 1},
		{"joiner between digits", []rune{'1', zwj, '2'}, 1},
		{"non-joiner between latin letters", []rune{'a', zwnj, 'b'}, 1},
		{"selector after a letter", []rune{'a', vs16}, 1},
		{"selector after a space", []rune{' ', vs16}, 1},
		{"text presentation after a letter", []rune{'a', vs15}, 1},
		{"ideographic selector after a letter", []rune{'a', vs1}, 1},
		{"ideographic variation selector after a letter", []rune{'a', 0xE0100}, 1},
		{"ideographic variation selector after a space", []rune{' ', 0xE0100}, 1},
		{"mongolian selector after a letter", []rune{'a', mongolFVS}, 1},
		{"byte order mark mid file", []rune{'a', 0xFEFF, 'b'}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if legit, reason := Legitimate(tc.text, tc.at, NewCensus(tc.text)); legit {
				t.Errorf("U+%04X at %d suppressed as %q, want reported", tc.text[tc.at], tc.at, reason)
			}
		})
	}
}

func TestSubdivisionFlagTagsAreLegitimate(t *testing.T) {
	for _, region := range []string{"gbeng", "gbsct", "gbwls"} {
		t.Run(region, func(t *testing.T) {
			runes := tagFlag(region)
			for i := 1; i < len(runes); i++ {
				if legit, _ := Legitimate(runes, i, NewCensus(runes)); !legit {
					t.Errorf("U+%04X at %d reported", runes[i], i)
				}
			}
		})
	}
}

func TestTagRunDetachedFromItsFlagIsReported(t *testing.T) {
	runes := append(tagFlag("gbsct"), 'x', 0xE0068, 0xE0069)

	if legit, reason := Legitimate(runes, len(runes)-1, NewCensus(runes)); legit {
		t.Errorf("detached tag suppressed as %q, want reported", reason)
	}
	if legit, _ := Legitimate(runes, 1, NewCensus(runes)); !legit {
		t.Error("the flag's own tag characters were reported")
	}
}

func TestJoinersAreJudgedIndependentlyWithinOneRun(t *testing.T) {
	runes := []rune{'h', 'i', ' ', 0x1F468, zwj, 0x1F469, ' ', 'a', zwj, 'b'}

	if legit, _ := Legitimate(runes, 4, NewCensus(runes)); !legit {
		t.Error("emoji joiner reported")
	}
	if legit, reason := Legitimate(runes, 8, NewCensus(runes)); legit {
		t.Errorf("latin joiner suppressed as %q", reason)
	}
}

func TestJoinerBetweenArbitrarySymbolsIsLegitimate(t *testing.T) {
	symbols := []rune{0x2695, zwj, 0x2696}
	if legit, _ := Legitimate(symbols, 1, NewCensus(symbols)); !legit {
		t.Error("joiner between two symbols reported")
	}
}

func TestRegionalIndicatorPairHasNoCarrier(t *testing.T) {
	germany := []rune{0x1F1E9, 0x1F1EA}
	for i := range germany {
		if legit, reason := Legitimate(germany, i, NewCensus(germany)); legit {
			t.Errorf("U+%04X judged legitimate as %q, want no verdict", germany[i], reason)
		}
	}
}
