package detect

import (
	"testing"
	"unicode/utf8"

	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/resolve"
)

func seedText(f *testing.F) {
	f.Add("plain text")
	f.Add("two" + nbsp + "words")
	f.Add("zero" + zwsp + "width")
	f.Add("love " + string(rune(0x2764)) + string(rune(0xFE0F)))
	f.Add("p" + string(rune(0x0430)) + "ypal")
	f.Add("tag" + string(rune(0xE0074)) + string(rune(0xE0078)))
	f.Add("skip me untrace:ignore")
	f.Add("")
}

func FuzzDetectOnlyNeverMutates(f *testing.F) {
	seedText(f)
	f.Fuzz(func(t *testing.T, text string) {
		d := &Detector{
			Markers:     markers.Options{Typographic: true, IVS: true},
			Policy:      resolve.PolicyFor(resolve.FormatSource),
			MixedScript: resolve.Report,
		}
		res := d.Run(text)

		if res.Text != text {
			t.Fatalf("detect-only changed the text\n got %q\nwant %q", res.Text, text)
		}
		if res.Changed {
			t.Fatal("detect-only reported a change")
		}
	})
}

func FuzzCleanIsIdempotent(f *testing.F) {
	seedText(f)
	f.Fuzz(func(t *testing.T, text string) {
		// Deleting a rune can splice surrounding invalid bytes into a new valid
		// one, so idempotence only holds on the valid UTF-8 Run documents.
		if !utf8.ValidString(text) {
			t.Skip()
		}

		clean := func(s string) Result {
			d := &Detector{
				Markers:     markers.Options{Typographic: true, IVS: true},
				Policy:      resolve.PolicyFor(resolve.FormatSource),
				Clean:       true,
				MixedScript: resolve.Report,
			}
			return d.Run(s)
		}

		once := clean(text)
		twice := clean(once.Text)

		// A second --fix has to be a no-op, or repeated runs keep editing files.
		if twice.Text != once.Text {
			t.Fatalf("cleaning is not idempotent\nfirst  %q\nsecond %q", once.Text, twice.Text)
		}
		if twice.Changed {
			t.Fatalf("second pass reported a change: %q", twice.Text)
		}
	})
}

func FuzzSuppressionOnlyReduces(f *testing.F) {
	seedText(f)
	f.Fuzz(func(t *testing.T, text string) {
		run := func(s string) Result {
			d := &Detector{
				Markers:     markers.Options{Typographic: true, IVS: true},
				Policy:      resolve.PolicyFor(resolve.FormatSource),
				MixedScript: resolve.Report,
			}
			return d.Run(s)
		}

		plain := run(text)
		suppressed := run("untrace:ignore-file\n" + text)

		if len(suppressed.Findings) > len(plain.Findings) {
			t.Fatalf("suppression added findings: %d then %d",
				len(plain.Findings), len(suppressed.Findings))
		}
		if len(suppressed.Findings) != 0 {
			t.Fatalf("ignore-file left %d findings", len(suppressed.Findings))
		}
	})
}
