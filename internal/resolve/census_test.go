package resolve

import (
	"strings"
	"testing"
)

func TestNeedsCensusOnlyForScriptPunctuation(t *testing.T) {
	for r := range scriptPunctuation {
		if !NeedsCensus(r) {
			t.Errorf("U+%04X has a script rule but does not ask for the census", r)
		}
	}
	for _, r := range []rune{'a', ' ', 0x200D, 0x2014, 0x3005} {
		if NeedsCensus(r) {
			t.Errorf("U+%04X asks for the census with no script rule", r)
		}
	}
}

func TestCensusCountsLettersPerGroup(t *testing.T) {
	c := NewCensus([]rune("abc" + "日本" + "مرحبا" + "κόσμος" + " ,.!"))

	if c.total != 3+2+5+6 {
		t.Errorf("total = %d, want every letter", c.total)
	}
	if c.cjk != 2 || c.arabic != 5 || c.greek != 6 {
		t.Errorf("cjk/arabic/greek = %d/%d/%d, want 2/5/6", c.cjk, c.arabic, c.greek)
	}
}

func TestNativeNeedsAShareOfTheDocument(t *testing.T) {
	sparse := NewCensus([]rune(strings.Repeat("a", 90) + "日"))
	if sparse.native(groupCJK) {
		t.Error("a single ideograph made the whole document Japanese")
	}

	// Nine of ninety is exactly nativeShare, and the boundary counts as native.
	atLine := NewCensus([]rune(strings.Repeat("a", 81) + strings.Repeat("日", 9)))
	if !atLine.native(groupCJK) {
		t.Error("a group reaching the threshold was not native")
	}

	if NewCensus([]rune("abc")).native(groupCJK) {
		t.Error("a group with no letters was native")
	}
	if NewCensus([]rune("日本語")).native(groupNone) {
		t.Error("groupNone was native")
	}
}
