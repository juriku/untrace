package detect

import "testing"

func TestDirectiveOnRequiresATrailingPosition(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"bare", "untrace:ignore", true},
		{"hash comment", `sample = "x"   # untrace:ignore`, true},
		{"slash comment", "value = 1  // untrace:ignore", true},
		{"block comment", "/* untrace:ignore */", true},
		{"html comment", "<!-- untrace:ignore -->", true},
		{"markdown code span", "`untrace:ignore`", true},
		{"trailing carriage return", "x  // untrace:ignore\r", true},

		{"markdown table row", "| `untrace:ignore` | the line it is on |", false},
		{"prose after", "untrace:ignore silences the line it is on", false},
		{"listed among others", "R13 is `untrace:ignore`, `untrace:ignore-file`", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := directiveOn(tc.line, directiveLine); got != tc.want {
				t.Errorf("directiveOn(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// A document that describes the directives must still be scanned; both this
// project's README and its resolver design doc silenced themselves this way.
func TestDocumentingTheDirectivesDoesNotSuppressTheFile(t *testing.T) {
	doc := "| `untrace:ignore` | the line it is on |\n" +
		"| `untrace:ignore-next-line` | the line below it |\n" +
		"| `untrace:ignore-file` | the whole file |\n" +
		"prose with a" + zwsp + "marker\n"

	got := parseSuppressions(doc)
	if got.wholeFile {
		t.Fatal("a table documenting ignore-file suppressed the whole file")
	}
	for line := 1; line <= 4; line++ {
		if got.covers(line) {
			t.Errorf("line %d suppressed", line)
		}
	}
}

func TestLongerDirectivesWinOverTheirPrefix(t *testing.T) {
	got := parseSuppressions("untrace:ignore-next-line\nbad\n")
	if !got.covers(2) {
		t.Error("ignore-next-line did not cover the line below")
	}
	if got.covers(1) {
		t.Error("ignore-next-line also matched as a bare ignore")
	}
	if got.wholeFile {
		t.Error("ignore-next-line matched as ignore-file")
	}
}
