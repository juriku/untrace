// Package media inspects and strips provenance metadata in image containers.
package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
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

// AIGCLabel marks a record carrying a TC260 generated-content declaration,
// required of Chinese services by GB 45438-2025.
const AIGCLabel = "AIGC"

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
	FormatWebP    Format = "webp"
	FormatTIFF    Format = "tiff"
	FormatGIF     Format = "gif"
	FormatUnknown Format = ""
)

type Report struct {
	Format   Format    `json:"format"`
	Findings []Finding `json:"findings,omitempty"`
}

var (
	pngSignature  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	xmpNamespace  = []byte("http://ns.adobe.com/xap/1.0/\x00")
	exifHeader    = []byte("Exif\x00\x00")
	riffSignature = []byte("RIFF")
	webpSignature = []byte("WEBP")
)

func Detect(data []byte) Format {
	switch {
	case bytes.HasPrefix(data, pngSignature):
		return FormatPNG
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return FormatJPEG
	case len(data) >= 12 && bytes.HasPrefix(data, riffSignature) &&
		bytes.Equal(data[8:12], webpSignature):
		return FormatWebP
	case isTIFF(data):
		return FormatTIFF
	case isGIF(data):
		return FormatGIF
	}
	return FormatUnknown
}

func Inspect(data []byte) Report {
	switch Detect(data) {
	case FormatPNG:
		return Report{Format: FormatPNG, Findings: inspectPNG(data)}
	case FormatJPEG:
		return Report{Format: FormatJPEG, Findings: inspectJPEG(data)}
	case FormatWebP:
		return Report{Format: FormatWebP, Findings: inspectWebP(data)}
	case FormatTIFF:
		return Report{Format: FormatTIFF, Findings: inspectTIFF(data)}
	case FormatGIF:
		return Report{Format: FormatGIF, Findings: inspectGIF(data)}
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
	case FormatWebP:
		return stripWebP(data, strip)
	case FormatTIFF:
		return stripTIFF(data, strip)
	case FormatGIF:
		return stripGIF(data, strip)
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
	overrideWithAIGC(&f, c.data)
	return f, true
}

func overrideWithAIGC(f *Finding, raw []byte) {
	producer, ok := provenance.AIGCLabel(raw)
	if !ok {
		return
	}
	f.Label = AIGCLabel
	if producer != "" {
		f.Value = producer
	}
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
		// SOS begins entropy-coded data and EOI ends the image; neither is
		// followed by a length field.
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
		if looksLikeManifest(s.data) {
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
	overrideWithAIGC(&f, s.data)
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
// contiguous.
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
			if name, ok := provenance.GeneratorInManifest(string(data[start:i])); ok {
				return name
			}
		}
		start = -1
	}
	return ""
}

// exifSoftware walks the TIFF header and first IFD for tag 0x0131.
func exifSoftware(data []byte) (string, bool) {
	v, _, _, ok := exifSoftwareSpan(data)
	return v, ok
}

// exifSoftwareSpan also reports where the value sits, so a fixer can blank it
// without moving anything: every other offset in the block is absolute.
func exifSoftwareSpan(data []byte) (value string, at, n int, ok bool) {
	if len(data) < 8 {
		return "", 0, 0, false
	}
	order, ok := tiffByteOrder(data)
	if !ok {
		return "", 0, 0, false
	}

	offset := int(order.Uint32(data[4:]))
	if offset+2 > len(data) {
		return "", 0, 0, false
	}
	count := int(order.Uint16(data[offset:]))
	entry := offset + 2

	for i := 0; i < count && entry+12 <= len(data); i, entry = i+1, entry+12 {
		tag := order.Uint16(data[entry:])
		if tag != 0x0131 {
			continue
		}
		size := int(order.Uint32(data[entry+4:]))
		valueOffset := int(order.Uint32(data[entry+8:]))
		if size <= 4 {
			valueOffset = entry + 8
		}
		if valueOffset < 0 || size < 0 || valueOffset+size > len(data) {
			return "", 0, 0, false
		}
		v := printableOrEmpty(strings.TrimRight(string(data[valueOffset:valueOffset+size]), "\x00"))
		return v, valueOffset, size, true
	}
	return "", 0, 0, false
}

