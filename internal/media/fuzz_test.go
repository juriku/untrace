package media

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func seedImages(f *testing.F) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))

	var p bytes.Buffer
	if err := png.Encode(&p, img); err == nil {
		f.Add(p.Bytes())
	}
	var j bytes.Buffer
	if err := jpeg.Encode(&j, img, nil); err == nil {
		f.Add(j.Bytes())
	}

	f.Add([]byte("not an image"))
	f.Add(pngSignature)
	f.Add([]byte{0xFF, 0xD8})
	f.Add([]byte{})
}

func FuzzInspect(f *testing.F) {
	seedImages(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		Inspect(data)
	})
}

func isSubsequence(out, in []byte) bool {
	i := 0
	for _, b := range in {
		if i < len(out) && out[i] == b {
			i++
		}
	}
	return i == len(out)
}

func FuzzStrip(f *testing.F) {
	seedImages(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		out, changed, err := Strip(data, everyRecord)
		if err != nil {
			return
		}
		if !changed && !bytes.Equal(out, data) {
			t.Fatal("Strip reported no change but returned different bytes")
		}
		// Stripping metadata must not turn a recognised container into an
		// unrecognised one; the picture has to survive the operation.
		if before := Detect(data); before != FormatUnknown {
			if after := Detect(out); after != before {
				t.Fatalf("format changed from %q to %q", before, after)
			}
		}
		if !isSubsequence(out, data) {
			t.Fatalf("Strip produced bytes that were not in the input")
		}
	})
}
