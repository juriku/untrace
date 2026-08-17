package docmeta

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

// FixOptions decides what Fix rewrites. A nil field leaves that part alone.
type FixOptions struct {
	// Called on markup-free spans only, never on an attribute or element name.
	Text func(string) string
	// A selected value is emptied; the element itself is kept.
	Strip func(Finding) bool
}

// Fix rewrites text and metadata in place, leaving everything it does not
// select byte-identical.
func Fix(data []byte, opt FixOptions) ([]byte, bool, error) {
	switch format := Detect(data); format {
	case FormatPDF:
		out, changed := FixPDF(data, opt.Strip)
		return out, changed, nil
	case FormatOOXML, FormatODF:
		return fixArchive(data, format, opt)
	default:
		return data, false, nil
	}
}

func fixArchive(data []byte, format Format, opt FixOptions) ([]byte, bool, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return data, false, err
	}

	metaParts, bodyParts := ooxmlMeta, ooxmlBody
	if format == FormatODF {
		metaParts, bodyParts = odfMeta, odfBody
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	changed := false

	for _, f := range r.File {
		rewrite := rewriterFor(f.Name, metaParts, bodyParts, opt)
		if rewrite == nil {
			if err := copyRaw(w, f); err != nil {
				return data, false, err
			}
			continue
		}

		content, err := readEntry(f)
		if err != nil {
			return data, false, err
		}

		fixed, did := rewrite(content)
		if !did {
			if err := copyRaw(w, f); err != nil {
				return data, false, err
			}
			continue
		}
		changed = true

		header := f.FileHeader
		out, err := w.CreateHeader(&header)
		if err != nil {
			return data, false, err
		}
		if _, err := out.Write(fixed); err != nil {
			return data, false, err
		}
	}

	if err := w.Close(); err != nil {
		return data, false, err
	}
	if !changed {
		return data, false, nil
	}
	return buf.Bytes(), true, nil
}

func rewriterFor(name string, metaParts, bodyParts []string, opt FixOptions) func([]byte) ([]byte, bool) {
	if opt.Text != nil && matchesAny(name, bodyParts) {
		return func(content []byte) ([]byte, bool) {
			return fixCharData(content, opt.Text)
		}
	}
	if opt.Strip != nil && matchesAny(name, metaParts) {
		return func(content []byte) ([]byte, bool) {
			return blankProperties(content, opt.Strip)
		}
	}
	return nil
}

// Strip sees the same Label and Value the report showed.
func blankProperties(content []byte, strip func(Finding) bool) ([]byte, bool) {
	dec := xml.NewDecoder(bytes.NewReader(content))

	var out bytes.Buffer
	var element string
	copied := int64(0)
	changed := false

	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			break
		}
		end := dec.InputOffset()

		switch t := tok.(type) {
		case xml.StartElement:
			element = t.Name.Local
		case xml.EndElement:
			element = ""
		case xml.CharData:
			if element == "" || start >= end || end > int64(len(content)) {
				continue
			}
			value := strings.TrimSpace(string(t))
			if value == "" || !strip(Finding{Label: element, Value: value}) {
				continue
			}
			out.Write(content[copied:start])
			copied = end
			changed = true
		}
	}

	if !changed {
		return content, false
	}
	out.Write(content[copied:])
	return out.Bytes(), true
}

// copyRaw moves an entry across still compressed, so untouched parts of the
// archive come out bit-identical.
func copyRaw(w *zip.Writer, f *zip.File) error {
	rc, err := f.OpenRaw()
	if err != nil {
		return err
	}
	header := f.FileHeader
	out, err := w.CreateRaw(&header)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, rc)
	return err
}

// Spliced rather than re-encoded: encoding/xml normalises namespaces, attribute
// quoting and self-closing tags, which corrupts an Office file.
//
// A marker written as a numeric entity is not reached, since spans are handled
// as raw bytes.
func fixCharData(content []byte, replace func(string) string) ([]byte, bool) {
	dec := xml.NewDecoder(bytes.NewReader(content))

	var out bytes.Buffer
	copied := int64(0)
	changed := false

	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if _, ok := tok.(xml.CharData); !ok {
			continue
		}

		end := dec.InputOffset()
		if start < 0 || end > int64(len(content)) || start >= end {
			continue
		}

		raw := string(content[start:end])
		fixed := replace(raw)
		if fixed == raw {
			continue
		}

		out.Write(content[copied:start])
		out.WriteString(fixed)
		copied = end
		changed = true
	}

	if !changed {
		return content, false
	}
	out.Write(content[copied:])
	return out.Bytes(), true
}
