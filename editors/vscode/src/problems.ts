import type { LineIndex, Range } from "./positions";
import type { Finding, FileReport, MixedWord, Payload } from "./report";

export type Severity = "warning" | "information";

export interface Fix {
	title: string;
	range: Range;
	newText: string;
}

export interface Problem {
	range: Range;
	message: string;
	severity: Severity;
	code: string;
	invisible: boolean;
	at: string;
	rewritable: boolean;
	fix?: Fix;
	rewrite?: Fix;
}

export type Scope = "hidden" | "all";

export function edits(found: readonly Problem[], scope: Scope): Fix[] {
	const out: Fix[] = [];
	for (const problem of found) {
		if (problem.fix !== undefined && (scope === "all" || problem.invisible)) {
			out.push(problem.fix);
		}
	}
	return out;
}

export function problems(report: FileReport, index: LineIndex): Problem[] {
	const out: Problem[] = [];
	for (const payload of report.payloads ?? []) {
		out.push(fromPayload(payload, index));
	}
	for (const finding of report.findings ?? []) {
		if (!finding.in_payload) {
			out.push(fromFinding(finding, index));
		}
	}
	for (const word of report.mixed_script ?? []) {
		out.push(fromMixed(word, index));
	}
	return out;
}

export function mergeRewrites(
	found: readonly Problem[],
	rewrites: FileReport,
	index: LineIndex,
): Problem[] {
	const byPosition = new Map<string, Finding>();
	for (const finding of rewrites.findings ?? []) {
		if (finding.actionable && finding.replacement) {
			byPosition.set(`${finding.line}:${finding.column}`, finding);
		}
	}

	return found.map((problem) => {
		if (!problem.rewritable || problem.rewrite !== undefined) {
			return problem;
		}
		const match = byPosition.get(problem.at);
		if (match?.replacement === undefined) {
			return problem;
		}
		return {
			...problem,
			rewrite: {
				title: `Rewrite the ${match.name} as ${JSON.stringify(match.replacement)}, changing the writer's script`,
				range: index.range(match.line, match.column, 1),
				newText: match.replacement,
			},
		};
	});
}

function fromPayload(payload: Payload, index: LineIndex): Problem {
	const range = index.range(payload.line, payload.column, payload.runes);
	const what = payload.printable
		? `Hidden message: ${JSON.stringify(payload.text)}.`
		: "Hidden data, not printable text.";
	return {
		range,
		message: `${what} Encoded in ${count(payload.runes, "invisible character")} as ${payload.scheme}.`,
		severity: "warning",
		code: payload.scheme,
		invisible: true,
		at: `${payload.line}:${payload.column}`,
		rewritable: false,
		fix: { title: "Remove this hidden message", range, newText: "" },
	};
}

const invisibleKinds = new Set(["hidden", "tag", "ideographic-vs"]);

function fromFinding(finding: Finding, index: LineIndex): Problem {
	const range = index.range(finding.line, finding.column, 1);
	const what = `${finding.name} (${finding.codepoint}).`;
	const invisible = invisibleKinds.has(finding.kind);
	const at = `${finding.line}:${finding.column}`;

	if (!finding.actionable || finding.replacement === undefined) {
		return {
			range,
			message: `${what} untrace reports this and does not change it.`,
			severity: finding.actionable ? "warning" : "information",
			code: finding.codepoint,
			invisible,
			at,
			rewritable: !finding.actionable,
		};
	}

	const gone = finding.replacement === "";
	return {
		range,
		message: gone
			? `${what} untrace removes this character.`
			: `${what} untrace replaces it with ${JSON.stringify(finding.replacement)}.`,
		severity: "warning",
		code: finding.codepoint,
		invisible,
		at,
		rewritable: false,
		fix: {
			title: gone
				? `Remove the ${finding.name}`
				: `Replace the ${finding.name} with ${JSON.stringify(finding.replacement)}`,
			range,
			newText: finding.replacement,
		},
	};
}

function fromMixed(word: MixedWord, index: LineIndex): Problem {
	return {
		range: index.range(word.line, word.column, [...word.word].length),
		message:
			`${JSON.stringify(word.word)} mixes ${and(word.scripts)}. ` +
			"Mixing scripts inside one word is how a lookalike name is disguised.",
		severity: "warning",
		code: "mixed-script",
		invisible: false,
		at: `${word.line}:${word.column}`,
		rewritable: false,
	};
}

function and(parts: readonly string[]): string {
	if (parts.length < 2) {
		return parts[0] ?? "";
	}
	return `${parts.slice(0, -1).join(", ")} and ${parts[parts.length - 1]}`;
}

function count(n: number, noun: string): string {
	return n === 1 ? `1 ${noun}` : `${n} ${noun}s`;
}
