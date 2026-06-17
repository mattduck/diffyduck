// Package config handles loading and validating revparrot.toml.
package rpconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const configFilename = "revparrot.toml"

// Config is the parsed contents of a revparrot.toml file.
type Config struct {
	Revparrot GlobalConfig        `toml:"revparrot"`
	Rules     []Rule              `toml:"rules"`
	Ignore    map[string][]string `toml:"ignore"`
}

// GlobalConfig holds top-level tool settings.
type GlobalConfig struct {
	Include []string `toml:"include"`
	Exclude []string `toml:"exclude"`
}

// Rule defines a single review rule.
type Rule struct {
	Code        string   `toml:"code"`
	Description string   `toml:"description"`
	Include     []string `toml:"include"`
	Exclude     []string `toml:"exclude"`
	Enabled     *bool    `toml:"enabled"`
}

// IsEnabled returns true if the rule is enabled (default true).
func (r Rule) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

// Load finds and parses the nearest revparrot.toml, walking up from dir.
// Returns ErrNotFound if no config file exists in dir or any parent.
func Load(dir string) (*Config, string, error) {
	path, err := findConfig(dir)
	if err != nil {
		return nil, "", err
	}
	cfg, err := parse(path)
	if err != nil {
		return nil, "", err
	}
	return cfg, path, nil
}

// LoadFromPath parses the config at the given explicit path.
func LoadFromPath(path string) (*Config, error) {
	return parse(path)
}

// ErrNotFound is returned when no revparrot.toml can be found.
var ErrNotFound = fmt.Errorf("no %s found", configFilename)

func findConfig(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, configFilename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

func parse(path string) (*Config, error) {
	var cfg Config
	// Set defaults.
	cfg.Revparrot.Include = []string{"**/*"}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return &cfg, nil
}

func validate(cfg *Config) error {
	seen := make(map[string]bool)
	for _, r := range cfg.Rules {
		if r.Code == "" {
			return fmt.Errorf("rule missing required field: code")
		}
		if seen[r.Code] {
			return fmt.Errorf("duplicate rule code: %q", r.Code)
		}
		seen[r.Code] = true
	}
	return nil
}

// RuleByCode returns the rule with the given code, or false if not found.
func (c *Config) RuleByCode(code string) (Rule, bool) {
	for _, r := range c.Rules {
		if r.Code == code {
			return r, true
		}
	}
	return Rule{}, false
}

// ActiveRules returns all enabled rules.
func (c *Config) ActiveRules() []Rule {
	var out []Rule
	for _, r := range c.Rules {
		if r.IsEnabled() {
			out = append(out, r)
		}
	}
	return out
}
