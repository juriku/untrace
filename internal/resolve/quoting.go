package resolve

import (
	"path/filepath"
	"strings"
)

var (
	jsonExts = map[string]bool{".json": true, ".jsonc": true, ".ipynb": true}
	yamlExts = map[string]bool{".yaml": true, ".yml": true}
	iniExts  = map[string]bool{".ini": true, ".cfg": true, ".conf": true}
)

// quoting is how a straight double quote must be written at one position.
type quoting int

const (
	// quotingUnknown means the enclosing syntax could not be determined, so no
	// replacement is safe.
	quotingUnknown quoting = iota
	quotingEscaped
	quotingRaw
)

// Rewriter adapts a replacement to the syntax at a rune offset. The bool is
// false when no replacement can be written safely there, which makes the
// finding report-only.
type Rewriter func(replacement string, at int) (string, bool)

// RewriterFor returns the Rewriter a document's syntax requires, or nil when
// replacements can always be written literally.
func RewriterFor(path, text string) Rewriter {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case jsonExts[ext]:
		// A curly quote can only appear inside a string literal in valid JSON,
		// so the enclosing style is known without scanning.
		return fixedQuoting(quotingEscaped)
	case ext == ".toml":
		return scannedQuoting(text, tomlQuoting)
	case yamlExts[ext]:
		return scannedQuoting(text, yamlQuoting)
	case iniExts[ext]:
		// No single grammar across INI dialects.
		return fixedQuoting(quotingUnknown)
	}
	return nil
}

func fixedQuoting(q quoting) Rewriter {
	return func(replacement string, _ int) (string, bool) {
		return applyQuoting(replacement, q)
	}
}

func scannedQuoting(text string, scan func(string) []quoting) Rewriter {
	var table []quoting
	return func(replacement string, at int) (string, bool) {
		if !strings.Contains(replacement, `"`) {
			return replacement, true
		}
		if table == nil {
			table = scan(text)
		}
		if at < 0 || at >= len(table) {
			return "", false
		}
		return applyQuoting(replacement, table[at])
	}
}

func applyQuoting(replacement string, q quoting) (string, bool) {
	if !strings.Contains(replacement, `"`) {
		return replacement, true
	}
	switch q {
	case quotingEscaped:
		return strings.ReplaceAll(replacement, `"`, `\"`), true
	case quotingRaw:
		return replacement, true
	}
	return "", false
}

// tomlQuoting classifies every rune position in a TOML document.
//
// Bare keys allow only A-Za-z0-9_- , so a curly quote outside a string or a
// comment means the document is already invalid and nothing is written there.
func tomlQuoting(text string) []quoting {
	runes := []rune(text)
	out := make([]quoting, len(runes))

	const (
		none = iota
		basic
		literal
		multiBasic
		multiLiteral
		comment
	)
	state := none

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch state {
		case none:
			switch {
			case r == '#':
				state = comment
			case hasRun(runes, i, '"', 3):
				state = multiBasic
				out[i], out[i+1], out[i+2] = quotingUnknown, quotingUnknown, quotingUnknown
				i += 2
				continue
			case r == '"':
				state = basic
			case hasRun(runes, i, '\'', 3):
				state = multiLiteral
				out[i], out[i+1], out[i+2] = quotingUnknown, quotingUnknown, quotingUnknown
				i += 2
				continue
			case r == '\'':
				state = literal
			}
			out[i] = quotingUnknown
			continue

		case basic, multiBasic:
			if r == '\\' && i+1 < len(runes) {
				out[i], out[i+1] = quotingEscaped, quotingEscaped
				i++
				continue
			}
			if state == basic && (r == '"' || r == '\n') {
				state = none
			} else if state == multiBasic && hasRun(runes, i, '"', 3) {
				state = none
				out[i], out[i+1], out[i+2] = quotingEscaped, quotingEscaped, quotingEscaped
				i += 2
				continue
			}
			out[i] = quotingEscaped
			continue

		case literal, multiLiteral:
			if state == literal && (r == '\'' || r == '\n') {
				state = none
			} else if state == multiLiteral && hasRun(runes, i, '\'', 3) {
				state = none
				out[i], out[i+1], out[i+2] = quotingRaw, quotingRaw, quotingRaw
				i += 2
				continue
			}
			out[i] = quotingRaw
			continue

		case comment:
			if r == '\n' {
				state = none
			}
			out[i] = quotingRaw
			continue
		}
	}
	return out
}

// yamlQuoting classifies every rune position in a YAML document, refusing every
// case whose enclosing style is not certain.
//
// A plain scalar is refused rather than written raw: a quote is legal inside
// one, but at its start it turns the scalar into a quoted scalar and drops the
// quotes from the value, which changes data rather than a character.
func yamlQuoting(text string) []quoting {
	runes := []rune(text)
	out := make([]quoting, len(runes))

	const (
		plain = iota
		double
		single
		comment
		blockScalar
	)
	state := plain
	lost := false

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if lost {
			out[i] = quotingUnknown
			continue
		}

		if r == '\n' {
			// A quote left open at end of line means a scalar spanning lines,
			// which this scanner cannot follow.
			if state == double || state == single {
				lost = true
			}
			if state != blockScalar {
				state = plain
			}
			out[i] = quotingUnknown
			continue
		}

		switch state {
		case plain:
			switch r {
			case '#':
				state = comment
			case '"':
				state = double
			case '\'':
				state = single
			case '|', '>':
				if restOfLineBlank(runes, i+1) {
					state = blockScalar
				}
			}
			out[i] = quotingUnknown

		case double:
			if r == '\\' && i+1 < len(runes) {
				out[i], out[i+1] = quotingEscaped, quotingEscaped
				i++
				continue
			}
			if r == '"' {
				state = plain
			}
			out[i] = quotingEscaped

		case single:
			if r == '\'' {
				// A doubled quote is an escaped one, not the end of the scalar.
				if i+1 < len(runes) && runes[i+1] == '\'' {
					out[i], out[i+1] = quotingRaw, quotingRaw
					i++
					continue
				}
				state = plain
			}
			out[i] = quotingRaw

		case comment, blockScalar:
			out[i] = quotingRaw
		}
	}
	return out
}

func hasRun(runes []rune, at int, r rune, n int) bool {
	if at+n > len(runes) {
		return false
	}
	for i := at; i < at+n; i++ {
		if runes[i] != r {
			return false
		}
	}
	return true
}

func restOfLineBlank(runes []rune, from int) bool {
	for i := from; i < len(runes) && runes[i] != '\n'; i++ {
		if runes[i] != ' ' && runes[i] != '\t' {
			return false
		}
	}
	return true
}
