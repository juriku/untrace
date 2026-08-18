package detect

import (
	"strings"
	"testing"

	"github.com/juriku/untrace/internal/resolve"
)

// The compatibility-forms detector was removed after it rewrote these; see
// docs/design/watermark-techniques.md section E. Nothing here may be detected.
func TestNonLatinProseIsNotDetected(t *testing.T) {
	fullwidth := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			b.WriteRune(r + 0xFEE0)
		}
		return b.String()
	}

	cases := map[string]string{
		"fullwidth title before any CJK": "# " + fullwidth("MyTool") + "\n\n日本語のテキスト\n",
		"fullwidth title after CJK":      "日本語のテキスト\n\n# " + fullwidth("MyTool") + "\n",
		"fullwidth in a bilingual doc": strings.Repeat("The quick brown fox jumps over the lazy dog. ", 8) +
			"全角の記号：" + fullwidth("API") + "\n",
		"japanese question mark":  "これはテストですか" + string(rune(0xFF1F)) + "\n",
		"japanese after a quote":  "He said 「テスト」" + string(rune(0xFF1F)) + "\n",
		"korean colon and stop":   "요약" + string(rune(0xFF1A)) + "새 버전 출시" + string(rune(0xFF0E)) + "\n",
		"fullwidth digits in CJK": "価格は" + fullwidth("1980") + "円です。\n",
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			res := newDetector(resolve.FormatProse, true).Run(in)

			if len(res.Findings) != 0 {
				t.Errorf("reported %+v", res.Findings)
			}
			if res.Text != in {
				t.Errorf("rewrote the text:\n got %q\nwant %q", res.Text, in)
			}
		})
	}
}

// C1 is where a latin-1 decode puts cp1252 punctuation, so detecting it deletes
// apostrophes and quotation marks from ordinary Windows-authored text.
func TestC1ControlsAreNotDetected(t *testing.T) {
	in := "It" + string(rune(0x92)) + "s a caf" + string(rune(0xE9)) + " " +
		string(rune(0x93)) + "quoted" + string(rune(0x94)) + " " + string(rune(0x97)) + " dash"

	res := newDetector(resolve.FormatProse, true).Run(in)

	if len(res.Findings) != 0 {
		t.Errorf("reported %+v", res.Findings)
	}
	if res.Text != in {
		t.Errorf("rewrote the text: %q", res.Text)
	}
}
