# Dynamic resolvers

## The problem

A character is not intrinsically a watermark. The same codepoint is legitimate in
one document and an attack in another:

- An em dash in a Word document is ordinary typography. In a Python identifier it
  is not.
- A zero-width joiner between two emoji builds `👨‍👩‍👧`. Between two Latin letters
  it hides data. In Persian it is grammatically required.
- A Cyrillic `а` inside a Russian word is correct. The same character inside
  `pаypal` is a homoglyph attack.
- U+FEFF at byte 0 is a byte-order mark. At byte 5000 it is a marker.

A static table of "bad characters" therefore produces false positives on real
documents and false negatives on real attacks. The tables tell us what a
character *is*; resolvers decide what it *means here*.

## Model

Each occurrence produces a **Finding** carrying a verdict, not a boolean:

| field | meaning |
|---|---|
| `Kind` | hidden, typographic, homoglyph, payload, metadata |
| `Severity` | high, medium, low |
| `Confidence` | certain, likely, possible |
| `Action` | remove, replace, report-only, ignore |
| `Reasons` | which resolvers fired |

Resolvers run in layers. Later layers see the output of earlier ones.

## Layer 0: file classification

**R1 Format** — extension, magic bytes, shebang, content sniff.
Classes: `source`, `prose`, `office`, `pdf`, `data`, `markup`, `notebook`,
`binary`. Sets the baseline policy. This is what makes a `.docx` tolerate em
dashes and curly quotes automatically, replacing the old manual `--word` flag.

**R2 Encoding** — BOM sniff for UTF-8/16/32, UTF-8 validity, `latin-1` fallback.
Also legitimises a BOM at offset 0.

**R3 Provenance** — C2PA manifest, EXIF `Software`, PDF `/Producer`, OOXML
`app.xml`. Not a suppressor: if a document declares AI origin, this *raises*
confidence on every other finding in it.

## Layer 1: document-wide analysis

**R4 Script census** — count letters per Unicode script. A document that is 40%
Cyrillic legitimately contains Cyrillic, so blanket-flagging Cyrillic homoglyphs
there is wrong.

