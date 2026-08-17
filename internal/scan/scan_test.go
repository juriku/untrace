package scan

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, root string, opts Options) []string {
	t.Helper()
	files, err := Files(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func defaults() Options {
	return Options{Recursive: true, UseGitignore: true, IgnoredDirs: map[string]bool{".git": true}}
}

func TestRecursiveWalk(t *testing.T) {
	root := t.TempDir()
	write(t, root, "top.txt", "a")
	write(t, root, "sub/deep.txt", "b")

	got := names(t, root, defaults())
	want := []string{"sub/deep.txt", "top.txt"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNonRecursiveStopsAtTopLevel(t *testing.T) {
	root := t.TempDir()
	write(t, root, "top.txt", "a")
	write(t, root, "sub/deep.txt", "b")

	opts := defaults()
	opts.Recursive = false
	got := names(t, root, opts)
	if !equal(got, []string{"top.txt"}) {
		t.Errorf("got %v, want [top.txt]", got)
	}
}

func TestGitDirAlwaysSkipped(t *testing.T) {
	root := t.TempDir()
	write(t, root, "code.txt", "a")
	write(t, root, ".git/config", "b")

	got := names(t, root, defaults())
	if !equal(got, []string{"code.txt"}) {
		t.Errorf("got %v, want [code.txt]", got)
	}
}

func TestIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	write(t, root, "keep.txt", "a")
	write(t, root, "node_modules/dep.js", "b")

	opts := defaults()
	opts.IgnoredDirs["node_modules"] = true
	got := names(t, root, opts)
	if !equal(got, []string{"keep.txt"}) {
		t.Errorf("got %v, want [keep.txt]", got)
	}
}

func TestGitignoreHonoured(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".gitignore", "secret.txt\nsub/\n")
	write(t, root, "secret.txt", "a")
	write(t, root, "keep.txt", "b")
	write(t, root, "sub/hidden.txt", "c")

	got := names(t, root, defaults())
	want := []string{".gitignore", "keep.txt"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGitignoreDisabled(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".gitignore", "secret.txt\n")
	write(t, root, "secret.txt", "a")

	opts := defaults()
	opts.UseGitignore = false
	got := names(t, root, opts)
	want := []string{".gitignore", "secret.txt"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNestedGitignoreScopedToSubtree(t *testing.T) {
	// A nested .gitignore listing "test" must not affect ./test at the root.
	root := t.TempDir()
	write(t, root, "test2/.gitignore", "test\n")
	write(t, root, "test", "root")
	write(t, root, "test2/test", "sub")
	write(t, root, "test2/keep.txt", "keep")

	got := names(t, root, defaults())
	want := []string{"test", "test2/.gitignore", "test2/keep.txt"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGitignoreNegation(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".gitignore", "*.log\n!keep.log\n")
	write(t, root, "drop.log", "a")
	write(t, root, "keep.log", "b")

	got := names(t, root, defaults())
	want := []string{".gitignore", "keep.log"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPatternFilter(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.py", "x")
	write(t, root, "b.txt", "y")

	opts := defaults()
	opts.Patterns = []string{"*.py"}
	got := names(t, root, opts)
	if !equal(got, []string{"a.py"}) {
		t.Errorf("got %v, want [a.py]", got)
	}
}

func TestBinaryFilesSkipped(t *testing.T) {
	root := t.TempDir()
	write(t, root, "text.txt", "hello")
	if err := os.WriteFile(filepath.Join(root, "blob.bin"), []byte("a\x00b"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := names(t, root, defaults())
	if !equal(got, []string{"text.txt"}) {
		t.Errorf("got %v, want [text.txt]", got)
	}
}

func TestSingleFileTarget(t *testing.T) {
	root := t.TempDir()
	write(t, root, "one.txt", "hello")

	files, err := Files(filepath.Join(root, "one.txt"), defaults())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %v, want one file", files)
	}
}
