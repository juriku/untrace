import * as assert from "node:assert/strict";
import * as vscode from "vscode";

import { Fixes, fixAllKind, fixAllTitle, type Problems } from "../actions";
import { toRange } from "../adapt";
import type { Problem } from "../problems";
import {
	analyse,
	context,
	cyrillicEr,
	discard,
	nbsp,
	open,
	tagH,
	tagI,
	whole,
	zwsp,
} from "./harness";

class Known implements Problems {
	constructor(private readonly found: Problem[] | undefined) {}
	current(): Problem[] | undefined {
		return this.found;
	}
	async refresh(): Promise<Problem[] | undefined> {
		return this.found;
	}
	async withRewrites(): Promise<Problem[] | undefined> {
		return this.found;
	}
}

const uncancelled = new vscode.CancellationTokenSource().token;

type Where = (document: vscode.TextDocument, found: Problem[]) => vscode.Range;

async function provide(
	document: vscode.TextDocument,
	at: Where,
	only?: vscode.CodeActionKind,
): Promise<{ found: Problem[]; actions: vscode.CodeAction[] }> {
	const found = await analyse(document);
	const actions = await new Fixes(new Known(found)).provideCodeActions(
		document,
		at(document, found),
		context(found, only),
		uncancelled,
	);
	return { found, actions };
}

const first: Where = (_document, found) => {
	const problem = found[0];
	assert.ok(problem, "expected at least one problem");
	return toRange(problem.range);
};

suite("the code action provider", () => {
	suiteTeardown(discard);

	test("every actionable finding carries a quick fix", async () => {
		const document = await open("offered.txt", `hi${zwsp}there\n`);
		const { actions } = await provide(document, first);

		assert.ok(actions.length > 0, "a diagnostic with no code action reads as unfixable");
	});

	test("offers exactly one preferred fix, and attaches its diagnostic", async () => {
		const document = await open("preferred.txt", `hi${zwsp}there\n`);
		const { actions } = await provide(document, first);

		const preferred = actions.filter((a) => a.isPreferred);
		assert.equal(preferred.length, 1);
		assert.equal(preferred[0]?.title, "untrace: Remove the Zero Width Space");
		assert.equal(preferred[0]?.kind?.value, vscode.CodeActionKind.QuickFix.value);
		assert.equal(
			preferred[0]?.diagnostics?.length,
			1,
			"an action with no diagnostic attached sorts below ones that have them",
		);
	});

	test("offers fixing the whole file second, never ahead of the single fix", async () => {
		const document = await open("second.txt", `hi${zwsp}there\n`);
		const { actions } = await provide(document, first);

		const all = actions.filter((a) => a.title === fixAllTitle);
		assert.equal(all.length, 1);
		assert.notEqual(all[0]?.isPreferred, true);
		assert.equal(actions.indexOf(all[0]!), actions.length - 1);
	});

	test("names the substitution when the character is replaced rather than removed", async () => {
		const document = await open("replace.txt", `foo${nbsp}bar\n`);
		const { actions } = await provide(document, first);

		const preferred = actions.find((a) => a.isPreferred);
		assert.equal(preferred?.title, 'untrace: Replace the Non-Breaking Space with " "');
	});

	test("offers one fix for a whole hidden payload, not one per character", async () => {
		const document = await open("payload.txt", `x = 1 ${tagH}${tagI}\n`);
		const { actions } = await provide(document, first);

		const preferred = actions.filter((a) => a.isPreferred);
		assert.equal(preferred.length, 1);
		assert.equal(preferred[0]?.title, "untrace: Remove this hidden message");
	});

	test("offers the fix from anywhere on the line, not only on the character", async () => {
		const document = await open("caret.txt", `const key = "ab${zwsp}cd";\n`);
		const end = document.lineAt(0).range.end;
		const { actions } = await provide(document, () => new vscode.Range(end, end));

		const preferred = actions.filter((a) => a.isPreferred);
		assert.equal(preferred.length, 1);
		assert.equal(preferred[0]?.title, "untrace: Remove the Zero Width Space");
	});

	test("offers only the whole-file fix from a line that has no finding on it", async () => {
		const document = await open("otherline.txt", `clean line\nhi${zwsp}there\n`);
		const { actions } = await provide(document, () => new vscode.Range(0, 0, 0, 10));

		assert.deepEqual(
			actions.map((a) => a.title),
			[fixAllTitle],
		);
	});

	test("offers nothing for a confusable letter", async () => {
		const document = await open("confusable.ts", `let ${cyrillicEr}assword = 1;\n`);
		const { actions } = await provide(document, whole);

		assert.deepEqual(actions, []);
	});

	test("offers nothing for a clean file", async () => {
		const document = await open("clean.txt", "nothing to see here\n");
		const { actions } = await provide(document, whole);

		assert.deepEqual(actions, []);
	});

	test("offers only the whole-file action when source.fixAll is requested", async () => {
		const document = await open("sourceaction.txt", `a${zwsp}b\n`);
		const { actions } = await provide(document, whole, vscode.CodeActionKind.SourceFixAll);

		assert.equal(actions.length, 1);
		assert.equal(actions[0]?.title, fixAllTitle);
		assert.equal(actions[0]?.kind?.value, fixAllKind.value);
	});

	test("offers nothing when neither the cache nor a fresh check has anything", async () => {
		const document = await open("stale.txt", `a${zwsp}b\n`);
		const empty = new Fixes(new Known(undefined));

		assert.deepEqual(
			await empty.provideCodeActions(document, whole(document), context([]), uncancelled),
			[],
		);
	});

	test("recomputes rather than giving up when the cache is behind the buffer", async () => {
		const document = await open("behind.txt", `a${zwsp}b\n`);
		const found = await analyse(document);
		let refreshed = false;
		const behind: Problems = {
			current: () => undefined,
			refresh: async () => {
				refreshed = true;
				return found;
			},
			withRewrites: async () => found,
		};

		const offered = await new Fixes(behind).provideCodeActions(
			document,
			whole(document),
			context(found),
			uncancelled,
		);

		assert.ok(refreshed, "a cache miss must trigger a fresh check");
		assert.ok(offered.some((a) => a.isPreferred));
	});
});
