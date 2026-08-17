package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, Name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{
	  "exclude": ["U+2014"],
	  "ignoreDirs": ["fixtures"],
	  "strict": true,
	  "formats": {"prose": {"typographic": "ignore"}}
	}`)

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Exclude) != 1 || c.Exclude[0] != "U+2014" {
		t.Errorf("exclude = %v", c.Exclude)
	}
	if !c.Strict {
		t.Error("strict not read")
	}
	if got := c.Formats["prose"].Typographic; got == nil || *got != "ignore" {
		t.Errorf("prose typographic override not read: %v", got)
	}
	if c.Path != path {
		t.Errorf("Path = %q, want %q", c.Path, path)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{"excludes": ["U+2014"]}`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected an error for a misspelled field")
	}
}

func TestLoadRejectsInvalidAction(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{"formats": {"prose": {"typographic": "delete"}}}`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error for an invalid action")
	}
	if !strings.Contains(err.Error(), "ignore, report, clean") {
		t.Errorf("error should list the valid actions, got: %v", err)
	}
}

func TestLoadToleratesBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Name)
	body := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"strict": true}`)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("a BOM should not break config loading: %v", err)
	}
	if !c.Strict {
		t.Error("strict not read from a config with a BOM")
	}
}

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `{"strict": true}`)
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	c, err := Find(deep)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Strict {
		t.Error("did not find the config in an ancestor directory")
	}
}

func TestFindStopsAtRepoRoot(t *testing.T) {
	outer := t.TempDir()
	writeConfig(t, outer, `{"strict": true}`)

	repo := filepath.Join(outer, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	c, err := Find(repo)
	if err != nil {
		t.Fatal(err)
	}
	if c.Strict {
		t.Error("search escaped the repository root")
	}
}

func TestFindReturnsEmptyWhenAbsent(t *testing.T) {
	c, err := Find(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != "" || c.Strict {
		t.Errorf("expected an empty config, got %+v", c)
	}
}
