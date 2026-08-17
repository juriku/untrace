package gitignore

import "testing"

// Guards the vendored copy: these are the gitignore semantics untrace relies on.
func TestMatcherSemantics(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		domain   []string
		path     []string
		isDir    bool
		want     bool
	}{
		{"plain name", []string{"secret.txt"}, nil, []string{"secret.txt"}, false, true},
		{"plain name nested", []string{"secret.txt"}, nil, []string{"sub", "secret.txt"}, false, true},
		{"non match", []string{"secret.txt"}, nil, []string{"keep.txt"}, false, false},

		{"dir only matches dir", []string{"sub/"}, nil, []string{"sub"}, true, true},
		{"dir only skips file", []string{"sub/"}, nil, []string{"sub"}, false, false},

		{"anchored at root", []string{"/root.txt"}, nil, []string{"root.txt"}, false, true},
		{"anchored not nested", []string{"/root.txt"}, nil, []string{"sub", "root.txt"}, false, false},

		{"glob", []string{"*.tmp"}, nil, []string{"a.tmp"}, false, true},
		{"glob does not cross slash", []string{"*.tmp"}, nil, []string{"sub", "a.tmp"}, false, true},

		{"doublestar", []string{"**/build"}, nil, []string{"a", "b", "build"}, true, true},

		{"negation wins when last", []string{"*.log", "!keep.log"}, nil, []string{"keep.log"}, false, false},
		{"negation does not affect others", []string{"*.log", "!keep.log"}, nil, []string{"drop.log"}, false, true},

		{"domain scopes pattern", []string{"test"}, []string{"proj", "test2"}, []string{"proj", "test"}, false, false},
		{"domain matches within", []string{"test"}, []string{"proj", "test2"}, []string{"proj", "test2", "test"}, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := make([]Pattern, 0, len(tc.patterns))
			for _, p := range tc.patterns {
				ps = append(ps, ParsePattern(p, tc.domain))
			}
			if got := NewMatcher(ps).Match(tc.path, tc.isDir); got != tc.want {
				t.Errorf("Match(%v, isDir=%v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
			}
		})
	}
}

func TestCommentsAndBlanksAreInert(t *testing.T) {
	for _, p := range []string{"", "   ", "# a comment"} {
		if ParsePattern(p, nil).Match([]string{"anything"}, false) != NoMatch {
			t.Errorf("pattern %q should not match anything", p)
		}
	}
}
