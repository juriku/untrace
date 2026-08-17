package scan

import (
	"path/filepath"
	"testing"
)

// os.UserHomeDir reads HOME on unix and USERPROFILE on Windows, so both are set.
func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"tilde alone", "~", home},
		{"tilde with path", "~/.config/git/ignore", filepath.Join(home, ".config", "git", "ignore")},
		{"absolute path untouched", "/etc/gitignore", "/etc/gitignore"},
		{"relative path untouched", "config/ignore", "config/ignore"},
		{"empty stays empty", "", ""},
		{"tilde inside the path is not expanded", "/tmp/~/x", "/tmp/~/x"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandHome(tc.in); got != tc.want {
				t.Errorf("expandHome(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Failing open here means a global excludesFile silently stops applying, so the
// scan covers files git would have skipped rather than skipping files it should
// have scanned.
func TestExpandHomeWithoutAHomeReturnsThePath(t *testing.T) {
	setHome(t, "")

	const path = "~/.config/git/ignore"
	if got := expandHome(path); got != path {
		t.Errorf("expandHome(%q) = %q, want it returned unchanged", path, got)
	}
}
