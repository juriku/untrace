package textfile

import (
	"os"
	"path/filepath"
)

// WriteAtomic replaces path via a temp file in the same directory, so a crash
// mid-write leaves the original intact and the rename is atomic. The temp file
// must share a filesystem with the target for that guarantee to hold.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".untrace-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
