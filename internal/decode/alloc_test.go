package decode

import "testing"

var payloadSink []Payload

// A run that never reaches its minimum length must cost the same as no run at
// all, or a file of stray carrier characters pays for buffers it discards.
func TestTextWithoutPayloadsAllocatesNothing(t *testing.T) {
	cases := []struct {
		name  string
		runes []rune
	}{
		{"no carriers", corpusNoCarriers},
		{"runs below minimum", corpusShortRuns},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := testing.AllocsPerRun(3, func() { payloadSink = Payloads(tc.runes) })
			if got > 0 {
				t.Errorf("%v allocations, want 0", got)
			}
		})
	}
}
