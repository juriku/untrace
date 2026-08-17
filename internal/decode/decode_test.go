package decode

import "testing"

func only(t *testing.T, ps []Payload, scheme Scheme) Payload {
	t.Helper()
	var found []Payload
	for _, p := range ps {
		if p.Scheme == scheme {
			found = append(found, p)
		}
	}
	if len(found) != 1 {
		t.Fatalf("got %d %s payloads, want 1 (all: %+v)", len(found), scheme, ps)
	}
	return found[0]
}

func TestDecodeTagPayload(t *testing.T) {
	text := "Hello" + EncodeTag("user-4471") + " world"
	p := only(t, Payloads([]rune(text)), SchemeTag)
	if !p.Printable || p.Text != "user-4471" {
		t.Errorf("got %q printable=%v, want %q", p.Text, p.Printable, "user-4471")
	}
}

func TestDecodeVariationPayload(t *testing.T) {
	text := string(rune(0x1F600)) + EncodeVariation([]byte("secret"))
	p := only(t, Payloads([]rune(text)), SchemeVariation)
	if !p.Printable || p.Text != "secret" {
		t.Errorf("got %q printable=%v, want %q", p.Text, p.Printable, "secret")
	}
}

func TestDecodeZeroWidthPayload(t *testing.T) {
	text := "visible" + EncodeZeroWidth([]byte("AB")) + "tail"
	p := only(t, Payloads([]rune(text)), SchemeZeroWidth)
	if !p.Printable || p.Text != "AB" {
		t.Errorf("got %q printable=%v, want %q", p.Text, p.Printable, "AB")
	}
}

func TestFlagSequenceIsNotAPayload(t *testing.T) {
	// The Scotland flag: waving black flag, tag letters "gbsct", cancel tag.
	text := string(rune(0x1F3F4)) + EncodeTag("gbsct") + string(rune(0xE007F))
	for _, p := range Payloads([]rune(text)) {
		if p.Scheme == SchemeTag {
			t.Errorf("subdivision flag reported as a hidden payload: %+v", p)
		}
	}
}

func TestSingleVariationSelectorIsNotAPayload(t *testing.T) {
	text := string(rune(0x2764)) + string(rune(0xFE0F))
	if ps := Payloads([]rune(text)); len(ps) != 0 {
		t.Errorf("ordinary emoji presentation reported as a payload: %+v", ps)
	}
}

func TestNonPrintableDecodeIsReportedWithoutText(t *testing.T) {
	text := EncodeVariation([]byte{0x01, 0x02, 0x03, 0x04})
	p := only(t, Payloads([]rune(text)), SchemeVariation)
	if p.Printable || p.Text != "" {
		t.Errorf("control bytes should not produce text, got %q", p.Text)
	}
	if p.Runes != 4 {
		t.Errorf("Runes = %d, want 4", p.Runes)
	}
}

func TestCleanTextHasNoPayloads(t *testing.T) {
	if ps := Payloads([]rune("just some ordinary text")); len(ps) != 0 {
		t.Errorf("clean text produced payloads: %+v", ps)
	}
}

func TestOffsetsPointAtTheRun(t *testing.T) {
	prefix := "Hello"
	text := prefix + EncodeTag("hi") + "tail"
	p := only(t, Payloads([]rune(text)), SchemeTag)
	if p.Start != len([]rune(prefix)) {
		t.Errorf("Start = %d, want %d", p.Start, len([]rune(prefix)))
	}
	if p.End != p.Start+2 {
		t.Errorf("End = %d, want %d", p.End, p.Start+2)
	}
}

func TestRoundTripAllSchemes(t *testing.T) {
	// Every scheme needs at least two carrier characters to be distinguishable
	// from ordinary use, so single-character payloads are covered separately.
	payloads := []string{"ab", "hello world", "tracked-by:acct-99213", "07", "~!@#$%^&*()"}

	for _, want := range payloads {
		t.Run(want, func(t *testing.T) {
			for _, tc := range []struct {
				scheme  Scheme
				encoded string
			}{
				{SchemeTag, EncodeTag(want)},
				{SchemeVariation, EncodeVariation([]byte(want))},
				{SchemeZeroWidth, EncodeZeroWidth([]byte(want))},
			} {
				got := only(t, Payloads([]rune("pad"+tc.encoded+"pad")), tc.scheme)
				if got.Text != want {
					t.Errorf("%s: decoded %q, want %q", tc.scheme, got.Text, want)
				}
			}
		})
	}
}

func TestEncodeTagDropsNonASCII(t *testing.T) {
	// The Tags block only mirrors printable ASCII, so anything else is dropped
	// rather than silently mis-encoded.
	if got := EncodeTag("a\u00e9b"); len([]rune(got)) != 2 {
		t.Errorf("got %d tag runes, want 2", len([]rune(got)))
	}
}

func TestSingleTagCharacterIsAPayload(t *testing.T) {
	p := only(t, Payloads([]rune("text"+EncodeTag("x"))), SchemeTag)
	if p.Text != "x" {
		t.Errorf("decoded %q, want %q", p.Text, "x")
	}
}