func tiffByteOrder(data []byte) (binary.ByteOrder, bool) {
	switch {
	case len(data) < 8:
		return nil, false
	case data[0] == 'I' && data[1] == 'I':
		return binary.LittleEndian, true
	case data[0] == 'M' && data[1] == 'M':
		return binary.BigEndian, true
	}
	return nil, false
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

// --- WebP ---
//
// RIFF: a header, then chunks of FourCC, a four byte little-endian size, the
// payload, and a pad byte when that size is odd.

const riffHeaderBytes = 12

type riffChunk struct {
	fourCC string
	data   []byte
	start  int
	// Includes the pad byte, so removing data[start:end] leaves the rest aligned.
	end int
}

func riffChunks(data []byte) ([]riffChunk, error) {
	if len(data) < riffHeaderBytes {
		return nil, fmt.Errorf("not a riff file")
	}

	var out []riffChunk
	for i := riffHeaderBytes; i+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[i+4:]))
		if size < 0 || i+8+size > len(data) {
			return out, fmt.Errorf("chunk at %d overruns the file", i)
		}
		end := i + 8 + size
		if size%2 == 1 && end < len(data) {
			end++
		}
		out = append(out, riffChunk{
			fourCC: string(data[i : i+4]),
			data:   data[i+8 : i+8+size],
			start:  i,
			end:    end,
		})
		i = end
	}
	return out, nil
}

func webpChunkKind(c riffChunk) (Kind, bool) {
	switch c.fourCC {
	case "EXIF":
		return EXIF, true
	case "XMP ":
		return XMP, true
	}
	if looksLikeManifest(c.data) {
		return C2PA, true
	}
	return "", false
}

func webpFinding(c riffChunk) (Finding, bool) {
	kind, ok := webpChunkKind(c)
	if !ok {
		return Finding{}, false
	}
	f := Finding{Kind: kind, Bytes: len(c.data)}
	switch kind {
	case C2PA:
		f.Value = manifestGenerator(c.data)
	case EXIF:
		// A bare TIFF block, with no "Exif\0\0" ahead of it.
		if s, ok := exifSoftware(c.data); ok {
			f.Label, f.Value = "Software", s
		}
	case XMP:
		if v, ok := xmpCreatorTool(c.data); ok {
			f.Label, f.Value = "CreatorTool", v
		}
	}
	overrideWithAIGC(&f, c.data)
	return f, true
}

func inspectWebP(data []byte) []Finding {
	chunks, _ := riffChunks(data)
	var out []Finding
	for _, c := range chunks {
		if f, ok := webpFinding(c); ok {
			out = append(out, f)
		}
	}
	return out
}

func stripWebP(data []byte, strip func(Finding) bool) ([]byte, bool, error) {
	chunks, err := riffChunks(data)
	if err != nil && len(chunks) == 0 {
		return data, false, err
	}

	drop := make([]riffChunk, 0, len(chunks))
	for _, c := range chunks {
		if f, ok := webpFinding(c); ok && strip(f) {
			drop = append(drop, c)
		}
	}
	if len(drop) == 0 {
		return data, false, nil
	}

	out := make([]byte, 0, len(data))
	prev := 0
	for _, c := range drop {
		out = append(out, data[prev:c.start]...)
		prev = c.end
	}
	out = append(out, data[prev:]...)

	// The RIFF size counts everything after itself.
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))

	for _, c := range drop {
		clearVP8XFlag(out, c.fourCC)
	}
	return out, true, nil
}

// VP8X announces which optional chunks a file carries, in the flags byte at the
// start of its payload. A chunk removed without clearing its bit leaves a file
// libwebp reports as containing metadata it cannot find.
// https://developers.google.com/speed/webp/docs/riff_container
const (
	vp8xEXIFFlag = 0x08
	vp8xXMPFlag  = 0x04
)

func clearVP8XFlag(data []byte, fourCC string) {
	var flag byte
	switch fourCC {
	case "EXIF":
		flag = vp8xEXIFFlag
	case "XMP ":
		flag = vp8xXMPFlag
	default:
		return
	}

	chunks, _ := riffChunks(data)
	for _, c := range chunks {
		if c.fourCC == "VP8X" && len(c.data) > 0 {
			data[c.start+8] &^= flag
			return
		}
	}
}

// --- TIFF ---

func isTIFF(data []byte) bool {
	order, ok := tiffByteOrder(data)
	return ok && order.Uint16(data[2:]) == 42
}

