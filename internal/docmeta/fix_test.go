package docmeta

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

var emDash = string(rune(0x2014))

func entry(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.File {
		if f.Name != name {
			continue
		}
		got, err := readEntry(f)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	t.Fatalf("%s missing from the archive", name)
	return nil
}

func names(t *testing.T, data []byte) []string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range r.File {
		out = append(out, f.Name)
	}
	return out
}

func dashToHyphen(s string) string { return strings.ReplaceAll(s, emDash, "-") }

func TestFixRewritesBodyTextOnly(t *testing.T) {
	in := docx(t, "a "+emDash+" b")

	out, changed, err := Fix(in, FixOptions{Text: dashToHyphen})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Fix reported no change")
	}

	body := string(entry(t, out, "word/document.xml"))
	if strings.Contains(body, emDash) {
		t.Errorf("em dash survived: %s", body)
	}
	if !strings.Contains(body, "a - b") {
		t.Errorf("body text = %s, want the hyphen", body)
	}
}

func TestFixLeavesOtherEntriesByteIdentical(t *testing.T) {
	in := docx(t, "a "+emDash+" b")

	out, _, err := Fix(in, FixOptions{Text: dashToHyphen})
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range names(t, in) {
		if name == "word/document.xml" {
			continue
		}
		if !bytes.Equal(entry(t, in, name), entry(t, out, name)) {
			t.Errorf("%s changed", name)
		}
	}
}

func TestFixKeepsEveryEntry(t *testing.T) {
	in := docx(t, "a "+emDash+" b")

	out, _, err := Fix(in, FixOptions{Text: dashToHyphen})
	if err != nil {
		t.Fatal(err)
	}

	before, after := names(t, in), names(t, out)
	if len(before) != len(after) {
		t.Fatalf("entries = %v, want %v", after, before)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("entry %d = %q, want %q", i, after[i], before[i])
		}
	}
}

// Nothing to fix must return the input untouched, so a scan of a clean tree
// never rewrites a document.
func TestFixWithNothingToDoReturnsTheInput(t *testing.T) {
	in := docx(t, "plain text")

	out, changed, err := Fix(in, FixOptions{Text: dashToHyphen})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("reported a change with nothing to fix")
	}
	if !bytes.Equal(in, out) {
		t.Error("bytes changed with nothing to fix")
	}
}

// Markup must never reach the replacement, or a marker inside an attribute or
// an element name would be rewritten and the document would break.
func TestFixNeverTouchesMarkup(t *testing.T) {
	in := docx(t, "text")

	var seen []string
	if _, _, err := Fix(in, FixOptions{Text: func(s string) string {
		seen = append(seen, s)
		return s
	}}); err != nil {
		t.Fatal(err)
	}

	for _, s := range seen {
		if strings.ContainsAny(s, "<>") {
			t.Errorf("markup reached the replacement: %q", s)
		}
	}
}

func TestFixIgnoresNonOfficeInput(t *testing.T) {
	in := []byte("%PDF-1.7\ntrailer\n%%EOF\n")

	out, changed, err := Fix(in, FixOptions{Text: dashToHyphen})
	if err != nil {
		t.Fatal(err)
	}
	if changed || !bytes.Equal(in, out) {
		t.Error("a PDF was rewritten")
	}
}

func TestFixOnCorruptZipDoesNotPanic(t *testing.T) {
	in := append([]byte("PK\x03\x04"), make([]byte, 64)...)

	if _, changed, _ := Fix(in, FixOptions{Text: dashToHyphen}); changed {
		t.Error("reported a change on a corrupt archive")
	}
}

func stripNamed(name string) func(Finding) bool {
	return func(f Finding) bool { return f.Value == name }
}

func TestFixBlanksSelectedMetadataOnly(t *testing.T) {
	in := docx(t, "body")

	out, changed, err := Fix(in, FixOptions{Strip: stripNamed("Someone Else")})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("reported no change")
	}

	doc, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := labelValue(doc, "lastModifiedBy"); got != "" {
		t.Errorf("lastModifiedBy = %q, want it blanked", got)
	}
	if got := labelValue(doc, "creator"); got != "A Person" {
		t.Errorf("creator = %q, want it left alone", got)
	}
}

// The element has to survive, since a reader may require the property.
func TestFixKeepsTheBlankedElement(t *testing.T) {
	in := docx(t, "body")

	out, _, err := Fix(in, FixOptions{Strip: stripNamed("Someone Else")})
	if err != nil {
		t.Fatal(err)
	}

	core := string(entry(t, out, "docProps/core.xml"))
	if !strings.Contains(core, "lastModifiedBy") {
		t.Errorf("element removed entirely: %s", core)
	}
}

func TestFixStripsEveryValueWhenAsked(t *testing.T) {
	in := docx(t, "body")

	out, _, err := Fix(in, FixOptions{Strip: func(Finding) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}

	doc, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Metadata) != 0 {
		t.Errorf("metadata = %v, want none", doc.Metadata)
	}
}

func TestFixLeavesBodyAloneWhenOnlyStripping(t *testing.T) {
	in := docx(t, "a "+emDash+" b")

	out, _, err := Fix(in, FixOptions{Strip: stripNamed("Someone Else")})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(entry(t, in, "word/document.xml"), entry(t, out, "word/document.xml")) {
		t.Error("body changed while only stripping metadata")
	}
}
