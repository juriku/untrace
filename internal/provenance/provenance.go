// Package provenance interprets metadata values as evidence of how a file was
// produced.
//
// A C2PA credential is deliberately not treated as an AI signal on its own.
// Camera makers sign photographs with C2PA to prove they are authentic, so the
// presence of a manifest means provenance was declared, not that a model made
// the file. Only a named generator is evidence of that.
package provenance

import "strings"

type Signal struct {
	// Source is where the evidence came from, such as "exif Software".
	Source string `json:"source"`
	Detail string `json:"detail"`
	// AI is true only when a generator was named.
	AI bool `json:"ai"`
}

// Names matched against metadata values. Both sides are reduced to lowercase
// letters and digits first, so punctuation in a product name cannot cause a
// miss: the several ways vendors punctuate a product name all reduce alike.
var generators = []string{
	"claude", "anthropic",
	"chatgpt", "openai", "dalle", "gpt4", "gpt5", "sora",
	"gemini", "imagen", "google ai",
	"midjourney",
	"stable diffusion", "stability ai", "sdxl", "automatic1111", "comfyui",
	"firefly",
	"copilot",
	"llama", "meta ai",
	"grok", "x.ai",
	"black forest labs", "flux.1",
	"leonardo.ai", "ideogram", "runway", "novelai", "playground ai",
}

var normalized = buildNormalized()

func buildNormalized() map[string]string {
	out := make(map[string]string, len(generators))
	for _, g := range generators {
		out[normalize(g)] = g
	}
	return out
}

func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Generator reports the AI tool named by a metadata value, if any.
func Generator(value string) (string, bool) {
	v := normalize(value)
	if v == "" {
		return "", false
	}
	for key, name := range normalized {
		if strings.Contains(v, key) {
			return name, true
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
