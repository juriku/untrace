import * as assert from "node:assert/strict";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import * as vscode from "vscode";

import { planWorkspaceFix, saveUntouched } from "../commands";
import { diagnostics, discard, focus, until, whole, zwsp } from "./harness";

function root(): string {
	const folder = vscode.workspace.workspaceFolders?.[0];
	assert.ok(folder, "these tests need a folder open");
	return folder.uri.fsPath;
}

async function write(name: string, content: string): Promise<vscode.Uri> {
	const file = path.join(root(), name);
	await fs.writeFile(file, content, "utf8");
	return vscode.Uri.file(file);
}

function ours(uri: vscode.Uri): vscode.Diagnostic[] {
	return vscode.languages.getDiagnostics(uri).filter((d) => d.source === "untrace");
}

async function clear(): Promise<void> {
	await vscode.commands.executeCommand("workbench.action.revertAndCloseActiveEditor");
	await vscode.commands.executeCommand("workbench.action.closeAllEditors");
	for (const entry of await fs.readdir(root())) {
		await fs.rm(path.join(root(), entry), { recursive: true, force: true });
	}
}

suite("scanning the workspace", () => {
	suiteTeardown(async () => {
		await discard();
		await clear();
	});

	test("reports a file nobody has opened", async () => {
		const uri = await write("never-opened.txt", `hi${zwsp}there\n`);

		await vscode.commands.executeCommand("untrace.scanWorkspace");
		await until(() => ours(uri).length > 0, 20000);

		const [found] = ours(uri);
		assert.equal(found?.code, "U+200B");
		assert.deepEqual(found?.range, new vscode.Range(0, 2, 0, 3));
	});

	test("leaves an open dirty buffer to the linter", async () => {
		const uri = await write("open-and-dirty.txt", `hi${zwsp}there\n`);
		const document = await vscode.workspace.openTextDocument(uri);
		const editor = await focus(document);

		await editor.edit((builder) => builder.insert(new vscode.Position(0, 0), "prefix "));
		await until(() => diagnostics(document)[0]?.range.start.character === 9, 10000);

		await vscode.commands.executeCommand("untrace.scanWorkspace");
		await new Promise((resolve) => setTimeout(resolve, 1500));

		const [found] = diagnostics(document);
		assert.ok(found);
		assert.equal(
			found.range.start.character,
			9,
			"the scan must not move the squiggle back to where disk says",
		);
		assert.equal(document.getText(), `prefix hi${zwsp}there\n`);
	});

	test("a fix offered after a scan still edits the right character", async () => {
		const uri = await write("fix-after-scan.txt", `a${zwsp}b\n`);

		await vscode.commands.executeCommand("untrace.scanWorkspace");
		await until(() => ours(uri).length > 0, 20000);

		const document = await vscode.workspace.openTextDocument(uri);
		const editor = await focus(document);
		const [found] = diagnostics(document);
		assert.ok(found);

		const offered = await vscode.commands.executeCommand<vscode.CodeAction[]>(
			"vscode.executeCodeActionProvider",
			uri,
			found.range,
		);
		const preferred = (offered ?? []).find((a) => a.isPreferred);
		assert.ok(preferred?.edit);
		assert.ok(await vscode.workspace.applyEdit(preferred.edit));

		assert.equal(document.getText(), "ab\n");
		assert.ok(editor);
	});

	test("plans one edit covering every file, opened or not", async () => {
		await clear();
		const untouched = await write("untouched.ts", `const a = "x${zwsp}y";\n`);
		const opened = await write("opened.ts", `const b = "p${zwsp}q";\n`);
		const clean = await write("spotless.ts", "const c = 1;\n");

		const document = await vscode.workspace.openTextDocument(opened);
		await focus(document);

		const plan = await planWorkspaceFix(() => "untrace");
		assert.deepEqual(
			plan.edit
				.entries()
				.map(([uri]) => path.basename(uri.fsPath))
				.sort(),
			["opened.ts", "untouched.ts"],
		);
		assert.equal(plan.characters, 2);
		assert.equal(plan.skipped, 0);

		assert.ok(await vscode.workspace.applyEdit(plan.edit));
		const left = await saveUntouched(plan);

		assert.equal(document.getText(), 'const b = "pq";\n', "the open buffer is edited, not the disk file under it");
		assert.equal(left, 1, "the file the user had open is left for them to save");
		assert.ok(document.isDirty, "and it stays dirty rather than being committed for them");

		assert.equal(
			await fs.readFile(untouched.fsPath, "utf8"),
			'const a = "xy";\n',
			"a file nobody had open is written to disk",
		);
		assert.equal(await fs.readFile(clean.fsPath, "utf8"), "const c = 1;\n");
	});

	test("rewrites a lookalike letter, which no other path does", async () => {
		await clear();
		const lookalike = await write("account.ts", "const \u{0440}assword = 1;\n");
		const invisible = await write("token.ts", `const a = "x${zwsp}y";\n`);

		const plan = await planWorkspaceFix(() => "untrace");
		assert.deepEqual(
			plan.edit
				.entries()
				.map(([uri]) => path.basename(uri.fsPath))
				.sort(),
			["account.ts", "token.ts"],
		);

		assert.ok(await vscode.workspace.applyEdit(plan.edit));
		await saveUntouched(plan);

		assert.equal(await fs.readFile(lookalike.fsPath, "utf8"), "const password = 1;\n");
		assert.equal(await fs.readFile(invisible.fsPath, "utf8"), 'const a = "xy";\n');
	});

	test("leaves nothing behind, so the summary empties", async () => {
		await clear();
		await write("mixed.ts", `const \u{0440}assword = "x${zwsp}y";\n`);

		const plan = await planWorkspaceFix(() => "untrace");
		assert.ok(await vscode.workspace.applyEdit(plan.edit));
		await saveUntouched(plan);

		const after = await planWorkspaceFix(() => "untrace");
		assert.equal(after.characters, 0, "a second pass must find nothing");
	});

	test("plans nothing when the workspace is already clean", async () => {
		await clear();
		await vscode.commands.executeCommand("workbench.action.closeAllEditors");
		await write("spotless.ts", "const c = 1;\n");

		const plan = await planWorkspaceFix(() => "untrace");
		assert.equal(plan.characters, 0);
		assert.equal(plan.files, 0);
	});

	test("says nothing is hidden when the workspace is clean", async () => {
		for (const entry of await fs.readdir(root())) {
			await fs.rm(path.join(root(), entry), { recursive: true, force: true });
		}
		await vscode.commands.executeCommand("workbench.action.closeAllEditors");
		const uri = await write("all-clean.txt", "nothing to see here\n");

		await vscode.commands.executeCommand("untrace.scanWorkspace");
		await new Promise((resolve) => setTimeout(resolve, 1500));

		assert.deepEqual(ours(uri), []);
		assert.ok(whole);
	});
});