**R5 Mixed-script words** — the correct homoglyph algorithm (UTS #39 §5).
Tokenise, then flag words that *mix* scripts rather than flagging every non-Latin
character. `раypal` is caught; a Russian sentence is not. This replaces the
curated confusable list as the primary mechanism, keeping the list only as a
naming table for reporting.

Two rules keep this off ordinary multilingual prose. A hyphen, underscore or
apostrophe **bounds** a token, because it joins two words rather than forming
one: `IT-специалист` is Latin beside Cyrillic and `β-carotene` is Greek beside
Latin, whereas `раypal` has no such boundary to hide behind. And Latin belongs to
each augmented CJK set, since product names and initialisms are written in Latin
inside CJK prose with no space to break the word.

**R6 Typography census** — is the document typographically consistent? A file
using curly quotes and em dashes throughout is authored prose. One curly quote
among five thousand straight ones is an anomaly worth flagging even in prose.

**R7 Density and distribution** — invisibles per KB, and whether they cluster or
scatter. Regular intervals suggest encoding rather than stray keystrokes.

**R8 Payload decode** — zero-width binary, variation-selector byte smuggling,
tag-block ASCII. If a run decodes to printable text, that is a definitive
watermark at maximum confidence, and it outranks every legitimacy resolver below.

## Layer 2: per-occurrence context

**R9 Grapheme legitimacy** — is the character doing its actual Unicode job?

- U+FE0F following an emoji base: emoji presentation selector.
- ZWJ between emoji: emoji ZWJ sequence.
- ZWJ/ZWNJ adjacent to Arabic, Persian or Indic letters: required for rendering.
- Variation selector after a CJK ideograph: legitimate IVS use.
- Combining mark after a base character.
- BOM at offset 0.
- Script-owned punctuation next to letters of that script: U+3002 and the
  fullwidth forms beside CJK, U+060C beside Arabic, U+037E beside Greek. These
  are that language's full stop and comma, not a stylised ASCII one.
- A bidi **isolate**, U+2066 to U+2069, that is terminated and encloses
  right-to-left letters. The overrides and embeddings U+202A to U+202E are
  deliberately excluded: they are the Trojan Source vector (CVE-2021-42574), and
  UAX #9 recommends isolates for mixing scripts precisely because they do not
  reorder text outside their span. An unterminated isolate is likewise never
  legitimate, since escaping its line is the whole mechanism of the attack.

**R10 Region** — where in the file. Markdown fenced code versus prose; source
comment or string literal versus identifier; HTML text node versus attribute;
notebook code cell versus markdown cell. An em dash is fine in a Markdown
paragraph and suspicious in a Go identifier.

**R11 Code adjacency** — a homoglyph inside an identifier is a security finding;
the same homoglyph in a comment is cosmetic. Severity differs even though the
character is identical.

## Layer 3: explicit policy

**R12 Config** — CLI flags, then `.untrace.toml` at the repo root, then nested
configs per directory, with glob-scoped overrides in the eslint style.

**R13 Inline suppression** — `untrace:ignore`, `untrace:ignore-line`,
`untrace:ignore-file` in formats that have comments.

**R14 Baseline** — `.untrace-baseline.json` recording accepted existing findings,
so a large repo can adopt the tool without fixing everything first.

## Precedence

Highest wins:

1. R13 inline suppression, then R12 config force include/exclude. Explicit human
   intent always wins.
2. R8 payload decode. A decoded payload is an attack regardless of format or of
   the character's normal legitimacy.
3. R9 grapheme legitimacy. Correct Unicode usage suppresses.
4. R10 region, then R1 format policy.
5. R4/R5 script analysis.
6. R3 provenance, R6 typography census, R7 density adjust *confidence* only. They
   never decide alone.

## Worked verdicts

The same characters, resolved differently:

| character | context | verdict | resolver |
|---|---|---|---|
| U+2014 em dash | `.docx` prose | ignore | R1 format |
| U+2014 | Python identifier | flag, high | R1 + R11 |
| U+2014 | `.md` prose, used throughout | ignore | R6 typography census |
| U+2014 | `.md` prose, sole instance | flag, low | R6 |
| U+00A0 | `.html` | ignore | R1, meaningful as `&nbsp;` |
| U+00A0 | `.py` | replace with space | R1 |
| U+200D ZWJ | between emoji | ignore | R9 |
| U+200D | between Latin letters | flag, high | R9 fails |
| U+200D | Persian text | ignore | R9 |
| Cyrillic а | Russian prose | ignore | R4 census |
| Cyrillic а | `pаypal` in English | flag, critical | R5 mixed-script |
| U+3002 。 | after Japanese kana | ignore | R9 script punctuation |
| U+3002 。 | between Latin words | flag | R9 fails |
| Cyrillic + Latin | `IT-специалист` | ignore | R5, the hyphen bounds the token |
| Cyrillic + Latin | `раypal` | flag, critical | R5, no boundary |
| U+2067 RLI | terminated, around Arabic | ignore | R9 bidi isolate |
| U+2067 RLI | unterminated | flag | R9 fails |
| U+202E RLO | anywhere | flag | never legitimate, CVE-2021-42574 |
| U+FEFF | offset 0 | ignore | R2 |
| U+FEFF | mid-file | flag | R2 fails |
| VS run decoding to text | anywhere | flag, critical | R8 |

## Consequences for the CLI

- `--word` disappears. R1 detects Office documents and applies that policy.
- `--check-typographic` and `--check-ivs` become overrides on a resolved default
  rather than global switches.
- `--strict` disables the suppressing resolvers for users who want the raw
  character census.
- `--json` carries severity, confidence and reasons, which is what the VS Code
  extension needs to choose squiggle colours.

## What is built

| resolver | state |
|---|---|
| R1 format | built |
| R2 encoding | built |
| R3 provenance | built, raises confidence only when metadata names a generator |
| R4 script census | built for script-owned punctuation: letters are counted per script group once per document, and a group owning at least a tenth of them makes its punctuation native. Below that line R9 falls back to the nearest letter either side, so a quoted sentence too short to move the census still reads correctly. Homoglyphs are folded into R5 |
| R5 mixed-script words | built |
| R6 typography census | not built |
| R7 density | not built |
| R8 payload decode | built, all three schemes |
| R9 grapheme legitimacy | built |
| R10 region | built for Markdown fences; only changes an outcome once a config loosens a format, since every format is otherwise equally strict |
| R11 code adjacency | not built |
| R12 config | built, including glob-scoped overrides |
| R13 inline suppression | built |
| R14 baseline | not built |

The rest of this document describes the intended design, not current state.

## Build order

Resolvers are independent, so they land incrementally. Each one only ever
*reduces* false positives or *raises* severity, so partial implementation is
safe.

1. R1, R2 with the core (Phase 1). Without R1 the tool cannot tell a `.docx` from
   a `.py`, which is the headline behaviour.
2. R9, R12 (Phase 1/3). R9 is what stops emoji being reported as attacks.
3. R8, R5 (Phase 3). The two that make the tool genuinely better than a grep.
4. R10, R11 (Phase 3+). Need light per-format parsing.
5. R3 (Phase 5, with metadata).
6. R6, R7, R13, R14 as refinements.
