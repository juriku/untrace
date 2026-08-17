package untrace_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func staged(t *testing.T, dir, name string) []byte {
	t.Helper()
	cmd := exec.Command("git", "show", ":"+name)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show :%s: %v", name, err)
	}
	return out
}

func TestCleanFilterStagesTextAndBinary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	png, err := os.ReadFile(filepath.Join("testdata", "cases", "provenance", "in", "camera.png"))
	if err != nil {
		t.Fatal(err)
	}
	text := []byte("invoice" + string(rune(0x200B)) + " total\n")

	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", ".")
	gitIn(t, dir, "config", "user.email", "test@example.com")
	gitIn(t, dir, "config", "user.name", "test")
	gitIn(t, dir, "config", "filter.untrace.clean", `"`+binary+`" --stdin --fix --quiet`)
	gitIn(t, dir, "config", "filter.untrace.smudge", "cat")

	for name, content := range map[string][]byte{
		".gitattributes": []byte("* filter=untrace\n"),
		"photo.png":      png,
		"invoice.txt":    text,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	gitIn(t, dir, "add", "photo.png", "invoice.txt")

	if got := staged(t, dir, "photo.png"); !bytes.Equal(got, png) {
		t.Errorf("staged png differs: %d bytes in, %d out", len(png), len(got))
	}

	// Without this the png assertion passes when the filter never ran at all.
	got := staged(t, dir, "invoice.txt")
	if bytes.ContainsRune(got, 0x200B) {
		t.Error("zero width space survived staging")
	}
	if want := []byte("invoice total\n"); !bytes.Equal(got, want) {
		t.Errorf("staged text = %q, want %q", got, want)
	}
}
