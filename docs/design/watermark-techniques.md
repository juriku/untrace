# AI watermarking techniques and what untrace can reach

Researched August 2026. Sources are linked at the bottom.

The techniques fall into four categories. Two are within reach of a character and
metadata tool. Two are not, and no amount of engineering will change that, so the
README must say so plainly.

## A. Character-level markers: in scope, implemented

Invisible or confusable codepoints inserted into the text itself: zero-width
characters, variation selectors, bidi controls, Unicode tag characters,
homoglyphs. Anyone can add these, and they are what most "AI watermark remover"
tools actually target.

untrace detects and removes these today. See `resolvers.md`.

Worth noting: no major lab publicly uses this method for text. It shows up in
third-party tooling, prompt-injection payloads, and ad-hoc tracking.

## B. Provenance metadata: in scope, Phase 5

**C2PA / Content Credentials.** A cryptographically signed manifest attached to a
file's metadata, from the Coalition for Content Provenance and Authenticity, the
same standard used by camera manufacturers and Adobe.

**Anthropic deploys this as of August 2026.** When Claude produces a `.png`,
`.jpg` or `.svg`, it attaches a signed content credential. Anthropic is explicit
that this is not a watermark: "Nothing in the file changes, it is not embedded or
hidden."

That makes it trivially strippable, and it already dies by accident: re-saving
through almost any image tool, converting format, screenshotting, or uploading to
most platforms removes it.

Also in this category: EXIF `Software` tags, XMP, PDF `/Producer`, OOXML
`docProps`.

untrace should **detect and report** these as the strongest available signal of
AI origin, and strip them on request. Detection is arguably the more valuable
half: a signed C2PA manifest is positive evidence, where an em dash is a guess.

## C. Statistical token watermarks: out of reach, permanently

**SynthID-Text**, developed by Google DeepMind and published in Nature in 2024.
Anthropic deployed it across every Claude model released on or after
**2 August 2026**, covering the API, claude.ai, Claude Code, Cowork, Tag, and
Claude via AWS, Google Cloud and Microsoft Foundry.

The mechanism biases the model's choice among statistically near-equivalent next
tokens, so the output distribution carries a detectable signature. Anthropic's
own description: the words "are still random, but now, one can check the sequence
of words and see if it's consistent with the choices Claude would make if it was
using the key."

Critically:

- **No characters are added.** No hidden codepoints, no extra tokens.
- **No metadata is added.** Nothing in the file to strip.
- The signal lives in *which words were chosen*, distributed across the whole
  passage.

**untrace cannot detect this, and cannot remove it.** Neither can any other
character or metadata tool. Detection requires the cryptographic key and a
statistical test over the token sequence; Anthropic says it will ship a detection
API. The only removal is heavy paraphrasing, which is a rewriting problem, not a
scrubbing one.

Documented weaknesses, none of which untrace can exploit: it is sparse on short
passages, sparse on factual text where few word choices exist, weak where a human
did most of the writing, and absent from exact outputs like code where a
substitution would break correctness.

One consequence worth stating in the README: any tool claiming to remove
statistical text watermarks is making an unverifiable claim, because the scheme
is unpublished and there is no way for a user to confirm removal.

## D. Pixel and audio watermarks: out of reach

SynthID for images and audio embeds the signal in the pixel or waveform domain.
Stripping metadata does nothing to it. Out of scope for the same reason as C.

## What this means for untrace

1. **Raise C2PA to the top of Phase 5.** It is the only technique in this
   research that a major lab actually deploys *and* that untrace can act on.
   Detecting a signed Claude manifest is a real, verifiable answer to "was AI
   involved", which no amount of character scanning provides.

2. **The README must carry an explicit limitations section.** Users will assume
   a tool called untrace removes Claude's watermark. It does not, and cannot.
   Saying so is both honest and a differentiator against tools that imply
   otherwise.

3. **A detected C2PA manifest should raise confidence on every other finding in
   the same file** (resolver R3). If a document is signed as AI-produced, an
   ambiguous em dash becomes less ambiguous.

4. **Detection is the durable product, removal is the commodity.** Character
   stripping is easy and already crowded. Accurate, explainable detection across
   all four categories, including honestly reporting "this carries a statistical
   watermark we cannot read", is the harder and more useful thing.

## Sources

- [Anthropic, Claude text watermark](https://www.anthropic.com/news/claude-text-watermark)
- [explainx, what Claude watermarks detect and miss](https://explainx.ai/blog/anthropic-claude-invisible-watermarks-c2pa-august-2026)
- [Axios, Anthropic's text watermarks](https://www.axios.com/2026/08/12/anthropic-claude-watermarks-ai-detection)
