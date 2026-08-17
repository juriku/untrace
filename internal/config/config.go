package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/juriku/untrace/internal/gitignore"
)

// Name is JSON rather than TOML or YAML so untrace keeps a stdlib-only
// dependency tree.
const Name = ".untrace.json"

type Config struct {
	Exclude          []string                  `json:"exclude"`
	IgnoreDirs       []string                  `json:"ignoreDirs"`
	Patterns         []string                  `json:"patterns"`
	NoDefaultIgnores bool                      `json:"noDefaultIgnores"`
	NoGitignore      bool                      `json:"noGitignore"`
	Strict           bool                      `json:"strict"`
	MixedScript      *string                   `json:"mixedScript"`
	Formats          map[string]FormatOverride `json:"formats"`
	Overrides        []Override                `json:"overrides"`

	// Path is where this config was loaded from, empty if defaults are in use.
	Path string `json:"-"`
}

// Override applies to the files matching its globs. Patterns use gitignore
// syntax, so "docs/**" and "*.md" both work.
type Override struct {
	Files       []string                  `json:"files"`
	MixedScript *string                   `json:"mixedScript"`
	Formats     map[string]FormatOverride `json:"formats"`
}

// A nil field leaves the built-in policy for that marker kind untouched.
type FormatOverride struct {
	Hidden      *string `json:"hidden"`
	Typographic *string `json:"typographic"`
	IVS         *string `json:"ivs"`
}

var validActions = map[string]bool{"ignore": true, "report": true, "clean": true}

// Find walks up from dir looking for a config file, stopping at the filesystem
// root or a directory containing .git.
func Find(dir string) (*Config, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	for {
		candidate := filepath.Join(abs, Name)
		if _, err := os.Stat(candidate); err == nil {
			return Load(candidate)
		}
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return &Config{}, nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	// A config file with a BOM would be a fitting way for this tool to fail.
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	return &c, nil
}

// Resolved is the configuration in force for one file, after applying every
// override whose globs match it. Later overrides win.
type Resolved struct {
	MixedScript *string
	Formats     map[string]FormatOverride
}

func (c *Config) For(relPath string) Resolved {
	r := Resolved{MixedScript: c.MixedScript, Formats: c.Formats}

	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, o := range c.Overrides {
		if !matchAny(o.Files, parts) {
			continue
		}
		if o.MixedScript != nil {
			r.MixedScript = o.MixedScript
		}
		if len(o.Formats) > 0 {
			merged := make(map[string]FormatOverride, len(r.Formats)+len(o.Formats))
			for k, v := range r.Formats {
				merged[k] = v
			}
			for k, v := range o.Formats {
				merged[k] = v
			}
			r.Formats = merged
		}
	}
	return r
}

func matchAny(globs []string, parts []string) bool {
	for _, g := range globs {
		if gitignore.ParsePattern(g, nil).Match(parts, false) == gitignore.Exclude {
			return true
		}
	}
	return false
}

func (c *Config) validate() error {
	if c.MixedScript != nil && !validActions[*c.MixedScript] {
		return fmt.Errorf("mixedScript: %q is not one of ignore, report, clean", *c.MixedScript)
	}
	for i, o := range c.Overrides {
		if len(o.Files) == 0 {
			return fmt.Errorf("overrides[%d]: files must not be empty", i)
		}
		if o.MixedScript != nil && !validActions[*o.MixedScript] {
			return fmt.Errorf("overrides[%d].mixedScript: %q is not one of ignore, report, clean",
				i, *o.MixedScript)
		}
		for format, fo := range o.Formats {
			if err := validateFormat(format, fo); err != nil {
				return fmt.Errorf("overrides[%d].%w", i, err)
			}
		}
	}
	for format, o := range c.Formats {
		if err := validateFormat(format, o); err != nil {
			return err
		}
	}
	return nil
}

func validateFormat(format string, o FormatOverride) error {
	for field, v := range map[string]*string{
		"hidden": o.Hidden, "typographic": o.Typographic, "ivs": o.IVS,
	} {
		if v != nil && !validActions[*v] {
			return fmt.Errorf("formats.%s.%s: %q is not one of ignore, report, clean",
				format, field, *v)
		}
	}
	return nil
}
