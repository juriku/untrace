package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func mkfile(tb testing.TB, dir, name, content string) {
	tb.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		tb.Fatal(err)
	}
}

func flatTree(tb testing.TB, files int) string {
	tb.Helper()
	root := tb.TempDir()
	for i := 0; i < files; i++ {
		mkfile(tb, root, fmt.Sprintf("file%03d.go", i), "package main\n")
	}
	return root
}

// Same file count as flatTree, distributed down a chain of directories, so the
// difference is the per-directory work rather than the per-file work.
func deepTree(tb testing.TB, depth, perLevel int) string {
	tb.Helper()
	root := tb.TempDir()
	dir := ""
	for d := 0; d < depth; d++ {
		dir = filepath.Join(dir, fmt.Sprintf("level%d", d))
		for i := 0; i < perLevel; i++ {
			mkfile(tb, root, filepath.Join(dir, fmt.Sprintf("file%03d.go", i)), "package main\n")
		}
	}
	return root
}

func benchFiles(b *testing.B, root string, opts Options) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Files(root, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func gitignoreOpts(use bool) Options {
	return Options{Recursive: true, IgnoredDirs: map[string]bool{".git": true}, UseGitignore: use}
}

// The pair isolates gitignore: same tree, the matcher off then on.
func BenchmarkWalkNoGitignore(b *testing.B) {
	root := flatTree(b, 500)
	benchFiles(b, root, gitignoreOpts(false))
}

func BenchmarkWalkWithGitignore(b *testing.B) {
	root := flatTree(b, 500)
	mkfile(b, root, ".gitignore", "*.tmp\nbuild/\n")
	benchFiles(b, root, gitignoreOpts(true))
}

// Matching walks the pattern list per entry, so the gap between these two is
// how the cost scales with the size of a project's ignore rules.
func BenchmarkWalkFewPatterns(b *testing.B) {
	root := flatTree(b, 500)
	mkfile(b, root, ".gitignore", "*.tmp\n")
	benchFiles(b, root, gitignoreOpts(true))
}

func BenchmarkWalkManyPatterns(b *testing.B) {
	root := flatTree(b, 500)
	patterns := ""
	for i := 0; i < 100; i++ {
		patterns += fmt.Sprintf("ignored%03d/\n*.ext%03d\n", i, i)
	}
	mkfile(b, root, ".gitignore", patterns)
	benchFiles(b, root, gitignoreOpts(true))
}

func BenchmarkWalkFlat(b *testing.B) {
	root := flatTree(b, 500)
	benchFiles(b, root, gitignoreOpts(true))
}

func BenchmarkWalkDeep(b *testing.B) {
	root := deepTree(b, 10, 50)
	benchFiles(b, root, gitignoreOpts(true))
}

// Nested .gitignore files accumulate patterns as the walk descends, which is
// the case where pattern count and directory depth compound.
func BenchmarkWalkNestedGitignores(b *testing.B) {
	root := deepTree(b, 10, 50)
	dir := ""
	for d := 0; d < 10; d++ {
		dir = filepath.Join(dir, fmt.Sprintf("level%d", d))
		mkfile(b, root, filepath.Join(dir, ".gitignore"), fmt.Sprintf("*.level%d\n", d))
	}
	benchFiles(b, root, gitignoreOpts(true))
}

func BenchmarkWalkWithNameFilter(b *testing.B) {
	root := flatTree(b, 500)
	opts := gitignoreOpts(true)
	opts.Patterns = []string{"*.go"}
	benchFiles(b, root, opts)
}

// Binary files are rejected by a content sniff rather than by name, so this is
// the cost of opening and reading a prefix of every file.
func BenchmarkWalkBinaryHeavy(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 500; i++ {
		mkfile(b, root, fmt.Sprintf("file%03d.bin", i), "\x00\x01\x02binary content\x00")
	}
	benchFiles(b, root, gitignoreOpts(true))
}
