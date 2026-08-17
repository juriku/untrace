# Security policy

## Reporting a vulnerability

Report privately through GitHub's [security advisory
form](https://github.com/juriku/untrace/security/advisories/new) rather than a
public issue.

Expect an acknowledgement within 7 days and an assessment within 14. If a fix is
warranted it ships in the next patch release, and the advisory is published once
that release is out.

## What is in scope

untrace parses untrusted input by design. These are the parts worth attacking:

- **Container parsing.** `.docx`, `.xlsx`, `.pptx` and `.odt` are zip archives;
  PDF is scanned for its trailer. Entries are capped so a zip bomb cannot
  exhaust memory, and the declared uncompressed size is never trusted beyond
  that cap.
- **Image parsing.** PNG chunk and JPEG segment structure, including
  attacker-controlled length fields.
- **Encoding detection and rewriting.** A file that cannot be decoded cleanly is
  skipped rather than rewritten, so `--fix` should never be able to corrupt one.
- **Path handling during a scan.** Symlinks are not followed, so a scan cannot
  be induced to read or rewrite a file outside the tree it was pointed at.

A crash, a hang, memory exhaustion, or any write outside the scanned tree is a
vulnerability. So is `--fix` changing bytes it was not supposed to change.

Eight fuzz targets cover these paths and run on every pull request. If you find
a crasher, the input itself is the most useful thing you can send.

## What is not in scope

untrace cannot detect statistical token watermarks such as SynthID-Text, and
cannot detect pixel or audio watermarks. This is a documented limit of what a
character and metadata tool can reach, not a defect. See the README.

Reports that untrace failed to detect a marker it does not claim to detect are
welcome as ordinary issues rather than security reports.
