package detect

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/resolve"
)

func syntaxDetector(path string, clean bool) *Detector {
	return &Detector{
		Markers: markers.Options{Typographic: true},
		Policy:  resolve.PolicyFor(resolve.FormatData),
		Clean:   clean,
		Rewrite: resolve.RewriterFor(path),
	}
}

func curly(s string) string {
	return string(rune(0x201C)) + s + string(rune(0x201D))
}

func TestFixingCurlyQuotesKeepsJSONValid(t *testing.T) {
	in := `{"note": "he said ` + curly("hi") + `"}`

	res := syntaxDetector("doc.json", true).Run(in)
	if !res.Changed {
		t.Fatal("nothing replaced")
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(res.Text), &out); err != nil {
		t.Fatalf("fix produced invalid JSON %q: %v", res.Text, err)
	}
	if out["note"] != `he said "hi"` {
		t.Errorf("decoded value = %q, want %q", out["note"], `he said "hi"`)
	}
}

func TestReportedReplacementIsWhatGetsWritten(t *testing.T) {
	res := syntaxDetector("doc.json", false).Run(`{"a": "` + curly("x") + `"}`)

	for _, f := range res.Findings {
		if f.Rune != 0x201C {
			continue
		}
		if f.Replacement != `\"` {
			t.Errorf("reported replacement = %q, want %q", f.Replacement, `\"`)
		}
		return
	}
	t.Fatal("the curly quote was not reported")
}

func TestNonQuoteReplacementsAreUntouchedInJSON(t *testing.T) {
	res := syntaxDetector("doc.json", true).Run(`{"a": "x` + string(rune(0x2014)) + `y"}`)

	if !strings.Contains(res.Text, "x-y") {
		t.Errorf("em dash not replaced plainly: %q", res.Text)
	}
}

// Writing a straight quote here would end the scalar that held the character.
func TestQuotesAreLeftAloneInYAMLAndTOML(t *testing.T) {
	for _, path := range []string{"a.yaml", "a.toml"} {
		in := `title: "he said ` + curly("hi") + `"`

		res := syntaxDetector(path, true).Run(in)
		if res.Text != in {
			t.Errorf("%s: text was rewritten to %q", path, res.Text)
		}
		for _, f := range res.Findings {
			if f.Rune == 0x201C && f.Actionable {
				t.Errorf("%s: curly quote reported as actionable", path)
			}
		}
	}
}

func TestOtherMarkersStillFixedInYAML(t *testing.T) {
	res := syntaxDetector("a.yaml", true).Run("title: a" + string(rune(0x2014)) + "b")

	if res.Text != "title: a-b" {
		t.Errorf("text = %q, want %q", res.Text, "title: a-b")
	}
}

func TestWithoutARewriterTheQuoteIsWrittenRaw(t *testing.T) {
	d := &Detector{
		Markers: markers.Options{Typographic: true},
		Policy:  resolve.PolicyFor(resolve.FormatProse),
		Clean:   true,
	}

	if res := d.Run("say " + curly("hi")); res.Text != `say "hi"` {
		t.Errorf("text = %q, want %q", res.Text, `say "hi"`)
	}
}
