package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/juriku/untrace/internal/decode"
	"github.com/juriku/untrace/internal/detect"
)

func sarifFrom(t *testing.T, reports []fileReport) sarifLog {
	t.Helper()

	var buf bytes.Buffer
	if err := writeSarif(&buf, reports, identitiesFor(reports)); err != nil {
		t.Fatal(err)
	}
	var got sarifLog
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	return got
}

func TestSarifCarriesTheFieldsCodeScanningRequires(t *testing.T) {
	got := sarifFrom(t, []fileReport{{
		Path:     "a.go",
		Findings: []detect.Finding{{Line: 1, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true}},
	}})

	if got.Version != "2.1.0" {
		t.Errorf("version = %q, want 2.1.0", got.Version)
	}
	if got.Schema == "" {
		t.Error("$schema is empty")
	}
	if len(got.Runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(got.Runs))
	}

	driver := got.Runs[0].Tool.Driver
	if driver.Name != "untrace" {
		t.Errorf("driver name = %q", driver.Name)
	}
	if len(driver.Rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(driver.Rules))
	}
	for _, r := range driver.Rules {
		if r.ID == "" || r.ShortDescription.Text == "" || r.FullDescription.Text == "" || r.Help.Text == "" {
			t.Errorf("rule %q is missing a required field: %+v", r.ID, r)
		}
	}

	if len(got.Runs[0].Results) != 1 {
		t.Fatalf("got %d results, want 1", len(got.Runs[0].Results))
	}
	res := got.Runs[0].Results[0]
	if res.Message.Text == "" {
		t.Error("result message is empty")
	}
	if len(res.Locations) == 0 {
		t.Fatal("result has no location")
	}
	if len(res.PartialFingerprints) == 0 {
		t.Error("result has no partialFingerprints, so alerts would duplicate across commits")
	}
}

