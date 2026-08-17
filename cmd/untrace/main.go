package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/juriku/untrace/internal/config"
	"github.com/juriku/untrace/internal/detect"
	"github.com/juriku/untrace/internal/docmeta"
	"github.com/juriku/untrace/internal/markers"
	"github.com/juriku/untrace/internal/media"
	"github.com/juriku/untrace/internal/provenance"
	"github.com/juriku/untrace/internal/resolve"
	"github.com/juriku/untrace/internal/scan"
	"github.com/juriku/untrace/internal/textfile"
)

var version = "dev"

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

type stripMode int

const (
	stripNone stripMode = iota
	stripAI
	stripAll
)

func (m *stripMode) String() string {
	if m != nil && *m == stripAll {
		return "all"
	}
	return "false"
}

func (m *stripMode) Set(v string) error {
	switch v {
	case "false":
		*m = stripNone
	case "true":
		*m = stripAI
	case "all":
		*m = stripAll
	default:
		return fmt.Errorf("invalid value %q for --strip-metadata, want \"all\"", v)
	}
	return nil
}

// Without this the parser takes the next argument as the value, so
// "untrace --strip-metadata ." would swallow the path.
func (m *stripMode) IsBoolFlag() bool { return true }

func (m stripMode) shouldStrip(f media.Finding) bool {
	switch m {
	case stripAll:
		return true
	case stripAI:
		_, ai := provenance.Generator(f.Value)
		return ai
	}
	return false
}

type options struct {
	fix              bool
	stdin            bool
	asJSON           bool
	failOnFind       bool
	quiet            bool
	noColor          bool
	noRecursive      bool
	noGitignore      bool
	noDefaultIgnores bool
	strict           bool
	fixHomoglyphs    bool
	stdinName        string
	stripMetadata    stripMode
	showVersion      bool
	configPath       string
	ignoreDirs       stringList
	patterns         stringList
	excludeChars     stringList
}

// Config supplies defaults; an explicitly passed flag always wins.
func (o *options) mergeConfig(c *config.Config, set map[string]bool) {
	if !set["strict"] {
		o.strict = o.strict || c.Strict
	}
	if !set["no-gitignore"] {
		o.noGitignore = o.noGitignore || c.NoGitignore
	}
	if !set["no-default-ignores"] {
		o.noDefaultIgnores = o.noDefaultIgnores || c.NoDefaultIgnores
	}
	o.ignoreDirs = append(o.ignoreDirs, c.IgnoreDirs...)
	o.excludeChars = append(o.excludeChars, c.Exclude...)
	if len(o.patterns) == 0 {
		o.patterns = append(o.patterns, c.Patterns...)
	}
}

func policyFor(format resolve.Format, r config.Resolved) resolve.Policy {
	p := resolve.PolicyFor(format)
	o, ok := r.Formats[format.String()]
	if !ok {
		return p
	}
	return p.Override(parseAction(o.Hidden), parseAction(o.Typographic), parseAction(o.IVS))
}

func (o options) detector(mo markers.Options, format resolve.Format,
	resolved config.Resolved, text string) *detect.Detector {

	return &detect.Detector{
		Markers:       mo,
		Policy:        policyFor(format, resolved),
		Clean:         o.fix,
		Strict:        o.strict,
		MixedScript:   mixedAction(resolved),
		FixHomoglyphs: o.fixHomoglyphs,
		Regions:       resolve.RegionsFor(format, text),
	}
}

// Confusable letters default to being reported.
func mixedAction(r config.Resolved) resolve.Action {
	if a := parseAction(r.MixedScript); a != nil {
		return *a
	}
	return resolve.Report
}

func parseAction(s *string) *resolve.Action {
	if s == nil {
		return nil
	}
	a, ok := resolve.ParseAction(*s)
	if !ok {
		return nil
	}
	return &a
}

