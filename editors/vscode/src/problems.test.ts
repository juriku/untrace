import * as assert from "node:assert/strict";

import { LineIndex } from "./positions";
import { edits, mergeRewrites, problems } from "./problems";
import type { Finding, FileReport } from "./report";

const tagH = "\u{E0068}";
const tagI = "\u{E0069}";
const cyrillicEr = "\u{0440}";

function report(over: Partial<FileReport>): FileReport {
	return { path: "sample.ts", format: "source", encoding: "utf-8", ...over };
}

function finding(over: Partial<Finding>): Finding {
	return {
		line: 1,
		column: 1,
		codepoint: "U+200B",
		name: "Zero Width Space",
		kind: "hidden",
		action: "detected",
		replacement: "",
		applied: false,
		actionable: true,
		...over,
	};
}

describe("problems", () => {
	it("offers a delete for a character with an empty replacement", () => {
		const index = new LineIndex("const a = 1;");
		const [problem] = problems(report({ findings: [finding({ column: 7 })] }), index);

		assert.ok(problem);
		assert.equal(problem.severity, "warning");
		assert.equal(problem.code, "U+200B");
		assert.equal(problem.message, "Zero Width Space (U+200B). untrace removes this character.");
		assert.deepEqual(problem.fix, {
			title: "Remove the Zero Width Space",
			range: { start: { line: 0, character: 6 }, end: { line: 0, character: 7 } },
			newText: "",
		});
	});

	it("offers a substitution when the replacement is not empty", () => {
		const index = new LineIndex("foo bar");
		const [problem] = problems(
			report({
				findings: [
					finding({
						column: 4,
						codepoint: "U+00A0",
						name: "Non-Breaking Space",
						replacement: " ",
					}),
				],
			}),
			index,
		);

		assert.ok(problem);
		assert.equal(
			problem.message,
			'Non-Breaking Space (U+00A0). untrace replaces it with " ".',
		);
		assert.equal(problem.fix?.newText, " ");
		assert.equal(problem.fix?.title, 'Replace the Non-Breaking Space with " "');
	});

	it("offers no fix for a character untrace only reports", () => {
		const index = new LineIndex("let password = 1;");
		const [problem] = problems(
			report({
				findings: [
					finding({
						column: 5,
						codepoint: "U+0440",
						name: "Cyrillic Small Letter Er",
						kind: "typographic",
						actionable: false,
					}),
				],
			}),
			index,
		);

		assert.ok(problem);
		assert.equal(problem.severity, "information");
		assert.equal(problem.fix, undefined);
	});

	it("offers no fix when the binary predates the replacement field", () => {
		const bare: Finding = finding({});
		delete bare.replacement;

		const [problem] = problems(report({ findings: [bare] }), new LineIndex("ab"));

		assert.ok(problem);
		assert.equal(problem.severity, "warning");
		assert.equal(problem.fix, undefined);
	});

	it("reports a payload once instead of each character in it", () => {
		const index = new LineIndex(`x = 1 ${tagH}${tagI}`);
		const found = problems(
			report({
				findings: [
					finding({ column: 7, codepoint: "U+E0068", name: 'Tag "h"', kind: "tag", in_payload: true }),
					finding({ column: 8, codepoint: "U+E0069", name: 'Tag "i"', kind: "tag", in_payload: true }),
				],
				payloads: [
					{
						scheme: "tag-ascii",
						start: 6,
						end: 8,
						runes: 2,
						text: "hi",
						printable: true,
						line: 1,
						column: 7,
					},
				],
			}),
			index,
		);

		assert.equal(found.length, 1);
		const [problem] = found;
		assert.ok(problem);
		assert.equal(
			problem.message,
			'Hidden message: "hi". Encoded in 2 invisible characters as tag-ascii.',
		);
		assert.deepEqual(problem.fix, {
			title: "Remove this hidden message",
			range: { start: { line: 0, character: 6 }, end: { line: 0, character: 10 } },
			newText: "",
		});
	});

	it("says so when a payload is not printable text", () => {
		const index = new LineIndex("abcdef");
		const [problem] = problems(
			report({
				payloads: [
					{
						scheme: "zero-width-binary",
						start: 0,
						end: 1,
						runes: 1,
						text: "",
						printable: false,
						line: 1,
						column: 1,
					},
				],
			}),
			index,
		);

		assert.ok(problem);
		assert.equal(
			problem.message,
			"Hidden data, not printable text. Encoded in 1 invisible character as zero-width-binary.",
		);
	});

	it("spans the whole word for a mixed-script finding and offers no fix", () => {
		const word = `${cyrillicEr}assword`;
		const index = new LineIndex(`let ${word} = 1;`);
		const [problem] = problems(
			report({
				mixed_script: [{ line: 1, column: 5, word, scripts: ["Cyrillic", "Latin"] }],
			}),
			index,
		);

		assert.ok(problem);
		assert.equal(problem.code, "mixed-script");
		assert.equal(problem.fix, undefined);
		assert.deepEqual(problem.range, {
			start: { line: 0, character: 4 },
			end: { line: 0, character: 12 },
		});
		assert.match(problem.message, /mixes Cyrillic and Latin\./);
	});

	it("returns nothing for a clean report", () => {
		assert.deepEqual(problems(report({}), new LineIndex("clean")), []);
	});
});

