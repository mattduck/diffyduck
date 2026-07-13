package rpconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mattduck/diffyduck/pkg/rpconfig"
)

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "revparrot.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadBasic(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
[revparrot]
include = ["**/*.py"]
exclude = ["vendor/**"]

[[rules]]
code = "bare-dict"
description = "Avoid bare dicts."
include = ["**/*.py"]

[[rules]]
code = "field-name-str"
description = "Use field name constants."
include = ["**/*.py"]
exclude = ["tests/**"]
`)

	cfg, _, err := rpconfig.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Code != "bare-dict" {
		t.Errorf("expected bare-dict, got %q", cfg.Rules[0].Code)
	}
	if cfg.Revparrot.Include[0] != "**/*.py" {
		t.Errorf("unexpected include: %v", cfg.Revparrot.Include)
	}
}

func TestLoadWalksUp(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `
[[rules]]
code = "test-rule"
description = "A rule."
`)
	subdir := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := rpconfig.Load(subdir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if filepath.Dir(path) != root {
		t.Errorf("expected config in root %s, got %s", root, path)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Code != "test-rule" {
		t.Errorf("unexpected rules: %v", cfg.Rules)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, _, err := rpconfig.Load(dir)
	if err != rpconfig.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestValidateDuplicateCode(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
[[rules]]
code = "dupe"
description = "First."

[[rules]]
code = "dupe"
description = "Second."
`)
	_, _, err := rpconfig.Load(dir)
	if err == nil {
		t.Error("expected error for duplicate rule code")
	}
}

func TestRuleIsEnabled(t *testing.T) {
	dir := t.TempDir()
	f := false
	writeConfig(t, dir, `
[[rules]]
code = "active"
description = "Enabled by default."

[[rules]]
code = "inactive"
description = "Explicitly disabled."
enabled = false
`)
	cfg, _, err := rpconfig.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = f
	active := cfg.ActiveRules()
	if len(active) != 1 || active[0].Code != "active" {
		t.Errorf("expected only 'active' rule, got %v", active)
	}
}

func TestLoadModelEffort(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
[[rules]]
code = "cheap-rule"
type = "refactor"
model = "haiku"
effort = "low"

[[rules]]
code = "plain-rule"
type = "refactor"
`)
	cfg, _, err := rpconfig.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Rules[0]; got.Model != "haiku" || got.Effort != "low" {
		t.Errorf("cheap-rule: got model=%q effort=%q, want haiku/low", got.Model, got.Effort)
	}
	if got := cfg.Rules[1]; got.Model != "" || got.Effort != "" {
		t.Errorf("plain-rule: got model=%q effort=%q, want empty", got.Model, got.Effort)
	}
}

func TestIgnoreSection(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
[[rules]]
code = "bare-dict"
description = "A rule."

[ignore]
"tests/legacy/**" = []
"src/migrations/**" = ["bare-dict"]
`)
	cfg, _, err := rpconfig.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Ignore) != 2 {
		t.Errorf("expected 2 ignore entries, got %d", len(cfg.Ignore))
	}
}
