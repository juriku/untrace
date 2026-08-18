import * as vscode from "vscode";

import { Fixes } from "./actions";
import { source } from "./adapt";
import { enableFixOnSave, fixFile, fixWorkspace, scanWorkspace } from "./commands";
import { Linter } from "./linter";
import type { Fix, Scope } from "./problems";
import { fixOnSave } from "./save";
import { settingsFor } from "./settings";
import { showStatus } from "./status";
import { Summary } from "./summary";
import { reportTrouble } from "./trouble";

export interface Api {
	check(document: vscode.TextDocument): Promise<void>;
	fixes(document: vscode.TextDocument, scope: Scope): Promise<Fix[]>;
}

export function activate(context: vscode.ExtensionContext): Api {
	const log = vscode.window.createOutputChannel(source, { log: true });
	const linter = new Linter(log);
	const summary = new Summary();


	context.subscriptions.push(
		log,
		linter,
		summary,
		showStatus(),
		vscode.commands.registerCommand("untrace.showSummary", () => summary.show()),
		vscode.languages.onDidChangeDiagnostics(() => summary.refresh()),
		reportTrouble(linter, context.globalState, log),
		vscode.languages.registerCodeActionsProvider({ scheme: "file" }, new Fixes(linter), Fixes.metadata),
		fixOnSave(linter),
		vscode.commands.registerCommand("untrace.fixFile", () => fixFile(linter)),
		vscode.commands.registerCommand("untrace.scanWorkspace", () =>
			scanWorkspace(linter, log, () => settingsFor().executable),
		),
		vscode.commands.registerCommand("untrace.fixWorkspace", () =>
			fixWorkspace(log, () => settingsFor().executable),
		),
		vscode.commands.registerCommand("untrace.showLog", () => log.show()),
		vscode.commands.registerCommand("untrace.enableFixOnSave", () => enableFixOnSave()),
		vscode.workspace.onDidOpenTextDocument((document) => linter.schedule(document)),
		vscode.workspace.onDidChangeTextDocument((event) => linter.schedule(event.document)),
		vscode.workspace.onDidSaveTextDocument((document) => void linter.check(document)),
		vscode.workspace.onDidCloseTextDocument((document) => linter.forget(document)),
		vscode.workspace.onDidChangeConfiguration((event) => {
			if (affectsChecking(event)) {
				recheckEverything(linter);
			}
		}),
		vscode.window.onDidChangeActiveTextEditor(() => updateHasFindingsContext()),
		vscode.languages.onDidChangeDiagnostics(() => updateHasFindingsContext()),
	);

	recheckEverything(linter);
	updateHasFindingsContext();

	if (settingsFor().checkWholeWorkspaceOnStartup) {
		void scanWorkspace(linter, log, () => settingsFor().executable, true);
	}

	return {
		check: (document) => linter.check(document),
		fixes: (document, scope) => linter.fixes(document, scope),
	};
}

export function deactivate(): void {}

const checkingSettings = ["enable", "path", "timeout", "maxFileSizeKB"];

function affectsChecking(event: vscode.ConfigurationChangeEvent): boolean {
	return checkingSettings.some((key) => event.affectsConfiguration(`${source}.${key}`));
}

function recheckEverything(linter: Linter): void {
	for (const document of vscode.workspace.textDocuments) {
		linter.schedule(document);
	}
}

function updateHasFindingsContext(): void {
	const editor = vscode.window.activeTextEditor;
	const has =
		editor !== undefined &&
		vscode.languages.getDiagnostics(editor.document.uri).some((d) => d.source === source);
	void vscode.commands.executeCommand("setContext", "untrace.hasFindings", has);
}
