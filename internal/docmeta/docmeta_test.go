package docmeta

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func buildZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func docx(t *testing.T, body string) []byte {
	t.Helper()
	return buildZip(t, map[string]string{
		"[Content_Types].xml": `<Types/>`,
		"docProps/core.xml": `<cp:coreProperties xmlns:cp="c" xmlns:dc="d">` +
			`<dc:creator>A Person</dc:creator>` +
			`<cp:lastModifiedBy>Someone Else</cp:lastModifiedBy></cp:coreProperties>`,
		"docProps/app.xml":  `<Properties><Application>Microsoft Word</Application></Properties>`,
		"word/document.xml": `<w:document><w:body><w:p><w:r><w:t>` + body + `</w:t></w:r></w:p></w:body></w:document>`,
	})
}

func labelValue(d Document, label string) string {
	for _, f := range d.Metadata {
		if f.Label == label {
			return f.Value
		}
	}
	return ""
}

func TestDetect(t *testing.T) {
	cases := map[string]struct {
		data []byte
		want Format
	}{
		"docx":  {docx(t, "hello"), FormatOOXML},
		"odf":   {buildZip(t, map[string]string{"meta.xml": "<m/>", "content.xml": "<c/>"}), FormatODF},
		"pdf":   {[]byte("%PDF-1.7\n..."), FormatPDF},
		"plain": {[]byte("just text"), FormatUnknown},
		"zip":   {buildZip(t, map[string]string{"a.txt": "x"}), FormatUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Detect(tc.data); got != tc.want {
				t.Errorf("Detect = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadDocxMetadata(t *testing.T) {
	d, err := Read(docx(t, "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if got := labelValue(d, "creator"); got != "A Person" {
		t.Errorf("creator = %q", got)
	}
	if got := labelValue(d, "Application"); got != "Microsoft Word" {
		t.Errorf("Application = %q", got)
	}
}

func TestReadDocxExtractsBodyText(t *testing.T) {
	// A zero-width space inside the document body must survive extraction, since
	// finding it is the whole point.
	body := "two" + string(rune(0x200B)) + "words"
	d, err := Read(docx(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Text, body) {
		t.Errorf("extracted text %q does not contain the body", d.Text)
	}
}

func TestReadDocxDoesNotLeakMarkup(t *testing.T) {
	d, err := Read(docx(t, "plain words"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(d.Text, "w:t") || strings.Contains(d.Text, "<") {
		t.Errorf("extracted text contains markup: %q", d.Text)
	}
}

func TestReadODF(t *testing.T) {
	data := buildZip(t, map[string]string{
		"meta.xml":    `<meta><meta:generator>LibreOffice</meta:generator></meta>`,
		"content.xml": `<office><text:p>body text here</text:p></office>`,
	})
	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := labelValue(d, "generator"); got != "LibreOffice" {
		t.Errorf("generator = %q", got)
	}
	if !strings.Contains(d.Text, "body text here") {
		t.Errorf("text = %q", d.Text)
	}
}

func TestReadPDFInfoDictionary(t *testing.T) {
	data := []byte("%PDF-1.7\n1 0 obj\n<< /Producer (Some AI Tool 2.1) /Creator (Claude) >>\nendobj\n")
	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := labelValue(d, "Producer"); got != "Some AI Tool 2.1" {
		t.Errorf("Producer = %q", got)
	}
	if got := labelValue(d, "Creator"); got != "Claude" {
		t.Errorf("Creator = %q", got)
	}
}

func TestReadPDFDetectsXMP(t *testing.T) {
	data := []byte("%PDF-1.7\n<x:xmpmeta xmlns:x='adobe:ns:meta/'></x:xmpmeta>")
	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if labelValue(d, "XMP") != "present" {
		t.Errorf("XMP not reported: %+v", d.Metadata)
	}
}

func TestReadPDFDeduplicates(t *testing.T) {
	data := []byte("%PDF-1.7 /Producer (Tool) trailer /Producer (Tool)")
	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Metadata) != 1 {
		t.Errorf("got %d records, want 1: %+v", len(d.Metadata), d.Metadata)
	}
}

func TestReadPlainDataIsEmpty(t *testing.T) {
	d, err := Read([]byte("nothing to see"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Format != FormatUnknown || len(d.Metadata) != 0 {
		t.Errorf("got %+v", d)
	}
}

func TestCorruptZipDoesNotPanic(t *testing.T) {
	data := append([]byte("PK\x03\x04"), bytes.Repeat([]byte{0xFF}, 64)...)
	if got := Detect(data); got != FormatUnknown {
		t.Errorf("corrupt zip detected as %q", got)
	}
	if _, err := Read(data); err != nil {
		t.Logf("Read returned %v", err)
	}
}

func TestMaybeContainerWorksOnAPrefix(t *testing.T) {
	// A zip's central directory is at the end, so Detect cannot work on a sniff
	// buffer while MaybeContainer must.
	full := docx(t, "hello")
	prefix := full[:64]

	if Detect(prefix) != FormatUnknown {
		t.Error("Detect unexpectedly succeeded on a prefix")
	}
	if !MaybeContainer(prefix) {
		t.Error("MaybeContainer rejected a zip prefix")
	}
	if !MaybeContainer([]byte("%PDF-1.7 and then some")) {
		t.Error("MaybeContainer rejected a pdf prefix")
	}
	if MaybeContainer([]byte("plain text")) {
		t.Error("MaybeContainer accepted plain text")
	}
}

func TestLargeDocxIsStillReadable(t *testing.T) {
	// Bigger than the scanner's sniff buffer, which is what broke detection.
	// The padding must not compress away or the archive stays small.
	var sb strings.Builder
	seed := uint32(12345)
	for i := 0; i < 20000; i++ {
		seed = seed*1664525 + 1013904223
		sb.WriteByte(byte('a' + seed%26))
	}
	body := sb.String() + string(rune(0x200B))

	data := docx(t, body)
	if len(data) < 8192 {
		t.Fatalf("fixture is only %d bytes, too small to exercise the case", len(data))
	}
	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Text, string(rune(0x200B))) {
		t.Error("hidden character lost from a large document")
	}
}

func epub(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	all := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
	}
	for k, v := range entries {
		all[k] = v
	}
	return buildZip(t, all)
}

func TestDetectEPUB(t *testing.T) {
	data := epub(t, map[string]string{"OEBPS/content.opf": `<package/>`})

	if got := Detect(data); got != FormatEPUB {
		t.Errorf("Detect = %q, want %q", got, FormatEPUB)
	}
}

// The OPF sits wherever the container points, so the reader matches on
// extension rather than on a fixed path.
func TestReadEPUBFindsTheOPFAnywhere(t *testing.T) {
	for _, path := range []string{"content.opf", "OEBPS/content.opf", "deep/nested/book.opf"} {
		t.Run(path, func(t *testing.T) {
			data := epub(t, map[string]string{
				path: `<package><metadata><dc:creator>A Person</dc:creator></metadata></package>`,
			})

			d, err := Read(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Metadata) != 1 || d.Metadata[0].Value != "A Person" {
				t.Errorf("metadata = %+v", d.Metadata)
			}
		})
	}
}

func TestReadEPUBExtractsChapterText(t *testing.T) {
	data := epub(t, map[string]string{
		"OEBPS/content.opf": `<package/>`,
		"OEBPS/ch1.xhtml":   `<html><body><p>first` + string(rune(0x200B)) + `chapter</p></body></html>`,
		"OEBPS/ch2.html":    `<html><body><p>second chapter</p></body></html>`,
	})

	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"first", "chapter", "second chapter", string(rune(0x200B))} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("text is missing %q: %q", want, d.Text)
		}
	}
}

// EPUB 2 and HTML put the generator in an attribute, where element text never
// appears, so reading only leaf text misses the most common declaration.
func TestReadEPUBReadsAttributeMetadata(t *testing.T) {
	data := epub(t, map[string]string{
		"OEBPS/content.opf": `<package><metadata>` +
			`<meta name="generator" content="Midjourney v6"/>` +
			`<meta property="dcterms:modified">2026-08-17</meta>` +
			`</metadata></package>`,
	})

	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, m := range d.Metadata {
		if m.Label == "generator" && m.Value == "Midjourney v6" {
			found = true
		}
	}
	if !found {
		t.Errorf("attribute metadata not read: %+v", d.Metadata)
	}
}

func TestMetaAttrPairIgnoresIncompleteTags(t *testing.T) {
	data := epub(t, map[string]string{
		"OEBPS/content.opf": `<package><metadata>` +
			`<meta name="generator"/>` +
			`<meta content="orphan"/>` +
			`<notmeta name="x" content="y"/>` +
			`</metadata></package>`,
	})

	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Metadata) != 0 {
		t.Errorf("incomplete meta tags produced %+v", d.Metadata)
	}
}
