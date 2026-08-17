# untrace for VS Code

Reports invisible characters, homoglyphs and hidden payloads as diagnostics
while you edit.

## Requirements

The `untrace` binary must be on your `PATH`:

```
brew install juriku/tap/untrace
go install github.com/juriku/untrace/cmd/untrace@latest
```

Set `untrace.path` if it lives somewhere else.

## What you get

| finding | severity |
|---|---|
| decoded hidden payload | Error |
| mixed-script word, such as `pаypal` | Warning |
| a character `--fix` would change | Warning |
| a marker reported but not rewritten | Information |

Characters inside a decoded payload are folded into the payload's own
diagnostic rather than reported one by one.

The document is checked as the file it is: a `.md` buffer resolves under prose
rules with fenced code blocks held to source rules, and any `overrides` in your
`.untrace.json` scoped to that path apply. Unsaved changes are checked too,
since the buffer is passed to untrace rather than the file on disk.

## Settings

| setting | default | meaning |
|---|---|---|
| `untrace.enable` | `true` | report findings at all |
| `untrace.path` | `untrace` | path to the binary |
| `untrace.run` | `onSave` | `onSave`, `onType` or `off` |
| `untrace.strict` | `false` | report markers doing a legitimate job too |

`onType` is debounced, and every check spawns a process, so `onSave` is the
default.

## Not here yet

Quick fixes, inserting `untrace:ignore`, and image and document metadata. This
version exists to get positions right first; a column in untrace counts runes
while VS Code counts UTF-16 units, and tag characters and emoji are astral, so
the conversion is where the bugs live.

## Development

```
npm install
npm run typecheck
npm test
npm run compile
```
