// Package provenance interprets metadata values as evidence of how a file was
// produced.
//
// A C2PA credential is deliberately not treated as an AI signal on its own.
// Camera makers sign photographs with C2PA to prove they are authentic, so the
// presence of a manifest means provenance was declared, not that a model made
// the file. Only a named generator is evidence of that.
package provenance

import (
	"regexp"
	"strings"
)

type Signal struct {
	// Source is where the evidence came from, such as "exif Software".
	Source string `json:"source"`
	Detail string `json:"detail"`
	// AI is true only when a generator was named.
	AI bool `json:"ai"`
}

// No leading \b: inside a C2PA manifest a CBOR length byte sits flush against
// the name, as in "_generatorqClaude". "kimi" is the exception, since a surname
// ends in it. A name that is also ordinary English or a company name needs a
// version beside it, or "Firefly LED Signage" reads as Adobe Firefly.
var generators = []struct {
	name    string
	pattern string
}{
	{"claude", `claude\b`},
	{"anthropic", `anthropic\b`},
	{"chatgpt", `chat ?gpt\b`},
	{"openai", `open ?ai\b`},
	{"dalle", `dall ?e`},
	{"sora", `sora\b`},
	{"midjourney", `mid ?journey\b`},
	{"stable diffusion", `stable ?diffusion\b`},
	{"stability ai", `stability ?ai\b`},
	{"sdxl", `sdxl\b`},
	{"automatic1111", `automatic1111\b`},
	{"comfyui", `comfy ?ui\b`},
	{"ideogram", `ideogram\b`},
	{"novelai", `novel ?ai\b`},
	{"leonardo.ai", `leonardo ?ai\b`},
	{"playground ai", `playground ?ai\b`},
	{"black forest labs", `black ?forest ?labs\b`},
	{"deepseek", `deep ?seek\b`},
	{"perplexity", `perplexity\b`},
	{"mistral", `mistral\b`},
	{"qwen", `qwen`},
	{"moonshot", `moonshot\b`},
	{"kimi", `\bkimi\b`},
	{"grok", `grok\b`},
	{"llama", `llama\b`},
	{"gemini", `gemini\b`},
	{"copilot", `copilot\b`},
	{"meta ai", `meta ?ai\b`},
	{"x.ai", `x ?ai\b`},
	{"google ai", `google ?ai\b`},

	{"gpt", `gpt ?\d`},
	{"glm", `glm ?\d`},
	{"flux.1", `flux ?\d`},
	{"firefly", `firefly ?\d`},
	{"imagen", `imagen ?\d`},
	{"runway", `runway ?(?:ml|gen)\b`},
}

var compiled = compileGenerators()

type generatorPattern struct {
	name string
	re   *regexp.Regexp
}

func compileGenerators() []generatorPattern {
	out := make([]generatorPattern, 0, len(generators))
	for _, g := range generators {
		out = append(out, generatorPattern{name: g.name, re: regexp.MustCompile(g.pattern)})
	}
	return out
}

// Punctuation collapses to a space rather than being deleted: deleting it would
// make "imagenes" contain "imagen".
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	gap := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if gap && b.Len() > 0 {
				b.WriteByte(' ')
			}
			gap = false
			b.WriteRune(r)
			continue
		}
		gap = true
	}
	return b.String()
}

// Generator reports the AI tool named by a metadata value, if any.
func Generator(value string) (string, bool) {
	v := fold(value)
	if v == "" {
		return "", false
	}
	for _, g := range compiled {
		if g.re.MatchString(v) {
			return g.name, true
		}
	}
	return "", false
}

type Confidence string

const (
	// Possible is a marker that could be innocent.
	Possible Confidence = "possible"
	// Likely is a marker in a file that names an AI generator.
	Likely Confidence = "likely"
	// Certain is reserved for evidence that cannot occur by accident, such as a
	// run of carrier characters that decodes to readable text.
	Certain Confidence = "certain"
)

// Assess turns the signals found in one file into a confidence level for the
// other findings in it.
func Assess(signals []Signal) Confidence {
	for _, s := range signals {
		if s.AI {
			return Likely
		}
	}
	return Possible
}
