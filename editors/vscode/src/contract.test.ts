import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { test } from "node:test";

import type { FileReport, Finding, Report } from "./untrace.ts";

// Only the fields the extension reads are listed. Extra ones on the Go side are
// ignored; a renamed or removed one breaks the extension silently.
// Resolved from the package directory, which is where npm runs the script.
const golden = resolve("../../testdata/cases/json-output/want.txt");

const required = {
  report: ["version", "files"] satisfies (keyof Report)[],
  file: ["path", "format", "encoding", "findings"] satisfies (keyof FileReport)[],
  finding: [
    "line",
    "column",
    "codepoint",
    "name",
    "kind",
    "action",
    "applied",
    "actionable",
  ] satisfies (keyof Finding)[],
};

test("the json golden carries every field the extension requires", () => {
  const report = JSON.parse(readFileSync(golden, "utf8")) as Record<string, unknown>;

  for (const key of required.report) {
    assert.ok(key in report, `report is missing ${key}`);
  }

  const files = report["files"] as Record<string, unknown>[];
  assert.ok(files.length > 0, "the golden has no files");

  for (const key of required.file) {
    assert.ok(key in files[0]!, `file report is missing ${key}`);
  }

  const findings = files[0]!["findings"] as Record<string, unknown>[];
  assert.ok(findings.length > 0, "the golden has no findings");

  for (const key of required.finding) {
    assert.ok(key in findings[0]!, `finding is missing ${key}`);
  }
});
