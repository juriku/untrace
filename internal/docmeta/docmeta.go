// Package docmeta reads metadata and text out of document containers.
//
// Office files are ZIP archives and PDFs are binary, so without this they are
// skipped as binary and their Office typography policy never applies to
// anything.
package docmeta

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Format string

const (
	FormatOOXML   Format = "ooxml"
	FormatODF     Format = "odf"
	FormatPDF     Format = "pdf"
	FormatUnknown Format = ""
)

type Finding struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Document struct {
	Format   Format    `json:"format"`
	Metadata []Finding `json:"metadata,omitempty"`
	// Text is the extracted document body, empty when the format cannot be read.
	Text string `json:"-"`
}

func Detect(data []byte) Format {
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		return FormatPDF
	}
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return FormatUnknown
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return FormatUnknown
	}
	for _, f := range r.File {
		switch {
		case strings.HasPrefix(f.Name, "docProps/"), strings.HasPrefix(f.Name, "word/"),
			strings.HasPrefix(f.Name, "xl/"), strings.HasPrefix(f.Name, "ppt/"):
			return FormatOOXML
		case f.Name == "meta.xml", f.Name == "content.xml":
			return FormatODF
		}
	}
	return FormatUnknown
}

// MaybeContainer works on a prefix of the file. Detect needs the whole archive
// because a zip's central directory lives at the end, so a sniffed buffer can
// only ever say "worth reading in full".
func MaybeContainer(prefix []byte) bool {
	return bytes.HasPrefix(prefix, []byte("%PDF-")) ||
		bytes.HasPrefix(prefix, []byte("PK\x03\x04"))
}

func Read(data []byte) (Document, error) {
	switch Detect(data) {
	case FormatOOXML:
		return readZipDoc(data, FormatOOXML)
	case FormatODF:
		return readZipDoc(data, FormatODF)
	case FormatPDF:
		return readPDF(data), nil
	}
	return Document{}, nil
}

// Metadata parts and the body parts worth scanning, per container layout.
var (
	ooxmlMeta = []string{"docProps/core.xml", "docProps/app.xml"}
	ooxmlBody = []string{
		"word/document.xml", "word/footnotes.xml", "word/endnotes.xml",
		"xl/sharedStrings.xml", "ppt/slides/", "word/header", "word/footer",
	}
	odfMeta = []string{"meta.xml"}
	odfBody = []string{"content.xml"}
)

func readZipDoc(data []byte, format Format) (Document, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Document{}, err
	}

	metaParts, bodyParts := ooxmlMeta, ooxmlBody
	if format == FormatODF {
		metaParts, bodyParts = odfMeta, odfBody
	}

	doc := Document{Format: format}
	var body strings.Builder

	for _, f := range r.File {
		switch {
		case matchesAny(f.Name, metaParts):
			content, err := readEntry(f)
			if err != nil {
				continue
			}
			doc.Metadata = append(doc.Metadata, xmlLeafValues(content)...)
		case matchesAny(f.Name, bodyParts):
			content, err := readEntry(f)
			if err != nil {
				continue
			}
			body.WriteString(xmlText(content))
			body.WriteString("\n")
		}
	}

	doc.Text = body.String()
	return doc, nil
}

func matchesAny(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if name == p || strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// Entries are capped: a zip bomb should not be able to exhaust memory here.
const maxEntryBytes = 32 << 20

func readEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// The declared size comes from the archive header and is attacker-controlled;
	// clamping is what stops it reserving memory the archive never delivers.
	size := f.UncompressedSize64
	if size > maxEntryBytes {
		size = maxEntryBytes
	}
	buf := bytes.NewBuffer(make([]byte, 0, size))
	if _, err := io.Copy(buf, io.LimitReader(rc, maxEntryBytes)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// xmlLeafValues collects elements holding only text, which is the shape of the
// Dublin Core and extended-property records in a document's metadata parts.
func xmlLeafValues(content []byte) []Finding {
	var out []Finding
	dec := xml.NewDecoder(bytes.NewReader(content))
	var current string
	var buf strings.Builder

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			current = t.Name.Local
			buf.Reset()
		case xml.CharData:
			buf.Write(t)
		case xml.EndElement:
			if t.Name.Local == current {
				if v := strings.TrimSpace(buf.String()); v != "" {
					out = append(out, Finding{Label: current, Value: truncate(v)})
				}
			}
			current = ""
			buf.Reset()
		}
	}
	return out
}

func xmlText(content []byte) string {
	var sb strings.Builder
	sb.Grow(len(content))
	dec := xml.NewDecoder(bytes.NewReader(content))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if cd, ok := tok.(xml.CharData); ok {
			sb.Write(cd)
		}
	}
	return sb.String()
}

// PDF metadata lives in the trailer's /Info dictionary. Locating it properly
// means parsing the cross-reference table, and rewriting it means rebuilding
// that table, so untrace reports these values without offering to remove them.
var pdfInfoKeys = regexp.MustCompile(`/(Producer|Creator|Author|Title|Subject)\s*\(([^)]{0,200})\)`)

func readPDF(data []byte) Document {
	doc := Document{Format: FormatPDF}
	seen := map[string]bool{}

	for _, m := range pdfInfoKeys.FindAllSubmatch(data, -1) {
		label, value := string(m[1]), decodePDFString(m[2])
		if value == "" || seen[label+value] {
			continue
		}
		seen[label+value] = true
		doc.Metadata = append(doc.Metadata, Finding{Label: label, Value: truncate(value)})
	}

	if bytes.Contains(data, []byte("<x:xmpmeta")) {
		doc.Metadata = append(doc.Metadata, Finding{Label: "XMP", Value: "present"})
	}
	return doc
}

func decodePDFString(b []byte) string {
	s := strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`).Replace(string(b))
	for _, r := range s {
		if r < 0x20 && r != '\t' {
			return ""
		}
	}
	return strings.TrimSpace(s)
}

func truncate(s string) string {
	const limit = 120
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}

func (d Document) String() string {
	return fmt.Sprintf("%s: %d metadata record(s), %d bytes of text",
		d.Format, len(d.Metadata), len(d.Text))
}
