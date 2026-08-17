package resolve

import "testing"

func TestSourceFilesHaveNoRegions(t *testing.T) {
	// Source is code throughout, so there is nothing to distinguish.
	if r := RegionsFor(FormatSource, "```\nnot markdown\n```\n"); r != nil {
		t.Errorf("source got regions: %v", r)
	}
}

func TestMarkdownFencedBlock(t *testing.T) {
	text := "prose line\n" +
		"```go\n" +
		"code line\n" +
		"```\n" +
		"more prose\n"

	r := RegionsFor(FormatProse, text)
	want := map[int]Region{
		1: RegionDefault,
		2: RegionCode, // opening fence
		3: RegionCode,
		4: RegionCode, // closing fence
		5: RegionDefault,
	}
	for line, expect := range want {
		if got := r.At(line); got != expect {
			t.Errorf("line %d = %v, want %v", line, got, expect)
		}
	}
}

func TestTildeFences(t *testing.T) {
	text := "prose\n~~~\ncode\n~~~\nprose\n"
	r := RegionsFor(FormatProse, text)
	if r.At(3) != RegionCode {
		t.Error("tilde fence not recognised")
	}
	if r.At(5) != RegionDefault {
		t.Error("tilde fence not closed")
	}
}

func TestBacktickFenceIsNotClosedByTildes(t *testing.T) {
	text := "prose\n```\ncode\n~~~\nstill code\n"
	r := RegionsFor(FormatProse, text)
	if r.At(5) != RegionCode {
		t.Error("a tilde line wrongly closed a backtick fence")
	}
}

func TestLongerFenceRequiredToClose(t *testing.T) {
	// A fence closes only on at least as many markers as opened it.
	text := "prose\n````\ncode ```\nstill code\n````\nprose\n"
	r := RegionsFor(FormatProse, text)
	if r.At(3) != RegionCode || r.At(4) != RegionCode {
		t.Error("shorter inner fence ended the block")
	}
	if r.At(6) != RegionDefault {
		t.Error("block did not close on the matching fence")
	}
}

func TestUnclosedFenceRunsToEndOfFile(t *testing.T) {
	text := "prose\n```\ncode\nmore code\n"
	r := RegionsFor(FormatProse, text)
	if r.At(4) != RegionCode {
		t.Error("unclosed fence should extend to the end")
	}
}

// Only prose and notebook ever carry regions, so this is the reachable case.
func TestInRegionKeepsWhatConfigAskedFor(t *testing.T) {
	ignore := Ignore
	prose := PolicyFor(FormatProse).Override(nil, &ignore, nil)

	code := prose.InRegion(RegionCode)
	if code.Typographic != Ignore {
		t.Errorf("code region typographic = %v, want the configured Ignore", code.Typographic)
	}
	if code.Format != FormatProse {
		t.Errorf("region policy lost the original format: %v", code.Format)
	}
}

func TestInRegionTightensKindsConfigLeftAlone(t *testing.T) {
	ignore := Ignore
	prose := PolicyFor(FormatProse).Override(nil, &ignore, nil)

	code := prose.InRegion(RegionCode)
	if code.Hidden != Clean {
		t.Errorf("code region hidden = %v, want Clean", code.Hidden)
	}
}

func TestFencedCodeIsStrictUntilTheUserSaysOtherwise(t *testing.T) {
	// Under the default policy prose is already as strict as source, so a fenced
	// block is no different from the paragraph around it.
	prose := PolicyFor(FormatProse)
	if prose.InRegion(RegionCode).Typographic != prose.Typographic {
		t.Error("default prose and code regions should be identical")
	}

	ignore := Ignore
	loosened := prose.Override(nil, &ignore, nil)
	if got := loosened.InRegion(RegionCode).Typographic; got != Ignore {
		t.Errorf("fenced typographic = %v, want the configured Ignore", got)
	}
	if got := loosened.InRegion(RegionDefault).Typographic; got != Ignore {
		t.Errorf("prose typographic = %v, want the configured Ignore", got)
	}
}

// Only the kind the user named is carried into a fence; the rest revert to the
// stricter source rules.
func TestFencedCodeKeepsSourceRulesForUnconfiguredKinds(t *testing.T) {
	ignore := Ignore
	loosened := PolicyFor(FormatProse).Override(nil, &ignore, nil)

	if got := loosened.InRegion(RegionCode).Hidden; got != Clean {
		t.Errorf("fenced hidden = %v, want Clean", got)
	}
	if got := loosened.InRegion(RegionCode).IVS; got != Clean {
		t.Errorf("fenced ivs = %v, want Clean", got)
	}
}

// An Office document's per-rune exemptions are part of the user-visible policy,
// not of the source ruleset, so they must survive a fenced region too.
func TestFencedCodeKeepsPerRuneExemptions(t *testing.T) {
	office := PolicyFor(FormatOffice)
	if len(office.PerRune) == 0 {
		t.Fatal("office policy has no per-rune exemptions to test")
	}
	if got := len(office.InRegion(RegionCode).PerRune); got != len(office.PerRune) {
		t.Errorf("fenced per-rune count = %d, want %d", got, len(office.PerRune))
	}
}

// resolvers.md records R10 as changing no outcome. This fails when that stops
// being true, so the doc cannot drift from the code.
func TestRegionNeverChangesAnOutcome(t *testing.T) {
	actions := []*Action{nil, ptrTo(Ignore), ptrTo(Report), ptrTo(Clean)}

	for _, f := range []Format{FormatProse, FormatNotebook} {
		for _, hidden := range actions {
			for _, typographic := range actions {
				for _, ivs := range actions {
					p := PolicyFor(f).Override(hidden, typographic, ivs)
					c := p.InRegion(RegionCode)
					if p.Hidden != c.Hidden || p.Typographic != c.Typographic ||
						p.IVS != c.IVS || p.Tag != c.Tag {
						t.Fatalf("%v region differs from default: %v vs %v",
							f, p, c)
					}
				}
			}
		}
	}
}

func ptrTo(a Action) *Action { return &a }

func TestNilRegionsAreDefault(t *testing.T) {
	var r Regions
	if r.At(1) != RegionDefault {
		t.Error("nil Regions should report RegionDefault")
	}
}
