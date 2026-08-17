# untrace

Text copied out of an AI chatbot carries marks you cannot see. untrace finds
them and takes them out.

A single static binary, no dependencies.

**[VS Code extension](editors/vscode/README.md)** ·
[Configuration](#configuration) ·
[Contributing](CONTRIBUTING.md) ·
[Security](SECURITY.md)

## Start here

```
brew install juriku/tap/untrace
go install github.com/juriku/untrace/cmd/untrace@latest
```

Point it at a file to see what is in there. Nothing is changed yet:

```
$ untrace pasted.txt

pasted.txt
  1:5   detected  U+2019 Right Single Quotation Mark (typographic)
  1:33  detected  U+2014 Em Dash (typographic)
  3:9   detected  U+00A0 Non-Breaking Space (hidden)
  3:22  detected  U+201C Left Double Quotation Mark (typographic)
  3:30  detected  U+201D Right Double Quotation Mark (typographic)

found: 5 marker(s) in 1 of 1 file(s)
```

Add `--fix` when you want them gone:

```
$ untrace --fix pasted.txt

pasted.txt
  1:33  replaced  U+2014 Em Dash -> "-" (typographic)
  3:9   replaced  U+00A0 Non-Breaking Space -> " " (hidden)
  ...
found: 5 marker(s) in 1 of 1 file(s), 5 fixed
```

`untrace .` checks a whole folder and `untrace --fix .` cleans it.

## What it removes

| | |
|---|---|
| **Invisible characters** | Zero-width spaces and joiners, odd spaces, soft hyphens, direction controls, stray byte-order marks. You cannot see them, and they survive copy and paste. |
| **Hidden payloads** | Some invisible characters carry a message. untrace decodes it, so you see `tracked-by:acct-99213` instead of "21 invisible characters". |
| **Giveaway punctuation** | Curly quotes, em and en dashes and the rest, normalised back to plain ASCII. |
| **Lookalike letters** | A Cyrillic `а` sitting inside an otherwise Latin word, as in `pаypal`. Reported rather than silently rewritten. |
| **"Made by" metadata** | C2PA Content Credentials, EXIF, XMP, and the properties inside PDFs and Office files. A file is flagged `[ai-generated: likely]` only when a generator is actually named. |

## What it does not do

**It cannot touch a statistical watermark, and neither can anything else that
works on characters.** Every Claude model released on or after 2 August 2026
carries SynthID-Text. It works by biasing the model's choice among
near-equivalent words, so the signal lives in *which words were chosen*, spread
across a whole passage. No characters are added and no metadata is added, so
there is nothing in the file for untrace to find or strip. Detecting one needs
the cryptographic key and a statistical test; removing one means rewriting the
text. SynthID for images and audio is the same story one layer down: it lives in
the pixel and waveform data, so deleting metadata does nothing to it.

A tool that claims to strip a statistical text watermark is making a claim you
cannot check, because the scheme is unpublished.

**It never changes your words.** untrace removes and normalises individual
characters, and removes metadata records. It does not rewrite sentences, reword
anything or paraphrase. Whatever lives in the words themselves it leaves exactly
as you wrote it.

**It edits documents in place, it never repacks them.** Word, Excel, PowerPoint
and OpenDocument files are fixed by copying every part of the archive across
untouched except the text, which is edited in place. A PDF's `/Producer` and
`/Creator` are emptied without changing the file's byte length, so its
cross-reference table stays valid and nothing is rebuilt. Image and document
metadata are removed behind `--strip-metadata`.

**A PDF can still name its generator somewhere untrace cannot reach.** Values
inside compressed object streams, hex strings, and XMP packets are not edited. If
a value untrace stripped is still in the file, it says so rather than reporting
the file clean. Body text inside a PDF is not read at all, so a hidden character
in a PDF's text is neither found nor removed.

**It will not break a file to fix it.** A curly quote normalises to a straight
one, which is a string delimiter in some formats, so the replacement is written
the way the surrounding syntax requires. In JSON, JSONC and notebooks it is
escaped. In TOML and YAML it is escaped inside a `"quoted"` string and written
plainly inside a `'literal'` one, a block scalar or a comment, where a quote is
ordinary content. The file stays valid and the text still reads `"like this"`.

Two cases are reported rather than fixed, because no replacement is safe: a
plain unquoted YAML scalar, where writing a quote would change the value rather
than a character, and `.ini`, `.cfg` and `.conf`, which have no single grammar.
Every other marker in those files is still fixed.

**It reads signed credentials, it does not verify them.** A C2PA manifest naming
a generator is evidence that tool appears in the file's history, not proof the
credential is genuine.

## Common tasks

**I pasted something from a chatbot.**

```
untrace --fix notes.md
```

**I want to check a project before committing.**

```
untrace --fail .
```

Exits non-zero if anything worth acting on is found, so CI can use it.

**I want to know what a file is hiding, without changing it.**

```
untrace --json report.docx
```

**I have an image.**

```
untrace --fix --strip-metadata photo.png       # remove records naming a generator
untrace --fix --strip-metadata=all photo.png   # remove every record
```

**I want this to happen automatically.** See [In VS Code](#in-vs-code),
[In CI or a pre-commit hook](#in-ci-or-a-pre-commit-hook) and
[As a git clean filter](#as-a-git-clean-filter).

## Things that surprise people

**The ellipsis, the bullet and the middle dot are left alone.** They are
ordinary punctuation in prose, lists and slides, and the middle dot is a letter
in Catalan `l·l`. Flagging them is noise everywhere and signal nowhere.

**Emoji are left alone.** The joiner inside a family emoji is the emoji working
as designed. The same joiner between two Latin letters is reported.

**Other writing systems are left alone.** Japanese and Arabic punctuation are
not wrong versions of ASCII, so they are never rewritten. No letter outside
Latin is replaced by default.

**Lookalike letters are reported, not fixed.** Turning `а` into `a` is a guess
about what the author meant, and in a genuinely Cyrillic word the guess corrupts
it. Pass `--fix-homoglyphs` if you want it done anyway.

**Your prose is treated like source code by default.** An em dash in a `.md`
file is normalised. If your writing uses them on purpose, see
[Configuration](#configuration); that is the one setting most projects change.

## Install

Binaries for macOS, Linux and Windows on amd64 and arm64 are attached to each
[release](https://github.com/juriku/untrace/releases).

### In VS Code

[`editors/vscode`](editors/vscode) reports findings as you edit, with fixes on
the lightbulb, and checks buffers you have not saved. It shells out to the
binary, so install that first.

Settings, quick fixes and fix-on-save are documented in
[the extension's own README](editors/vscode/README.md).

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

## How it decides

A character is not a watermark by itself. The same one can be perfectly normal
in one file and a problem in another, so untrace works out what a file *is*
before deciding what its characters *mean*:

| character | in a Word document | in a log file | everywhere else |
|---|---|---|---|
| em dash | ignored | ignored | normalised to `-` |
| curly quotes | ignored | ignored | normalised to `"` |
| non-breaking space | ignored | ignored | normalised to a space |
| zero-width space | reported | ignored | removed |

Only two formats deviate. **Word and other Office documents** ignore the twelve
characters those editors insert by themselves: you type `--` and Word makes it
an em dash, so its presence says nothing about who wrote the document. A
zero-width space in one is not ignored, but it is only reported: taking it out
would mean repacking the archive, so no document is ever rewritten. **Log
files** are ignored outright. Everything else is treated the same way.

No context excuses the direction **override** U+202E, the Trojan Source vector.
It is never treated as doing a legitimate job, so only a format that ignores
everything, such as a log file, passes over it.

Run `--json` to see the format and encoding it picked per file, or `--strict` to
switch the judgement off and see every marker. `docs/design/resolvers.md`
explains the whole scheme.

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

`actionable` is what tells a consumer a fix exists, and `replacement` is what to
put there. An actionable finding with no `replacement` field means delete the
character, which is the normal case for invisible ones. `applied` records what
`--fix` actually did, so it stays false when only reporting.

Content arriving on stdin has no name, so it is classified as source and no
path-scoped override applies. `--stdin-name path/to/file.md` resolves format,
config and regions as though the content lived there, without reading the file.
That is what lets an editor check an unsaved buffer under the rules its real
path would get.

### Flags

```
--fix, -c              rewrite files
--fix-homoglyphs       with --fix, also rewrite confusable letters to Latin
--strip-metadata       with --fix, remove metadata naming an AI generator
--strip-metadata=all   with --fix, remove every metadata record
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

## Detail

- **Invisible characters**: zero-width space, joiner and non-joiner, word joiner,
  BOM, soft hyphen, bidi controls and isolates, invisible maths operators,
  non-standard spaces, variation selectors, ideographic variation selectors.
- **Hidden payloads**: runs of tag characters, variation selectors or zero-width
  characters that encode data, decoded and printed.
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
camera's does, survives until you pass `=all`. untrace finds that name as text:
the claim generator is a CBOR string and C2PA forbids splitting it, so no JUMBF
or CBOR parsing is needed to read one. A name mentioned anywhere in the manifest
counts, including in an ingredient from an earlier edit.

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
dashes will fail `--fail` until you set it to `report` or `ignore`. A setting you
write covers the whole file, fenced code blocks included. Inside a fence the
kinds you left unset go back to the stricter source rules, so loosening
`typographic` there does not also loosen `hidden`.

`overrides` scope settings to paths using gitignore glob syntax, resolved
relative to the config file, with later entries winning. untrace uses one on
itself so its own documentation can contain lookalike-letter examples without
failing its own scan.

### Ignoring a single line

Sometimes a marker is there on purpose: a test fixture that needs a zero-width
space, or documentation showing what a lookalike-letter attack looks like. Add a
comment and untrace skips it, the same way `# noqa` or `// eslint-disable-line`
work.

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

- `docs/design/resolvers.md` for how context decides what a character means
- `docs/design/watermark-techniques.md` for the watermarking landscape and what
  is reachable
- `docs/design/performance.md` for the allocation invariants a scan has to hold
