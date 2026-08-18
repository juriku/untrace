import * as assert from "node:assert/strict";
import * as fs from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";

import type { FileReport } from "./report";
import { check, fix, scan, triage, UntraceFailed } from "./untrace";

const exe = process.env.UNTRACE_BIN ?? "untrace";

const zwsp = "\u{200B}";
const zwj = "\u{200D}";
const tagH = "\u{E0068}";
const tagI = "\u{E0069}";

describe("the untrace binary contract", () => {
	it("is on PATH, or named by UNTRACE_BIN", async () => {
		await check(exe, "probe.ts", "clean\n");
	});

	it("reports an invisible character with an empty replacement", async () => {
		const report = await check(exe, "probe.ts", `const a = "hi${zwsp}there";\n`);
		const [found] = report.findings ?? [];

		assert.ok(found, "expected a finding");
		assert.equal(found.codepoint, "U+200B");
		assert.equal(found.line, 1);
		assert.equal(found.actionable, true);
		assert.equal(found.replacement, "", "an empty replacement means delete");
	});

	it("honours --stdin-name when classifying the document", async () => {
		const source = await check(exe, "probe.ts", "text\n");
		const prose = await check(exe, "probe.md", "text\n");

		assert.equal(source.format, "source");
		assert.equal(prose.format, "prose");
	});

	it("decodes a tag payload into readable text", async () => {
		const report = await check(exe, "probe.txt", `x = 1 ${tagH}${tagI}\n`);
		const [payload] = report.payloads ?? [];

		assert.ok(payload, "expected a payload");
		assert.equal(payload.text, "hi");
		assert.equal(payload.printable, true);
		assert.equal(payload.runes, 2);
		assert.equal(payload.line, 1);
	});

	it("leaves a joiner alone inside an emoji sequence", async () => {
		const report = await check(exe, "probe.ts", `family \u{1F468}${zwj}\u{1F469} ok\n`);
		assert.deepEqual(report.findings ?? [], []);
	});

	it("reports the same joiner between letters", async () => {
		const report = await check(exe, "probe.ts", `he${zwj}llo\n`);
		const [found] = report.findings ?? [];

		assert.ok(found);
		assert.equal(found.codepoint, "U+200D");
		assert.equal(found.actionable, true);
	});

	it("returns the cleaned document from fix, leaving the rest byte for byte", async () => {
		const before = `const a = "hi${zwsp}there";\n`;
		const after = await fix(exe, "probe.ts", before);

		assert.equal(after.text, 'const a = "hithere";\n');
		assert.equal(after.report.findings?.[0]?.applied, true);
	});

	it("returns the document unchanged when there is nothing to fix", async () => {
		const before = "const a = 1;\n";
		const after = await fix(exe, "probe.ts", before);

		assert.equal(after.text, before);
	});

	it("walks a directory and reports every file, on stdout rather than stderr", async () => {
		const root = await fs.mkdtemp(path.join(os.tmpdir(), "untrace-scan-"));
		try {
			await fs.writeFile(path.join(root, "dirty.txt"), `hi${zwsp}there\n`, "utf8");
			await fs.writeFile(path.join(root, "clean.txt"), "nothing here\n", "utf8");
			await fs.mkdir(path.join(root, "nested"));
			await fs.writeFile(path.join(root, "nested", "deep.txt"), `a${zwsp}b\n`, "utf8");

			const reports = await scan(exe, root);
			const flagged = reports
				.filter((r) => (r.findings ?? []).length > 0)
				.map((r) => path.relative(root, r.path))
				.sort();

			assert.deepEqual(flagged, ["dirty.txt", path.join("nested", "deep.txt")]);
			assert.equal(reports.length, 3);
			assert.ok(reports.every((r) => r.encoding === "utf-8"));
		} finally {
			await fs.rm(root, { recursive: true, force: true });
		}
	});

	it("reports an empty directory without failing", async () => {
		const root = await fs.mkdtemp(path.join(os.tmpdir(), "untrace-scan-"));
		try {
			assert.deepEqual(await scan(exe, root), []);
		} finally {
			await fs.rm(root, { recursive: true, force: true });
		}
	});

	it("says the binary is missing rather than that the run failed", async () => {
		await assert.rejects(() => check("untrace-does-not-exist", "probe.ts", "text\n"), (err) => {
			assert.ok(err instanceof UntraceFailed);
			assert.equal(err.kind, "missing");
			return true;
		});
	});

	it("stops a binary that will not finish, and says why", async () => {
		const stub = await stubBinary("#!/bin/sh\nsleep 30\n");
		try {
			const started = Date.now();
			await assert.rejects(
				() => check(stub, "probe.ts", "text\n", { timeoutMs: 400 }),
				(err) => {
					assert.ok(err instanceof UntraceFailed);
					assert.equal(err.kind, "timeout");
					return true;
				},
			);
			assert.ok(Date.now() - started < 5000, "the timeout has to actually fire");
		} finally {
			await fs.rm(path.dirname(stub), { recursive: true, force: true });
		}
	});

	it("reports an aborted run as superseded, not as a failure", async () => {
		const stub = await stubBinary("#!/bin/sh\nsleep 30\n");
		const controller = new AbortController();
		try {
			const pending = check(stub, "probe.ts", "text\n", { signal: controller.signal });
			controller.abort();
			await assert.rejects(pending, (err) => {
				assert.ok(err instanceof UntraceFailed);
				assert.equal(err.kind, "superseded");
				return true;
			});
		} finally {
			await fs.rm(path.dirname(stub), { recursive: true, force: true });
		}
	});

	it("reports a binary that answers with something other than JSON", async () => {
		const stub = await stubBinary("#!/bin/sh\ncat > /dev/null\necho not json >&2\n");
		try {
			await assert.rejects(() => check(stub, "probe.ts", "text\n"), (err) => {
				assert.ok(err instanceof UntraceFailed);
				assert.equal(err.kind, "failed");
				return true;
			});
		} finally {
			await fs.rm(path.dirname(stub), { recursive: true, force: true });
		}
	});
});

