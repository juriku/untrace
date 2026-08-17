package detect

import (
	"testing"

	"github.com/juriku/untrace/internal/decode"
	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/resolve"
)

// Built from codepoints rather than literals: invisible characters are
// unreadable in source, and the Go compiler rejects a literal U+FEFF outright.
var (
	zwsp    = string(rune(0x200B))
	nbsp    = string(rune(0x00A0))
	thin    = string(rune(0x2009))
	ideoSp  = string(rune(0x3000))
	emDash  = string(rune(0x2014))
	lquote  = string(rune(0x201C))
	rquote  = string(rune(0x201D))
	bom     = string(rune(0xFEFF))
	ivsChar = string(rune(0xE0101))
)

// resolve.Ignore is the zero value, so an unset MixedScript silently differs
// from the CLI default.
func newDetector(f resolve.Format, clean bool) *Detector {
	return &Detector{
		Markers:     markers.Options{Typographic: true, IVS: true},
		Policy:      resolve.PolicyFor(f),
		Clean:       clean,
		MixedScript: resolve.Report,
	}
}

func TestHomoglyphsAreReportedButNotRewrittenByDefault(t *testing.T) {
	phish := "login at " + string(rune(0x0440)) + string(rune(0x0430)) + "ypal"

	d := newDetector(resolve.FormatSource, true)
	d.MixedScript = resolve.Report
	res := d.Run(phish)

	if res.Text != phish {
		t.Errorf("text = %q, want it left alone", res.Text)
	}
	if res.Changed {
		t.Error("Changed set with no rewrite applied")
	}
	if len(res.Mixed) != 1 {
		t.Fatalf("mixed words = %d, want the attack still reported", len(res.Mixed))
	}
	for _, f := range res.Findings {
		if f.Applied || f.Actionable {
			t.Errorf("U+%04X applied=%v actionable=%v, want report only",
				f.Rune, f.Applied, f.Actionable)
		}
	}
}

func TestFixHomoglyphsOptsIntoRewriting(t *testing.T) {
	phish := "login at " + string(rune(0x0440)) + string(rune(0x0430)) + "ypal"

	d := newDetector(resolve.FormatSource, true)
	d.MixedScript = resolve.Report
	d.FixHomoglyphs = true
	res := d.Run(phish)

	if res.Text != "login at paypal" {
		t.Errorf("text = %q, want the Latin form", res.Text)
	}
}

// A wholly Cyrillic word mixes nothing, so it never reaches the rewrite even
// with the flag on.
func TestFixHomoglyphsLeavesSingleScriptProseAlone(t *testing.T) {
	russian := "Привет мир"

	d := newDetector(resolve.FormatSource, true)
	d.MixedScript = resolve.Report
	d.FixHomoglyphs = true

	if got := d.Run(russian).Text; got != russian {
		t.Errorf("got %q, want %q", got, russian)
	}
}

func TestSpacesNormaliseRatherThanVanish(t *testing.T) {
	got := newDetector(resolve.FormatSource, true).Run("two" + nbsp + "words").Text
	if got != "two words" {
		t.Errorf("got %q, want %q", got, "two words")
	}
}

func TestExoticSpacesBecomeRegularSpaces(t *testing.T) {
	got := newDetector(resolve.FormatSource, true).Run("a" + thin + "b").Text
	if got != "a b" {
		t.Errorf("got %q, want %q", got, "a b")
	}
}

// The ideographic space is a layout unit in CJK typography, so it is reported
// where it looks out of place and never rewritten.
func TestIdeographicSpaceIsReportedNotRewritten(t *testing.T) {
	in := "a" + ideoSp + "b"
	res := newDetector(resolve.FormatSource, true).Run(in)

	if res.Text != in {
		t.Errorf("got %q, want it left alone", res.Text)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want it still reported", len(res.Findings))
	}
	if res.Findings[0].Actionable {
		t.Error("reported as actionable, want report only")
	}
}

