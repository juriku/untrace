package untrace_test

// Golden-file tests over the real binary. Each case under testdata/cases holds
// a tree in in/ and one or more runs over it:
//
//	in/                    files copied into a scratch directory before each run
//	runs/<name>/args       flags for that run, default "."
//	runs/<name>/want.txt   expected stdout
//	runs/<name>/want-stderr.txt   expected stderr, only when non-empty
//	runs/<name>/want-exit  expected exit code, only when non-zero
//	runs/<name>/want-files/   expected contents afterwards, compared byte for byte
//	runs/<name>/stdin      optional stdin content
//
// A case with no runs/ directory holds those same files at its top level and is
// treated as a single run, which is how the focused single-purpose cases are
// written.
//
// Every run gets its own copy of in/, so a --fix run cannot affect the next.
//
// Run with -update to regenerate the golden files.

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "regenerate golden files")

var binary string

func TestMain(m *testing.M) {
	flag.Parse()

	dir, err := os.MkdirTemp("", "untrace-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	// Windows will not execute a file without the extension, and "go build -o"
	// with an explicit filename does not add one.
	binary = filepath.Join(dir, "untrace")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	// These cases run the built binary as a subprocess, which ordinary coverage
	// instrumentation cannot see. Building with -cover makes it write counters
	// to GOCOVERDIR, which the subprocess inherits from this process.
	args := []string{"build", "-o", binary, "./cmd/untrace"}
	if os.Getenv("GOCOVERDIR") != "" {
		args = []string{"build", "-cover", "-o", binary, "./cmd/untrace"}
	}

	build := exec.Command("go", args...)
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building untrace: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestGolden(t *testing.T) {
	cases, err := filepath.Glob("testdata/cases/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases found under testdata/cases")
	}

	for _, dir := range cases {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		t.Run(filepath.Base(dir), func(t *testing.T) {
			tree := filepath.Join(dir, "in")

			runs, err := filepath.Glob(filepath.Join(dir, "runs", "*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(runs) == 0 {
				runCase(t, tree, dir)
				return
			}
			for _, run := range runs {
				t.Run(filepath.Base(run), func(t *testing.T) { runCase(t, tree, run) })
			}
		})
	}
}

func runCase(t *testing.T, tree, dir string) {
	t.Helper()

	work := t.TempDir()
	if exists(tree) {
		copyTree(t, tree, work)
	}

	args := []string{"."}
	if raw := readOptional(t, filepath.Join(dir, "args")); raw != "" {
		args = strings.Fields(raw)
	}

	cmd := exec.Command(binary, args...)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdinPath := filepath.Join(dir, "stdin"); exists(stdinPath) {
		f, err := os.Open(stdinPath)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		cmd.Stdin = f
	}

	exitCode := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errorsAs(err, &ee) {
			t.Fatalf("running untrace: %v", err)
		}
		exitCode = ee.ExitCode()
	}

	compare(t, filepath.Join(dir, "want.txt"), scrub(work, stdout.String()))
	compareOptional(t, filepath.Join(dir, "want-stderr.txt"), scrub(work, stderr.String()))
	compareExit(t, filepath.Join(dir, "want-exit"), exitCode)
	compareFiles(t, filepath.Join(dir, "want-files"), work)
}

// The scratch directory is different every run, so any absolute path in the
// output has to be replaced before comparing against a golden file.
func scrub(work, s string) string {
	// The resolved path goes first: on macOS it is the longer /private prefix of
	// the same directory, and replacing the short form first would leave a stub.
	if resolved, err := filepath.EvalSymlinks(work); err == nil {
		s = strings.ReplaceAll(s, resolved, "<scratch>")
	}
	s = strings.ReplaceAll(s, work, "<scratch>")

	// Reported paths come from filepath.Join, so one golden file cannot match
	// both separators. Only a separator is rewritten: a blanket replace turns
	// the escaped quote in JSON output into /".
	if runtime.GOOS == "windows" {
		s = pathSeparator.ReplaceAllString(s, "/$1")
	}
	return s
}

var pathSeparator = regexp.MustCompile(`\\([A-Za-z0-9._-])`)

func compare(t *testing.T, golden, got string) {
	t.Helper()
	if *update {
		writeGolden(t, golden, got)
		return
	}
	want := readOptional(t, golden)
	if got != want {
		t.Errorf("stdout mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func compareOptional(t *testing.T, golden, got string) {
	t.Helper()
	if *update {
		if strings.TrimSpace(got) == "" {
			os.Remove(golden)
		} else {
			writeGolden(t, golden, got)
		}
		return
	}
	want := readOptional(t, golden)
	if got != want {
		t.Errorf("stderr mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func compareExit(t *testing.T, golden string, got int) {
	t.Helper()
	if *update {
		if got == 0 {
			os.Remove(golden)
		} else {
			writeGolden(t, golden, fmt.Sprintf("%d\n", got))
		}
		return
	}
	want := 0
	if raw := strings.TrimSpace(readOptional(t, golden)); raw != "" {
		fmt.Sscanf(raw, "%d", &want)
	}
	if got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
}

// compareFiles checks the scratch directory against want-files byte for byte,
// which is what catches encoding damage that a text diff would hide.
func compareFiles(t *testing.T, golden, work string) {
	t.Helper()

	if *update {
		if !exists(golden) {
			return
		}
		os.RemoveAll(golden)
		storeTree(t, work, golden)
		return
	}
	if !exists(golden) {
		return
	}

	err := filepath.Walk(golden, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(golden, path)
		if err != nil {
			return err
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(work, rel))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			return nil
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: bytes differ\n got %q\nwant %q", rel, got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeGolden(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readOptional(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Git refuses to track any path containing .git, so a fixture that needs a
// repository directory stores it as dot-git and it is renamed on the way in.
const storedGitDir = "dot-git"

func swapComponent(rel, from, to string) string {
	parts := strings.Split(rel, string(filepath.Separator))
	for i, p := range parts {
		if p == from {
			parts[i] = to
		}
	}
	return filepath.Join(parts...)
}

func copyTree(t *testing.T, src, dst string) {
	copyTreeMapped(t, src, dst, func(rel string) string {
		return swapComponent(rel, storedGitDir, ".git")
	})
}

// storeTree is the inverse, for -update writing a scratch directory back into
// testdata where a real .git could never be committed.
func storeTree(t *testing.T, src, dst string) {
	copyTreeMapped(t, src, dst, func(rel string) string {
		return swapComponent(rel, ".git", storedGitDir)
	})
}

func copyTreeMapped(t *testing.T, src, dst string, mapRel func(string) string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = mapRel(rel)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

func errorsAs(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}
