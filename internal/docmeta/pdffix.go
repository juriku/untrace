package docmeta

import "bytes"

// FixPDF empties the Info dictionary values strip selects.
//
// The replacement is the same byte length as what it replaces, which is what
// keeps every cross-reference offset valid.
func FixPDF(data []byte, strip func(Finding) bool) ([]byte, bool) {
	if strip == nil || Detect(data) != FormatPDF {
		return data, false
	}

	var out []byte
	last := 0
	changed := false

	for _, m := range pdfInfoKeys.FindAllSubmatchIndex(data, -1) {
		valueStart, valueEnd := m[4], m[5]
		if valueStart < 0 || valueEnd < valueStart {
			continue
		}

		raw := data[valueStart:valueEnd]

		// pdfInfoKeys stops at the first ")", escaped or not, so a value holding
		// a backslash may be a truncated match.
		if bytes.ContainsRune(raw, '\\') {
			continue
		}

		value := decodePDFString(raw)
		if value == "" || !strip(Finding{Label: string(data[m[2]:m[3]]), Value: value}) {
			continue
		}

		// readPDF skips an empty value but reports a value made of spaces.
		out = append(out, data[last:valueStart-1]...)
		out = append(out, '(', ')')
		out = append(out, bytes.Repeat([]byte(" "), len(raw))...)
		last = valueEnd + 1
		changed = true
	}

	if !changed {
		return data, false
	}
	return append(out, data[last:]...), true
}

// Residue reports which values still appear in data. Compressed object streams,
// hex strings and XMP packets are outside what FixPDF matches.
func Residue(data []byte, values []string) []string {
	var out []string
	for _, v := range values {
		if v != "" && bytes.Contains(data, []byte(v)) {
			out = append(out, v)
		}
	}
	return out
}
