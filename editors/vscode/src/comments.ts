/**
 * VS Code exposes no API for a language's comment syntax, so the tokens are
 * listed here. A language absent from the map gets no suppress action rather
 * than a guessed token written into the file.
 */
const lineComment: Record<string, string> = {
  bat: "REM",
  c: "//",
  clojure: ";;",
  coffeescript: "#",
  cpp: "//",
  csharp: "//",
  dart: "//",
  dockerfile: "#",
  elixir: "#",
  erlang: "%",
  fsharp: "//",
  go: "//",
  graphql: "#",
  groovy: "//",
  haskell: "--",
  ini: ";",
  java: "//",
  javascript: "//",
  javascriptreact: "//",
  json5: "//",
  jsonc: "//",
  julia: "#",
  kotlin: "//",
  latex: "%",
  less: "//",
  lua: "--",
  makefile: "#",
  markdown: "<!--",
  nix: "#",
  objectivec: "//",
  perl: "#",
  php: "//",
  powershell: "#",
  properties: "#",
  python: "#",
  r: "#",
  ruby: "#",
  rust: "//",
  scala: "//",
  scss: "//",
  shellscript: "#",
  sql: "--",
  swift: "//",
  terraform: "#",
  toml: "#",
  typescript: "//",
  typescriptreact: "//",
  vb: "'",
  xml: "<!--",
  yaml: "#",
  zig: "//",
};

// Markup has no line comment, so the directive is wrapped and closed.
const blockClose: Record<string, string> = {
  markdown: "-->",
  xml: "-->",
};

export interface Directive {
  open: string;
  close: string;
}

export function commentFor(languageId: string): Directive | undefined {
  const open = lineComment[languageId];
  if (open === undefined) {
    return undefined;
  }
  return { open, close: blockClose[languageId] ?? "" };
}
