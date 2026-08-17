import { execFile } from "node:child_process";

export interface Finding {
  line: number;
  column: number;
  codepoint: string;
  name: string;
  kind: string;
  action: string;
  applied: boolean;
  actionable: boolean;
  // Absent on an actionable finding means delete the character, which is the
  // normal case for an invisible one.
  replacement?: string;
  in_payload?: boolean;
}

export interface MixedWord {
  line: number;
  column: number;
  word: string;
  scripts: string[];
}

export interface Payload {
  line: number;
  column: number;
  scheme: string;
  runes: number;
  text?: string;
  printable: boolean;
}

export interface FileReport {
  path: string;
  format: string;
  encoding: string;
  findings: Finding[] | null;
  payloads?: Payload[];
  mixed_script?: MixedWord[];
  error?: string;
}

export interface Report {
  version: string;
  files: FileReport[];
}

export class UntraceError extends Error {
  constructor(
    message: string,
    readonly missingBinary: boolean = false,
  ) {
    super(message);
  }
}

export interface CheckOptions {
  binary: string;
  /** Path the content should be resolved as, deciding format and config. */
  name: string;
  strict: boolean;
}

/**
 * Exit 2 means untrace could not read the input or the config. Exit 1 only
 * happens under --fail, which is never passed here, so it is not an error.
 */
export function check(text: string, opt: CheckOptions): Promise<FileReport> {
  const args = ["--stdin", "--json", "--stdin-name", opt.name];
  if (opt.strict) {
    args.push("--strict");
  }

  return new Promise((resolve, reject) => {
    const child = execFile(
      opt.binary,
      args,
      { maxBuffer: 32 * 1024 * 1024 },
      (err, _stdout, stderr) => {
        if (err && (err as NodeJS.ErrnoException).code === "ENOENT") {
          reject(
            new UntraceError(
              `untrace not found at ${opt.binary}. Install it, or set untrace.path.`,
              true,
            ),
          );
          return;
        }
        if (err && typeof err.code === "number" && err.code >= 2) {
          reject(new UntraceError(stderr.trim() || `untrace exited ${err.code}`));
          return;
        }

        let report: Report;
        try {
          report = JSON.parse(stderr) as Report;
        } catch {
          reject(new UntraceError(`could not parse untrace output: ${stderr.slice(0, 200)}`));
          return;
        }

        const file = report.files[0];
        if (!file) {
          reject(new UntraceError("untrace reported no files"));
          return;
        }
        if (file.error) {
          reject(new UntraceError(file.error));
          return;
        }
        resolve(file);
      },
    );

    child.stdin?.on("error", () => {
      // The child can exit before the document is written, which surfaces as
      // the callback error rather than here.
    });
    child.stdin?.end(text);
  });
}
