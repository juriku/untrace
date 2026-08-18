import * as vscode from "vscode";

import type { Linter, Trouble } from "./linter";

const silenceKey = "untrace.silenceMissingBinary";
const installUrl = "https://github.com/juriku/untrace#install";

export function reportTrouble(
	linter: Linter,
	memento: vscode.Memento,
	log: vscode.LogOutputChannel,
): vscode.Disposable {
	const status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 0);
	status.name = "untrace";
	status.command = "untrace.showLog";
	status.backgroundColor = new vscode.ThemeColor("statusBarItem.errorBackground");

	let told = false;
	const listener = linter.onTrouble((trouble) => {
		if (trouble.kind !== "missing") {
			return;
		}
		status.text = "$(circle-slash) untrace not found";
		status.tooltip = trouble.detail;
		status.show();

		if (told || memento.get<boolean>(silenceKey) === true) {
			return;
		}
		told = true;
		void offer(memento, log, trouble);
	});

	return vscode.Disposable.from(status, listener, {
		dispose: () => {
			told = true;
		},
	});
}

async function offer(
	memento: vscode.Memento,
	log: vscode.LogOutputChannel,
	trouble: Trouble,
): Promise<void> {
	const choice = await vscode.window.showWarningMessage(
		`untrace: ${trouble.detail}. Set untrace.path, or install it, and hidden characters will be found again.`,
		"How to install",
		"Open settings",
		"Do not show again",
	);

	switch (choice) {
		case "How to install":
			void vscode.env.openExternal(vscode.Uri.parse(installUrl));
			break;
		case "Open settings":
			void vscode.commands.executeCommand("workbench.action.openSettings", "untrace.path");
			break;
		case "Do not show again":
			await memento.update(silenceKey, true);
			log.info("untrace will not report the missing binary again in this workspace.");
			break;
		default:
			break;
	}
}
