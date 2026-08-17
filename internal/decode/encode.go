package decode

import "strings"

// The encoders are the inverse of the decoders in this package. They exist so
// tests can build fixtures from real code rather than reimplementing the
// schemes, and so a decode can be verified by re-encoding it.

func EncodeTag(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7E {
			continue
		}
		b.WriteRune(rune(0xE0000 + int(c)))
	}
	return b.String()
}

func EncodeVariation(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		if c < 16 {
			b.WriteRune(rune(0xFE00 + int(c)))
		} else {
			b.WriteRune(rune(0xE0100 + int(c) - 16))
		}
	}
	return b.String()
}

func EncodeZeroWidth(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		for i := 7; i >= 0; i-- {
			if c>>uint(i)&1 == 1 {
				b.WriteRune(0x200C)
			} else {
				b.WriteRune(0x200B)
			}
		}
	}
	return b.String()
}
