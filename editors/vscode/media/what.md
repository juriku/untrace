## What untrace adds

VS Code already draws a box around most invisible characters. It does not tell
you which character it is, what it spells, or how to get rid of it.

untrace does four things the editor does not:

- **Names it.** `Zero Width Space (U+200B)`, not "this character is invisible".
- **Decodes it.** A run of tag characters becomes `Hidden message: "hi"`, read
  straight from the squiggle.
- **Catches lookalike words.** A word whose first letter is Cyrillic and whose
  rest is Latin is reported as mixing scripts, which is how a fake `password`
  is disguised. It is never rewritten without you asking.
- **Covers Markdown.** The built-in highlighting is switched off there by
  default. untrace is not.

Everything it reports comes from the same binary your CI and pre-commit hook
run, so the editor and the build can never disagree.