// stripTIFF leaves a blanked tag in place, so an empty value means the record
// is already gone.
func inspectTIFF(data []byte) []Finding {
	value, _, _, ok := exifSoftwareSpan(data)
	if !ok || value == "" {
		return nil
	}

	f := Finding{Kind: EXIF, Label: "Software", Value: value, Bytes: len(data)}
	overrideWithAIGC(&f, data)
	return []Finding{f}
}

// Offsets in a TIFF are absolute, so the file has to keep its length.
func stripTIFF(data []byte, strip func(Finding) bool) ([]byte, bool, error) {
	findings := inspectTIFF(data)
	if len(findings) == 0 || !strip(findings[0]) {
		return data, false, nil
	}

	_, at, n, ok := exifSoftwareSpan(data)
	if !ok || n == 0 {
		return data, false, nil
	}

	out := append([]byte(nil), data...)
	for i := at; i < at+n; i++ {
		out[i] = 0
	}
	return out, true, nil
}

// --- SVG ---

const svgHeadBytes = 4096

func LooksLikeSVG(text string) bool {
	head := text
	if len(head) > svgHeadBytes {
		head = head[:svgHeadBytes]
	}
	return strings.Contains(head, "<svg")
}

func InspectSVG(text string) []Finding {
	var out []Finding
	for _, span := range svgMetadataSpans(text) {
		if f, ok := svgFinding(text[span[0]:span[1]]); ok {
			out = append(out, f)
		}
	}
	return out
}

func svgFinding(raw string) (Finding, bool) {
	f := Finding{Kind: XMP, Bytes: len(raw)}

	switch v, ok := xmpCreatorTool([]byte(raw)); {
	case ok:
		f.Label, f.Value = "CreatorTool", v
	default:
		if v, ok := svgElementText(raw, "dc:creator"); ok {
			f.Label, f.Value = "creator", v
		} else if looksLikeManifest([]byte(raw)) {
			if g := manifestGenerator([]byte(raw)); g != "" {
				f.Kind, f.Label, f.Value = C2PA, "claim_generator", g
			}
		}
	}
	overrideWithAIGC(&f, []byte(raw))

	if f.Value == "" && f.Label == "" {
		return Finding{}, false
	}
	return f, true
}

func StripSVG(text string, strip func(Finding) bool) (string, bool) {
	var b strings.Builder
	prev := 0
	changed := false

	for _, span := range svgMetadataSpans(text) {
		raw := text[span[0]:span[1]]

		f, ok := svgFinding(raw)
		if !ok {
			f = Finding{Kind: XMP, Bytes: len(raw)}
		}
		if !strip(f) {
			continue
		}
		b.WriteString(text[prev:span[0]])
		prev = span[1]
		changed = true
	}
	if !changed {
		return text, false
	}
	b.WriteString(text[prev:])
	return b.String(), true
}

func svgMetadataSpans(text string) [][2]int {
	var out [][2]int
	for _, name := range []string{"metadata", "x:xmpmeta"} {
		open, closing := "<"+name, "</"+name+">"
		from := 0
		for {
			i := strings.Index(text[from:], open)
			if i < 0 {
				break
			}
			i += from
			from = i + len(open)

			gt := strings.IndexByte(text[i:], '>')
			if gt < 0 {
				break
			}
			tag := text[i : i+gt+1]

			// <metadataFoo> is a different element, and <metadata/> has no
			// content: pairing it with a later close swallows everything between.
			if isNameByte(tag[len(open)]) || strings.HasSuffix(tag, "/>") {
				continue
			}

			start := i + gt + 1
			j := strings.Index(text[start:], closing)
			if j < 0 {
				break
			}
			out = append(out, [2]int{start, start + j})
			from = start + j + len(closing)
		}
	}
	return outermost(out)
}

func isNameByte(c byte) bool {
	return c == '-' || c == '_' || c == ':' || c == '.' ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// An XMP packet normally sits inside <metadata>, so both match the same bytes.
func outermost(spans [][2]int) [][2]int {
	if len(spans) == 0 {
		return nil
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i][0] != spans[j][0] {
			return spans[i][0] < spans[j][0]
		}
		return spans[i][1] > spans[j][1]
	})

	out := spans[:1]
	for _, s := range spans[1:] {
		if s[1] <= out[len(out)-1][1] {
			continue
		}
		out = append(out, s)
	}
	return out
}

