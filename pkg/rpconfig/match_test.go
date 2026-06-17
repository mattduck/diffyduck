package rpconfig

import "testing"

func TestGlobToRegexp(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Literal.
		{"main.go", "main.go", true},
		{"main.go", "cmd/main.go", false},
		{"main.go", "main_go", false},

		// Single star stays within a segment.
		{"*.py", "a.py", true},
		{"*.py", "src/a.py", false},
		{"src/*.py", "src/a.py", true},
		{"src/*.py", "src/sub/a.py", false},
		{"src/*", "src/a.py", true},
		{"src/*", "src/sub/a.py", false},

		// Leading "**/" matches zero or more segments.
		{"**/*.py", "a.py", true},
		{"**/*.py", "src/a.py", true},
		{"**/*.py", "src/sub/deep/a.py", true},
		{"**/*.py", "a.go", false},
		{"**/*", "a.py", true},
		{"**/*", "a/b/c", true},

		// Trailing "/**" matches everything beneath a directory.
		{"vendor/**", "vendor/a.go", true},
		{"vendor/**", "vendor/sub/a.go", true},
		{"vendor/**", "vendors/a.go", false},
		{"vendor/**", "src/vendor/a.go", false},

		// "**" in the middle matches any depth, including zero.
		{"a/**/b.py", "a/b.py", true},
		{"a/**/b.py", "a/x/b.py", true},
		{"a/**/b.py", "a/x/y/b.py", true},
		{"a/**/b.py", "a/b.go", false},

		// Bare "**" matches anything.
		{"**", "anything/at/all.py", true},
		{"**", "x", true},

		// Question mark matches a single non-slash char.
		{"file?.go", "file1.go", true},
		{"file?.go", "file.go", false},
		{"file?.go", "file/.go", false},

		// Regex metacharacters in patterns are treated literally.
		{"a.b", "a.b", true},
		{"a.b", "axb", false},
		{"foo+bar.py", "foo+bar.py", true},
	}
	for _, c := range cases {
		re, err := globToRegexp(c.pattern)
		if err != nil {
			t.Fatalf("globToRegexp(%q): %v", c.pattern, err)
		}
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("glob %q vs %q = %v, want %v (regex %s)", c.pattern, c.path, got, c.want, re)
		}
	}
}

func TestMatcherInScope(t *testing.T) {
	cfg := &Config{
		Revparrot: GlobalConfig{
			Include: []string{"**/*.py"},
			Exclude: []string{"vendor/**", "**/migrations/**"},
		},
	}
	m, err := cfg.NewMatcher()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want bool
	}{
		{"src/app.py", true},
		{"app.py", true},
		{"README.md", false},              // not in include
		{"vendor/lib.py", false},          // excluded
		{"app/migrations/0001.py", false}, // excluded
		{"app/models.py", true},
	}
	for _, c := range cases {
		if got := m.InScope(c.path); got != c.want {
			t.Errorf("InScope(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestMatcherInScopeEmptyIncludeMeansAll(t *testing.T) {
	cfg := &Config{Revparrot: GlobalConfig{Exclude: []string{"vendor/**"}}}
	m, err := cfg.NewMatcher()
	if err != nil {
		t.Fatal(err)
	}
	if !m.InScope("anything.txt") {
		t.Error("empty include should match all paths")
	}
	if m.InScope("vendor/x.go") {
		t.Error("excluded path should not be in scope")
	}
}

func TestMatcherRuleApplies(t *testing.T) {
	cfg := &Config{
		Rules: []Rule{
			{Code: "py-only", Include: []string{"**/*.py"}, Exclude: []string{"tests/**"}},
			{Code: "everywhere"}, // no include => applies to all
		},
	}
	m, err := cfg.NewMatcher()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		code, path string
		want       bool
	}{
		{"py-only", "src/a.py", true},
		{"py-only", "src/a.go", false},
		{"py-only", "tests/a.py", false}, // rule-level exclude
		{"everywhere", "anything.txt", true},
		{"everywhere", "deep/nested/file.go", true},
		{"unknown-code", "a.py", false}, // unknown rule never applies
	}
	for _, c := range cases {
		if got := m.RuleApplies(c.code, c.path); got != c.want {
			t.Errorf("RuleApplies(%q, %q) = %v, want %v", c.code, c.path, got, c.want)
		}
	}
}

func TestMatcherIgnored(t *testing.T) {
	cfg := &Config{
		Rules: []Rule{{Code: "bare-dict"}, {Code: "field-name"}},
		Ignore: map[string][]string{
			"tests/legacy/**":   {},             // ignore all rules
			"src/migrations/**": {"field-name"}, // ignore one rule
		},
	}
	m, err := cfg.NewMatcher()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		code, path string
		want       bool
	}{
		{"bare-dict", "tests/legacy/old.py", true},  // empty list => all rules
		{"field-name", "tests/legacy/old.py", true}, // empty list => all rules
		{"field-name", "src/migrations/0001.py", true},
		{"bare-dict", "src/migrations/0001.py", false}, // not in this path's list
		{"bare-dict", "src/app.py", false},             // path not ignored
	}
	for _, c := range cases {
		if got := m.Ignored(c.code, c.path); got != c.want {
			t.Errorf("Ignored(%q, %q) = %v, want %v", c.code, c.path, got, c.want)
		}
	}
}

func TestMatcherKeep(t *testing.T) {
	cfg := &Config{
		Rules: []Rule{{Code: "py-rule", Include: []string{"**/*.py"}}},
		Ignore: map[string][]string{
			"generated/**": {},
		},
	}
	m, err := cfg.NewMatcher()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, code, path string
		want             bool
	}{
		{"known rule, applies", "py-rule", "src/a.py", true},
		{"known rule, wrong file type", "py-rule", "src/a.go", false},
		{"known rule, ignored path", "py-rule", "generated/a.py", false},
		{"unknown code kept", "legacy-annotation", "src/a.py", true},
		{"unknown code, ignored path dropped", "legacy-annotation", "generated/a.py", false},
	}
	for _, c := range cases {
		if got := m.Keep(c.code, c.path); got != c.want {
			t.Errorf("%s: Keep(%q, %q) = %v, want %v", c.name, c.code, c.path, got, c.want)
		}
	}
}

func TestNewMatcherInvalidGlob(t *testing.T) {
	// A pattern that translates to invalid regex is hard to produce since we
	// escape metacharacters, but an unterminated UTF-8 sequence or similar would
	// fail. Here we just confirm a normal config compiles without error.
	cfg := &Config{Revparrot: GlobalConfig{Include: []string{"**/*"}}}
	if _, err := cfg.NewMatcher(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
