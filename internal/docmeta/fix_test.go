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

func epubFixture(t *testing.T, opf, chapter string) []byte {
	t.Helper()
	return buildZip(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf":      opf,
		"OEBPS/ch1.xhtml":        chapter,
	})
}

func TestFixEPUBCleansChapterText(t *testing.T) {
	data := epubFixture(t,
		`<package><metadata><dc:creator>A Person</dc:creator></metadata></package>`,
		`<html><body><p>hello`+string(rune(0x200B))+`world</p></body></html>`)

	out, changed, err := Fix(data, FixOptions{Text: func(s string) string {
		return strings.ReplaceAll(s, string(rune(0x200B)), "")
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("an epub with a hidden character was left unchanged")
	}

	d, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(d.Text, string(rune(0x200B))) {
		t.Error("the hidden character survived")
	}
	if !strings.Contains(d.Text, "helloworld") {
		t.Errorf("surrounding text was damaged: %q", d.Text)
	}
}

func TestFixEPUBBlanksOnlyWhatStripSelects(t *testing.T) {
	data := epubFixture(t,
		`<package><metadata>`+
			`<dc:creator>Claude</dc:creator><dc:publisher>Acme</dc:publisher>`+
			`<meta name="generator" content="Midjourney v6"/></metadata></package>`,
		`<html><body><p>text</p></body></html>`)

	out, changed, err := Fix(data, FixOptions{Strip: func(f Finding) bool {
		return f.Value == "Claude" || f.Value == "Midjourney v6"
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("nothing was stripped")
	}

	d, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range d.Metadata {
		switch m.Label {
		case "creator", "generator":
			if m.Value != "" {
				t.Errorf("%s survived as %q", m.Label, m.Value)
			}
		case "publisher":
			if m.Value != "Acme" {
				t.Errorf("publisher = %q, want Acme", m.Value)
			}
		}
	}
}

// A rewritten archive has to stay readable, or fixing a book destroys it.
func TestFixEPUBKeepsEveryEntry(t *testing.T) {
	data := epubFixture(t,
		`<package><metadata><dc:creator>Claude</dc:creator></metadata></package>`,
		`<html><body><p>hello`+string(rune(0x200B))+`world</p></body></html>`)

	out, _, err := Fix(data, FixOptions{Text: func(s string) string {
		return strings.ReplaceAll(s, string(rune(0x200B)), "")
	}})
	if err != nil {
		t.Fatal(err)
	}

	r, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("the fixed epub is not a readable archive: %v", err)
	}
	want := []string{"mimetype", "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/ch1.xhtml"}
	got := map[string]bool{}
	for _, f := range r.File {
		got[f.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("%s was dropped", name)
		}
	}
}

func TestBlankAttrValue(t *testing.T) {
	cases := map[string]string{
		`<meta name="generator" content="X"/>`:   `<meta name="generator" content=""/>`,
		`<meta name='generator' content='X'/>`:   `<meta name='generator' content=''/>`,
		`<meta content = "X" name="generator"/>`: `<meta content = "" name="generator"/>`,

		// encoding/xml hands the caller "ChatGPT", which appears nowhere in
		// these bytes. Locating the attribute by name is what survives that.
		`<meta name="generator" content="Chat&#71;PT"/>`: `<meta name="generator" content=""/>`,
		`<meta name="generator" content="A &amp; B"/>`:   `<meta name="generator" content=""/>`,

		// A longer attribute name must not be mistaken for this one.
		`<meta xcontent="keep" content="X"/>`: `<meta xcontent="keep" content=""/>`,
	}

	for in, want := range cases {
		got, ok := blankAttrValue([]byte(in), "content")
		if !ok {
			t.Errorf("blankAttrValue(%q) did not match", in)
			continue
		}
		if string(got) != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestBlankAttrValueLeavesAnAbsentAttributeAlone(t *testing.T) {
	for _, in := range []string{
		`<meta name="generator"/>`,
		`<meta content/>`,
		`<meta content=/>`,
		`<meta content="unterminated />`,
	} {
		if _, ok := blankAttrValue([]byte(in), "content"); ok {
			t.Errorf("blankAttrValue(%q) reported a blanking", in)
		}
	}
}

// One entity in the value used to defeat --strip-metadata while the report
// still said the record was stripped.
func TestFixEPUBStripsAGeneratorWrittenWithAnEntity(t *testing.T) {
	data := epubFixture(t,
		`<package><metadata><meta name="generator" content="Chat&#71;PT 4o"/></metadata></package>`,
		`<html><body><p>text</p></body></html>`)

	out, changed, err := Fix(data, FixOptions{Strip: func(f Finding) bool {
		return f.Value == "ChatGPT 4o"
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("the entity-written generator was not stripped")
	}

	if got := entry(t, out, "OEBPS/content.opf"); bytes.Contains(got, []byte("PT 4o")) {
		t.Errorf("the generator survived: %s", got)
	}
}
