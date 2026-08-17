import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

import * as vscode from "vscode";

const tag = (ascii: string) =>
  [...ascii].map((c) => String.fromCodePoint(0xe0000 + c.charCodeAt(0))).join("");

const emDash = String.fromCodePoint(0x2014);

// "tracked" is 7 tag characters: 7 runes, but 14 UTF-16 code units. Everything
// after them on the line is where a rune offset and a VS Code column diverge.
const line = `const id = "${tag("tracked")}" ${emDash} 1;`;

const repoRoot = resolve(__dirname, "..", "..", "..", "..");

function buildBinary(): string {
  const dir = mkdtempSync(join(tmpdir(), "untrace-bin-"));
  const bin = join(dir, process.platform === "win32" ? "untrace.exe" : "untrace");
  execFileSync("go", ["build", "-o", bin, "./cmd/untrace"], { cwd: repoRoot });
  return bin;
}

function writeFixture(): vscode.Uri {
  const dir = mkdtempSync(join(tmpdir(), "untrace-fixture-"));
  const file = join(dir, "probe.ts");
  writeFileSync(file, `${line}\n`, "utf8");
  return vscode.Uri.file(file);
}

// lint() is fire and forget, so opening a document returns before any
// diagnostic exists.
async function waitForDiagnostics(uri: vscode.Uri): Promise<vscode.Diagnostic[]> {
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    const found = vscode.languages.getDiagnostics(uri);
    if (found.length > 0) {
      return found;
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  assert.fail(`no diagnostics for ${uri.fsPath} within 30s`);
}

suite("diagnostics in a real editor", () => {
  let diags: vscode.Diagnostic[];

  suiteSetup(async () => {
    await vscode.workspace
      .getConfiguration("untrace")
      .update("path", buildBinary(), vscode.ConfigurationTarget.Global);

    const uri = writeFixture();
    await vscode.workspace.openTextDocument(uri);
    diags = await waitForDiagnostics(uri);
  });

  test("the payload is one diagnostic, not one per carrier character", () => {
    const payloads = diags.filter((d) => d.message.startsWith("Hidden payload"));
    assert.equal(payloads.length, 1);
    assert.match(payloads[0]!.message, /tracked/);
  });

  test("a marker after an astral run lands on the right column", () => {
    const dash = diags.find((d) => d.message.includes("Em Dash"));
    assert.ok(dash, `no em dash diagnostic in ${diags.map((d) => d.message).join(", ")}`);

    // untrace reports rune column 22. The seven tag characters ahead of it are
    // fourteen UTF-16 units, so the editor column is 28. Reading the rune
    // column straight through would put it at 21, on top of the payload.
    assert.equal(dash.range.start.character, 28);
    assert.equal(dash.range.start.line, 0);
  });

  test("the range covers exactly the em dash", () => {
    const dash = diags.find((d) => d.message.includes("Em Dash"))!;
    assert.equal(dash.range.end.character - dash.range.start.character, 1);
  });

  test("every diagnostic is attributed to untrace", () => {
    for (const d of diags) {
      assert.equal(d.source, "untrace");
    }
  });
});
