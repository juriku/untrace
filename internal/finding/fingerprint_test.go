package finding

import (
	"path/filepath"
	"testing"
)

func TestIdentityIgnoresTheLine(t *testing.T) {
	a := Identity("src/main.go", "U+200B", 0)
	b := Identity("src/main.go", "U+200B", 0)

	if a != b {
		t.Errorf("identity is not deterministic: %q then %q", a, b)
	}
}

func TestIdentitySeparatesOccurrences(t *testing.T) {
	first := Identity("src/main.go", "U+200B", 0)
	second := Identity("src/main.go", "U+200B", 1)

	if first == second {
		t.Errorf("two occurrences share identity %q", first)
	}
}

func TestIdentitySeparatesFilesAndCodepoints(t *testing.T) {
	base := Identity("a.go", "U+200B", 0)

	if other := Identity("b.go", "U+200B", 0); other == base {
		t.Error("two files share an identity")
	}
	if other := Identity("a.go", "U+2014", 0); other == base {
		t.Error("two codepoints share an identity")
	}
}

// A path the walker built must hash the same on every platform, or a baseline
// written on one rejects every entry on the other.
func TestIdentityIsSeparatorIndependent(t *testing.T) {
	native := filepath.Join("src", "main.go")

	if got, want := Identity(native, "U+200B", 0), Identity("src/main.go", "U+200B", 0); got != want {
		t.Errorf("Identity(%q) = %q, want %q", native, got, want)
	}
}
