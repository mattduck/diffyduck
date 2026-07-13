package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
  ls          List rules and their in-scope files across the working tree
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
	case "ls":
		os.Exit(cmdLs(os.Args[2:]))
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
// For built-in rule codes the RPT prefix is omitted and just the code is shown.
func formatViolationOneline(v scanner.Violation, displayRoot string, vs violationStyles) string {
	displayPath := relTo(displayRoot, v.File)
	path := styleViolationPath(displayPath, v.Line, vs)
	var keyword string
	if rpconfig.IsBuiltinCode(v.Code) {
		keyword = vs.rule.Render(v.Code)
	} else if v.Type != "" {
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
	if r, ok := rpconfig.AllRuleByCode(cfg, v.Code); ok {
		if typeName == "" {
			typeName = r.Type
		}
	}
	var ruleVal string
	if typeName != "" {
		ruleVal = vs.typeStyle.Render(typeName) + vs.label.Render("(") + vs.rule.Render(v.Code) + vs.label.Render(")")
	} else {
		ruleVal = vs.rule.Render(v.Code)
	}
	if r, ok := rpconfig.AllRuleByCode(cfg, v.Code); ok {
		if t := r.ShortTitle(); t != "" {
			ruleVal += "  " + t
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
		if r, ok := rpconfig.AllRuleByCode(cfg, code); ok {
			title = r.ShortTitle()
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
	flagSelect := fs.String("select", "", "run only these rule codes (comma-separated; replaces config rules)")
	flagExtendSelect := fs.String("extend-select", "", "add rule codes to the active set (comma-separated)")
	flagType := fs.String("type", "", "filter to a specific type")
	flagConfig := fs.String("config", "", "explicit config file path")
	flagOneline := fs.Bool("oneline", false, "compact one-line output instead of verbose blocks")
	flagStats := fs.Bool("statistics", false, "show per-rule violation counts instead of individual violations")
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

When a revparrot.toml is found, only violations whose code matches an
active rule are reported. Use -select or -extend-select to override or
extend the active rule set.

Built-in annotation-quality rules (rpt-syntax, rpt-unknown-scope,
rpt-type-mismatch) are disabled by default; enable with -extend-select.

Flags:
  --oneline               compact one-line output (default is verbose blocks)
  --statistics            show per-rule counts instead of individual violations
  --color                 force color output (alias: --colour)
  --no-color              disable color output (alias: --no-colour)
  -n <count>              show at most N violations (total count still reported)
  -select <codes>         run only these rule codes (replaces config rules)
  -extend-select <codes>  add rule codes to the active set
  -type <name>            filter output to a specific type
  -config <path>          explicit config file path

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

	selectList := splitComma(*flagSelect)
	extendList := splitComma(*flagExtendSelect)

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

	// Resolve active rule set from config + selection flags.
	activeRules, err := rpconfig.ResolveRuleset(cfg, selectList, extendList)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	activeBuiltins := builtinActiveSet(activeRules)

	// Build the path matcher from config, if any.
	var matcher *rpconfig.Matcher
	if cfg != nil {
		matcher, err = cfg.NewMatcher()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error in config patterns:", err)
			return 2
		}
	}

	opts := walkOptions(matcher, cfgRoot)

	// Strict scan — collect raw code violations before any filtering so that
	// the full set is available for built-in rule generation and syntax checking.
	var rawCodeViolations []scanner.Violation
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil {
			fmt.Fprintln(os.Stderr, "error:", statErr)
			return 2
		}
		var vs []scanner.Violation
		if info.IsDir() {
			vs, statErr = scanner.ScanDir(path, opts)
		} else {
			// Explicitly named files are always scanned; per-rule scope and
			// [ignore] still apply to their violations below.
			vs, statErr = scanner.ScanFile(path)
		}
		if statErr != nil {
			fmt.Fprintln(os.Stderr, "error scanning:", statErr)
			return 2
		}
		rawCodeViolations = append(rawCodeViolations, vs...)
	}

	// Apply rule-level include/exclude and [ignore] suppression to code violations.
	codeViolations := rawCodeViolations
	if matcher != nil {
		var kept []scanner.Violation
		for _, v := range rawCodeViolations {
			if matcher.Keep(v.Code, relTo(cfgRoot, v.File)) {
				kept = append(kept, v)
			}
		}
		codeViolations = kept
	}

	// Generate built-in violations from the filtered code violations.
	// rpt-unknown-scope: well-formed annotations whose scope isn't in config.
	var builtinViolations []scanner.Violation
	if activeBuiltins[rpconfig.CodeRPTUnknownScope] && cfg != nil {
		for _, v := range codeViolations {
			if !rpconfig.IsBuiltinCode(v.Code) {
				if _, ok := cfg.RuleByCode(v.Code); !ok {
					builtinViolations = append(builtinViolations, scanner.Violation{
						File:    v.File,
						Line:    v.Line,
						Code:    rpconfig.CodeRPTUnknownScope,
						Message: fmt.Sprintf("unknown scope %q", v.Code),
					})
				}
			}
		}
	}
	// rpt-type-mismatch: annotations whose type doesn't match the config rule type.
	if activeBuiltins[rpconfig.CodeRPTTypeMismatch] && cfg != nil {
		for _, v := range codeViolations {
			if !rpconfig.IsBuiltinCode(v.Code) && v.Type != "" {
				if r, ok := cfg.RuleByCode(v.Code); ok && r.Type != "" && !strings.EqualFold(v.Type, r.Type) {
					builtinViolations = append(builtinViolations, scanner.Violation{
						File:    v.File,
						Line:    v.Line,
						Code:    rpconfig.CodeRPTTypeMismatch,
						Message: fmt.Sprintf("type %q does not match rule type %q", v.Type, r.Type),
					})
				}
			}
		}
	}

	// Merge rule-tagged tickets from the git-state store. State sourcing is
	// best-effort: outside a git repo (or on any store error) we warn and
	// continue with code violations only.
	stateViols, serr := gatherStateViolations()
	if serr != nil {
		fmt.Fprintln(os.Stderr, "warning: skipping git-state tickets:", serr)
	}
	// The global [revparrot] include/exclude is enforced for code by not walking
	// excluded files; state violations bypass the walk, so apply it here.
	if matcher != nil {
		var inScope []scanner.Violation
		for _, sv := range stateViols {
			if matcher.InScope(relTo(cfgRoot, sv.File)) {
				inScope = append(inScope, sv)
			}
		}
		stateViols = inScope
	}

	// Combine: filtered code + state + built-in violations.
	violations := make([]scanner.Violation, 0, len(codeViolations)+len(stateViols)+len(builtinViolations))
	violations = append(violations, codeViolations...)
	violations = append(violations, stateViols...)
	violations = append(violations, builtinViolations...)

	// Apply active-rules filter: when a ruleset is active, keep only those codes.
	if activeRules != nil {
		activeSet := make(map[string]bool, len(activeRules))
		for _, r := range activeRules {
			activeSet[r.Code] = true
		}
		var kept []scanner.Violation
		for _, v := range violations {
			if activeSet[v.Code] {
				kept = append(kept, v)
			}
		}
		violations = kept
	}

	// rpt-syntax: loose scan to find malformed RPT annotations not caught by the
	// strict scanner. Subtract the strict-scan locations to avoid double-reporting.
	if activeBuiltins[rpconfig.CodeRPTSyntax] {
		strictSet := make(map[fileLineKey]bool, len(rawCodeViolations))
		for _, v := range rawCodeViolations {
			strictSet[fileLineKey{v.File, v.Line}] = true
		}
		syntaxViols, syntaxErr := gatherSyntaxViolations(paths, opts, strictSet, matcher, cfgRoot)
		if syntaxErr != nil {
			fmt.Fprintln(os.Stderr, "error scanning for syntax violations:", syntaxErr)
			return 2
		}
		violations = append(violations, syntaxViols...)
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

// cmdRules lists rules from rpconfig, including any selected built-in rules.
// Exit code: 0 = ok, 2 = error.
func cmdRules(args []string) int {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	flagSelect := fs.String("select", "", "show only these rule codes (comma-separated)")
	flagExtendSelect := fs.String("extend-select", "", "show these codes in addition to config rules (comma-separated)")
	flagConfig := fs.String("config", "", "explicit config file path")
	flagType := fs.String("type", "", "filter to a specific type")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt rules [flags]

List rules defined in revparrot.toml plus any selected built-in rules.

Flags:
  -select <codes>         show only these rule codes (comma-separated)
  -extend-select <codes>  show built-in rules in addition to config rules
  -type <name>            filter to a specific type
  -config <path>          explicit config file path
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	selectList := splitComma(*flagSelect)
	extendList := splitComma(*flagExtendSelect)

	// Load config. Without any selection flags, a missing config is an error.
	var cfg *rpconfig.Config
	{
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
				if len(selectList) == 0 && len(extendList) == 0 {
					fmt.Fprintln(os.Stderr, "no revparrot.toml found")
					return 2
				}
				cfg = nil // no config but selection specified: only built-in rules
			} else {
				fmt.Fprintln(os.Stderr, "error:", err)
				return 2
			}
		}
	}

	// Build the list of rules to display.
	// With -select: show only those rules.
	// Without -select: show all config rules (enabled + disabled) plus -extend-select additions.
	var displayRules []rpconfig.Rule
	if len(selectList) > 0 {
		for _, code := range selectList {
			r, ok := rpconfig.AllRuleByCode(cfg, code)
			if !ok {
				fmt.Fprintf(os.Stderr, "error: unknown rule code: %q\n", code)
				return 2
			}
			displayRules = append(displayRules, r)
		}
	} else if cfg != nil {
		displayRules = append(displayRules, cfg.Rules...)
	}
	for _, code := range extendList {
		r, ok := rpconfig.AllRuleByCode(cfg, code)
		if !ok {
			fmt.Fprintf(os.Stderr, "error: unknown rule code: %q\n", code)
			return 2
		}
		dup := false
		for _, x := range displayRules {
			if x.Code == code {
				dup = true
				break
			}
		}
		if !dup {
			displayRules = append(displayRules, r)
		}
	}

	if len(displayRules) == 0 {
		fmt.Println("No rules defined.")
		return 0
	}

	vs := defaultViolationStyles(colorAuto)

	maxID := 0
	for _, r := range displayRules {
		if w := len(ruleIDPlain(r)); w > maxID {
			maxID = w
		}
	}

	for _, r := range displayRules {
		if *flagType != "" && !strings.EqualFold(r.Type, *flagType) {
			continue
		}
		var status string
		if rpconfig.IsBuiltinCode(r.Code) {
			status = "builtin"
		} else if r.IsEnabled() {
			status = "enabled"
		} else {
			status = "disabled"
		}
		includes := strings.Join(r.Include, ", ")
		if includes == "" {
			includes = "(all files)"
		}
		id := ruleIDPlain(r)
		pad := strings.Repeat(" ", maxID-len(id))
		line := fmt.Sprintf("%s%s  %-8s  %s", ruleIDStyled(r, vs), pad, status, filepath.ToSlash(includes))
		if hint := orchestrationHint(r); hint != "" {
			line += "  " + vs.label.Render(hint)
		}
		fmt.Println(line)
	}
	return 0
}

// orchestrationHint renders a rule's advisory model/effort metadata as a compact
// "model=… effort=…" suffix, or "" when neither is set.
func orchestrationHint(r rpconfig.Rule) string {
	var parts []string
	if r.Model != "" {
		parts = append(parts, "model="+r.Model)
	}
	if r.Effort != "" {
		parts = append(parts, "effort="+r.Effort)
	}
	return strings.Join(parts, " ")
}

// cmdDiff shows, for each active rule, which files touched by the specified
// diff fall within that rule's scope, together with the rule description.
// Rules with no matching files are omitted. Designed as a scoping tool for an
// LLM agent: the agent reads the output to learn what to check and where.
// Exit code: 0 = ok, 2 = error.
func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	flagSelect := fs.String("select", "", "show only these rule codes (comma-separated; replaces config rules)")
	flagExtendSelect := fs.String("extend-select", "", "add rule codes to the active set (comma-separated)")
	flagType := fs.String("type", "", "filter to a specific type")
	flagConfig := fs.String("config", "", "explicit config file path")
	flagAll := fs.Bool("a", false, "include untracked files (working-tree mode only)")
	flagCached := fs.Bool("cached", false, "show staged changes only (alias: --staged)")
	fs.Bool("staged", false, "alias for --cached") // handled below via Lookup
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt diff [flags] [ref...]

Show rules and their in-scope files touched by the given diff.

For each active rule, lists the matching files together with the rule
description so an agent knows what to check. Rules with no matching
files are omitted.

Diff sources (pick one):
  rpt diff                  working-tree changes vs HEAD (staged + unstaged)
  rpt diff -a               same, plus untracked files
  rpt diff --cached         staged changes only
  rpt diff <ref>            working tree vs ref  (git diff <ref>)
  rpt diff <ref1> <ref2>    between two refs     (git diff <ref1> <ref2>)

Flags:
  -a                      include untracked files (working-tree mode only)
  --cached                staged changes only
  -select <codes>         show only these rule codes (replaces config rules)
  -extend-select <codes>  add rule codes to the active set
  -type <name>            filter to a specific type
  -config <path>          explicit config file path

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

	selectList := splitComma(*flagSelect)
	extendList := splitComma(*flagExtendSelect)

	// Load config. Without selection flags, a missing config is an error.
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
				if len(selectList) == 0 && len(extendList) == 0 {
					fmt.Fprintln(os.Stderr, "no revparrot.toml found")
					return 2
				}
				cfg = nil
			} else {
				fmt.Fprintln(os.Stderr, "error loading config:", err)
				return 2
			}
		} else {
			cfgRoot = filepath.Dir(cfgPath)
		}
	}

	activeRules, err := rpconfig.ResolveRuleset(cfg, selectList, extendList)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	// When no selection is given and cfg is nil we error above; when cfg is
	// non-nil with no selection, activeRules = cfg.ActiveRules() (may be empty).
	if len(activeRules) == 0 {
		fmt.Println("No active rules.")
		return 0
	}

	var matcher *rpconfig.Matcher
	if cfg != nil {
		matcher, err = cfg.NewMatcher()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
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
	groups := ruleFileGroups(activeRules, diffFiles, matcher, cfgRoot, *flagType)
	printRuleFileGroups(os.Stdout, groups, vs)
	if len(groups) == 0 {
		fmt.Println("No rules have files in scope for the current diff.")
	}
	return 0
}

