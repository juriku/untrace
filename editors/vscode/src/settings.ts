import * as vscode from "vscode";

import type { Scope } from "./problems";

export type OnSave = "off" | Scope;

export interface Settings {
	enabled: boolean;
	executable: string;
	timeoutMs: number;
	maxBytes: number;
	fixOnSave: OnSave;
	checkWholeWorkspaceOnStartup: boolean;
}

export function settingsFor(document?: vscode.TextDocument): Settings {
	const config = vscode.workspace.getConfiguration("untrace", document ?? null);
	const onSave = config.get<string>("fixOnSave");

	return {
		enabled: config.get<boolean>("enable") ?? true,
		executable: config.get<string>("path")?.trim() || "untrace",
		timeoutMs: config.get<number>("timeout") ?? 5000,
		maxBytes: (config.get<number>("maxFileSizeKB") ?? 5120) * 1024,
		fixOnSave: onSave === "hidden" || onSave === "all" ? onSave : "off",
		checkWholeWorkspaceOnStartup: config.get<boolean>("checkWholeWorkspaceOnStartup") ?? true,
	};
}
