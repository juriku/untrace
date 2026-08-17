package resolve

import (
	"strings"
	"testing"
)

// writeAt returns what the rewriter would write for a straight quote replacing
// the first curly quote in text.
func writeAt(t *testing.T, path, text string) (string, bool) {
	t.Helper()
	i := strings.IndexRune(text, 0x201C)
	if i < 0 {
		t.Fatalf("no curly quote in %q", text)
	}
	return RewriterFor(path, text)(`"`, len([]rune(text[:i])))
}

func TestRewriterOnlyWhereQuotesAreStructural(t *testing.T) {
	cases := map[string]bool{
		"a.json": true, "a.jsonc": true, "nb.ipynb": true, "A.IPYNB": true,
		"a.yaml": true, "a.yml": true, "a.toml": true,
		"a.ini": true, "a.cfg": true, "a.conf": true,
		"a.md": false, "a.go": false, "a.txt": false, "a": false,
		"dir.json/a.c": false,
	}
	for path, want := range cases {
		if got := RewriterFor(path, "") != nil; got != want {
			t.Errorf("%s: rewriter present = %v, want %v", path, got, want)
		}
	}
}

func TestNonQuoteReplacementsAlwaysPass(t *testing.T) {
	for _, path := range []string{"a.json", "a.toml", "a.yaml", "a.ini"} {
		rw := RewriterFor(path, "anything")
		for _, safe := range []string{"-", "'", "...", " "} {
			if got, ok := rw(safe, 0); !ok || got != safe {
				t.Errorf("%s: rewrite(%q) = %q, %v", path, safe, got, ok)
			}
		}
	}
}

func TestTOMLStringStyles(t *testing.T) {
	q := string(rune(0x201C))
	cases := map[string]struct {
		text string
		want string
		ok   bool
	}{
		"basic takes an escape":     {`a = "x` + q + `y"`, `\"`, true},
		"literal takes none":        {`a = 'x` + q + `y'`, `"`, true},
		"multi-line basic escapes":  {`a = """x` + q + `y"""`, `\"`, true},
		"multi-line literal raw":    {`a = '''x` + q + `y'''`, `"`, true},
		"comment is content":        {`a = 1 # x` + q + `y`, `"`, true},
		"outside a string is bogus": {`a` + q + ` = 1`, "", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := writeAt(t, "a.toml", tc.text)
			if ok != tc.ok || got != tc.want {
				t.Errorf("= %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// A backslash escapes the next character, so \" must not be read as the end of
// the string.
func TestTOMLEscapedQuoteDoesNotEndTheString(t *testing.T) {
	q := string(rune(0x201C))
	if got, ok := writeAt(t, "a.toml", `a = "he said \"hi\" and `+q+`bye"`); !ok || got != `\"` {
		t.Errorf("= %q, %v; want %q, true", got, ok, `\"`)
	}
}

func TestTOMLBasicStringEndsAtNewline(t *testing.T) {
	q := string(rune(0x201C))
	if _, ok := writeAt(t, "a.toml", "a = \"unterminated\nb = "+q+"\n"); ok {
		t.Error("a quote after an unterminated string was accepted")
	}
}

func TestYAMLScalarStyles(t *testing.T) {
	q := string(rune(0x201C))
	cases := map[string]struct {
		text string
		want string
		ok   bool
	}{
		"double quoted escapes": {`a: "x` + q + `y"`, `\"`, true},
		"single quoted raw":     {`a: 'x` + q + `y'`, `"`, true},
		"comment is content":    {`a: 1 # x` + q + `y`, `"`, true},
		"plain scalar refused":  {`a: x` + q + `y`, "", false},
		"block scalar raw":      {"a: |\n  x" + q + "y\n", `"`, true},
		"folded scalar raw":     {"a: >\n  x" + q + "y\n", `"`, true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := writeAt(t, "a.yaml", tc.text)
			if ok != tc.ok || got != tc.want {
				t.Errorf("= %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// A doubled apostrophe is an escaped quote inside a single-quoted scalar,
// not the end of it.
func TestYAMLDoubledSingleQuoteStaysInsideTheScalar(t *testing.T) {
	q := string(rune(0x201C))
	if got, ok := writeAt(t, "a.yaml", `a: 'it''s `+q+`x'`); !ok || got != `"` {
		t.Errorf("= %q, %v; want %q, true", got, ok, `"`)
	}
}

// A scalar left open at end of line spans lines, which this scanner cannot
// follow, so everything after it is refused.
func TestYAMLUnterminatedQuoteRefusesTheRest(t *testing.T) {
	q := string(rune(0x201C))
	if _, ok := writeAt(t, "a.yaml", "a: \"open\nb: \"x"+q+"y\"\n"); ok {
		t.Error("a quote after an unterminated scalar was accepted")
	}
}

func TestINIRefusesEverything(t *testing.T) {
	q := string(rune(0x201C))
	for _, path := range []string{"a.ini", "a.cfg", "a.conf"} {
		if _, ok := writeAt(t, path, `a = "x`+q+`y"`); ok {
			t.Errorf("%s: accepted a quote replacement", path)
		}
	}
}

func TestJSONAlwaysEscapes(t *testing.T) {
	q := string(rune(0x201C))
	if got, ok := writeAt(t, "a.json", `{"a": "x`+q+`y"}`); !ok || got != `\"` {
		t.Errorf("= %q, %v; want %q, true", got, ok, `\"`)
	}
}
