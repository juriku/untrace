import * as fsSync from "node:fs";
import * as fs from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";
import * as vscode from "vscode";

import { toDiagnostic } from "../adapt";
import type { Api } from "../extension";
import { LineIndex } from "../positions";
import { problems, type Problem } from "../problems";
import { check } from "../untrace";

let root: string | undefined;
let sequence = 0;

async function fixtures(): Promise<string> {
	if (root === undefined) {
		root = await fs.mkdtemp(path.join(os.tmpdir(), "untrace-vscode-"));
		const at = root;
		process.on("exit", () => fsSync.rmSync(at, { recursive: true, force: true }));
	}
	return root;
}

export const zwsp = "\u{200B}";
export const nbsp = "\u{00A0}";
export const tagH = "\u{E0068}";
export const tagI = "\u{E0069}";
export const cyrillicEr = "\u{0440}";
export const emDash = "\u{2014}";

export async function api(): Promise<Api> {
	const extension = vscode.extensions.getExtension<Api>("juriku.untrace");
	if (extension === undefined) {
		throw new Error("the untrace extension is not present in this host");
	}
	return extension.activate();
}

export async function open(name: string, content: string): Promise<vscode.TextDocument> {
	const dir = await fixtures();
	sequence += 1;
	const file = path.join(dir, `${sequence}-${name}`);
	await fs.writeFile(file, content, "utf8");

	const document = await vscode.workspace.openTextDocument(vscode.Uri.file(file));
	await recheck(document);
	return document;
}

export async function recheck(document: vscode.TextDocument): Promise<void> {
	await (await api()).check(document);
}

export async function discard(): Promise<void> {
	await vscode.commands.executeCommand("workbench.action.closeAllEditors");
}

export async function focus(document: vscode.TextDocument): Promise<vscode.TextEditor> {
	await vscode.commands.executeCommand("workbench.action.closeAllEditors");
	const editor = await vscode.window.showTextDocument(document);
	await until(() => vscode.window.activeTextEditor?.document === document);
	await recheck(document);
	return editor;
}

export async function typeInto(
	document: vscode.TextDocument,
	content: string,
): Promise<vscode.TextEditor> {
	const editor = await focus(document);
	await editor.edit((builder) => builder.replace(whole(document), content));
	await recheck(document);
	return editor;
}

export function diagnostics(document: vscode.TextDocument): vscode.Diagnostic[] {
	return vscode.languages
		.getDiagnostics(document.uri)
		.filter((d) => d.source === "untrace")
		.sort((a, b) => a.range.start.compareTo(b.range.start));
}

export async function actions(
	document: vscode.TextDocument,
	range: vscode.Range,
	kind?: vscode.CodeActionKind,
): Promise<vscode.CodeAction[]> {
	const found = await vscode.commands.executeCommand<vscode.CodeAction[]>(
		"vscode.executeCodeActionProvider",
		document.uri,
		range,
		kind?.value,
	);
	return (found ?? []).filter((a) => a.title.startsWith("untrace: "));
}

export async function analyse(document: vscode.TextDocument): Promise<Problem[]> {
	const text = document.getText();
	const report = await check("untrace", document.uri.fsPath, text, {
		cwd: path.dirname(document.uri.fsPath),
	});
	return problems(report, new LineIndex(text));
}

export function context(
	found: readonly Problem[],
	only?: vscode.CodeActionKind,
): vscode.CodeActionContext {
	return {
		diagnostics: found.map(toDiagnostic),
		triggerKind: vscode.CodeActionTriggerKind.Invoke,
		only,
	};
}

export function whole(document: vscode.TextDocument): vscode.Range {
	return document.validateRange(new vscode.Range(0, 0, document.lineCount, 0));
}

export async function until(
	condition: () => boolean | Promise<boolean>,
	timeoutMs = 5000,
): Promise<void> {
	const deadline = Date.now() + timeoutMs;
	while (!(await condition())) {
		if (Date.now() > deadline) {
			throw new Error("timed out waiting for the editor to settle");
		}
		await new Promise((resolve) => setTimeout(resolve, 25));
	}
}
