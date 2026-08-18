import * as vscode from "vscode";

import { source } from "./adapt";

export interface Row {
	file: string;
	label: string;
	message: string;
	line: number;
	character: number;
	uri: string;
	removable: boolean;
}

function untraceWillRemove(d: vscode.Diagnostic): boolean {
	const reportOnly = d.severity === vscode.DiagnosticSeverity.Information;
	const lookalikeWord = d.code === "mixed-script";
	return !reportOnly && !lookalikeWord;
}

export function collect(): Row[] {
	const rows: Row[] = [];
	for (const [uri, all] of vscode.languages.getDiagnostics()) {
		for (const d of all.filter((x) => x.source === source)) {
			rows.push({
				file: vscode.workspace.asRelativePath(uri),
				label: uri.path.split("/").pop() ?? uri.path,
				message: d.message,
				line: d.range.start.line + 1,
				character: d.range.start.character,
				uri: uri.toString(),
				removable: untraceWillRemove(d),
			});
		}
	}
	return rows.sort((a, b) => a.file.localeCompare(b.file) || a.line - b.line);
}

export class Summary {
	private panel: vscode.WebviewPanel | undefined;

	show(): void {
		if (this.panel === undefined) {
			this.panel = vscode.window.createWebviewPanel(
				"untrace.summary",
				"untrace",
				{ viewColumn: vscode.ViewColumn.Active, preserveFocus: false },
				{ enableScripts: true },
			);
			this.panel.onDidDispose(() => {
				this.panel = undefined;
			});
			this.panel.webview.onDidReceiveMessage((message: { open?: string; line?: number; character?: number; fix?: boolean }) => {
				if (message.fix === true) {
					void vscode.commands.executeCommand("untrace.fixWorkspace");
					return;
				}
				if (message.open !== undefined) {
					const at = new vscode.Position(message.line ?? 0, message.character ?? 0);
					void vscode.window.showTextDocument(vscode.Uri.parse(message.open), {
						selection: new vscode.Range(at, at),
					});
				}
			});
		}
		this.panel.webview.html = render(collect());
		this.panel.reveal(vscode.ViewColumn.Active);
	}

	refresh(): void {
		if (this.panel !== undefined) {
			this.panel.webview.html = render(collect());
		}
	}

	dispose(): void {
		this.panel?.dispose();
	}
}

function escape(text: string): string {
	return text.replace(
		/[&<>"']/g,
		(c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c] ?? c,
	);
}

export function render(rows: readonly Row[]): string {
	const files = [...new Set(rows.map((r) => r.file))];
	const headline =
		rows.length === 0
			? "Nothing hidden in the files checked so far."
			: `${plural(rows.length, "hidden character")} across ${plural(files.length, "file")}.`;

	const body =
		rows.length === 0
			? `<p class="empty">untrace checks a file when you open it. To check every file in the folder,
			   including ones you have never opened, run <b>untrace: Search every file</b>.</p>`
			: files
					.map((file) => {
						const mine = rows.filter((r) => r.file === file);
						return `<section>
							<h2>${escape(file)}</h2>
							<ul>${mine
								.map(
									(r) => `<li data-uri="${escape(r.uri)}" data-line="${r.line - 1}" data-char="${r.character}">
										<span class="where">line ${r.line}</span>
										<span class="what">${escape(r.message)}</span>
									</li>`,
								)
								.join("")}</ul>
						</section>`;
					})
					.join("");

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline';">
<style>
  body { font-family: var(--vscode-font-family); color: var(--vscode-foreground);
         padding: 1.5rem 2rem; line-height: 1.5; }
  h1 { font-size: 1.35rem; font-weight: 600; margin: 0 0 .25rem; }
  .sub { color: var(--vscode-descriptionForeground); margin: 0 0 1.5rem; }
  button { font: inherit; padding: .5rem 1rem; border: none; border-radius: 2px; cursor: pointer;
           background: var(--vscode-button-background); color: var(--vscode-button-foreground); }
  button:hover { background: var(--vscode-button-hoverBackground); }
  section { margin: 1.5rem 0; }
  h2 { font-size: .95rem; font-weight: 600; margin: 0 0 .35rem;
       color: var(--vscode-textLink-foreground); }
  ul { list-style: none; margin: 0; padding: 0; border-left: 2px solid var(--vscode-panel-border); }
  li { display: flex; gap: .75rem; padding: .3rem .75rem; cursor: pointer; }
  li:hover { background: var(--vscode-list-hoverBackground); }
  .where { color: var(--vscode-descriptionForeground); min-width: 5rem; }
  .empty { color: var(--vscode-descriptionForeground); max-width: 42rem; }
</style>
</head>
<body>
  <h1>${escape(headline)}</h1>
  <p class="sub">Characters that are in your files but that you cannot see.</p>
  ${rows.length === 0 ? "" : '<button id="fix">Remove them all</button>'}
  ${body}
<script>
  const api = acquireVsCodeApi();
  document.getElementById("fix")?.addEventListener("click", () => api.postMessage({ fix: true }));
  for (const row of document.querySelectorAll("li[data-uri]")) {
    row.addEventListener("click", () => api.postMessage({
      open: row.dataset.uri,
      line: Number(row.dataset.line),
      character: Number(row.dataset.char),
    }));
  }
</script>
</body>
</html>`;
}

function plural(n: number, noun: string): string {
	return n === 1 ? `1 ${noun}` : `${n} ${noun}s`;
}
