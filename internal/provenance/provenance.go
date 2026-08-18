// Package provenance interprets metadata values as evidence of how a file was
// produced.
//
// A C2PA credential is deliberately not treated as an AI signal on its own.
// Camera makers sign photographs with C2PA to prove they are authentic, so the
// presence of a manifest means provenance was declared, not that a model made
// the file. Evidence of that is a named generator, or a TC260 label.
package provenance

import (
	"bytes"
	"regexp"
	"strings"
)

type Signal struct {
	// Source is where the evidence came from, such as "exif Software".
	Source string `json:"source"`
	Detail string `json:"detail"`
	// AI is true when a generator was named, or a TC260 label declared origin.
	AI bool `json:"ai"`
}

// Patterns carry no leading \b of their own: compileGenerators adds one for
// metadata values, and omits it for manifests, where a CBOR length byte sits
// flush against the name as in "_generatorqClaude".
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

	{"doubao", `doubao\b`},
	{"jimeng", `jimeng\b`},
	{"kling", `\bkling\b`},
	{"keling", `keling\b`},
	{"yuanbao", `yuanbao\b`},
	{"ernie", `ernie ?(?:bot|\d)`},
	{"wenxin", `wenxin\b`},
	{"liblib", `liblib(?: ?ai)?\b`},
	{"hailuo", `hailuo\b`},
	{"minimax", `minimax ?(?:ai|abab|\d)`},
	{"seedream", `seedream\b`},
	{"seedance", `seedance\b`},
	{"hunyuan", `hunyuan\b`},
	{"zhipu", `zhipu\b`},
	{"tongyi", `tongyi\b`},
	{"nano banana", `nano ?banana\b`},

	{"gpt", `gpt ?\d`},
	{"glm", `glm ?\d`},
	{"flux.1", `flux ?\d`},
	{"firefly", `firefly ?\d`},
	{"imagen", `imagen ?\d`},
	{"runway", `runway ?(?:ml|gen)\b`},
	// "wan" is Dutch and Indonesian, "vidu" is Latin and Romanian.
	{"wan", `wan ?\d`},
	{"vidu", `vidu ?(?:q|\d)`},
}

var (
	anchored   = compileGenerators(`\b`)
	unanchored = compileGenerators("")
)

type generatorPattern struct {
	name string
	re   *regexp.Regexp
}

func compileGenerators(prefix string) []generatorPattern {
	out := make([]generatorPattern, 0, len(generators))
	for _, g := range generators {
		out = append(out, generatorPattern{name: g.name, re: regexp.MustCompile(prefix + g.pattern)})
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
func Generator(value string) (string, bool) { return match(anchored, value) }

// GeneratorInManifest is Generator without the leading word boundary, for a
// C2PA manifest where a CBOR length byte precedes the name with no separator.
func GeneratorInManifest(value string) (string, bool) { return match(unanchored, value) }

func match(patterns []generatorPattern, value string) (string, bool) {
	v := fold(value)
	if v == "" {
		return "", false
	}
	for _, g := range patterns {
		if g.re.MatchString(v) {
			return g.name, true
		}
	}
	return "", false
}

// China's GB 45438-2025 requires generated content to carry a label. TC260's
// implementation guide puts it in an XMP packet under this namespace, and an
// earlier scheme used a bare AIGC key holding a ServiceProvider.
// https://www.tc260.org.cn/front/postDetail.html?id=20250829165432
const (
	aigcNamespace = "http://www.tc260.org.cn/ns/AIGC/"
	aigcLegacyKey = `"ServiceProvider"`
)

// AIGCLabel reports whether raw metadata carries a TC260 label, and names the
// producer it declares. Unlike a generator name this is a legal declaration, so
// its presence alone is evidence, whether or not the producer is one untrace
// recognises.
func AIGCLabel(raw []byte) (string, bool) {
	switch {
	case bytes.Contains(raw, []byte(aigcNamespace)):
	case legacyAIGCObject(raw):
	default:
		return "", false
	}

	for _, field := range []string{"ContentProducer", "ServiceProvider"} {
		if v, ok := xmlOrJSONValue(raw, field); ok {
			return v, true
		}
	}
	return "", true
}

// The legacy scheme writes AIGC as a key introducing an object, as in
// `AIGC: {"ServiceProvider": ...}`. Both tokens merely appearing in one record
// is not that.
func legacyAIGCObject(raw []byte) bool {
	for from := 0; ; {
		i := bytes.Index(raw[from:], []byte("AIGC"))
		if i < 0 {
			return false
		}
		from += i + len("AIGC")

		rest := bytes.TrimLeft(raw[from:], " \t\r\n\"':")
		if !bytes.HasPrefix(rest, []byte("{")) {
			continue
		}
		if obj, ok := jsonObject(rest); ok && bytes.Contains(obj, []byte(aigcLegacyKey)) {
			return true
		}
	}
}

// Brace-balanced, so a key found inside belongs to this object rather than to
// something later in the record.
func jsonObject(raw []byte) ([]byte, bool) {
	depth := 0
	for i, c := range raw {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[:i+1], true
			}
		}
	}
	return nil, false
}

// The same field is written as an XML element by the XMP scheme and as a JSON
// member by the legacy one, so both shapes are tried.
func xmlOrJSONValue(raw []byte, field string) (string, bool) {
	if v, ok := between(raw, field+">", "<"); ok {
		return v, true
	}
	return jsonStringValue(raw, field)
}

// Ends at the closing quote, not at the next comma: a comma-bounded read runs
// past the string and swallows the rest of the record.
func jsonStringValue(raw []byte, field string) (string, bool) {
	i := bytes.Index(raw, []byte(`"`+field+`"`))
	if i < 0 {
		return "", false
	}
	rest := bytes.TrimLeft(raw[i+len(field)+2:], " \t\r\n:")
	if !bytes.HasPrefix(rest, []byte(`"`)) {
		return "", false
	}
	rest = rest[1:]

	j := bytes.IndexByte(rest, '"')
	if j < 0 {
		return "", false
	}
	v := strings.TrimSpace(string(rest[:j]))
	return v, v != ""
}

func between(raw []byte, open, close string) (string, bool) {
	i := bytes.Index(raw, []byte(open))
	if i < 0 {
		return "", false
	}
	rest := raw[i+len(open):]
	j := bytes.Index(rest, []byte(close))
	if j < 0 {
		j = len(rest)
	}
	v := strings.Trim(strings.TrimSpace(string(rest[:j])), `"}`)
	if v == "" {
		return "", false
	}
	return v, true
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
