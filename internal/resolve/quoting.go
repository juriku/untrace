package resolve

import (
	"path/filepath"
	"strings"
)

// Syntaxes where a straight double quote delimits a string rather than being
// content, so writing one raw can end the string that held the character.
var (
	jsonExts   = map[string]bool{".json": true, ".jsonc": true, ".ipynb": true}
	quotedExts = map[string]bool{
		".yaml": true, ".yml": true, ".toml": true,
		".ini": true, ".cfg": true, ".conf": true,
	}
)

// Rewriter adapts a replacement to a document's syntax. The bool is false when
// the replacement cannot be written safely, which makes the finding
// report-only.
type Rewriter func(string) (string, bool)

// RewriterFor returns the Rewriter a document's syntax requires, or nil when
// replacements can be written literally.
func RewriterFor(path string) Rewriter {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case jsonExts[ext]:
		return jsonRewriter
	case quotedExts[ext]:
		return quotedRewriter
	}
	return nil
}

// A curly quote can only appear inside a string literal in valid JSON, so the
// straight quote replacing it needs the escape that literal requires.
func jsonRewriter(replacement string) (string, bool) {
	if !strings.Contains(replacement, `"`) {
		return replacement, true
	}
	return strings.ReplaceAll(replacement, `"`, `\"`), true
}

// yaml and toml both have a quoting style that takes no backslash escape, so
// whether a straight quote must be escaped depends on which style encloses it.
// Neither can be known without parsing the document.
func quotedRewriter(replacement string) (string, bool) {
	if strings.Contains(replacement, `"`) {
		return "", false
	}
	return replacement, true
}