func TestZeroWidthIsRemoved(t *testing.T) {
	got := newDetector(resolve.FormatSource, true).Run("one" + zwsp + "word").Text
	if got != "oneword" {
		t.Errorf("got %q, want %q", got, "oneword")
	}
}

func TestIVSIsRemoved(t *testing.T) {
	got := newDetector(resolve.FormatSource, true).Run("text" + ivsChar + "here").Text
	if got != "texthere" {
		t.Errorf("got %q, want %q", got, "texthere")
	}
}

func TestSingleIVSOnAnIdeographIsKept(t *testing.T) {
	in := "姓は辻" + ivsChar + "です"
	got := newDetector(resolve.FormatProse, true).Run(in).Text
	if got != in {
		t.Errorf("got %q, want it unchanged", got)
	}
}

// R8 outranks R9, so a run of selectors is a payload even where a single one
// would be a legitimate variation sequence.
func TestIVSPayloadAfterAnIdeographIsStillDecoded(t *testing.T) {
	res := newDetector(resolve.FormatProse, false).Run("辻" + decode.EncodeVariation([]byte("leak")))

	if len(res.Payloads) != 1 {
		t.Fatalf("got %d payloads, want 1", len(res.Payloads))
	}
	if got := res.Payloads[0].Text; got != "leak" {
		t.Errorf("decoded %q, want %q", got, "leak")
	}
}

func TestOfficeFormatIgnoresWordTypography(t *testing.T) {
	in := "a " + emDash + " dash and " + lquote + "quotes" + rquote
	res := newDetector(resolve.FormatOffice, true).Run(in)
	if res.Changed {
		t.Errorf("office document was rewritten: %q", res.Text)
	}
	if len(res.Findings) != 0 {
		t.Errorf("office document reported %d findings, want 0", len(res.Findings))
	}
}

func TestSourceFormatCleansTypography(t *testing.T) {
	res := newDetector(resolve.FormatSource, true).Run("a " + emDash + " dash")
	if res.Text != "a - dash" {
		t.Errorf("got %q, want %q", res.Text, "a - dash")
	}
}

func TestProseTypographyIsCleanedLikeSource(t *testing.T) {
	// Prose carries no typographic exemption: only Office formats do.
	res := newDetector(resolve.FormatProse, true).Run("a " + emDash + " dash")
	if res.Text != "a - dash" {
		t.Errorf("got %q, want %q", res.Text, "a - dash")
	}
}

func TestLogFilesAreFullyIgnored(t *testing.T) {
	in := "a " + emDash + " dash" + zwsp + " and " + nbsp + "spaces"
	res := newDetector(resolve.FormatLog, true).Run(in)
	if len(res.Findings) != 0 {
		t.Errorf("log file reported %d findings", len(res.Findings))
	}
	if res.Changed || res.Text != in {
		t.Errorf("log file was rewritten: %q", res.Text)
	}
}

func TestMarkupIsCleanedLikeSource(t *testing.T) {
	// Markup has no typographic exemptions: an nbsp in HTML source is still an
	// invisible character, whatever it was authored as.
	res := newDetector(resolve.FormatMarkup, true).Run("a" + nbsp + "b" + emDash + "c")
	if res.Text != "a b-c" {
		t.Errorf("got %q, want %q", res.Text, "a b-c")
	}
}

func TestLeadingBOMIsIgnoredMidFileIsNot(t *testing.T) {
	res := newDetector(resolve.FormatSource, true).Run(bom + "hello")
	if len(res.Findings) != 0 {
		t.Errorf("leading BOM reported: %+v", res.Findings)
	}

	res = newDetector(resolve.FormatSource, true).Run("hello" + bom + "there")
	if len(res.Findings) != 1 {
		t.Fatalf("mid-file BOM: got %d findings, want 1", len(res.Findings))
	}
}

func TestColumnIsRuneBased(t *testing.T) {
	res := newDetector(resolve.FormatSource, false).Run("café" + zwsp + "x")
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(res.Findings))
	}
	if res.Findings[0].Column != 5 {
		t.Errorf("column = %d, want 5", res.Findings[0].Column)
	}
}

