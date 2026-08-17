package detect

import (
	"strings"
	"testing"

	"github.com/juriku/untrace/internal/decode"
	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/resolve"
)

const corpusBytes = 1 << 20

// Corpora are sized in bytes, not runes: divide B/op by corpusBytes to read the
// allocation amplification.
func repeatTo(unit string, n int) string {
	if unit == "" {
		return ""
	}
	count := n / len(unit)
	if count < 1 {
		count = 1
	}
	return strings.Repeat(unit, count)
}

var (
	corpusCleanASCII  = repeatTo("the quick brown fox jumps over the lazy dog. ", corpusBytes)
	corpusMarkerDense = repeatTo("a"+emDash+"b"+nbsp+"c"+zwsp+"d ", corpusBytes)
	corpusTypical     = repeatTo(strings.Repeat("ordinary source line of text\n", 40)+
		"a line with an "+emDash+" in it\n", corpusBytes)

	// A lone 0xFF can never begin a UTF-8 sequence, so each drives the
	// invalid-byte branch.
	corpusInvalidBytes = repeatTo("valid text \xff more text \xfe ", corpusBytes)

	zwj    = string(rune(0x200D))
	family = "\U0001F468" + zwj + "\U0001F469" + zwj + "\U0001F467"

	corpusEmoji  = repeatTo(family+" ", corpusBytes)
	corpusCJKIVS = repeatTo(string(rune(0x8FBB))+ivsChar+string(rune(0x9089))+ivsChar+" ", corpusBytes)

	corpusPayloads = repeatTo(
		"carrier text "+decode.EncodeZeroWidth([]byte("tracked-by:acct-99213"))+" more ",
		corpusBytes)

	// Cyrillic a inside an otherwise Latin word, the canonical homoglyph shape.
	corpusMixedScript = repeatTo("login at p"+string(rune(0x0430))+"ypal today ", corpusBytes)

	corpusSuppressed = repeatTo("value = \"x"+zwsp+"y\"  // untrace:ignore\n", corpusBytes)

	corpusOneLongLine    = repeatTo("word ", corpusBytes)
	corpusManyShortLines = repeatTo("word\n", corpusBytes)

	corpusMarkdown = repeatTo("prose paragraph here\n\n```bash\ncurl "+emDash+"silent url\n```\n\n",
		corpusBytes)
)

func benchRun(b *testing.B, d *Detector, text string) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Run(text)
	}
}

// Paired with BenchmarkRunFix: the gap is the output builder, which should not
// run without --fix.
func BenchmarkRunDetectOnly(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusTypical)
}

func BenchmarkRunFix(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, true), corpusTypical)
}

// Paired with BenchmarkRunInvalidBytes: only invalid input should cost extra.
func BenchmarkRunValidUTF8(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusCleanASCII)
}

func BenchmarkRunInvalidBytes(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusInvalidBytes)
}

func BenchmarkRunCleanASCII(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusCleanASCII)
}

func BenchmarkRunMarkerDense(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusMarkerDense)
}

// Same byte count, opposite line counts.
func BenchmarkRunOneLongLine(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusOneLongLine)
}

func BenchmarkRunManyShortLines(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusManyShortLines)
}

func BenchmarkRunEmojiHeavy(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusEmoji)
}

func BenchmarkRunCJKWithIVS(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusCJKIVS)
}

func BenchmarkRunPayloadHeavy(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusPayloads)
}

func BenchmarkRunMixedScript(b *testing.B) {
	d := newDetector(resolve.FormatSource, false)
	d.MixedScript = resolve.Report
	benchRun(b, d, corpusMixedScript)
}

// Paired with BenchmarkRunMixedScript: the gap is the script census.
func BenchmarkRunMixedScriptDisabled(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusMixedScript)
}

func BenchmarkRunSuppressionHeavy(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatSource, false), corpusSuppressed)
}

// Paired with BenchmarkRunEmojiHeavy over the same corpus: the gap is the
// legitimacy resolver, which --strict disables.
func BenchmarkRunStrict(b *testing.B) {
	d := newDetector(resolve.FormatSource, false)
	d.Strict = true
	benchRun(b, d, corpusEmoji)
}

func BenchmarkRunMarkdownFences(b *testing.B) {
	d := newDetector(resolve.FormatProse, false)
	d.Regions = resolve.RegionsFor(resolve.FormatProse, corpusMarkdown)
	benchRun(b, d, corpusMarkdown)
}

func BenchmarkRunOfficePolicy(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatOffice, false), corpusMarkerDense)
}

func BenchmarkRunLogPolicy(b *testing.B) {
	benchRun(b, newDetector(resolve.FormatLog, false), corpusMarkerDense)
}

func BenchmarkRunTypographyOff(b *testing.B) {
	d := &Detector{
		Markers: markers.Options{Typographic: false, IVS: false},
		Policy:  resolve.PolicyFor(resolve.FormatSource),
	}
	benchRun(b, d, corpusTypical)
}

// MB/s must hold flat across sizes; a falling figure is quadratic behaviour.
func BenchmarkRunBySize(b *testing.B) {
	sizes := []struct {
		name string
		n    int
	}{
		{"64KB", 64 << 10},
		{"256KB", 256 << 10},
		{"1MB", 1 << 20},
		{"4MB", 4 << 20},
	}
	for _, s := range sizes {
		text := repeatTo("the quick brown fox jumps over the lazy dog. ", s.n)
		b.Run(s.name, func(b *testing.B) {
			benchRun(b, newDetector(resolve.FormatSource, false), text)
		})
	}
}
