package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/juriku/untrace/internal/config"
	"github.com/juriku/untrace/internal/detect"
	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/media"
	"github.com/juriku/untrace/internal/resolve"
)

func TestParseExcluded(t *testing.T) {
	cases := []struct {
		name  string
		in    []string
		want  []rune
		fails bool
	}{
		{"empty", nil, nil, false},
		{"codepoint", []string{"U+2014"}, []rune{0x2014}, false},
		{"lowercase prefix", []string{"u+2014"}, []rune{0x2014}, false},
		{"literal character", []string{string(rune(0x2014))}, []rune{0x2014}, false},
		{"several", []string{"U+2014", "U+200B"}, []rune{0x2014, 0x200B}, false},
		{"whitespace tolerated", []string{"  U+2014  "}, []rune{0x2014}, false},

		{"not hex", []string{"U+ZZZZ"}, nil, true},
		{"bare word", []string{"emdash"}, nil, true},
		{"out of range", []string{"U+110000"}, nil, true},
		{"negative", []string{"U+-1"}, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExcluded(tc.in)
			if tc.fails {
				if err == nil {
					t.Fatalf("expected an error for %v", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, r := range tc.want {
				if !got[r] {
					t.Errorf("U+%04X missing from the result", r)
				}
			}
			if len(got) != len(tc.want) {
				t.Errorf("got %d runes, want %d", len(got), len(tc.want))
			}
		})
	}
}

func TestMergeConfigLeavesExplicitFlagsAlone(t *testing.T) {
	opt := options{strict: false, noGitignore: false}
	cfg := &config.Config{Strict: true, NoGitignore: true}

	// The user passed --strict=false explicitly, so the config must not win.
	opt.mergeConfig(cfg, map[string]bool{"strict": true})

	if opt.strict {
		t.Error("config overrode an explicitly set flag")
	}
	if !opt.noGitignore {
		t.Error("config should have supplied the unset flag")
	}
}

func TestMergeConfigAppendsLists(t *testing.T) {
	opt := options{ignoreDirs: stringList{"fromflag"}}
	cfg := &config.Config{IgnoreDirs: []string{"fromconfig"}}

	opt.mergeConfig(cfg, map[string]bool{})

	if len(opt.ignoreDirs) != 2 {
		t.Errorf("ignoreDirs = %v, want both entries", opt.ignoreDirs)
	}
}

func TestMergeConfigPatternsOnlyWhenFlagAbsent(t *testing.T) {
	withFlag := options{patterns: stringList{"*.go"}}
	withFlag.mergeConfig(&config.Config{Patterns: []string{"*.md"}}, map[string]bool{})
	if len(withFlag.patterns) != 1 || withFlag.patterns[0] != "*.go" {
		t.Errorf("patterns = %v, want the flag to win outright", withFlag.patterns)
	}

	withoutFlag := options{}
	withoutFlag.mergeConfig(&config.Config{Patterns: []string{"*.md"}}, map[string]bool{})
	if len(withoutFlag.patterns) != 1 || withoutFlag.patterns[0] != "*.md" {
		t.Errorf("patterns = %v, want the config value", withoutFlag.patterns)
	}
}

func TestStringListAccumulates(t *testing.T) {
	var l stringList
	l.Set("a")
	l.Set("b")
	if l.String() != "a,b" {
		t.Errorf("String() = %q, want %q", l.String(), "a,b")
	}
}

func TestMixedActionDefaultsToReport(t *testing.T) {
	if got := mixedAction(config.Resolved{}); got != resolve.Report {
		t.Errorf("default = %v, want Report", got)
	}
	ignore := "ignore"
	if got := mixedAction(config.Resolved{MixedScript: &ignore}); got != resolve.Ignore {
		t.Errorf("configured = %v, want Ignore", got)
	}
	bogus := "nonsense"
	if got := mixedAction(config.Resolved{MixedScript: &bogus}); got != resolve.Report {
		t.Errorf("invalid action = %v, want the default", got)
	}
}

func TestPolicyForAppliesConfigOverride(t *testing.T) {
	ignore := "ignore"
	resolved := config.Resolved{
		Formats: map[string]config.FormatOverride{"source": {Typographic: &ignore}},
	}
	if got := policyFor(resolve.FormatSource, resolved).Typographic; got != resolve.Ignore {
		t.Errorf("typographic = %v, want Ignore", got)
	}
	// A format the config says nothing about keeps its built-in policy.
	if got := policyFor(resolve.FormatProse, resolved).Typographic; got != resolve.Clean {
		t.Errorf("prose typographic = %v, want Clean", got)
	}
}

func TestRelativeTo(t *testing.T) {
	if got := relativeTo("", "some/path.go"); got != "some/path.go" {
		t.Errorf("without a config the path is unchanged, got %q", got)
	}

	cfgPath := filepath.Join("/tmp", "repo", ".untrace.json")
	got := relativeTo(cfgPath, filepath.Join("/tmp", "repo", "src", "a.go"))
	if want := filepath.Join("src", "a.go"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSummarise(t *testing.T) {
	reports := []fileReport{
		{Findings: []detect.Finding{{Actionable: true, Applied: true}, {}}},
		{Metadata: []metaRecord{{Kind: "c2pa"}}},
		{},
		{Payloads: nil, Mixed: []detect.MixedWord{{Word: "x"}}, Suppressed: 2},
	}
	got := summarise(reports)

	if got.FilesScanned != 4 {
		t.Errorf("FilesScanned = %d, want 4", got.FilesScanned)
	}
	if got.FilesWithMarks != 2 {
		t.Errorf("FilesWithMarks = %d, want 2", got.FilesWithMarks)
	}
	if got.Detected != 2 || got.Actionable != 1 || got.Applied != 1 {
		t.Errorf("counts = %d/%d/%d, want 2/1/1", got.Detected, got.Actionable, got.Applied)
	}
	if got.Metadata != 1 || got.Mixed != 1 || got.Suppressed != 2 {
		t.Errorf("metadata/mixed/suppressed = %d/%d/%d, want 1/1/2",
			got.Metadata, got.Mixed, got.Suppressed)
	}
}

var pngHeader = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

func TestPassthroughClassifiesNonText(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"png", pngHeader, "png"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "jpeg"},
		{"pdf", []byte("%PDF-1.7\ntrailer\n"), "pdf"},
		{"unparseable zip", append([]byte("PK\x03\x04"), make([]byte, 64)...), "binary"},
		{"nul bytes", []byte{'a', 0x00, 'b'}, "binary"},
		{"unprintable high bytes", []byte{0xA0, 0xAD, 0xB4, 0xB7}, "binary"},
		{"plain text", []byte("hello\n"), ""},
		{"latin-1 prose", []byte("caf\xe9 au lait\n"), ""},
		{"utf-16 with bom", []byte{0xFF, 0xFE, 'h', 0x00}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			format, _, ok := passthrough(tc.in)
			if tc.want == "" {
				if ok {
					t.Errorf("passed through as %q, want the text path", format)
				}
				return
			}
			if !ok {
				t.Fatalf("not passed through, want %q", tc.want)
			}
			if format != tc.want {
				t.Errorf("format = %q, want %q", format, tc.want)
			}
		})
	}
}

// Decoded as latin-1, 0xA0 and 0xAD are a non-breaking space and a soft hyphen,
// which cleaning would rewrite and delete.
func TestStdinLeavesNonTextByteIdentical(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
	}{
		{"raw high bytes", []byte{0xA0, 0xAD, 0xB4, 0xB7}},
		{"png", append(pngHeader, make([]byte, 32)...)},
		{"pdf", []byte("%PDF-1.7\n" + string(rune(0x2014)) + "\ntrailer\n")},
		{"nul bytes", []byte{'a', 0x00, 0xA0, 'b'}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stdinRoundTrip(t, tc.in, options{fix: true})
			if !bytes.Equal(got, tc.in) {
				t.Errorf("stdout = % x, want % x", got, tc.in)
			}
		})
	}
}

