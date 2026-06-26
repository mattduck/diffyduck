package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattduck/diffyduck/pkg/rpconfig"
	"github.com/mattduck/diffyduck/pkg/scanner"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
	"github.com/muesli/termenv"
)

var version = "dev"

const usageGeneral = `Usage: rpt <command> [flags]

Commands:
  check       Scan for RPT violations in code and git-state tickets
  rules       List rules defined in config
  diff        Show rules and their in-scope files touched by a diff
  show        Show rules and their in-scope files changed in a commit
  completion  Print shell completion script

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
	case "show":
		os.Exit(cmdShow(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println("reviewparrot", version)
	case "completion":
		if err := runCompletion(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
	case "__complete":
		runComplete(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, usageGeneral)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usageGeneral)
		os.Exit(2)
	}
}

// violationStyles holds lipgloss styles for violation output (compact and verbose).
type violationStyles struct {
	header    lipgloss.Style // bold white — file basename
	label     lipgloss.Style // dim — metadata labels, line numbers, separators
	dirPart   lipgloss.Style // dim gray — directory part of file paths
	rule      lipgloss.Style // red — rule code
	typeStyle lipgloss.Style // blue — annotation type
	target    lipgloss.Style // bold — verbose block > marker and target line number
}

type colorMode int

const (
	colorAuto colorMode = iota
	colorAlways
	colorNever
)

// colorFlag lets positive and negative spelling aliases update one shared
// mode. If conflicting flags are supplied, the last one wins.
type colorFlag struct {
	mode  *colorMode
	value colorMode
}

func (f colorFlag) String() string {
	return strconv.FormatBool(*f.mode == f.value)
}

func (f colorFlag) IsBoolFlag() bool {
	return true
}

func (f colorFlag) Set(value string) error {
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	if enabled {
		*f.mode = f.value
	} else if f.value == colorAlways {
		*f.mode = colorNever
	} else {
		*f.mode = colorAlways
	}
	return nil
}

func registerColorFlags(fs *flag.FlagSet, mode *colorMode) {
	fs.Var(colorFlag{mode: mode, value: colorAlways}, "color", "force color output")
	fs.Var(colorFlag{mode: mode, value: colorAlways}, "colour", "force colour output")
	fs.Var(colorFlag{mode: mode, value: colorNever}, "no-color", "disable color output")
	fs.Var(colorFlag{mode: mode, value: colorNever}, "no-colour", "disable colour output")
}

func defaultViolationStyles(mode colorMode) violationStyles {
	renderer := lipgloss.NewRenderer(os.Stdout)
	switch mode {
	case colorAlways:
		renderer.SetColorProfile(termenv.ANSI)
	case colorNever:
		renderer.SetColorProfile(termenv.Ascii)
	}
	return violationStyles{
		header:    renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		label:     renderer.NewStyle().Foreground(lipgloss.Color("8")),
		dirPart:   renderer.NewStyle().Foreground(lipgloss.Color("7")),
		rule:      renderer.NewStyle().Foreground(lipgloss.Color("9")),
		typeStyle: renderer.NewStyle().Foreground(lipgloss.Color("12")),
		target:    renderer.NewStyle().Bold(true),
	}
}

// ruleIDPlain returns the plain-text rule identifier: "type(code)" when the
// rule has a type, otherwise just "code". Use this for column-width math.
func ruleIDPlain(r rpconfig.Rule) string {
	if r.Type != "" {
		return r.Type + "(" + r.Code + ")"
	}
	return r.Code
}

// ruleIDStyled returns the styled rule identifier with the type part in blue
// and the code part in the rule color.
func ruleIDStyled(r rpconfig.Rule, vs violationStyles) string {
	if r.Type != "" {
		return vs.typeStyle.Render(r.Type) + vs.label.Render("(") + vs.rule.Render(r.Code) + vs.label.Render(")")
	}
	return vs.rule.Render(r.Code)
}

// styleViolationPath renders a file:line with the directory part dimmed and the
// basename bold, matching the ticketcli comment-list path style.
func styleViolationPath(path string, line int, vs violationStyles) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	lineStr := vs.label.Render(strconv.Itoa(line))
	if dir == "." {
		return vs.header.Render(base) + vs.label.Render(":") + lineStr
	}
	return vs.dirPart.Render(dir+"/") + vs.header.Render(base) + vs.label.Render(":") + lineStr
}

// readViolationContext reads 2 lines of context above and below targetLine from
// file. It tries file as an absolute path first, then as relative to cfgRoot.
// Returns empty slices and an empty string when the file cannot be read.
func readViolationContext(file, cfgRoot string, targetLine int) (above []string, line string, below []string) {
	candidates := []string{file}
	if cfgRoot != "" && !filepath.IsAbs(file) {
		candidates = append(candidates, filepath.Join(cfgRoot, file))
	}

	var fileLines []string
	for _, p := range candidates {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fileLines = append(fileLines, sc.Text())
		}
		f.Close()
		break
	}

	const ctx = 2
	if len(fileLines) == 0 || targetLine < 1 || targetLine > len(fileLines) {
		return nil, "", nil
	}
	line = fileLines[targetLine-1]
	start := targetLine - ctx
	if start < 1 {
		start = 1
	}
	for i := start; i < targetLine; i++ {
		above = append(above, fileLines[i-1])
	}
	end := targetLine + ctx
	if end > len(fileLines) {
		end = len(fileLines)
	}
	for i := targetLine + 1; i <= end; i++ {
		below = append(below, fileLines[i-1])
	}
	return above, line, below
}

// formatViolationOneline renders a violation as a single coloured line:
// [dim dir/][bold file][dim :linenum][dim :] [dim RPT ][cyan type][dim (][red code][dim )] message
func formatViolationOneline(v scanner.Violation, displayRoot string, vs violationStyles) string {
	displayPath := relTo(displayRoot, v.File)
	path := styleViolationPath(displayPath, v.Line, vs)
	var keyword string
	if v.Type != "" {
		inner := vs.typeStyle.Render(v.Type) + vs.label.Render("(") + vs.rule.Render(v.Code) + vs.label.Render(")")
		keyword = vs.label.Render("RPT ") + inner
	} else {
		keyword = vs.label.Render("RPT(") + vs.rule.Render(v.Code) + vs.label.Render(")")
	}
	return path + vs.label.Render(":") + " " + keyword + " " + v.Message
}

// formatViolationBlock renders a single violation as a ┃-bordered block showing
// the rule, file location, surrounding code context, and message.
func formatViolationBlock(v scanner.Violation, cfg *rpconfig.Config, displayRoot string, vs violationStyles) string {
	var b strings.Builder

	labelVal := func(label, value string) string {
		return vs.label.Render(label) + value
	}

	// Rule line: optional type(code) + short title when available.
	typeName := v.Type
	if cfg != nil {
		if r, ok := cfg.RuleByCode(v.Code); ok {
			if typeName == "" {
				typeName = r.Type
			}
		}
	}
	var ruleVal string
	if typeName != "" {
		ruleVal = vs.typeStyle.Render(typeName) + vs.label.Render("(") + vs.rule.Render(v.Code) + vs.label.Render(")")
	} else {
		ruleVal = vs.rule.Render(v.Code)
	}
	if cfg != nil {
		if r, ok := cfg.RuleByCode(v.Code); ok {
			if t := r.ShortTitle(); t != "" {
				ruleVal += "  " + t
			}
		}
	}
	fmt.Fprintf(&b, "%s\n", labelVal("Rule:", "   ")+ruleVal)

	// File line.
	displayPath := relTo(displayRoot, v.File)
	fmt.Fprintf(&b, "%s\n", labelVal("File:", "   ")+styleViolationPath(displayPath, v.Line, vs))

	// Code context.
	above, line, below := readViolationContext(v.File, displayRoot, v.Line)
	if line != "" {
		b.WriteString("\n")
		all := make([]string, 0, len(above)+1+len(below))
		all = append(all, above...)
		all = append(all, line)
		all = append(all, below...)
		targetIdx := len(above)
		startNo := v.Line - len(above)
		gutterW := len(strconv.Itoa(startNo + len(all) - 1))
		for i, l := range all {
			lineNo := startNo + i
			numStr := fmt.Sprintf("%*d", gutterW, lineNo)
			if i == targetIdx {
				fmt.Fprintf(&b, "%s %s  %s\n", vs.target.Render(">"), vs.target.Render(numStr), l)
			} else {
				fmt.Fprintf(&b, "  %s  %s\n", vs.label.Render(numStr), l)
			}
		}
	}

	// Message.
	if v.Message != "" {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s\n", v.Message)
	}

	// Prefix every line with ┃.
	bar := vs.label.Render("┃") + " "
	raw := strings.TrimRight(b.String(), "\n")
	var out strings.Builder
	for _, l := range strings.Split(raw, "\n") {
		out.WriteString(bar)
		out.WriteString(l)
		out.WriteByte('\n')
	}
	return out.String()
}

// printViolationStats prints a per-rule count table followed by a total row.
// Rules are listed in config order; codes absent from config are appended
// alphabetically. Each row shows the rule code, count, and (when available)
// the first line of the rule description.
func printViolationStats(violations []scanner.Violation, cfg *rpconfig.Config, vs violationStyles) {
	counts := make(map[string]int)
	for _, v := range violations {
		counts[v.Code]++
	}

	// Ordered codes: config order first, then extras alphabetically.
	var codes []string
	seen := make(map[string]bool)
	if cfg != nil {
		for _, r := range cfg.Rules {
			if counts[r.Code] > 0 {
				codes = append(codes, r.Code)
				seen[r.Code] = true
			}
		}
	}
	var extras []string
	for code := range counts {
		if !seen[code] {
			extras = append(extras, code)
		}
	}
	sort.Strings(extras)
	codes = append(codes, extras...)

	// Column widths from plain (unstyled) content.
	codeW := len("total")
	for _, code := range codes {
		if len(code) > codeW {
			codeW = len(code)
		}
	}
	countW := len(strconv.Itoa(len(violations)))

	for _, code := range codes {
		count := counts[code]
		title := ""
		if cfg != nil {
			if r, ok := cfg.RuleByCode(code); ok {
				title = r.ShortTitle()
			}
		}
		countStr := fmt.Sprintf("%*d", countW, count)
		codePad := strings.Repeat(" ", codeW-len(code))
		if title != "" {
			fmt.Printf("%s  %s%s  %s\n", vs.label.Render(countStr), vs.rule.Render(code), codePad, title)
		} else {
			fmt.Printf("%s  %s\n", vs.label.Render(countStr), vs.rule.Render(code))
		}
	}

	n := len(violations)
	fmt.Printf("\nFound %d violation%s.\n", n, pluralS(n))
}

// cmdCheck scans paths for RPT violations and prints them.
// Exit code: 0 = clean, 1 = violations found, 2 = error.
func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	flagRule := fs.String("rule", "", "filter to a specific rule code")
	flagType := fs.String("type", "", "filter to a specific type")
	flagConfig := fs.String("config", "", "explicit config file path")
	flagOneline := fs.Bool("oneline", false, "compact one-line output instead of verbose blocks")
	flagStats := fs.Bool("statistics", false, "show per-rule violation counts instead of individual violations")
	flagUnknown := fs.Bool("unknown", false, "include violations with codes not defined in the config")
	flagN := fs.Int("n", 0, "show at most N violations (0 = no limit)")
	color := colorAuto
	registerColorFlags(fs, &color)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt check [flags] [path...]

Scan for RPT violation annotations in source files.
Reports violations not suppressed by NORPT.

Also reports file-attached rule-tagged tickets from the git-state
store (tdb): an unresolved ticket with a rule code is a violation
(resolved suppresses), subject to the same revparrot.toml scope as
code annotations.

When a revparrot.toml is found, only violations whose code matches a
defined rule are reported. Use --unknown to include annotations with
codes not defined in the config.

Flags:
  --oneline           compact one-line output (default is verbose blocks)
  --statistics        show per-rule counts instead of individual violations
  --unknown           include violations with codes not defined in the config
  --color             force color output (alias: --colour)
  --no-color          disable color output (alias: --no-colour)
  -n <count>          show at most N violations (total count still reported)
  -rule <code>        filter output to a specific rule code
  -type <name>        filter output to a specific type
  -config <path>      explicit config file path

Exit codes:
  0  no violations found
  1  violations found
  2  error
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *flagN < 0 {
		fmt.Fprintf(os.Stderr, "error: -n must not be negative\n")
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

	// When a config is present, drop violations with codes not defined in any
	// rule unless --unknown is set. Track the skipped count for the summary.
	var skippedUnknown int
	if cfg != nil && !*flagUnknown {
		var kept []scanner.Violation
		for _, v := range violations {
			if _, ok := cfg.RuleByCode(v.Code); ok {
				kept = append(kept, v)
			} else {
				skippedUnknown++
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

	// Apply -type filter. Resolves effective type from the annotation first,
	// falling back to the rule config when the annotation has none.
	if *flagType != "" {
		var filtered []scanner.Violation
		for _, v := range violations {
			typeName := v.Type
			if typeName == "" && cfg != nil {
				if r, ok := cfg.RuleByCode(v.Code); ok {
					typeName = r.Type
				}
			}
			if strings.EqualFold(typeName, *flagType) {
				filtered = append(filtered, v)
			}
		}
		violations = filtered
	}

	n := len(violations)
	if n == 0 {
		fmt.Println("No violations found.")
		if skippedUnknown > 0 {
			fmt.Printf("(%d annotation%s with unknown rule code%s skipped — use --unknown to include)\n",
				skippedUnknown, pluralS(skippedUnknown), pluralS(skippedUnknown))
		}
		return 0
	}

	// Prefer cfgRoot for relative paths; fall back to cwd so absolute paths
	// are never shown when a reasonable anchor is available.
	displayRoot := cfgRoot
	if displayRoot == "" {
		displayRoot, _ = os.Getwd()
	}

	vs := defaultViolationStyles(color)
	if *flagStats {
		printViolationStats(violations, cfg, vs)
		if skippedUnknown > 0 {
			fmt.Printf("(%d annotation%s with unknown rule code%s skipped — use --unknown to include)\n",
				skippedUnknown, pluralS(skippedUnknown), pluralS(skippedUnknown))
		}
		return 1
	}

	renderN := n
	if *flagN > 0 && *flagN < n {
		renderN = *flagN
	}

	if *flagOneline {
		for _, v := range violations[:renderN] {
			fmt.Println(formatViolationOneline(v, displayRoot, vs))
		}
	} else {
		for i, v := range violations[:renderN] {
			if i > 0 {
				fmt.Println()
			}
			fmt.Print(formatViolationBlock(v, cfg, displayRoot, vs))
		}
	}

	fmt.Printf("\n%s\n", violationSummary(renderN, n))
	if skippedUnknown > 0 {
		fmt.Printf("(%d annotation%s with unknown rule code%s skipped — use --unknown to include)\n",
			skippedUnknown, pluralS(skippedUnknown), pluralS(skippedUnknown))
	}
	return 1
}

// violationSummary returns the summary line for violation output. When fewer
// violations are rendered than total (due to -n), it says "Showing R of N"
// instead of "Found N".
func violationSummary(rendered, total int) string {
	if rendered < total {
		return fmt.Sprintf("Showing %d of %d violation%s.", rendered, total, pluralS(total))
	}
	return fmt.Sprintf("Found %d violation%s.", total, pluralS(total))
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
	flagType := fs.String("type", "", "filter to a specific type")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt rules [flags]

List rules defined in revparrot.toml.

Flags:
  -type <name>       filter to a specific type
  -config <path>     explicit config file path
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

	vs := defaultViolationStyles(colorAuto)

	// Find column width for the displayed rule id (category:code or code).
	maxID := 0
	for _, r := range cfg.Rules {
		if w := len(ruleIDPlain(r)); w > maxID {
			maxID = w
		}
	}

	for _, r := range cfg.Rules {
		if *flagType != "" && !strings.EqualFold(r.Type, *flagType) {
			continue
		}
		status := "enabled"
		if !r.IsEnabled() {
			status = "disabled"
		}
		includes := strings.Join(r.Include, ", ")
		if includes == "" {
			includes = "(all files)"
		}
		id := ruleIDPlain(r)
		pad := strings.Repeat(" ", maxID-len(id))
		fmt.Printf("%s%s  %-8s  %s\n", ruleIDStyled(r, vs), pad, status, filepath.ToSlash(includes))
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
	flagType := fs.String("type", "", "filter to a specific type")
	flagConfig := fs.String("config", "", "explicit config file path")
	flagAll := fs.Bool("a", false, "include untracked files (working-tree mode only)")
	flagCached := fs.Bool("cached", false, "show staged changes only (alias: --staged)")
	fs.Bool("staged", false, "alias for --cached") // handled below via Lookup
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

Flags:
  -a                 include untracked files (working-tree mode only)
  --cached           staged changes only
  -rule <code>       filter to a specific rule code
  -type <name>       filter to a specific type
  -config <path>     explicit config file path

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
	if *flagAll && *flagCached {
		fmt.Fprintln(os.Stderr, "error: -a and --cached cannot be used together")
		return 2
	}
	if *flagAll && len(refs) > 0 {
		fmt.Fprintln(os.Stderr, "error: -a is only valid for working-tree mode (no refs)")
		return 2
	}
	if *flagCached && len(refs) > 0 {
		fmt.Fprintln(os.Stderr, "error: --cached cannot be used with ref arguments")
		return 2
	}
	if len(refs) > 2 {
		fmt.Fprintf(os.Stderr, "error: diff accepts at most 2 refs, got %d\n", len(refs))
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

	diffFiles, err := diffedFiles(gitRoot, refs, false, *flagCached, *flagAll)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error getting diff files:", err)
		return 2
	}

	vs := defaultViolationStyles(colorAuto)
	printed := 0
	for _, r := range cfg.Rules {
		if !r.IsEnabled() {
			continue
		}
		if *flagRule != "" && r.Code != *flagRule {
			continue
		}
		if *flagType != "" && !strings.EqualFold(r.Type, *flagType) {
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
		fmt.Printf("Rule: %s\n", ruleIDStyled(r, vs))
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

// cmdShow lists rules and their in-scope files changed in a single commit.
// Exit code: 0 = ok, 2 = error.
func cmdShow(args []string) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	flagRule := fs.String("rule", "", "filter to a specific rule code")
	flagType := fs.String("type", "", "filter to a specific type")
	flagConfig := fs.String("config", "", "explicit config file path")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt show [flags] [ref]

Show rules and their in-scope files changed in a single commit.

Defaults to HEAD. Pass a ref to inspect a specific commit.

Flags:
  -rule <code>       filter to a specific rule code
  -type <name>       filter to a specific type
  -config <path>     explicit config file path

Exit codes:
  0  ok
  2  error
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "error: show accepts at most 1 ref, got %d\n", fs.NArg())
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

	diffFiles, err := diffedFiles(gitRoot, fs.Args(), true, false, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error getting diff files:", err)
		return 2
	}

	vs := defaultViolationStyles(colorAuto)
	printed := 0
	for _, r := range cfg.Rules {
		if !r.IsEnabled() {
			continue
		}
		if *flagRule != "" && r.Code != *flagRule {
			continue
		}
		if *flagType != "" && !strings.EqualFold(r.Type, *flagType) {
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
		fmt.Printf("Rule: %s\n", ruleIDStyled(r, vs))
		fmt.Println("Files:")
		for _, f := range matching {
			fmt.Printf("  %s\n", f)
		}
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
		fmt.Println("No rules have files in scope for the current commit.")
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