// ruleFileGroup pairs a rule with the in-scope files that fall under it.
type ruleFileGroup struct {
	rule  rpconfig.Rule
	files []string
}

// ruleFileGroups returns, for each rule (optionally filtered by type), the
// subset of files that fall within the rule's scope, preserving the input file
// order. Rules with no matching files are omitted. Shared by `rpt diff`,
// `rpt show`, and `rpt ls`.
func ruleFileGroups(rules []rpconfig.Rule, files []string, matcher *rpconfig.Matcher, cfgRoot, typeFilter string) []ruleFileGroup {
	var groups []ruleFileGroup
	for _, r := range rules {
		if typeFilter != "" && !strings.EqualFold(r.Type, typeFilter) {
			continue
		}
		var matching []string
		for _, f := range files {
			rel := relTo(cfgRoot, f)
			if fileInScopeForRule(r, rel, matcher) {
				matching = append(matching, rel)
			}
		}
		if len(matching) == 0 {
			continue
		}
		groups = append(groups, ruleFileGroup{rule: r, files: matching})
	}
	return groups
}

// printRuleFileGroups renders rule/file groups in the human-readable form shared
// by `rpt diff`, `rpt show`, and `rpt ls`.
func printRuleFileGroups(w io.Writer, groups []ruleFileGroup, vs violationStyles) {
	for i, g := range groups {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "Rule: %s\n", ruleIDStyled(g.rule, vs))
		fmt.Fprintln(w, "Files:")
		for _, f := range g.files {
			fmt.Fprintf(w, "  %s\n", f)
		}
		desc := strings.TrimSpace(g.rule.Description)
		if desc == "" {
			desc = "(no description)"
		}
		lines := strings.Split(desc, "\n")
		if len(lines) == 1 {
			fmt.Fprintf(w, "Check: %s\n", lines[0])
		} else {
			fmt.Fprintln(w, "Check:")
			for _, l := range lines {
				fmt.Fprintf(w, "  %s\n", l)
			}
		}
	}
}

