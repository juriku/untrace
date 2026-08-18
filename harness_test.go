package untrace_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Built from codepoints: a pasted invisible character is unreadable in source
// and flags itself when untrace scans its own tests.
var (
	zwsp   = string(rune(0x200B))
	emDash = string(rune(0x2014))
	nbsp   = string(rune(0x00A0))
	cyrA   = string(rune(0x0430))
)

// Dir is a real directory on disk that a test builds up and then runs the
// compiled binary against.
type Dir struct {
	t    *testing.T
	path string
}

func NewDir(t *testing.T) *Dir {
	t.Helper()
	return &Dir{t: t, path: t.TempDir()}
}

func (d *Dir) Path(rel ...string) string {
	return filepath.Join(append([]string{d.path}, rel...)...)
}

func (d *Dir) File(rel, body string) *Dir {
	d.t.Helper()
	return d.Bytes(rel, []byte(body))
}

func (d *Dir) Bytes(rel string, body []byte) *Dir {
	d.t.Helper()
	full := d.Path(rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		d.t.Fatal(err)
	}
	return d
}

func (d *Dir) Mkdir(rel string) *Dir {
	d.t.Helper()
	if err := os.MkdirAll(d.Path(rel), 0o755); err != nil {
		d.t.Fatal(err)
	}
	return d
}

// Symlink points rel at target, which may be relative, absent or a directory.
func (d *Dir) Symlink(rel, target string) *Dir {
	d.t.Helper()
	full := d.Path(rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		d.t.Fatal(err)
	}
	if err := os.Symlink(target, full); err != nil {
		d.t.Fatal(err)
	}
	return d
}

// Chmod restores the mode afterwards, or TempDir cannot remove the tree.
func (d *Dir) Chmod(rel string, mode os.FileMode) *Dir {
	d.t.Helper()
	full := d.Path(rel)
	info, err := os.Stat(full)
	if err != nil {
		d.t.Fatal(err)
	}
	if err := os.Chmod(full, mode); err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(func() { os.Chmod(full, info.Mode()) })
	return d
}

func (d *Dir) Read(rel string) string {
	d.t.Helper()
	body, err := os.ReadFile(d.Path(rel))
	if err != nil {
		d.t.Fatal(err)
	}
	return string(body)
}

func (d *Dir) Exists(rel string) bool {
	_, err := os.Lstat(d.Path(rel))
	return err == nil
}

// Run executes the compiled binary with d as its working directory.
func (d *Dir) Run(args ...string) *Result {
	d.t.Helper()
	return d.run(nil, args...)
}

func (d *Dir) RunStdin(stdin string, args ...string) *Result {
	d.t.Helper()
	return d.run(strings.NewReader(stdin), args...)
}

func (d *Dir) run(stdin *strings.Reader, args ...string) *Result {
	d.t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Dir = d.path
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	if stdin != nil {
		cmd.Stdin = stdin
	}

	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb

	exit := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errorsAs(err, &ee) {
			d.t.Fatalf("running untrace %v: %v", args, err)
		}
		exit = ee.ExitCode()
	}
	return &Result{t: d.t, argv: args, stdout: out.String(), stderr: errb.String(), exit: exit}
}

type Result struct {
	t      *testing.T
	argv   []string
	stdout string
	stderr string
	exit   int
}

func (r *Result) fail(format string, args ...any) {
	r.t.Helper()
	detail := append(args, r.stdout, r.stderr)
	r.t.Errorf("untrace %s\n  "+format+"\n  stdout: %q\n  stderr: %q",
		append([]any{strings.Join(r.argv, " ")}, detail...)...)
}

// Exit is required on every run: without it a command that did nothing passes
// for one that worked.
func (r *Result) Exit(want int) *Result {
	r.t.Helper()
	if r.exit != want {
		r.fail("exit = %d, want %d", r.exit, want)
	}
	return r
}

func (r *Result) StdoutHas(want ...string) *Result {
	r.t.Helper()
	for _, w := range want {
		if !strings.Contains(r.stdout, w) {
			r.fail("stdout does not contain %q", w)
		}
	}
	return r
}

func (r *Result) StdoutLacks(unwanted ...string) *Result {
	r.t.Helper()
	for _, u := range unwanted {
		if strings.Contains(r.stdout, u) {
			r.fail("stdout contains %q", u)
		}
	}
	return r
}

func (r *Result) StderrHas(want ...string) *Result {
	r.t.Helper()
	for _, w := range want {
		if !strings.Contains(r.stderr, w) {
			r.fail("stderr does not contain %q", w)
		}
	}
	return r
}

func (r *Result) Stdout() string { return r.stdout }
