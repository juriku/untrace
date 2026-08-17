package resolve

import "testing"

func TestRewriterOnlyWhereQuotesAreStructural(t *testing.T) {
	cases := map[string]bool{
		"a.json": true, "a.jsonc": true, "nb.ipynb": true, "A.IPYNB": true,
		"a.yaml": true, "a.yml": true, "a.toml": true, "a.ini": true,
		"a.cfg": true, "a.conf": true,
		"a.md": false, "a.go": false, "a.txt": false, "a": false,
		"dir.json/a.c": false,
	}
	for path, want := range cases {
		if got := RewriterFor(path) != nil; got != want {
			t.Errorf("%s: rewriter present = %v, want %v", path, got, want)
		}
	}
}

func TestJSONEscapesTheQuoteAndPassesTheRest(t *testing.T) {
	rw := RewriterFor("a.json")

	cases := map[string]string{`"`: `\"`, "-": "-", "'": "'", "...": "..."}
	for in, want := range cases {
		got, ok := rw(in)
		if !ok {
			t.Errorf("rewrite(%q) refused", in)
			continue
		}
		if got != want {
			t.Errorf("rewrite(%q) = %q, want %q", in, got, want)
		}
	}
}

// Escaping is only correct inside a style that takes backslash escapes, and
// yaml and toml each have one that does not.
func TestQuotedFormatsRefuseTheQuoteAndAllowTheRest(t *testing.T) {
	for _, path := range []string{"a.yaml", "a.toml", "a.ini"} {
		rw := RewriterFor(path)

		if _, ok := rw(`"`); ok {
			t.Errorf("%s: accepted a bare quote replacement", path)
		}
		for _, safe := range []string{"-", "'", "...", " "} {
			got, ok := rw(safe)
			if !ok || got != safe {
				t.Errorf("%s: rewrite(%q) = %q, %v", path, safe, got, ok)
			}
		}
	}
}
