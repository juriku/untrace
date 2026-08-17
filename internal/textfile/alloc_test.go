package textfile

import "testing"

var (
	boolSink bool
	decSink  Decoded
)

// IsBinary runs against every file a scan opens, before anything else does.
func TestIsBinaryAllocatesNothing(t *testing.T) {
	data := []byte(corpusText)
	got := testing.AllocsPerRun(5, func() { boolSink = IsBinary(data) })
	if got > 0 {
		t.Errorf("%v allocations, want 0", got)
	}
}

func TestDecodeIsASingleAllocation(t *testing.T) {
	cases := []struct {
		name string
		enc  Encoding
	}{
		{"utf-8", UTF8},
		{"latin-1", Latin1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := encodedAs(t, tc.enc, false)
			got := testing.AllocsPerRun(5, func() {
				decSink, _ = Decode(data)
			})
			if got > 2 {
				t.Errorf("%v allocations, ceiling 2", got)
			}
		})
	}
}
