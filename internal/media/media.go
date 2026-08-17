// Package media inspects and strips provenance metadata in image containers.
//
// This is where AI marking actually lives in practice. Anthropic attaches a
// C2PA Content Credential to images Claude produces, and that manifest is a
// signed statement of origin rather than a guess drawn from typography.
package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/juriku/untrace/internal/provenance"
)

type Kind string

const (
	C2PA        Kind = "c2pa"
	XMP         Kind = "xmp"
	EXIF        Kind = "exif"
	PNGText     Kind = "png-text"
	JPEGComment Kind = "jpeg-comment"
)

type Finding struct {
	Kind Kind `json:"kind"`
	// Label names the record, such as a PNG keyword or an EXIF tag.
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
	Bytes int    `json:"bytes"`
}

type Format string

const (
	FormatPNG     Format = "png"
	FormatJPEG    Format = "jpeg"
	FormatUnknown Format = ""
)

type Report struct {
	Format   Format    `json:"format"`
	Findings []Finding `json:"findings,omitempty"`
}

var (
	pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	xmpNamespace = []byte("http://ns.adobe.com/xap/1.0/\x00")
	exifHeader   = []byte("Exif\x00\x00")
)

func Detect(data []byte) Format {
	switch {
	case bytes.HasPrefix(data, pngSignature):
		return FormatPNG
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return FormatJPEG
	}
	return FormatUnknown
}

func Inspect(data []byte) Report {
	switch Detect(data) {
	case FormatPNG:
		return Report{Format: FormatPNG, Findings: inspectPNG(data)}
	case FormatJPEG:
		return Report{Format: FormatJPEG, Findings: inspectJPEG(data)}
	}
	return Report{}
}

// Pixel data is untouched, so the image itself is unchanged.
func Strip(data []byte, strip func(Finding) bool) ([]byte, bool, error) {
	switch Detect(data) {
	case FormatPNG:
		return stripPNG(data, strip)
	case FormatJPEG:
		return stripJPEG(data, strip)
	}
	return data, false, nil
}

// --- PNG ---
//
// After the signature, a PNG is a sequence of chunks: a four byte big-endian
// length, a four byte type, that many bytes of data, and a four byte CRC.

type pngChunk struct {
	typ   string
	data  []byte
	start int
	end   int
}

func pngChunks(data []byte) ([]pngChunk, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, fmt.Errorf("not a png")
	}
	var out []pngChunk
	pos := len(pngSignature)
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos:]))
		if length < 0 || pos+12+length > len(data) {
			return out, fmt.Errorf("truncated png chunk at offset %d", pos)
		}
		typ := string(data[pos+4 : pos+8])
		out = append(out, pngChunk{
			typ:   typ,
			data:  data[pos+8 : pos+8+length],
			start: pos,
			end:   pos + 12 + length,
		})
		pos += 12 + length
		if typ == "IEND" {
			break
		}
	}
	return out, nil
}

func pngChunkKind(typ string) (Kind, bool) {
	switch typ {
	case "caBX":
		return C2PA, true
	case "tEXt", "iTXt", "zTXt":
		return PNGText, true
	case "eXIf":
		return EXIF, true
	}
	return "", false
}

func pngFinding(c pngChunk) (Finding, bool) {
	kind, ok := pngChunkKind(c.typ)
	if !ok {
		return Finding{}, false
	}
	f := Finding{Kind: kind, Bytes: len(c.data)}
	switch kind {
	case C2PA:
		f.Value = manifestGenerator(c.data)
	case PNGText:
		f.Label, f.Value = pngTextPair(c.typ, c.data)
	case EXIF:
		if s, ok := exifSoftware(c.data); ok {
			f.Label, f.Value = "Software", s
		}
	}
	return f, true
}

func inspectPNG(data []byte) []Finding {
	chunks, _ := pngChunks(data)
	var out []Finding
	for _, c := range chunks {
		if f, ok := pngFinding(c); ok {
			out = append(out, f)
		}
	}
	return out
}

// tEXt and zTXt are keyword\0value; iTXt adds compression and language fields
// between the two, which are skipped here since only the keyword is reported.
func pngTextPair(typ string, data []byte) (string, string) {
	i := bytes.IndexByte(data, 0)
	if i < 0 {
		return "", ""
	}
	keyword := string(data[:i])
	if typ != "tEXt" {
		return keyword, ""
	}
	return keyword, printableOrEmpty(string(data[i+1:]))
}

func stripPNG(data []byte, strip func(Finding) bool) ([]byte, bool, error) {
	chunks, err := pngChunks(data)
	if err != nil {
		return data, false, err
	}
	drop := make([]pngChunk, 0, len(chunks))
	for _, c := range chunks {
		if f, ok := pngFinding(c); ok && strip(f) {
			drop = append(drop, c)
		}
	}
	if len(drop) == 0 {
		return data, false, nil
	}

	// pngChunks stops at IEND, so the chunk list does not cover the whole file.
	out := make([]byte, 0, len(data))
	prev := 0
	for _, c := range drop {
		out = append(out, data[prev:c.start]...)
		prev = c.end
	}
	out = append(out, data[prev:]...)
	return out, true, nil
}

// --- JPEG ---
//
// After the SOI marker, a JPEG is a sequence of segments: 0xFF, a marker byte,
// then for most markers a two byte big-endian length that includes itself.

type jpegSegment struct {
	marker byte
	data   []byte
	start  int
	end    int
}

