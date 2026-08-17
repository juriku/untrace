package script

import (
	"strings"
	"testing"
)

const corpusBytes = 1 << 20

func repeatTo(unit string, n int) string {
	count := n / len(unit)
	if count < 1 {
		count = 1
	}
	return strings.Repeat(unit, count)
}

var (
	cyrillicA = string(rune(0x0430))
	greekO    = string(rune(0x03BF))

	corpusPureLatin = []rune(repeatTo("the quick brown fox jumps over the lazy dog ", corpusBytes))

	// Cyrillic "привет", single-script and therefore not a finding.
	corpusPureCyrillic = []rune(repeatTo(
		string(rune(0x043F))+string(rune(0x0440))+string(rune(0x0438))+string(rune(0x0432))+
			string(rune(0x0435))+string(rune(0x0442))+" ", corpusBytes))

	corpusMixed = []rune(repeatTo("login at p"+cyrillicA+"ypal and g"+greekO+"ogle ", corpusBytes))

	// Paired with corpusOneLongWord: same bytes, opposite word counts.
	corpusManyShortWords = []rune(repeatTo("a b c d e f g ", corpusBytes))
	corpusOneLongWord    = []rune(repeatTo("abcdefghijklmnopqrstuvwxyz", corpusBytes))
)

func benchMixed(b *testing.B, runes []rune) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(runes)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MixedWords(runes)
	}
}

func BenchmarkMixedWordsPureLatin(b *testing.B) {
	benchMixed(b, corpusPureLatin)
}

func BenchmarkMixedWordsPureCyrillic(b *testing.B) {
	benchMixed(b, corpusPureCyrillic)
}

func BenchmarkMixedWordsAllMixed(b *testing.B) {
	benchMixed(b, corpusMixed)
}

func BenchmarkMixedWordsManyShortWords(b *testing.B) {
	benchMixed(b, corpusManyShortWords)
}

func BenchmarkMixedWordsOneLongWord(b *testing.B) {
	benchMixed(b, corpusOneLongWord)
}
