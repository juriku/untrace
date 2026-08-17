package config

import "testing"

func str(s string) *string { return &s }

func cfg(overrides ...Override) *Config {
	return &Config{Overrides: overrides}
}

func TestForWithoutOverridesReturnsTheBase(t *testing.T) {
	c := &Config{
		MixedScript: str("ignore"),
		Formats:     map[string]FormatOverride{"prose": {Typographic: str("ignore")}},
	}
	got := c.For("any/path.md")

	if got.MixedScript == nil || *got.MixedScript != "ignore" {
		t.Errorf("MixedScript = %v", got.MixedScript)
	}
	if got.Formats["prose"].Typographic == nil {
		t.Error("base formats lost")
	}
}

func TestOverrideAppliesToMatchingPath(t *testing.T) {
	c := cfg(Override{Files: []string{"docs/**"}, MixedScript: str("ignore")})

	if got := c.For("docs/design/notes.md"); got.MixedScript == nil {
		t.Error("override did not apply to a matching path")
	}
	if got := c.For("internal/scan.go"); got.MixedScript != nil {
		t.Errorf("override leaked to a non-matching path: %v", got.MixedScript)
	}
}

func TestOverrideGlobForms(t *testing.T) {
	cases := map[string]struct {
		glob  string
		path  string
		match bool
	}{
		"doublestar":           {"docs/**", "docs/a/b/c.md", true},
		"doublestar at root":   {"docs/**", "docs/c.md", true},
		"extension anywhere":   {"*.md", "a/b/readme.md", true},
		"extension mismatch":   {"*.md", "a/b/readme.txt", false},
		"exact path":           {"internal/script/mixed.go", "internal/script/mixed.go", true},
		"exact path elsewhere": {"internal/script/mixed.go", "other/mixed.go", false},
		"directory name":       {"vendor/", "vendor/x.go", true},
		"unrelated":            {"docs/**", "src/main.go", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := cfg(Override{Files: []string{tc.glob}, MixedScript: str("ignore")})
			applied := c.For(tc.path).MixedScript != nil
			if applied != tc.match {
				t.Errorf("glob %q against %q: applied=%v, want %v",
					tc.glob, tc.path, applied, tc.match)
			}
		})
	}
}

func TestLaterOverrideWins(t *testing.T) {
	c := cfg(
		Override{Files: []string{"**"}, MixedScript: str("ignore")},
		Override{Files: []string{"src/**"}, MixedScript: str("clean")},
	)

	if got := c.For("src/a.go"); got.MixedScript == nil || *got.MixedScript != "clean" {
		t.Errorf("later override did not win: %v", got.MixedScript)
	}
	if got := c.For("docs/a.md"); got.MixedScript == nil || *got.MixedScript != "ignore" {
		t.Errorf("earlier override lost outside the later scope: %v", got.MixedScript)
	}
}

func TestOverrideMergesFormatsRatherThanReplacing(t *testing.T) {
	c := &Config{
		Formats: map[string]FormatOverride{
			"prose":  {Typographic: str("ignore")},
			"source": {Hidden: str("report")},
		},
		Overrides: []Override{{
			Files:   []string{"docs/**"},
			Formats: map[string]FormatOverride{"prose": {Typographic: str("clean")}},
		}},
	}
	got := c.For("docs/a.md")

	if v := got.Formats["prose"].Typographic; v == nil || *v != "clean" {
		t.Errorf("prose not overridden: %v", v)
	}
	if v := got.Formats["source"].Hidden; v == nil || *v != "report" {
		t.Errorf("unrelated base format was dropped: %v", v)
	}
}

func TestOverrideDoesNotMutateTheBaseConfig(t *testing.T) {
	c := &Config{
		Formats: map[string]FormatOverride{"prose": {Typographic: str("ignore")}},
		Overrides: []Override{{
			Files:   []string{"docs/**"},
			Formats: map[string]FormatOverride{"prose": {Typographic: str("clean")}},
		}},
	}

	c.For("docs/a.md")

	if v := c.Formats["prose"].Typographic; v == nil || *v != "ignore" {
		t.Errorf("For mutated the shared base config: %v", v)
	}
}

func TestAnyOfSeveralGlobsMatches(t *testing.T) {
	c := cfg(Override{Files: []string{"docs/**", "*.test.ts"}, MixedScript: str("ignore")})

	for _, path := range []string{"docs/a.md", "src/x.test.ts"} {
		if c.For(path).MixedScript == nil {
			t.Errorf("no glob matched %q", path)
		}
	}
	if c.For("src/x.ts").MixedScript != nil {
		t.Error("matched a path no glob covers")
	}
}
