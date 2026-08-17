package docmeta

import (
	"strings"
	"testing"
)

var formatSink Format

// Detect runs against every file a scan opens, and almost none of them are
// containers.
func TestDetectPlainDataAllocatesNothing(t *testing.T) {
	data := []byte(strings.Repeat("ordinary text that is not a container. ", 1000))
	got := testing.AllocsPerRun(5, func() { formatSink = Detect(data) })
	if got > 0 {
		t.Errorf("%v allocations, want 0", got)
	}
}
