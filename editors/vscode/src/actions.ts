import * as vscode from "vscode";

import { source, toRange, workspaceEdit } from "./adapt";
import { edits, type Fix, type Problem } from "./problems";

export const fixAllKind = vscode.CodeActionKind.SourceFixAll.append(source);

export const fixAllTitle = label("Remove every hidden character in this file");

export interface Problems {
	current(document: vscode.TextDocument): Problem[] | undefined;
	refresh(document: vscode.TextDocument): Promise<Problem[] | undefined>;
	withRewrites(document: vscode.TextDocument): Promise<Problem[] | undefined>;
}

function label(title: string): string {
	return `${source}: ${title}`;
}

interface Candidate {
	problem: Problem;
	fix: Fix;
	preferred: boolean;
}

export class Fixes implements vscode.CodeActionProvider {
	static readonly metadata: vscode.CodeActionProviderMetadata = {
		providedCodeActionKinds: [vscode.CodeActionKind.QuickFix, fixAllKind],
	};

	constructor(private readonly problems: Problems) {}

	async provideCodeActions(
		document: vscode.TextDocument,
		range: vscode.Range,
		context: vscode.CodeActionContext,
		token: vscode.CancellationToken,
	): Promise<vscode.CodeAction[]> {
		let found = await this.findings(document, token);
		if (token.isCancellationRequested) {
			return [];
		}
		if (needsRewrites(found, range)) {
			found = (await this.problems.withRewrites(document)) ?? found;
			if (token.isCancellationRequested) {
				return [];
			}
		}
		const all = edits(found, "all");

		if (context.only?.contains(fixAllKind)) {
			return all.length === 0 ? [] : [every(document, all, context, fixAllKind)];
		}
		if (context.only !== undefined && !context.only.contains(vscode.CodeActionKind.QuickFix)) {
			return [];
		}

		const actions = near(found, range).map((c) => one(document, c, context));
		if (all.length > 0) {
			actions.push(every(document, all, context, vscode.CodeActionKind.QuickFix));
		}
		return actions;
	}
}

// microsoft/vscode#254459
function needsRewrites(found: readonly Problem[], range: vscode.Range): boolean {
	return onLineOrTouching(found, range).some((p) => p.rewritable && p.rewrite === undefined);
}

function onLineOrTouching(found: readonly Problem[], range: vscode.Range): Problem[] {
	const touching = found.filter((p) => toRange(p.range).intersection(range) !== undefined);
	if (touching.length > 0) {
		return touching;
	}
	return found.filter(
		(p) => p.range.start.line >= range.start.line && p.range.start.line <= range.end.line,
	);
}

function near(found: readonly Problem[], range: vscode.Range): Candidate[] {
	const out: Candidate[] = [];
	for (const problem of onLineOrTouching(found, range)) {
		if (problem.fix !== undefined) {
			out.push({ problem, fix: problem.fix, preferred: true });
		}
		if (problem.rewrite !== undefined) {
			out.push({ problem, fix: problem.rewrite, preferred: false });
		}
	}
	return out;
}

function one(
	document: vscode.TextDocument,
	candidate: Candidate,
	context: vscode.CodeActionContext,
): vscode.CodeAction {
	const action = new vscode.CodeAction(label(candidate.fix.title), vscode.CodeActionKind.QuickFix);
	action.edit = workspaceEdit(document.uri, [candidate.fix]);
	if (candidate.preferred) {
		action.isPreferred = true;
	}

	const at = toRange(candidate.problem.range);
	const diagnostic = context.diagnostics.find((d) => d.source === source && d.range.isEqual(at));
	if (diagnostic !== undefined) {
		action.diagnostics = [diagnostic];
	}
	return action;
}

function every(
	document: vscode.TextDocument,
	fixes: readonly Fix[],
	context: vscode.CodeActionContext,
	kind: vscode.CodeActionKind,
): vscode.CodeAction {
	const action = new vscode.CodeAction(fixAllTitle, kind);
	action.edit = workspaceEdit(document.uri, fixes);
	action.diagnostics = context.diagnostics.filter((d) => d.source === source);
	return action;
}
