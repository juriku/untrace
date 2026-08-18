package untrace_test

import "testing"

// The Windows path scrub replaced every backslash, which rewrote the escaped
// quote in JSON and SARIF output and made those golden files unmatchable.
func TestPathSeparatorRewriteSparesEscapes(t *testing.T) {
	cases := map[string]string{
		`src\app\main.go`:            "src/app/main.go",
		`a\b`:                        "a/b",
		`dir\.gitignore`:             "dir/.gitignore",
		`{"path":"src\app\main.go"}`: `{"path":"src/app/main.go"}`,
		`replaced U+201C -> "\""`:    `replaced U+201C -> "\""`,
		`decoding to \"tx\"`:         `decoding to \"tx\"`,
		`"text": "word \\\" here"`:   `"text": "word \\\" here"`,
		`no backslash at all`:        "no backslash at all",
		`trailing backslash \`:       `trailing backslash \`,
	}

	for in, want := range cases {
		if got := pathSeparator.ReplaceAllString(in, "/$1"); got != want {
			t.Errorf("scrub(%q) = %q, want %q", in, got, want)
		}
	}
}
