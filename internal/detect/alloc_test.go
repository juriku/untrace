package detect

import (
	"runtime"
	"testing"

	"github.com/juriku/untrace/internal/decode"
	"github.com/juriku/untrace/internal/resolve"
)

var resultSink Result

// The distinction that matters is a fixed number of buffers against one
// allocation per rune, so the input grows fourfold and the count must not.
func TestAllocationsDoNotScaleWithInput(t *testing.T) {
	const unit = "the quick brown fox jumps over the lazy dog. "
	d := newDetector(resolve.FormatSource, false)

	allocs := func(text string) float64 {
		return testing.AllocsPerRun(3, func() { resultSink = d.Run(text) })
	}

	small := allocs(repeatTo(unit, 64<<10))
	large := allocs(repeatTo(unit, 256<<10))

	if large > small+2 {
		t.Errorf("64KB cost %v allocations, 256KB cost %v", small, large)
	}
}

// Markers must cost a bounded number of allocations between them, not one each.
// The count is markers met rather than findings reported: Lookup runs before
// the legitimacy resolver, so a marker suppressed as legitimate has already
// cost whatever Lookup spends.
func TestAllocationsDoNotScaleWithMarkerCount(t *testing.T) {
	const size = 256 << 10

	sparse := repeatTo("plain words with one "+emDash+" every so often. ", size)

	// One case per branch of Lookup: a per-marker cost in any of them is
	// invisible to the others.
	cases := []struct {
		name  string
		dense string
	}{
		{"typographic", repeatTo("a"+emDash+"b"+lquote+"c"+rquote+"d ", size)},
		{"hidden", repeatTo("a"+nbsp+"b"+zwsp+"c"+thin+"d ", size)},
		{"ideographic variation selector",
			repeatTo(string(rune(0x8FBB))+ivsChar+string(rune(0x9089))+ivsChar+" ", size)},
	}

	d := newDetector(resolve.FormatSource, false)
	allocs := func(text string) float64 {
		return testing.AllocsPerRun(3, func() { resultSink = d.Run(text) })
	}
	markers := func(text string) int {
		r := d.Run(text)
		return len(r.Findings) + r.Legitimate
	}

	sparseAllocs := allocs(sparse)
	sparseMarkers := markers(sparse)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			denseMarkers := markers(tc.dense)
			if denseMarkers < sparseMarkers*4 {
				t.Fatalf("corpus is not dense enough: %d markers against %d",
					denseMarkers, sparseMarkers)
			}

			// Doubling growth rises with the logarithm of the count, so a few
			// extra allocations are expected and a multiple is not.
			if got := allocs(tc.dense); got > sparseAllocs+16 {
				t.Errorf("%d markers cost %v allocations, %d markers cost %v",
					sparseMarkers, sparseAllocs, denseMarkers, got)
			}
		})
	}
}

// The floor is one []rune conversion, which is 4 bytes per input byte on ASCII.
func TestBytesAllocatedPerInputByte(t *testing.T) {
	const ceiling = 8

	corpora := map[string]string{
		"clean":           corpusCleanASCII,
		"one mixed word":  corpusCleanASCII + " p" + string(rune(0x0430)) + "ypal\n",
		"one tag payload": corpusCleanASCII + " " + decode.EncodeTag("leak") + "\n",
	}

	d := newDetector(resolve.FormatSource, false)

	for name, corpus := range corpora {
		t.Run(name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			resultSink = d.Run(corpus)
			runtime.ReadMemStats(&after)

			ratio := float64(after.TotalAlloc-before.TotalAlloc) / float64(len(corpus))
			if ratio > ceiling {
				t.Errorf("%.1f bytes allocated per input byte, ceiling %d", ratio, ceiling)
			}
		})
	}
}

// Detect-only must not build the output document.
func TestDetectOnlyCostsLessThanFix(t *testing.T) {
	detectOnly := testing.AllocsPerRun(3, func() {
		resultSink = newDetector(resolve.FormatSource, false).Run(corpusTypical)
	})
	fix := testing.AllocsPerRun(3, func() {
		resultSink = newDetector(resolve.FormatSource, true).Run(corpusTypical)
	})

	if detectOnly > fix {
		t.Errorf("detect-only cost %v allocations, fix cost %v", detectOnly, fix)
	}
}
