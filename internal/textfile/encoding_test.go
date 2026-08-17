package textfile

import (
	"bytes"
	"testing"
	"unicode/utf16"
)

func utf16Bytes(t *testing.T, s string, bigEndian, bom bool) []byte {
	t.Helper()
	var out []byte
	if bom {
		if bigEndian {
			out = append(out, bomUTF16BE...)
		} else {
			out = append(out, bomUTF16LE...)
		}
	}
	for _, u := range utf16.Encode([]rune(s)) {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	return out
}

func TestRoundTripPreservesBytes(t *testing.T) {
	// Built from a codepoint: a literal em dash in source is exactly what this
	// tool flags elsewhere.
	wide := "hello " + string(rune(0x2014)) + " world\n"

	cases := []struct {
		name string
		data []byte
		want Encoding
	}{
		{"ascii", []byte("hello world\n"), UTF8},
		{"utf8 multibyte", []byte("caf\xc3\xa9 \xe2\x80\x94 dash\n"), UTF8},
		{"utf8 with bom", append(append([]byte{}, bomUTF8...), []byte("hello\n")...), UTF8},
		{"latin-1", []byte("caf\xe9 has\xa0nbsp\n"), Latin1},
		{"utf16le with bom", utf16Bytes(t, wide, false, true), UTF16LE},
		{"utf16be with bom", utf16Bytes(t, wide, true, true), UTF16BE},
		{"empty", []byte{}, UTF8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dec, err := Decode(tc.data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if dec.Enc != tc.want {
				t.Errorf("encoding = %v, want %v", dec.Enc, tc.want)
			}
			got, err := Encode(dec.Text, dec)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if !bytes.Equal(got, tc.data) {
				t.Errorf("round trip changed bytes:\n got %q\nwant %q", got, tc.data)
			}
		})
	}
}

func TestUTF32BOMWinsOverUTF16(t *testing.T) {
	// The UTF-16LE BOM is a prefix of the UTF-32LE BOM, so order matters.
	data := append(append([]byte{}, bomUTF32LE...), 'A', 0, 0, 0)
	dec, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if dec.Enc != UTF32LE {
		t.Fatalf("encoding = %v, want utf-32le", dec.Enc)
	}
	if dec.Text != "A" {
		t.Errorf("text = %q, want %q", dec.Text, "A")
	}
}

func TestLatin1RoundTripsEveryByte(t *testing.T) {
	data := make([]byte, 0, 256)
	for i := 1; i < 256; i++ {
		data = append(data, byte(i))
	}
	dec, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := Encode(dec.Text, dec)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("latin-1 round trip lost bytes")
	}
}

func TestIsBinary(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"plain text", []byte("hello world"), false},
		{"utf8 multibyte", []byte("caf\xc3\xa9"), false},
		{"null byte", []byte("hello\x00world"), true},
		{"utf16le is text", utf16Bytes(t, "hello", false, true), false},
		{"utf16be is text", utf16Bytes(t, "hello", true, true), false},
		{"latin-1 prose", []byte("caf\xe9 and plenty of ascii padding here"), false},
		{"mostly non printable", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 'a'}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBinary(tc.data); got != tc.want {
				t.Errorf("IsBinary = %v, want %v", got, tc.want)
			}
		})
	}
}