func TestSarifLevels(t *testing.T) {
	got := sarifFrom(t, []fileReport{{
		Path: "a.go",
		Findings: []detect.Finding{
			{Line: 1, Column: 1, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
			{Line: 2, Column: 1, Codepoint: "U+0430", Name: "Cyrillic Small Letter A", Kind: "homoglyph"},
		},
		Payloads: []detect.Payload{{Payload: decode.Payload{Runes: 2, Text: "hi", Printable: true}, Line: 3, Column: 1}},
		Mixed:    []detect.MixedWord{{Line: 4, Column: 1, Word: "paypal", Scripts: []string{"Latin", "Cyrillic"}}},
	}})

	want := map[string]string{
		"untrace/U+200B": "warning",
		"untrace/U+0430": "note",
		rulePayload:      "error",
		ruleMixedScript:  "warning",
	}

	seen := map[string]string{}
	for _, r := range got.Runs[0].Results {
		seen[r.RuleID] = r.Level
	}
	for id, level := range want {
		if seen[id] != level {
			t.Errorf("%s level = %q, want %q", id, seen[id], level)
		}
	}
}

// A region ends at the character after it, so a span of n runes is n columns
// wide. An off-by-one here puts every squiggle in the wrong place.
func TestSarifRegionsSpanTheFinding(t *testing.T) {
	got := sarifFrom(t, []fileReport{{
		Path:     "a.go",
		Findings: []detect.Finding{{Line: 1, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true}},
		Mixed:    []detect.MixedWord{{Line: 4, Column: 3, Word: "paypal", Scripts: []string{"Latin", "Cyrillic"}}},
		Payloads: []detect.Payload{{Payload: decode.Payload{Runes: 5, Text: "leak", Printable: true}, Line: 3, Column: 2}},
	}})

	regions := map[string]sarifRegion{}
	for _, r := range got.Runs[0].Results {
		regions[r.RuleID] = r.Locations[0].PhysicalLocation.Region
	}

	cases := []struct {
		rule             string
		start, end, line int
	}{
		{"untrace/U+200B", 8, 9, 1},
		{ruleMixedScript, 3, 9, 4},
		{rulePayload, 2, 7, 3},
	}
	for _, c := range cases {
		got := regions[c.rule]
		if got.StartColumn != c.start || got.EndColumn != c.end {
			t.Errorf("%s columns = %d..%d, want %d..%d", c.rule, got.StartColumn, got.EndColumn, c.start, c.end)
		}
		if got.StartLine != c.line || got.EndLine != c.line {
			t.Errorf("%s lines = %d..%d, want %d", c.rule, got.StartLine, got.EndLine, c.line)
		}
	}
}

func TestSarifSeparatesRepeatedCodepointsInOneFile(t *testing.T) {
	got := sarifFrom(t, []fileReport{{
		Path: "a.go",
		Findings: []detect.Finding{
			{Line: 1, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
			{Line: 2, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
		},
	}})

	results := got.Runs[0].Results
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	first := results[0].PartialFingerprints["untraceIdentity/v1"]
	second := results[1].PartialFingerprints["untraceIdentity/v1"]
	if first == second {
		t.Errorf("both occurrences carry identity %q, so one alert would mask the other", first)
	}

	if len(got.Runs[0].Tool.Driver.Rules) != 1 {
		t.Errorf("one codepoint produced %d rules, want 1", len(got.Runs[0].Tool.Driver.Rules))
	}
}

// A character inside a decoded run is reported as part of the payload, so
// emitting it separately would double-count the same bytes.
func TestSarifSkipsCharactersInsideAPayload(t *testing.T) {
	got := sarifFrom(t, []fileReport{{
		Path: "a.go",
		Findings: []detect.Finding{
			{Line: 1, Column: 5, Codepoint: "U+E0074", Name: "Tag", Kind: "tag", InPayload: true},
			{Line: 1, Column: 6, Codepoint: "U+E0078", Name: "Tag", Kind: "tag", InPayload: true},
		},
		Payloads: []detect.Payload{{Payload: decode.Payload{Runes: 2, Text: "tx", Printable: true}, Line: 1, Column: 5}},
	}})

	if n := len(got.Runs[0].Results); n != 1 {
		t.Errorf("got %d results, want only the payload", n)
	}
}

func TestSarifReportsAnUnprintablePayloadHonestly(t *testing.T) {
	got := sarifFrom(t, []fileReport{{
		Path:     "a.go",
		Payloads: []detect.Payload{{Payload: decode.Payload{Runes: 3, Printable: false}, Line: 1, Column: 1}},
	}})

	msg := got.Runs[0].Results[0].Message.Text
	if !strings.Contains(msg, "not printable") {
		t.Errorf("message = %q, want it to say the payload is not printable", msg)
	}
}

// Encoding an empty scan as null instead of [] makes the document invalid.
func TestSarifOnACleanScanIsStillValid(t *testing.T) {
	var buf bytes.Buffer
	clean := []fileReport{{Path: "a.go"}}
	if err := writeSarif(&buf, clean, identitiesFor(clean)); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("\"results\": null")) {
		t.Error("results encoded as null, want an empty array")
	}

	got := sarifFrom(t, []fileReport{{Path: "a.go"}})
	if len(got.Runs[0].Results) != 0 {
		t.Errorf("got %d results on a clean scan", len(got.Runs[0].Results))
	}
}

func TestSarifSkipsFilesThatCouldNotBeRead(t *testing.T) {
	got := sarifFrom(t, []fileReport{{Path: "a.go", Error: "permission denied"}})

	if len(got.Runs[0].Results) != 0 {
		t.Errorf("an unreadable file produced %d results", len(got.Runs[0].Results))
	}
}

func TestSarifTruncatesAtTheCodeScanningLimit(t *testing.T) {
	over := sarifMaxResults + 10
	findings := make([]detect.Finding, 0, over)
	for i := 0; i < over; i++ {
		findings = append(findings, detect.Finding{
			Line: i + 1, Column: 1, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true,
		})
	}

	got := sarifFrom(t, []fileReport{{Path: "a.go", Findings: findings}})

	if n := len(got.Runs[0].Results); n != sarifMaxResults {
		t.Errorf("got %d results, want the limit of %d", n, sarifMaxResults)
	}
}

// A confusable letter carries the typographic kind, the same as a curly quote,
// so describing it by kind alone called a Cyrillic "a" a piece of punctuation.
func TestSarifDescribesAConfusableLetterAsALetter(t *testing.T) {
	got := sarifFrom(t, []fileReport{{
		Path: "a.txt",
		Findings: []detect.Finding{
			{Line: 1, Column: 1, Rune: 0x0430, Codepoint: "U+0430", Name: "Cyrillic Small Letter A", Kind: "typographic"},
			{Line: 2, Column: 1, Rune: 0x2014, Codepoint: "U+2014", Name: "Em Dash", Kind: "typographic", Actionable: true},
		},
	}})

	descriptions := map[string]string{}
	for _, r := range got.Runs[0].Tool.Driver.Rules {
		descriptions[r.ID] = r.FullDescription.Text
	}

	if !strings.Contains(descriptions["untrace/U+0430"], "letter from another script") {
		t.Errorf("a confusable letter is described as %q", descriptions["untrace/U+0430"])
	}
	if !strings.Contains(descriptions["untrace/U+2014"], "ordinary punctuation") {
		t.Errorf("a dash is described as %q", descriptions["untrace/U+2014"])
	}
}
