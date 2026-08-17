package resolve

import (
	"path/filepath"
	"strings"

	"github.com/juriku/untrace/internal/markers"
)

type Format int

const (
	FormatSource Format = iota
	FormatProse
	FormatData
	FormatMarkup
	FormatNotebook
	FormatOffice
	FormatPDF
	FormatLog
)

func (f Format) String() string {
	switch f {
	case FormatSource:
		return "source"
	case FormatProse:
		return "prose"
	case FormatData:
		return "data"
	case FormatMarkup:
		return "markup"
	case FormatNotebook:
		return "notebook"
	case FormatOffice:
		return "office"
	case FormatPDF:
		return "pdf"
	case FormatLog:
		return "log"
	}
	return "source"
}

type Action int

const (
	Ignore Action = iota
	Report
	Clean
)

type Policy struct {
	Format      Format
	Hidden      Action
	Typographic Action
	IVS         Action
	Tag         Action
	// PerRune overrides the kind-level action for characters the format uses
	// legitimately, such as U+00A0 in HTML.
	PerRune    map[rune]Action
	configured struct{ hidden, typographic, ivs *Action }
}

func ParseAction(s string) (Action, bool) {
	switch s {
	case "ignore":
		return Ignore, true
	case "report":
		return Report, true
	case "clean":
		return Clean, true
	}
	return Ignore, false
}

// Override replaces the action for one or more marker kinds. A nil field leaves
// that kind as the format resolved it.
func (p Policy) Override(hidden, typographic, ivs *Action) Policy {
	if hidden != nil {
		p.Hidden = *hidden
	}
	if typographic != nil {
		p.Typographic = *typographic
	}
	if ivs != nil {
		p.IVS = *ivs
	}
	p.configured.hidden = hidden
	p.configured.typographic = typographic
	p.configured.ivs = ivs
	return p
}

func (p Policy) For(m markers.Marker) Action {
	if a, ok := p.PerRune[m.Rune]; ok {
		return a
	}
	switch m.Kind {
	case markers.Hidden:
		return p.Hidden
	case markers.Typographic:
		return p.Typographic
	case markers.IdeographicVS:
		return p.IVS
	case markers.Tag:
		return p.Tag
	}
	return Report
}

// Only the exceptions are listed; anything unrecognised defaults to FormatSource,
// which carries the strictest policy.
var extFormats = map[string]Format{
	".md": FormatProse, ".markdown": FormatProse, ".txt": FormatProse, ".rst": FormatProse,
	".adoc": FormatProse, ".asciidoc": FormatProse, ".org": FormatProse, ".tex": FormatProse,

	".log": FormatLog,

	".json": FormatData, ".jsonc": FormatData, ".yaml": FormatData, ".yml": FormatData,
	".toml": FormatData, ".ini": FormatData, ".cfg": FormatData, ".conf": FormatData,
	".csv": FormatData, ".tsv": FormatData, ".env": FormatData, ".properties": FormatData,

	".html": FormatMarkup, ".htm": FormatMarkup, ".xhtml": FormatMarkup, ".xml": FormatMarkup,
	".svg": FormatMarkup, ".vue": FormatMarkup, ".svelte": FormatMarkup, ".css": FormatMarkup,
	".scss": FormatMarkup, ".less": FormatMarkup,

	".ipynb": FormatNotebook,

	".docx": FormatOffice, ".doc": FormatOffice, ".odt": FormatOffice, ".rtf": FormatOffice,
	".xlsx": FormatOffice, ".pptx": FormatOffice, ".pages": FormatOffice,

	".pdf": FormatPDF,
}

// Extensionless files that are conventionally prose rather than source.
var baseFormats = map[string]Format{
	"README": FormatProse, "LICENSE": FormatProse, "LICENCE": FormatProse,
	"CHANGELOG": FormatProse, "CHANGES": FormatProse, "NOTICE": FormatProse,
	"AUTHORS": FormatProse, "CONTRIBUTORS": FormatProse, "CONTRIBUTING": FormatProse,
	"COPYING": FormatProse, "TODO": FormatProse,
}

func DetectFormat(path string) Format {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		if f, ok := extFormats[strings.ToLower(ext)]; ok {
			return f
		}
		return FormatSource
	}
	if f, ok := baseFormats[strings.ToUpper(base)]; ok {
		return f
	}
	return FormatSource
}

// Office suites insert curly quotes, em dashes and non-breaking spaces
// mechanically, so in those formats their presence carries no authorship signal.
func PolicyFor(f Format) Policy {
	p := Policy{Format: f, Hidden: Clean, Typographic: Clean, IVS: Clean, Tag: Clean}

	switch f {
	case FormatLog:
		p.Hidden, p.Typographic, p.IVS, p.Tag = Ignore, Ignore, Ignore, Ignore

	case FormatOffice:
		p.Typographic = Ignore
		p.PerRune = map[rune]Action{}
		for r := range markers.WordCommonRunes() {
			p.PerRune[r] = Ignore
		}
	}

	return p
}
