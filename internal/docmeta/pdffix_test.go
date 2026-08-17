package docmeta

import (
	"bytes"
	"strings"
	"testing"
)

func pdf(info string) []byte {
	return []byte("%PDF-1.7\n1 0 obj\n<< " + info + " >>\nendobj\ntrailer\n%%EOF\n")
}

func stripAll(Finding) bool { return true }

func TestFixPDFPreservesByteLength(t *testing.T) {
	in := pdf("/Producer (Claude) /Creator (Some AI Tool)")

	out, changed := FixPDF(in, stripAll)
	if !changed {
		t.Fatal("reported no change")
	}
	if len(out) != len(in) {
		t.Errorf("length = %d, want %d", len(out), len(in))
	}
}

func TestFixPDFEmptiesTheValue(t *testing.T) {
	in := pdf("/Producer (Claude)")

	out, _ := FixPDF(in, stripAll)
	if bytes.Contains(out, []byte("Claude")) {
		t.Errorf("value survived: %s", out)
	}

	doc := readPDF(out)
	if len(doc.Metadata) != 0 {
		t.Errorf("metadata = %v, want none once emptied", doc.Metadata)
	}
}

func TestFixPDFStripsOnlyWhatWasSelected(t *testing.T) {
	in := pdf(`/Producer (Claude) /Creator (Leica M11)`)

	out, _ := FixPDF(in, func(f Finding) bool { return f.Value == "Claude" })
	if bytes.Contains(out, []byte("Claude")) {
		t.Error("selected value survived")
	}
	if !bytes.Contains(out, []byte("Leica M11")) {
		t.Errorf("unselected value removed: %s", out)
	}
}

func TestFixPDFSkipsEscapedValues(t *testing.T) {
	in := pdf(`/Producer (Acme \(Pro\) Suite)`)

	out, changed := FixPDF(in, stripAll)
	if changed {
		t.Error("rewrote a value containing an escape")
	}
	if !bytes.Equal(in, out) {
		t.Error("bytes changed")
	}
}

func TestFixPDFIgnoresNonPDF(t *testing.T) {
	in := docx(t, "body")

	out, changed := FixPDF(in, stripAll)
	if changed || !bytes.Equal(in, out) {
		t.Error("a docx was rewritten by FixPDF")
	}
}

func TestFixRoutesPDFs(t *testing.T) {
	in := pdf("/Producer (Claude)")

	out, changed, err := Fix(in, FixOptions{Strip: stripAll})
	if err != nil {
		t.Fatal(err)
	}
	if !changed || bytes.Contains(out, []byte("Claude")) {
		t.Errorf("Fix did not route the PDF: %s", out)
	}
}

// XMP commonly repeats Producer and is never parsed.
func TestResidueFindsWhatStrippingMissed(t *testing.T) {
	data := []byte(`<< /Producer () >> <x:xmpmeta><pdf:Producer>Claude</pdf:Producer></x:xmpmeta>`)

	got := Residue(data, []string{"Claude", "Nowhere"})
	if len(got) != 1 || got[0] != "Claude" {
		t.Errorf("residue = %v, want [Claude]", got)
	}
}

func TestResidueIsEmptyWhenGone(t *testing.T) {
	if got := Residue([]byte("<< /Producer () >>"), []string{"Claude"}); len(got) != 0 {
		t.Errorf("residue = %v, want none", got)
	}
}

// Each xref entry is exactly 20 bytes: 10-digit offset, 5-digit generation,
// type, then " \n". Trailing spaces are load-bearing.
const validPDF = "%PDF-1.7\n" +
	"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
	"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
	"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n" +
	"4 0 obj\n<< /Producer (Claude) /Creator (Acme Editor) >>\nendobj\n" +
	"xref\n0 5\n" +
	"0000000000 65535 f \n" +
	"0000000009 00000 n \n" +
	"0000000058 00000 n \n" +
	"0000000115 00000 n \n" +
	"0000000186 00000 n \n" +
	"trailer\n<< /Size 5 /Root 1 0 R /Info 4 0 R >>\nstartxref\n249\n%%EOF\n"

func xrefOffsets(t *testing.T, data []byte) []int {
	t.Helper()
	at := bytes.Index(data, []byte("xref\n0 5\n"))
	if at < 0 {
		t.Fatal("no xref table")
	}
	rows := data[at+len("xref\n0 5\n"):]

	var out []int
	for i := 1; i < 5; i++ {
		row := rows[i*20 : i*20+20]
		if len(row) != 20 || row[18] != ' ' || row[19] != '\n' {
			t.Fatalf("entry %d is not 20 bytes: %q", i, row)
		}
		n := 0
		for _, c := range row[:10] {
			n = n*10 + int(c-'0')
		}
		out = append(out, n)
	}
	return out
}

func TestFixPDFKeepsEveryXrefOffsetValid(t *testing.T) {
	in := []byte(validPDF)

	for i, off := range xrefOffsets(t, in) {
		want := []byte{byte('1' + i), ' ', '0', ' ', 'o', 'b', 'j'}
		if !bytes.HasPrefix(in[off:], want) {
			t.Fatalf("fixture is wrong: offset %d does not start object %d", off, i+1)
		}
	}

	out, changed := FixPDF(in, stripAll)
	if !changed {
		t.Fatal("nothing stripped")
	}
	if len(out) != len(in) {
		t.Fatalf("length changed: %d, want %d", len(out), len(in))
	}

	for i, off := range xrefOffsets(t, out) {
		want := []byte{byte('1' + i), ' ', '0', ' ', 'o', 'b', 'j'}
		if !bytes.HasPrefix(out[off:], want) {
			t.Errorf("offset %d no longer starts object %d: %q", off, i+1, out[off:off+12])
		}
	}

	if got := bytes.Index(out, []byte("xref\n")); got != 249 {
		t.Errorf("xref moved to %d, startxref still says 249", got)
	}
}

func TestFixPDFStillParses(t *testing.T) {
	in := pdf("/Producer (Claude) /Creator (Some AI Tool)")

	out, _ := FixPDF(in, stripAll)
	if !strings.HasPrefix(string(out), "%PDF-") || !strings.Contains(string(out), "%%EOF") {
		t.Errorf("structure damaged: %s", out)
	}
	if Detect(out) != FormatPDF {
		t.Error("no longer detected as a PDF")
	}
}
