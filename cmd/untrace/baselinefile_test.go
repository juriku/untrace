package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/juriku/untrace/internal/baseline"
	"github.com/juriku/untrace/internal/decode"
	"github.com/juriku/untrace/internal/detect"
)

func sampleReports() []fileReport {
	return []fileReport{{
		Path: "a.go",
		Findings: []detect.Finding{
			{Line: 1, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
			{Line: 2, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
			{Line: 3, Column: 1, Codepoint: "U+E0074", Name: "Tag", Kind: "tag", InPayload: true},
		},
		Payloads: []detect.Payload{{Payload: decode.Payload{Runes: 2, Text: "hi", Printable: true}, Line: 3, Column: 1}},
		Mixed:    []detect.MixedWord{{Line: 4, Column: 1, Word: "paypal", Scripts: []string{"Latin", "Cyrillic"}}},
	}}
}

// The whole point of one identity function: an entry written from a scan has to
// match the fingerprint SARIF publishes for the same occurrence. If these drift,
// a baselined finding still raises a code-scanning alert.
func TestBaselineAndSarifAgreeOnIdentity(t *testing.T) {
	reports := sampleReports()

	fromBaseline := map[string]bool{}
	for _, e := range baselineEntries(reports) {
		fromBaseline[e.ID] = true
	}

	got := sarifFrom(t, reports)
	for _, r := range got.Runs[0].Results {
		id := r.PartialFingerprints["untraceIdentity/v1"]
		if !fromBaseline[id] {
			t.Errorf("SARIF published %q for %s, which no baseline entry names", id, r.RuleID)
		}
	}

	if len(fromBaseline) != len(got.Runs[0].Results) {
		t.Errorf("baseline recorded %d entries, SARIF published %d results",
			len(fromBaseline), len(got.Runs[0].Results))
	}
}

func TestBaselineEntriesSkipCharactersInsideAPayload(t *testing.T) {
	for _, e := range baselineEntries(sampleReports()) {
		if e.Codepoint == "U+E0074" {
			t.Error("a character inside a payload got its own baseline entry")
		}
	}
}

func TestApplyBaselineDropsWhatItAccepted(t *testing.T) {
	reports := sampleReports()
	entries := baselineEntries(reports)

	path := t.TempDir() + "/b.json"
	if err := baseline.Write(path, entries); err != nil {
		t.Fatal(err)
	}
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	accepted := applyBaseline(reports, identitiesFor(reports), set)

	if accepted != len(entries) {
		t.Errorf("accepted %d, want %d", accepted, len(entries))
	}
	r := reports[0]
	if len(r.Payloads) != 0 || len(r.Mixed) != 0 {
		t.Errorf("payloads/mixed survived: %+v %+v", r.Payloads, r.Mixed)
	}
	// The payload's own character is not baselined separately, so it remains.
	for _, f := range r.Findings {
		if !f.InPayload {
			t.Errorf("a baselined finding survived: %+v", f)
		}
	}
	if len(set.Stale()) != 0 {
		t.Errorf("entries went stale against the scan that produced them: %+v", set.Stale())
	}
}

func TestApplyBaselineKeepsWhatItDoesNotName(t *testing.T) {
	reports := sampleReports()
	set := baseline.New()

	if accepted := applyBaseline(reports, identitiesFor(reports), set); accepted != 0 {
		t.Errorf("an empty baseline accepted %d findings", accepted)
	}
	if len(reports[0].Payloads) != 1 || len(reports[0].Mixed) != 1 {
		t.Error("an empty baseline dropped something")
	}
}

// A finding added below a baselined one must still fail, or a baseline silently
// accepts everything a file gains after it is written.
func TestApplyBaselineStillReportsANewOccurrence(t *testing.T) {
	first := []fileReport{{
		Path:     "a.go",
		Findings: []detect.Finding{{Line: 1, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true}},
	}}

	path := t.TempDir() + "/b.json"
	if err := baseline.Write(path, baselineEntries(first)); err != nil {
		t.Fatal(err)
	}
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// The same file later, with a second occurrence and the first moved down.
	later := []fileReport{{
		Path: "a.go",
		Findings: []detect.Finding{
			{Line: 9, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
			{Line: 12, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
		},
	}}

	accepted := applyBaseline(later, identitiesFor(later), set)

	if accepted != 1 {
		t.Errorf("accepted %d, want 1", accepted)
	}
	if len(later[0].Findings) != 1 {
		t.Fatalf("got %d surviving findings, want 1", len(later[0].Findings))
	}
	if len(set.Stale()) != 0 {
		t.Errorf("the entry went stale despite matching: %+v", set.Stale())
	}
}

// Baselining the first of two occurrences must not hand its identity to the
// second: in code scanning a dismissal on that fingerprint would transfer, and
// silently suppress a finding nobody accepted.
func TestSarifKeepsIdentitiesAcrossBaselineFiltering(t *testing.T) {
	reports := func() []fileReport {
		return []fileReport{{
			Path: "a.py",
			Findings: []detect.Finding{
				{Line: 1, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
				{Line: 2, Column: 8, Codepoint: "U+200B", Name: "Zero Width Space", Kind: "hidden", Actionable: true},
			},
		}}
	}

	unfiltered := sarifFrom(t, reports())
	if len(unfiltered.Runs[0].Results) != 2 {
		t.Fatalf("got %d results, want 2", len(unfiltered.Runs[0].Results))
	}
	first := unfiltered.Runs[0].Results[0].PartialFingerprints["untraceIdentity/v1"]
	second := unfiltered.Runs[0].Results[1].PartialFingerprints["untraceIdentity/v1"]

	filtered := reports()
	ids := identitiesFor(filtered)
	path := t.TempDir() + "/b.json"
	if err := baseline.Write(path, []baseline.Entry{{ID: first, Path: "a.py", Codepoint: "U+200B"}}); err != nil {
		t.Fatal(err)
	}
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if accepted := applyBaseline(filtered, ids, set); accepted != 1 {
		t.Fatalf("accepted %d, want 1", accepted)
	}

	var buf bytes.Buffer
	if err := writeSarif(&buf, filtered, ids); err != nil {
		t.Fatal(err)
	}
	var got sarifLog
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if len(got.Runs[0].Results) != 1 {
		t.Fatalf("got %d results, want the one that survived", len(got.Runs[0].Results))
	}
	published := got.Runs[0].Results[0].PartialFingerprints["untraceIdentity/v1"]
	if published == first {
		t.Error("the surviving finding inherited the baselined finding's identity")
	}
	if published != second {
		t.Errorf("identity = %q, want %q", published, second)
	}
}

func TestBaselineEntriesSkipUnreadableFiles(t *testing.T) {
	entries := baselineEntries([]fileReport{{Path: "a.go", Error: "permission denied"}})

	if len(entries) != 0 {
		t.Errorf("an unreadable file produced %d entries", len(entries))
	}
}
