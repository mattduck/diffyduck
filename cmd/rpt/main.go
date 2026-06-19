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

// cmdDiff shows, for each enabled rule, which files touched by the specified
// diff fall within that rule's scope, together with the rule description.
// Rules with no matching files are omitted. Designed as a scoping tool for an
// LLM agent: the agent reads the output to learn what to check and where.
// Exit code: 0 = ok, 2 = error.
func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	flagRule := fs.String("rule", "", "filter to a specific rule code")
	flagConfig := fs.String("config", "", "explicit config file path")
	flagAll := fs.Bool("a", false, "include untracked files (working-tree mode only)")
	flagCached := fs.Bool("cached", false, "show staged changes only (alias: --staged)")
	fs.Bool("staged", false, "alias for --cached") // handled below via Lookup
	flagShow := fs.Bool("show", false, "show files changed in a single commit (default: HEAD)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt diff [flags] [ref...]

Show rules and their in-scope files touched by the given diff.

For each enabled rule, lists the matching files together with the rule
description so an agent knows what to check. Rules with no matching
files are omitted.

Diff sources (pick one):
  rpt diff                  working-tree changes vs HEAD (staged + unstaged)
  rpt diff -a               same, plus untracked files
  rpt diff --cached         staged changes only
  rpt diff <ref>            working tree vs ref  (git diff <ref>)
  rpt diff <ref1> <ref2>    between two refs     (git diff <ref1> <ref2>)
  rpt diff --show           files changed in HEAD (git show)
  rpt diff --show <ref>     files changed in commit <ref>

Flags:
  -a               include untracked files (working-tree mode only)
  --cached         staged changes only
  --show           single-commit mode (git show)
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

	// --staged is an alias for --cached.
	if f := fs.Lookup("staged"); f != nil && f.Value.String() == "true" {
		*flagCached = true
	}

	refs := fs.Args()

	// Validate flag/ref combinations.
	if *flagShow && *flagCached {
		fmt.Fprintln(os.Stderr, "error: --show and --cached cannot be used together")
		return 2
	}
	if *flagAll && *flagCached {
		fmt.Fprintln(os.Stderr, "error: -a and --cached cannot be used together")
		return 2
	}
	if *flagAll && len(refs) > 0 {
		fmt.Fprintln(os.Stderr, "error: -a is only valid for working-tree mode (no refs)")
		return 2
	}
	if *flagAll && *flagShow {
		fmt.Fprintln(os.Stderr, "error: -a is only valid for working-tree mode (not --show)")
		return 2
	}
	if *flagCached && len(refs) > 0 {
		fmt.Fprintln(os.Stderr, "error: --cached cannot be used with ref arguments")
		return 2
	}
	if !*flagShow && len(refs) > 2 {
		fmt.Fprintf(os.Stderr, "error: diff accepts at most 2 refs, got %d\n", len(refs))
		return 2
	}
	if *flagShow && len(refs) > 1 {
		fmt.Fprintf(os.Stderr, "error: --show accepts at most 1 ref, got %d\n", len(refs))
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

	gitRoot, err := gitTopLevel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}

	diffFiles, err := diffedFiles(gitRoot, refs, *flagShow, *flagCached, *flagAll)
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

// gitTopLevel returns the absolute path of the git repository root, found from
// the current working directory.
func gitTopLevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git rev-parse: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// diffedFiles returns the absolute paths of files touched by the requested
// diff. The source is determined by the combination of flags and refs:
//
//   - showMode=true: files changed in the given commit (git diff-tree); refs[0]
//     is the commit (default HEAD).
//   - cached=true: staged changes only (git diff --cached).
//   - len(refs)>0: diff between working tree and ref, or between two refs
//     (git diff refs...).
//   - default (no refs, not cached, not show): staged+unstaged vs HEAD
//     (git diff HEAD); includeUntracked appends ls-files --others output.
func diffedFiles(gitRoot string, refs []string, showMode, cached, includeUntracked bool) ([]string, error) {
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

	addOutput := func(out []byte) {
		for _, line := range strings.Split(string(out), "\n") {
			add(line)
		}
	}

	gitErr := func(cmd string, err error) error {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return fmt.Errorf("%s: %s", cmd, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("%s: %w", cmd, err)
	}

	switch {
	case showMode:
		// Single-commit mode: list files changed in the commit.
		// git diff-tree is more reliable than git show --name-only since it
		// never outputs the commit message.
		ref := "HEAD"
		if len(refs) == 1 {
			ref = refs[0]
		}
		out, err := exec.Command("git", "-C", gitRoot, "diff-tree", "--no-commit-id", "-r", "--name-only", ref).Output()
		if err != nil {
			return nil, gitErr("git diff-tree", err)
		}
		addOutput(out)

	case cached:
		out, err := exec.Command("git", "-C", gitRoot, "diff", "--name-only", "--cached").Output()
		if err != nil {
			return nil, gitErr("git diff --cached", err)
		}
		addOutput(out)

	case len(refs) > 0:
		// Explicit ref(s): pass through to git diff.
		gitArgs := append([]string{"-C", gitRoot, "diff", "--name-only"}, refs...)
		out, err := exec.Command("git", gitArgs...).Output()
		if err != nil {
			return nil, gitErr("git diff", err)
		}
		addOutput(out)

	default:
		// Working-tree mode: staged + unstaged vs HEAD.
		out, err := exec.Command("git", "-C", gitRoot, "diff", "--name-only", "HEAD").Output()
		if err != nil {
			// No HEAD yet (empty repo) — not fatal.
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
				return nil, gitErr("git diff", err)
			}
		} else {
			addOutput(out)
		}

		if includeUntracked {
			out, err = exec.Command("git", "-C", gitRoot, "ls-files", "--others", "--exclude-standard").Output()
			if err != nil {
				return nil, gitErr("git ls-files", err)
			}
			addOutput(out)
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
