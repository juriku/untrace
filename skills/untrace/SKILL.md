---
name: untrace
description: Find and remove invisible characters, hidden payloads, confusable letters and AI provenance metadata in text and files. Use when text was pasted from a chatbot, before committing content of unknown origin, when a file may carry tracking characters or a C2PA credential, or when asked to "clean this text", "check for hidden characters", "strip AI metadata" or "remove watermarks".
---

# untrace

A single static binary that finds what is in a file and takes it out. It never
rewrites your words: it removes and normalises individual characters, and
removes metadata records.

## Before you start

Check the binary is there:

```
untrace --version
```

If it is missing: `brew install juriku/tap/untrace`, or
`go install github.com/juriku/untrace/cmd/untrace@latest`.

## Checking text you already have

Write the text to a file and scan it. Give the file the extension it will really
have, because that decides the rules: an em dash is ordinary in a `.md` file
that uses them throughout and suspicious in a `.py` identifier.

```
untrace --json notes.md
```

Read `--json` rather than the human report. Each finding carries `actionable`,
which tells you a fix exists, and `replacement`, which tells you what to put
there. An actionable finding with no `replacement` means delete the character.

To fix in place:

```
untrace --fix notes.md
```

## Checking text without touching disk

```
printf '%s' "$TEXT" | untrace --stdin --stdin-name draft.md --json
```

In `--stdin` mode stdout carries only the document, so `--fix` can be piped:

```
printf '%s' "$TEXT" | untrace --stdin --fix > clean.txt
```

`--stdin-name` resolves format and config as though the content lived at that
path, without reading it. Without a name, content is treated as source, which is
the strictest policy.

## Checking a project

```
untrace --fail .
```

Exits 1 if anything actionable is found. Honours `.gitignore` and skips
dependency directories.

## Reading the result

| field | meaning |
|---|---|
| `findings` | one entry per character, with `line`, `column`, `codepoint`, `name` |
| `payloads` | runs of invisible characters that decoded to data, with the decoded `text` |
| `mixed_script` | words mixing scripts, the confusable-letter attack |
| `metadata` | provenance records found in the file |
| `confidence` | `likely` when the file names an AI generator |

`column` is a 1-based rune offset, not bytes and not UTF-16 units.

A payload is the finding worth surfacing first. It is not ambiguous: a run of
invisible characters that decodes to readable text was put there deliberately,
and the decoded text usually names what was being tracked.

## What to tell the user

Report what was found and what changed. If a payload decoded, quote the decoded
text. If the file carries a C2PA manifest or a TC260 label naming a generator,
say so: that is a statement of origin in the file rather than an inference from
the prose.

Do not call a credential verified. untrace reads the generator name out of a
manifest as text; it checks no signature. A manifest naming a tool is evidence
that tool appears in the file's history, not proof the credential is genuine.

## What this cannot do

**It cannot detect or remove a statistical text watermark**, and neither can any
other character or metadata tool. Every Claude model released on or after
2 August 2026 carries SynthID-Text, which biases word choice rather than adding
characters or metadata. There is nothing in the file to find. Detecting one
needs the cryptographic key; removing one means rewriting the text.

Say so plainly if asked. Do not imply a clean untrace result means text is not
AI-generated. It means the file carries no character-level or metadata marker.

**Confusable letters are reported, not fixed.** Turning a Cyrillic `а` into `a`
guesses the script the author meant, and in a genuinely Cyrillic word the guess
corrupts it.

**A generator name is only trusted in a field that names a tool.** EXIF
`Software`, XMP `CreatorTool`, a document's `Application` or `generator`, a
PDF's `/Producer`. The same name in an author field or a description is
reported as metadata and declares nothing: books have authors called Ernie and
papers discuss minimax.

**Documents and images are edited, never repacked.** A PDF can still name its
generator inside a compressed object stream, where untrace cannot reach. If a
value it stripped is still in the file, it says so rather than reporting clean.

## Flags worth knowing

```
--fix                  rewrite what is found
--json                 machine-readable output
--sarif                SARIF 2.1.0, for GitHub code scanning
--fail                 exit 1 if anything actionable is found
--strip-metadata       with --fix, remove metadata naming an AI generator
--strip-metadata=all   with --fix, remove every metadata record
--stdin                read the document from stdin, write it to stdout
--stdin-name PATH      resolve format and config as this path
--strict               report every marker, including legitimate ones
--baseline PATH        accept the findings recorded there, so only new ones fail
```

`--strip-metadata` without `=all` leaves camera authenticity credentials in
place, so scrubbing a directory of photographs does not destroy them.
