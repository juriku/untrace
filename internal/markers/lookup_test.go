package markers

import "testing"

func all() Options { return Options{Typographic: true, IVS: true} }

func TestLookupByKind(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		kind Kind
	}{
		{"zero width space", 0x200B, Hidden},
		{"non-breaking space", 0x00A0, Hidden},
		{"bidi override", 0x202E, Hidden},
		{"variation selector 16", 0xFE0F, Hidden},
		{"hangul filler", 0x3164, Hidden},
		{"braille blank", 0x2800, Hidden},
		{"em dash", 0x2014, Typographic},
		{"left curly quote", 0x201C, Typographic},
		{"cyrillic a", 0x0430, Typographic},
		{"ideographic vs", 0xE0101, IdeographicVS},
		{"tag letter", 0xE0041, Tag},
		{"cancel tag", 0xE007F, Tag},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, ok := Lookup(tc.r, all())
			if !ok {
				t.Fatalf("U+%04X not found", tc.r)
			}
			if m.Kind != tc.kind {
				t.Errorf("kind = %v, want %v", m.Kind, tc.kind)
			}
			if m.Name == "" {
				t.Error("marker has no name")
			}
		})
	}
}

func TestLookupMisses(t *testing.T) {
	for _, r := range []rune{'a', 'Z', '0', ' ', '\n', 0x4E2D} {
		if _, ok := Lookup(r, all()); ok {
			t.Errorf("U+%04X should not be a marker", r)
		}
	}
}

func TestOptionsGateTypographicAndIVS(t *testing.T) {
	if _, ok := Lookup(0x2014, Options{}); ok {
		t.Error("em dash reported with typographic off")
	}
	if _, ok := Lookup(0xE0101, Options{}); ok {
		t.Error("ivs reported with ivs off")
	}
	// Hidden markers and tags are not gated: they have no legitimate use.
	if _, ok := Lookup(0x200B, Options{}); !ok {
		t.Error("zero width space should always be reported")
	}
	if _, ok := Lookup(0xE0041, Options{}); !ok {
		t.Error("tag character should always be reported")
	}
}

func TestExcludedRunesAreSkipped(t *testing.T) {
	opts := all()
	opts.Excluded = map[rune]bool{0x2014: true}

	if _, ok := Lookup(0x2014, opts); ok {
		t.Error("excluded rune still reported")
	}
	if _, ok := Lookup(0x2013, opts); !ok {
		t.Error("exclusion leaked to another rune")
	}
}

func TestNeverDetected(t *testing.T) {
	for _, r := range []rune{0x2026, 0x2022, 0x00B7} {
		if !NeverDetected(r) {
			t.Errorf("U+%04X should be in the never-detected set", r)
		}
		if _, ok := Lookup(r, all()); ok {
			t.Errorf("U+%04X reported despite being never-detected", r)
		}
	}
	if NeverDetected(0x2014) {
		t.Error("em dash should not be never-detected")
	}
}

func TestHiddenWinsOverTypographic(t *testing.T) {
	// U+00A0 is in both tables; resolving it as typographic would let the Office
	// typographic exemption hide it everywhere.
	m, ok := Lookup(0x00A0, all())
	if !ok {
		t.Fatal("nbsp not found")
	}
	if m.Kind != Hidden {
		t.Errorf("kind = %v, want Hidden", m.Kind)
	}
}

func TestSpacesReplaceRatherThanDelete(t *testing.T) {
	for _, r := range []rune{0x00A0, 0x2009, 0x3000, 0x2800, 0x3164} {
		m, ok := Lookup(r, all())
		if !ok {
			t.Fatalf("U+%04X not found", r)
		}
		if m.Replacement != " " {
			t.Errorf("U+%04X replacement = %q, want a space", r, m.Replacement)
		}
	}
}

func TestZeroWidthCharactersAreDeleted(t *testing.T) {
	for _, r := range []rune{0x200B, 0x200C, 0x200D, 0xFEFF} {
		m, ok := Lookup(r, all())
		if !ok {
			t.Fatalf("U+%04X not found", r)
		}
		if m.Replacement != "" || !m.CanClean {
			t.Errorf("U+%04X replacement = %q, cleanable %v", r, m.Replacement, m.CanClean)
		}
	}
}

func TestReplacementCharacterIsReportedNotCleaned(t *testing.T) {
	m, ok := Lookup(0xFFFD, all())
	if !ok {
		t.Fatal("U+FFFD not found")
	}
	if m.CanClean {
		t.Error("U+FFFD is evidence of prior damage and must not be removed")
	}
}

func TestTagToASCII(t *testing.T) {
	cases := map[rune]byte{0xE0041: 'A', 0xE0061: 'a', 0xE0030: '0', 0xE0020: ' ', 0xE007E: '~'}
	for r, want := range cases {
		if got, ok := TagToASCII(r); !ok || got != want {
			t.Errorf("TagToASCII(U+%05X) = %q,%v want %q", r, got, ok, want)
		}
	}
	for _, r := range []rune{0xE0001, 0xE007F, 'A'} {
		if _, ok := TagToASCII(r); ok {
			t.Errorf("U+%05X should not map to ASCII", r)
		}
	}
}

func TestIsTagBounds(t *testing.T) {
	for _, r := range []rune{0xE0000, 0xE0041, 0xE007F} {
		if !IsTag(r) {
			t.Errorf("U+%05X should be a tag", r)
		}
	}
	for _, r := range []rune{0xDFFFF, 0xE0080, 'a'} {
		if IsTag(r) {
			t.Errorf("U+%05X should not be a tag", r)
		}
	}
}

func TestWordCommonRunesIsACopy(t *testing.T) {
	got := WordCommonRunes()
	if len(got) == 0 {
		t.Fatal("no Word characters")
	}
	got[0x1234] = true
	if IsWordCommon(0x1234) {
		t.Error("WordCommonRunes returned the live map")
	}
}

func TestKindNames(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range []Kind{Hidden, Typographic, IdeographicVS, Tag} {
		name := k.String()
		if name == "" || name == "unknown" {
			t.Errorf("kind %d has no name", k)
		}
		if seen[name] {
			t.Errorf("duplicate kind name %q", name)
		}
		seen[name] = true
	}
}
