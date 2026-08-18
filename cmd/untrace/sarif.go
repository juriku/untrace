package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/juriku/untrace/internal/detect"
)

// https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning
const (
	sarifSchema     = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json"
	sarifVersion    = "2.1.0"
	sarifToolURI    = "https://github.com/juriku/untrace"
	sarifMaxResults = 25000
)

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ShortDescription sarifText `json:"shortDescription"`
	FullDescription  sarifText `json:"fullDescription"`
	Help             sarifText `json:"help"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             sarifText         `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

// Columns count runes, matching --json. A region ends at the character after
// it, so a single-rune finding spans start to start+1.
type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

const (
	ruleMixedScript = "untrace/mixed-script"
	rulePayload     = "untrace/payload"
)

type sarifBuilder struct {
	results []sarifResult
	rules   map[string]sarifRule
}

func newSarifBuilder() *sarifBuilder {
	return &sarifBuilder{rules: map[string]sarifRule{}}
}

func (b *sarifBuilder) rule(id, name, short, full, help string) {
	if _, ok := b.rules[id]; ok {
		return
	}
	b.rules[id] = sarifRule{
		ID:               id,
		Name:             name,
		ShortDescription: sarifText{Text: short},
		FullDescription:  sarifText{Text: full},
		Help:             sarifText{Text: help},
	}
}

func (b *sarifBuilder) add(path, ruleID, level, message string, line, col, runes int, identity string) {
	b.results = append(b.results, sarifResult{
		RuleID:  ruleID,
		Level:   level,
		Message: sarifText{Text: message},
		Locations: []sarifLocation{{
			PhysicalLocation: sarifPhysical{
				ArtifactLocation: sarifArtifact{URI: filepath.ToSlash(path)},
				Region: sarifRegion{
					StartLine:   line,
					StartColumn: col,
					EndLine:     line,
					EndColumn:   col + runes,
				},
			},
		}},
		PartialFingerprints: map[string]string{"untraceIdentity/v1": identity},
	})
}

func (b *sarifBuilder) addReport(r fileReport, ids reportIdentities) {
	for i, f := range r.Findings {
		if f.InPayload {
			continue
		}
		id := "untrace/" + f.Codepoint
		b.rule(id, f.Name,
			f.Name+" ("+f.Codepoint+")",
			f.Name+" is a "+f.Kind+" marker. "+kindGuidance(f),
			"Run untrace --fix on this file, or add an untrace:ignore comment if the character is deliberate.")

		b.add(r.Path, id, sarifLevel(f.Actionable),
			f.Name+" ("+f.Codepoint+"), "+outcome(f.Actionable, f.Replacement),
			f.Line, f.Column, 1, ids.Findings[i])
	}

	for i, p := range r.Payloads {
		b.rule(rulePayload, "Hidden payload",
			"A run of invisible characters encoding data",
			"Invisible characters carrying an encoded message. A decoded payload is an attack regardless of the format of the file it sits in.",
			"Run untrace --fix on this file. Read the decoded text first: it names what was being tracked.")

		text := p.Text
		if !p.Printable {
			text = "not printable text"
		}
		b.add(r.Path, rulePayload, "error",
			fmt.Sprintf("Hidden payload decoding to %q, %s scheme", text, p.Scheme),
			p.Line, p.Column, p.Runes, ids.Payloads[i])
	}

	for i, m := range r.Mixed {
		b.rule(ruleMixedScript, "Mixed-script word",
			"A word mixing letters from more than one script",
			"A word combining scripts, such as a Cyrillic letter inside an otherwise Latin word. This is the confusable-letter attack described in UTS #39.",
			"Check whether the word is deliberate. untrace does not rewrite it by default, since replacing a letter guesses the script the author meant.")

		b.add(r.Path, ruleMixedScript, "warning",
			fmt.Sprintf("Mixed-script word %q, scripts: %s", m.Word, strings.Join(m.Scripts, ", ")),
			m.Line, m.Column, len([]rune(m.Word)), ids.Mixed[i])
	}
}

func sarifLevel(actionable bool) string {
	if actionable {
		return "warning"
	}
	return "note"
}

func outcome(actionable bool, replacement string) string {
	switch {
	case actionable && replacement == "":
		return "removed by --fix"
	case actionable:
		return fmt.Sprintf("replaced with %q by --fix", replacement)
	}
	return "reported only, not changed by --fix"
}

// Confusable letters share the typographic kind with punctuation.
func kindGuidance(f detect.Finding) string {
	switch {
	case f.Kind == "hidden":
		return "It is invisible when rendered and survives copy and paste."
	case f.Kind == "tag":
		return "Tag characters have no legitimate use outside subdivision flag emoji."
	case f.Kind == "typographic" && unicode.IsLetter(f.Rune):
		return "It is a letter from another script that looks like a Latin one."
	case f.Kind == "typographic":
		return "It is a non-ASCII form of ordinary punctuation."
	}
	return "What it means depends on the format of the file it sits in."
}

func (b *sarifBuilder) log(w io.Writer) (int, error) {
	ids := make([]string, 0, len(b.rules))
	for id := range b.rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rules := make([]sarifRule, 0, len(ids))
	for _, id := range ids {
		rules = append(rules, b.rules[id])
	}

	results := b.results
	dropped := 0
	if len(results) > sarifMaxResults {
		dropped = len(results) - sarifMaxResults
		results = results[:sarifMaxResults]
	}
	if results == nil {
		results = []sarifResult{}
	}

	out := sarifLog{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "untrace",
				Version:        version,
				InformationURI: sarifToolURI,
				Rules:          rules,
			}},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return dropped, enc.Encode(out)
}

func writeSarif(w io.Writer, reports []fileReport, ids []reportIdentities) error {
	b := newSarifBuilder()
	for i, r := range reports {
		if r.Error == "" {
			b.addReport(r, ids[i])
		}
	}

	dropped, err := b.log(w)
	if err != nil {
		return err
	}
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "untrace: %d result(s) beyond the SARIF limit of %d were not written\n",
			dropped, sarifMaxResults)
	}
	return nil
}
