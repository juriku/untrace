export interface Position {
	line: number;
	character: number;
}

export interface Range {
	start: Position;
	end: Position;
}

// untrace counts columns in runes and splits lines on \n alone; VS Code counts
// UTF-16 code units and lines from zero. Astral characters shift the two apart.
export class LineIndex {
	private readonly lines: string[];
	private readonly widths = new Map<number, number[]>();

	constructor(text: string) {
		this.lines = text.split("\n");
	}

	range(line: number, column: number, runes: number): Range {
		const at = Math.max(line, 1);
		return {
			start: { line: at - 1, character: this.character(at, column) },
			end: { line: at - 1, character: this.character(at, column + Math.max(runes, 1)) },
		};
	}

	character(line: number, column: number): number {
		const widths = this.widthsOf(line);
		const i = Math.min(Math.max(column - 1, 0), widths.length - 1);
		return widths[i]!;
	}

	private widthsOf(line: number): number[] {
		const cached = this.widths.get(line);
		if (cached) {
			return cached;
		}
		const widths = [0];
		let units = 0;
		for (const rune of this.lines[line - 1] ?? "") {
			units += rune.length;
			widths.push(units);
		}
		this.widths.set(line, widths);
		return widths;
	}
}
