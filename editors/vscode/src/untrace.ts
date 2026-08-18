import { execFile } from "node:child_process";

import type { FileReport, Report } from "./report";

export interface RunOptions {
	timeoutMs?: number;
	// untrace discovers .untrace.json by walking up from its own working
	// directory, not from --stdin-name, so this decides which config applies.
	cwd?: string;
	signal?: AbortSignal;
}

export type FailureKind = "missing" | "timeout" | "superseded" | "failed";

export class UntraceFailed extends Error {
	constructor(
		readonly kind: FailureKind,
		message: string,
	) {
		super(message);
	}
}

export interface Fixed {
	text: string;
	report: FileReport;
}

const defaultTimeoutMs = 5000;
const maxOutputBytes = 64 * 1024 * 1024;

export async function check(
	exe: string,
	name: string,
	text: string,
	opt: RunOptions = {},
	extra: readonly string[] = [],
): Promise<FileReport> {
	const { stderr } = await run(exe, [...args(name), ...extra], text, opt);
	return single(exe, stderr);
}

export async function fix(
	exe: string,
	name: string,
	text: string,
	extra: readonly string[] = [],
	opt: RunOptions = {},
): Promise<Fixed> {
	const { stdout, stderr } = await run(exe, [...args(name), "--fix", ...extra], text, opt);
	return { text: stdout, report: single(exe, stderr) };
}

export interface Triage {
	publish: FileReport[];
	skipped: number;
	alreadyOpen: number;
}

export function triage(
	reports: readonly FileReport[],
	open: ReadonlySet<string>,
): Triage {
	const out: Triage = { publish: [], skipped: 0, alreadyOpen: 0 };
	for (const report of reports) {
		if (!interesting(report)) {
			continue;
		}
		if (open.has(report.path)) {
			out.alreadyOpen++;
		} else if (report.encoding !== "utf-8") {
			out.skipped++;
		} else {
			out.publish.push(report);
		}
	}
	return out;
}

function interesting(report: FileReport): boolean {
	return (
		(report.findings ?? []).length > 0 ||
		(report.payloads ?? []).length > 0 ||
		(report.mixed_script ?? []).length > 0
	);
}

export async function scan(
	exe: string,
	dir: string,
	opt: RunOptions = {},
	extra: readonly string[] = [],
): Promise<FileReport[]> {
	// Go's flag package stops parsing at the first non-flag argument.
	const { stdout } = await run(exe, ["--json", ...extra, dir], "", { cwd: dir, ...opt });
	return parse(exe, stdout).files ?? [];
}

function args(name: string): string[] {
	return ["--stdin", "--stdin-name", name, "--json"];
}

function run(
	exe: string,
	argv: string[],
	input: string,
	opt: RunOptions,
): Promise<{ stdout: string; stderr: string }> {
	return new Promise((resolve, reject) => {
		const child = execFile(
			exe,
			argv,
			{
				timeout: opt.timeoutMs ?? defaultTimeoutMs,
				maxBuffer: maxOutputBytes,
				encoding: "utf8",
				...(opt.cwd === undefined ? {} : { cwd: opt.cwd }),
				...(opt.signal === undefined ? {} : { signal: opt.signal }),
			},
			(err, stdout, stderr) => {
				if (err) {
					reject(explain(exe, err, stderr));
					return;
				}
				resolve({ stdout, stderr });
			},
		);
		child.stdin?.on("error", () => {});
		child.stdin?.end(input);
	});
}

function explain(exe: string, err: Error, stderr: string): UntraceFailed {
	const detail = err as NodeJS.ErrnoException & { killed?: boolean };
	const first = stderr.trim().split("\n", 1)[0] ?? "";
	const message = `${exe}: ${first || err.message}`;

	if (detail.code === "ENOENT") {
		return new UntraceFailed("missing", `${exe}: not found`);
	}
	if (err.name === "AbortError" || detail.code === "ABORT_ERR") {
		return new UntraceFailed("superseded", message);
	}
	if (detail.killed === true) {
		return new UntraceFailed("timeout", `${exe}: took too long and was stopped`);
	}
	return new UntraceFailed("failed", message);
}

function parse(exe: string, text: string): Report {
	try {
		return JSON.parse(text) as Report;
	} catch {
		throw new UntraceFailed("failed", `${exe}: the report was not JSON`);
	}
}

function single(exe: string, stderr: string): FileReport {
	const file = parse(exe, stderr).files?.[0];
	if (!file) {
		throw new UntraceFailed("failed", `${exe}: the report named no file`);
	}
	if (file.error) {
		throw new UntraceFailed("failed", `${exe}: ${file.error}`);
	}
	return file;
}