// An undecodable document must still reach stdout: "untrace --stdin < a > b"
// would otherwise truncate b to nothing.
func TestStdinEmitsUndecodableInputUnchanged(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
	}{
		{"odd byte count", []byte{0xFF, 0xFE, 'h', 0x00, 'i'}},
		{"unpaired high surrogate", []byte{0xFF, 0xFE, 0x00, 0xD8, 'A', 0x00}},
		{"invalid utf-32 scalar", []byte{0xFF, 0xFE, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stdinRoundTrip(t, tc.in, options{fix: true})
			if !bytes.Equal(got, tc.in) {
				t.Errorf("stdout = % x, want % x", got, tc.in)
			}
		})
	}
}

func TestStdinStillCleansText(t *testing.T) {
	in := []byte("smart " + string(rune(0x201C)) + "quotes" + string(rune(0x201D)))
	want := []byte(`smart "quotes"`)

	if got := stdinRoundTrip(t, in, options{fix: true}); !bytes.Equal(got, want) {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func stdinRoundTrip(t *testing.T, in []byte, opt options) []byte {
	t.Helper()
	dir := t.TempDir()

	inPath := filepath.Join(dir, "in")
	if err := os.WriteFile(inPath, in, 0o600); err != nil {
		t.Fatal(err)
	}
	stdin, err := os.Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	outPath := filepath.Join(dir, "out")
	stdout, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdin, stdout
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()

	runStdin(opt, markers.Options{Typographic: true, IVS: true}, &config.Config{})
	stdout.Close()

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestStripModeParsing(t *testing.T) {
	cases := []struct {
		in    string
		want  stripMode
		fails bool
	}{
		{in: "true", want: stripAI},
		{in: "all", want: stripAll},
		{in: "false", want: stripNone},
		{in: "yes", fails: true},
		{in: "ai", fails: true},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			var m stripMode
			err := m.Set(tc.in)
			if tc.fails {
				if err == nil {
					t.Fatalf("Set(%q) succeeded, want an error", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if m != tc.want {
				t.Errorf("Set(%q) = %v, want %v", tc.in, m, tc.want)
			}
		})
	}
}

func TestStripMetadataDoesNotConsumeTheScanTarget(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want stripMode
	}{
		{"bare", []string{"--strip-metadata", "."}, stripAI},
		{"explicit all", []string{"--strip-metadata=all", "."}, stripAll},
		{"explicit false", []string{"--strip-metadata=false", "."}, stripNone},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m stripMode
			fs := flag.NewFlagSet("untrace", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			fs.Var(&m, "strip-metadata", "")

			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if m != tc.want {
				t.Errorf("strip-metadata = %v, want %v", m, tc.want)
			}
			if got := fs.Args(); len(got) != 1 || got[0] != "." {
				t.Errorf("scan targets = %v, want [.]", got)
			}
		})
	}
}

// Every detector must be built the same way whatever route the file took: a
// container that resolves to FormatSource once produced exit 0 where the same
// text in a .txt produced exit 1.
func TestDetectorIsIdenticalOnEveryRoute(t *testing.T) {
	opt := options{fix: true, strict: true, fixHomoglyphs: true}
	mo := markers.Options{Typographic: true, IVS: true}

	text := newDetectorText()
	plain := opt.detector("a.go", mo, resolve.FormatSource, config.Resolved{}, text)
	container := opt.detector("a.go", mo, resolve.FormatSource, config.Resolved{}, text)
	container.Clean = false

	if plain.FixHomoglyphs != container.FixHomoglyphs {
		t.Error("FixHomoglyphs differs between routes")
	}
	if plain.Strict != container.Strict {
		t.Error("Strict differs between routes")
	}
	if plain.MixedScript != container.MixedScript {
		t.Error("MixedScript differs between routes")
	}
	if container.Clean {
		t.Error("containers must never be rewritten")
	}

	if !plain.Run(text).Findings[0].Actionable {
		t.Error("homoglyph not actionable with --fix-homoglyphs")
	}
}

func newDetectorText() string {
	return "login at p" + string(rune(0x0430)) + "ypal"
}

func TestShouldStripSparesNonAIRecords(t *testing.T) {
	records := []struct {
		name    string
		finding media.Finding
		wantAI  bool
	}{
		{"named generator", media.Finding{Kind: media.PNGText, Value: "Adobe Firefly 3"}, true},
		{"camera firmware", media.Finding{Kind: media.EXIF, Value: "Leica M11"}, false},
		{"unparsed credential", media.Finding{Kind: media.C2PA, Bytes: 18432}, false},
		{"empty comment", media.Finding{Kind: media.JPEGComment}, false},
	}

	for _, r := range records {
		t.Run(r.name, func(t *testing.T) {
			if got := stripAI.shouldStrip(r.finding); got != r.wantAI {
				t.Errorf("stripAI = %v, want %v", got, r.wantAI)
			}
			if !stripAll.shouldStrip(r.finding) {
				t.Error("stripAll left a record behind")
			}
			if stripNone.shouldStrip(r.finding) {
				t.Error("stripNone stripped a record")
			}
		})
	}
}

func TestAssessProvenance(t *testing.T) {
	t.Run("named generator raises confidence", func(t *testing.T) {
		r := fileReport{Metadata: []metaRecord{
			{Kind: "png-text", Label: "Software", Value: "Adobe Firefly 3"},
		}}
		r.assessProvenance()
		if r.Confidence != "likely" {
			t.Errorf("confidence = %q, want likely", r.Confidence)
		}
	})

	t.Run("a credential alone does not", func(t *testing.T) {
		// Cameras sign authentic photographs with C2PA.
		r := fileReport{Metadata: []metaRecord{
			{Kind: string(media.C2PA)},
			{Kind: "png-text", Label: "Software", Value: "Leica M11"},
		}}
		r.assessProvenance()
		if r.Confidence == "likely" {
			t.Error("a camera credential was reported as AI")
		}
		if len(r.Signals) != 1 {
			t.Errorf("signals = %v, want just the credential", r.Signals)
		}
	})

	t.Run("no metadata means no signals", func(t *testing.T) {
		r := fileReport{}
		r.assessProvenance()
		if len(r.Signals) != 0 || r.Confidence != "" {
			t.Errorf("got %v / %q", r.Signals, r.Confidence)
		}
	})
}
