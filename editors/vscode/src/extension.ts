import * as vscode from "vscode";

import { Actionable, Actions } from "./actions";
import { Freshness } from "./freshness";
import { runeSpanToUtf16, runeToUtf16 } from "./positions";
import { check, FileReport, Finding, UntraceError } from "./untrace";

const SOURCE = "untrace";

let diagnostics: vscode.DiagnosticCollection;
let output: vscode.OutputChannel;
let warnedMissingBinary = false;
const pending = new Map<string, NodeJS.Timeout>();
const freshness = new Freshness();
const lastFindings = new Map<string, Finding[]>();

export function activate(context: vscode.ExtensionContext): void {
  diagnostics = vscode.languages.createDiagnosticCollection(SOURCE);
  output = vscode.window.createOutputChannel("untrace");
  context.subscriptions.push(diagnostics, output);

  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument((doc) => void lint(doc)),
    vscode.workspace.onDidSaveTextDocument((doc) => void lint(doc)),
    vscode.languages.registerCodeActionsProvider(
      { scheme: "file" },
      new Actions((uri) => lastFindings.get(uri.toString()) ?? []),
      { providedCodeActionKinds: Actions.kinds },
    ),
    vscode.workspace.onDidCloseTextDocument((doc) => {
      diagnostics.delete(doc.uri);
      clearPending(doc);
      freshness.forget(doc.uri.toString());
      lastFindings.delete(doc.uri.toString());
    }),
    vscode.workspace.onDidChangeTextDocument((e) => {
      if (settings().run === "onType") {
        debounce(e.document);
      }
    }),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration(SOURCE)) {
        diagnostics.clear();
        vscode.workspace.textDocuments.forEach((doc) => void lint(doc));
      }
    }),
    vscode.commands.registerCommand("untrace.checkFile", () => {
      const doc = vscode.window.activeTextEditor?.document;
      if (doc) {
        lint(doc, true);
      }
    }),
  );

  vscode.workspace.textDocuments.forEach((doc) => void lint(doc));
}

export function deactivate(): void {
  pending.forEach(clearTimeout);
  pending.clear();
}

interface Settings {
  enable: boolean;
  path: string;
  run: string;
  strict: boolean;
}

function settings(): Settings {
  const c = vscode.workspace.getConfiguration(SOURCE);
  return {
    enable: c.get<boolean>("enable", true),
    path: c.get<string>("path", "untrace"),
    run: c.get<string>("run", "onSave"),
    strict: c.get<boolean>("strict", false),
  };
}

function debounce(doc: vscode.TextDocument): void {
  clearPending(doc);
  pending.set(
    doc.uri.toString(),
    setTimeout(() => {
      pending.delete(doc.uri.toString());
      lint(doc);
    }, 500),
  );
}

function clearPending(doc: vscode.TextDocument): void {
  const key = doc.uri.toString();
  const timer = pending.get(key);
  if (timer !== undefined) {
    clearTimeout(timer);
    pending.delete(key);
  }
}

async function lint(doc: vscode.TextDocument, force = false): Promise<void> {
  const cfg = settings();
  if (!cfg.enable || (cfg.run === "off" && !force)) {
    return;
  }
  // Output channels and diff views are not files anyone can fix.
  if (doc.uri.scheme !== "file" && doc.uri.scheme !== "untitled") {
    return;
  }

  const key = doc.uri.toString();
  const version = doc.version;

  try {
    const report = await check(doc.getText(), {
      binary: cfg.path,
      name: doc.uri.scheme === "file" ? doc.uri.fsPath : doc.fileName,
      strict: cfg.strict,
    });
    if (!freshness.accept(key, version)) {
      return;
    }
    lastFindings.set(doc.uri.toString(), report.findings ?? []);
    diagnostics.set(doc.uri, toDiagnostics(doc, report));
  } catch (err) {
    if (err instanceof UntraceError && err.missingBinary) {
      if (!warnedMissingBinary) {
        warnedMissingBinary = true;
        void vscode.window.showWarningMessage(err.message);
      }
      return;
    }
    output.appendLine(`${doc.uri.fsPath}: ${(err as Error).message}`);
  }
}

function toDiagnostics(doc: vscode.TextDocument, report: FileReport): vscode.Diagnostic[] {
  const out: vscode.Diagnostic[] = [];

  for (const p of report.payloads ?? []) {
    const detail = p.printable
      ? `decodes to ${JSON.stringify(p.text ?? "")}`
      : `${p.runes} characters, not printable text`;
    out.push(
      diagnostic(
        doc,
        p.line,
        p.column,
        p.runes,
        `Hidden payload (${p.scheme}) ${detail}`,
        vscode.DiagnosticSeverity.Error,
        p.scheme,
      ),
    );
  }

  for (const m of report.mixed_script ?? []) {
    out.push(
      diagnostic(
        doc,
        m.line,
        m.column,
        [...m.word].length,
        `${JSON.stringify(m.word)} mixes ${m.scripts.join(" and ")}`,
        vscode.DiagnosticSeverity.Warning,
        "mixed-script",
      ),
    );
  }

  for (const f of report.findings ?? []) {
    // Characters inside a payload are already covered by the payload itself.
    if (f.in_payload) {
      continue;
    }
    const d: Actionable = diagnostic(
      doc,
      f.line,
      f.column,
      1,
      `${f.codepoint} ${f.name} (${f.kind})`,
      f.actionable
        ? vscode.DiagnosticSeverity.Warning
        : vscode.DiagnosticSeverity.Information,
      f.codepoint,
    );
    d.finding = f;
    out.push(d);
  }

  return out;
}

/** Line and column are 1-based, and column counts runes rather than UTF-16 units. */
function diagnostic(
  doc: vscode.TextDocument,
  line: number,
  column: number,
  runes: number,
  message: string,
  severity: vscode.DiagnosticSeverity,
  code: string,
): vscode.Diagnostic {
  const row = Math.min(Math.max(line - 1, 0), Math.max(doc.lineCount - 1, 0));
  const text = doc.lineAt(row).text;

  const start = runeToUtf16(text, column - 1);
  const end = runeSpanToUtf16(text, start, runes);

  const d = new vscode.Diagnostic(
    new vscode.Range(row, start, row, Math.max(end, start + 1)),
    message,
    severity,
  );
  d.source = SOURCE;
  d.code = code;
  return d;
}
