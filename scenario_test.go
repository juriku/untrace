package untrace_test

import (
	"os"
	"strings"
	"testing"
)

func TestScanADeepTree(t *testing.T) {
	d := NewDir(t).
		File("README.md", "clean\n").
		File("src/app/main.go", "x := \"a"+zwsp+"b\"\n").
		File("src/app/deep/nested/util.go", "y := \"c"+zwsp+"d\"\n").
		File("docs/guide/intro.md", "a dash "+emDash+" here\n")

	d.Run(".").Exit(0).StdoutHas("main.go", "util.go", "intro.md").StdoutLacks("README.md")
	d.Run("--fail", ".").Exit(1)
	d.Run("--fix", ".").Exit(0)

	for _, f := range []string{"src/app/main.go", "src/app/deep/nested/util.go"} {
		if strings.Contains(d.Read(f), zwsp) {
			t.Errorf("%s still has a zero-width space", f)
		}
	}
	d.Run("--fail", ".").Exit(0)
}

func TestScanASinglePathNotTheWholeTree(t *testing.T) {
	d := NewDir(t).
		File("a.md", "one "+emDash+" here\n").
		File("b.md", "two "+emDash+" here\n")

	d.Run("--fix", "a.md").Exit(0)

	if strings.Contains(d.Read("a.md"), emDash) {
		t.Error("the named file was not fixed")
	}
	if !strings.Contains(d.Read("b.md"), emDash) {
		t.Error("a file that was not named was fixed")
	}
}

// The README says symlinks are never followed, so a scan cannot escape the tree
// it was pointed at and --fix cannot write through a link.
func TestSymlinksAreNotFollowed(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("symlinks need privilege on windows")
	}

	outside := NewDir(t).File("secret.md", "outside "+emDash+" the tree\n")
	d := NewDir(t).
		File("real.md", "inside "+emDash+" the tree\n").
		Symlink("link.md", outside.Path("secret.md")).
		Symlink("linkdir", outside.Path())

	d.Run("--fix", ".").Exit(0).StdoutHas("real.md").StdoutLacks("link.md", "secret.md")

	if !strings.Contains(outside.Read("secret.md"), emDash) {
		t.Error("--fix wrote through a symlink and changed a file outside the tree")
	}
}

func TestBrokenSymlinkDoesNotFail(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("symlinks need privilege on windows")
	}

	d := NewDir(t).
		File("real.md", "a dash "+emDash+" here\n").
		Symlink("dangling.md", "no-such-target.md")

	d.Run(".").Exit(0).StdoutHas("real.md")
}

func TestSymlinkLoopTerminates(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("symlinks need privilege on windows")
	}

	d := NewDir(t).
		File("real.md", "a dash "+emDash+" here\n").
		Mkdir("a").
		Symlink("a/loop", "..")

	d.Run(".").Exit(0).StdoutHas("real.md")
}

// A file the process cannot read is an error, not a silent skip: reporting a
// tree clean because part of it was unreadable is the worst possible answer.
func TestUnreadableFileIsReportedNotSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything")
	}

	d := NewDir(t).
		File("ok.md", "clean\n").
		File("locked.md", "a dash "+emDash+" here\n").
		Chmod("locked.md", 0)

	r := d.Run(".")
	if r.exit == 0 && !strings.Contains(r.Stdout(), "error") {
		t.Errorf("an unreadable file was passed over silently:\n%s", r.Stdout())
	}
}

// A 0444 file is still rewritten, because the atomic rename needs permission on
// the directory rather than on the file. What must survive is the mode.
func TestReadOnlyFileKeepsItsMode(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes anything")
	}

	d := NewDir(t).File("locked.md", "a dash "+emDash+" here\n").Chmod("locked.md", 0o444)

	d.Run("--fix", "locked.md").Exit(0)

	info, err := os.Stat(d.Path("locked.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o444 {
		t.Errorf("mode = %o, want 444", got)
	}
	if strings.Contains(d.Read("locked.md"), emDash) {
		t.Error("the file was not fixed")
	}
}

// A read-only directory blocks the atomic rename, not the read.
func TestReadOnlyDirectoryDoesNotTruncate(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes anything")
	}

	body := "a dash " + emDash + " here\n"
	d := NewDir(t).File("sub/locked.md", body).Chmod("sub", 0o555)

	d.Run("--fix", "sub")

	if got := d.Read("sub/locked.md"); got != body {
		t.Errorf("content changed to %q, want it intact", got)
	}
}

// untrace claims it honours every .gitignore up to the repository root, not
// just the nearest one.
func TestGitignoreFromAParentApplies(t *testing.T) {
	d := NewDir(t).
		Mkdir(".git").
		File(".gitignore", "generated/\n").
		File("src/keep.md", "a dash "+emDash+" here\n").
		File("src/generated/skip.md", "a dash "+emDash+" here\n")

	d.Run("src").Exit(0).StdoutHas("keep.md").StdoutLacks("skip.md")
	d.Run("--no-gitignore", "src").Exit(0).StdoutHas("keep.md", "skip.md")
}