describe("triage", () => {
	function report(over: Partial<FileReport>): FileReport {
		return { path: "/w/a.txt", format: "source", encoding: "utf-8", ...over };
	}
	const dirty = { findings: [{ line: 1, column: 1 } as never] };

	it("leaves a file the editor has open to the linter", () => {
		const sorted = triage(
			[report({ path: "/w/open.txt", ...dirty }), report({ path: "/w/shut.txt", ...dirty })],
			new Set(["/w/open.txt"]),
		);

		assert.deepEqual(
			sorted.publish.map((r) => r.path),
			["/w/shut.txt"],
		);
		assert.equal(sorted.alreadyOpen, 1);
		assert.equal(sorted.skipped, 0);
	});

	it("counts an open file as already checked rather than skipped", () => {
		const sorted = triage([report({ encoding: "utf-16le", ...dirty })], new Set(["/w/a.txt"]));

		assert.equal(sorted.alreadyOpen, 1);
		assert.equal(sorted.skipped, 0, "open wins, so the reason reported is the true one");
	});

	it("skips an encoding whose rune offsets cannot be reproduced", () => {
		const sorted = triage([report({ encoding: "utf-16le", ...dirty })], new Set());

		assert.deepEqual(sorted.publish, []);
		assert.equal(sorted.skipped, 1);
	});

	it("ignores files with nothing in them", () => {
		const sorted = triage([report({}), report({ path: "/w/b.txt", findings: [] })], new Set());

		assert.deepEqual(sorted, { publish: [], skipped: 0, alreadyOpen: 0 });
	});

	it("counts a payload or a mixed-script word as something to report", () => {
		const sorted = triage(
			[
				report({ path: "/w/p.txt", payloads: [{ scheme: "tag-ascii" } as never] }),
				report({ path: "/w/m.txt", mixed_script: [{ word: "x" } as never] }),
			],
			new Set(),
		);

		assert.equal(sorted.publish.length, 2);
	});
});

async function stubBinary(script: string): Promise<string> {
	const dir = await fs.mkdtemp(path.join(os.tmpdir(), "untrace-stub-"));
	const file = path.join(dir, "untrace-stub");
	await fs.writeFile(file, script, { mode: 0o755 });
	return file;
}
