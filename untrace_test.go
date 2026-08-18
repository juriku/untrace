package untrace_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/juriku/untrace"
)

func TestScanTextReportsWithoutChanging(t *testing.T) {
	in := "one" + string(rune(0x200B)) + "word"

	res := untrace.ScanText(in, "a.go", untrace.Options{})

	if res.Text != in {
		t.Errorf("text changed without Fix: %q", res.Text)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(res.Findings))
	}
	if !res.Actionable() {
		t.Error("a removable character was not reported as actionable")
	}
}

func TestScanTextFixes(t *testing.T) {
	res := untrace.ScanText("one"+string(rune(0x200B))+"word", "a.go", untrace.Options{Fix: true})

	if res.Text != "oneword" {
		t.Errorf("text = %q, want %q", res.Text, "oneword")
	}
	if !res.Changed {
		t.Error("Changed was not set")
	}
}

// The name decides the policy, so the same text is judged differently.
func TestScanTextResolvesFormatFromTheName(t *testing.T) {
	emDash := string(rune(0x2014))

	cases := map[string]struct {
		name  string
		fixed string
	}{
		"source":   {"a.go", "a - b"},
		"log":      {"a.log", "a " + emDash + " b"},
		"no name":  {"", "a - b"},
		"markdown": {"a.md", "a - b"},
	}

	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			res := untrace.ScanText("a "+emDash+" b", c.name, untrace.Options{Fix: true})
			if res.Text != c.fixed {
				t.Errorf("text = %q, want %q", res.Text, c.fixed)
			}
		})
	}
}

func TestScanTextReportsFormat(t *testing.T) {
	if got := untrace.ScanText("x", "a.md", untrace.Options{}).Format; got != "prose" {
		t.Errorf("format = %q, want prose", got)
	}
}

func TestScanTextDecodesPayloads(t *testing.T) {
	tagged := "data" + string(rune(0xE0074)) + string(rune(0xE0078))

	res := untrace.ScanText(tagged, "a.txt", untrace.Options{})

	if len(res.Payloads) != 1 {
		t.Fatalf("got %d payloads, want 1", len(res.Payloads))
	}
	if !res.Actionable() {
		t.Error("a decoded payload was not actionable")
	}
}

func TestScanTextFindsMixedScript(t *testing.T) {
	res := untrace.ScanText("login p"+string(rune(0x0430))+"ypal", "a.txt", untrace.Options{})

	if len(res.Mixed) != 1 {
		t.Fatalf("got %d mixed words, want 1", len(res.Mixed))
	}
}

func TestScanTextExcludesCodepoints(t *testing.T) {
	in := "a " + string(rune(0x2014)) + " b"

	res := untrace.ScanText(in, "a.go", untrace.Options{Fix: true, Exclude: []rune{0x2014}})

	if res.Text != in {
		t.Errorf("an excluded codepoint was rewritten: %q", res.Text)
	}
	if len(res.Findings) != 0 {
		t.Errorf("an excluded codepoint was reported: %+v", res.Findings)
	}
}

func TestScanTextStrictReportsLegitimateUse(t *testing.T) {
	family := string(rune(0x1F468)) + string(rune(0x200D)) + string(rune(0x1F469))

	if res := untrace.ScanText(family, "a.txt", untrace.Options{}); len(res.Findings) != 0 {
		t.Errorf("an emoji sequence was reported by default: %+v", res.Findings)
	}
	if res := untrace.ScanText(family, "a.txt", untrace.Options{Strict: true}); len(res.Findings) == 0 {
		t.Error("strict did not report the joiner")
	}
}

func TestScanTextCleanIsIdempotent(t *testing.T) {
	once := untrace.ScanText("one"+string(rune(0x200B))+"word", "a.go", untrace.Options{Fix: true})
	twice := untrace.ScanText(once.Text, "a.go", untrace.Options{Fix: true})

	if twice.Changed || twice.Text != once.Text {
		t.Errorf("a second pass changed %q to %q", once.Text, twice.Text)
	}
}

// A latin-1 document has to come back as latin-1, not as UTF-8. 0xA0 is a
// non-breaking space in latin-1, so the fixture carries a real marker without
// mixing encodings.
func TestScanBytesPreservesEncoding(t *testing.T) {
	in := []byte("caf\xe9 au\xa0lait\n")

	res, out, err := untrace.ScanBytes(in, "a.txt", untrace.Options{Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("the non-breaking space was not cleaned")
	}
	if bytes.Contains(out, []byte{0xc3, 0xa9}) {
		t.Errorf("latin-1 input came back as UTF-8: % x", out)
	}
	if want := []byte("caf\xe9 au lait\n"); !bytes.Equal(out, want) {
		t.Errorf("got % x, want % x", out, want)
	}
}

func TestScanBytesLeavesACleanFileByteIdentical(t *testing.T) {
	in := []byte("nothing to find here\n")

	res, out, err := untrace.ScanBytes(in, "a.txt", untrace.Options{Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Error("Changed set on a clean file")
	}
	if !bytes.Equal(out, in) {
		t.Errorf("bytes changed: % x", out)
	}
}

// Without a guard, a binary decodes as latin-1 and its bytes are rewritten.
func TestScanBytesRejectsBinary(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 32)...)
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8L\x9d\x01\x2a\x00")
	docx := append([]byte("PK\x03\x04"), make([]byte, 64)...)
	nul := []byte{'a', 0x00, 0xA0, 'b'}

	for name, data := range map[string][]byte{
		"png": png, "webp": webp, "zip": docx, "nul bytes": nul,
	} {
		t.Run(name, func(t *testing.T) {
			res, out, err := untrace.ScanBytes(data, "a.bin", untrace.Options{Fix: true})

			if !errors.Is(err, untrace.ErrNotText) {
				t.Fatalf("err = %v, want ErrNotText", err)
			}
			if res.Changed || out != nil {
				t.Errorf("returned a rewritten document: %q", out)
			}
		})
	}
}

func TestScanBytesRejectsUndecodableInput(t *testing.T) {
	unpaired := []byte{0xFF, 0xFE, 0x00, 0xD8}

	if _, _, err := untrace.ScanBytes(unpaired, "a.txt", untrace.Options{}); err == nil {
		t.Error("an unpaired surrogate was accepted, which would change the bytes")
	}
}

func TestActionableIsFalseOnACleanScan(t *testing.T) {
	if untrace.ScanText("clean text", "a.go", untrace.Options{}).Actionable() {
		t.Error("a clean scan reported something actionable")
	}
}

func TestLoadConfigOnADirectoryWithout(t *testing.T) {
	cfg, err := untrace.LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("a missing config is not an error: %v", err)
	}
	if cfg == nil {
		t.Error("no config returned")
	}
}
