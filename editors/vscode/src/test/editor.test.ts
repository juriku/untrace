import * as assert from "node:assert/strict";
import * as path from "node:path";
import * as vscode from "vscode";

import { fixAllKind, fixAllTitle } from "../actions";
import { fix } from "../untrace";
import {
	actions,
	diagnostics,
	discard,
	focus,
	nbsp,
	open,
	recheck,
	tagH,
	tagI,
	whole,
	zwsp,
} from "./harness";

suite("in a real editor", () => {
	suiteTeardown(discard);

	test("offers autoFix exactly what it needs, at the caret", async () => {
		const document = await open("auto.txt", `hi${zwsp}there\n`);
		await focus(document);

		const [found] = diagnostics(document);
		assert.ok(found);

		const caret = new vscode.Range(found.range.start, found.range.start);
		const preferred = (await actions(document, caret)).filter((a) => a.isPreferred);

		assert.equal(preferred.length, 1, "autoFix applies a preferred quickfix, and only one");
		assert.equal(preferred[0]?.kind?.value, vscode.CodeActionKind.QuickFix.value);
		assert.ok(preferred[0]?.edit, "an action with no edit would apply nothing");

		assert.ok(await vscode.workspace.applyEdit(preferred[0].edit));
		assert.equal(document.getText(), "hithere\n");
	});

	test("the quick fix menu reaches the provider", async () => {
		const document = await open("menu.txt", `hi${zwsp}there\n`);
		const [found] = diagnostics(document);
		assert.ok(found);

		const offered = await actions(document, found.range);
		assert.deepEqual(
			offered.map((a) => a.title),
			["untrace: Remove the Zero Width Space", fixAllTitle],
		);
	});

	test("fixing everything produces exactly what --fix produces", async () => {
		const text = `a${zwsp}b ${nbsp}c ${tagH}${tagI}\n`;
		const document = await open("everything.txt", text);

		const offered = await actions(document, whole(document));
		const all = offered.find((a) => a.title === fixAllTitle);
		assert.ok(all?.edit, "expected a whole-file action carrying an edit");
		assert.ok(await vscode.workspace.applyEdit(all.edit));

		const cli = await fix("untrace", document.uri.fsPath, text, [], {
			cwd: path.dirname(document.uri.fsPath),
		});
		assert.equal(document.getText(), cli.text);
		await recheck(document);
		assert.deepEqual(diagnostics(document), []);
	});

	test("source.fixAll.untrace is reachable as a source action", async () => {
		const document = await open("source.txt", `a${zwsp}b\n`);
		const offered = await actions(document, whole(document), fixAllKind);

		assert.equal(offered.length, 1);
		assert.equal(offered[0]?.title, fixAllTitle);
	});

	test("a fix stays correct after the buffer is edited", async () => {
		const document = await open("moved.txt", `hi${zwsp}there\n`);
		const editor = await focus(document);
		await editor.edit((builder) => builder.insert(new vscode.Position(0, 0), "let x = "));
		await recheck(document);

		const [found] = diagnostics(document);
		assert.ok(found);
		const preferred = (await actions(document, found.range)).find(
			(a) => a.title !== fixAllTitle,
		);
		assert.ok(preferred?.edit);
		assert.ok(await vscode.workspace.applyEdit(preferred.edit));

		assert.equal(document.getText(), "let x = hithere\n");
	});

	test("a check overtaken by an edit still leaves a usable fix behind", async () => {
		const document = await open("overtaken.txt", `hi${zwsp}there\n`);
		const editor = await focus(document);

		const inflight = recheck(document);
		await editor.edit((builder) => builder.insert(new vscode.Position(0, 0), "let x = "));
		await inflight;

		const [found] = diagnostics(document);
		assert.ok(found, "the diagnostic must survive");
		const preferred = (await actions(document, found.range)).find((a) => a.isPreferred);
		assert.ok(preferred?.edit, "a visible diagnostic must keep an applicable fix");
		assert.ok(await vscode.workspace.applyEdit(preferred.edit));
		assert.equal(document.getText(), "let x = hithere\n");
	});

	test("clears its diagnostics when the file stops having any", async () => {
		const document = await open("cleared.txt", `a${zwsp}b\n`);
		assert.equal(diagnostics(document).length, 1);

		const editor = await focus(document);
		await editor.edit((builder) =>
			builder.replace(whole(document), "nothing to see here\n"),
		);
		await recheck(document);

		assert.deepEqual(diagnostics(document), []);
	});
});
