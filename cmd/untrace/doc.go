// Untrace finds and removes the invisible characters, lookalike letters
// (homoglyphs), hidden payloads and provenance metadata used to mark text,
// documents and images as AI-generated.
//
// It resolves what a file is before deciding what its characters mean, so a
// curly quote in a Word document is ordinary typography while the same
// character in source is normalised. Emoji sequences, Arabic and Indic joiners,
// CJK punctuation and terminated bidi isolates are left alone; no context
// excuses the direction override U+202E, the Trojan Source vector.
//
// Usage:
//
//	untrace [flags] [path ...]     scan paths, defaulting to the current directory
//	untrace --fix .                rewrite what is found
//	untrace --json .               machine-readable output
//	untrace --fail .               exit 1 if anything actionable is found
//	cat file.txt | untrace --stdin --fix > clean.txt
//
// Statistical token watermarks such as SynthID-Text add no characters and no
// metadata, so there is nothing in the file for untrace to find: it cannot
// detect or remove them, and neither can any other character or metadata tool.
//
// Full documentation is at https://github.com/juriku/untrace.
package main
