package textfile

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAtomicReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(path, []byte("after")); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("after")) {
		t.Errorf("content = %q, want %q", got, "after")
	}
}

func TestWriteAtomicPreservesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes differ on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(path, []byte("#!/bin/sh\necho hi\n")); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// An executable that loses its bit after cleaning is a broken repository.
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("mode = %v, want 0755", got)
	}
}

func TestWriteAtomicCreatesANewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	if err := WriteAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestWriteAtomicLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("after")); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only a.txt", names)
	}
}

func TestWriteAtomicFailsAndKeepsTheOriginal(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs an unwritable directory")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if err := WriteAtomic(path, []byte("replacement")); err == nil {
		t.Fatal("expected an error writing into a read-only directory")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("original")) {
		t.Errorf("original was damaged: %q", got)
	}
}

func TestWriteAtomicHandlesBinaryContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blob.bin")
	data := []byte{0x00, 0xFF, 0x89, 'P', 'N', 'G', 0x00}
	if err := WriteAtomic(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("got %q, want %q", got, data)
	}
}
