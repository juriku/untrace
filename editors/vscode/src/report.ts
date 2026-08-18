export interface Finding {
	line: number;
	column: number;
	codepoint: string;
	name: string;
	kind: string;
	action: string;
	// Absent means a binary predating the field. Empty means delete the
	// character, and is only meaningful when actionable is true.
	replacement?: string;
	applied: boolean;
	in_payload?: boolean;
	actionable: boolean;
}

export interface Payload {
	scheme: string;
	start: number;
	end: number;
	runes: number;
	text: string;
	printable: boolean;
	line: number;
	column: number;
}

export interface MixedWord {
	line: number;
	column: number;
	word: string;
	scripts: string[];
}

export interface FileReport {
	path: string;
	format: string;
	encoding: string;
	findings?: Finding[] | null;
	payloads?: Payload[] | null;
	mixed_script?: MixedWord[] | null;
	suppressed?: number;
	error?: string;
}

export interface Report {
	version: string;
	files: FileReport[] | null;
}
