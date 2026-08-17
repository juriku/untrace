package textfile

import (
	"bytes"
	"testing"
)

func FuzzDecodeEncode(f *testing.F) {
	f.Add([]byte("hello world\n"))
	f.Add([]byte("caf\xe9 latin-1\n"))
	f.Add(append(append([]byte{}, bomUTF8...), []byte("with bom\n")...))
	f.Add(append(append([]byte{}, bomUTF16LE...), 'h', 0, 'i', 0))
	f.Add(append(append([]byte{}, bomUTF16BE...), 0, 'h', 0, 'i'))
	f.Add(append(append([]byte{}, bomUTF32LE...), 'A', 0, 0, 0))
	f.Add([]byte{0xFF, 0xFE})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		dec, err := Decode(data)
		if err != nil {
			return
		}

		got, err := Encode(dec.Text, dec)
		if err != nil {
			return
		}

		// Cleaning rewrites whatever Decode produced, so a decode that cannot be
		// re-encoded to the original bytes means untrace would corrupt the file.
		if !bytes.Equal(got, data) {
			t.Fatalf("round trip changed bytes\nencoding %v\n got %q\nwant %q",
				dec.Enc, got, data)
		}
	})
}

func FuzzIsBinary(f *testing.F) {
	f.Add([]byte("plain text"))
	f.Add([]byte{0x00, 0x01, 0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		IsBinary(data)
	})
}
