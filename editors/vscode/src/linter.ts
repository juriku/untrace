import * as path from "node:path";
import * as vscode from "vscode";

import { source, toDiagnostic } from "./adapt";
import { LineIndex } from "./positions";
import { edits, mergeRewrites, problems, type Fix, type Problem, type Scope } from "./problems";
import { settingsFor, type Settings } from "./settings";
import { check, UntraceFailed } from "./untrace";

const debounceMs = 250;

interface Checked {
	version: number;
	problems: Problem[];
}

type Outcome =
	| { state: "checked"; problems: Problem[] }
	| { state: "stale" }
	| { state: "stopped" };

export interface Trouble {
	kind: "missing" | "timeout" | "failed";
	detail: string;
}

export class Linter implements vscode.Disposable {
	private readonly collection = vscode.languages.createDiagnosticCollection(source);
	private readonly checked = new Map<string, Checked>();
	private readonly pending = new Map<string, NodeJS.Timeout>();
	private readonly running = new Map<string, AbortController>();
	private readonly trouble = new vscode.EventEmitter<Trouble>();

	readonly onTrouble = this.trouble.event;

	constructor(private readonly log: vscode.LogOutputChannel) {}

	current(document: vscode.TextDocument): Problem[] | undefined {
		const entry = this.checked.get(document.uri.toString());
		return entry?.version === document.version ? entry.problems : undefined;
	}

	publish(uri: vscode.Uri, found: readonly Problem[]): void {
		this.collection.set(uri, found.map(toDiagnostic));
	}

	refresh(document: vscode.TextDocument): Promise<Problem[] | undefined> {
		return this.analyse(document);
	}

	async withRewrites(document: vscode.TextDocument): Promise<Problem[] | undefined> {
		const key = document.uri.toString();
		const entry = this.checked.get(key);
		if (entry === undefined || entry.version !== document.version) {
			return this.analyse(document);
		}
		if (!entry.problems.some((p) => p.rewritable && p.rewrite === undefined)) {
			return entry.problems;
		}

		const settings = settingsFor(document);
		const name = document.uri.fsPath;
		try {
			const rewrites = await check(settings.executable, name, document.getText(), {
				cwd: path.dirname(name),
				timeoutMs: settings.timeoutMs,
			}, ["--fix-homoglyphs"]);
			if (document.version !== entry.version) {
				return entry.problems;
			}
			const merged = mergeRewrites(entry.problems, rewrites, new LineIndex(document.getText()));
			this.checked.set(key, { version: entry.version, problems: merged });
			return merged;
		} catch (err) {
			this.log.error(`${name}: ${err instanceof Error ? err.message : String(err)}`);
			return entry.problems;
		}
	}

	async check(document: vscode.TextDocument): Promise<void> {
		await this.analyse(document);
	}

	async fixes(document: vscode.TextDocument, scope: Scope): Promise<Fix[]> {
		return edits((await this.analyse(document)) ?? [], scope);
	}

	schedule(document: vscode.TextDocument): void {
		if (document.uri.scheme !== "file") {
			return;
		}
		const key = document.uri.toString();
		this.cancel(key);
		this.pending.set(
			key,
			setTimeout(() => void this.analyse(document), debounceMs),
		);
	}

	private async analyse(document: vscode.TextDocument): Promise<Problem[] | undefined> {
		const key = document.uri.toString();
		this.cancel(key);
		if (document.uri.scheme !== "file") {
			return undefined;
		}

		const settings = settingsFor(document);
		if (!settings.enabled) {
			this.forget(document);
			return undefined;
		}
		if (this.tooBig(document, settings.maxBytes)) {
			return undefined;
		}

		for (let attempt = 0; attempt < 3; attempt++) {
			const outcome = await this.once(document, key, settings);
			if (outcome.state === "checked") {
				return outcome.problems;
			}
			if (outcome.state === "stopped") {
				return undefined;
			}
		}
		this.schedule(document);
		return undefined;
	}

	private async once(
		document: vscode.TextDocument,
		key: string,
		settings: Settings,
	): Promise<Outcome> {
		const version = document.version;
		const text = document.getText();
		const name = document.uri.fsPath;

		this.running.get(key)?.abort();
		const controller = new AbortController();
		this.running.set(key, controller);

		const options = {
			cwd: path.dirname(name),
			timeoutMs: settings.timeoutMs,
			signal: controller.signal,
		};
		const started = Date.now();

		try {
			const report = await check(settings.executable, name, text, options);
			this.log.debug(`${name} checked in ${Date.now() - started}ms`);

			if (document.version !== version) {
				return { state: "stale" };
			}
			const found = problems(report, new LineIndex(text));
			this.checked.set(key, { version, problems: found });
			this.publish(document.uri, found);
			return { state: "checked", problems: found };
		} catch (err) {
			return this.trip(document, err);
		} finally {
			if (this.running.get(key) === controller) {
				this.running.delete(key);
			}
		}
	}

	private trip(document: vscode.TextDocument, err: unknown): Outcome {
		if (err instanceof UntraceFailed && err.kind === "superseded") {
			return { state: "stopped" };
		}
		const kind = err instanceof UntraceFailed && err.kind !== "superseded" ? err.kind : "failed";
		const detail = err instanceof Error ? err.message : String(err);

		this.log.error(`${document.uri.fsPath}: ${detail}`);
		this.forget(document);
		this.trouble.fire({ kind, detail });
		return { state: "stopped" };
	}

	private tooBig(document: vscode.TextDocument, max: number): boolean {
		const bytes = Buffer.byteLength(document.getText(), "utf8");
		if (bytes <= max) {
			return false;
		}
		const message = `untrace skipped this file: ${Math.round(bytes / 1024)} KB is over the ${Math.round(max / 1024)} KB limit set by untrace.maxFileSizeKB.`;
		this.checked.delete(document.uri.toString());

		const published = this.collection.get(document.uri);
		if (published?.length === 1 && published[0]?.message === message) {
			return true;
		}
		const note = new vscode.Diagnostic(
			new vscode.Range(0, 0, 0, 0),
			message,
			vscode.DiagnosticSeverity.Information,
		);
		note.source = source;
		this.collection.set(document.uri, [note]);
		return true;
	}

	forget(document: vscode.TextDocument): void {
		const key = document.uri.toString();
		this.cancel(key);
		this.running.get(key)?.abort();
		this.running.delete(key);
		this.checked.delete(key);
		this.collection.delete(document.uri);
	}

	private cancel(key: string): void {
		const timer = this.pending.get(key);
		if (timer !== undefined) {
			clearTimeout(timer);
			this.pending.delete(key);
		}
	}

	dispose(): void {
		for (const timer of this.pending.values()) {
			clearTimeout(timer);
		}
		for (const controller of this.running.values()) {
			controller.abort();
		}
		this.pending.clear();
		this.running.clear();
		this.checked.clear();
		this.collection.dispose();
		this.trouble.dispose();
	}
}
