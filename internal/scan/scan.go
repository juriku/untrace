package scan

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/juriku/untrace/internal/docmeta"
	"github.com/juriku/untrace/internal/gitignore"
	"github.com/juriku/untrace/internal/media"
	"github.com/juriku/untrace/internal/textfile"
)

type Options struct {
	Recursive    bool
	IgnoredDirs  map[string]bool
	UseGitignore bool
	Patterns     []string
}

const sniffLen = 4096

// Files walks root and returns the text files worth scanning.
//
// Each pattern carries the domain of the directory it came from, so it stops
// applying outside its own subtree without any stack bookkeeping. Patterns are
// appended as the walk descends, which is the increasing-priority order the
// matcher expects.
//
// Paths are matched relative to the repository root rather than the scan root,
// because a pattern inherited from a parent directory cannot be expressed
// against a path relative to a directory below it.
func Files(root string, opts Options) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if IsScannable(root) {
			return []string{root}, nil
		}
		return nil, nil
	}

	var (
		files    []string
		patterns []gitignore.Pattern
	)

	matchBase := root
	if opts.UseGitignore {
		if repoRoot, ok := findRepoRoot(root); ok {
			matchBase = repoRoot
			patterns = basePatterns(repoRoot, root)
		}
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel := relComponents(matchBase, path)

		if d.IsDir() {
			if path != root {
				if opts.IgnoredDirs[d.Name()] {
					return fs.SkipDir
				}
				if !opts.Recursive {
					return fs.SkipDir
				}
				if opts.UseGitignore && len(patterns) > 0 &&
					gitignore.NewMatcher(patterns).Match(rel, true) {
					return fs.SkipDir
				}
			}
			if opts.UseGitignore {
				patterns = append(patterns, readGitignore(path, rel)...)
			}
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}
		if !matchesPatterns(d.Name(), opts.Patterns) {
			return nil
		}
		if opts.UseGitignore && len(patterns) > 0 &&
			gitignore.NewMatcher(patterns).Match(rel, false) {
			return nil
		}
		if IsScannable(path) {
			files = append(files, path)
		}
		return nil
	})

	return files, walkErr
}

// Both sides are absolutised first: the walk yields paths relative to the scan
// root while the match base may be the repository root above it, and
// filepath.Rel fails outright when one side is absolute and the other is not.
func relComponents(root, path string) []string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == "." {
		return nil
	}
	return strings.Split(filepath.ToSlash(rel), "/")
}

func readGitignore(dir string, domain []string) []gitignore.Pattern {
	return parseLines(readFile(filepath.Join(dir, ".gitignore")), domain)
}

func matchesPatterns(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ok, err := filepath.Match(p, name); err == nil && ok {
			return true
		}
	}
	return false
}

// IsScannable accepts text files and the image containers whose metadata is
// worth inspecting; everything else is skipped as binary.
func IsScannable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, sniffLen)
	n, err := f.Read(buf)
	if n == 0 {
		return err == nil || errors.Is(err, io.EOF)
	}
	if media.Detect(buf[:n]) != media.FormatUnknown {
		return true
	}
	if docmeta.MaybeContainer(buf[:n]) {
		return true
	}
	return !textfile.IsBinary(buf[:n])
}
