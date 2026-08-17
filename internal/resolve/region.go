package resolve

import "strings"

// Region is the part of a document a character sits in. The same character can
// be authorial in one region and a defect in another: an em dash in a Markdown
// paragraph is deliberate typography, while the same character inside a fenced
// code block will be copied into a terminal and break.
type Region int

const (
	RegionDefault Region = iota
	RegionCode
)

// Regions maps 1-based line numbers to their region. A nil result means the
// whole document is RegionDefault.
type Regions map[int]Region

func (r Regions) At(line int) Region {
	if r == nil {
		return RegionDefault
	}
	return r[line]
}

// RegionsFor identifies the code regions of a document. Only Markdown is
// handled: source files are code throughout, and picking comments and string
// literals out of arbitrary languages needs a real parser per language.
func RegionsFor(format Format, text string) Regions {
	if format != FormatProse && format != FormatNotebook {
		return nil
	}
	return markdownRegions(text)
}

// A fence is three or more backticks or tildes, and must be closed by at least
// as many of the same character. Indented code blocks are not treated as code:
// four-space indentation is too common in ordinary prose lists to be reliable.
func markdownRegions(text string) Regions {
	out := Regions{}

	var fenceChar byte
	var fenceLen int
	inFence := false

	for i, line := range strings.Split(text, "\n") {
		lineNum := i + 1
		trimmed := strings.TrimLeft(line, " \t")

		if char, n := fenceMarker(trimmed); n > 0 {
			switch {
			case !inFence:
				inFence, fenceChar, fenceLen = true, char, n
				out[lineNum] = RegionCode
				continue
			case char == fenceChar && n >= fenceLen:
				out[lineNum] = RegionCode
				inFence = false
				continue
			}
		}

		if inFence {
			out[lineNum] = RegionCode
		}
	}
	return out
}

func fenceMarker(trimmed string) (byte, int) {
	if len(trimmed) < 3 {
		return 0, 0
	}
	char := trimmed[0]
	if char != '`' && char != '~' {
		return 0, 0
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == char {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return char, n
}

// A configured action survives the switch to source rules: an exclusion covers
// the whole file, fences included.
func (p Policy) InRegion(r Region) Policy {
	if r != RegionCode {
		return p
	}
	source := PolicyFor(FormatSource)
	source.Format = p.Format
	source.PerRune = p.PerRune
	return source.Override(p.configured.hidden, p.configured.typographic, p.configured.ivs)
}
