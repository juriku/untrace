import * as vscode from "vscode";

import { collect, type Row } from "./summary";

export interface Status {
	text: string;
	tooltip: string;
}

export function statusFor(rows: readonly Row[]): Status | undefined {
	if (rows.length === 0) {
		return undefined;
	}
	const files = new Set(rows.map((r) => r.file)).size;
	return {
		text: `$(eye-closed) ${rows.length} hidden`,
		tooltip: `untrace: ${plural(rows.length, "hidden character")} in ${plural(files, "file")}. Click to see them.`,
	};
}

export function showStatus(): vscode.Disposable {
	const item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
	item.name = "untrace";
	item.command = "untrace.showSummary";

	const update = (): void => {
		const status = statusFor(collect());
		if (status === undefined) {
			item.hide();
			return;
		}
		item.text = status.text;
		item.tooltip = status.tooltip;
		item.show();
	};

	update();
	return vscode.Disposable.from(item, vscode.languages.onDidChangeDiagnostics(update));
}

function plural(n: number, noun: string): string {
	return n === 1 ? `1 ${noun}` : `${n} ${noun}s`;
}
