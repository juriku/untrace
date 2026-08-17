package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/juriku/untrace/internal/gitignore"
)

// Git reads ignore rules from three sources: .gitignore files in a directory
// and all its parents, $GIT_DIR/info/exclude, and the file named by
// core.excludesFile (see gitignore(5)).
//
// Precedence, highest first: deepest .gitignore, shallower .gitignore,
// info/exclude, global excludes. The matcher wants the reverse, so patterns are
// assembled lowest priority first.

// .git is a directory in a normal clone and a file in a worktree or submodule.
func findRepoRoot(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			return abs, true
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}

func basePatterns(repoRoot, scanRoot string) []gitignore.Pattern {
	var out []gitignore.Pattern

	out = append(out, parseLines(readFile(globalExcludesPath()), nil)...)
	out = append(out, parseLines(readFile(filepath.Join(repoRoot, ".git", "info", "exclude")), nil)...)

	for _, dir := range ancestors(repoRoot, scanRoot) {
		domain := relComponents(repoRoot, dir)
		out = append(out, parseLines(readFile(filepath.Join(dir, ".gitignore")), domain)...)
	}
	return out
}

// scanRoot itself is excluded: the walk reads that .gitignore when it visits
// the directory, and adding it here would apply it twice.
func ancestors(repoRoot, scanRoot string) []string {
	absScan, err := filepath.Abs(scanRoot)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(repoRoot, absScan)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil
	}

	dirs := []string{repoRoot}
	if rel == "." {
		return nil
	}
	current := repoRoot
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		dirs = append(dirs, current)
	}
	return dirs
}

// With core.excludesFile unset, git falls back to $XDG_CONFIG_HOME/git/ignore.
func globalExcludesPath() string {
	out, err := exec.Command("git", "config", "--get", "core.excludesFile").Output()
	if path := strings.TrimSpace(string(out)); err == nil && path != "" {
		return expandHome(path)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "git", "ignore")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "git", "ignore")
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func readFile(path string) []byte {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

func parseLines(data []byte, domain []string) []gitignore.Pattern {
	if len(data) == 0 {
		return nil
	}
	var out []gitignore.Pattern
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, gitignore.ParsePattern(line, domain))
	}
	return out
}