func jpegSegments(data []byte) ([]jpegSegment, error) {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, fmt.Errorf("not a jpeg")
	}
	var out []jpegSegment
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			break
		}
		marker := data[pos+1]
		// Start of scan: entropy-coded data follows, so stop parsing segments.
		if marker == 0xDA || marker == 0xD9 {
			break
		}
		length := int(binary.BigEndian.Uint16(data[pos+2:]))
		if length < 2 || pos+2+length > len(data) {
			return out, fmt.Errorf("truncated jpeg segment at offset %d", pos)
		}
		out = append(out, jpegSegment{
			marker: marker,
			data:   data[pos+4 : pos+2+length],
			start:  pos,
			end:    pos + 2 + length,
		})
		pos += 2 + length
	}
	return out, nil
}

func jpegSegmentKind(s jpegSegment) (Kind, bool) {
	switch {
	case s.marker == 0xEB: // APP11 carries the JUMBF box holding a C2PA manifest
		if bytes.Contains(s.data, []byte("c2pa")) || bytes.Contains(s.data, []byte("jumb")) {
			return C2PA, true
		}
	case s.marker == 0xE1:
		if bytes.HasPrefix(s.data, xmpNamespace) {
			return XMP, true
		}
		if bytes.HasPrefix(s.data, exifHeader) {
			return EXIF, true
		}
	case s.marker == 0xFE:
		return JPEGComment, true
	}
	return "", false
}

func jpegFinding(s jpegSegment) (Finding, bool) {
	kind, ok := jpegSegmentKind(s)
	if !ok {
		return Finding{}, false
	}
	f := Finding{Kind: kind, Bytes: len(s.data)}
	switch kind {
	case C2PA:
		f.Value = manifestGenerator(s.data)
	case JPEGComment:
		f.Value = printableOrEmpty(string(s.data))
	case EXIF:
		if v, ok := exifSoftware(s.data[len(exifHeader):]); ok {
			f.Label, f.Value = "Software", v
		}
	case XMP:
		if v, ok := xmpCreatorTool(s.data); ok {
			f.Label, f.Value = "CreatorTool", v
		}
	}
	return f, true
}

func inspectJPEG(data []byte) []Finding {
	segments, _ := jpegSegments(data)
	var out []Finding
	for _, s := range segments {
		if f, ok := jpegFinding(s); ok {
			out = append(out, f)
		}
	}
	return out
}

func stripJPEG(data []byte, strip func(Finding) bool) ([]byte, bool, error) {
	segments, err := jpegSegments(data)
	if err != nil {
		return data, false, err
	}

	drop := make([]jpegSegment, 0, len(segments))
	for _, s := range segments {
		if f, ok := jpegFinding(s); ok && strip(f) {
			drop = append(drop, s)
		}
	}
	if len(drop) == 0 {
		return data, false, nil
	}

	out := make([]byte, 0, len(data))
	prev := 0
	for _, s := range drop {
		out = append(out, data[prev:s.start]...)
		prev = s.end
	}
	out = append(out, data[prev:]...)
	return out, true, nil
}

// --- shared ---

// claim_generator is a CBOR text string, and C2PA claims use deterministic
// encoding (RFC 8949 4.2.1), which forbids chunked strings, so the name is
// contiguous. Runs rather than raw bytes: Generator discards non-alphanumerics
// before comparing, so scattered bytes would splice into a name.
func manifestGenerator(data []byte) string {
	const minRun = 4

	start := -1
	for i := 0; i <= len(data); i++ {
		if i < len(data) && data[i] >= 0x20 && data[i] < 0x7F {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 && i-start >= minRun {
			if name, ok := provenance.Generator(string(data[start:i])); ok {
				return name
			}
		}
		start = -1
	}
	return ""
}

// exifSoftware walks the TIFF header and first IFD for tag 0x0131.
func exifSoftware(data []byte) (string, bool) {
	if len(data) < 8 {
		return "", false
	}
	var order binary.ByteOrder
	switch {
	case data[0] == 'I' && data[1] == 'I':
		order = binary.LittleEndian
	case data[0] == 'M' && data[1] == 'M':
		order = binary.BigEndian
	default:
		return "", false
	}

	offset := int(order.Uint32(data[4:]))
	if offset+2 > len(data) {
		return "", false
	}
	count := int(order.Uint16(data[offset:]))
	entry := offset + 2

	for i := 0; i < count && entry+12 <= len(data); i, entry = i+1, entry+12 {
		tag := order.Uint16(data[entry:])
		if tag != 0x0131 {
			continue
		}
		n := int(order.Uint32(data[entry+4:]))
		valueOffset := int(order.Uint32(data[entry+8:]))
		if n <= 4 {
			valueOffset = entry + 8
		}
		if valueOffset < 0 || valueOffset+n > len(data) {
			return "", false
		}
		return printableOrEmpty(strings.TrimRight(string(data[valueOffset:valueOffset+n]), "\x00")), true
	}
	return "", false
}

func xmpCreatorTool(data []byte) (string, bool) {
	const tag = "xmp:CreatorTool>"
	i := bytes.Index(data, []byte(tag))
	if i < 0 {
		return "", false
	}
	rest := data[i+len(tag):]
	j := bytes.IndexByte(rest, '<')
	if j < 0 {
		return "", false
	}
	return printableOrEmpty(string(rest[:j])), true
}

func printableOrEmpty(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' {
			return ""
		}
	}
	return s
}
