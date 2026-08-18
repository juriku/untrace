import * as assert from "node:assert/strict";

import { LineIndex } from "./positions";

const tagH = "\u{E0068}";
const tagI = "\u{E0069}";
const zwj = "\u{200D}";

describe("LineIndex", () => {
	it("maps the first column of the first line to the origin", () => {
		const index = new LineIndex("hello\n");
		assert.deepEqual(index.range(1, 1, 1), {
			start: { line: 0, character: 0 },
			end: { line: 0, character: 1 },
		});
	});

	it("shifts every column after an astral character by one code unit", () => {
		const index = new LineIndex(`x = 1 ${tagH}${tagI}`);

		assert.equal(index.character(1, 7), 6, "first tag character");
		assert.equal(index.character(1, 8), 8, "second tag character, where column - 1 gives 7");
		assert.deepEqual(index.range(1, 8, 1), {
			start: { line: 0, character: 8 },
			end: { line: 0, character: 10 },
		});
	});

	it("spans a run of astral characters", () => {
		const index = new LineIndex(`x = 1 ${tagH}${tagI}`);
		assert.deepEqual(index.range(1, 7, 2), {
			start: { line: 0, character: 6 },
			end: { line: 0, character: 10 },
		});
	});

	it("counts each line from its own start", () => {
		const index = new LineIndex(`first${tagH}\nsecond\nthird`);
		assert.deepEqual(index.range(3, 2, 1), {
			start: { line: 2, character: 1 },
			end: { line: 2, character: 2 },
		});
	});

	it("is unaffected by a carriage return at the end of the line", () => {
		const crlf = new LineIndex("alpha\r\nbeta\r\n");
		const lf = new LineIndex("alpha\nbeta\n");
		assert.deepEqual(crlf.range(2, 3, 1), lf.range(2, 3, 1));
	});

	it("counts a tab as one code unit, not as its rendered width", () => {
		const index = new LineIndex("a\tb");
		assert.equal(index.character(1, 3), 2);
	});

	it("handles the empty line left by a trailing newline", () => {
		const index = new LineIndex("abc\n");
		assert.deepEqual(index.range(2, 1, 1), {
			start: { line: 1, character: 0 },
			end: { line: 1, character: 0 },
		});
	});

	it("clamps a column past the end of the line", () => {
		const index = new LineIndex("ab\n");
		assert.deepEqual(index.range(1, 99, 1), {
			start: { line: 0, character: 2 },
			end: { line: 0, character: 2 },
		});
	});

	it("clamps a line past the end of the document", () => {
		const index = new LineIndex("ab\n");
		assert.deepEqual(index.range(50, 1, 1), {
			start: { line: 49, character: 0 },
			end: { line: 49, character: 0 },
		});
	});

	it("treats a zero-width joiner inside an emoji sequence as one rune", () => {
		const index = new LineIndex(`a\u{1F468}${zwj}\u{1F469}b`);
		assert.equal(index.character(1, 2), 1, "first emoji");
		assert.equal(index.character(1, 3), 3, "the joiner");
		assert.equal(index.character(1, 4), 4, "second emoji");
		assert.equal(index.character(1, 5), 6, "the letter after the sequence");
	});

	it("widens a zero-rune run to one character, so it stays visible", () => {
		const index = new LineIndex("abc");
		assert.deepEqual(index.range(1, 2, 0), {
			start: { line: 0, character: 1 },
			end: { line: 0, character: 2 },
		});
	});
});
