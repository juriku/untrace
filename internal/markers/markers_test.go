package markers

import "testing"

func TestOrdinaryPunctuationIsNeverDetected(t *testing.T) {
	opts := Options{Typographic: true, IVS: true}
	for _, r := range []rune{0x2026, 0x2022, 0x00B7} {
		if _, ok := Lookup(r, opts); ok {
			t.Errorf("U+%04X should never be reported", r)
		}
	}
}

func TestOtherTypographyStillDetected(t *testing.T) {
	opts := Options{Typographic: true}
	for _, r := range []rune{0x2014, 0x201C, 0x00A0} {
		if _, ok := Lookup(r, opts); !ok {
			t.Errorf("U+%04X should still be reported", r)
		}
	}
}