func svgElementText(raw, tag string) (string, bool) {
	i := strings.Index(raw, "<"+tag+">")
	if i < 0 {
		return "", false
	}
	rest := raw[i+len(tag)+2:]
	j := strings.Index(rest, "</"+tag+">")
	if j < 0 {
		return "", false
	}
	v := printableOrEmpty(rest[:j])
	return v, v != ""
}

// --- GIF ---
//
// An extension block is 0x21, a label byte, then sub-blocks each led by a one
// byte length and terminated by a zero length.

var gifSignatures = [][]byte{[]byte("GIF87a"), []byte("GIF89a")}

const (
	gifExtension      = 0x21
	gifApplicationExt = 0xFF
	gifCommentExt     = 0xFE
	gifImageSeparator = 0x2C
	gifTrailer        = 0x3B
)

func isGIF(data []byte) bool {
	for _, sig := range gifSignatures {
		if bytes.HasPrefix(data, sig) {
			return true
		}
	}
	return false
}

type gifBlock struct {
	label byte
	data  []byte
	start int
	end   int
}

// Extension blocks only: everything provenance-bearing precedes the image data.
func gifBlocks(data []byte) ([]gifBlock, error) {
	if !isGIF(data) {
		return nil, fmt.Errorf("not a gif")
	}
	pos := 13
	if len(data) < pos {
		return nil, fmt.Errorf("truncated gif header")
	}
	// Bit 7 of the packed field marks a global colour table, sized 3*2^(n+1).
	if data[10]&0x80 != 0 {
		pos += 3 * (1 << ((data[10] & 0x07) + 1))
	}

	var out []gifBlock
	for pos+1 < len(data) {
		switch data[pos] {
		case gifTrailer, gifImageSeparator:
			return out, nil
		case gifExtension:
			label := data[pos+1]
			end, ok := gifSubBlockEnd(data, pos+2)
			if !ok {
				return out, fmt.Errorf("truncated gif extension at %d", pos)
			}
			out = append(out, gifBlock{label: label, data: data[pos+2 : end], start: pos, end: end})
			pos = end
		default:
			return out, nil
		}
	}
	return out, nil
}

func gifSubBlockEnd(data []byte, pos int) (int, bool) {
	for pos < len(data) {
		n := int(data[pos])
		if n == 0 {
			return pos + 1, true
		}
		pos += 1 + n
	}
	return 0, false
}

func gifSubBlockData(raw []byte) []byte {
	var out []byte
	for pos := 0; pos < len(raw); {
		n := int(raw[pos])
		if n == 0 {
			break
		}
		if pos+1+n > len(raw) {
			break
		}
		out = append(out, raw[pos+1:pos+1+n]...)
		pos += 1 + n
	}
	return out
}

func gifFinding(b gifBlock) (Finding, bool) {
	payload := gifSubBlockData(b.data)

	switch b.label {
	case gifApplicationExt:
		f := Finding{Kind: XMP, Bytes: len(payload)}
		if v, ok := xmpCreatorTool(payload); ok {
			f.Label, f.Value = "CreatorTool", v
		}
		overrideWithAIGC(&f, payload)
		if f.Value == "" {
			return Finding{}, false
		}
		return f, true
	case gifCommentExt:
		v := printableOrEmpty(string(payload))
		if v == "" {
			return Finding{}, false
		}
		return Finding{Kind: JPEGComment, Value: v, Bytes: len(payload)}, true
	}
	return Finding{}, false
}

func inspectGIF(data []byte) []Finding {
	blocks, _ := gifBlocks(data)
	var out []Finding
	for _, b := range blocks {
		if f, ok := gifFinding(b); ok {
			out = append(out, f)
		}
	}
	return out
}

func stripGIF(data []byte, strip func(Finding) bool) ([]byte, bool, error) {
	blocks, err := gifBlocks(data)
	if err != nil && len(blocks) == 0 {
		return data, false, err
	}

	drop := make([]gifBlock, 0, len(blocks))
	for _, b := range blocks {
		if f, ok := gifFinding(b); ok && strip(f) {
			drop = append(drop, b)
		}
	}
	if len(drop) == 0 {
		return data, false, nil
	}

	out := make([]byte, 0, len(data))
	prev := 0
	for _, b := range drop {
		out = append(out, data[prev:b.start]...)
		prev = b.end
	}
	return append(out, data[prev:]...), true, nil
}

// The JUMBF box a C2PA manifest rides in.
func looksLikeManifest(data []byte) bool {
	return bytes.Contains(data, []byte("c2pa")) || bytes.Contains(data, []byte("jumb"))
}
