package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
)

// Names an occurrence without its line, so text inserted above neither raises a
// new alert nor invalidates a baseline entry. Occurrence counts prior findings
// of the same codepoint in the same file.
func Identity(path, codepoint string, occurrence int) string {
	h := sha256.New()
	h.Write([]byte(filepath.ToSlash(path)))
	h.Write([]byte{0})
	h.Write([]byte(codepoint))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(occurrence)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
