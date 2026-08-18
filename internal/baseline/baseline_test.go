package baseline

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Leaving --baseline on in a repository that has not written one yet must not
// be an error, or adopting the flag has to be coordinated with writing a file.
func TestLoadTreatsAnAbsentFileAsEmpty(t *testing.T) {
	set, err := Load(filepath.Join(t.TempDir(), "nothing.json"))
	if err != nil {
		t.Fatalf("absent file: %v", err)
	}
	if set.Len() != 0 {
		t.Errorf("got %d entries, want 0", set.Len())
	}
}

func TestLoadRejectsAnUnknownVersion(t *testing.T) {
	path := write(t, `{"version": 99, "entries": []}`)

	if _, err := Load(path); err == nil {
		t.Error("a future baseline version was accepted")
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	path := write(t, `{"version": 1, "entries": [`)

	if _, err := Load(path); err == nil {
		t.Error("a truncated baseline was accepted")
	}
}

func TestHasMatchesOnlyRecordedIDs(t *testing.T) {
	path := write(t, `{"version":1,"entries":[{"id":"abc","path":"a.go","codepoint":"U+200B"}]}`)
	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if !set.Has("abc") {
		t.Error("a recorded id did not match")
	}
	if set.Has("def") {
		t.Error("an unrecorded id matched")
	}
}

func TestStaleListsWhatNothingMatched(t *testing.T) {
	path := write(t, `{"version":1,"entries":[
		{"id":"keep","path":"b.go","codepoint":"U+200B"},
		{"id":"gone","path":"a.go","codepoint":"U+2014"}
	]}`)
	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	set.Has("keep")

	stale := set.Stale()
	if len(stale) != 1 {
		t.Fatalf("got %d stale, want 1: %+v", len(stale), stale)
	}
	if stale[0].ID != "gone" {
		t.Errorf("stale = %q, want %q", stale[0].ID, "gone")
	}
}

func TestWriteRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	entries := []Entry{
		{ID: "second", Path: "z.go", Codepoint: "U+200B"},
		{ID: "first", Path: "a.go", Codepoint: "U+2014"},
	}

	if err := Write(path, entries); err != nil {
		t.Fatal(err)
	}
	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !set.Has(e.ID) {
			t.Errorf("%q did not survive the round trip", e.ID)
		}
	}
}

// Ordered by path so a regenerated baseline produces a reviewable diff rather
// than a reshuffle of every line.
func TestWriteIsOrdered(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	err := Write(path, []Entry{
		{ID: "b", Path: "z.go"},
		{ID: "a", Path: "a.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	first, second := indexOf(string(data), `"a.go"`), indexOf(string(data), `"z.go"`)
	if first > second {
		t.Errorf("entries are not ordered by path:\n%s", data)
	}
}

func TestWriteOnNoFindingsIsStillLoadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := Write(path, nil); err != nil {
		t.Fatal(err)
	}

	set, err := Load(path)
	if err != nil {
		t.Fatalf("an empty baseline did not load: %v", err)
	}
	if set.Len() != 0 {
		t.Errorf("got %d entries, want 0", set.Len())
	}
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
