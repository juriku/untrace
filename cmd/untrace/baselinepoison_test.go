package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juriku/untrace/internal/baseline"
	"github.com/juriku/untrace/internal/config"
	"github.com/juriku/untrace/internal/decode"
	"github.com/juriku/untrace/internal/detect"
	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/resolve"
)

func poisonedReports() []fileReport {
	cyrillicA := string(rune(0x0430))

	return []fileReport{{
		Path: "a.txt",
		Findings: []detect.Finding{
			{Line: 1, Column: 1, Rune: 0x200B, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
			{Line: 3, Column: 11, Rune: 0x0430, Codepoint: "U+0430", Name: "Cyrillic Small Letter A", Kind: "typographic"},
		},
		Payloads: []detect.Payload{{
			Payload: decode.Payload{Runes: 4, Text: "leak" + cyrillicA, Printable: true},
			Line:    4, Column: 1,
		}},
		Mixed: []detect.MixedWord{{
			Line: 3, Column: 10, Word: "p" + cyrillicA + "ypal", Scripts: []string{"Latin", "Cyrillic"},
		}},
	}}
}

func TestBaselineEntriesCarryNoLiveMarkers(t *testing.T) {
	var b strings.Builder
	for _, e := range baselineEntries(poisonedReports()) {
		b.WriteString(e.ID)
		b.WriteString(" ")
		b.WriteString(e.Path)
		b.WriteString(" ")
		b.WriteString(e.Codepoint)
		b.WriteString("\n")
	}

	d := &detect.Detector{
		Markers:     markers.Options{Typographic: true, IVS: true},
		Policy:      resolve.PolicyFor(resolve.FormatData),
		MixedScript: resolve.Report,
	}
	res := d.Run(b.String())

	if len(res.Findings) != 0 {
		t.Errorf("baseline text carries %d marker(s): %+v", len(res.Findings), res.Findings)
	}
	if len(res.Mixed) != 0 {
		t.Errorf("baseline text carries a mixed-script word: %+v", res.Mixed)
	}
	if len(res.Payloads) != 0 {
		t.Errorf("baseline text carries a payload: %+v", res.Payloads)
	}
}

func TestBaselineEntriesStayReadable(t *testing.T) {
	var mixed baseline.Entry
	for _, e := range baselineEntries(poisonedReports()) {
		if strings.HasPrefix(e.Codepoint, "mixed-script") {
			mixed = e
		}
	}

	if mixed.ID == "" {
		t.Fatal("no mixed-script entry was written")
	}
	if !strings.Contains(mixed.Codepoint, "U+0430") {
		t.Errorf("codepoint = %q, want the confusable named by number", mixed.Codepoint)
	}
	if !strings.Contains(mixed.Codepoint, "p") || !strings.Contains(mixed.Codepoint, "ypal") {
		t.Errorf("codepoint = %q, want the ASCII of the word kept readable", mixed.Codepoint)
	}
}

func TestBaselineRoundTripLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	cyrillicA := string(rune(0x0430))
	content := "login at p" + cyrillicA + "ypal\nid = \"a" + string(rune(0x200B)) + "b\"\n"

	if err := os.WriteFile(filepath.Join(dir, "legacy.py"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	mo := markers.Options{Typographic: true, IVS: true}
	opt := options{}

	first := runPaths([]string{dir}, opt, mo, &config.Config{})
	entries := baselineEntries(first)
	if len(entries) == 0 {
		t.Fatal("the fixture produced no findings, so the round trip proves nothing")
	}

	path := filepath.Join(dir, baseline.DefaultPath)
	if err := baseline.Write(path, entries); err != nil {
		t.Fatal(err)
	}

	opt.baselinePath = path
	second := runPaths([]string{dir}, opt, mo, &config.Config{})
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	applyBaseline(second, identitiesFor(second), set)

	for _, r := range second {
		if len(r.Findings) != 0 || len(r.Mixed) != 0 || len(r.Payloads) != 0 {
			t.Errorf("%s still reports %+v %+v %+v", r.Path, r.Findings, r.Mixed, r.Payloads)
		}
	}
	if stale := set.Stale(); len(stale) != 0 {
		t.Errorf("entries went stale against the scan that wrote them: %+v", stale)
	}
}

func TestBaselineEntriesEscapeThePath(t *testing.T) {
	reports := []fileReport{{
		Path:     "dir/we" + string(rune(0x200B)) + "ird.txt",
		Findings: []detect.Finding{{Line: 1, Column: 1, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true}},
	}}

	for _, e := range baselineEntries(reports) {
		if strings.ContainsRune(e.Path, 0x200B) {
			t.Errorf("path %q carries the marker verbatim", e.Path)
		}
	}
}
