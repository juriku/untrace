package resolve

import (
	"testing"

	"github.com/juriku/untrace/internal/markers"
)

func TestDetectFormat(t *testing.T) {
	cases := map[string]Format{
		"main.go":            FormatSource,
		"script.py":          FormatSource,
		"deeply/nested/a.rs": FormatSource,
		"unknown.xyz":        FormatSource,
		"noextension":        FormatSource,

		"README.md":    FormatProse,
		"notes.txt":    FormatProse,
		"paper.tex":    FormatProse,
		"README":       FormatProse,
		"LICENSE":      FormatProse,
		"CHANGELOG":    FormatProse,
		"contributing": FormatProse,
		"data/COPYING": FormatProse,

		"config.json": FormatData,
		"a.yaml":      FormatData,
		".env":        FormatData,

		"index.html": FormatMarkup,
		"icon.svg":   FormatMarkup,
		"style.css":  FormatMarkup,

		"notebook.ipynb": FormatNotebook,
		"report.docx":    FormatOffice,
		"sheet.xlsx":     FormatOffice,
		"paper.pdf":      FormatPDF,
		"server.log":     FormatLog,

		"MAIN.GO":   FormatSource,
		"Notes.MD":  FormatProse,
		"readme.Md": FormatProse,
	}

	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			if got := DetectFormat(path); got != want {
				t.Errorf("DetectFormat(%q) = %v, want %v", path, got, want)
			}
		})
	}
}

func TestFormatStringRoundTrips(t *testing.T) {
	all := []Format{
		FormatSource, FormatProse, FormatData, FormatMarkup,
		FormatNotebook, FormatOffice, FormatPDF, FormatLog,
	}
	seen := map[string]bool{}
	for _, f := range all {
		name := f.String()
		if name == "" {
			t.Errorf("format %d has no name", f)
		}
		if seen[name] {
			t.Errorf("duplicate format name %q", name)
		}
		seen[name] = true
	}
}

func TestParseAction(t *testing.T) {
	cases := map[string]struct {
		want Action
		ok   bool
	}{
		"ignore": {Ignore, true},
		"report": {Report, true},
		"clean":  {Clean, true},
		"delete": {Ignore, false},
		"":       {Ignore, false},
		"CLEAN":  {Ignore, false},
	}
	for in, tc := range cases {
		t.Run(in, func(t *testing.T) {
			got, ok := ParseAction(in)
			if ok != tc.ok || got != tc.want {
				t.Errorf("ParseAction(%q) = %v,%v want %v,%v", in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// Only Office and log deviate. A third exception is a policy change, not a
// refactor, so this pins the whole table.
func TestPolicyTable(t *testing.T) {
	type row struct{ hidden, typographic, ivs, tag Action }
	want := map[Format]row{
		FormatSource:   {Clean, Clean, Clean, Clean},
		FormatData:     {Clean, Clean, Clean, Clean},
		FormatProse:    {Clean, Clean, Clean, Clean},
		FormatNotebook: {Clean, Clean, Clean, Clean},
		FormatMarkup:   {Clean, Clean, Clean, Clean},
		FormatPDF:      {Clean, Clean, Clean, Clean},
		FormatOffice:   {Clean, Clean, Clean, Clean},
		FormatLog:      {Ignore, Ignore, Ignore, Ignore},
	}

	for format, w := range want {
		t.Run(format.String(), func(t *testing.T) {
			p := PolicyFor(format)
			if p.Hidden != w.hidden || p.Typographic != w.typographic ||
				p.IVS != w.ivs || p.Tag != w.tag {
				t.Errorf("policy = %v/%v/%v/%v, want %v/%v/%v/%v",
					p.Hidden, p.Typographic, p.IVS, p.Tag,
					w.hidden, w.typographic, w.ivs, w.tag)
			}
		})
	}
}

func TestOnlyOfficeHasPerRuneExceptions(t *testing.T) {
	for _, f := range []Format{
		FormatSource, FormatProse, FormatData, FormatMarkup,
		FormatNotebook, FormatPDF, FormatLog,
	} {
		if n := len(PolicyFor(f).PerRune); n != 0 {
			t.Errorf("%v has %d per-rune exceptions, want 0", f, n)
		}
	}
	if n := len(PolicyFor(FormatOffice).PerRune); n == 0 {
		t.Error("office should exempt the characters Word inserts")
	}
}

func TestPolicyForResolvesPerRuneBeforeKind(t *testing.T) {
	p := PolicyFor(FormatOffice)
	m, ok := markers.Lookup(0x00A0, markers.Options{Typographic: true})
	if !ok {
		t.Fatal("nbsp should be a marker")
	}
	// Nbsp is a hidden marker, which office cleans, but Word inserts it, so the
	// per-rune exemption has to win.
	if got := p.For(m); got != Ignore {
		t.Errorf("office nbsp = %v, want Ignore", got)
	}
}

func TestOverrideLeavesNilFieldsAlone(t *testing.T) {
	base := PolicyFor(FormatSource)
	clean := Clean
	got := base.Override(nil, &clean, nil)

	if got.Hidden != base.Hidden || got.IVS != base.IVS {
		t.Error("nil fields should not change")
	}
	if got.Typographic != Clean {
		t.Errorf("typographic = %v, want Clean", got.Typographic)
	}
}
