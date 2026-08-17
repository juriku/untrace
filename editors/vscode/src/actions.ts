import * as vscode from "vscode";

import { commentFor } from "./comments";
import { runeToUtf16 } from "./positions";
import { Finding } from "./untrace";

export const SOURCE = "untrace";

/** Marks a diagnostic as one this extension can act on, and carries the fix. */
export interface Actionable extends vscode.Diagnostic {
  finding?: Finding;
}

/**
 * A build predating the replacement field omits it entirely. Empty means delete,
 * so reading absent as empty would silently remove characters that should have
 * been normalised.
 */
function fixable(f: Finding): f is Finding & { replacement: string } {
  return f.actionable && f.replacement !== undefined;
}

function edit(doc: vscode.TextDocument, f: Finding & { replacement: string }): vscode.TextEdit {
  const line = doc.lineAt(f.line - 1).text;
  const start = runeToUtf16(line, f.column - 1);
  const end = runeToUtf16(line, f.column);

  return vscode.TextEdit.replace(
    new vscode.Range(f.line - 1, start, f.line - 1, end),
    f.replacement,
  );
}

function describe(f: Finding & { replacement: string }): string {
  return f.replacement === ""
    ? `Remove ${f.codepoint}`
    : `Replace ${f.codepoint} with ${JSON.stringify(f.replacement)}`;
}

function fix(doc: vscode.TextDocument, f: Finding & { replacement: string }, d: vscode.Diagnostic): vscode.CodeAction {
  const action = new vscode.CodeAction(describe(f), vscode.CodeActionKind.QuickFix);
  action.edit = new vscode.WorkspaceEdit();
  action.edit.set(doc.uri, [edit(doc, f)]);
  action.diagnostics = [d];
  action.isPreferred = true;
  return action;
}

function fixEveryOfKind(doc: vscode.TextDocument, same: (Finding & { replacement: string })[]): vscode.CodeAction {
  const label = `Fix all ${same.length} ${same[0]!.name} in this file`;
  const action = new vscode.CodeAction(label, vscode.CodeActionKind.QuickFix);
  action.edit = new vscode.WorkspaceEdit();
  action.edit.set(
    doc.uri,
    same.map((f) => edit(doc, f)),
  );
  return action;
}

function suppress(doc: vscode.TextDocument, line: number): vscode.CodeAction | undefined {
  const comment = commentFor(doc.languageId);
  if (comment === undefined) {
    return undefined;
  }

  const text = doc.lineAt(line).text;
  const directive = `  ${comment.open} untrace:ignore${comment.close && ` ${comment.close}`}`;

  const action = new vscode.CodeAction("Ignore this line", vscode.CodeActionKind.QuickFix);
  action.edit = new vscode.WorkspaceEdit();
  action.edit.set(doc.uri, [
    vscode.TextEdit.insert(new vscode.Position(line, text.length), directive),
  ]);
  return action;
}

export class Actions implements vscode.CodeActionProvider {
  static readonly kinds = [
    vscode.CodeActionKind.QuickFix,
    vscode.CodeActionKind.SourceFixAll.append(SOURCE),
  ];

  constructor(private readonly findingsFor: (uri: vscode.Uri) => Finding[]) {}

  provideCodeActions(
    doc: vscode.TextDocument,
    range: vscode.Range | vscode.Selection,
    context: vscode.CodeActionContext,
  ): vscode.CodeAction[] {
    const all = this.findingsFor(doc.uri).filter(fixable);
    if (all.length === 0) {
      return [];
    }

    const out: vscode.CodeAction[] = [];

    const here = context.diagnostics.filter(
      (d): d is Actionable => d.source === SOURCE && (d as Actionable).finding !== undefined,
    );

    for (const d of here) {
      const f = d.finding!;
      if (!fixable(f)) {
        continue;
      }
      out.push(fix(doc, f, d));

      const same = all.filter((o) => o.codepoint === f.codepoint);
      if (same.length > 1) {
        out.push(fixEveryOfKind(doc, same));
      }
    }

    if (here.length > 0) {
      const ignore = suppress(doc, range.start.line);
      if (ignore !== undefined) {
        out.push(ignore);
      }
    }

    out.push(this.fixAll(doc, all));
    return out;
  }

  private fixAll(doc: vscode.TextDocument, all: (Finding & { replacement: string })[]): vscode.CodeAction {
    const action = new vscode.CodeAction(
      `Fix all ${all.length} in this file`,
      vscode.CodeActionKind.SourceFixAll.append(SOURCE),
    );
    action.edit = new vscode.WorkspaceEdit();
    action.edit.set(
      doc.uri,
      all.map((f) => edit(doc, f)),
    );
    return action;
  }
}