// cmdShow lists rules and their in-scope files changed in a single commit.
// Exit code: 0 = ok, 2 = error.
func cmdShow(args []string) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	flagSelect := fs.String("select", "", "show only these rule codes (comma-separated; replaces config rules)")
	flagExtendSelect := fs.String("extend-select", "", "add rule codes to the active set (comma-separated)")
	flagType := fs.String("type", "", "filter to a specific type")
	flagConfig := fs.String("config", "", "explicit config file path")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt show [flags] [ref]

Show rules and their in-scope files changed in a single commit.

Defaults to HEAD. Pass a ref to inspect a specific commit.

Flags:
  -select <codes>         show only these rule codes (replaces config rules)
  -extend-select <codes>  add rule codes to the active set
  -type <name>            filter to a specific type
  -config <path>          explicit config file path

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

	selectList := splitComma(*flagSelect)
	extendList := splitComma(*flagExtendSelect)

	// Load config. Without selection flags, a missing config is an error.
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
				if len(selectList) == 0 && len(extendList) == 0 {
					fmt.Fprintln(os.Stderr, "no revparrot.toml found")
					return 2
				}
				cfg = nil
			} else {
				fmt.Fprintln(os.Stderr, "error loading config:", err)
				return 2
			}
		} else {
			cfgRoot = filepath.Dir(cfgPath)
		}
	}

	activeRules, err := rpconfig.ResolveRuleset(cfg, selectList, extendList)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if len(activeRules) == 0 {
		fmt.Println("No active rules.")
		return 0
	}

	var matcher *rpconfig.Matcher
	if cfg != nil {
		matcher, err = cfg.NewMatcher()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
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
	groups := ruleFileGroups(activeRules, diffFiles, matcher, cfgRoot, *flagType)
	printRuleFileGroups(os.Stdout, groups, vs)
	if len(groups) == 0 {
		fmt.Println("No rules have files in scope for the current commit.")
	}
	return 0
}

