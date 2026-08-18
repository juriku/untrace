import * as assert from "node:assert/strict";
import * as vscode from "vscode";

import { diagnostics, discard, focus, open, recheck, until, whole, zwsp } from "./harness";

async function set(key: string, value: unknown): Promise<void> {
	await vscode.workspace
		.getConfiguration("untrace")
		.update(key, value, vscode.ConfigurationTarget.Global);
}

async function reset(): Promise<void> {
	for (const key of ["enable", "path", "timeout", "maxFileSizeKB"]) {
		await set(key, undefined);
	}
}

suite("robustness", () => {
	suiteTeardown(async () => {
		await reset();
		await discard();
	});

	test("a burst of edits settles, leaving the diagnostics correct", async () => {
		await reset();
		const document = await open("burst.txt", "start\n");
		const editor = await focus(document);

		for (let i = 0; i < 60; i++) {
			await editor.edit((builder) => builder.insert(new vscode.Position(0, 0), "x"));
		}
		await editor.edit((builder) => builder.replace(whole(document), `done${zwsp}here\n`));

		await until(() => diagnostics(document).length === 1, 15000);
		const [found] = diagnostics(document);
		assert.equal(found?.code, "U+200B");
		assert.deepEqual(found?.range, new vscode.Range(0, 4, 0, 5));
	});

	test("untrace.enable false clears everything and stops checking", async () => {
		await reset();
		const document = await open("switch.txt", `a${zwsp}b\n`);
		assert.equal(diagnostics(document).length, 1);

		await set("enable", false);
		await until(() => diagnostics(document).length === 0);

		await recheck(document);
		assert.deepEqual(diagnostics(document), []);

		await set("enable", true);
		await until(() => diagnostics(document).length === 1);
	});

	test("a file over the size limit says it was skipped rather than going quiet", async () => {
		await reset();
		await set("maxFileSizeKB", 1);
		const document = await open("big.txt", `${"filler ".repeat(400)}a${zwsp}b\n`);
		await recheck(document);

		const [found] = diagnostics(document);
		assert.ok(found, "a skipped file must say so");
		assert.equal(found.severity, vscode.DiagnosticSeverity.Information);
		assert.match(found.message, /untrace skipped this file/);
		assert.match(found.message, /untrace\.maxFileSizeKB/);
	});

	test("an oversized file is not re-announced on every check", async () => {
		await reset();
		await set("maxFileSizeKB", 1);
		const document = await open("republish.txt", `${"filler ".repeat(400)}a${zwsp}b\n`);
		await recheck(document);

		let changes = 0;
		const watching = vscode.languages.onDidChangeDiagnostics((event) => {
			if (event.uris.some((u) => u.toString() === document.uri.toString())) {
				changes++;
			}
		});
		try {
			await recheck(document);
			await recheck(document);
			await recheck(document);
			assert.equal(changes, 0, "the same skip note must not be published again");
		} finally {
			watching.dispose();
		}
	});

	test("a file under the size limit is checked as usual", async () => {
		await reset();
		await set("maxFileSizeKB", 5120);
		const document = await open("small.txt", `a${zwsp}b\n`);
		await recheck(document);

		assert.equal(diagnostics(document)[0]?.code, "U+200B");
	});

	test("a missing binary clears the diagnostics instead of leaving stale ones", async () => {
		await reset();
		const document = await open("nobinary.txt", `a${zwsp}b\n`);
		assert.equal(diagnostics(document).length, 1);

		await set("path", "/nonexistent/untrace");
		await until(() => diagnostics(document).length === 0, 10000);

		await set("path", undefined);
		await until(() => diagnostics(document).length === 1, 10000);
	});

	test("a binary that never answers does not wedge the editor", async () => {
		await reset();
		const document = await open("slow.txt", `a${zwsp}b\n`);
		await set("path", "/bin/sleep");
		await set("timeout", 300);

		const started = Date.now();
		await recheck(document);
		assert.ok(Date.now() - started < 8000, "the check has to give up, not hang");

		await set("path", undefined);
		await set("timeout", undefined);
		await until(() => diagnostics(document).length === 1, 10000);
	});
});
