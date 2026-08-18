import * as assert from "node:assert/strict";
import * as vscode from "vscode";

import {
	actions,
	cyrillicEr,
	diagnostics,
	discard,
	emDash,
	focus,
	open,
	recheck,
	until,
	whole,
	zwsp,
} from "./harness";

suite("commands", () => {
	suiteTeardown(discard);

	test("untrace.fixFile removes everything untrace can fix in the open file", async () => {
		const document = await open("command.txt", `hi${zwsp}there ${emDash} ok\n`);
		await focus(document);

		await vscode.commands.executeCommand("untrace.fixFile");
		await until(() => document.getText() === "hithere - ok\n");
	});

	test("untrace.fixFile leaves a clean file alone", async () => {
		const before = "nothing to see here\n";
		const document = await open("commandclean.txt", before);
		await focus(document);

		await vscode.commands.executeCommand("untrace.fixFile");
		assert.equal(document.getText(), before);
	});

	test("untrace.fixFile never rewrites a lookalike letter", async () => {
		const before = `let ${cyrillicEr}assword = secret;\n`;
		const document = await open("commandlookalike.ts", before);
		await focus(document);

		await vscode.commands.executeCommand("untrace.fixFile");
		assert.equal(document.getText(), before);
	});

	test("the context menu and keybinding are gated on there being findings", async () => {
		const dirty = await open("gated.txt", `a${zwsp}b\n`);
		await focus(dirty);
		await until(() => diagnostics(dirty).length > 0);

		assert.ok(
			vscode.languages.getDiagnostics(dirty.uri).some((d) => d.source === "untrace"),
			"the context key would be true here",
		);

		const clean = await open("ungated.txt", "nothing to see here\n");
		await focus(clean);
		assert.deepEqual(
			vscode.languages.getDiagnostics(clean.uri).filter((d) => d.source === "untrace"),
			[],
			"the context key would be false here",
		);
	});

	test("a lookalike letter costs nothing extra until someone opens the lightbulb", async () => {
		const document = await open("lazy.ts", `let ${cyrillicEr}assword = secret;\n`);
		await recheck(document);

		const letter = diagnostics(document).find((d) => d.code === "U+0440");
		assert.ok(letter, "the letter is reported by the ordinary check");

		const before = await actions(document, letter.range);
		assert.equal(before.length, 1, "the rewrite arrives with the first lightbulb");

		const after = await actions(document, letter.range);
		assert.deepEqual(
			after.map((a) => a.title),
			before.map((a) => a.title),
			"and is cached, so a second lightbulb is free",
		);
	});

	test("a lookalike letter is offered a rewrite, and it is never preferred", async () => {
		const document = await open("rewrite.ts", `let ${cyrillicEr}assword = secret;\n`);
		const letter = diagnostics(document).find((d) => d.code === "U+0440");
		assert.ok(letter, "expected the letter to be reported");

		const offered = await actions(document, letter.range);
		assert.deepEqual(
			offered.map((a) => a.title),
			['untrace: Rewrite the Cyrillic Small Letter Er as "p", changing the writer\'s script'],
		);
		assert.notEqual(offered[0]?.isPreferred, true);
	});

	test("the rewrite really does replace the letter when applied", async () => {
		const document = await open("rewriteapply.ts", `let ${cyrillicEr}assword = secret;\n`);
		const letter = diagnostics(document).find((d) => d.code === "U+0440");
		assert.ok(letter);

		const [offered] = await actions(document, letter.range);
		assert.ok(offered?.edit);
		assert.ok(await vscode.workspace.applyEdit(offered.edit));

		assert.equal(document.getText(), "let password = secret;\n");
	});

	test("fixing everything still skips the lookalike letter", async () => {
		const document = await open("rewriteskip.ts", `let ${cyrillicEr}assword = "a${zwsp}b";\n`);
		const offered = await actions(document, whole(document));
		const all = offered.find((a) => a.title.includes("every hidden character"));
		assert.ok(all?.edit);
		assert.ok(await vscode.workspace.applyEdit(all.edit));

		assert.equal(document.getText(), `let ${cyrillicEr}assword = "ab";\n`);
	});
});
