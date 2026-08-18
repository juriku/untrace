package main

import (
	"github.com/juriku/untrace/internal/detect"
	"github.com/juriku/untrace/internal/finding"
)

// A baseline entry and a SARIF fingerprint must name the same occurrence, so
// both read one identitiesFor result. Walking again after a filter renumbers.
type identityWalker struct {
	seen map[string]int
}

func newIdentityWalker() *identityWalker {
	return &identityWalker{seen: map[string]int{}}
}

func (w *identityWalker) next(path, key string) string {
	k := path + "\x00" + key
	n := w.seen[k]
	w.seen[k]++
	return finding.Identity(path, key, n)
}

// Each slice is parallel to the one it names. A finding inside a payload gets
// an empty identity and consumes no occurrence.
type reportIdentities struct {
	Findings []string
	Payloads []string
	Mixed    []string
}

func (w *identityWalker) forReport(r fileReport) reportIdentities {
	out := reportIdentities{
		Findings: make([]string, len(r.Findings)),
		Payloads: make([]string, len(r.Payloads)),
		Mixed:    make([]string, len(r.Mixed)),
	}

	for i, f := range r.Findings {
		if f.InPayload {
			continue
		}
		out.Findings[i] = w.next(r.Path, findingKey(f))
	}
	for i := range r.Payloads {
		out.Payloads[i] = w.next(r.Path, payloadKey())
	}
	for i, m := range r.Mixed {
		out.Mixed[i] = w.next(r.Path, mixedKey(m.Word))
	}
	return out
}

func findingKey(f detect.Finding) string { return f.Codepoint }
func mixedKey(word string) string        { return "mixed:" + word }
func payloadKey() string                 { return "payload" }
