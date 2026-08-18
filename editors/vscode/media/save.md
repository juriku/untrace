## Fixing on save

Off until you turn it on. untrace does not rewrite your files behind your back.

Two settings for `untrace.fixOnSave`:

- **`hidden`** removes invisible characters and hidden messages, and leaves
  typography alone. Nothing you can see on the page changes.
- **`all`** also rewrites typography that the policy for that file type does not
  allow, such as an em dash in source code.

The button turns on `hidden`, which is the safe one. Neither setting ever
rewrites a lookalike letter.

Turn it off again in Settings, or set `untrace.enable` to false to stop untrace
doing anything at all.
