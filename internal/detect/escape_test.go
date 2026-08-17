package detect

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/resolve"
)

func syntaxDetector(path, text string, clean bool) *Detector {
	return &Detector{
		Markers: markers.Options{Typographic: true},
		Policy:  resolve.PolicyFor(resolve.FormatData),
		Clean:   clean,
		Rewrite: resolve.RewriterFor(path, text),
	}
}

func curly(s string) string {
	return string(rune(0x201C)) + s + string(rune(0x201D))
}

func fixed(t *testing.T, path, text string) string {
	t.Helper()
	return syntaxDetector(path, text, true).Run(text).Text
}

func TestFixingCurlyQuotesKeepsJSONValid(t *testing.T) {
	in := `{"note": "he said ` + curly("hi") + `"}`

	var out map[string]string
	got := fixed(t, "doc.json", in)
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("fix produced invalid JSON %q: %v", got, err)
	}
	if out["note"] != `he said "hi"` {
		t.Errorf("decoded value = %q, want %q", out["note"], `he said "hi"`)
	}
}

func TestReportedReplacementIsWhatGetsWritten(t *testing.T) {
	in := `{"a": "` + curly("x") + `"}`

	res := syntaxDetector("doc.json", in, false).Run(in)
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
	in := `{"a": "x` + string(rune(0x2014)) + `y"}`

	if got := fixed(t, "doc.json", in); !strings.Contains(got, "x-y") {
		t.Errorf("em dash not replaced plainly: %q", got)
	}
}

func TestQuotedScalarsAreEscaped(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"yaml double": {`a: "he said ` + curly("hi") + `"`, `a: "he said \"hi\""`},
		"toml basic":  {`a = "he said ` + curly("hi") + `"`, `a = "he said \"hi\""`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := "f.yaml"
			if strings.HasPrefix(name, "toml") {
				path = "f.toml"
			}
			if got := fixed(t, path, tc.in); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// A quote is ordinary content in these styles, so escaping would insert a
// literal backslash.
func TestUnescapedStylesTakeTheQuoteRaw(t *testing.T) {
	cases := map[string]struct{ path, in, want string }{
		"yaml single":     {"f.yaml", `a: 'he said ` + curly("hi") + `'`, `a: 'he said "hi"'`},
		"toml literal":    {"f.toml", `a = 'he said ` + curly("hi") + `'`, `a = 'he said "hi"'`},
		"toml multi-line": {"f.toml", `a = '''` + curly("hi") + `'''`, `a = '''"hi"'''`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := fixed(t, tc.path, tc.in); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// Writing a straight quote at the start of a plain scalar would turn it into a
// quoted one and drop the quotes from the value.
func TestPlainYAMLScalarsAreLeftAlone(t *testing.T) {
	in := `a: he said ` + curly("hi")

	res := syntaxDetector("f.yaml", in, true).Run(in)
	if res.Text != in {
		t.Errorf("text rewritten to %q", res.Text)
	}
	for _, f := range res.Findings {
		if f.Rune == 0x201C && f.Actionable {
			t.Error("curly quote in a plain scalar reported as actionable")
		}
	}
}

func TestOtherMarkersStillFixedInYAML(t *testing.T) {
	in := "title: a" + string(rune(0x2014)) + "b"

	if got := fixed(t, "f.yaml", in); got != "title: a-b" {
		t.Errorf("= %q, want %q", got, "title: a-b")
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
