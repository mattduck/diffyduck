package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattduck/diffyduck/pkg/rpconfig"
)

// lsFixture creates a temp tree and a matcher for ls tests. Layout:
//
//	src/pages/Home.tsx
//	src/pages/About.tsx
//	src/components/Button.tsx
//	src/util.ts
//	node_modules/pkg/Ignored.tsx
//	revparrot.toml
//
// Rules: "pages" (src/pages/**/*.tsx) and "styles" (src/**/*.tsx). Global
// include is src/**, node_modules excluded.
func lsFixture(t *testing.T) (root string, cfg *rpconfig.Config, matcher *rpconfig.Matcher) {
	t.Helper()
	root = t.TempDir()
	files := []string{
		"src/pages/Home.tsx",
		"src/pages/About.tsx",
		"src/components/Button.tsx",
		"src/util.ts",
		"node_modules/pkg/Ignored.tsx",
		"revparrot.toml",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg = &rpconfig.Config{
		Revparrot: rpconfig.GlobalConfig{
			Include: []string{"src/**"},
			Exclude: []string{"**/node_modules/**"},
		},
		Rules: []rpconfig.Rule{
			{Code: "pages", Type: "refactor", Description: "Use components.", Include: []string{"src/pages/**/*.tsx"}},
			{Code: "styles", Type: "refactor", Description: "Use tailwind.", Include: []string{"src/**/*.tsx"}},
		},
	}
	var err error
	matcher, err = cfg.NewMatcher()
	if err != nil {
		t.Fatal(err)
	}
	return root, cfg, matcher
}

func relFiles(root string, abs []string) []string {
	out := make([]string, len(abs))
	for i, a := range abs {
		out[i] = relTo(root, a)
	}
	return out
}

func TestLsCandidateFilesWholeTree(t *testing.T) {
	root, _, matcher := lsFixture(t)
	files, err := lsCandidateFiles(root, nil, matcher)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(relFiles(root, files), ",")
	// Global include is src/**, so only src files; node_modules and the
	// top-level toml are out of scope. Sorted.
	want := "src/components/Button.tsx,src/pages/About.tsx,src/pages/Home.tsx,src/util.ts"
	if got != want {
		t.Errorf("whole-tree candidates:\n got %s\nwant %s", got, want)
	}
}

func TestLsCandidateFilesPathScoped(t *testing.T) {
	root, _, matcher := lsFixture(t)

	// Directory arg: only files under it.
	dir := filepath.Join(root, "src", "pages")
	files, err := lsCandidateFiles(root, []string{dir}, matcher)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(relFiles(root, files), ","); got != "src/pages/About.tsx,src/pages/Home.tsx" {
		t.Errorf("dir-scoped candidates = %s", got)
	}

	// Explicit file arg is included directly (bypasses walk-time scope filter;
	// per-rule scoping still applies later in ruleFileGroups).
	f := filepath.Join(root, "src", "util.ts")
	files, err = lsCandidateFiles(root, []string{f}, matcher)
	if err != nil {
		t.Fatal(err)
	}
	if got := relFiles(root, files); len(got) != 1 || got[0] != "src/util.ts" {
		t.Errorf("file-scoped candidates = %v", got)
	}
}

func TestRuleFileGroups(t *testing.T) {
	root, cfg, matcher := lsFixture(t)
	files, err := lsCandidateFiles(root, nil, matcher)
	if err != nil {
		t.Fatal(err)
	}

	groups := ruleFileGroups(cfg.Rules, files, matcher, root, "")
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].rule.Code != "pages" || strings.Join(groups[0].files, ",") != "src/pages/About.tsx,src/pages/Home.tsx" {
		t.Errorf("pages group = %+v", groups[0])
	}
	if groups[1].rule.Code != "styles" || strings.Join(groups[1].files, ",") != "src/components/Button.tsx,src/pages/About.tsx,src/pages/Home.tsx" {
		t.Errorf("styles group = %+v", groups[1])
	}

	// Type filter keeps matching rules only.
	if got := ruleFileGroups(cfg.Rules, files, matcher, root, "nope"); len(got) != 0 {
		t.Errorf("type filter 'nope' should yield 0 groups, got %d", len(got))
	}

	// A rule with no in-scope files is omitted.
	extra := append([]rpconfig.Rule{}, cfg.Rules...)
	extra = append(extra, rpconfig.Rule{Code: "docs", Type: "refactor", Include: []string{"docs/**"}})
	if got := ruleFileGroups(extra, files, matcher, root, ""); len(got) != 2 {
		t.Errorf("empty rule should be omitted; got %d groups", len(got))
	}
}

func TestPrintRuleFileGroups(t *testing.T) {
	rules := []rpconfig.Rule{
		{Code: "pages", Type: "refactor", Description: "Use components."},
		{Code: "multi", Type: "fix", Description: "Line one.\nLine two."},
	}
	groups := []ruleFileGroup{
		{rule: rules[0], files: []string{"a.tsx", "b.tsx"}},
		{rule: rules[1], files: []string{"c.tsx"}},
	}
	var buf bytes.Buffer
	printRuleFileGroups(&buf, groups, defaultViolationStyles(colorNever))
	want := `Rule: refactor(pages)
Files:
  a.tsx
  b.tsx
Check: Use components.

Rule: fix(multi)
Files:
  c.tsx
Check:
  Line one.
  Line two.
`
	if buf.String() != want {
		t.Errorf("output mismatch:\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}

func TestLsJSONOutput(t *testing.T) {
	groups := []ruleFileGroup{
		{
			rule:  rpconfig.Rule{Code: "cheap", Type: "refactor", Title: "T", Description: " desc ", Model: "haiku", Effort: "low"},
			files: []string{"a.tsx"},
		},
		{
			rule:  rpconfig.Rule{Code: "plain", Type: "refactor"},
			files: nil,
		},
	}
	out := lsOutput("/root", groups)
	if out.Root != "/root" || len(out.Rules) != 2 {
		t.Fatalf("unexpected output: %+v", out)
	}
	if out.Rules[0].Model != "haiku" || out.Rules[0].Effort != "low" || out.Rules[0].Description != "desc" {
		t.Errorf("rule[0] = %+v", out.Rules[0])
	}
	// nil files must serialize as [] not null.
	if out.Rules[1].Files == nil {
		t.Errorf("nil files should be normalized to empty slice")
	}

	// model/effort must be omitted when empty; files always present.
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var round map[string]any
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	plain := round["rules"].([]any)[1].(map[string]any)
	if _, ok := plain["model"]; ok {
		t.Errorf("empty model should be omitted, got %v", plain["model"])
	}
	if _, ok := plain["files"]; !ok {
		t.Errorf("files key should always be present")
	}
}
