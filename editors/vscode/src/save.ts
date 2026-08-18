import * as vscode from "vscode";

import { textEdits } from "./adapt";
import type { Linter } from "./linter";
import type { Scope } from "./problems";
import { settingsFor } from "./settings";

export function fixOnSave(linter: Linter): vscode.Disposable {
	return vscode.workspace.onWillSaveTextDocument((event) => {
		const settings = settingsFor(event.document);
		if (!settings.enabled || settings.fixOnSave === "off") {
			return;
		}
		event.waitUntil(pending(linter, event.document, settings.fixOnSave));
	});
}

async function pending(
	linter: Linter,
	document: vscode.TextDocument,
	scope: Scope,
): Promise<vscode.TextEdit[]> {
	return textEdits(await linter.fixes(document, scope));
}
