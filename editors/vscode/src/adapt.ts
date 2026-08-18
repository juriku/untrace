import * as vscode from "vscode";

import type { Range } from "./positions";
import type { Fix, Problem } from "./problems";

export const source = "untrace";

export function toRange(range: Range): vscode.Range {
	return new vscode.Range(
		range.start.line,
		range.start.character,
		range.end.line,
		range.end.character,
	);
}

export function workspaceEdit(uri: vscode.Uri, fixes: readonly Fix[]): vscode.WorkspaceEdit {
	const edit = new vscode.WorkspaceEdit();
	addFixes(edit, uri, fixes);
	return edit;
}

export function addFixes(
	edit: vscode.WorkspaceEdit,
	uri: vscode.Uri,
	fixes: readonly Fix[],
): void {
	for (const fix of fixes) {
		edit.replace(uri, toRange(fix.range), fix.newText);
	}
}

export function textEdits(fixes: readonly Fix[]): vscode.TextEdit[] {
	return fixes.map((fix) => vscode.TextEdit.replace(toRange(fix.range), fix.newText));
}

export function toDiagnostic(problem: Problem): vscode.Diagnostic {
	const diagnostic = new vscode.Diagnostic(
		toRange(problem.range),
		problem.message,
		problem.severity === "warning"
			? vscode.DiagnosticSeverity.Warning
			: vscode.DiagnosticSeverity.Information,
	);
	diagnostic.source = source;
	diagnostic.code = problem.code;
	return diagnostic;
}
