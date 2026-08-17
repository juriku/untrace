import { defineConfig } from "@vscode/test-cli";

export default defineConfig({
  files: "out/test/**/*.test.js",
  mocha: {
    ui: "tdd",
    // A VS Code download plus a process spawn per document exceeds Mocha's 2s default.
    timeout: 60000,
  },
});
