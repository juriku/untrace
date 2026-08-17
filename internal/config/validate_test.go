package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadJSON(t *testing.T, body string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".untrace.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestValidateRejectsBadActions(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"top-level mixedScript",
			`{"mixedScript": "delete"}`,
			"mixedScript",
		},
		{
			"override with no files",
			`{"overrides": [{"mixedScript": "ignore"}]}`,
			"files must not be empty",
		},
		{
			"override mixedScript",
			`{"overrides": [{"files": ["*.md"], "mixedScript": "delete"}]}`,
			"overrides[0].mixedScript",
		},
		{
			"override format action",
			`{"overrides": [{"files": ["*.md"], "formats": {"prose": {"hidden": "delete"}}}]}`,
			"formats.prose.hidden",
		},
		{
			"top-level format action",
			`{"formats": {"source": {"typographic": "delete"}}}`,
			"formats.source.typographic",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadJSON(t, tc.body)
			if err == nil {
				t.Fatalf("accepted invalid config %s", tc.body)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestValidateAcceptsEveryAction(t *testing.T) {
	for _, action := range []string{"ignore", "report", "clean"} {
		t.Run(action, func(t *testing.T) {
			body := `{"mixedScript": "` + action + `",
				"formats": {"prose": {"hidden": "` + action + `", "typographic": "` + action +
				`", "ivs": "` + action + `"}},
				"overrides": [{"files": ["docs/**"], "mixedScript": "` + action + `"}]}`
			if _, err := loadJSON(t, body); err != nil {
				t.Errorf("rejected valid config: %v", err)
			}
		})
	}
}
