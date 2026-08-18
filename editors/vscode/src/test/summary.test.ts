import * as assert from "node:assert/strict";
import * as vscode from "vscode";

import { statusFor } from "../status";
import { collect, render, type Row } from "../summary";
import { cyrillicEr, diagnostics, discard, open, until, zwsp } from "./harness";

function row(over: Partial<Row>): Row {
	return {
		file: "src/a.ts",
		label: "a.ts",
		message: "Zero Width Space (U+200B). untrace removes this character.",
		line: 1,
		character: 4,
		uri: "file:///w/src/a.ts",
		removable: true,
		...over,
	};
}

suite("the summary", () => {
	suiteTeardown(discard);

	test("counts characters and files in words, not bare numbers", () => {
		const html = render([row({}), row({ line: 2 }), row({ file: "src/b.ts", uri: "file:///w/src/b.ts" })]);

		assert.match(html, /3 hidden characters across 2 files\./);
		assert.match(html, /Remove them all/);
	});

	test("says one, not 1, when there is one of something", () => {
		assert.match(render([row({})]), /1 hidden character across 1 file\./);
	});

	test("offers no fix button when there is nothing to fix", () => {
		const html = render([]);

		assert.match(html, /Nothing hidden in the files checked so far\./);
		assert.doesNotMatch(html, /Remove them all/);
	});

	test("carries the line and position needed to jump to the character", () => {
		const html = render([row({ line: 7, character: 12 })]);

		assert.match(html, /data-line="6"/, "the webview is zero-based, the label is not");
		assert.match(html, /data-char="12"/);
		assert.match(html, /line 7/);
	});

	test("escapes anything that came out of a file", () => {
		const html = render([
			row({ file: "src/<script>.ts", message: 'He said "<b>hi</b>" & left' }),
		]);

		assert.doesNotMatch(html, /<script>\.ts/);
		assert.match(html, /src\/&lt;script&gt;\.ts/);
		assert.match(html, /&quot;&lt;b&gt;hi&lt;\/b&gt;&quot; &amp; left/);
	});

	test("groups every finding under its own file", () => {
		const html = render([
			row({ file: "src/a.ts" }),
			row({ file: "src/b.ts", uri: "file:///w/src/b.ts" }),
			row({ file: "src/a.ts", line: 9 }),
		]);

		assert.equal(html.match(/<h2>src\/a\.ts<\/h2>/g)?.length, 1, "a file is listed once");
		assert.equal(html.match(/<h2>src\/b\.ts<\/h2>/g)?.length, 1);
	});

	test("marks a lookalike letter as something untrace will not remove", async () => {
		const document = await open("summary.ts", `const ${cyrillicEr}assword = 1;\n`);
		await until(() => diagnostics(document).length > 0);

		const mine = collect().filter((r) => r.uri === document.uri.toString());
		assert.ok(mine.length >= 2, "the letter and the mixed-script word");
		assert.ok(
			mine.every((r) => !r.removable),
			"neither is removed by anything automatic",
		);
	});

	test("marks an invisible character as removable", async () => {
		const document = await open("summaryzw.ts", `const a = "x${zwsp}y";\n`);
		await until(() => diagnostics(document).length > 0);

		const mine = collect().filter((r) => r.uri === document.uri.toString());
		assert.equal(mine.length, 1);
		assert.equal(mine[0]?.removable, true);
	});
});

suite("the status bar", () => {
	test("says nothing when there is nothing", () => {
		assert.equal(statusFor([]), undefined);
	});

	test("shows the count and explains it on hover", () => {
		const status = statusFor([row({}), row({ file: "src/b.ts" })]);

		assert.equal(status?.text, "$(eye-closed) 2 hidden");
		assert.equal(
			status?.tooltip,
			"untrace: 2 hidden characters in 2 files. Click to see them.",
		);
	});

	test("stays singular for one finding in one file", () => {
		assert.equal(
			statusFor([row({})])?.tooltip,
			"untrace: 1 hidden character in 1 file. Click to see them.",
		);
	});

	test("counts characters, not files", () => {
		const status = statusFor([row({}), row({ line: 2 }), row({ line: 3 })]);

		assert.equal(status?.text, "$(eye-closed) 3 hidden");
		assert.match(status?.tooltip ?? "", /3 hidden characters in 1 file\./);
	});

	test("the command it runs is registered", async () => {
		const registered = await vscode.commands.getCommands(true);
		assert.ok(registered.includes("untrace.showSummary"));
	});
});
