// Package script implements mixed-script detection along the lines of
// UTS #39 section 5, Confusable Detection.
//
// Flagging every Cyrillic character is wrong: it makes the tool unusable on
// Russian text. What matters is a word that mixes scripts, because "раypal"
// mixing Cyrillic and Latin is an attack while a wholly Cyrillic word is prose.
package script

import (
	"sort"
	"unicode"
)

// Checked first for speed; the full table is consulted only on a miss.
var common = []string{
	"Latin", "Cyrillic", "Greek", "Han", "Hiragana", "Katakana", "Hangul",
	"Arabic", "Hebrew", "Devanagari", "Thai", "Armenian", "Georgian",
}

// Script combinations that occur in ordinary text: Japanese mixes Han with the
// kana, Korean mixes Han with Hangul, Chinese mixes Han with Bopomofo. Latin is
// in each of them because product names and initialisms are written in Latin
// inside CJK prose without a space to break the word.
var augmented = [][]string{
	{"Han", "Hiragana", "Katakana", "Latin"},
	{"Han", "Hangul", "Latin"},
	{"Han", "Bopomofo", "Latin"},
}

type Word struct {
	Text  string
	Start int
	End   int
	// Scripts present, excluding Common and Inherited.
	Scripts []string
	// Minority holds rune indices of characters in the least-common script,
	// which are the ones likely to have been substituted.
	Minority []int
}

type scriptTable struct {
	name  string
	table *unicode.RangeTable
}

var (
	commonTables = buildCommonTables()
	otherTables  = buildOtherTables()
)

func buildCommonTables() []scriptTable {
	out := make([]scriptTable, 0, len(common))
	for _, name := range common {
		if t, ok := unicode.Scripts[name]; ok {
			out = append(out, scriptTable{name: name, table: t})
		}
	}
	return out
}

// Sorted, so a rune resolves to the same script on every run regardless of map
// iteration order.
func buildOtherTables() []scriptTable {
	skip := make(map[string]bool, len(common)+2)
	skip["Common"] = true
	skip["Inherited"] = true
	for _, name := range common {
		skip[name] = true
	}

	out := make([]scriptTable, 0, len(unicode.Scripts))
	for name, t := range unicode.Scripts {
		if skip[name] {
			continue
		}
		out = append(out, scriptTable{name: name, table: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// ASCII resolves without consulting a table at all. Digits and punctuation are
// script Common, which is excluded from both tables, so without this every
// digit walks the whole of unicode.Scripts to conclude nothing.
func Of(r rune) string {
	if r < 0x80 {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return "Latin"
		}
		return ""
	}
	for _, s := range commonTables {
		if unicode.Is(s.table, r) {
			return s.name
		}
	}
	for _, s := range otherTables {
		if unicode.Is(s.table, r) {
			return s.name
		}
	}
	return ""
}

// Hyphens, underscores and apostrophes bound a token: "IT-специалист" is two
// single-script words, and joining them reports Latin beside Cyrillic as mixed.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

// MixedWords returns the words that draw on more than one script.
func MixedWords(runes []rune) []Word {
	var out []Word
	for i := 0; i < len(runes); {
		if !isWordRune(runes[i]) {
			i++
			continue
		}
		start := i
		for i < len(runes) && isWordRune(runes[i]) {
			i++
		}
		if w, ok := classify(runes, start, i); ok {
			out = appendWord(out, w)
		}
	}
	return out
}

// append grows a large slice by about 1.25x, which copies roughly five times
// the final size. Doubling costs two.
func appendWord(dst []Word, w Word) []Word {
	if len(dst) == cap(dst) {
		n := cap(dst) * 2
		if n == 0 {
			n = 8
		}
		grown := make([]Word, len(dst), n)
		copy(grown, dst)
		dst = grown
	}
	return append(dst, w)
}

func mixesScripts(runes []rune, start, end int) bool {
	// A word of pure ASCII is Latin or nothing, so it cannot mix.
	ascii := true
	for i := start; i < end; i++ {
		if runes[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return false
	}

	first := ""
	for i := start; i < end; i++ {
		s := Of(runes[i])
		if s == "" {
			continue
		}
		if first == "" {
			first = s
			continue
		}
		if s != first {
			return true
		}
	}
	return false
}

func classify(runes []rune, start, end int) (Word, bool) {
	if !mixesScripts(runes, start, end) {
		return Word{}, false
	}

	var buf [8]scriptCount
	seen := buf[:0]
	for i := start; i < end; i++ {
		s := Of(runes[i])
		if s == "" {
			continue
		}
		seen = countScript(seen, s)
	}

	sortScripts(seen)

	names := make([]string, len(seen))
	for i, sc := range seen {
		names[i] = sc.name
	}
	if isAugmented(names) {
		return Word{}, false
	}

	// Sorted, so a tie breaks the same way every run.
	minority := seen[0]
	for _, sc := range seen {
		if sc.n < minority.n {
			minority = sc
		}
	}

	positions := make([]int, 0, minority.n)
	for i := start; i < end; i++ {
		if Of(runes[i]) == minority.name {
			positions = append(positions, i)
		}
	}

	return Word{
		Text:     string(runes[start:end]),
		Start:    start,
		End:      end,
		Scripts:  names,
		Minority: positions,
	}, true
}

type scriptCount struct {
	name string
	n    int
}

func countScript(seen []scriptCount, name string) []scriptCount {
	for i := range seen {
		if seen[i].name == name {
			seen[i].n++
			return seen
		}
	}
	return append(seen, scriptCount{name: name, n: 1})
}

func isAugmented(names []string) bool {
	for _, set := range augmented {
		if subset(names, set) {
			return true
		}
	}
	return false
}

func subset(names, of []string) bool {
	for _, n := range names {
		found := false
		for _, o := range of {
			if n == o {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sortScripts(s []scriptCount) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].name < s[j-1].name; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
