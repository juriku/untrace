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
