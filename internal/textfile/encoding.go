package textfile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type Encoding int

const (
	UTF8 Encoding = iota
	UTF16LE
	UTF16BE
	UTF32LE
	UTF32BE
	Latin1
)

func (e Encoding) String() string {
	switch e {
	case UTF8:
		return "utf-8"
	case UTF16LE:
		return "utf-16le"
	case UTF16BE:
		return "utf-16be"
	case UTF32LE:
		return "utf-32le"
	case UTF32BE:
		return "utf-32be"
	case Latin1:
		return "latin-1"
	}
	return "unknown"
}

type Decoded struct {
	Text   string
	Enc    Encoding
	HadBOM bool
}

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
	bomUTF32LE = []byte{0xFF, 0xFE, 0x00, 0x00}
	bomUTF32BE = []byte{0x00, 0x00, 0xFE, 0xFF}
)

// sniffBOM must test UTF-32LE before UTF-16LE: the UTF-16LE BOM is a prefix of it.
func sniffBOM(data []byte) (Encoding, int, bool) {
	switch {
	case hasPrefix(data, bomUTF32LE):
		return UTF32LE, len(bomUTF32LE), true
	case hasPrefix(data, bomUTF32BE):
		return UTF32BE, len(bomUTF32BE), true
	case hasPrefix(data, bomUTF8):
		return UTF8, len(bomUTF8), true
	case hasPrefix(data, bomUTF16LE):
		return UTF16LE, len(bomUTF16LE), true
	case hasPrefix(data, bomUTF16BE):
		return UTF16BE, len(bomUTF16BE), true
	}
	return UTF8, 0, false
}

func hasPrefix(data, prefix []byte) bool {
	if len(data) < len(prefix) {
		return false
	}
	for i, b := range prefix {
		if data[i] != b {
			return false
		}
	}
	return true
}

// Encode inverts a Latin1 decode exactly, but only while the text is unchanged:
// cleaning maps 0xA0 and 0xAD onto their typographic meanings.
func Decode(data []byte) (Decoded, error) {
	enc, offset, hadBOM := sniffBOM(data)
	body := data[offset:]

	switch enc {
	case UTF16LE, UTF16BE:
		text, err := decodeUTF16(body, enc == UTF16BE)
		return Decoded{Text: text, Enc: enc, HadBOM: hadBOM}, err
	case UTF32LE, UTF32BE:
		text, err := decodeUTF32(body, enc == UTF32BE)
		return Decoded{Text: text, Enc: enc, HadBOM: hadBOM}, err
	}

	if utf8.Valid(body) {
		return Decoded{Text: string(body), Enc: UTF8, HadBOM: hadBOM}, nil
	}

	var sb strings.Builder
	sb.Grow(len(body))
	for _, b := range body {
		sb.WriteRune(rune(b))
	}
	// A UTF-8 BOM can precede non-UTF-8 bytes and still belongs to the file.
	return Decoded{Text: sb.String(), Enc: Latin1, HadBOM: hadBOM}, nil
}

func decodeUTF16(body []byte, bigEndian bool) (string, error) {
	if len(body)%2 != 0 {
		return "", errors.New("truncated utf-16: odd byte count")
	}
	units := make([]uint16, 0, len(body)/2)
	for i := 0; i < len(body); i += 2 {
		if bigEndian {
			units = append(units, binary.BigEndian.Uint16(body[i:]))
		} else {
			units = append(units, binary.LittleEndian.Uint16(body[i:]))
		}
	}
	if err := checkSurrogates(units); err != nil {
		return "", err
	}
	return string(utf16.Decode(units)), nil
}

// utf16.Decode would substitute U+FFFD, changing the bytes on re-encode.
func checkSurrogates(units []uint16) error {
	for i := 0; i < len(units); i++ {
		u := units[i]
		switch {
		case u >= 0xD800 && u <= 0xDBFF:
			if i+1 >= len(units) || units[i+1] < 0xDC00 || units[i+1] > 0xDFFF {
				return fmt.Errorf("invalid utf-16: unpaired high surrogate U+%04X", u)
			}
			i++
		case u >= 0xDC00 && u <= 0xDFFF:
			return fmt.Errorf("invalid utf-16: unpaired low surrogate U+%04X", u)
		}
	}
	return nil
}

func decodeUTF32(body []byte, bigEndian bool) (string, error) {
	if len(body)%4 != 0 {
		return "", errors.New("truncated utf-32: byte count not a multiple of four")
	}
	var sb strings.Builder
	for i := 0; i < len(body); i += 4 {
		var v uint32
		if bigEndian {
			v = binary.BigEndian.Uint32(body[i:])
		} else {
			v = binary.LittleEndian.Uint32(body[i:])
		}
		// A surrogate is not a scalar value; WriteRune would substitute U+FFFD.
		if v > utf8.MaxRune || (v >= 0xD800 && v <= 0xDFFF) {
			return "", fmt.Errorf("invalid utf-32 code point U+%X", v)
		}
		sb.WriteRune(rune(v))
	}
	return sb.String(), nil
}

func Encode(text string, d Decoded) ([]byte, error) {
	var out []byte

	switch d.Enc {
	case UTF16LE, UTF16BE:
		if d.HadBOM {
			out = append(out, bomFor(d.Enc)...)
		}
		for _, u := range utf16.Encode([]rune(text)) {
			if d.Enc == UTF16BE {
				out = binary.BigEndian.AppendUint16(out, u)
			} else {
				out = binary.LittleEndian.AppendUint16(out, u)
			}
		}
		return out, nil

	case UTF32LE, UTF32BE:
		if d.HadBOM {
			out = append(out, bomFor(d.Enc)...)
		}
		for _, r := range text {
			if d.Enc == UTF32BE {
				out = binary.BigEndian.AppendUint32(out, uint32(r))
			} else {
				out = binary.LittleEndian.AppendUint32(out, uint32(r))
			}
		}
		return out, nil

	case Latin1:
		if d.HadBOM {
			out = append(out, bomUTF8...)
		}
		for _, r := range text {
			if r > 0xFF {
				return nil, fmt.Errorf("cannot encode U+%04X as latin-1", r)
			}
			out = append(out, byte(r))
		}
		return out, nil
	}

	if d.HadBOM {
		out = append(out, bomUTF8...)
	}
	return append(out, text...), nil
}

func bomFor(e Encoding) []byte {
	switch e {
	case UTF16LE:
		return bomUTF16LE
	case UTF16BE:
		return bomUTF16BE
	case UTF32LE:
		return bomUTF32LE
	case UTF32BE:
		return bomUTF32BE
	}
	return bomUTF8
}

// IsBinary reports whether data looks like a non-text file. A UTF-16 or UTF-32
// BOM wins over the null-byte test, since those encodings are full of nulls.
func IsBinary(data []byte) bool {
	if enc, _, hadBOM := sniffBOM(data); hadBOM && enc != UTF8 {
		return false
	}
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}

	// Control bytes are valid UTF-8, so utf8.Valid alone would accept a binary
	// file. Text contains essentially none beyond whitespace and ESC.
	ctrl := 0
	for _, b := range data {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' && b != '\f' && b != 0x1B {
			ctrl++
		}
	}
	if float64(ctrl)/float64(len(data)) > 0.1 {
		return true
	}

	if utf8.Valid(data) {
		return false
	}

	printable := 0
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13 {
			printable++
		}
	}
	return float64(printable)/float64(len(data)) <= 0.75
}