// cmdLs lists, for each active rule, the in-scope files currently present in
// the working tree (or under the given paths). It is the whole-tree analogue of
// `rpt diff`: a scoping tool that tells an agent what to review and where,
// independent of any diff. Exit code: 0 = ok, 2 = error.
func cmdLs(args []string) int {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	flagSelect := fs.String("select", "", "show only these rule codes (comma-separated; replaces config rules)")
	flagExtendSelect := fs.String("extend-select", "", "add rule codes to the active set (comma-separated)")
	flagType := fs.String("type", "", "filter to a specific type")
	flagConfig := fs.String("config", "", "explicit config file path")
	flagJSON := fs.Bool("json", false, "machine-readable JSON output")
	color := colorAuto
	registerColorFlags(fs, &color)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: rpt ls [flags] [path...]

List, for each active rule, the in-scope files in the working tree.

Unlike 'rpt diff'/'rpt show', this is not tied to a diff: it walks the whole
working tree (honouring revparrot.toml include/exclude), or only the given
paths, and reports every file each rule applies to. Rules with no in-scope
files are omitted.

With no paths, the tree is walked from the revparrot.toml directory. Pass
paths (files or directories) to restrict the listing to a specific set.

Flags:
  --json                  machine-readable JSON output
  -select <codes>         show only these rule codes (replaces config rules)
  -extend-select <codes>  add rule codes to the active set
  -type <name>            filter to a specific type
  -config <path>          explicit config file path
  --color                 force color output (alias: --colour)
  --no-color              disable color output (alias: --no-colour)

Exit codes:
  0  ok
  2  error
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	paths := fs.Args()
	selectList := splitComma(*flagSelect)
	extendList := splitComma(*flagExtendSelect)

	// Load config. Without selection flags, a missing config is an error (same
	// policy as `rpt diff`).
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
				if len(selectList) == 0 && len(extendList) == 0 {
					fmt.Fprintln(os.Stderr, "no revparrot.toml found")
					return 2
				}
				cfg = nil
			} else {
				fmt.Fprintln(os.Stderr, "error loading config:", err)
				return 2
			}
		} else {
			cfgRoot = filepath.Dir(cfgPath)
		}
	}

	activeRules, err := rpconfig.ResolveRuleset(cfg, selectList, extendList)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}

	var matcher *rpconfig.Matcher
	if cfg != nil {
		matcher, err = cfg.NewMatcher()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
	}

	var groups []ruleFileGroup
	if len(activeRules) > 0 {
		// Walk from the config directory when we have one, else the cwd.
		walkRoot := cfgRoot
		if walkRoot == "" {
			cwd, werr := os.Getwd()
			if werr != nil {
				fmt.Fprintln(os.Stderr, "error:", werr)
				return 2
			}
			walkRoot = cwd
		}
		files, ferr := lsCandidateFiles(walkRoot, paths, matcher)
		if ferr != nil {
			fmt.Fprintln(os.Stderr, "error:", ferr)
			return 2
		}
		groups = ruleFileGroups(activeRules, files, matcher, cfgRoot, *flagType)
	}

	if *flagJSON {
		return printLsJSON(os.Stdout, cfgRoot, groups)
	}
	if len(activeRules) == 0 {
		fmt.Println("No active rules.")
		return 0
	}
	printRuleFileGroups(os.Stdout, groups, defaultViolationStyles(color))
	if len(groups) == 0 {
		fmt.Println("No rules have files in scope.")
	}
	return 0
}

