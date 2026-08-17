package markers

import "fmt"

type Kind int

const (
	Hidden Kind = iota
	Typographic
	IdeographicVS
	Tag
)

func (k Kind) String() string {
	switch k {
	case Hidden:
		return "hidden"
	case Typographic:
		return "typographic"
	case IdeographicVS:
		return "ideographic-vs"
	case Tag:
		return "tag"
	}
	return "unknown"
}

const (
	ivsFirst = 0xE0100
	ivsLast  = 0xE01EF
)

var ivsNames = buildIVSNames()

func buildIVSNames() []string {
	out := make([]string, ivsLast-ivsFirst+1)
	for i := range out {
		out[i] = fmt.Sprintf("Ideographic Variation Selector-%d", 17+i)
	}
	return out
}

type Marker struct {
	Rune rune
	Name string
	Kind Kind
	// Empty Replacement means delete the rune; see CanClean to distinguish it
	// from "no rule defined".
	Replacement string
	CanClean    bool
}

type Options struct {
	Typographic bool
	IVS         bool
	Excluded    map[rune]bool
}

// Hidden is checked before typographic: characters such as U+00A0 appear in both
// tables and must resolve as hidden.
func Lookup(r rune, opts Options) (Marker, bool) {
	if opts.Excluded[r] || neverDetected[r] {
		return Marker{}, false
	}

	if name, ok := hiddenNames[r]; ok {
		return Marker{
			Rune:        r,
			Name:        name,
			Kind:        Hidden,
			Replacement: replacements[r],
			CanClean:    !neverCleaned[r],
		}, true
	}

	if e, ok := extraHidden[r]; ok {
		return Marker{
			Rune:        r,
			Name:        e.name,
			Kind:        Hidden,
			Replacement: e.replace,
			CanClean:    !e.noClean,
		}, true
	}

	if IsTag(r) {
		return Marker{
			Rune:     r,
			Name:     tagName(r),
			Kind:     Tag,
			CanClean: true,
		}, true
	}

	if opts.IVS && r >= ivsFirst && r <= ivsLast {
		return Marker{
			Rune:     r,
			Name:     ivsNames[r-ivsFirst],
			Kind:     IdeographicVS,
			CanClean: true,
		}, true
	}

	if opts.Typographic {
		if name, ok := typographicNames[r]; ok {
			repl, hasRepl := replacements[r]
			return Marker{
				Rune:        r,
				Name:        name,
				Kind:        Typographic,
				Replacement: repl,
				CanClean:    hasRepl && !neverCleaned[r],
			}, true
		}
	}

	return Marker{}, false
}

func IsWordCommon(r rune) bool { return wordCommon[r] }

func WordCommonRunes() map[rune]bool {
	out := make(map[rune]bool, len(wordCommon))
	for r := range wordCommon {
		out[r] = true
	}
	return out
}
