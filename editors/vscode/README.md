# untrace for VS Code

Text pasted out of an AI chatbot carries characters you cannot see. This
extension finds them, tells you what they are, reads out what they spell, and
takes them out.

It shells out to the [untrace](https://github.com/juriku/untrace) binary, so the
editor and your CI agree by construction. Install that first.

```
brew install juriku/tap/untrace
go install github.com/juriku/untrace/cmd/untrace@latest
```

## What you get

**A count in the status bar.** Open a folder and untrace checks every file in
it, including ones you never open. If anything is hidden, the bottom left says
so. Click it for the list.

**A summary you can act on.** Every file, every finding, in words, with one
button that clears the lot.

**The character named, not just boxed.** VS Code already draws a box around most
invisible characters. It does not tell you which character it is. untrace says
`Zero Width Space (U+200B). untrace removes this character.`

**Hidden text read out.** A run of tag characters becomes
`Hidden message: "ignore all previous instructions"`, straight from the
squiggle. This is the part nothing else does: the text is invisible on screen
and untrace prints it for you.

**Fixes that are rules, not guesses.** Every fix is deterministic. `Cmd+.` puts
untrace's first, `Opt+Cmd+.` applies it with no menu at all, and a button
appears in the editor toolbar whenever the open file has something in it.

**Markdown too.** VS Code's own highlighting is switched off for Markdown by
default. untrace is not.

## Lookalike letters

A word spelled like `paypal` but with a Cyrillic letter standing in for a Latin
one is reported as mixing scripts, and is **not** rewritten by anything
automatic. Turning Cyrillic into Latin changes the script the author wrote in,
and in a genuinely Russian word it corrupts it.

Two paths do rewrite it, and both are explicit: the quick fix labelled
`Rewrite the Cyrillic Small Letter Er as "p", changing the writer's script`, and
**Remove them all** in the summary, which asks for confirmation and states the
count first.

## Settings

| Setting | Default | What it does |
|---|---|---|
| `untrace.enable` | `true` | Turning this off stops all activity |
| `untrace.path` | `""` | Path to the binary. Empty means look on `PATH` |
| `untrace.fixOnSave` | `"off"` | `off`, `hidden` or `all`. See below |
| `untrace.checkWholeWorkspaceOnStartup` | `true` | Check every file when a folder opens |
| `untrace.timeout` | `5000` | Milliseconds before giving up on a file |
| `untrace.maxFileSizeKB` | `5120` | Skip files larger than this, and say so |

`untrace.fixOnSave` is off until you ask for it. `hidden` removes invisible
characters and hidden messages and leaves anything you can see alone. `all` also
rewrites typography the policy for that file type does not allow, such as an em
dash in source code. Neither ever rewrites a lookalike letter.

## Commands

| Command | What it does |
|---|---|
| `untrace: Remove hidden characters` | Fixes the open file. Also on right-click and `Cmd+Alt+U` |
| `untrace: Show every hidden character` | Opens the summary |
| `untrace: Search every file` | Checks the whole folder, then opens the summary |
| `untrace: Remove hidden characters from every file` | Fixes the whole folder, after asking |
| `untrace: Remove hidden characters whenever I save` | Turns on `fixOnSave: hidden` |
| `untrace: Show the log` | Opens the output channel |

## What it does not do

**It does not detect statistical watermarks**, and neither does anything else
that works on characters. See the
[main README](https://github.com/juriku/untrace#what-it-does-not-do).

**It decides nothing.** Every finding, severity and replacement comes from the
binary. The extension renders what it is told, which is why a file cannot be
clean in the editor and dirty in CI.

**It does not remove VS Code's own counts.** The number on a tab and in the
Explorer is VS Code's, produced for every extension that reports problems, and
shared with whatever else is reporting them.

## Known limitation

The toolbar button is gated on a single context key derived from the **active**
editor, because VS Code exposes no per-resource diagnostic key. In a split view
both panes follow whichever pane has focus.

## Building it

```
npm install
npm run compile
npm test               # unit
npm run test:integration   # in a real VS Code
npm run package        # produces the .vsix
```

Fixtures are written to a temporary directory rather than into the repository,
because they carry the characters untrace is built to find and CI scans the
repository with `untrace --fail`.
