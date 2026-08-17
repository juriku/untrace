# untrace

Detect and remove the invisible characters and metadata used to mark text as
AI-generated.

A single static binary, no dependencies.

## What makes it different

A character is not intrinsically a watermark. untrace resolves what a file *is*
before deciding what its characters *mean*:

| character | in a Word document | in a log file | everywhere else |
|---|---|---|---|
| em dash | ignored | ignored | normalised to `-` |
| curly quotes | ignored | ignored | normalised to `"` |
| non-breaking space | ignored | ignored | normalised to a space |
| zero-width space | removed | ignored | removed |

Two formats deviate and no others. **Word and other Office documents** ignore
the twelve characters those editors insert by themselves: you type `--` and Word
makes it an em dash, so its presence says nothing about who wrote the document.
**Log files** are ignored outright. Everything else is treated the same way.

Emoji are left alone. A zero-width joiner inside `👨‍👩‍👧` is the sequence working
as designed, as is a joiner in Persian or Devanagari and a variation selector on
a CJK ideograph. The same joiner between two Latin letters is reported.

Other writing systems are left alone too. `。` and `？` are Japanese punctuation,
`،` is an Arabic comma, and a terminated bidi isolate around right-to-left text
is doing its job, so none of them are rewritten. No letter outside Latin is ever
replaced by default. What stays flagged everywhere is the bidi **override**
U+202E, which is the Trojan Source vector and never legitimate.

Run with `--json` to see the format and encoding resolved per file, or
`--strict` to switch all of that off and see every marker.

## Install

```
brew install juriku/tap/untrace
go install github.com/juriku/untrace/cmd/untrace@latest
```

