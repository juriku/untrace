package media

import "testing"

var (
	formatSink Format
	reportSink Report
)

// Detect runs against every file a scan opens.
func TestDetectAllocatesNothing(t *testing.T) {
	data := benchPNG(t, 64, 64)
	got := testing.AllocsPerRun(5, func() { formatSink = Detect(data) })
	if got > 0 {
		t.Errorf("%v allocations, want 0", got)
	}
}

func TestInspectCleanImageIsASmallConstant(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"png", benchPNG(t, 256, 256)},
		{"jpeg", benchJPEG(t, 256, 256)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := testing.AllocsPerRun(5, func() { reportSink = Inspect(tc.data) })
			if got > 10 {
				t.Errorf("%v allocations, ceiling 10", got)
			}
		})
	}
}
