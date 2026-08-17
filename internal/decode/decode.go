package decode

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Scheme string

const (
	SchemeTag       Scheme = "tag-ascii"
	SchemeVariation Scheme = "variation-selector"
	SchemeZeroWidth Scheme = "zero-width-binary"
)

type Payload struct {
	Scheme Scheme `json:"scheme"`
	// Start and End are rune offsets into the scanned text.
	Start int `json:"start"`
	End   int `json:"end"`
	Runes int `json:"runes"`
	// Text is the decoded content, set only when it decodes to printable text.
	Text      string `json:"text,omitempty"`
	Bytes     []byte `json:"-"`
	Printable bool   `json:"printable"`
}

const (
	wavingBlackFlag = 0x1F3F4

	// Tag characters have no legitimate use outside subdivision flags, which are
	// excluded separately, so even one carries meaning. A single variation
	// selector is ordinary emoji presentation, so that needs a run.
	minTagRun       = 1
	minVariationRun = 2
	minZeroWidthRun = 8
)

// Payloads finds runs of carrier characters and decodes what they spell.
//
// Presence of a marker is weak evidence; a run that decodes to readable text is
// strong evidence, because random damage does not spell words.
func Payloads(runes []rune) []Payload {
	var out []Payload
	out = append(out, tagPayloads(runes)...)
	out = append(out, variationPayloads(runes)...)
	out = append(out, zeroWidthPayloads(runes)...)
	return out
}

func isTag(r rune) bool { return r >= 0xE0000 && r <= 0xE007F }

func tagToASCII(r rune) (byte, bool) {
	if r < 0xE0020 || r > 0xE007E {
		return 0, false
	}
	return byte(r - 0xE0000), true
}

// A subdivision flag is U+1F3F4 followed by tag letters, so the run spells a
// region code rather than a hidden message.
func partOfFlag(runes []rune, start int) bool {
	return start > 0 && runes[start-1] == wavingBlackFlag
}

func tagPayloads(runes []rune) []Payload {
	var out []Payload
	for i := 0; i < len(runes); {
		if !isTag(runes[i]) {
			i++
			continue
		}
		start := i
		n := 0
		for i < len(runes) && isTag(runes[i]) {
			if _, ok := tagToASCII(runes[i]); ok {
				n++
			}
			i++
		}
		if n < minTagRun || partOfFlag(runes, start) {
			continue
		}
		b := make([]byte, 0, n)
		for j := start; j < i; j++ {
			if c, ok := tagToASCII(runes[j]); ok {
				b = append(b, c)
			}
		}
		out = append(out, newPayload(SchemeTag, start, i, b))
	}
	return out
}

func variationByte(r rune) (byte, bool) {
	switch {
	case r >= 0xFE00 && r <= 0xFE0F:
		return byte(r - 0xFE00), true
	case r >= 0xE0100 && r <= 0xE01EF:
		return byte(16 + (r - 0xE0100)), true
	}
	return 0, false
}

func variationPayloads(runes []rune) []Payload {
	var out []Payload
	for i := 0; i < len(runes); {
		if _, ok := variationByte(runes[i]); !ok {
			i++
			continue
		}
		start := i
		for i < len(runes) {
			if _, ok := variationByte(runes[i]); !ok {
				break
			}
			i++
		}
		// One selector after a symbol is ordinary emoji presentation; a run is not.
		if i-start < minVariationRun {
			continue
		}
		b := make([]byte, 0, i-start)
		for j := start; j < i; j++ {
			c, _ := variationByte(runes[j])
			b = append(b, c)
		}
		out = append(out, newPayload(SchemeVariation, start, i, b))
	}
	return out
}

// The common convention is zero-width space for 0 and non-joiner for 1, with
// joiners and word joiners acting as separators.
func zeroWidthBit(r rune) (bit int, sep bool, ok bool) {
	switch r {
	case 0x200B:
		return 0, false, true
	case 0x200C:
		return 1, false, true
	case 0x200D, 0x2060, 0xFEFF:
		return 0, true, true
	}
	return 0, false, false
}

func zeroWidthPayloads(runes []rune) []Payload {
	var out []Payload
	for i := 0; i < len(runes); {
		if _, _, ok := zeroWidthBit(runes[i]); !ok {
			i++
			continue
		}
		start := i
		n := 0
		for i < len(runes) {
			_, sep, ok := zeroWidthBit(runes[i])
			if !ok {
				break
			}
			if !sep {
				n++
			}
			i++
		}
		if n < minZeroWidthRun {
			continue
		}
		b := make([]byte, 0, n/8)
		var v byte
		held := 0
		for j := start; j < i; j++ {
			bit, sep, _ := zeroWidthBit(runes[j])
			if sep {
				continue
			}
			v = v<<1 | byte(bit)
			held++
			if held == 8 {
				b = append(b, v)
				v, held = 0, 0
			}
		}
		if len(b) == 0 {
			continue
		}
		out = append(out, newPayload(SchemeZeroWidth, start, i, b))
	}
	return out
}

func newPayload(s Scheme, start, end int, b []byte) Payload {
	p := Payload{Scheme: s, Start: start, End: end, Runes: end - start, Bytes: b}
	if text, ok := printableText(b); ok {
		p.Text = text
		p.Printable = true
	}
	return p
}

// printableText accepts only decodings that look like deliberate content, so a
// run of arbitrary selectors is reported without a bogus decoded string.
func printableText(b []byte) (string, bool) {
	if len(b) == 0 || !utf8.Valid(b) {
		return "", false
	}
	s := string(b)
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		if !unicode.IsPrint(r) {
			return "", false
		}
	}
	if strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}
