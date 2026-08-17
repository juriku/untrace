package script

import "testing"

func mixed(t *testing.T, s string) []Word {
	t.Helper()
	return MixedWords([]rune(s))
}

func TestPureLatinIsClean(t *testing.T) {
	if got := mixed(t, "ordinary english words here"); len(got) != 0 {
		t.Errorf("flagged pure Latin: %+v", got)
	}
}

func TestPureCyrillicIsClean(t *testing.T) {
	// Blanket homoglyph flagging would ruin Russian prose; this must stay quiet.
	if got := mixed(t, "Привет мир"); len(got) != 0 {
		t.Errorf("flagged pure Cyrillic prose: %+v", got)
	}
}

func TestCyrillicInsideLatinWordIsFlagged(t *testing.T) {
	// "paypal" with a Cyrillic а in place of the Latin one.
	word := "pаypal"
	got := mixed(t, "login at "+word+" now")
	if len(got) != 1 {
		t.Fatalf("got %d mixed words, want 1: %+v", len(got), got)
	}
	if got[0].Text != word {
		t.Errorf("Text = %q, want %q", got[0].Text, word)
	}
	if len(got[0].Minority) != 1 {
		t.Errorf("Minority = %v, want one character", got[0].Minority)
	}
	if len(got[0].Scripts) != 2 {
		t.Errorf("Scripts = %v, want two", got[0].Scripts)
	}
}

func TestGreekInsideLatinWordIsFlagged(t *testing.T) {
	// Greek omicron substituted into "google".
	if got := mixed(t, "gοοgle"); len(got) != 1 {
		t.Errorf("got %d mixed words, want 1", len(got))
	}
}

func TestJapaneseMixIsAllowed(t *testing.T) {
	// Han plus hiragana is ordinary Japanese, not an attack.
	if got := mixed(t, "日本語のテキスト"); len(got) != 0 {
		t.Errorf("flagged ordinary Japanese: %+v", got)
	}
}

func TestKoreanMixIsAllowed(t *testing.T) {
	if got := mixed(t, "韓国어"); len(got) != 0 {
		t.Errorf("flagged ordinary Korean: %+v", got)
	}
}

func TestDigitsAndPunctuationDoNotCount(t *testing.T) {
	// Digits are Common script, so they must not make a word look mixed.
	if got := mixed(t, "version2 build-3 x_9"); len(got) != 0 {
		t.Errorf("flagged words containing digits: %+v", got)
	}
}

func TestSeparateWordsInDifferentScriptsAreClean(t *testing.T) {
	// Mixing is only suspicious inside a word, not across a sentence.
	if got := mixed(t, "hello мир world"); len(got) != 0 {
		t.Errorf("flagged a bilingual sentence: %+v", got)
	}
}

// Equal counts must resolve to the alphabetically first script, which is what
// keeps Minority stable across runs.
func TestEqualCountsBreakAlphabetically(t *testing.T) {
	word := "ab" + string(rune(0x0430)) + string(rune(0x0431))
	got := mixed(t, word)
	if len(got) != 1 {
		t.Fatalf("got %d words, want 1", len(got))
	}
	if want := []string{"Cyrillic", "Latin"}; !equalStrings(got[0].Scripts, want) {
		t.Errorf("Scripts = %v, want %v", got[0].Scripts, want)
	}
	if want := []int{2, 3}; !equalInts(got[0].Minority, want) {
		t.Errorf("Minority = %v, want %v", got[0].Minority, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestOffsets(t *testing.T) {
	prefix := "see "
	got := mixed(t, prefix+"pаypal")
	if len(got) != 1 {
		t.Fatalf("got %d words, want 1", len(got))
	}
	if got[0].Start != len([]rune(prefix)) {
		t.Errorf("Start = %d, want %d", got[0].Start, len([]rune(prefix)))
	}
}