func TestConfigIsDiscoveredByWalkingUp(t *testing.T) {
	d := NewDir(t).
		Mkdir(".git").
		File(".untrace.json", `{"formats":{"prose":{"typographic":"ignore"}}}`).
		File("docs/deep/note.md", "a dash "+emDash+" here\n")

	d.Run("--fail", "docs/deep").Exit(0).StdoutLacks("U+2014")
	d.Run("--fail", "--config", "/dev/null", "docs/deep").Exit(2)
}

// A latin-1 file must come back latin-1, and a file untrace cannot decode
// without changing bytes must be refused rather than rewritten.
func TestEncodingsSurviveAFix(t *testing.T) {
	d := NewDir(t).
		Bytes("latin1.txt", []byte("caf\xe9 au\xa0lait\n")).
		Bytes("utf16.txt", append([]byte{0xFF, 0xFE}, 'h', 0, 'i', 0, '\n', 0))

	d.Run("--fix", ".").Exit(0)

	got := []byte(d.Read("latin1.txt"))
	if want := []byte("caf\xe9 au lait\n"); string(got) != string(want) {
		t.Errorf("latin-1 file = % x, want % x", got, want)
	}
	if head := []byte(d.Read("utf16.txt"))[:2]; head[0] != 0xFF || head[1] != 0xFE {
		t.Errorf("utf-16 file lost its byte order mark: % x", head)
	}
}

// Binary files must come out of a --fix byte-identical.
func TestBinaryFilesAreUntouched(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 64)...)
	d := NewDir(t).Bytes("image.png", png).Bytes("data.bin", []byte{0x00, 0x01, 0xA0, 0xAD, 0xFF})

	d.Run("--fix", ".").Exit(0)

	if got := []byte(d.Read("data.bin")); string(got) != string([]byte{0x00, 0x01, 0xA0, 0xAD, 0xFF}) {
		t.Errorf("binary file changed: % x", got)
	}
	if got := []byte(d.Read("image.png")); string(got) != string(png) {
		t.Error("png changed without --strip-metadata")
	}
}

func TestAdoptALegacyRepositoryWithABaseline(t *testing.T) {
	d := NewDir(t).
		File("legacy/old.md", "login at p"+cyrA+"ypal and a dash "+emDash+"\n").
		File("legacy/more.go", "x := \"a"+zwsp+"b\"\n")

	d.Run("--fail", ".").Exit(1)
	d.Run("--write-baseline", ".").Exit(0)
	d.Run("--fail", "--baseline", ".untrace-baseline.json", ".").Exit(0)

	d.File("legacy/new.go", "y := \"c"+zwsp+"d\"\n")
	d.Run("--fail", "--baseline", ".untrace-baseline.json", ".").Exit(1).StdoutHas("new.go")
}

func TestStdinRedirectCarriesOnlyTheDocument(t *testing.T) {
	d := NewDir(t)

	r := d.RunStdin("hello"+zwsp+"world\n", "--stdin", "--fix", "--stdin-name", "x.md")

	r.Exit(0).StderrHas("removed")
	if r.Stdout() != "helloworld\n" {
		t.Errorf("stdout = %q, want the document alone", r.Stdout())
	}
}

func TestStdinLeavesBinaryUntouched(t *testing.T) {
	d := NewDir(t)

	r := d.RunStdin(string([]byte{0xA0, 0xAD, 0xB4, 0xB7}), "--stdin", "--fix")

	r.Exit(0)
	if r.Stdout() != string([]byte{0xA0, 0xAD, 0xB4, 0xB7}) {
		t.Errorf("stdout = % x, want the bytes back unchanged", r.Stdout())
	}
}

func TestIgnoreDirectivesSurviveAFix(t *testing.T) {
	d := NewDir(t).
		File("a.py", "keep = \"x"+zwsp+"y\"  # untrace:ignore\ndrop = \"p"+zwsp+"q\"\n")

	d.Run("--fix", "a.py").Exit(0).StdoutHas("suppressed")

	body := d.Read("a.py")
	if !strings.Contains(strings.SplitN(body, "\n", 2)[0], zwsp) {
		t.Error("a suppressed line was rewritten")
	}
	if strings.Contains(strings.SplitN(body, "\n", 3)[1], zwsp) {
		t.Error("an unsuppressed line was not fixed")
	}
}

func TestDefaultIgnoredDirectoriesAreSkipped(t *testing.T) {
	d := NewDir(t).
		File("src/a.js", "x = \"a"+zwsp+"b\"\n").
		File("node_modules/dep/index.js", "y = \"c"+zwsp+"d\"\n")

	d.Run(".").Exit(0).StdoutHas("a.js").StdoutLacks("index.js")
	d.Run("--no-default-ignores", ".").Exit(0).StdoutHas("a.js", "index.js")
}

func TestNothingIsWrittenWithoutFix(t *testing.T) {
	body := "a dash " + emDash + " and a space" + nbsp + "here\n"
	d := NewDir(t).File("a.md", body)

	before, err := os.Stat(d.Path("a.md"))
	if err != nil {
		t.Fatal(err)
	}

	d.Run("--fail", ".").Exit(1)

	if d.Read("a.md") != body {
		t.Error("a scan without --fix changed the file")
	}
	after, err := os.Stat(d.Path("a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("a scan without --fix touched the modification time")
	}
}
