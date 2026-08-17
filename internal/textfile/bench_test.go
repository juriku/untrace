package textfile

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

var corpusText = repeatTo("the quick brown fox jumps over the lazy dog. ", corpusBytes)

// Built through Encode so the input to Decode is exactly what this package
// produces, rather than a hand-assembled approximation of it.
func encodedAs(tb testing.TB, enc Encoding, bom bool) []byte {
	tb.Helper()
	data, err := Encode(corpusText, Decoded{Text: corpusText, Enc: enc, HadBOM: bom})
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

func benchDecode(b *testing.B, data []byte) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(data); err != nil {
			b.Fatal(err)
		}
	}
}

func benchEncode(b *testing.B, dec Decoded) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(dec.Text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(dec.Text, dec); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeUTF8(b *testing.B) {
	benchDecode(b, encodedAs(b, UTF8, false))
}

func BenchmarkDecodeUTF8WithBOM(b *testing.B) {
	benchDecode(b, encodedAs(b, UTF8, true))
}

func BenchmarkDecodeUTF16LE(b *testing.B) {
	benchDecode(b, encodedAs(b, UTF16LE, true))
}

func BenchmarkDecodeLatin1(b *testing.B) {
	benchDecode(b, encodedAs(b, Latin1, false))
}

func BenchmarkEncodeUTF8(b *testing.B) {
	benchEncode(b, Decoded{Text: corpusText, Enc: UTF8})
}

func BenchmarkEncodeUTF16LE(b *testing.B) {
	benchEncode(b, Decoded{Text: corpusText, Enc: UTF16LE, HadBOM: true})
}

func BenchmarkEncodeLatin1(b *testing.B) {
	benchEncode(b, Decoded{Text: corpusText, Enc: Latin1})
}

// The sniff that decides whether a file is scanned at all, so it runs against
// every file in a tree before anything else does.
func BenchmarkIsBinaryText(b *testing.B) {
	data := []byte(corpusText)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsBinary(data)
	}
}

func BenchmarkIsBinaryBinary(b *testing.B) {
	data := []byte(repeatTo("\x00\x01\x02\x03binary\xff", corpusBytes))
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsBinary(data)
	}
}
