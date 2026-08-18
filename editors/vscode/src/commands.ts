import * as fs from "node:fs/promises";
import * as path from "node:path";
import * as vscode from "vscode";

import { addFixes, workspaceEdit } from "./adapt";
import type { Linter } from "./linter";
import { LineIndex } from "./positions";
import { edits, problems, type Fix, type Scope } from "./problems";
import { check, scan, triage } from "./untrace";

export async function fixFile(linter: Linter): Promise<void> {
	const editor = vscode.window.activeTextEditor;
	if (editor === undefined) {
		return;
	}

	const fixes = await linter.fixes(editor.document, "all");
	if (fixes.length === 0) {
		vscode.window.setStatusBarMessage("untrace: nothing hidden in this file", 3000);
		return;
	}

	if (await vscode.workspace.applyEdit(workspaceEdit(editor.document.uri, fixes))) {
		vscode.window.setStatusBarMessage(`untrace: removed ${count(fixes.length)}`, 3000);
	}
}

export async function enableFixOnSave(): Promise<void> {
	await vscode.workspace
		.getConfiguration("untrace")
		.update("fixOnSave", "hidden", vscode.ConfigurationTarget.Global);
	vscode.window.setStatusBarMessage(
		"untrace: hidden characters will be removed when you save",
		4000,
	);
}

export interface Fixer {
	fixes(document: vscode.TextDocument, scope: Scope): Promise<Fix[]>;
}

export interface Planned {
	edit: vscode.WorkspaceEdit;
	characters: number;
	files: number;
	skipped: number;
	wereOpen: Set<string>;
}

export async function saveUntouched(plan: Planned): Promise<number> {
	let left = 0;
	for (const [uri] of plan.edit.entries()) {
		if (plan.wereOpen.has(uri.fsPath)) {
			left++;
			continue;
		}
		await (await vscode.workspace.openTextDocument(uri)).save();
	}
	return left;
}

export async function planWorkspaceFix(executable: () => string): Promise<Planned> {
	const plan: Planned = {
		edit: new vscode.WorkspaceEdit(),
		characters: 0,
		files: 0,
		skipped: 0,
		wereOpen: new Set(),
	};
	const open = new Map<string, vscode.TextDocument>();
	for (const document of vscode.workspace.textDocuments) {
		if (document.uri.scheme !== "file") {
			continue;
		}
		if (vscode.workspace.getWorkspaceFolder(document.uri) === undefined) {
			continue;
		}
		if (await exists(document.uri.fsPath)) {
			open.set(document.uri.fsPath, document);
			plan.wereOpen.add(document.uri.fsPath);
		}
	}

	const everything = ["--fix-homoglyphs"];

	for (const folder of vscode.workspace.workspaceFolders ?? []) {
		const reports = await scan(executable(), folder.uri.fsPath, { timeoutMs: 120_000 }, everything);
		const sorted = triage(reports, new Set(open.keys()));
		plan.skipped += sorted.skipped;

		for (const report of sorted.publish) {
			const text = await fs.readFile(report.path, "utf8");
			add(plan, vscode.Uri.file(report.path), edits(problems(report, new LineIndex(text)), "all"));
		}
	}

	for (const document of open.values()) {
		const text = document.getText();
		const report = await check(executable(), document.uri.fsPath, text, {
			cwd: path.dirname(document.uri.fsPath),
		}, everything);
		add(plan, document.uri, edits(problems(report, new LineIndex(text)), "all"));
	}
	return plan;
}

function add(plan: Planned, uri: vscode.Uri, fixes: readonly Fix[]): void {
	if (fixes.length === 0) {
		return;
	}
	addFixes(plan.edit, uri, fixes);
	plan.characters += fixes.length;
	plan.files++;
}

async function exists(file: string): Promise<boolean> {
	try {
		await fs.access(file);
		return true;
	} catch {
		return false;
	}
}

export async function fixWorkspace(
	log: vscode.OutputChannel,
	executable: () => string,
): Promise<void> {
	if ((vscode.workspace.workspaceFolders ?? []).length === 0) {
		vscode.window.showInformationMessage("untrace: open a folder to fix it.");
		return;
	}

	let plan: Planned;
	try {
		plan = await vscode.window.withProgress(
			{ location: vscode.ProgressLocation.Window, title: "untrace: looking" },
			() => planWorkspaceFix(executable),
		);
	} catch (err) {
		log.appendLine(`fixWorkspace: ${err instanceof Error ? err.message : String(err)}`);
		vscode.window.showErrorMessage("untrace: the scan failed. See the untrace output for why.");
		return;
	}

	if (plan.characters === 0) {
		vscode.window.showInformationMessage("untrace: nothing to fix in this workspace.");
		return;
	}

	const note = plan.skipped === 0 ? "" : ` ${plan.skipped} file(s) were skipped as not UTF-8.`;
	const choice = await vscode.window.showWarningMessage(
		`Remove ${count(plan.characters)} in ${plan.files} file(s)?${note}`,
		{ modal: true },
		"Remove them",
	);
	if (choice !== "Remove them") {
		return;
	}

	if (!(await vscode.workspace.applyEdit(plan.edit))) {
		vscode.window.showErrorMessage("untrace: the edit could not be applied.");
		return;
	}

	const left = await saveUntouched(plan);
	const tail = left === 0 ? "" : `, ${left} left unsaved because you had them open`;
	vscode.window.setStatusBarMessage(
		`untrace: removed ${count(plan.characters)} in ${plan.files} file(s)${tail}`,
		5000,
	);
}

export async function scanWorkspace(
	linter: Linter,
	log: vscode.OutputChannel,
	executable: () => string,
	quiet = false,
): Promise<void> {
	const folders = vscode.workspace.workspaceFolders ?? [];
	if (folders.length === 0) {
		if (!quiet) {
			vscode.window.showInformationMessage("untrace: open a folder to scan it.");
		}
		return;
	}

	await vscode.window.withProgress(
		{ location: vscode.ProgressLocation.Window, title: "untrace: scanning" },
		async () => {
			const open = openPaths();
			let flagged = 0;
			let skipped = 0;
			let alreadyOpen = 0;
			for (const folder of folders) {
				try {
					const reports = await scan(executable(), folder.uri.fsPath, { timeoutMs: 120_000 });
					const sorted = triage(reports, open);
					skipped += sorted.skipped;
					alreadyOpen += sorted.alreadyOpen;
					for (const report of sorted.publish) {
						const text = await fs.readFile(report.path, "utf8");
						linter.publish(vscode.Uri.file(report.path), problems(report, new LineIndex(text)));
						flagged++;
					}
				} catch (err) {
					log.appendLine(`${folder.uri.fsPath}: ${err instanceof Error ? err.message : String(err)}`);
					vscode.window.showErrorMessage("untrace: the scan failed. See the untrace output for why.");
					return;
				}
			}
			summarise(flagged, skipped, alreadyOpen, quiet);
		},
	);
}

function openPaths(): Set<string> {
	return new Set(
		vscode.workspace.textDocuments
			.filter((d) => d.uri.scheme === "file")
			.map((d) => d.uri.fsPath),
	);
}

function summarise(
	flagged: number,
	skipped: number,
	alreadyOpen: number,
	quiet: boolean,
): void {
	if (quiet) {
		return;
	}
	if (flagged === 0 && skipped === 0 && alreadyOpen === 0) {
		vscode.window.showInformationMessage("untrace: nothing hidden in this workspace.");
		return;
	}
	void vscode.commands.executeCommand("untrace.showSummary");
}

function count(n: number): string {
	return n === 1 ? "1 hidden character" : `${n} hidden characters`;
}
