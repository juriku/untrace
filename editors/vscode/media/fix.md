## Removing them

Three ways, in order of least effort:

1. **Option+Cmd+.** on a squiggle applies untrace's fix with no menu at all.
   On Windows and Linux, `Ctrl+.` then Enter.
2. **Cmd+.** opens the list. untrace's entry is first and starts with
   `untrace:`, so you can tell it from anything else offering to help.
3. **Right-click, Remove hidden characters** fixes the whole file.

Every fix is a fixed rule, not a guess. Removing a zero-width space always
removes exactly that character.

The one thing untrace will not do on its own is rewrite a lookalike letter,
because turning Cyrillic into Latin changes the script the author used. It
offers that as a separate, clearly labelled action you have to pick yourself.
