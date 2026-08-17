package docmeta

import (
	"bytes"
	"strings"
	"testing"
)

var zeroWidth = string(rune(0x200B))

func para(text string) string {
	return `<w:document><w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body></w:document>`
}

func stripZeroWidth(s string) string { return strings.ReplaceAll(s, zeroWidth, "") }

func TestEveryBodyPartIsReadAndFixed(t *testing.T) {
	cases := map[string]struct{ part, content string }{
		"word document":  {"word/document.xml", para("in" + zeroWidth + "doc")},
		"word footnotes": {"word/footnotes.xml", para("in" + zeroWidth + "footnote")},
		"word endnotes":  {"word/endnotes.xml", para("in" + zeroWidth + "endnote")},
		"word header":    {"word/header1.xml", para("in" + zeroWidth + "header")},
		"word footer":    {"word/footer1.xml", para("in" + zeroWidth + "footer")},
		"excel strings": {"xl/sharedStrings.xml",
			`<sst><si><t>in` + zeroWidth + `cell</t></si></sst>`},
		"powerpoint slide": {"ppt/slides/slide1.xml",
			`<p:sld><a:t>in` + zeroWidth + `slide</a:t></p:sld>`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := buildZip(t, map[string]string{
				"[Content_Types].xml": `<Types/>`,
				tc.part:               tc.content,
			})

			doc, err := Read(in)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(doc.Text, zeroWidth) {
				t.Fatalf("%s: text not read, got %q", tc.part, doc.Text)
			}

			out, changed, err := Fix(in, FixOptions{Text: stripZeroWidth})
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatalf("%s: reported no change", tc.part)
			}
			if bytes.Contains(entry(t, out, tc.part), []byte(zeroWidth)) {
				t.Errorf("%s: marker survived the fix", tc.part)
			}
		})
	}
}

func TestEveryBodyPartIsFixedInOnePass(t *testing.T) {
	parts := []string{
		"word/document.xml", "word/footnotes.xml", "word/endnotes.xml",
		"word/header1.xml", "word/header2.xml", "word/footer1.xml",
	}
	entries := map[string]string{"[Content_Types].xml": `<Types/>`}
	for _, p := range parts {
		entries[p] = para("mark" + zeroWidth + "ed")
	}
	in := buildZip(t, entries)

	out, changed, err := Fix(in, FixOptions{Text: stripZeroWidth})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("reported no change")
	}

	for _, p := range parts {
		if bytes.Contains(entry(t, out, p), []byte(zeroWidth)) {
			t.Errorf("%s: marker survived when six parts were fixed at once", p)
		}
	}
	if got := entry(t, out, "[Content_Types].xml"); !bytes.Equal(got, []byte(`<Types/>`)) {
		t.Errorf("untouched entry changed to %q", got)
	}
}

// "ppt/slides/" is a prefix match, so it also selects the rels directory beside
// the slides.
func TestSlideRelsSurvivesTheFix(t *testing.T) {
	const rels = `<Relationships><Relationship Id="rId1" Target="../media/image1.png"/></Relationships>`
	in := buildZip(t, map[string]string{
		"[Content_Types].xml":              `<Types/>`,
		"ppt/slides/slide1.xml":            `<p:sld><a:t>a` + zeroWidth + `b</a:t></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": rels,
	})

	out, _, err := Fix(in, FixOptions{Text: stripZeroWidth})
	if err != nil {
		t.Fatal(err)
	}
	if got := entry(t, out, "ppt/slides/_rels/slide1.xml.rels"); !bytes.Equal(got, []byte(rels)) {
		t.Errorf("rels rewritten to %q", got)
	}
}