// lsCandidateFiles returns the absolute paths of files to consider for `rpt ls`.
// With no paths it walks root (applying the matcher's global scope and pruning
// version-control directories). With paths, each is resolved relative to the
// current directory: a directory is walked, a file is included directly.
// Results are deduplicated and sorted.
func lsCandidateFiles(root string, paths []string, matcher *rpconfig.Matcher) ([]string, error) {
	opts := walkOptions(matcher, root)
	seen := make(map[string]bool)
	var out []string
	add := func(abs string) {
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	walk := func(dir string) error {
		return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if opts.KeepDir != nil && !opts.KeepDir(path) {
					return filepath.SkipDir
				}
				return nil
			}
			if opts.KeepFile != nil && !opts.KeepFile(path) {
				return nil
			}
			add(path)
			return nil
		})
	}

	if len(paths) == 0 {
		if err := walk(root); err != nil {
			return nil, err
		}
	} else {
		for _, p := range paths {
			abs, err := filepath.Abs(p)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(abs)
			if err != nil {
				return nil, err
			}
			if info.IsDir() {
				if err := walk(abs); err != nil {
					return nil, err
				}
			} else {
				add(abs)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// lsJSONRule is the JSON encoding of a rule and its in-scope files.
type lsJSONRule struct {
	Code        string   `json:"code"`
	Type        string   `json:"type,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Model       string   `json:"model,omitempty"`
	Effort      string   `json:"effort,omitempty"`
	Files       []string `json:"files"`
}

// lsJSONOutput is the top-level `rpt ls --json` document: rule-centric, each
// rule carrying its metadata and the files it applies to.
type lsJSONOutput struct {
	Root  string       `json:"root"`
	Rules []lsJSONRule `json:"rules"`
}

// lsOutput builds the rule-centric JSON document from rule/file groups.
func lsOutput(root string, groups []ruleFileGroup) lsJSONOutput {
	out := lsJSONOutput{Root: root, Rules: []lsJSONRule{}}
	for _, g := range groups {
		files := g.files
		if files == nil {
			files = []string{}
		}
		out.Rules = append(out.Rules, lsJSONRule{
			Code:        g.rule.Code,
			Type:        g.rule.Type,
			Title:       g.rule.Title,
			Description: strings.TrimSpace(g.rule.Description),
			Model:       g.rule.Model,
			Effort:      g.rule.Effort,
			Files:       files,
		})
	}
	return out
}

// printLsJSON writes the rule-centric JSON document to w.
func printLsJSON(w io.Writer, root string, groups []ruleFileGroup) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(lsOutput(root, groups)); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
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

// splitComma splits a comma-separated flag value into trimmed, non-empty tokens.
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// builtinActiveSet returns a map keyed by the built-in rule codes present in rules.
func builtinActiveSet(rules []rpconfig.Rule) map[string]bool {
	m := make(map[string]bool)
	for _, r := range rules {
		if rpconfig.IsBuiltinCode(r.Code) {
			m[r.Code] = true
		}
	}
	return m
}

// fileLineKey uniquely identifies a file:line position for set membership tests.
type fileLineKey struct {
	file string
	line int
}

// gatherSyntaxViolations runs a loose RPT scan (with NORPT suppression) and
// returns rpt-syntax violations for matches not present in strictSet (which
// contains the well-formed annotations found by a prior strict scan).
func gatherSyntaxViolations(paths []string, opts scanner.WalkOptions, strictSet map[fileLineKey]bool, matcher *rpconfig.Matcher, cfgRoot string) ([]scanner.Violation, error) {
	looseRPT := scanner.Marker{Keyword: "RPT", Strict: false, Suppress: "NORPT"}
	var out []scanner.Violation
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		var ms []scanner.Match
		if info.IsDir() {
			ms, err = scanner.ScanDirMarkers(path, []scanner.Marker{looseRPT}, opts)
		} else {
			ms, err = scanner.ScanFileMarkers(path, []scanner.Marker{looseRPT})
		}
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", path, err)
		}
		for _, m := range ms {
			if strictSet[fileLineKey{m.File, m.Line}] {
				continue // well-formed annotation already reported
			}
			if matcher != nil && !matcher.Keep(rpconfig.CodeRPTSyntax, relTo(cfgRoot, m.File)) {
				continue
			}
			out = append(out, scanner.Violation{
				File:    m.File,
				Line:    m.Line,
				Code:    rpconfig.CodeRPTSyntax,
				Message: "RPT annotation does not match expected format",
			})
		}
	}
	return out, nil
}

// fileInScopeForRule reports whether the given relative path falls within a
// rule's scope. For built-in rules (no per-rule globs) the global scope filter
// applies; for config rules the per-rule include/exclude also applies.
func fileInScopeForRule(r rpconfig.Rule, rel string, matcher *rpconfig.Matcher) bool {
	if matcher == nil {
		return true
	}
	if rpconfig.IsBuiltinCode(r.Code) {
		return matcher.InScope(rel) && !matcher.Ignored(r.Code, rel)
	}
	return matcher.InScope(rel) && matcher.RuleApplies(r.Code, rel) && !matcher.Ignored(r.Code, rel)
}
