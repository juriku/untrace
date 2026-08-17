package detect

import "strings"

// Directives are matched anywhere on a line rather than inside a comment,
// because untrace has no per-language parser.
const (
	directiveFile     = "untrace:ignore-file"
	directiveNextLine = "untrace:ignore-next-line"
	directiveLine     = "untrace:ignore"
)

// Comment terminators and markup punctuation, the only things allowed to follow
// a directive. Prose after one means the line documents the directive rather
// than using it, which is how this file's own README once silenced itself.
const directiveClosers = "*/->}])\"'`"

type suppression struct {
	wholeFile bool
	lines     map[int]bool
}

func (s suppression) covers(line int) bool {
	return s.wholeFile || s.lines[line]
}

func directiveOn(line, directive string) bool {
	i := strings.Index(line, directive)
	if i < 0 {
		return false
	}
	for _, r := range line[i+len(directive):] {
		if r == ' ' || r == '\t' || r == '\r' {
			continue
		}
		if !strings.ContainsRune(directiveClosers, r) {
			return false
		}
	}
	return true
}

// Longer directives are tested first: ignore-next-line contains ignore.
func parseSuppressions(text string) suppression {
	out := suppression{lines: map[int]bool{}}

	for i, line := range strings.Split(text, "\n") {
		lineNum := i + 1
		switch {
		case directiveOn(line, directiveFile):
			out.wholeFile = true
			return out
		case directiveOn(line, directiveNextLine):
			out.lines[lineNum+1] = true
		case directiveOn(line, directiveLine):
			out.lines[lineNum] = true
		}
	}
	return out
}
