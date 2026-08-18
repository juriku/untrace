package resolve

// R6 typography census, per docs/design/resolvers.md.

const establishedShare = 0.1

const minSample = 20

// A bulleted list contributes its hyphens to the straight side.
const minLines = 3

type Typography struct {
	straightQuote, curlyQuote int
	hyphen, dash              int
	curlyLines, dashLines     int
}

func NewTypography(runes []rune) Typography {
	var t Typography
	line := 1
	lastCurly, lastDash := 0, 0

	for _, r := range runes {
		switch r {
		case '\n':
			line++
		case '\'', '"':
			t.straightQuote++
		case 0x2018, 0x2019, 0x201C, 0x201D:
			t.curlyQuote++
			if line != lastCurly {
				t.curlyLines++
				lastCurly = line
			}
		case '-':
			t.hyphen++
		case 0x2013, 0x2014:
			t.dash++
			if line != lastDash {
				t.dashLines++
				lastDash = line
			}
		}
	}
	return t
}

// Only the typeset forms are asked about: a straight quote is never a finding,
// so it never reaches here.
func (t Typography) Consistent(r rune) bool {
	switch r {
	case 0x2018, 0x2019, 0x201C, 0x201D:
		return established(t.curlyQuote, t.straightQuote, t.curlyLines)
	case 0x2013, 0x2014:
		return established(t.dash, t.hyphen, t.dashLines)
	}
	return false
}

func established(form, other, lines int) bool {
	total := form + other
	if total < minSample || lines < minLines {
		return false
	}
	return float64(form)/float64(total) > establishedShare
}
