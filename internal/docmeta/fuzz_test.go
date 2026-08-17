package docmeta

import (
	"archive/zip"
	"bytes"
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
