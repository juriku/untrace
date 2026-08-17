package scan

import (
	"runtime"
	"testing"
)

var filesSink []string

// Walking allocates per file legitimately, so the ceiling is per file rather
// than per walk.
func TestWalkAllocationsPerFile(t *testing.T) {
	const files = 200

	// Every path crossing a Windows syscall is converted to UTF-16, so its floor
	// sits above the POSIX one.
	ceiling := 12.0
	if runtime.GOOS == "windows" {
		ceiling = 20
	}

	root := flatTree(t, files)
	mkfile(t, root, ".gitignore", "*.tmp\nbuild/\n")
	opts := gitignoreOpts(true)

	total := testing.AllocsPerRun(3, func() {
		filesSink, _ = Files(root, opts)
	})

	perFile := total / files
	if perFile > ceiling {
		t.Errorf("%.1f allocations per file, ceiling %.0f", perFile, ceiling)
	}
}
