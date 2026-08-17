package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "."},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func gitConfig(t *testing.T, dir, key string) string {
	t.Helper()
	cmd := exec.Command("git", "config", "--local", "--get", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// The subcommand shells out to git in the working directory, so the test runs
// it from inside a scratch repository.
func inDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func TestInstallFilterWritesGitConfig(t *testing.T) {
	dir := gitRepo(t)

	inDir(t, dir, func() {
		if code := runInstallFilter(nil); code != 0 {
			t.Fatalf("install-filter exited %d", code)
		}
	})

	clean := gitConfig(t, dir, "filter.untrace.clean")
	if !strings.Contains(clean, "--stdin") || !strings.Contains(clean, "--fix") {
		t.Errorf("clean filter = %q, want it to pipe through --stdin --fix", clean)
	}
	if !strings.Contains(clean, "--quiet") {
		t.Errorf("clean filter must be quiet or the report joins the document: %q", clean)
	}
	if got := gitConfig(t, dir, "filter.untrace.smudge"); got != "cat" {
		t.Errorf("smudge = %q, want cat", got)
	}
}

func TestInstallFilterRemove(t *testing.T) {
	dir := gitRepo(t)

	inDir(t, dir, func() {
		if code := runInstallFilter(nil); code != 0 {
			t.Fatalf("install exited %d", code)
		}
		if code := runInstallFilter([]string{"--remove"}); code != 0 {
			t.Fatalf("remove exited %d", code)
		}
	})

	if got := gitConfig(t, dir, "filter.untrace.clean"); got != "" {
		t.Errorf("clean filter still set after --remove: %q", got)
	}
}

func TestInstallFilterRemoveIsIdempotent(t *testing.T) {
	dir := gitRepo(t)
	inDir(t, dir, func() {
		// Unsetting a missing key exits non-zero in git; that must not surface.
		if code := runInstallFilter([]string{"--remove"}); code != 0 {
			t.Errorf("removing an absent filter exited %d, want 0", code)
		}
	})
}

func TestQuoteCommand(t *testing.T) {
	cases := map[string]string{
		"/usr/local/bin/untrace":        "/usr/local/bin/untrace",
		"/Applications/My Apps/untrace": `"/Applications/My Apps/untrace"`,
	}
	for in, want := range cases {
		if got := quoteCommand(in); got != want {
			t.Errorf("quoteCommand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilterCommandUsesAbsolutePath(t *testing.T) {
	dir := gitRepo(t)
	inDir(t, dir, func() {
		if code := runInstallFilter(nil); code != 0 {
			t.Fatalf("install exited %d", code)
		}
	})
	clean := gitConfig(t, dir, "filter.untrace.clean")
	// A bare name would depend on PATH at the time git runs the filter.
	if !filepath.IsAbs(strings.Trim(strings.Fields(clean)[0], `"`)) {
		t.Errorf("clean filter should use an absolute path, got %q", clean)
	}
}