describe("edits", () => {
	const index = new LineIndex("a b c d");
	const found = problems(
		report({
			findings: [
				finding({ column: 1 }),
				finding({
					column: 3,
					codepoint: "U+2014",
					name: "Em Dash",
					kind: "typographic",
					replacement: "-",
				}),
				finding({ column: 5, codepoint: "U+E0068", name: 'Tag "h"', kind: "tag" }),
				finding({
					column: 7,
					codepoint: "U+0440",
					name: "Cyrillic Small Letter Er",
					kind: "typographic",
					actionable: false,
				}),
			],
		}),
		index,
	);

	it("takes only the characters that occupy no space when the scope is hidden", () => {
		assert.deepEqual(
			edits(found, "hidden").map((f) => f.title),
			["Remove the Zero Width Space", 'Remove the Tag "h"'],
		);
	});

	it("takes every fixable character when the scope is all", () => {
		assert.deepEqual(
			edits(found, "all").map((f) => f.title),
			[
				"Remove the Zero Width Space",
				'Replace the Em Dash with "-"',
				'Remove the Tag "h"',
			],
		);
	});

	it("never takes a rewrite that would change the writer's script", () => {
		const withRewrite = problems(
			report({
				findings: [
					finding({
						column: 7,
						codepoint: "U+0440",
						name: "Cyrillic Small Letter Er",
						kind: "typographic",
						actionable: false,
					}),
				],
			}),
			index,
		);
		const merged = mergeRewrites(
			withRewrite,
			report({
				findings: [
					finding({
						column: 7,
						codepoint: "U+0440",
						name: "Cyrillic Small Letter Er",
						kind: "typographic",
						replacement: "p",
					}),
				],
			}),
			index,
		);

		const [problem] = merged;
		assert.ok(problem);
		assert.equal(problem.fix, undefined);
		assert.equal(
			problem.rewrite?.title,
			'Rewrite the Cyrillic Small Letter Er as "p", changing the writer\'s script',
		);
		assert.deepEqual(edits(merged, "all"), []);
	});

	it("offers no rewrite when the second report has nothing to say", () => {
		const bare = problems(
			report({
				findings: [finding({ codepoint: "U+0440", name: "Cyrillic Small Letter Er", actionable: false })],
			}),
			index,
		);
		const [problem] = mergeRewrites(bare, report({ findings: [] }), index);

		assert.ok(problem);
		assert.equal(problem.rewrite, undefined);
	});

	it("marks only a non-actionable finding as rewritable", () => {
		const found = problems(
			report({
				findings: [
					finding({ column: 1 }),
					finding({ column: 3, codepoint: "U+0440", actionable: false }),
				],
			}),
			index,
		);

		assert.deepEqual(
			found.map((p) => p.rewritable),
			[false, true],
		);
	});
});
