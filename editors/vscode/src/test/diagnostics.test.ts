import * as assert from "node:assert/strict";
import * as vscode from "vscode";

import { cyrillicEr, diagnostics, discard, open, tagH, tagI, zwsp } from "./harness";

suite("diagnostics", () => {
	suiteTeardown(discard);

	test("reports an invisible character with the name and what untrace will do", async () => {
		const document = await open("sample.txt", `hi${zwsp}there\n`);
		const [found] = diagnostics(document);

		assert.ok(found, "expected one diagnostic");
		assert.equal(found.severity, vscode.DiagnosticSeverity.Warning);
		assert.equal(found.code, "U+200B");
		assert.equal(found.message, "Zero Width Space (U+200B). untrace removes this character.");
		assert.deepEqual(found.range, new vscode.Range(0, 2, 0, 3));
	});

	test("reports a hidden payload once, with the decoded text, not once per character", async () => {
		const document = await open("payload.txt", `x = 1 ${tagH}${tagI}\n`);
		const found = diagnostics(document);

		assert.equal(found.length, 1, "one diagnostic for the whole run");
		assert.equal(
			found[0]?.message,
			'Hidden message: "hi". Encoded in 2 invisible characters as tag-ascii.',
		);
		assert.deepEqual(found[0]?.range, new vscode.Range(0, 6, 0, 10));
	});

	test("reports a confusable letter as information, not a warning", async () => {
		const document = await open("confusable.ts", `let ${cyrillicEr}assword = 1;\n`);
		const found = diagnostics(document);

		const letter = found.find((d) => d.code === "U+0440");
		assert.ok(letter, "expected the Cyrillic letter to be reported");
		assert.equal(letter.severity, vscode.DiagnosticSeverity.Information);
		assert.match(letter.message, /untrace reports this and does not change it\./);

		const mixed = found.find((d) => d.code === "mixed-script");
		assert.ok(mixed, "expected the mixed-script word to be reported");
		assert.equal(mixed.severity, vscode.DiagnosticSeverity.Warning);
	});

	test("reports nothing for a clean file", async () => {
		const document = await open("clean.txt", "nothing to see here\n");
		assert.deepEqual(diagnostics(document), []);
	});

	test("leaves a joiner alone inside an emoji sequence", async () => {
		const document = await open("emoji.txt", "family \u{1F468}\u{200D}\u{1F469} ok\n");
		assert.deepEqual(diagnostics(document), []);
	});
});
