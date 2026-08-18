package markers

import "fmt"

// The Tags block mirrors printable ASCII: U+E0020..U+E007E map to 0x20..0x7E.
// A run of them is an invisible ASCII string, which is why it is the preferred
// carrier for both watermarking and prompt injection.
const (
	tagFirst      = 0xE0000
	tagLast       = 0xE007F
	tagPrintFirst = 0xE0020
	tagPrintLast  = 0xE007E
	tagASCIIBase  = 0xE0000
)

// TagToASCII returns the ASCII character a tag codepoint stands for.
func TagToASCII(r rune) (byte, bool) {
	if r < tagPrintFirst || r > tagPrintLast {
		return 0, false
	}
	return byte(r - tagASCIIBase), true
}

func IsTag(r rune) bool { return r >= tagFirst && r <= tagLast }

var tagNames = buildTagNames()

func buildTagNames() []string {
	out := make([]string, tagLast-tagFirst+1)
	for i := range out {
		r := rune(tagFirst + i)
		switch {
		case r == 0xE0001:
			out[i] = "Language Tag"
		case r == 0xE007F:
			out[i] = "Cancel Tag"
		default:
			if b, ok := TagToASCII(r); ok {
				out[i] = fmt.Sprintf("Tag %q", string(rune(b)))
			} else {
				out[i] = "Tag Character"
			}
		}
	}
	return out
}

func tagName(r rune) string {
	if !IsTag(r) {
		return "Tag Character"
	}
	return tagNames[r-tagFirst]
}

// Blank-rendering and format characters absent from the original tables.
// Those that occupy visual width normalise to a space; zero-width ones are
// removed, so cleaning never joins two words together.
var extraHidden = map[rune]struct {
	name    string
	replace string
	noClean bool
}{
	0x115F: {"Hangul Choseong Filler", "", false},
	0x1160: {"Hangul Jungseong Filler", "", false},
	0x3164: {"Hangul Filler", " ", false},
	0xFFA0: {"Halfwidth Hangul Filler", " ", false},
	0x2800: {"Braille Pattern Blank", " ", false},
	0x17B4: {"Khmer Vowel Inherent Aq", "", false},
	0x17B5: {"Khmer Vowel Inherent Aa", "", false},
	0x206A: {"Inhibit Symmetric Swapping", "", false},
	0x206B: {"Activate Symmetric Swapping", "", false},
	0x206C: {"Inhibit Arabic Form Shaping", "", false},
	0x206D: {"Activate Arabic Form Shaping", "", false},
	0x206E: {"National Digit Shapes", "", false},
	0x206F: {"Nominal Digit Shapes", "", false},
	0xFFF9: {"Interlinear Annotation Anchor", "", false},
	0xFFFA: {"Interlinear Annotation Separator", "", false},
	0xFFFB: {"Interlinear Annotation Terminator", "", false},

	// Evidence that a decode already went wrong; deleting it would destroy the
	// only sign that the file was damaged.
	0xFFFD: {"Replacement Character", "", true},
}

// Dependency, cache and fixture directories skipped during recursive scans.
// Ambiguous names such as build, dist, out, target, bin, obj and vendor are
// deliberately absent: they hold real source in some projects.
var DefaultIgnoredDirs = map[string]bool{
	".angular":         true,
	".astro":           true,
	".bundle":          true,
	".cache":           true,
	".dart_tool":       true,
	".eggs":            true,
	".git":             true,
	".gradle":          true,
	".hg":              true,
	".mypy_cache":      true,
	".next":            true,
	".nox":             true,
	".nuxt":            true,
	".nyc_output":      true,
	".parcel-cache":    true,
	".pnpm-store":      true,
	".pytest_cache":    true,
	".pytype":          true,
	".ruff_cache":      true,
	".sass-cache":      true,
	".serverless":      true,
	".svelte-kit":      true,
	".svn":             true,
	".terraform":       true,
	".tests":           true,
	".tox":             true,
	".turbo":           true,
	".venv":            true,
	".vite":            true,
	".yarn":            true,
	"Pods":             true,
	"__pycache__":      true,
	"__snapshots__":    true,
	"bower_components": true,
	"htmlcov":          true,
	"jspm_packages":    true,
	"node_modules":     true,
	"testdata":         true,
	"venv":             true,
}

// Never reported, in any format. The middle dot is a letter in Catalan "l·l".
var neverDetected = map[rune]bool{
	0x2026: true, // horizontal ellipsis
	0x2022: true, // bullet
	0x00B7: true, // middle dot
}

func NeverDetected(r rune) bool { return neverDetected[r] }

// Punctuation belonging to a writing system rather than to typography. Reported
// when it appears out of context, but never rewritten: the ASCII counterpart is
// a different character in the language that owns it, so replacing U+3002 with
// a full stop corrupts Japanese rather than normalising it.
var neverCleaned = map[rune]bool{
	0x037E: true, // greek question mark
	0x060C: true, // arabic comma
	0x3000: true, // ideographic space
	0x3002: true, // ideographic full stop
	0xFE50: true, // small comma
	0xFE52: true, // small full stop
	0xFE55: true, // small colon
	0xFE56: true, // small semicolon
	0xFE63: true, // small hyphen-minus
	0xFF01: true, // fullwidth exclamation mark
	0xFF07: true, // fullwidth apostrophe
	0xFF0C: true, // fullwidth comma
	0xFF0E: true, // fullwidth full stop
	0xFF0F: true, // fullwidth solidus
	0xFF1A: true, // fullwidth colon
	0xFF1B: true, // fullwidth semicolon
	0xFF1F: true, // fullwidth question mark
}

func NeverCleaned(r rune) bool { return neverCleaned[r] }
