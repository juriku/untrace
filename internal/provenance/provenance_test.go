package provenance

import "testing"

func TestGeneratorMatches(t *testing.T) {
	cases := map[string]bool{
		"Claude":                  true,
		"claude 4.5":              true,
		"Adobe Firefly 3":         true,
		"Made with Midjourney v6": true,
		"OpenAI DALL-E 3":         true,
		"ComfyUI":                 true,

		"Adobe Photoshop 25.0": false,
		"Canon EOS R5":         false,
		"GIMP 2.10":            false,
		"":                     false,
		"Microsoft Word":       false,

		"DeepSeek-V3":   true,
		"Mistral Large": true,
		"Perplexity AI": true,
		"Qwen2.5-Max":   true,
		"Kimi K2.6":     true,
		"GLM-5.2":       true,
		"GPT-6":         true,

		// A name is a whole word, not a substring.
		"Editor de imagenes 2.0": false,
		"Grokking Algorithms":    false,
		"Soraya Publishing":      false,
		"LlamaSoft Reporting":    false,
		"Copilots Design Suite":  false,
		"Firefly LED Signage":    false,

		"Doubao 1.5":      true,
		"Jimeng AI":       true,
		"Kling 2.0":       true,
		"Keling":          true,
		"Tencent Yuanbao": true,
		"ERNIE Bot":       true,
		"Wenxin Yiyan":    true,
		"LiblibAI":        true,
		"Hailuo AI":       true,
		"MiniMax abab":    true,
		"Seedream 4.0":    true,
		"Seedance 1.0":    true,
		"Hunyuan-DiT":     true,
		"Zhipu AI":        true,
		"Tongyi Wanxiang": true,
		"Nano Banana":     true,

		// Ordinary words elsewhere, so these need a version beside them.
		"Wan 2.5":               true,
		"Vidu Q2":               true,
		"de wan is leeg":        false,
		"quod vidu in foro":     false,
		"Klingon Language Inst": false,

		// A PDF's Producer names the converter, not the author.
		"Skia/PDF m140":                    false,
		"jsPDF 2.5.1":                      false,
		"Microsoft Word for Microsoft 365": false,

		"Achraf Hakimi": false,

		// Ordinary words and names that a bare substring match reads as a tool.
		"Ernie Ball":                false,
		"Bernie Sanders":            false,
		"Taiwan 2024 Report":        false,
		"Rowan 3 Press":             false,
		"Swan 1 Photography":        false,
		"minimax alpha-beta search": false,
		"Vidua paradisaea":          false,

		// The vendors those words collide with are still matched.
		"ERNIE Bot 4.0": true,

		// A bare vendor name in a tool field is a real signal, so these stay
		// matched here even though they are also surnames. Which fields reach
		// Generator at all is the caller's decision; see TestFreeTextFieldsDo
		// NotDeclareAI in cmd/untrace.
		"Sora": true, "Kling": true,
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			if _, got := Generator(value); got != want {
				t.Errorf("Generator(%q) = %v, want %v", value, got, want)
			}
		})
	}
}

func TestAssessRaisesConfidenceOnlyForNamedGenerators(t *testing.T) {
	// A camera-signed photograph carries a C2PA manifest and is not AI, so a
	// credential alone must not raise confidence.
	credentialOnly := []Signal{{Source: "c2pa", Detail: "manifest present", AI: false}}
	if got := Assess(credentialOnly); got != Possible {
		t.Errorf("credential alone gave %q, want %q", got, Possible)
	}

	named := []Signal{
		{Source: "c2pa", Detail: "manifest present", AI: false},
		{Source: "exif Software", Detail: "claude", AI: true},
	}
	if got := Assess(named); got != Likely {
		t.Errorf("named generator gave %q, want %q", got, Likely)
	}

	if got := Assess(nil); got != Possible {
		t.Errorf("no signals gave %q, want %q", got, Possible)
	}
}

func TestGeneratorIgnoresPunctuation(t *testing.T) {
	// Vendors punctuate product names inconsistently, and the middle dot in one
	// spelling is itself a typographic marker, so matching must not depend on it.
	middleDot := "DALL" + string(rune(0x00B7)) + "E 3"
	for _, spelling := range []string{"DALL-E 3", middleDot, "dalle3", "OpenAI DALL E"} {
		if _, ok := Generator(spelling); !ok {
			t.Errorf("Generator(%q) did not match", spelling)
		}
	}
}

