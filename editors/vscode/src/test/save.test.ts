import * as assert from "node:assert/strict";
import * as fs from "node:fs/promises";
import * as vscode from "vscode";

import { cyrillicEr, discard, emDash, open, typeInto, zwsp } from "./harness";

async function setFixOnSave(value: string): Promise<void> {
	await vscode.workspace
		.getConfiguration("untrace")
		.update("fixOnSave", value, vscode.ConfigurationTarget.Global);
}

async function typeThenSave(
	name: string,
	content: string,
): Promise<{ buffer: string; disk: string }> {
	const document = await open(name, "");
	await typeInto(document, content);
	assert.ok(document.isDirty, "the save hook only runs on a dirty document");
	assert.ok(await document.save());

	return { buffer: document.getText(), disk: await fs.readFile(document.uri.fsPath, "utf8") };
}

suite("saving", () => {
	suiteTeardown(async () => {
		await setFixOnSave("off");
		await discard();
	});

	test("changes nothing by default", async () => {
		await setFixOnSave("off");
		const content = `hi${zwsp}there ${emDash} ok\n`;
		const saved = await typeThenSave("untouched.txt", content);

		assert.equal(saved.buffer, content);
		assert.equal(saved.disk, content);
	});

	test("removes invisible characters and leaves typography alone at hidden", async () => {
		await setFixOnSave("hidden");
		const saved = await typeThenSave("hidden.txt", `hi${zwsp}there ${emDash} ok\n`);

		assert.equal(saved.buffer, `hithere ${emDash} ok\n`);
		assert.equal(saved.disk, `hithere ${emDash} ok\n`);
	});

	test("rewrites typography too at all", async () => {
		await setFixOnSave("all");
		const saved = await typeThenSave("all.txt", `hi${zwsp}there ${emDash} ok\n`);

		assert.equal(saved.buffer, "hithere - ok\n");
		assert.equal(saved.disk, "hithere - ok\n");
	});

	test("removes a hidden message at hidden", async () => {
		await setFixOnSave("hidden");
		const saved = await typeThenSave("payloadsave.txt", "x = 1 \u{E0068}\u{E0069}\n");

		assert.equal(saved.buffer, "x = 1 \n");
	});

	test("never rewrites a lookalike letter, at any setting", async () => {
		await setFixOnSave("all");
		const content = `let ${cyrillicEr}assword = secret;\n`;
		const saved = await typeThenSave("lookalike.ts", content);

		assert.equal(saved.buffer, content);
	});

	test("leaves a clean file untouched", async () => {
		await setFixOnSave("all");
		const content = "nothing to see here\n";
		const saved = await typeThenSave("cleansave.txt", content);

		assert.equal(saved.buffer, content);
		assert.equal(saved.disk, content);
	});
});
