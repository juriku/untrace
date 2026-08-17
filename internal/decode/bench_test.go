package decode

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
	corpusNoCarriers = []rune(repeatTo("the quick brown fox jumps over the lazy dog. ", corpusBytes))

	corpusTagDense = []rune(repeatTo(
		"text "+EncodeTag("tracked-by:acct-99213")+" ", corpusBytes))
	corpusVariationDense = []rune(repeatTo(
		"text "+EncodeVariation([]byte("tracked-by:acct-99213"))+" ", corpusBytes))
	corpusZeroWidthDense = []rune(repeatTo(
		"text "+EncodeZeroWidth([]byte("tracked-by:acct-99213"))+" ", corpusBytes))

	// Below minZeroWidthRun, so every run is scanned and then rejected. This is
	// the path a file of stray zero-width characters takes.
	corpusShortRuns = []rune(repeatTo("word"+strings.Repeat(string(rune(0x200B)), 3), corpusBytes))
)

func benchPayloads(b *testing.B, runes []rune) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(runes)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Payloads(runes)
	}
}

func BenchmarkPayloadsNoCarriers(b *testing.B) {
	benchPayloads(b, corpusNoCarriers)
}

func BenchmarkPayloadsTagDense(b *testing.B) {
	benchPayloads(b, corpusTagDense)
}

func BenchmarkPayloadsVariationDense(b *testing.B) {
	benchPayloads(b, corpusVariationDense)
}

func BenchmarkPayloadsZeroWidthDense(b *testing.B) {
	benchPayloads(b, corpusZeroWidthDense)
}

// Paired with BenchmarkPayloadsNoCarriers: runs that never qualify should cost
// the same as no runs at all.
func BenchmarkPayloadsShortRuns(b *testing.B) {
	benchPayloads(b, corpusShortRuns)
}

func BenchmarkEncodeTag(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodeTag("tracked-by:acct-99213")
	}
}

func BenchmarkEncodeZeroWidth(b *testing.B) {
	payload := []byte("tracked-by:acct-99213")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodeZeroWidth(payload)
	}
}
