// Package untrace finds and removes invisible characters, hidden payloads and
// AI provenance metadata in text.
//
// The command line tool in cmd/untrace is the primary interface. This package
// exposes the scanning pipeline for callers embedding it in another program.
package untrace

import (
	"errors"

	"github.com/juriku/untrace/internal/config"
	"github.com/juriku/untrace/internal/detect"
	"github.com/juriku/untrace/internal/docmeta"
	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/media"
	"github.com/juriku/untrace/internal/resolve"
	"github.com/juriku/untrace/internal/textfile"
)

type Finding = detect.Finding

type Payload = detect.Payload

type MixedWord = detect.MixedWord

type Options struct {
	Fix           bool
	Strict        bool
	FixHomoglyphs bool
	Exclude       []rune
}

type Result struct {
	Text       string
	Findings   []Finding
	Payloads   []Payload
	Mixed      []MixedWord
	Changed    bool
	Suppressed int
	Format     string
}

// ScanText resolves format and region rules as though the content lived at
// name, without reading anything from disk. An empty name is treated as source,
// which is the strictest policy.
func ScanText(text, name string, opt Options) Result {
	format := resolve.FormatSource
	if name != "" {
		format = resolve.DetectFormat(name)
	}

	excluded := map[rune]bool{}
	for _, r := range opt.Exclude {
		excluded[r] = true
	}

	d := &detect.Detector{
		Markers: markers.Options{
			Typographic: true,
			IVS:         true,
			Excluded:    excluded,
		},
		Policy:        resolve.PolicyFor(format),
		Clean:         opt.Fix,
		Strict:        opt.Strict,
		MixedScript:   resolve.Report,
		FixHomoglyphs: opt.FixHomoglyphs,
		Regions:       resolve.RegionsFor(format, text),
		Rewrite:       resolve.RewriterFor(name, text),
	}

	res := d.Run(text)
	return Result{
		Text:       res.Text,
		Findings:   res.Findings,
		Payloads:   res.Payloads,
		Mixed:      res.Mixed,
		Changed:    res.Changed,
		Suppressed: res.Suppressed,
		Format:     format.String(),
	}
}

var ErrNotText = errors.New("untrace: not a text document")

// ScanBytes decodes data before scanning and re-encodes the result, so a
// latin-1 or UTF-16 document keeps its encoding.
func ScanBytes(data []byte, name string, opt Options) (Result, []byte, error) {
	if media.Detect(data) != media.FormatUnknown ||
		docmeta.Detect(data) != docmeta.FormatUnknown ||
		textfile.IsBinary(data) {
		return Result{}, nil, ErrNotText
	}

	dec, err := textfile.Decode(data)
	if err != nil {
		return Result{}, nil, err
	}

	res := ScanText(dec.Text, name, opt)
	if !res.Changed {
		return res, data, nil
	}

	out, err := textfile.Encode(res.Text, dec)
	if err != nil {
		return res, nil, err
	}
	return res, out, nil
}

func (r Result) Actionable() bool {
	if len(r.Payloads) > 0 || len(r.Mixed) > 0 {
		return true
	}
	for _, f := range r.Findings {
		if f.Actionable {
			return true
		}
	}
	return false
}

type Config = config.Config

// LoadConfig walks up from target to the repository root. A missing file is not
// an error.
func LoadConfig(target string) (*Config, error) {
	return config.Find(target)
}
