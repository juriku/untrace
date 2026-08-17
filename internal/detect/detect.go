package detect

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/juriku/untrace/internal/decode"
	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/resolve"
	"github.com/juriku/untrace/internal/script"
)

type Finding struct {
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Rune        rune   `json:"-"`
	Codepoint   string `json:"codepoint"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Action      string `json:"action"`
	Replacement string `json:"replacement,omitempty"`
	Applied     bool   `json:"applied"`
	// InPayload marks a character that belongs to a decoded run, so reports can
	// show the payload instead of every character in it.
	InPayload bool `json:"in_payload,omitempty"`
	// Actionable is true when --fix would change this character. Findings the
	// policy only reports, such as typography in prose, are informational.
	Actionable bool `json:"actionable"`
}

type Detector struct {
	Markers markers.Options
	Policy  resolve.Policy
	Clean   bool
	// Strict disables the legitimacy resolver, reporting every marker found.
	Strict bool
	// MixedScript governs confusable letters; Ignore switches the check off.
	MixedScript resolve.Action
	// FixHomoglyphs lets --fix rewrite a confusable letter to its Latin
	// counterpart, which is a guess about the script the author meant.
	FixHomoglyphs bool
	// Regions marks lines that follow different rules, such as fenced code
	// inside prose. Nil means the whole document uses Policy as given.
	Regions resolve.Regions
}

type MixedWord struct {
	Line    int      `json:"line"`
	Column  int      `json:"column"`
	Word    string   `json:"word"`
	Scripts []string `json:"scripts"`
}

// decode works in absolute rune offsets and has no concept of lines, so the
// position is attached here rather than there.
type Payload struct {
	decode.Payload
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Result struct {
	Text     string
	Findings []Finding
	Payloads []Payload   `json:"payloads,omitempty"`
	Mixed    []MixedWord `json:"mixed_script,omitempty"`
	Changed  bool
	// Legitimate counts occurrences suppressed because the character was doing
	// its designed job, such as a joiner inside an emoji sequence.
	Legitimate int
	// Suppressed counts occurrences silenced by an untrace:ignore directive.
	Suppressed int
}

// Run requires valid UTF-8. Column is a 1-based rune offset.
func (d *Detector) Run(text string) Result {
	runes := []rune(text)
	res := Result{Text: text}

	suppressed := parseSuppressions(text)

	// A decoded run outranks legitimacy: two variation selectors carrying bytes
	// are an attack even where one would be ordinary emoji presentation.
	var lines []int
	lineAt := func(offset int) int {
		if lines == nil {
			lines = lineIndex(runes)
		}
		return lineOf(lines, offset)
	}

	for _, p := range decode.Payloads(runes) {
		line := lineAt(p.Start)
		if suppressed.covers(line) {
			res.Suppressed++
			continue
		}
		res.Payloads = append(res.Payloads, Payload{
			Payload: p,
			Line:    line,
			Column:  columnOf(runes, p.Start),
		})
	}
	var inPayload bitset
	payloadRunes := 0
	for _, p := range res.Payloads {
		if inPayload == nil {
			inPayload = newBitset(len(runes))
		}
		for i := p.Start; i < p.End; i++ {
			inPayload.set(i)
		}
		payloadRunes += p.End - p.Start
	}
	if payloadRunes > 0 {
		res.Findings = make([]Finding, 0, payloadRunes)
	}

	// A confusable letter only means something inside a word that mixes scripts.
	// In single-script text it is just that language being written.
	var mixedWords []script.Word
	var suspect bitset
	if d.MixedScript != resolve.Ignore {
		for _, w := range script.MixedWords(runes) {
			if suppressed.covers(lineAt(w.Start)) {
				res.Suppressed++
				continue
			}
			if suspect == nil {
				suspect = newBitset(len(runes))
			}
			mixedWords = append(mixedWords, w)
			for _, i := range w.Minority {
				suspect.set(i)
			}
		}
	}
	nextMixed := 0

	var out strings.Builder
	if d.Clean {
		out.Grow(len(text))
	}
	write := func(s string) {
		if d.Clean {
			out.WriteString(s)
		}
	}

	var codepoints map[rune]string
	var census resolve.Census
	censusReady := false

	line, col := 1, 1
	byteAt := 0

	for i, r := range runes {
		_, size := utf8.DecodeRuneInString(text[byteAt:])
		orig := text[byteAt : byteAt+size]
		byteAt += size

		// The cursor is only correct because MixedWords returns words in
		// ascending Start order.
		if nextMixed < len(mixedWords) && mixedWords[nextMixed].Start == i {
			w := mixedWords[nextMixed]
			nextMixed++
			res.Mixed = append(res.Mixed, MixedWord{
				Line: line, Column: col, Word: w.Text, Scripts: w.Scripts,
			})
		}

		if r == '\n' {
			write(orig)
			line++
			col = 1
			continue
		}

		// An invalid byte decodes as RuneError with width 1; a genuine U+FFFD is
		// three bytes wide.
		if r == utf8.RuneError && size == 1 {
			write(orig)
			col++
			continue
		}

		m, ok := markers.Lookup(r, d.Markers)
		if !ok {
			write(orig)
			col++
			continue
		}

		if !d.Strict && m.Kind == markers.Typographic && unicode.IsLetter(r) && !suspect.has(i) {
			write(orig)
			res.Legitimate++
			col++
			continue
		}

		if !d.Strict && !inPayload.has(i) {
			if !censusReady && resolve.NeedsCensus(r) {
				census = resolve.NewCensus(runes)
				censusReady = true
			}
			if legit, _ := resolve.Legitimate(runes, i, census); legit {
				write(orig)
				res.Legitimate++
				col++
				continue
			}
		}

		policy := d.Policy.InRegion(d.Regions.At(line))

		if policy.For(m) == resolve.Ignore {
			write(orig)
			col++
			continue
		}

		if suppressed.covers(line) {
			write(orig)
			res.Suppressed++
			col++
			continue
		}

		if codepoints == nil {
			codepoints = make(map[rune]string)
		}
		cp, ok := codepoints[r]
		if !ok {
			cp = codepoint(r)
			codepoints[r] = cp
		}

		f := Finding{
			Line:      line,
			Column:    col,
			Rune:      r,
			Codepoint: cp,
			Name:      m.Name,
			Kind:      m.Kind.String(),
			Action:    "detected",
			InPayload: inPayload.has(i),
		}

		homoglyph := m.Kind == markers.Typographic && unicode.IsLetter(r)

		f.Actionable = policy.For(m) == resolve.Clean && m.CanClean &&
			!sameRune(m.Replacement, r) && (d.FixHomoglyphs || !homoglyph)

		apply := d.Clean && f.Actionable
		if apply && !sameRune(m.Replacement, r) {
			f.Replacement = m.Replacement
			f.Applied = true
			if m.Replacement == "" {
				f.Action = "removed"
			} else {
				f.Action = "replaced"
			}
			write(m.Replacement)
			res.Changed = true
		} else {
			write(orig)
		}

		res.Findings = appendFinding(res.Findings, f)
		col++
	}

	if d.Clean {
		res.Text = out.String()
	}
	return res
}

// The size check is load-bearing: DecodeRuneInString("") returns U+FFFD, so
// without it an empty replacement would match a genuine U+FFFD in the input.
func sameRune(s string, r rune) bool {
	rr, size := utf8.DecodeRuneInString(s)
	return size > 0 && size == len(s) && rr == r
}

// append grows a large slice by about 1.25x, which copies roughly five times
// the final size. Doubling costs two.
func appendFinding(dst []Finding, f Finding) []Finding {
	if len(dst) == cap(dst) {
		n := cap(dst) * 2
		if n == 0 {
			n = 8
		}
		grown := make([]Finding, len(dst), n)
		copy(grown, dst)
		dst = grown
	}
	return append(dst, f)
}

func codepoint(r rune) string {
	const hex = "0123456789ABCDEF"
	v := uint32(r)
	buf := make([]byte, 0, 8)
	for shift := 20; shift >= 0; shift -= 4 {
		d := (v >> uint(shift)) & 0xF
		if len(buf) == 0 && d == 0 && shift > 12 {
			continue
		}
		buf = append(buf, hex[d])
	}
	return "U+" + string(buf)
}

// One entry per line, not per rune: a per-rune index costs eight bytes for
// every rune in the document to answer a handful of lookups.
func lineIndex(runes []rune) []int {
	starts := []int{0}
	for i, r := range runes {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// columnOf returns the 1-based rune offset within the line holding offset.
func columnOf(runes []rune, offset int) int {
	if offset > len(runes) {
		offset = len(runes)
	}
	col := 1
	for i := offset - 1; i >= 0 && runes[i] != '\n'; i-- {
		col++
	}
	return col
}

func lineOf(starts []int, offset int) int {
	if offset < 0 {
		return 1
	}
	lo, hi := 0, len(starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if starts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1
}
