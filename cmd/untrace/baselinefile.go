package main

import (
	"fmt"
	"strings"

	"github.com/juriku/untrace/internal/baseline"
	"github.com/juriku/untrace/internal/markers"
)

func identitiesFor(reports []fileReport) []reportIdentities {
	w := newIdentityWalker()
	out := make([]reportIdentities, len(reports))
	for i, r := range reports {
		out[i] = w.forReport(r)
	}
	return out
}

// ids is compacted in lockstep, keeping each surviving finding's identity.
func applyBaseline(reports []fileReport, ids []reportIdentities, set *baseline.Set) int {
	accepted := 0

	for i := range reports {
		r, id := &reports[i], &ids[i]

		findings, keptFindings := r.Findings[:0], id.Findings[:0]
		for j, f := range r.Findings {
			if id.Findings[j] != "" && set.Has(id.Findings[j]) {
				accepted++
				continue
			}
			findings = append(findings, f)
			keptFindings = append(keptFindings, id.Findings[j])
		}
		r.Findings, id.Findings = findings, keptFindings

		payloads, keptPayloads := r.Payloads[:0], id.Payloads[:0]
		for j, p := range r.Payloads {
			if set.Has(id.Payloads[j]) {
				accepted++
				continue
			}
			payloads = append(payloads, p)
			keptPayloads = append(keptPayloads, id.Payloads[j])
		}
		r.Payloads, id.Payloads = payloads, keptPayloads

		mixed, keptMixed := r.Mixed[:0], id.Mixed[:0]
		for j, m := range r.Mixed {
			if set.Has(id.Mixed[j]) {
				accepted++
				continue
			}
			mixed = append(mixed, m)
			keptMixed = append(keptMixed, id.Mixed[j])
		}
		r.Mixed, id.Mixed = mixed, keptMixed
	}
	return accepted
}

func escapeMarkers(s string) string {
	var b strings.Builder
	for _, r := range s {
		if _, ok := markers.Lookup(r, markers.Options{Typographic: true, IVS: true}); ok {
			fmt.Fprintf(&b, "U+%04X", r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func baselineEntries(reports []fileReport) []baseline.Entry {
	ids := identitiesFor(reports)
	var out []baseline.Entry

	for i, r := range reports {
		if r.Error != "" {
			continue
		}
		id := ids[i]

		path := escapeMarkers(r.Path)

		for j, f := range r.Findings {
			if id.Findings[j] == "" {
				continue
			}
			out = append(out, baseline.Entry{ID: id.Findings[j], Path: path, Codepoint: f.Codepoint})
		}
		for j := range r.Payloads {
			out = append(out, baseline.Entry{ID: id.Payloads[j], Path: path, Codepoint: "payload"})
		}
		for j, m := range r.Mixed {
			out = append(out, baseline.Entry{
				ID: id.Mixed[j], Path: path, Codepoint: "mixed-script " + escapeMarkers(m.Word),
			})
		}
	}
	return out
}
