package docmeta

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func benchZip(tb testing.TB, entries map[string]string) []byte {
	tb.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range entries {
		f, err := w.Create(name)
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			tb.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

func benchDocx(tb testing.TB, paragraphs int) []byte {
	tb.Helper()
	var body strings.Builder
	for i := 0; i < paragraphs; i++ {
		fmt.Fprintf(&body, `<w:p><w:r><w:t>paragraph %d of ordinary document text</w:t></w:r></w:p>`, i)
	}
	return benchZip(tb, map[string]string{
		"[Content_Types].xml": `<Types/>`,
		"docProps/core.xml": `<cp:coreProperties xmlns:cp="c" xmlns:dc="d">` +
			`<dc:creator>A Person</dc:creator>` +
			`<cp:lastModifiedBy>Someone Else</cp:lastModifiedBy></cp:coreProperties>`,
		"docProps/app.xml":  `<Properties><Application>Microsoft Word</Application></Properties>`,
		"word/document.xml": `<w:document><w:body>` + body.String() + `</w:body></w:document>`,
	})
}

func benchPDF(tb testing.TB, objects int) []byte {
	tb.Helper()
	var buf strings.Builder
	buf.WriteString("%PDF-1.7\n")
	for i := 0; i < objects; i++ {
		fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /Page /Contents %d 0 R >>\nendobj\n", i+1, i+2)
	}
	buf.WriteString("trailer\n<< /Info << /Producer (Some Tool 1.0) /Creator (Another Tool) >> >>\n%%EOF\n")
	return []byte(buf.String())
}

func benchRead(b *testing.B, data []byte) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Read(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadDocxSmall(b *testing.B) {
	benchRead(b, benchDocx(b, 10))
}

func BenchmarkReadDocxLarge(b *testing.B) {
	benchRead(b, benchDocx(b, 5000))
}

func BenchmarkReadPDFSmall(b *testing.B) {
	benchRead(b, benchPDF(b, 10))
}

func BenchmarkReadPDFLarge(b *testing.B) {
	benchRead(b, benchPDF(b, 5000))
}

func BenchmarkDetectPlainData(b *testing.B) {
	data := []byte(strings.Repeat("ordinary text that is not a container. ", 1000))
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Detect(data)
	}
}

func BenchmarkDetectDocx(b *testing.B) {
	data := benchDocx(b, 10)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Detect(data)
	}
}
