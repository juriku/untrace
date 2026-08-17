# untrace for VS Code

Finds the invisible characters, lookalike letters and hidden payloads that AI
tools and copy-paste leave behind, and fixes them from the lightbulb.

**[untrace](../../README.md)** ·
[What it finds](../../README.md#what-it-removes) ·
[Configuration](../../README.md#configuration)

## Requirements

The `untrace` binary does the work, so install it first:

```
brew install juriku/tap/untrace
go install github.com/juriku/untrace/cmd/untrace@latest
```

Set `untrace.path` if it is not on your `PATH`.

## What you see

| finding | severity |
|---|---|
| decoded hidden payload | Error |
| lookalike letters, such as `pаypal` | Warning |
| a character `--fix` would change | Warning |
| a marker reported but not rewritten | Information |

Characters inside a decoded payload are folded into the payload's own
diagnostic rather than reported one by one, so a twenty-character payload is one
squiggle and not twenty.

## Quick fixes

Put the cursor on a finding and press `Cmd+.` or `Ctrl+.`:

| action | what it does |
|---|---|
| Replace `U+2014` with `"-"` | rewrites that one character |
| Remove `U+200B` | deletes it, for invisible characters |
| Fix all 3 Em Dash in this file | every occurrence of that character |
| Fix all 7 in this file | every fixable finding |
| Ignore this line | appends `untrace:ignore` in the file's comment syntax |

**A lookalike letter is never offered a fix.** Turning `а` into `a` guesses
which script the author meant, and in a genuinely Cyrillic word the guess
corrupts it. The squiggle stays so you can decide.

To clean a file whenever you save it:

```json
"editor.codeActionsOnSave": { "source.fixAll.untrace": "explicit" }
```

## Settings

| setting | default | meaning |
|---|---|---|
| `untrace.enable` | `true` | report findings at all |
| `untrace.path` | `untrace` | path to the binary |
| `untrace.run` | `onSave` | `onSave`, `onType` or `off` |
| `untrace.strict` | `false` | report markers doing a legitimate job too |

`onType` is debounced, and every check spawns a process, so `onSave` is the
default.

## How it reads your file

The buffer is checked as the file it actually is. A `.md` document resolves
under prose rules with fenced code blocks held to the stricter source rules, and
any `overrides` in your `.untrace.json` scoped to that path apply.

Unsaved changes are checked too, because the buffer is passed to untrace rather
than the file on disk.

## Not here yet

Image and document metadata. Those are reported by the command line tool but do
not appear as diagnostics, since a `.png` has no text to put a squiggle on.

## Development

```
npm install
npm run typecheck
npm test                 # unit tests, plain node
npm run test:integration # inside a real VS Code
npm run compile
```

`F5` from this folder opens an Extension Development Host with the extension
loaded.

Positions are the part worth testing. A column from untrace counts runes while
VS Code counts UTF-16 code units, and tag characters and emoji are astral, so
every marker after one on the same line shifts if the conversion is wrong. The
integration suite asserts that against a real editor.
