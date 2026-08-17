import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

import * as vscode from "vscode";

const emDash = String.fromCodePoint(0x2014);
const zwsp = String.fromCodePoint(0x200b);
const cyrillicA = String.fromCodePoint(0x0430);

const repoRoot = resolve(__dirname, "..", "..", "..", "..");

function binary(): string {
  const dir = mkdtempSync(join(tmpdir(), "untrace-bin-"));
  const bin = join(dir, process.platform === "win32" ? "untrace.exe" : "untrace");
  execFileSync("go", ["build", "-o", bin, "./cmd/untrace"], { cwd: repoRoot });
  return bin;
}

async function open(content: string): Promise<vscode.TextDocument> {
  const dir = mkdtempSync(join(tmpdir(), "untrace-actions-"));
  const file = join(dir, "sample.ts");
  writeFileSync(file, content, "utf8");

  const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(file));
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    if (vscode.languages.getDiagnostics(doc.uri).length > 0) {
      return doc;
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  assert.fail("no diagnostics arrived");
}

async function actionsFor(doc: vscode.TextDocument): Promise<vscode.CodeAction[]> {
  const full = new vscode.Range(0, 0, doc.lineCount, 0);
  return (
    (await vscode.commands.executeCommand<vscode.CodeAction[]>(
      "vscode.executeCodeActionProvider",
      doc.uri,
      full,
    )) ?? []
  );
}

async function applyTo(doc: vscode.TextDocument, action: vscode.CodeAction): Promise<string> {
  assert.ok(action.edit, `${action.title} carries no edit`);
  assert.ok(await vscode.workspace.applyEdit(action.edit), `${action.title} did not apply`);
  return doc.getText();
}

suite("code actions", () => {
  suiteSetup(async () => {
    await vscode.workspace
      .getConfiguration("untrace")
      .update("path", binary(), vscode.ConfigurationTarget.Global);
  });

  test("replacing one character leaves the rest alone", async () => {
    const doc = await open(`const a = 1 ${emDash} 2;\n`);
    const actions = await actionsFor(doc);

    const one = actions.find((a) => a.title.startsWith("Replace U+2014"));
    assert.ok(one, `no replace action in: ${actions.map((a) => a.title).join(" | ")}`);
    assert.equal(await applyTo(doc, one), "const a = 1 - 2;\n");
  });

  test("an invisible character is removed rather than replaced", async () => {
    const doc = await open(`const s = "draft${zwsp}copy";\n`);
    const actions = await actionsFor(doc);

    const one = actions.find((a) => a.title === "Remove U+200B");
    assert.ok(one, `no remove action in: ${actions.map((a) => a.title).join(" | ")}`);
    assert.equal(await applyTo(doc, one), 'const s = "draftcopy";\n');
  });

  test("fix all rewrites every finding in one edit", async () => {
    const doc = await open(`const a = 1 ${emDash} 2;\nconst b = "x${zwsp}y";\n`);
    const actions = await actionsFor(doc);

    const all = actions.find((a) => a.title.startsWith("Fix all"));
    assert.ok(all, `no fix-all action in: ${actions.map((a) => a.title).join(" | ")}`);
    assert.equal(await applyTo(doc, all), 'const a = 1 - 2;\nconst b = "xy";\n');
  });

  test("a lookalike letter is offered no fix", async () => {
    const doc = await open(`const site = "p${cyrillicA}ypal";\n`);
    const actions = await actionsFor(doc);

    assert.ok(
      vscode.languages.getDiagnostics(doc.uri).length > 0,
      "expected the homoglyph to still be reported",
    );
    for (const a of actions) {
      assert.ok(
        !a.title.includes("U+0430"),
        `offered a fix for a homoglyph: ${a.title}`,
      );
    }
  });

  // The editor asks at the cursor, not over the whole document, and it passes
  // only the diagnostics overlapping that position.
  test("a fix is offered at a cursor position, as the lightbulb asks", async () => {
    const doc = await open(`const a = 1 ${emDash} 2;\n`);

    const at = doc.getText().indexOf(emDash);
    const cursor = new vscode.Range(0, at, 0, at);
    const actions =
      (await vscode.commands.executeCommand<vscode.CodeAction[]>(
        "vscode.executeCodeActionProvider",
        doc.uri,
        cursor,
      )) ?? [];

    const one = actions.find((a) => a.title.startsWith("Replace U+2014"));
    assert.ok(one, `no replace action at the cursor, only: ${actions.map((a) => a.title).join(" | ")}`);
    assert.equal(await applyTo(doc, one), "const a = 1 - 2;\n");
  });

  // A binary predating the replacement field reports actionable findings with
  // no replacement. Treating that as "delete" silently removes characters that
  // should have been normalised.
  test("an older binary is not read as delete-everything", async () => {
    const dir = mkdtempSync(join(tmpdir(), "untrace-old-"));
    const stub = join(dir, "untrace");
    writeFileSync(
      stub,
      `#!/bin/sh\ncat >/dev/null\ncat >&2 <<'JSON'\n${JSON.stringify({
        version: "0.0.1",
        files: [
          {
            path: "sample.ts",
            format: "source",
            encoding: "utf-8",
            findings: [
              {
                line: 1,
                column: 13,
                codepoint: "U+2014",
                name: "Em Dash",
                kind: "typographic",
                action: "detected",
                applied: false,
                actionable: true,
              },
            ],
          },
        ],
      })}\nJSON\n`,
      { mode: 0o755 },
    );

    await vscode.workspace
      .getConfiguration("untrace")
      .update("path", stub, vscode.ConfigurationTarget.Global);

    try {
      const doc = await open(`const a = 1 ${emDash} 2;\n`);
      const actions = await actionsFor(doc);

      const destructive = actions.find((a) => a.title === "Remove U+2014");
      assert.ok(
        !destructive,
        "offered to delete a character that should have been replaced",
      );
    } finally {
      await vscode.workspace
        .getConfiguration("untrace")
        .update("path", binary(), vscode.ConfigurationTarget.Global);
    }
  });

  test("ignore this line appends the directive in the file's comment syntax", async () => {
    const doc = await open(`const a = 1 ${emDash} 2;\n`);
    const actions = await actionsFor(doc);

    const ignore = actions.find((a) => a.title === "Ignore this line");
    assert.ok(ignore, `no ignore action in: ${actions.map((a) => a.title).join(" | ")}`);
    assert.match(await applyTo(doc, ignore), /\/\/ untrace:ignore$/m);
  });
});
