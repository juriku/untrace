package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func benchPNG(tb testing.TB, w, h int) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

func benchJPEG(tb testing.TB, w, h int) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

// Ancillary chunks legally live immediately before IEND.
func addPNGChunk(tb testing.TB, data []byte, typ string, payload []byte) []byte {
	tb.Helper()
	iend := bytes.LastIndex(data, []byte("IEND"))
	if iend < 4 {
		tb.Fatal("no IEND in fixture")
	}
	at := iend - 4

	chunk := make([]byte, 0, len(payload)+12)
	chunk = binary.BigEndian.AppendUint32(chunk, uint32(len(payload)))
	chunk = append(chunk, typ...)
	chunk = append(chunk, payload...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(append([]byte(typ), payload...)))

	out := make([]byte, 0, len(data)+len(chunk))
	out = append(out, data[:at]...)
	out = append(out, chunk...)
	return append(out, data[at:]...)
}

// A JPEG segment length field is 16 bits, so a payload cannot exceed 65533
// bytes. A real C2PA manifest larger than that arrives split across segments.
func addJPEGSegment(tb testing.TB, data []byte, marker byte, payload []byte) []byte {
	tb.Helper()
	if len(payload)+2 > 0xFFFF {
		tb.Fatalf("payload %d exceeds the segment length field", len(payload))
	}
	seg := []byte{0xFF, marker}
	seg = binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2))
	seg = append(seg, payload...)

	out := make([]byte, 0, len(data)+len(seg))
	out = append(out, data[:2]...)
	out = append(out, seg...)
	return append(out, data[2:]...)
}

func benchInspect(b *testing.B, data []byte) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Inspect(data)
	}
}

func BenchmarkInspectCleanPNG(b *testing.B) {
	benchInspect(b, benchPNG(b, 256, 256))
}

func BenchmarkInspectCleanJPEG(b *testing.B) {
	benchInspect(b, benchJPEG(b, 256, 256))
}

func BenchmarkInspectPNGWithC2PA(b *testing.B) {
	data := addPNGChunk(b, benchPNG(b, 256, 256), "caBX", bytes.Repeat([]byte("c2pa"), 4096))
	benchInspect(b, data)
}

func BenchmarkInspectJPEGWithC2PA(b *testing.B) {
	data := addJPEGSegment(b, benchJPEG(b, 256, 256), 0xEB, bytes.Repeat([]byte("jumb"), 4096))
	benchInspect(b, data)
}

func BenchmarkInspectPNGManyTextChunks(b *testing.B) {
	data := benchPNG(b, 64, 64)
	for i := 0; i < 50; i++ {
		data = addPNGChunk(b, data, "tEXt", []byte("Software\x00Some Tool"))
	}
	benchInspect(b, data)
}

func BenchmarkStripPNG(b *testing.B) {
	data := addPNGChunk(b, benchPNG(b, 256, 256), "caBX", bytes.Repeat([]byte("c2pa"), 4096))
	stripC2PA := func(f Finding) bool { return f.Kind == C2PA }
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := Strip(data, stripC2PA); err != nil {
			b.Fatal(err)
		}
	}
}

// Runs against every file a scan opens, before any parsing happens.
func BenchmarkDetectFormat(b *testing.B) {
	data := benchPNG(b, 64, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Detect(data)
	}
}