func TestLineNumbers(t *testing.T) {
	res := newDetector(resolve.FormatSource, false).Run("clean\nline" + zwsp + "two\n")
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(res.Findings))
	}
	if res.Findings[0].Line != 2 {
		t.Errorf("line = %d, want 2", res.Findings[0].Line)
	}
}

func TestDetectOnlyLeavesTextAlone(t *testing.T) {
	in := "one" + zwsp + "word"
	res := newDetector(resolve.FormatSource, false).Run(in)
	if res.Text != in || res.Changed {
		t.Errorf("detect-only modified text: %q", res.Text)
	}
	if len(res.Findings) != 1 {
		t.Errorf("got %d findings, want 1", len(res.Findings))
	}
}

func TestCodepointFormatting(t *testing.T) {
	cases := map[rune]string{
		0x200B:  "U+200B",
		0x00A0:  "U+00A0",
		0xE0101: "U+E0101",
	}
	for r, want := range cases {
		if got := codepoint(r); got != want {
			t.Errorf("codepoint(%#x) = %q, want %q", r, got, want)
		}
	}
}

func TestEmojiSequencesAreLegitimate(t *testing.T) {
	zwj := string(rune(0x200D))
	vs16 := string(rune(0xFE0F))

	cases := []struct {
		name string
		in   string
	}{
		{"heart with vs16", "love " + string(rune(0x2764)) + vs16 + " here"},
		{"family zwj sequence", string(rune(0x1F468)) + zwj + string(rune(0x1F469)) + zwj + string(rune(0x1F467))},
		{"keycap", "1" + vs16 + string(rune(0x20E3))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := newDetector(resolve.FormatSource, true).Run(tc.in)
			if len(res.Findings) != 0 {
				t.Errorf("reported %d findings on legitimate emoji: %+v", len(res.Findings), res.Findings)
			}
			if res.Text != tc.in {
				t.Errorf("emoji was rewritten: %q", res.Text)
			}
		})
	}
}

func TestZWJBetweenLatinLettersIsFlagged(t *testing.T) {
	in := "he" + string(rune(0x200D)) + "llo"
	res := newDetector(resolve.FormatSource, true).Run(in)
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(res.Findings))
	}
	if res.Text != "hello" {
		t.Errorf("got %q, want %q", res.Text, "hello")
	}
}

func TestJoinersInIndicAndArabicAreLegitimate(t *testing.T) {
	zwnj := string(rune(0x200C))
	// Devanagari ka + ZWNJ + virama, and an Arabic letter pair.
	for _, in := range []string{
		string(rune(0x0915)) + zwnj + string(rune(0x094D)),
		string(rune(0x0645)) + zwnj + string(rune(0x0646)),
	} {
		res := newDetector(resolve.FormatSource, true).Run(in)
		if len(res.Findings) != 0 {
			t.Errorf("flagged a required joiner in %q: %+v", in, res.Findings)
		}
	}
}

func TestVariationSelectorOnIdeographIsLegitimate(t *testing.T) {
	in := string(rune(0x845B)) + string(rune(0xFE00))
	res := newDetector(resolve.FormatSource, true).Run(in)
	if len(res.Findings) != 0 {
		t.Errorf("flagged a CJK variation selector: %+v", res.Findings)
	}
}

func TestStrictModeReportsLegitimateOccurrences(t *testing.T) {
	in := "love " + string(rune(0x2764)) + string(rune(0xFE0F)) + " here"
	d := newDetector(resolve.FormatSource, false)
	d.Strict = true
	res := d.Run(in)
	if len(res.Findings) != 1 {
		t.Errorf("strict mode got %d findings, want 1", len(res.Findings))
	}
}

func TestLegitimateCountIsReported(t *testing.T) {
	in := "love " + string(rune(0x2764)) + string(rune(0xFE0F))
	res := newDetector(resolve.FormatSource, false).Run(in)
	if res.Legitimate != 1 {
		t.Errorf("Legitimate = %d, want 1", res.Legitimate)
	}
}