Binaries for macOS, Linux and Windows on amd64 and arm64 are attached to each
[release](https://github.com/juriku/untrace/releases).

### In VS Code

[`editors/vscode`](editors/vscode) reports findings as diagnostics while you
edit, including in a buffer you have not saved. It shells out to the binary
above, so install that first and make sure it is on `PATH` or point
`untrace.path` at it.

| setting | default | meaning |
|---|---|---|
| `untrace.enable` | `true` | report findings as diagnostics |
| `untrace.path` | `untrace` | path to the binary |
| `untrace.run` | `onSave` | `onSave`, `onType` or `off` |
| `untrace.strict` | `false` | report every marker, as `--strict` does |

### In CI or a pre-commit hook

```yaml
repos:
  - repo: https://github.com/juriku/untrace
    rev: v0.1.0
    hooks:
      - id: untrace          # fail on anything actionable
      # - id: untrace-fix    # or rewrite files in place
```

### As a git clean filter

If you want content scrubbed automatically as it is staged:

```
untrace install-filter
echo '* filter=untrace' >> .gitattributes
git add --renormalize .
```

This is deliberately opt-in rather than the recommended path. Git's own
documentation warns that a project must stay usable without the filter, the
driver definition lives in git config and cannot be committed, so every clone
needs it installed again, and your working tree will then differ from the index,
which surprises people. `untrace install-filter --remove` undoes it. A
pre-commit hook is the less surprising choice for most projects.

## Usage

```
untrace                        scan the current directory
untrace src/ docs/             scan specific paths
untrace --fix .                rewrite what is found
untrace --json .               machine-readable output
untrace --fail .               exit 1 if anything is found, for CI
cat file.txt | untrace --stdin --fix > clean.txt
```

In `--stdin` mode stdout carries only the document, so it is safe to redirect.
The report goes to stderr, and so does `--json`.

Positions in `--json` are 1-based, and a column counts runes rather than bytes
or UTF-16 units. A finding spans exactly one rune, a mixed-script word spans its
`word`, and a payload spans `runes` characters from its line and column.

Content arriving on stdin has no name, so it is classified as source and no
path-scoped override applies. `--stdin-name path/to/file.md` resolves format,
config and regions as though the content lived there, without reading the file.
That is what lets an editor check an unsaved buffer under the rules its real
path would get.

### Flags

```
--fix, -c              rewrite files
--fix-homoglyphs       with --fix, also rewrite confusable letters to Latin
--strip-metadata       with --fix, remove image metadata naming an AI generator
--strip-metadata=all   with --fix, remove every image metadata record
--stdin                read the document from stdin, write it to stdout
--stdin-name PATH      with --stdin, resolve format and config as this path
--json                 emit findings as JSON (stderr in --stdin mode)
--fail                 exit 1 if anything actionable is found
--strict               report every marker, including legitimate ones
--quiet                suppress the per-file report
--no-recursive         do not descend into subdirectories
--no-gitignore         scan files git would ignore
--no-default-ignores   descend into dependency and cache directories
--ignore-dir NAME      additional directory to skip (repeatable)
--pattern GLOB         only scan files matching this glob (repeatable)
--exclude-char CP      codepoint to ignore, as U+XXXX or a literal (repeatable)
--config PATH          use this config file instead of discovering one
```

## What it detects

- **Invisible characters**: zero-width space, joiner and non-joiner, word joiner,
  BOM, soft hyphen, bidi controls and isolates, invisible maths operators,
  non-standard spaces, variation selectors, ideographic variation selectors.
- **Typographic markers**: smart quotes, em and en dashes, and other characters
  that survive a copy out of a word processor. The ellipsis, the bullet and the
  middle dot are deliberately never reported, in any format: they are ordinary
  punctuation in prose, lists and slides, so flagging them is noise everywhere
  and signal nowhere. The middle dot is also a letter in Catalan `l·l`.
- **Homoglyphs**: Cyrillic and Greek letters that imitate Latin ones. These are
  reported but never rewritten unless you pass `--fix-homoglyphs`. Replacing
  `а` with `a` is a guess about which script the author meant, and in a word
  that is genuinely Cyrillic or Greek the guess corrupts it. `раypal` is worth
  telling you about; silently Latinising it is a different decision, and yours
  to make.
- **Hidden payloads**: runs of tag characters, variation selectors or zero-width
  characters that encode data. untrace decodes them and reports what they spell,
  which is the difference between "40 invisible characters" and
  `tracked-by:acct-99213`.
- **Image provenance metadata**: C2PA Content Credentials, EXIF `Software`, XMP
  `CreatorTool`, PNG text chunks and JPEG comments, in PNG and JPEG.
- **Document metadata and body text**: `.docx`, `.xlsx`, `.pptx` and `.odt`
  properties (`creator`, `lastModifiedBy`, `Application`) plus the text inside
  them, and PDF `/Producer` and `/Creator`.

A C2PA manifest is the strongest signal available. Anthropic attaches one to
images Claude produces, and unlike an em dash it is a signed statement of origin
rather than an inference.

**A credential is not the same as an AI credential.** Camera manufacturers sign
photographs with C2PA to prove they are authentic, so untrace reports a manifest
as provenance without calling it AI. A file is marked `[ai-generated: likely]`
only when its metadata names a known generator, such as `Software = "Adobe
Firefly 3"`. A Leica firmware string gets no such mark.

The same distinction governs removal. `--fix --strip-metadata` takes out only
the records that name a generator and leaves the rest in place, so scrubbing a
directory does not destroy the authenticity credentials on your photographs.
`--strip-metadata=all` removes every record.

That applies to signed credentials too. A manifest records the tool that made
the claim, so a manifest naming a generator is removed by the default and marks
the file `[ai-generated: likely]` on its own, while one that names none, as a
camera's does, survives until you pass `=all`.

Encodings are preserved. A latin-1 file stays latin-1, a UTF-16 file stays
UTF-16 with its BOM, and a file that cannot be decoded cleanly is skipped rather
than rewritten. A UTF-16 file holding an unpaired surrogate is refused for that
reason: decoding it would substitute U+FFFD and change the bytes on the way out.
Cleaning normalises exotic spaces to a regular space rather than deleting them,
so words never get glued together.

untrace skips what git skips. That means all three sources git reads from, not
just the nearest file: `.gitignore` in the scanned directory **and every parent
up to the repository root**, `.git/info/exclude`, and the global
`core.excludesFile`. Nesting, negation and precedence follow git's rules, so
`untrace src/` respects the project's root `.gitignore` rather than only the one
inside `src/`. `--no-gitignore` turns off all three.

Dependency directories such as `node_modules`, `__pycache__`, `.venv` and
`.terraform` are skipped by default too. Ambiguous names like `build`, `dist`,
`target` and `vendor` are **not**, because they hold real source in some
projects.

Symlinks are never followed, so a symlinked file is neither scanned nor fixed.
That keeps a scan inside the tree you pointed it at and stops `--fix` writing
through a link to somewhere else.

## What it cannot detect

Being clear about this matters, because other tools are not.

**Statistical token watermarks, including the one Claude now uses.** Every Claude
model released on or after 2 August 2026 carries a SynthID-Text watermark. It
works by biasing the model's choice among near-equivalent next words, so the
signal lives in *which words were chosen*, spread across a whole passage. No
characters are added and no metadata is added. There is nothing in the file for
untrace to find or strip. Detecting it requires the cryptographic key and a
statistical test; Anthropic ships its own detection API. The only way to remove
it is to rewrite the text.

Any tool that claims to strip a statistical text watermark is making a claim you
cannot verify, because the scheme is unpublished.

**Pixel and audio watermarks.** SynthID for images and audio lives in the pixel
and waveform data. Removing metadata does nothing to it.

**Documents and images are reported, never rewritten.** Removing a record from a
PDF means rebuilding its cross-reference table and from an Office file means
repacking the archive, both of which can corrupt the file. Images can be
rewritten, but only behind `--strip-metadata`. Plain text is the only thing
`--fix` touches on its own.

**C2PA manifests are read, not parsed or verified.** untrace reports that a
signed credential is present, how large it is, and the generator named inside
it. It finds that name as text: the claim generator is a CBOR string and C2PA
forbids splitting it, so no JUMBF or CBOR parsing is needed to read one. It does
not verify the signature, read the assertions, or check the chain, so a manifest
naming a generator is evidence that tool appears in the file's provenance, not
proof the credential is authentic. A name mentioned anywhere in the manifest
counts, including in an ingredient from an earlier edit.

## Configuration

Drop a `.untrace.json` in your repository root. untrace walks up from the scan
target and stops at the repository root.

```json
{
  "exclude": ["U+2014"],
  "ignoreDirs": ["fixtures"],
  "strict": false,
  "formats": {
    "prose": { "typographic": "ignore" },
    "source": { "hidden": "clean" }
  },
  "overrides": [
    {
      "files": ["docs/**", "*.test.ts"],
      "mixedScript": "ignore"
    }
  ]
}
```

Actions are `ignore`, `report` or `clean`. Formats are `source`, `prose`, `data`,
`markup`, `notebook`, `office`, `pdf` and `log`. Command-line flags always win.

Loosening `prose` is the one most projects need. Prose is cleaned as strictly as
source by default, so a repository whose documentation contains deliberate em
dashes will fail `--fail` until you set it to `report` or `ignore`. Fenced code
blocks stay strict even then, because a command someone will paste into a
terminal is not prose:

````markdown
Prose with an — em dash, which is deliberate.     <- left alone

```bash
curl —silent https://example.com                  <- still fixed to -silent
```
````

`overrides` scope settings to paths using gitignore glob syntax, resolved
relative to the config file, with later entries winning. untrace uses one on
itself so its own documentation can contain homoglyph examples without failing
its own scan.

### Ignoring a single line

Sometimes a marker is there on purpose: a test fixture that needs a zero-width
space, or documentation showing what a homoglyph attack looks like. Add a
comment and untrace skips it, the same way `# noqa` or `// eslint-disable-line`
work.

```python
sample = "zero​width"   # untrace:ignore
```

Three forms:

| comment | skips |
|---|---|
| `untrace:ignore` | the line it is on |
| `untrace:ignore-next-line` | the line below it |
| `untrace:ignore-file` | the whole file |

Skipped lines are never changed by `--fix`, and each run tells you how many were
skipped, so they do not quietly pile up:

```
found: 1 marker(s) in 1 of 1 file(s), 2 suppressed
```

Write it in whatever comment syntax your file uses. untrace looks for the text
itself, so `#`, `//`, `<!-- -->` and the rest all work.

`--strict` does not override these. It switches off the automatic judgements
about emoji and joiners; a line you skipped on purpose stays skipped.

## Exit codes

| code | meaning |
|---|---|
| 0 | completed; nothing actionable found, or `--fail` not given |
| 1 | `--fail` was given and something actionable was found |
| 2 | a file could not be read, or the config is invalid |

**Actionable** means a decoded payload, a mixed-script word, or a character
`--fix` would change. Only a format the policy sets to `report` or `ignore`
produces findings that do not fail a build.

Prose is not one of them. Em dashes and curly quotes in a `.md` file are cleaned
like anywhere else, so they are actionable and `--fail` exits 1 on them. If your
prose legitimately contains them, loosen the `prose` format in `.untrace.json`
as described under [Configuration](#configuration); that is the step that makes
`--fail` usable in CI on a prose-heavy repository.

## Design

- `docs/design/resolvers.md` — how context decides what a character means
- `docs/design/watermark-techniques.md` — the watermarking landscape and what is
  reachable
- `docs/design/performance.md` — the allocation invariants a scan has to hold
