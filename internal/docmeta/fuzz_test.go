package docmeta

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func FuzzRead(f *testing.F) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	if entry, err := w.Create("docProps/core.xml"); err == nil {
		entry.Write([]byte(`<cp><dc:creator>A</dc:creator></cp>`))
	}
	if err := w.Close(); err == nil {
		f.Add(buf.Bytes())
	}

	f.Add([]byte("%PDF-1.7\n/Producer (Tool)\n"))
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte("plain text"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		Detect(data)
		if _, err := Read(data); err != nil {
			return
		}
	})
}

func FuzzFix(f *testing.F) {
	seeds := []map[string]string{
		{
			"[Content_Types].xml": `<Types/>`,
			"docProps/core.xml": `<cp:coreProperties xmlns:cp="c" xmlns:dc="d">` +
				`<dc:creator>Claude</dc:creator></cp:coreProperties>`,
			"word/document.xml": `<w:document><w:body><w:p><w:r><w:t>a` +
				string(rune(0x200B)) + `b</w:t></w:r></w:p></w:body></w:document>`,
		},
		{"content.xml": `<office:body><text:p>x</text:p></office:body>`, "meta.xml": `<m/>`},
		{"ppt/slides/slide1.xml": `<p:sld><a:t>x</a:t></p:sld>`, "[Content_Types].xml": `<Types/>`},
		{"xl/sharedStrings.xml": `<sst><si><t>x</t></si></sst>`, "[Content_Types].xml": `<Types/>`},
	}
	for _, s := range seeds {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		ok := true
		for name, body := range s {
			e, err := w.Create(name)
			if err != nil {
				ok = false
				break
			}
			e.Write([]byte(body))
		}
		if err := w.Close(); err == nil && ok {
			f.Add(buf.Bytes())
		}
	}

	f.Add([]byte("%PDF-1.7\n1 0 obj\n<< /Producer (Claude) >>\nendobj\n%%EOF\n"))
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		opt := FixOptions{
			Text:  func(s string) string { return strings.ReplaceAll(s, string(rune(0x200B)), "") },
			Strip: func(Finding) bool { return true },
		}

		out, changed, err := Fix(data, opt)
		if err != nil {
			return
		}
		if !changed {
			if !bytes.Equal(out, data) {
				t.Fatalf("unchanged output differs from input")
			}
			return
		}

		if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
			return
		}
		if _, err := zip.NewReader(bytes.NewReader(out), int64(len(out))); err != nil {
			t.Fatalf("rewrote a readable archive into an unreadable one: %v", err)
		}
	})
}