// metaRecord unifies image and document metadata for reporting.
type metaRecord struct {
	Kind     string `json:"kind"`
	Label    string `json:"label,omitempty"`
	Value    string `json:"value,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Stripped bool   `json:"stripped,omitempty"`
}

type fileReport struct {
	Path       string                `json:"path"`
	Format     string                `json:"format"`
	Encoding   string                `json:"encoding"`
	Findings   []detect.Finding      `json:"findings"`
	Payloads   []detect.Payload      `json:"payloads,omitempty"`
	Mixed      []detect.MixedWord    `json:"mixed_script,omitempty"`
	Metadata   []metaRecord          `json:"metadata,omitempty"`
	Signals    []provenance.Signal   `json:"provenance,omitempty"`
	Confidence provenance.Confidence `json:"confidence,omitempty"`
	Suppressed int                   `json:"suppressed,omitempty"`
	Error      string                `json:"error,omitempty"`
}

type jsonOutput struct {
	Version string       `json:"version"`
	Files   []fileReport `json:"files"`
	Summary summary      `json:"summary"`
}

type summary struct {
	FilesScanned   int `json:"files_scanned"`
	FilesWithMarks int `json:"files_with_markers"`
	Detected       int `json:"markers_detected"`
	Actionable     int `json:"markers_actionable"`
	Applied        int `json:"markers_applied"`
	Payloads       int `json:"payloads_decoded"`
	Metadata       int `json:"metadata_records"`
	Suppressed     int `json:"suppressed"`
	Mixed          int `json:"mixed_script_words"`
}

func main() { os.Exit(run()) }

// Subcommands are dispatched before flag parsing so their own flag sets own the
// remaining arguments.
func dispatch(args []string) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "install-filter":
		return runInstallFilter(args[1:]), true
	}
	return 0, false
}

func run() int {
	if code, handled := dispatch(os.Args[1:]); handled {
		return code
	}

	var opt options
	fs := flag.NewFlagSet("untrace", flag.ContinueOnError)
	fs.Usage = func() { usage(fs) }

	fs.BoolVar(&opt.fix, "fix", false, "rewrite files, removing or normalising what is found")
	fs.BoolVar(&opt.fix, "c", false, "alias for --fix")
	fs.BoolVar(&opt.stdin, "stdin", false, "read the document from standard input")
	fs.StringVar(&opt.stdinName, "stdin-name", "",
		"with --stdin, resolve format, config and regions as if the content were this path")
	fs.BoolVar(&opt.asJSON, "json", false,
		"emit findings as JSON, on stdout or on stderr in --stdin mode")
	fs.BoolVar(&opt.failOnFind, "fail", false,
		"exit 1 if anything actionable is found (payloads, mixed script, or characters --fix would change)")
	fs.BoolVar(&opt.quiet, "quiet", false, "suppress the per-file report")
	fs.BoolVar(&opt.noColor, "no-color", false, "disable coloured output")
	fs.BoolVar(&opt.noRecursive, "no-recursive", false, "do not descend into subdirectories")
	fs.BoolVar(&opt.noGitignore, "no-gitignore", false, "do not honour .gitignore files")
	fs.BoolVar(&opt.noDefaultIgnores, "no-default-ignores", false,
		"descend into dependency and cache directories")
	fs.Var(&opt.stripMetadata, "strip-metadata",
		"with --fix, remove image metadata that names an AI generator; =all removes every record")
	fs.BoolVar(&opt.strict, "strict", false,
		"report every marker, including ones doing a legitimate job such as emoji joiners")
	fs.BoolVar(&opt.fixHomoglyphs, "fix-homoglyphs", false,
		"with --fix, also rewrite confusable letters to Latin; off by default, since it rewrites the author's script")
	fs.BoolVar(&opt.showVersion, "version", false, "print the version and exit")
	fs.StringVar(&opt.configPath, "config", "", "path to a config file, overriding discovery")
	fs.Var(&opt.ignoreDirs, "ignore-dir", "additional directory name to skip (repeatable)")
	fs.Var(&opt.patterns, "pattern", "only scan files matching this glob (repeatable)")
	fs.Var(&opt.excludeChars, "exclude-char", "codepoint to ignore, as U+XXXX or a literal (repeatable)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if opt.showVersion {
		fmt.Println("untrace", version)
		return 0
	}

	targets := fs.Args()
	if len(targets) == 0 {
		targets = []string{"."}
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	cfg, err := loadConfig(opt.configPath, targets[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "untrace:", err)
		return 2
	}
	opt.mergeConfig(cfg, set)

	excluded, err := parseExcluded(opt.excludeChars)
	if err != nil {
		fmt.Fprintln(os.Stderr, "untrace:", err)
		return 2
	}

	markerOpts := markers.Options{Typographic: true, IVS: true, Excluded: excluded}

	// In stdin mode stdout carries the document, so the report moves to stderr.
	reportTo := os.Stdout
	if opt.stdin {
		reportTo = os.Stderr
	}
	color := !opt.noColor && os.Getenv("NO_COLOR") == "" && isTTY(reportTo)

	var reports []fileReport
	if opt.stdin {
		reports = append(reports, runStdin(opt, markerOpts, cfg))
	} else {
		reports = runPaths(targets, opt, markerOpts, cfg)
	}

	sum := summarise(reports)

	if opt.asJSON {
		enc := json.NewEncoder(reportTo)
		enc.SetIndent("", "  ")
		if err := enc.Encode(jsonOutput{Version: version, Files: reports, Summary: sum}); err != nil {
			fmt.Fprintln(os.Stderr, "untrace:", err)
			return 2
		}
	} else if !opt.quiet {
		printReport(reportTo, reports, sum, opt, color)
	}

	for _, r := range reports {
		if r.Error != "" {
			return 2
		}
	}
	// Payloads and mixed-script words are actionable too, and are counted from
	// their own slices rather than from Findings.
	if opt.failOnFind && sum.Actionable+sum.Payloads+sum.Mixed > 0 {
		return 1
	}
	return 0
}

func runStdin(opt options, mo markers.Options, cfg *config.Config) fileReport {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fileReport{Path: "-", Error: err.Error()}
	}

	// Git runs a clean filter over every staged file, binaries included, and
	// replaces the file with whatever the filter writes.
	if format, encoding, ok := passthrough(data); ok {
		os.Stdout.Write(data)
		return fileReport{Path: "-", Format: format, Encoding: encoding}
	}

	dec, err := textfile.Decode(data)
	if err != nil {
		// The document leaves on stdout even when it cannot be read, or a
		// redirect would truncate the file being scrubbed to nothing.
		os.Stdout.Write(data)
		return fileReport{Path: "-", Error: err.Error()}
	}

	// Without a name, stdin content has no extension to classify and no path to
	// match overrides against, so the strictest policy applies.
	name := "-"
	format := resolve.FormatSource
	if opt.stdinName != "" {
		name = opt.stdinName
		format = resolve.DetectFormat(name)
	}

	resolved := cfg.For(relativeTo(cfg.Path, name))
	d := opt.detector(mo, format, resolved, dec.Text)
	res := d.Run(dec.Text)

	out := dec.Text
	if opt.fix {
		out = res.Text
	}
	encoded, err := textfile.Encode(out, dec)
	if err != nil {
		return fileReport{Path: "-", Error: err.Error()}
	}
	os.Stdout.Write(encoded)

	return fileReport{
		Path:       name,
		Format:     format.String(),
		Encoding:   dec.Enc.String(),
		Findings:   res.Findings,
		Payloads:   res.Payloads,
		Mixed:      res.Mixed,
		Suppressed: res.Suppressed,
	}
}

func passthrough(data []byte) (format, encoding string, ok bool) {
	if f := media.Detect(data); f != media.FormatUnknown {
		return string(f), "binary", true
	}
	if f := docmeta.Detect(data); f != docmeta.FormatUnknown {
		return string(f), "container", true
	}
	if textfile.IsBinary(data) {
		return "binary", "binary", true
	}
	return "", "", false
}

func runPaths(targets []string, opt options, mo markers.Options, cfg *config.Config) []fileReport {
	ignored := map[string]bool{".git": true}
	if !opt.noDefaultIgnores {
		for d := range markers.DefaultIgnoredDirs {
			ignored[d] = true
		}
	}
	for _, d := range opt.ignoreDirs {
		ignored[d] = true
	}

	scanOpts := scan.Options{
		Recursive:    !opt.noRecursive,
		IgnoredDirs:  ignored,
		UseGitignore: !opt.noGitignore,
		Patterns:     opt.patterns,
	}

	var reports []fileReport
	for _, target := range targets {
		files, err := scan.Files(target, scanOpts)
		if err != nil {
			reports = append(reports, fileReport{Path: target, Error: err.Error()})
			continue
		}
		for _, path := range files {
			reports = append(reports, processFile(path, opt, mo, cfg))
		}
	}
	return reports
}

func processFile(path string, opt options, mo markers.Options, cfg *config.Config) fileReport {
	format := resolve.DetectFormat(path)
	rep := fileReport{Path: path, Format: format.String()}

	data, err := os.ReadFile(path)
	if err != nil {
		rep.Error = err.Error()
		return rep
	}

	if media.Detect(data) != media.FormatUnknown {
		return processMedia(path, data, opt)
	}
	if docmeta.Detect(data) != docmeta.FormatUnknown {
		return processDocument(path, data, opt, mo, cfg, format)
	}

	dec, err := textfile.Decode(data)
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	rep.Encoding = dec.Enc.String()

	resolved := cfg.For(relativeTo(cfg.Path, path))
	d := opt.detector(mo, format, resolved, dec.Text)
	res := d.Run(dec.Text)
	rep.Findings = res.Findings
	rep.Payloads = res.Payloads
	rep.Mixed = res.Mixed
	rep.Suppressed = res.Suppressed

	if opt.fix && res.Changed {
		encoded, err := textfile.Encode(res.Text, dec)
		if err != nil {
			rep.Error = err.Error()
			return rep
		}
		if err := textfile.WriteAtomic(path, encoded); err != nil {
			rep.Error = err.Error()
		}
	}
	return rep
}

func summarise(reports []fileReport) summary {
	var s summary
	for _, r := range reports {
		s.FilesScanned++
		if len(r.Findings) > 0 || len(r.Metadata) > 0 {
			s.FilesWithMarks++
		}
		for _, f := range r.Findings {
			s.Detected++
			if f.Actionable {
				s.Actionable++
			}
			if f.Applied {
				s.Applied++
			}
		}
		s.Payloads += len(r.Payloads)
		s.Mixed += len(r.Mixed)
		s.Metadata += len(r.Metadata)
		s.Suppressed += r.Suppressed
	}
	return s
}

func parseExcluded(vals []string) (map[rune]bool, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	out := make(map[rune]bool, len(vals))
	for _, raw := range vals {
		tok := strings.TrimSpace(raw)
		if r := []rune(tok); len(r) == 1 {
			out[r[0]] = true
			continue
		}
		hex := strings.TrimPrefix(strings.TrimPrefix(tok, "U+"), "u+")
		v, err := strconv.ParseInt(hex, 16, 32)
		if err != nil || v < 0 || v > 0x10FFFF {
			return nil, fmt.Errorf("invalid --exclude-char %q: use U+XXXX or a single character", raw)
		}
		out[rune(v)] = true
	}
	return out, nil
}

func isTTY(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func loadConfig(explicit, target string) (*config.Config, error) {
	if explicit != "" {
		return config.Load(explicit)
	}
	return config.Find(target)
}

// Override globs are written relative to the config file, so paths are made
// relative to it before matching.
func relativeTo(configPath, path string) string {
	if configPath == "" {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	// On Windows a rooted path such as \repo\x carries no volume, and Rel refuses
	// to relate it to one that does.
	base, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return path
	}
	return rel
}

// Images carry provenance metadata rather than hidden characters. Removal is
// irreversible and unrelated to fixing text, so it needs --strip-metadata as
// well as --fix.
func processMedia(path string, data []byte, opt options) fileReport {
	report := media.Inspect(data)
	rep := fileReport{
		Path:     path,
		Format:   string(report.Format),
		Encoding: "binary",
	}
	for _, f := range report.Findings {
		rep.Metadata = append(rep.Metadata, metaRecord{
			Kind: string(f.Kind), Label: f.Label, Value: f.Value, Bytes: f.Bytes,
		})
	}
	rep.assessProvenance()
	if !opt.fix || opt.stripMetadata == stripNone || len(report.Findings) == 0 {
		return rep
	}

	for i, f := range report.Findings {
		rep.Metadata[i].Stripped = opt.stripMetadata.shouldStrip(f)
	}
	out, changed, err := media.Strip(data, opt.stripMetadata.shouldStrip)
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	if changed {
		if err := textfile.WriteAtomic(path, out); err != nil {
			rep.Error = err.Error()
		}
	}
	return rep
}

// Office files and PDFs are containers. Their body text is scanned under the
// format policy, which is what lets a Word document keep its em dashes while
// still being checked for hidden characters. Rewriting them would mean
// repacking the container, so they are reported and never fixed.
func processDocument(path string, data []byte, opt options, mo markers.Options,
	cfg *config.Config, format resolve.Format) fileReport {

	doc, err := docmeta.Read(data)
	rep := fileReport{Path: path, Format: string(doc.Format), Encoding: "container"}
	if err != nil {
		rep.Error = err.Error()
		return rep
	}

	for _, f := range doc.Metadata {
		rep.Metadata = append(rep.Metadata, metaRecord{
			Kind: string(doc.Format) + "-property", Label: f.Label, Value: f.Value,
		})
	}

	if doc.Text == "" {
		rep.assessProvenance()
		return rep
	}

	resolved := cfg.For(relativeTo(cfg.Path, path))
	d := opt.detector(mo, format, resolved, doc.Text)
	d.Clean = false
	res := d.Run(doc.Text)
	rep.Findings = res.Findings
	rep.Payloads = res.Payloads
	rep.Mixed = res.Mixed
	rep.Suppressed = res.Suppressed
	rep.assessProvenance()
	return rep
}

// assessProvenance reads the file's own metadata as evidence about its origin.
// A named generator raises confidence in everything else found in that file; a
// bare credential does not, since cameras sign authentic photographs too.
func (r *fileReport) assessProvenance() {
	for _, m := range r.Metadata {
		if tool, ok := provenance.Generator(m.Value); ok {
			source := m.Kind
			if m.Label != "" {
				source = m.Kind + " " + m.Label
			}
			r.Signals = append(r.Signals, provenance.Signal{
				Source: source, Detail: tool, AI: true,
			})
			continue
		}
		if m.Kind == string(media.C2PA) {
			r.Signals = append(r.Signals, provenance.Signal{
				Source: "c2pa", Detail: "signed credential present",
			})
		}
	}
	if len(r.Signals) == 0 {
		return
	}
	r.Confidence = provenance.Assess(r.Signals)
}