func TestGeneratorIgnoresSpacingInMultiWordNames(t *testing.T) {
	for _, spelling := range []string{"Stable Diffusion XL", "stable-diffusion", "StableDiffusion"} {
		if _, ok := Generator(spelling); !ok {
			t.Errorf("Generator(%q) did not match", spelling)
		}
	}
}

// A CBOR length byte sits flush against the name, so the manifest scan cannot
// require the word boundary a metadata value can.
func TestGeneratorInManifestMatchesAcrossACBORLengthByte(t *testing.T) {
	const run = "_generatorqClaude 3.5 Sonnet"

	if _, ok := GeneratorInManifest(run); !ok {
		t.Errorf("GeneratorInManifest(%q) did not match", run)
	}
	if _, ok := Generator(run); ok {
		t.Errorf("Generator(%q) matched, want the anchored form to decline", run)
	}
}

func TestGeneratorInManifestStillNeedsTheName(t *testing.T) {
	for _, run := range []string{"Achraf Hakimi", "Canon EOS R5", ""} {
		if _, ok := GeneratorInManifest(run); ok {
			t.Errorf("GeneratorInManifest(%q) matched", run)
		}
	}
}

const xmpAIGC = `<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:AIGC="http://www.tc260.org.cn/ns/AIGC/1.0/">
    <rdf:Description>
      <AIGC:Label>1</AIGC:Label>
      <AIGC:ContentProducer>Doubao</AIGC:ContentProducer>
      <AIGC:ProduceID>7f3a</AIGC:ProduceID>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`

func TestAIGCLabelReadsTheXMPScheme(t *testing.T) {
	producer, ok := AIGCLabel([]byte(xmpAIGC))

	if !ok {
		t.Fatal("a TC260 XMP packet was not recognised")
	}
	if producer != "Doubao" {
		t.Errorf("producer = %q, want %q", producer, "Doubao")
	}
}

func TestAIGCLabelReadsTheLegacyScheme(t *testing.T) {
	raw := []byte(`AIGC: {"ServiceProvider": "Jimeng", "Time": "2026-08-17", "ContentID": "a1"}`)

	producer, ok := AIGCLabel(raw)

	if !ok {
		t.Fatal("the legacy AIGC scheme was not recognised")
	}
	if producer != "Jimeng" {
		t.Errorf("producer = %q, want %q", producer, "Jimeng")
	}
}

// A comma-bounded read runs past the closing quote and swallows the rest of
// the record.
func TestAIGCProducerStopsAtTheClosingQuote(t *testing.T) {
	raw := []byte(`AIGC: {"ServiceProvider": "doubao", "Time": "2026-08-18"} // notes on AIGC`)

	producer, ok := AIGCLabel(raw)

	if !ok {
		t.Fatal("a real legacy label was not recognised")
	}
	if producer != "doubao" {
		t.Errorf("producer = %q, want %q", producer, "doubao")
	}
}

// The declaration is the evidence. A packet naming no producer is still a
// statement that the file was generated.
func TestAIGCLabelWithoutAProducerStillCounts(t *testing.T) {
	raw := []byte(`<rdf:RDF xmlns:AIGC="http://www.tc260.org.cn/ns/AIGC/1.0/"><AIGC:Label>1</AIGC:Label></rdf:RDF>`)

	producer, ok := AIGCLabel(raw)

	if !ok {
		t.Error("a label with no producer was ignored")
	}
	if producer != "" {
		t.Errorf("producer = %q, want empty", producer)
	}
}

func TestAIGCLabelIgnoresOrdinaryMetadata(t *testing.T) {
	cases := map[string][]byte{
		"adobe xmp":       []byte(`<x:xmpmeta><xmp:CreatorTool>Adobe Photoshop</xmp:CreatorTool></x:xmpmeta>`),
		"empty":           nil,
		"the word alone":  []byte("this document discusses AIGC labelling"),
		"a provider only": []byte(`{"ServiceProvider": "Acme Hosting"}`),

		// Both tokens present, but ServiceProvider is a sibling key rather than
		// a member of an object the AIGC key introduces.
		"both tokens, unrelated": []byte(
			`{"pipeline":"diagram","ServiceProvider":"acme-cdn","topic":"AIGC compliance overview"}`),
		"aigc key, no provider": []byte(`{"AIGC":{"Version":"1.0"}}`),
		"aigc not a key":        []byte(`{"notes":"AIGC","ServiceProvider":"acme-cdn"}`),
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := AIGCLabel(raw); ok {
				t.Errorf("AIGCLabel(%q) matched", raw)
			}
		})
	}
}
