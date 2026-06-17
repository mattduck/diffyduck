package rpconfig

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// globToRegexp translates a glob pattern into an anchored regular expression.
//
// Patterns operate on slash-separated paths. Supported syntax:
//
//	?      matches any single character except '/'
//	*      matches any run of characters except '/' (within one path segment)
//	**     matches any number of characters including '/' (any depth)
//	**/    matches zero or more complete leading path segments
//	c      any other character matches itself literally
//
// All regex metacharacters are escaped, so patterns are pure globs — character
// classes ([...]) and brace expansion ({a,b}) are treated literally and are not
// supported.
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteByte('^')
	runes := []rune(pattern)
	i := 0
	for i < len(runes) {
		c := runes[i]
		switch c {
		case '*':
			if i+1 < len(runes) && runes[i+1] == '*' {
				// "**": consume both stars.
				i += 2
				if i < len(runes) && runes[i] == '/' {
					// "**/" matches zero or more complete leading segments,
					// so "**/*.py" matches both "a.py" and "x/y/a.py".
					b.WriteString("(?:[^/]*/)*")
					i++
				} else {
					// "**" at end, or not followed by '/': match anything.
					b.WriteString(".*")
				}
			} else {
				// Single "*": match within a single path segment.
				b.WriteString("[^/]*")
				i++
			}
		case '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}
	b.WriteByte('$')
	return regexp.Compile(b.String())
}

// globSet is a compiled set of glob patterns. A path matches the set if it
// matches any one of the patterns.
type globSet struct {
	res []*regexp.Regexp
}

func compileGlobs(patterns []string) (*globSet, error) {
	gs := &globSet{}
	for _, p := range patterns {
		re, err := globToRegexp(filepath.ToSlash(p))
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", p, err)
		}
		gs.res = append(gs.res, re)
	}
	return gs, nil
}

func (g *globSet) empty() bool { return g == nil || len(g.res) == 0 }

func (g *globSet) match(path string) bool {
	if g == nil {
		return false
	}
	for _, re := range g.res {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// Matcher applies a Config's include/exclude/ignore globs to file paths. Paths
// passed to its methods are interpreted relative to the config root (the
// directory containing revparrot.toml) and may use either path separator.
type Matcher struct {
	include *globSet
	exclude *globSet
	rules   map[string]ruleMatcher
	ignore  []ignoreMatcher
}

type ruleMatcher struct {
	include *globSet
	exclude *globSet
}

type ignoreMatcher struct {
	glob  *globSet
	codes map[string]bool // empty => all rules ignored for matching paths
}

// NewMatcher compiles the config's globs into a Matcher. It returns an error if
// any include, exclude, or ignore pattern is not a valid glob.
func (c *Config) NewMatcher() (*Matcher, error) {
	m := &Matcher{rules: make(map[string]ruleMatcher, len(c.Rules))}
	var err error
	if m.include, err = compileGlobs(c.Revparrot.Include); err != nil {
		return nil, err
	}
	if m.exclude, err = compileGlobs(c.Revparrot.Exclude); err != nil {
		return nil, err
	}
	for _, r := range c.Rules {
		var rm ruleMatcher
		if rm.include, err = compileGlobs(r.Include); err != nil {
			return nil, err
		}
		if rm.exclude, err = compileGlobs(r.Exclude); err != nil {
			return nil, err
		}
		m.rules[r.Code] = rm
	}
	for pat, codes := range c.Ignore {
		gs, err := compileGlobs([]string{pat})
		if err != nil {
			return nil, err
		}
		set := make(map[string]bool, len(codes))
		for _, code := range codes {
			set[code] = true
		}
		m.ignore = append(m.ignore, ignoreMatcher{glob: gs, codes: set})
	}
	return m, nil
}

func normPath(p string) string {
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

// InScope reports whether relPath falls within the global include/exclude
// scope. An empty include set means "include everything"; exclude always wins
// over include.
func (m *Matcher) InScope(relPath string) bool {
	p := normPath(relPath)
	if !m.include.empty() && !m.include.match(p) {
		return false
	}
	if m.exclude.match(p) {
		return false
	}
	return true
}

// RuleKnown reports whether code refers to a rule defined in the config.
func (m *Matcher) RuleKnown(code string) bool {
	_, ok := m.rules[code]
	return ok
}

// RuleApplies reports whether the rule with the given code applies to relPath,
// per the rule's own include/exclude. An empty rule include set means the rule
// applies to every file (subject to global scope). Unknown codes return false.
func (m *Matcher) RuleApplies(code, relPath string) bool {
	rm, ok := m.rules[code]
	if !ok {
		return false
	}
	p := normPath(relPath)
	if !rm.include.empty() && !rm.include.match(p) {
		return false
	}
	if rm.exclude.match(p) {
		return false
	}
	return true
}

// Ignored reports whether the rule with the given code is ignored for relPath
// via the [ignore] section. An ignore entry with an empty code list ignores all
// rules for matching paths.
func (m *Matcher) Ignored(code, relPath string) bool {
	p := normPath(relPath)
	for _, ig := range m.ignore {
		if !ig.glob.match(p) {
			continue
		}
		if len(ig.codes) == 0 || ig.codes[code] {
			return true
		}
	}
	return false
}

// Keep reports whether a violation for the given rule code at relPath should be
// reported during the check phase. It drops violations suppressed by [ignore],
// and violations whose code names a known rule that does not apply to the path.
// Violations whose code is not a configured rule are kept (so unknown or legacy
// annotations stay visible) unless an [ignore] entry suppresses them.
func (m *Matcher) Keep(code, relPath string) bool {
	if m.Ignored(code, relPath) {
		return false
	}
	if m.RuleKnown(code) && !m.RuleApplies(code, relPath) {
		return false
	}
	return true
}
