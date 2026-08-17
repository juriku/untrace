package scan

import "testing"

var filesSink []string

// Walking allocates per file legitimately, so the ceiling is per file rather
// than per walk.
func TestWalkAllocationsPerFile(t *testing.T) {
	const files = 200
	const ceiling = 12

	root := flatTree(t, files)
	mkfile(t, root, ".gitignore", "*.tmp\nbuild/\n")
	opts := gitignoreOpts(true)

	total := testing.AllocsPerRun(3, func() {
		filesSink, _ = Files(root, opts)
	})

	perFile := total / files
	if perFile > ceiling {
		t.Errorf("%.1f allocations per file, ceiling %d", perFile, ceiling)
	}
}
