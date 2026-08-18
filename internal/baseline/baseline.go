// Package baseline records findings a repository has accepted, so a large
// codebase can adopt untrace without fixing everything first.
package baseline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
)

// DefaultPath is where --write-baseline writes when --baseline is absent.
const DefaultPath = ".untrace-baseline.json"

const currentVersion = 1

// Only ID is matched against; Path and Codepoint are there to be read.
type Entry struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Codepoint string `json:"codepoint,omitempty"`
}

type file struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

type Set struct {
	entries map[string]Entry
	matched map[string]bool
}

func New() *Set {
	return &Set{entries: map[string]Entry{}, matched: map[string]bool{}}
}

// An absent file is an empty set, so --baseline can be left on before one exists.
func Load(path string) (*Set, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}

	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Version != currentVersion {
		return nil, fmt.Errorf("%s: baseline version %d, want %d", path, f.Version, currentVersion)
	}

	s := New()
	for _, e := range f.Entries {
		s.entries[e.ID] = e
	}
	return s, nil
}

func (s *Set) Has(id string) bool {
	if _, ok := s.entries[id]; !ok {
		return false
	}
	s.matched[id] = true
	return true
}

func (s *Set) Len() int { return len(s.entries) }

func (s *Set) Stale() []Entry {
	var out []Entry
	for id, e := range s.entries {
		if !s.matched[id] {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func Write(path string, entries []Entry) error {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].ID < sorted[j].ID
	})
	if sorted == nil {
		sorted = []Entry{}
	}

	data, err := json.MarshalIndent(file{Version: currentVersion, Entries: sorted}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