func TestIgnoreDirectiveSuppressesItsOwnLine(t *testing.T) {
	text := "clean line\nbad" + zwsp + "here  untrace:ignore\nalso" + zwsp + "bad\n"
	res := newDetector(resolve.FormatSource, false).Run(text)

	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(res.Findings), res.Findings)
	}
	if res.Findings[0].Line != 3 {
		t.Errorf("surviving finding is on line %d, want 3", res.Findings[0].Line)
	}
	if res.Suppressed != 1 {
		t.Errorf("Suppressed = %d, want 1", res.Suppressed)
	}
}

func TestIgnoreNextLineDirective(t *testing.T) {
	text := "untrace:ignore-next-line\nbad" + zwsp + "here\nalso" + zwsp + "bad\n"
	res := newDetector(resolve.FormatSource, false).Run(text)

	if len(res.Findings) != 1 || res.Findings[0].Line != 3 {
		t.Fatalf("got %+v, want a single finding on line 3", res.Findings)
	}
}

func TestIgnoreFileDirective(t *testing.T) {
	text := "header untrace:ignore-file\nbad" + zwsp + "\nmore" + zwsp + "\n"
	res := newDetector(resolve.FormatSource, false).Run(text)

	if len(res.Findings) != 0 {
		t.Errorf("ignore-file left %d findings", len(res.Findings))
	}
	if res.Suppressed != 2 {
		t.Errorf("Suppressed = %d, want 2", res.Suppressed)
	}
}

func TestSuppressedLinesAreNotRewritten(t *testing.T) {
	text := "keep" + nbsp + "this  untrace:ignore\n"
	res := newDetector(resolve.FormatSource, true).Run(text)

	if res.Changed || res.Text != text {
		t.Errorf("suppressed line was rewritten: %q", res.Text)
	}
}

func TestPayloadCarriesItsLineAndColumn(t *testing.T) {
	payload := decode.EncodeTag("leak")
	text := "first line\nsecond line\nhere: " + payload + " end\n"

	res := newDetector(resolve.FormatSource, false).Run(text)
	if len(res.Payloads) != 1 {
		t.Fatalf("got %d payloads, want 1", len(res.Payloads))
	}

	got := res.Payloads[0]
	if got.Line != 3 {
		t.Errorf("Line = %d, want 3", got.Line)
	}
	// "here: " is six runes, so the run starts at the seventh.
	if got.Column != 7 {
		t.Errorf("Column = %d, want 7", got.Column)
	}
	if got.Text != "leak" {
		t.Errorf("Text = %q, want %q", got.Text, "leak")
	}
}

func TestDirectiveSuppressesPayloads(t *testing.T) {
	tagged := "data" + string(rune(0xE0074)) + string(rune(0xE0078)) + "  untrace:ignore\n"
	res := newDetector(resolve.FormatSource, false).Run(tagged)

	if len(res.Payloads) != 0 {
		t.Errorf("payload survived a directive: %+v", res.Payloads)
	}
	if res.Suppressed == 0 {
		t.Error("suppression was not counted")
	}
}

func TestDirectiveSuppressesMixedScript(t *testing.T) {
	cyrillicA := string(rune(0x0430))
	text := "login at p" + cyrillicA + "ypal  untrace:ignore\n"
	res := newDetector(resolve.FormatSource, false).Run(text)

	if len(res.Mixed) != 0 {
		t.Errorf("mixed-script word survived a directive: %+v", res.Mixed)
	}
}

func TestDirectiveOnlyAffectsItsLine(t *testing.T) {
	text := "first" + zwsp + "\nsecond untrace:ignore\nthird" + zwsp + "\n"
	res := newDetector(resolve.FormatSource, false).Run(text)

	if len(res.Findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(res.Findings))
	}
	for _, f := range res.Findings {
		if f.Line == 2 {
			t.Errorf("finding on the suppressed line: %+v", f)
		}
	}
}
