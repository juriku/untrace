import * as assert from "node:assert/strict";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import * as vscode from "vscode";

interface Step {
	id: string;
	title: string;
	description: string;
	media?: { markdown?: string };
}

interface Manifest {
	contributes: {
		commands: { command: string; title: string; icon?: { light: string; dark: string } }[];
		menus: Record<string, { command: string; when?: string }[]>;
		keybindings: { command: string }[];
		walkthroughs: { id: string; steps: Step[] }[];
	};
}

const root = path.join(__dirname, "..", "..");

async function manifest(): Promise<Manifest> {
	return JSON.parse(await fs.readFile(path.join(root, "package.json"), "utf8")) as Manifest;
}

function buttonCommands(step: Step): string[] {
	return [...step.description.matchAll(/\(command:([^)?\s]+)/g)].map((m) => m[1] ?? "");
}

suite("the manifest", () => {
	test("every walkthrough button names a command that is registered", async () => {
		const registered = new Set(await vscode.commands.getCommands(true));
		const steps = (await manifest()).contributes.walkthroughs.flatMap((w) => w.steps);
		const buttons = steps.flatMap(buttonCommands);

		assert.ok(buttons.length > 0, "a walkthrough with no buttons onboards nobody");
		for (const command of buttons) {
			assert.ok(registered.has(command), `${command} is not registered`);
		}
	});

	test("every declared command is registered", async () => {
		const registered = new Set(await vscode.commands.getCommands(true));
		for (const { command } of (await manifest()).contributes.commands) {
			assert.ok(registered.has(command), `${command} is declared but not registered`);
		}
	});

	test("every menu and keybinding entry names a declared command", async () => {
		const contributes = (await manifest()).contributes;
		const declared = new Set(contributes.commands.map((c) => c.command));

		const referenced = [
			...Object.values(contributes.menus).flat().map((m) => m.command),
			...contributes.keybindings.map((k) => k.command),
		];
		for (const command of referenced) {
			assert.ok(declared.has(command), `${command} is used but never declared`);
		}
	});

	test("the toolbar button is gated, so it never sits on a clean file", async () => {
		const contributes = (await manifest()).contributes;
		const toolbar = contributes.menus["editor/title"] ?? [];

		assert.equal(toolbar.length, 1);
		assert.equal(toolbar[0]?.command, "untrace.fixFile");
		assert.equal(toolbar[0]?.when, "untrace.hasFindings");

		const icon = contributes.commands.find((c) => c.command === "untrace.fixFile")?.icon;
		assert.ok(icon, "a title-bar entry with no icon is hidden in the overflow menu");
		for (const theme of ["light", "dark"] as const) {
			await fs.access(path.join(root, icon[theme]));
		}
	});

	test("every walkthrough step points at media that exists", async () => {
		const steps = (await manifest()).contributes.walkthroughs.flatMap((w) => w.steps);
		assert.ok(steps.length > 0);

		for (const step of steps) {
			const media = step.media?.markdown;
			assert.ok(media, `step ${step.id} has no media`);
			await fs.access(path.join(root, media));
		}
	});
});
