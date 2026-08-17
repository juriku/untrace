package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const filterName = "untrace"

// A clean filter rewrites content on its way into the index. Git's own
// documentation warns that a project must stay usable without the filter, and
// the driver definition lives in gitconfig, which cannot be committed, so this
// is opt-in per clone rather than the default way to use untrace.
func runInstallFilter(args []string) int {
	fs := flag.NewFlagSet("untrace install-filter", flag.ContinueOnError)
	global := fs.Bool("global", false, "write to the user's global git config")
	remove := fs.Bool("remove", false, "remove the filter configuration")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: untrace install-filter [--global] [--remove]

Registers a git clean filter that runs untrace over content as it is staged.

After installing, tell git which files to route through it by adding a line to
.gitattributes, which you can commit:

    * filter=untrace

To undo, run with --remove and delete that line.

Caveats worth knowing before you use this:

  - The filter definition lives in git config and cannot be committed, so every
    clone needs this command run again.
  - Changing the filter later requires 'git add --renormalize .'.
  - Your working tree will differ from the index, which surprises people.

A pre-commit hook is the less surprising choice for most projects.

flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	scope := "--local"
	if *global {
		scope = "--global"
	}

	if *remove {
		for _, key := range []string{"clean", "smudge", "required"} {
			// A missing key exits 5; that is success for a removal.
			_ = exec.Command("git", "config", scope, "--unset",
				"filter."+filterName+"."+key).Run()
		}
		fmt.Fprintln(os.Stderr, "untrace: filter removed")
		return 0
	}

	self, err := os.Executable()
	if err != nil {
		self = "untrace"
	}

	settings := [][2]string{
		{"clean", quoteCommand(self) + " --stdin --fix --quiet"},
		{"smudge", "cat"},
		{"required", "true"},
	}
	for _, kv := range settings {
		cmd := exec.Command("git", "config", scope, "filter."+filterName+"."+kv[0], kv[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "untrace: git config failed: %v\n%s", err, out)
			return 2
		}
	}

	fmt.Fprintf(os.Stderr, `untrace: clean filter installed (%s)

Add this to .gitattributes to route files through it:

    * filter=untrace

Then run 'git add --renormalize .' to apply it to existing files.
`, strings.TrimPrefix(scope, "--"))
	return 0
}

func quoteCommand(path string) string {
	if strings.ContainsAny(path, " \t\"") {
		return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
	}
	return path
}
