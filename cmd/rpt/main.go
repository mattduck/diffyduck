package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mattduck/diffyduck/pkg/rpconfig"
	"github.com/mattduck/diffyduck/pkg/scanner"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
)

var version = "dev"

const usageGeneral = `Usage: rpt <command> [flags]

Commands:
  check    Scan for REVP violations in code and git-state tickets
  rules    List rules defined in config
  diff     Show rules and their in-scope files touched by the current diff

Run 'rpt <command> -h' for command-specific help.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageGeneral)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "check":
		os.Exit(cmdCheck(os.Args[2:]))
	case "rules":
		os.Exit(cmdRules(os.Args[2:]))
	case "diff":
		os.Exit(cmdDiff(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println("reviewparrot", version)
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, usageGeneral)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usageGeneral)
		os.Exit(2)
	}
}

// cmdCheck scans paths for REVP violations and prints them.
// Exit code: 0 = clean, 1 = violations found, 2 = error.
func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	flagRule := fs.String("rule", "", "filter to a specific rule code")
	flagConfig := fs.String("config", "", "explicit config file path")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt check [flags] [path...]

Scan for REVP violation annotations in source files.
Reports violations not suppressed by NOREVP.

Also reports file-attached rule-tagged tickets from the git-state
store (tdb): an unresolved ticket with a rule code is a violation
(resolved suppresses), subject to the same revparrot.toml scope as
code annotations.

Flags:
  -rule <code>     filter output to a specific rule code
  -config <path>   explicit config file path

Exit codes:
  0  no violations found
  1  violations found
  2  error
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	paths := fs.Args()
	if len(paths) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
		paths = []string{cwd}
	}

	// Load config so include/exclude/ignore scoping applies. A missing config is
	// not an error: we fall back to scanning everything unfiltered.
	var cfg *rpconfig.Config
	var cfgRoot string
	{
		var err error
		var cfgPath string
		if *flagConfig != "" {
			cfg, err = rpconfig.LoadFromPath(*flagConfig)
			cfgPath = *flagConfig
		} else {
			cfg, cfgPath, err = rpconfig.Load(paths[0])
		}
		if err != nil {
			if !errors.Is(err, rpconfig.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "error loading config:", err)
				return 2
			}
			cfg = nil // not found: scan unfiltered
		} else {
			cfgRoot = filepath.Dir(cfgPath)
		}
	}

	if cfg != nil && *flagRule != "" {
		if _, ok := cfg.RuleByCode(*flagRule); !ok {
			fmt.Fprintf(os.Stderr, "unknown rule code: %q\n", *flagRule)
			return 2
		}
	}

	// Build the path matcher from config, if any.
	var matcher *rpconfig.Matcher
	if cfg != nil {
		var err error
		matcher, err = cfg.NewMatcher()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error in config patterns:", err)
			return 2
		}
	}

	opts := walkOptions(matcher, cfgRoot)

	var violations []scanner.Violation
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
		var vs []scanner.Violation
		if info.IsDir() {
			vs, err = scanner.ScanDir(path, opts)
		} else {
			// Explicitly named files are always scanned; per-rule scope and
			// [ignore] still apply to their violations below.
			vs, err = scanner.ScanFile(path)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error scanning:", err)
			return 2
		}
		violations = append(violations, vs...)
	}

	// Merge rule-tagged tickets from the git-state store. They go through the same
	// per-rule scope and -rule filtering as code violations below. State sourcing
	// is best-effort: outside a git repo (or on any store error) we warn and
	// continue with code violations only.
	stateViols, serr := gatherStateViolations()
	if serr != nil {
		fmt.Fprintln(os.Stderr, "warning: skipping git-state tickets:", serr)
	}
	// The global [revparrot] include/exclude is enforced for code by not walking
	// excluded files; state violations bypass the walk, so apply it here.
	if matcher != nil {
		var inScope []scanner.Violation
		for _, v := range stateViols {
			if matcher.InScope(relTo(cfgRoot, v.File)) {
				inScope = append(inScope, v)
			}
		}
		stateViols = inScope
	}
	violations = append(violations, stateViols...)

	// Apply rule-level include/exclude and [ignore] suppression.
	if matcher != nil {
		var kept []scanner.Violation
		for _, v := range violations {
			if matcher.Keep(v.Code, relTo(cfgRoot, v.File)) {
				kept = append(kept, v)
			}
		}
		violations = kept
	}

	// Apply -rule filter.
	if *flagRule != "" {
		var filtered []scanner.Violation
		for _, v := range violations {
			if v.Code == *flagRule {
				filtered = append(filtered, v)
			}
		}
		violations = filtered
	}

	for _, v := range violations {
		fmt.Println(v)
	}

	n := len(violations)
	if n == 0 {
		fmt.Println("No violations found.")
		return 0
	}
	fmt.Printf("\nFound %d violation%s.\n", n, pluralS(n))
	return 1
}

// gatherStateViolations reads rule-tagged tickets from the git-state store and
// maps them to scanner.Violation so they merge with code violations under the
// same config scope. Resolved tickets are excluded (resolved is the state-side
// suppression). Standalone (file-less) tickets are excluded: a rule violation is
// inherently located in code, and the config path scope can't apply to a ticket
// with no path.
func gatherStateViolations() ([]scanner.Violation, error) {
	store := ticketdb.NewStore("")
	all, err := store.AllComments()
	if err != nil {
		return nil, err
	}

	var out []scanner.Violation
	for _, c := range all {
		if c.Rule == "" || c.Resolved || c.IsStandalone() {
			continue
		}
		out = append(out, scanner.Violation{
			File:    c.File,
			Line:    c.Line,
			Code:    c.Rule,
			Message: stateMessage(c),
		})
	}
	return out, nil
}

// stateMessage returns the violation message for a ticket: its title if set,
// else the first line of its body.
func stateMessage(c *ticketdb.Comment) string {
	if c.Title != "" {
		return c.Title
	}
	t := c.Text
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[:i]
	}
	return t
}

// cmdRules lists rules from rpconfig.
// Exit code: 0 = ok, 2 = error.
func cmdRules(args []string) int {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	flagConfig := fs.String("config", "", "explicit config file path")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt rules [flags]

List rules defined in revparrot.toml.

Flags:
  -config <path>   explicit config file path
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var cfg *rpconfig.Config
	var err error
	if *flagConfig != "" {
		cfg, err = rpconfig.LoadFromPath(*flagConfig)
	} else {
		cwd, werr := os.Getwd()
		if werr != nil {
			fmt.Fprintln(os.Stderr, "error:", werr)
			return 2
		}
		cfg, _, err = rpconfig.Load(cwd)
	}
	if err != nil {
		if errors.Is(err, rpconfig.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "no revparrot.toml found")
			return 2
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}

	if len(cfg.Rules) == 0 {
		fmt.Println("No rules defined.")
		return 0
	}

	// Find column width for code.
	maxCode := 0
	for _, r := range cfg.Rules {
		if len(r.Code) > maxCode {
			maxCode = len(r.Code)
		}
	}

	for _, r := range cfg.Rules {
		status := "enabled"
		if !r.IsEnabled() {
			status = "disabled"
		}
		includes := strings.Join(r.Include, ", ")
		if includes == "" {
			includes = "(all files)"
		}
		fmt.Printf("%-*s  %-8s  %s\n", maxCode, r.Code, status, filepath.ToSlash(includes))
	}
	return 0
}

// cmdDiff shows, for each enabled rule, which files touched by the current
// working-tree diff (staged + unstaged changes + untracked files) fall within
// that rule's scope. Rules with no matching files are omitted. Designed as a
// scoping tool for an LLM agent: the agent reads the output to learn exactly
// which files it needs to review for each rule and what to look for.
// Exit code: 0 = ok, 2 = error.
func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	flagRule := fs.String("rule", "", "filter to a specific rule code")
	flagConfig := fs.String("config", "", "explicit config file path")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt diff [flags]

Show rules and their in-scope files touched by the current diff.

For each enabled rule, lists the files from the working-tree diff
(staged + unstaged + untracked) that fall within that rule's scope,
together with the rule's description so an agent knows what to check.
Rules with no matching files are omitted.

Flags:
  -rule <code>     filter to a specific rule code
  -config <path>   explicit config file path

Exit codes:
  0  ok
  2  error
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var cfg *rpconfig.Config
	var cfgRoot string
	{
		var err error
		var cfgPath string
		if *flagConfig != "" {
			cfg, err = rpconfig.LoadFromPath(*flagConfig)
			cfgPath = *flagConfig
		} else {
			cwd, werr := os.Getwd()
			if werr != nil {
				fmt.Fprintln(os.Stderr, "error:", werr)
				return 2
			}
			cfg, cfgPath, err = rpconfig.Load(cwd)
		}
		if err != nil {
			if errors.Is(err, rpconfig.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "no revparrot.toml found")
				return 2
			}
			fmt.Fprintln(os.Stderr, "error loading config:", err)
			return 2
		}
		cfgRoot = filepath.Dir(cfgPath)
	}

	if *flagRule != "" {
		if _, ok := cfg.RuleByCode(*flagRule); !ok {
			fmt.Fprintf(os.Stderr, "unknown rule code: %q\n", *flagRule)
			return 2
		}
	}

	matcher, err := cfg.NewMatcher()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}

	// Collect files touched by the current diff: tracked changes (staged +
	// unstaged vs HEAD) plus untracked files. Paths are returned as absolute so
	// they can be made relative to cfgRoot by relTo below.
	diffFiles, err := diffedFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error getting diff files:", err)
		return 2
	}

	printed := 0
	for _, r := range cfg.Rules {
		if !r.IsEnabled() {
			continue
		}
		if *flagRule != "" && r.Code != *flagRule {
			continue
		}

		var matching []string
		for _, f := range diffFiles {
			rel := relTo(cfgRoot, f)
			if matcher.InScope(rel) && matcher.RuleApplies(r.Code, rel) && !matcher.Ignored(r.Code, rel) {
				matching = append(matching, rel)
			}
		}
		if len(matching) == 0 {
			continue
		}

		if printed > 0 {
			fmt.Println()
		}
		fmt.Printf("Rule: %s\n", r.Code)
		fmt.Println("Files:")
		for _, f := range matching {
			fmt.Printf("  %s\n", f)
		}
		// Print the description as "Check:" so the agent understands its purpose.
		desc := strings.TrimSpace(r.Description)
		if desc == "" {
			desc = "(no description)"
		}
		lines := strings.Split(desc, "\n")
		if len(lines) == 1 {
			fmt.Printf("Check: %s\n", lines[0])
		} else {
			fmt.Println("Check:")
			for _, l := range lines {
				fmt.Printf("  %s\n", l)
			}
		}
		printed++
	}

	if printed == 0 {
		fmt.Println("No rules have files in scope for the current diff.")
	}
	return 0
}

// diffedFiles returns the set of files touched by the current working-tree
// diff relative to HEAD (staged + unstaged tracked changes) plus untracked
// files. Paths are absolute. Finds the git root from cwd, independent of
// where revparrot.toml lives.
func diffedFiles() ([]string, error) {
	// Find git root from cwd.
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git rev-parse: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}
	gitRoot := strings.TrimSpace(string(out))

	var files []string
	seen := make(map[string]bool)

	add := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		abs := filepath.Clean(filepath.Join(gitRoot, line))
		if !seen[abs] {
			seen[abs] = true
			files = append(files, abs)
		}
	}

	// Tracked changes: staged + unstaged vs HEAD.
	out, err = exec.Command("git", "-C", gitRoot, "diff", "--name-only", "HEAD").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("git diff: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		// No HEAD yet (empty repo) — not fatal, fall through.
	} else {
		for _, line := range strings.Split(string(out), "\n") {
			add(line)
		}
	}

	// Untracked files (new files not yet staged).
	out, err = exec.Command("git", "-C", gitRoot, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("git ls-files: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
	} else {
		for _, line := range strings.Split(string(out), "\n") {
			add(line)
		}
	}

	return files, nil
}

// walkOptions builds scanner walk predicates from the matcher. Version-control
// directories are always pruned. When a matcher is present, files outside the
// global include/exclude scope are skipped during directory walks.
func walkOptions(matcher *rpconfig.Matcher, root string) scanner.WalkOptions {
	keepDir := func(p string) bool {
		switch filepath.Base(p) {
		case ".git", ".hg", ".svn":
			return false
		}
		return true
	}
	if matcher == nil {
		return scanner.WalkOptions{KeepDir: keepDir}
	}
	return scanner.WalkOptions{
		KeepDir: keepDir,
		KeepFile: func(p string) bool {
			return matcher.InScope(relTo(root, p))
		},
	}
}

// relTo returns path relative to root, using forward slashes. It falls back to
// the original path if a relative path cannot be computed.
func relTo(root, path string) string {
	if root == "" {
		return filepath.ToSlash(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
