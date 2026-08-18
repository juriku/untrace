package resolve

import (
	"strings"
	"testing"
)

func typographyOf(s string) Typography { return NewTypography([]rune(s)) }

func TestConsistentTypographyIsEstablished(t *testing.T) {
	// A document typeset with curly quotes throughout.
	doc := strings.Repeat("the "+string(rune(0x201C))+"word"+string(rune(0x201D))+" here.\n", 20)

	if !typographyOf(doc).Consistent(0x201C) {
		t.Error("curly quotes used throughout were treated as an anomaly")
	}
}

func TestOneCurlyQuoteAmongManyStraightIsAnomalous(t *testing.T) {
	doc := strings.Repeat(`say "word" here. `, 40) + "one " + string(rune(0x201C)) + "odd"

	if typographyOf(doc).Consistent(0x201C) {
		t.Error("a lone curly quote was treated as the established form")
	}
}

func TestDashesAndHyphensAreJudgedSeparately(t *testing.T) {
	emDash := string(rune(0x2014))
	doc := strings.Repeat("a "+emDash+" b\n", 30)

	got := typographyOf(doc)
	if !got.Consistent(0x2014) {
		t.Error("dashes used throughout were treated as an anomaly")
	}
	// Quotes are absent, so nothing can be established about them.
	if got.Consistent(0x201C) {
		t.Error("a quote was called established in a document with none")
	}
}

// A ratio over a handful of characters says nothing, so a short document must
// never suppress on it.
func TestASmallSampleIsNeverEstablished(t *testing.T) {
	doc := "a " + string(rune(0x2014)) + " b"

	if typographyOf(doc).Consistent(0x2014) {
		t.Error("a three-character sample established a convention")
	}
}

// Nine bullets contribute eighteen hyphens to the straight side, which clears
// the share threshold for three em dashes pasted into one paragraph. Requiring
// three distinct lines is what separates a typeset document from a paste.
func TestAPasteOnOneLineIsNotEstablished(t *testing.T) {
	em := string(rune(0x2014))
	doc := strings.Repeat("- item - detail\n", 9) +
		"\nA pasted line " + em + " with three " + em + " em dashes " + em + " here.\n"

	got := typographyOf(doc)
	if got.dash+got.hyphen < minSample {
		t.Fatalf("fixture does not reach the sample floor: %d", got.dash+got.hyphen)
	}
	if float64(got.dash)/float64(got.dash+got.hyphen) <= establishedShare {
		t.Fatalf("fixture does not clear the share threshold, so it proves nothing")
	}
	if got.Consistent(0x2014) {
		t.Error("three dashes on one line were treated as the document's convention")
	}
}

func TestTheSameDashesSpreadAcrossLinesAreEstablished(t *testing.T) {
	em := string(rune(0x2014))
	doc := strings.Repeat("- item - detail\n", 9) +
		"A line " + em + " here.\nAnother " + em + " there.\nA third " + em + " one.\n"

	if !typographyOf(doc).Consistent(0x2014) {
		t.Error("dashes across three lines were treated as a paste")
	}
}

func TestStraightFormsAreNeverAsked(t *testing.T) {
	doc := strings.Repeat(`say "word" here. `, 40)

	for _, r := range []rune{'"', '\'', '-'} {
		if typographyOf(doc).Consistent(r) {
			t.Errorf("%q was reported as an established typeset form", r)
		}
	}
}

func TestMixedButMajorityStraightStillEstablishesTheMinority(t *testing.T) {
	// Twelve curly against sixty-eight straight is above the anomaly line:
	// deliberate, if untidy.
	curly := "say " + string(rune(0x201C)) + "word" + string(rune(0x201D)) + ".\n"
	doc := strings.Repeat("say \"word\".\n", 34) + strings.Repeat(curly, 6)

	if !typographyOf(doc).Consistent(0x201C) {
		t.Error("a consistent minority form was treated as a stray")
	}
}
