package untrace_test

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// Every flag the binary advertises, read from the binary rather than a list a
// test author kept in their head. A flag added without a test appears here.
func advertisedFlags(t *testing.T) []string {
	t.Helper()

	out, _ := exec.Command(binary, "--help").CombinedOutput()
	found := regexp.MustCompile(`(?m)^\s+-([a-z][a-z-]*)`).FindAllStringSubmatch(string(out), -1)
	if len(found) == 0 {
		t.Fatalf("no flags parsed from --help:\n%s", out)
	}

	var flags []string
	for _, m := range found {
		flags = append(flags, m[1])
	}
	return flags
}

// value supplies an argument for the flags that take one, so the matrix can
// exercise them instead of skipping them.
var value = map[string]string{
	"config":       ".untrace.json",
	"baseline":     ".untrace-baseline.json",
	"exclude-char": "U+2014",
	"ignore-dir":   "vendor",
	"pattern":      "*.md",
	"stdin-name":   "x.md",
}

// Flags that end the run before any path is read, so position cannot matter.
var terminal = map[string]bool{"version": true, "stdin": true, "stdin-name": true}

// "untrace . --fix" must do what "untrace --fix ." does. Go's flag package
// stops at the first non-flag argument, so a flag written after a path is
// silently taken as another path to scan.
func TestEveryFlagWorksInEveryPosition(t *testing.T) {
	for _, flag := range advertisedFlags(t) {
		if terminal[flag] {
			continue
		}

		t.Run(flag, func(t *testing.T) {
			argv := []string{"--" + flag}
			if v, ok := value[flag]; ok {
				argv = append(argv, v)
			}

			before := newMatrixDir(t)
			after := newMatrixDir(t)

			first := before.Run(append(argv, ".")...)
			second := after.Run(append([]string{"."}, argv...)...)

			if first.exit != second.exit {
				t.Errorf("exit differs by flag position: %v gave %d, %v gave %d\n  stderr: %q",
					append(argv, "."), first.exit,
					append([]string{"."}, argv...), second.exit, second.stderr)
			}
			if before.Read("a.md") != after.Read("a.md") {
				t.Errorf("the file differs by flag position:\n  flag first: %q\n  flag last:  %q",
					before.Read("a.md"), after.Read("a.md"))
			}
		})
	}
}

func newMatrixDir(t *testing.T) *Dir {
	t.Helper()
	return NewDir(t).
		File("a.md", "a dash "+emDash+" and a space"+nbsp+"here\n").
		File(".untrace.json", `{}`).
		File(".untrace-baseline.json", `{"version":1,"entries":[]}`)
}

// A flag written after a path was accepted as a path, so an unknown flag in
// that position produced no error at all.
func TestUnknownFlagIsRejectedInEveryPosition(t *testing.T) {
	for _, argv := range [][]string{
		{"--nonsense", "."},
		{".", "--nonsense"},
		{"--fix", ".", "--nonsense"},
	} {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			d := newMatrixDir(t)

			r := d.Run(argv...)

			if r.exit != 2 {
				t.Errorf("exit = %d, want 2", r.exit)
			}
			if !strings.Contains(r.stderr, "nonsense") {
				t.Errorf("stderr = %q, want it to name the unknown flag", r.stderr)
			}
		})
	}
}
