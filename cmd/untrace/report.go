package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/juriku/untrace/internal/media"
	"github.com/juriku/untrace/internal/provenance"
)

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31;1m"
	ansiYellow = "\x1b[33;1m"
	ansiGreen  = "\x1b[32;1m"
	ansiCyan   = "\x1b[36m"
	ansiDim    = "\x1b[2m"
)

type painter struct{ on bool }

func (p painter) paint(code, s string) string {
	if !p.on {
		return s
	}
	return code + s + ansiReset
}

func printReport(w io.Writer, reports []fileReport, sum summary, opt options, color bool) {
	p := painter{on: color}

	for _, r := range reports {
		if r.Error != "" {
			fmt.Fprintf(w, "%s %s: %s\n", p.paint(ansiRed, "error"), r.Path, r.Error)
			continue
		}
		if len(r.Findings) == 0 && len(r.Payloads) == 0 && len(r.Mixed) == 0 &&
			len(r.Metadata) == 0 {
			continue
		}

		fmt.Fprintf(w, "\n%s", p.paint(ansiCyan, r.Path))
		if r.Confidence == provenance.Likely {
			fmt.Fprintf(w, " %s", p.paint(ansiRed, "[ai-generated: likely]"))
		}
		fmt.Fprintln(w)

		for _, pl := range r.Payloads {
			label := fmt.Sprintf("hidden payload (%s)", pl.Scheme)
			if pl.Printable {
				fmt.Fprintf(w, "  %s %s decodes to %q across %d characters\n",
					p.paint(ansiRed, "!!"), label, pl.Text, pl.Runes)
			} else {
				fmt.Fprintf(w, "  %s %s of %d characters, not printable text\n",
					p.paint(ansiRed, "!!"), label, pl.Runes)
			}
		}

		for _, sig := range r.Signals {
			if sig.AI {
				fmt.Fprintf(w, "  %s  %s names %q\n",
					p.paint(ansiRed, "ai-declared"), sig.Source, sig.Detail)
			}
		}

		for _, md := range r.Metadata {
			label := string(md.Kind)
			if md.Label != "" {
				label += " " + md.Label
			}
			marker := p.paint(ansiYellow, "metadata")
			if md.Kind == string(media.C2PA) {
				marker = p.paint(ansiRed, "provenance")
			}
			size := ""
			if md.Bytes > 0 {
				size = fmt.Sprintf(" (%d bytes)", md.Bytes)
			}
			kept := ""
			switch {
			case md.Residue:
				kept = p.paint(ansiRed, " [still present elsewhere in the file]")
			case opt.fix && opt.stripMetadata == stripAI && !md.Stripped:
				kept = p.paint(ansiDim, " [kept: names no generator]")
			}
			if md.Value != "" {
				fmt.Fprintf(w, "  %s  %s = %q%s%s\n", marker, label, md.Value, size, kept)
			} else {
				fmt.Fprintf(w, "  %s  %s%s%s\n", marker, label, size, kept)
			}
		}

		for _, mw := range r.Mixed {
			fmt.Fprintf(w, "  %d:%d  %s  %q mixes %s\n",
				mw.Line, mw.Column, p.paint(ansiRed, "mixed-script"),
				mw.Word, strings.Join(mw.Scripts, " and "))
		}

		hidden := 0
		for _, f := range r.Findings {
			if f.InPayload {
				hidden++
				continue
			}
			action := p.paint(ansiYellow, f.Action)
			if f.Applied {
				action = p.paint(ansiRed, f.Action)
			}
			detail := fmt.Sprintf("%s %s", f.Codepoint, f.Name)
			if f.Applied && f.Replacement != "" {
				detail += fmt.Sprintf(" -> %q", f.Replacement)
			}
			fmt.Fprintf(w, "  %d:%d  %s  %s %s\n",
				f.Line, f.Column, action, detail, p.paint(ansiDim, "("+f.Kind+")"))
		}
		if hidden > 0 {
			fmt.Fprintf(w, "  %s\n",
				p.paint(ansiDim, fmt.Sprintf("(%d character(s) in the payload above; --json to list them)", hidden)))
		}
	}

	if sum.Detected == 0 && sum.Mixed == 0 && sum.Metadata == 0 {
		fmt.Fprintf(w, "%s %d file(s) scanned", p.paint(ansiGreen, "clean:"), sum.FilesScanned)
		if sum.Suppressed > 0 {
			fmt.Fprintf(w, ", %d suppressed", sum.Suppressed)
		}
		if sum.Baselined > 0 {
			fmt.Fprintf(w, ", %d baselined", sum.Baselined)
		}
		fmt.Fprintln(w)
		return
	}

	fmt.Fprintf(w, "\n%s %d marker(s) in %d of %d file(s)",
		p.paint(ansiYellow, "found:"), sum.Detected, sum.FilesWithMarks, sum.FilesScanned)
	if sum.Metadata > 0 {
		fmt.Fprintf(w, ", %d metadata record(s)", sum.Metadata)
	}
	if sum.Mixed > 0 {
		fmt.Fprintf(w, ", %d mixed-script word(s)", sum.Mixed)
	}
	if sum.Payloads > 0 {
		fmt.Fprintf(w, ", %d decoded payload(s)", sum.Payloads)
	}
	if sum.Applied > 0 {
		fmt.Fprintf(w, ", %d fixed", sum.Applied)
	}
	if sum.Suppressed > 0 {
		fmt.Fprintf(w, ", %d suppressed", sum.Suppressed)
	}
	if sum.Baselined > 0 {
		fmt.Fprintf(w, ", %d baselined", sum.Baselined)
	}
	fmt.Fprintln(w)
}

func usage(fs *flag.FlagSet) {
	fmt.Fprint(os.Stderr, `untrace - find and remove hidden markers that tag text as AI-generated

usage:
  untrace [flags] [path ...]     scan paths, defaulting to the current directory
  untrace --fix [path ...]       rewrite what is found
  cat file | untrace --stdin     read a document from stdin, write it to stdout

subcommands:
  untrace install-filter         register an opt-in git clean filter

What counts as a marker depends on the file. Word documents keep their curly
quotes and em dashes; every other format, prose included, has them normalised.
Use --json to see the format and encoding resolved per file.

flags:
`)
	fs.PrintDefaults()
}
