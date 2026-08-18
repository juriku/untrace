import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { defineConfig } from "@vscode/test-cli";

const workspace = mkdtempSync(join(tmpdir(), "untrace-workspace-"));
process.on("exit", () => rmSync(workspace, { recursive: true, force: true }));

export default defineConfig({
	files: "out/test/**/*.test.js",
	workspaceFolder: workspace,
	mocha: {
		timeout: 30000,
	},
});
