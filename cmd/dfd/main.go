package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/internal/tui"
	"github.com/user/diffyduck/pkg/branches"
	"github.com/user/diffyduck/pkg/comments"
	"github.com/user/diffyduck/pkg/config"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/status"
	"golang.org/x/term"
)

// version is set at build time via -ldflags "-X main.version=...".
// When not set, versionString() enriches it with VCS info from debug.BuildInfo.
var version = "dev"

// versionString returns the version string for display.
// If version was set via ldflags, returns it as-is.
// Otherwise, appends VCS revision and date from Go's embedded build info.
func versionString() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	// If installed via "go install module@version", use the module version.
	// Skip pseudo-versions (v0.0.0-...) and (devel).
	if v := info.Main.Version; v != "" && v != "(devel)" && !strings.HasPrefix(v, "v0.0.0-") {
		return v
	}

	// Fall back to VCS info embedded by go build
	var rev, date string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			date = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return version
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	// Trim time to just the date
	if i := strings.Index(date, "T"); i > 0 {
		date = date[:i]
	}
	if dirty {
		return fmt.Sprintf("dev (%s, %s, dirty)", rev, date)
	}
	return fmt.Sprintf("dev (%s, %s)", rev, date)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// parsedArgs contains parsed command line arguments.
type parsedArgs struct {
	cmd      string   // "diff", "show", "log", "clean", "branches", "config"
	refs     []string // 0-2 refs for diff, 0-1 for show/log (before --)
	paths    []string // file paths (after --)
	excludes []string // --exclude/-e glob patterns

	// diff-specific
	cached    bool  // --cached/--staged
	allMode   bool  // --all/-a
	unstaged  bool  // --unstaged
	snapshots *bool // nil=default, true=--snapshots, false=--no-snapshots

	// log-specific
	count int // -n <count>, 0 = unlimited

	// status-specific
	symbols        int    // -S/--symbols [N], -1 = not set, 0 = unlimited, >0 = max per file
	untrackedFiles string // --untracked-files/-u: "all" (default), "normal", "no"
	showBranches   bool   // --branches/-b: show branch tree

	// branches-specific
	verbose bool   // -v/--verbose
	since   string // --since duration (e.g. "30d", "2w", "3m", "all")

	// comment-specific
	commentSub         string // "list" or "edit"
	commentID          string // comment ID for edit
	commentN           int    // -n count: positive=newest, negative=oldest, 0=uncapped
	commentNSet        bool   // true if -n was explicitly passed
	commentStatus      string // --status: "unresolved" (default), "resolved", "all"
	commentOneline     bool   // --oneline: compact single-line output
	commentAllBranches bool   // --all-branches: show comments from all branches

	// config-specific
	configInit  bool // --init
	configForce bool // --force
	configPrint bool // --print
	configPath  bool // --path
	configEdit  bool // --edit

	// global
	debug      bool
	cpuProfile string

	// help/version
	showHelp    bool
	showVersion bool
	helpCmd     string // subcommand for targeted help (e.g. "diff")

	// Derived (set by deriveMode after parsing)
	mode content.Mode
	ref1 string
	ref2 string
}

// expandAlias maps single-letter subcommand aliases to their canonical names.
func expandAlias(s string) string {
	switch s {
	case "d":
		return "diff"
	case "l":
		return "log"
	case "b":
		return "branches"
	case "s":
		return "status"
	case "c":
		return "comment"
	default:
		return s
	}
}

// parseArgs parses command line arguments into structured fields.
// Unknown flags produce an error. No arguments are passed through to git verbatim.
func parseArgs(args []string) (parsedArgs, error) {
	result := parsedArgs{cmd: "diff", symbols: -1, untrackedFiles: "all"}

	// Consume subcommand if present
	remaining := args
	if len(remaining) > 0 {
		switch remaining[0] {
		case "diff", "d", "show", "log", "l", "clean", "branches", "b", "config", "status", "s", "comment", "c":
			result.cmd = expandAlias(remaining[0])
			result.helpCmd = result.cmd // target for --help flag
			remaining = remaining[1:]

			// Consume comment sub-subcommand and ID
			if result.cmd == "comment" && len(remaining) > 0 {
				switch remaining[0] {
				case "list", "edit":
					result.commentSub = remaining[0]
					remaining = remaining[1:]
				}
				if result.commentSub == "edit" && len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
					result.commentID = remaining[0]
					remaining = remaining[1:]
				}
			}
		case "help":
			result.showHelp = true
			remaining = remaining[1:]
			// Optional target command: "help diff", "help d", etc.
			if len(remaining) > 0 {
				result.helpCmd = expandAlias(remaining[0])
			}
			return result, nil
		}
	}

	// Single-pass parse of remaining args
	afterSeparator := false
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		// Everything after -- is a path
		if arg == "--" {
			afterSeparator = true
			continue
		}
		if afterSeparator {
			result.paths = append(result.paths, arg)
			continue
		}

		// Flags
		if len(arg) > 0 && arg[0] == '-' {
			consumed, err := result.parseFlag(arg, remaining, i)
			if err != nil {
				return parsedArgs{}, err
			}
			i += consumed // skip consumed value args
			continue
		}

		// Non-flag, non-path: it's a ref
		result.refs = append(result.refs, arg)
	}

	// Short-circuit: skip validation for help/version
	if result.showHelp || result.showVersion {
		return result, nil
	}

	// Validate and derive mode
	if err := result.validate(); err != nil {
		return parsedArgs{}, err
	}
	result.deriveMode()

	return result, nil
}

// parseFlag handles a single flag argument. Returns the number of extra args consumed
// (0 for standalone flags, 1 for flags that take a value).
func (p *parsedArgs) parseFlag(arg string, args []string, i int) (int, error) {
	switch {
	// Help and version
	case arg == "--help" || arg == "-h":
		p.showHelp = true
		return 0, nil
	case arg == "--version":
		p.showVersion = true
		return 0, nil

	// Global flags
	case arg == "--debug":
		p.debug = true
	case strings.HasPrefix(arg, "--cpuprofile="):
		p.cpuProfile = strings.TrimPrefix(arg, "--cpuprofile=")
	case arg == "--cpuprofile":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--cpuprofile requires a path argument")
		}
		p.cpuProfile = args[i+1]
		return 1, nil

	// branches flags
	case arg == "-v" || arg == "--verbose":
		p.verbose = true
	case strings.HasPrefix(arg, "--since="):
		p.since = strings.TrimPrefix(arg, "--since=")
	case arg == "--since":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--since requires a duration (e.g. 30d, 2w, 3m, 1y, all)")
		}
		p.since = args[i+1]
		return 1, nil

	// diff flags
	case arg == "--cached" || arg == "--staged":
		p.cached = true
	case arg == "--all" || arg == "-a":
		p.allMode = true
	case arg == "--unstaged":
		p.unstaged = true
	case arg == "--snapshots":
		t := true
		p.snapshots = &t
	case arg == "--no-snapshots":
		f := false
		p.snapshots = &f

	// Exclude: --exclude=<glob>, --exclude <glob>, -e<glob>, -e <glob>
	case strings.HasPrefix(arg, "--exclude="):
		p.excludes = append(p.excludes, strings.TrimPrefix(arg, "--exclude="))
	case arg == "--exclude":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--exclude requires a glob pattern argument")
		}
		p.excludes = append(p.excludes, args[i+1])
		return 1, nil
	case arg == "-e":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("-e requires a glob pattern argument")
		}
		p.excludes = append(p.excludes, args[i+1])
		return 1, nil
	case strings.HasPrefix(arg, "-e"):
		p.excludes = append(p.excludes, strings.TrimPrefix(arg, "-e"))

	// Count: -n <count>, -n<count>
	case arg == "-n":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("-n requires a count argument")
		}
		if p.cmd == "comment" {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, fmt.Errorf("-n requires an integer, got %q", args[i+1])
			}
			p.commentN = n
			p.commentNSet = true
			return 1, nil
		}
		n, err := strconv.Atoi(args[i+1])
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("-n requires a positive integer, got %q", args[i+1])
		}
		p.count = n
		return 1, nil
	case strings.HasPrefix(arg, "-n"):
		nStr := strings.TrimPrefix(arg, "-n")
		if p.cmd == "comment" {
			n, err := strconv.Atoi(nStr)
			if err != nil {
				return 0, fmt.Errorf("-n requires an integer, got %q", nStr)
			}
			p.commentN = n
			p.commentNSet = true
			break
		}
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("-n requires a positive integer, got %q", nStr)
		}
		p.count = n

	// Symbols: -S [N], --symbols [N], --symbols=N
	case arg == "-S" || arg == "--symbols":
		// Default to 5 per file; consume optional integer argument
		p.symbols = 5
		if i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				if n <= 0 {
					p.symbols = 2000
				} else if n > 2000 {
					p.symbols = 2000
				} else {
					p.symbols = n
				}
				return 1, nil
			}
		}
	case strings.HasPrefix(arg, "--symbols="):
		nStr := strings.TrimPrefix(arg, "--symbols=")
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return 0, fmt.Errorf("--symbols requires an integer, got %q", nStr)
		}
		if n <= 0 {
			p.symbols = 2000
		} else if n > 2000 {
			p.symbols = 2000
		} else {
			p.symbols = n
		}

	// Untracked files: -u<mode>, --untracked-files=<mode>
	// Modes: no, normal, all (default: all)
	case arg == "--untracked-files":
		// Bare flag without value means "all"
		p.untrackedFiles = "all"
	case strings.HasPrefix(arg, "--untracked-files="):
		mode := strings.TrimPrefix(arg, "--untracked-files=")
		switch mode {
		case "no", "normal", "all":
			p.untrackedFiles = mode
		default:
			return 0, fmt.Errorf("--untracked-files must be no, normal, or all; got %q", mode)
		}
	case arg == "-u":
		p.untrackedFiles = "all"
	case strings.HasPrefix(arg, "-u"):
		mode := strings.TrimPrefix(arg, "-u")
		switch mode {
		case "no", "normal", "all":
			p.untrackedFiles = mode
		default:
			return 0, fmt.Errorf("-u mode must be no, normal, or all; got %q", mode)
		}

	// status: show branches
	case arg == "--branches" || arg == "-b":
		p.showBranches = true

	// comment flags
	case arg == "--oneline":
		p.commentOneline = true
	case arg == "--all-branches":
		p.commentAllBranches = true
	case arg == "--status":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--status requires a value (unresolved, resolved, all)")
		}
		switch args[i+1] {
		case "unresolved", "resolved", "all":
			p.commentStatus = args[i+1]
		default:
			return 0, fmt.Errorf("--status must be unresolved, resolved, or all; got %q", args[i+1])
		}
		return 1, nil
	case strings.HasPrefix(arg, "--status="):
		val := strings.TrimPrefix(arg, "--status=")
		switch val {
		case "unresolved", "resolved", "all":
			p.commentStatus = val
		default:
			return 0, fmt.Errorf("--status must be unresolved, resolved, or all; got %q", val)
		}

	// config flags
	case arg == "--init":
		p.configInit = true
	case arg == "--force":
		p.configForce = true
	case arg == "--print":
		p.configPrint = true
	case arg == "--path":
		p.configPath = true
	case arg == "--edit":
		p.configEdit = true

	default:
		return 0, fmt.Errorf("unknown flag: %s", arg)
	}
	return 0, nil
}

// validate checks for invalid flag combinations and ref count limits.
func (p *parsedArgs) validate() error {
	switch p.cmd {
	case "diff":
		if len(p.refs) > 2 {
			return fmt.Errorf("diff accepts at most 2 refs, got %d", len(p.refs))
		}
		if p.cached && p.unstaged {
			return fmt.Errorf("--cached and --unstaged cannot be used together")
		}
		if p.cached && len(p.refs) > 0 {
			return fmt.Errorf("--cached cannot be used with ref arguments")
		}
		if p.count > 0 {
			return fmt.Errorf("-n is only valid for log command")
		}
		if p.verbose {
			return fmt.Errorf("-v is only valid for branches command")
		}
		if p.symbols >= 0 || p.untrackedFiles != "all" {
			return fmt.Errorf("-S/--symbols and -u/--untracked-files are only valid for status command")
		}
		if p.showBranches {
			return fmt.Errorf("-b/--branches is only valid for status command")
		}
	case "show":
		if len(p.refs) > 1 {
			return fmt.Errorf("show accepts at most 1 ref, got %d", len(p.refs))
		}
		if p.cached || p.unstaged || p.allMode {
			return fmt.Errorf("--cached, --unstaged, and --all are only valid for diff command")
		}
		if p.snapshots != nil {
			return fmt.Errorf("--snapshots/--no-snapshots are only valid for diff command")
		}
		if p.count > 0 {
			return fmt.Errorf("-n is only valid for log command")
		}
		if p.verbose {
			return fmt.Errorf("-v is only valid for branches command")
		}
		if p.symbols >= 0 || p.untrackedFiles != "all" {
			return fmt.Errorf("-S/--symbols and -u/--untracked-files are only valid for status command")
		}
		if p.showBranches {
			return fmt.Errorf("-b/--branches is only valid for status command")
		}
	case "log":
		if len(p.refs) > 1 {
			return fmt.Errorf("log accepts at most 1 ref range, got %d", len(p.refs))
		}
		if p.cached || p.unstaged || p.allMode {
			return fmt.Errorf("--cached, --unstaged, and --all are only valid for diff command")
		}
		if p.snapshots != nil {
			return fmt.Errorf("--snapshots/--no-snapshots are only valid for diff command")
		}
		if p.verbose {
			return fmt.Errorf("-v is only valid for branches command")
		}
		if p.symbols >= 0 || p.untrackedFiles != "all" {
			return fmt.Errorf("-S/--symbols and -u/--untracked-files are only valid for status command")
		}
		if p.showBranches {
			return fmt.Errorf("-b/--branches is only valid for status command")
		}
	case "clean":
		if len(p.refs) > 0 || len(p.paths) > 0 || len(p.excludes) > 0 {
			return fmt.Errorf("%s does not accept arguments", p.cmd)
		}
		if p.cached || p.unstaged || p.allMode || p.count > 0 || p.verbose || p.symbols >= 0 || p.untrackedFiles != "all" || p.showBranches {
			return fmt.Errorf("%s does not accept flags", p.cmd)
		}
	case "branches":
		if len(p.refs) > 0 || len(p.paths) > 0 || len(p.excludes) > 0 {
			return fmt.Errorf("branches does not accept arguments")
		}
		if p.cached || p.unstaged || p.allMode || p.count > 0 || p.symbols >= 0 || p.untrackedFiles != "all" || p.showBranches {
			return fmt.Errorf("branches only accepts -v/--verbose and --since")
		}
	case "status":
		if len(p.refs) > 0 || len(p.paths) > 0 || len(p.excludes) > 0 {
			return fmt.Errorf("status does not accept arguments")
		}
		if p.cached || p.unstaged || p.count > 0 || p.verbose || p.allMode {
			return fmt.Errorf("status only accepts -S/--symbols, -u/--untracked-files, and -b/--branches")
		}
	case "comment":
		if len(p.refs) > 0 || len(p.paths) > 0 || len(p.excludes) > 0 {
			return fmt.Errorf("comment does not accept ref or path arguments")
		}
		if p.cached || p.unstaged || p.allMode || p.count > 0 || p.verbose || p.symbols >= 0 || p.untrackedFiles != "all" || p.showBranches {
			return fmt.Errorf("comment only accepts -n and --status")
		}
		if p.commentSub == "edit" && p.commentID == "" {
			return fmt.Errorf("comment edit requires a comment ID")
		}
	case "config":
		if len(p.refs) > 0 || len(p.paths) > 0 || len(p.excludes) > 0 {
			return fmt.Errorf("config does not accept arguments")
		}
		if p.cached || p.unstaged || p.allMode || p.count > 0 || p.symbols >= 0 || p.untrackedFiles != "all" || p.showBranches {
			return fmt.Errorf("config does not accept diff/log flags")
		}
		if p.configForce && !p.configInit {
			return fmt.Errorf("--force can only be used with --init")
		}
		if p.configInit && p.configPath {
			return fmt.Errorf("--init and --path cannot be used together")
		}
		if p.configPrint && p.configPath {
			return fmt.Errorf("--print and --path cannot be used together")
		}
		if p.configEdit && (p.configInit || p.configPrint || p.configPath) {
			return fmt.Errorf("--edit cannot be combined with --init, --print, or --path")
		}
	}

	// Config-specific flags are only valid for the config command
	if p.cmd != "config" && (p.configInit || p.configForce || p.configPrint || p.configPath || p.configEdit) {
		return fmt.Errorf("--init, --force, --print, --path, --edit are only valid for config command")
	}

	// --since is only valid for branches
	if p.cmd != "branches" && p.since != "" {
		return fmt.Errorf("--since is only valid for branches command")
	}

	// --status and --oneline are only valid for comment
	if p.cmd != "comment" && p.commentStatus != "" {
		return fmt.Errorf("--status is only valid for comment command")
	}
	if p.cmd != "comment" && p.commentOneline {
		return fmt.Errorf("--oneline is only valid for comment command")
	}
	if p.cmd != "comment" && p.commentAllBranches {
		return fmt.Errorf("--all-branches is only valid for comment command")
	}
	return nil
}

// deriveMode sets the mode, ref1, and ref2 fields based on parsed args.
func (p *parsedArgs) deriveMode() {
	switch p.cmd {
	case "show", "log":
		p.mode = content.ModeShow
		if len(p.refs) > 0 {
			p.ref1 = p.refs[0]
		} else {
			p.ref1 = "HEAD"
		}
	case "diff":
		if p.cached {
			p.mode = content.ModeDiffCached
		} else if len(p.refs) >= 2 {
			p.mode = content.ModeDiffRefs
			p.ref1 = p.refs[0]
			p.ref2 = p.refs[1]
		} else if len(p.refs) == 1 {
			p.mode = content.ModeDiffRefs
			p.ref1 = p.refs[0]
			p.ref2 = "" // compare to working tree
		} else {
			p.mode = content.ModeDiffUnstaged
		}
	}
}

// workingTreeInvolved returns true if the diff involves the current working tree.
func workingTreeInvolved(args parsedArgs) bool {
	switch args.mode {
	case content.ModeDiffUnstaged:
		return true
	case content.ModeDiffCached:
		return true
	case content.ModeDiffRefs:
		return args.ref2 == ""
	default:
		return false
	}
}

// buildPathspec constructs git pathspec arguments from paths and excludes.
// Returns nil if there are no paths or excludes.
func buildPathspec(paths, excludes []string) []string {
	if len(paths) == 0 && len(excludes) == 0 {
		return nil
	}
	result := []string{"--"}
	result = append(result, paths...)
	for _, e := range excludes {
		result = append(result, ":!"+e)
	}
	return result
}

// filterFiles filters a file list by include paths and exclude patterns.
// If paths is empty, all files are included. Excludes remove matching files.
// Used to filter untracked file lists in --all mode.
func filterFiles(files []string, paths, excludes []string) []string {
	if len(paths) == 0 && len(excludes) == 0 {
		return files
	}
	var result []string
	for _, f := range files {
		if len(paths) > 0 && !matchesAnyPath(f, paths) {
			continue
		}
		if matchesAnyExclude(f, excludes) {
			continue
		}
		result = append(result, f)
	}
	return result
}

// matchesAnyPath returns true if the file matches any of the include paths.
// Supports directory prefixes (e.g. "src/") and glob patterns (e.g. "*.go").
func matchesAnyPath(file string, paths []string) bool {
	for _, p := range paths {
		// Directory prefix: "src/" matches "src/foo.go"
		if strings.HasSuffix(p, "/") && strings.HasPrefix(file, p) {
			return true
		}
		// Exact match
		if file == p {
			return true
		}
		// Glob match on the basename
		if matched, _ := filepath.Match(p, filepath.Base(file)); matched {
			return true
		}
		// Glob match on the full path
		if matched, _ := filepath.Match(p, file); matched {
			return true
		}
	}
	return false
}

// matchesAnyExclude returns true if the file matches any exclude pattern.
// Supports glob patterns and ** prefix matching (e.g. "vendor/**" matches "vendor/foo/bar.go").
func matchesAnyExclude(file string, excludes []string) bool {
	for _, e := range excludes {
		// Handle ** patterns as prefix match: "vendor/**" → prefix "vendor/"
		if strings.HasSuffix(e, "/**") {
			prefix := strings.TrimSuffix(e, "/**") + "/"
			if strings.HasPrefix(file, prefix) {
				return true
			}
			continue
		}
		// Glob match on basename
		if matched, _ := filepath.Match(e, filepath.Base(file)); matched {
			return true
		}
		// Glob match on full path
		if matched, _ := filepath.Match(e, file); matched {
			return true
		}
	}
	return false
}

// printUsage prints usage information. If cmd is non-empty, prints
// subcommand-specific help; otherwise prints general usage.
func printUsage(cmd string) {
	switch cmd {
	case "diff":
		fmt.Print(usageDiff)
	case "show":
		fmt.Print(usageShow)
	case "log":
		fmt.Print(usageLog)
	case "clean":
		fmt.Print(usageClean)
	case "branches":
		fmt.Print(usageBranches)
	case "status":
		fmt.Print(usageStatus)
	case "config":
		fmt.Print(usageConfig)
	case "comment":
		fmt.Print(usageComment)
	default:
		fmt.Print(usageGeneral)
	}
}

const usageGeneral = `dfd - terminal side-by-side diff viewer

Usage:
  dfd [flags] [refs] [-- paths]
  dfd <command> [flags] [args]

Commands:
  diff, d    Compare changes (default)
  show       Show a commit
  log, l     Browse commit history
  clean      Delete persisted snapshots
  branches, b
             Show branch dependency tree
  status, s  Show rich working tree status
  comment, c List and edit comments
  config     Manage configuration

Global flags:
  -h, --help       Show help
      --version    Show version

Diff flags:
      --cached     Diff staged changes (alias: --staged)
      --unstaged   Diff unstaged changes only
  -a, --all        Include untracked files
      --snapshots  Show snapshot history view
      --no-snapshots
                   Disable taking snapshots
  -e, --exclude <glob>
                   Exclude files matching glob (repeatable)

Log flags:
  -n <count>       Limit number of commits

Use "dfd help <command>" for more about a command.
Press C-h inside dfd for keybinding help.
`

const usageDiff = `dfd diff - compare changes

Usage:
  dfd [diff] [flags] [<ref>] [-- <path>...]
  dfd [diff] [flags] <ref1> <ref2> [-- <path>...]

With no refs, diffs HEAD against the working tree.
With one ref, diffs that ref against the working tree.
With two refs, diffs ref1 against ref2.

Flags:
      --cached     Diff staged changes (alias: --staged)
      --unstaged   Diff unstaged changes only
  -a, --all        Include untracked files
      --snapshots  Show snapshot history view
      --no-snapshots
                   Disable taking snapshots
  -e, --exclude <glob>
                   Exclude files matching glob (repeatable)

Examples:
  dfd                        Diff HEAD vs working tree
  dfd --cached               Diff staged changes
  dfd HEAD~3                 Diff HEAD~3 vs working tree
  dfd main feature           Diff main vs feature
  dfd -e vendor/** -- src/   Only src/, excluding vendor/
`

const usageShow = `dfd show - show a commit

Usage:
  dfd show [<ref>] [-- <path>...]

Displays the diff for a single commit. Defaults to HEAD.

Examples:
  dfd show                   Show HEAD
  dfd show abc1234           Show specific commit
  dfd show HEAD~2 -- src/    Show commit, filtered to src/
`

const usageLog = `dfd log - browse commit history

Usage:
  dfd log [flags] [<ref-range>] [-- <path>...]

Browse commits interactively. Supports ref ranges (e.g. main..feature).

Flags:
  -n <count>       Limit number of commits
  -e, --exclude <glob>
                   Exclude files matching glob (repeatable)

Examples:
  dfd log                    Browse recent commits
  dfd log -n 20              Browse last 20 commits
  dfd log main..feature      Commits in feature not in main
  dfd log -- src/            Commits touching src/
`

const usageClean = `dfd clean - delete persisted snapshots

Usage:
  dfd clean

Removes all persisted snapshot refs (refs/dfd/snapshots/*) from the
current repository.
`

const usageBranches = `dfd branches - show branch dependency tree

Usage:
  dfd branches [flags]

Displays local branches as a tree based on their upstream relationships.
By default, only branches active in the last 30 days are shown. The current
branch, default branch, and worktree branches are always included.

Flags:
  -v, --verbose          Show commit subject for each branch
      --since <duration> Only show branches active within duration (default: 30d)
                         Accepts: 7d (days), 2w (weeks), 3m (months), 1y (years), all

Examples:
  dfd branches                Show branch tree (last 30 days)
  dfd branches --since=1y     Show branches from the last year
  dfd branches --since=all    Show all branches
  dfd branches -v             Show branch tree with commit subjects
`

const usageStatus = `dfd status - show rich working tree status

Usage:
  dfd status [-S [N]] [-u<mode>] [-b]

Flags:
  -S, --symbols [N]              Show structural diffs (functions, types) per file
                                 N = max symbols per file (default 5, 0 = unlimited)
  -u, --untracked-files[=<mode>] Show untracked files (default: all)
                                 no     = hide untracked files
                                 normal = list paths only
                                 all    = expand with diffs (default)
  -b, --branches                 Include branch tree

Displays staged/unstaged changes and untracked files.
Untracked files are expanded with diffs by default; use -uno or
-unormal to just list paths or hide them entirely.
With -S, each changed file shows affected functions, methods, and types
with per-element line counts.
With -b, the branch tree is shown above the changes.

Examples:
  dfd status               Show working tree status (untracked expanded)
  dfd status -b            Include branch tree
  dfd status -S            Include structural diffs (5 per file)
  dfd status -S 10         Include structural diffs (10 per file)
  dfd status -uno          Hide untracked files
  dfd status -unormal      List untracked paths only
  dfd s                    Same (short alias)
`

const usageConfig = `dfd config - manage configuration

Usage:
  dfd config [flags]

With no flags, prints the default configuration to stdout.
Config file location: ~/.config/dfd/config.toml (or $XDG_CONFIG_HOME/dfd/).

Flags:
      --init       Write default config file
      --force      Overwrite existing file (use with --init)
      --print      Print default config to stdout (default behavior)
      --path       Print config file path
      --edit       Open config file in $VISUAL or $EDITOR

Examples:
  dfd config                   Print default config
  dfd config --init            Create config file with defaults
  dfd config --init --force    Overwrite existing config file
  dfd config --path            Show config file location
  dfd config --edit            Edit config in your editor
`

const usageComment = `dfd comment - manage comments

Usage:
  dfd comment list [flags]
  dfd comment edit <id>

Sub-commands:
  list       List comments (default: 5 newest unresolved)
  edit       Open a comment in $EDITOR

List flags:
  -n <count>       Positive: newest N, negative: oldest |N|, 0: uncapped (default: 5)
      --status <s> Filter: unresolved (default), resolved, all
      --oneline    Compact single-line output per comment
      --all-branches
                   Show comments from all branches (default: current branch only)

Examples:
  dfd comment list                    Show 5 newest unresolved
  dfd comment list -n 10              Show 10 newest unresolved
  dfd comment list -n -3              Show 3 oldest unresolved
  dfd comment list -n 0               Show all unresolved
  dfd comment list --status all       Include resolved
  dfd comment list --status resolved  Only resolved
  dfd comment list --oneline          Compact output
  dfd comment edit <id>               Edit in $EDITOR
  dfd c list                          Short alias
`

func run() error {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		return fmt.Errorf("%w\nRun 'dfd --help' for usage.", err)
	}

	if args.showVersion {
		fmt.Printf("dfd %s\n", versionString())
		return nil
	}
	if args.showHelp {
		printUsage(args.helpCmd)
		return nil
	}

	// Load user config (missing file is fine — returns zero config)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Start CPU profiling if requested
	if args.cpuProfile != "" {
		f, err := os.Create(args.cpuProfile)
		if err != nil {
			return fmt.Errorf("create cpu profile: %w", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("start cpu profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Handle comment command - list and edit comments
	if args.cmd == "comment" {
		return runComment(args)
	}

	// Handle clean command - deletes all persisted snapshot refs
	if args.cmd == "clean" {
		return runClean()
	}

	// Handle branches command - show branch dependency tree
	if args.cmd == "branches" {
		return runBranches(args.verbose, args.since)
	}

	// Handle status command - rich working tree status
	if args.cmd == "status" {
		maxSymbols := 0 // default: no symbols
		if args.symbols >= 0 {
			maxSymbols = args.symbols
		}
		return runStatus(args.untrackedFiles, maxSymbols, args.showBranches)
	}

	// Handle config command
	if args.cmd == "config" {
		return runConfig(args)
	}

	// Handle log command separately
	if args.cmd == "log" {
		return runLogMode(cfg, args)
	}

	g := git.New()

	// Run async expiry of old snapshot refs (doesn't block startup)
	go func() {
		_, _ = g.ExpireOldSnapshotRefs(14)
	}()

	// Build pathspec for git commands
	pathspec := buildPathspec(args.paths, args.excludes)

	// Determine base ref and branch for the diff (used for snapshot keying)
	var baseSHA, branch string
	if args.cmd == "diff" && !args.unstaged {
		baseRef := args.ref1
		if baseRef == "" {
			baseRef = "HEAD"
		}
		sha, err := g.Show("--format=%H", "-s", baseRef)
		if err == nil {
			baseSHA = strings.TrimSpace(sha)
		}
		if b, err := g.CurrentBranch(); err == nil {
			branch = b
		}
	}

	// Determine if snapshots should be taken (auto_snapshots).
	// --no-snapshots disables taking; auto_snapshots config can also disable.
	autoSnapshotsDisabled := args.snapshots != nil && !*args.snapshots
	if !autoSnapshotsDisabled && args.snapshots == nil && cfg.Features.AutoSnapshots != nil && !*cfg.Features.AutoSnapshots {
		autoSnapshotsDisabled = true
	}
	autoSnapshots := args.cmd == "diff" &&
		!autoSnapshotsDisabled &&
		!args.unstaged &&
		workingTreeInvolved(args)

	// Determine if snapshot view should be shown.
	// --snapshots flag forces show; show_snapshots config can also enable.
	showSnapshots := false
	if args.snapshots != nil && *args.snapshots {
		showSnapshots = true
	} else if args.snapshots == nil && cfg.Features.ShowSnapshots != nil && *cfg.Features.ShowSnapshots {
		showSnapshots = true
	}
	showSnapshots = showSnapshots && autoSnapshots

	// Load persisted snapshot SHAs (needed for the SHA chain regardless of view)
	var snapshotInfos []git.SnapshotInfo
	var persistedSnapshots []string
	if autoSnapshots && baseSHA != "" {
		infos, err := g.ListSnapshotRefs(branch, baseSHA)
		if err == nil {
			snapshotInfos = infos
			persistedSnapshots = make([]string, len(infos))
			for i, info := range infos {
				persistedSnapshots[i] = info.SHA
			}
		}
	}

	// Detect merge/rebase conflict state (used to adjust diff commands and enable
	// conflict marker highlighting in the TUI).
	hasConflicts := g.HasConflicts()

	// Get diff from git, with optional commit metadata
	var output string
	var commitInfo sidebyside.CommitInfo

	switch args.cmd {
	case "diff":
		if args.allMode {
			output, err = getDiffAll(g, "HEAD", args.paths, args.excludes)
			if err != nil {
				return err
			}
			args.mode = content.ModeDiffRefs
			args.ref1 = "HEAD"
			args.ref2 = ""
		} else if args.unstaged {
			// During merge/rebase conflicts, bare "git diff" produces combined
			// diff format for unmerged files which the parser can't handle.
			// Fall back to "git diff HEAD" which gives standard unified diff.
			if hasConflicts {
				diffArgs := []string{"HEAD"}
				diffArgs = append(diffArgs, pathspec...)
				output, err = g.Diff(diffArgs...)
				if err != nil {
					return fmt.Errorf("git diff: %w", err)
				}
				args.mode = content.ModeDiffRefs
				args.ref1 = "HEAD"
				args.ref2 = ""
			} else {
				diffArgs := append([]string{}, pathspec...)
				output, err = g.Diff(diffArgs...)
				if err != nil {
					return fmt.Errorf("git diff: %w", err)
				}
			}
		} else {
			if args.cached {
				diffArgs := []string{"--cached"}
				diffArgs = append(diffArgs, pathspec...)
				output, err = g.Diff(diffArgs...)
			} else if args.ref1 == "" && args.mode == content.ModeDiffUnstaged {
				diffArgs := []string{"HEAD"}
				diffArgs = append(diffArgs, pathspec...)
				output, err = g.Diff(diffArgs...)
				args.mode = content.ModeDiffRefs
				args.ref1 = "HEAD"
				args.ref2 = ""
			} else {
				diffArgs := append([]string{}, args.refs...)
				diffArgs = append(diffArgs, pathspec...)
				output, err = g.Diff(diffArgs...)
			}
			if err != nil {
				return fmt.Errorf("git diff: %w", err)
			}
		}
	case "show":
		showArgs := []string{args.ref1}
		showArgs = append(showArgs, pathspec...)
		var meta *git.CommitMeta
		meta, output, err = g.ShowWithMeta(showArgs...)
		if err != nil {
			return fmt.Errorf("git show: %w", err)
		}
		if meta != nil {
			commitInfo = sidebyside.CommitInfo{
				SHA:     meta.SHA,
				Author:  meta.Author,
				Email:   meta.Email,
				Date:    meta.Date,
				Subject: meta.Subject,
				Body:    meta.Body,
				Refs:    sidebyside.ParseRefs(meta.Refs),
			}
		}
	}

	// Parse the diff
	d, err := diff.Parse(output)
	if err != nil {
		return fmt.Errorf("parse diff: %w", err)
	}

	// Build base→WT commit (always shown in normal view)
	var commits []sidebyside.CommitSet

	if len(d.Files) > 0 {
		files, truncatedFileCount := sidebyside.TransformDiff(d)
		commit := sidebyside.CommitSet{
			Info:               commitInfo,
			Files:              files,
			FoldLevel:          sidebyside.CommitFolded,
			FilesLoaded:        true,
			TruncatedFileCount: truncatedFileCount,
		}
		commits = append(commits, commit)
	}

	// Build snapshot view commits (only when showing snapshot view at startup)
	var snapshotViewCommits []sidebyside.CommitSet
	if showSnapshots && len(snapshotInfos) > 0 {
		snapshotViewCommits = buildSnapshotHistory(g, snapshotInfos, baseSHA, persistedSnapshots, pathspec)
	}

	if len(commits) == 0 && len(snapshotViewCommits) == 0 {
		fmt.Println("No changes")
		return nil
	}

	// Create content fetcher for lazy file loading
	fetcher := content.NewFetcher(g, args.mode, args.ref1, args.ref2)

	// Create comment store for persistence
	commentStore := comments.NewStore("")

	// Create and run the TUI (WithConfig first so CLI flags in later Options win)
	opts := []tui.Option{tui.WithConfig(cfg), tui.WithFetcher(fetcher), tui.WithGit(g), tui.WithCommentStore(commentStore)}
	if args.debug {
		opts = append(opts, tui.WithDebugMode())
	}
	if hasConflicts {
		opts = append(opts, tui.WithConflicts())
	}
	if args.allMode {
		opts = append(opts, tui.WithAllMode(true))
	}
	if autoSnapshots {
		opts = append(opts, tui.WithAutoSnapshots(true))
		if len(persistedSnapshots) > 0 {
			opts = append(opts, tui.WithPersistedSnapshots(persistedSnapshots))
		}
	}
	if showSnapshots {
		opts = append(opts, tui.WithShowSnapshots(true))
	}
	if len(snapshotViewCommits) > 0 {
		opts = append(opts, tui.WithSnapshotViewCommits(snapshotViewCommits))
	}
	if baseSHA != "" {
		opts = append(opts, tui.WithBaseSHA(baseSHA))
	}
	if branch != "" {
		opts = append(opts, tui.WithBranch(branch))
	}

	model := tui.NewWithCommits(commits, opts...)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus(), tea.WithMouseCellMotion())

	_, err = p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

// parseSinceDuration parses a human-friendly duration string.
// Accepts: "7d" (days), "2w" (weeks), "3m" (months), "1y" (years), "all" (no filter).
// Returns 0 for "all" or empty string (caller should skip filtering).
func parseSinceDuration(s string) (time.Duration, error) {
	if s == "" || s == "all" {
		return 0, nil
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q: expected number + unit (d/w/m/y)", s)
	}
	numStr := s[:len(s)-1]
	unit := s[len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid duration %q: expected positive number + unit (d/w/m/y)", s)
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit %q: expected d, w, m, or y", string(unit))
	}
}

// filterBranchList applies the recency filter to a branch list.
// dur is the lookback duration; 0 means no filtering.
func filterBranchList(g git.Git, branchList []git.BranchInfo, dur time.Duration) []git.BranchInfo {
	if dur <= 0 {
		return branchList
	}
	since := time.Now().Add(-dur)
	defaultBranch, _ := g.DefaultBranch()
	worktrees, _ := g.WorktreeBranches()
	return branches.FilterBranches(branchList, since, defaultBranch, worktrees)
}

// runBranches prints a tree view of local branch dependencies.
func runBranches(verbose bool, sinceStr string) error {
	g := git.New()
	branchList, err := g.LocalBranches()
	if err != nil {
		return fmt.Errorf("list branches: %w", err)
	}
	if len(branchList) == 0 {
		fmt.Println("No local branches")
		return nil
	}

	dur := 30 * 24 * time.Hour // default: 1 month
	if sinceStr != "" {
		parsed, err := parseSinceDuration(sinceStr)
		if err != nil {
			return fmt.Errorf("invalid --since value: %w", err)
		}
		dur = parsed
	}
	branchList = filterBranchList(g, branchList, dur)
	if len(branchList) == 0 {
		fmt.Println("No local branches")
		return nil
	}

	roots, err := branches.BuildTree(branchList, g)
	if err != nil {
		return fmt.Errorf("build branch tree: %w", err)
	}
	fmt.Print(branches.Render(roots, verbose))
	return nil
}

// runStatus prints a rich working tree status: branch tree, staged/unstaged
// changes with structural diffs, and untracked files.
// untrackedMode is "all" (expand with diffs), "normal" (list paths), or "no" (hide).
func runStatus(untrackedMode string, maxSymbols int, showBranches bool) error {
	g := git.New()
	hl := highlight.New()

	// 1. Branch tree (only with --branches, filtered to last 30 days)
	var branchTree string
	if showBranches {
		branchList, err := g.LocalBranches()
		if err == nil && len(branchList) > 0 {
			branchList = filterBranchList(g, branchList, 30*24*time.Hour)
			if len(branchList) > 0 {
				roots, err := branches.BuildTree(branchList, g)
				if err == nil {
					branchTree = branches.Render(roots, false)
				}
			}
		}
	}

	// Content fetcher using git
	fetchContent := func(ref, path string) (string, error) {
		return g.GetFileContent(ref, path)
	}

	// Working directory for reading unstaged files
	workDir, _ := os.Getwd()
	readFile := status.ReadWorkingFile(workDir)

	// 2. Staged changes
	stagedDiffStr, _ := g.Diff("--cached")
	var stagedChanges []status.FileChange
	if stagedDiffStr != "" {
		parsed, _ := diff.Parse(stagedDiffStr)
		stagedChanges = status.BuildFileChanges(parsed, hl, fetchContent, readFile, true, maxSymbols)
	}

	// 3. Unstaged changes
	unstagedDiffStr, _ := g.Diff()
	var unstagedChanges []status.FileChange
	if unstagedDiffStr != "" {
		parsed, _ := diff.Parse(unstagedDiffStr)
		unstagedChanges = status.BuildFileChanges(parsed, hl, fetchContent, readFile, false, maxSymbols)
	}

	// 4. Untracked files
	var untracked []string
	var untrackedChanges []status.FileChange
	if untrackedMode != "no" {
		untracked, _ = g.ListUntrackedFiles()

		if untrackedMode == "all" && len(untracked) > 0 {
			for _, path := range untracked {
				newDiff, err := g.DiffNewFile(path)
				if err != nil || newDiff == "" {
					continue
				}
				parsed, _ := diff.Parse(newDiff)
				fcs := status.BuildFileChanges(parsed, hl, fetchContent, readFile, false, maxSymbols)
				untrackedChanges = append(untrackedChanges, fcs...)
			}
			untracked = nil // expanded changes replace the plain list
		}
	}

	// 5. Repo state (rebase, merge, etc.)
	repoOp, repoDetail := g.RepoState()

	// Render and print
	fmt.Print(status.Render(branchTree, repoOp, repoDetail, stagedChanges, unstagedChanges, untracked, untrackedChanges))
	return nil
}

// runConfig handles the config subcommand: --init, --print, --path, --edit.
func runConfig(args parsedArgs) error {
	if args.configPath {
		fmt.Println(config.Path())
		return nil
	}

	if args.configEdit {
		return runConfigEdit()
	}

	example := config.GenerateExample(tui.DefaultKeysConfig())

	if args.configInit {
		path := config.Path()

		// Check if file exists (unless --force)
		if !args.configForce {
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config file already exists at %s (use --force to overwrite)", path)
			}
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}

		if err := os.WriteFile(path, []byte(example), 0o644); err != nil {
			return fmt.Errorf("write config file: %w", err)
		}

		fmt.Printf("Wrote default config to %s\n", path)
		return nil
	}

	// Default (no flags or --print): print to stdout
	fmt.Print(example)
	return nil
}

// editorCmd returns the user's preferred editor from $VISUAL or $EDITOR,
// or an empty string if neither is set.
func editorCmd() string {
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return os.Getenv("EDITOR")
}

// runConfigEdit opens the config file in the user's editor.
// Creates the file with defaults if it doesn't exist yet.
func runConfigEdit() error {
	editor := editorCmd()
	if editor == "" {
		return fmt.Errorf("$VISUAL or $EDITOR must be set")
	}

	path := config.Path()

	// Create with defaults if file doesn't exist
	if _, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
		example := config.GenerateExample(tui.DefaultKeysConfig())
		if err := os.WriteFile(path, []byte(example), 0o644); err != nil {
			return fmt.Errorf("write config file: %w", err)
		}
	}

	cmd := exec.Command("sh", "-c", editor+` "$@"`, "--", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runComment dispatches comment sub-commands.
func runComment(args parsedArgs) error {
	switch args.commentSub {
	case "list":
		return runCommentList(args)
	case "edit":
		return runCommentEdit(args.commentID)
	default:
		printUsage("comment")
		return nil
	}
}

// runCommentList lists comments filtered by status and limited by -n.
func runCommentList(args parsedArgs) error {
	store := comments.NewStore("")
	all, err := store.AllComments()
	if err != nil {
		return fmt.Errorf("reading comments: %w", err)
	}

	// Filter by --status (default: unresolved)
	status := args.commentStatus
	if status == "" {
		status = "unresolved"
	}
	if status != "all" {
		var filtered []*comments.Comment
		for _, c := range all {
			if status == "resolved" && c.Resolved {
				filtered = append(filtered, c)
			} else if status == "unresolved" && !c.Resolved {
				filtered = append(filtered, c)
			}
		}
		all = filtered
	}

	// Filter by reachability from HEAD (default: on, --all-branches disables)
	if !args.commentAllBranches {
		reachable, revListErr := store.ReachableCommits("HEAD")
		currentBranch, _ := store.CurrentBranch()

		if revListErr == nil {
			var filtered []*comments.Comment
			for _, c := range all {
				if c.CommitSHA != "" {
					// Has a commit: check if reachable from HEAD
					if reachable[c.CommitSHA] {
						filtered = append(filtered, c)
					}
				} else if c.Branch != "" {
					// Working-tree comment: check branch match
					if c.Branch == currentBranch {
						filtered = append(filtered, c)
					}
				} else {
					// No commit and no branch: include (legacy comment)
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}
	}

	if len(all) == 0 {
		fmt.Println("No comments")
		return nil
	}

	// Sort by Created descending (newest first)
	sort.Slice(all, func(i, j int) bool {
		return all[i].Created.After(all[j].Created)
	})

	// Apply -n limiting
	if !args.commentNSet {
		// Default: 5 newest
		if len(all) > 5 {
			all = all[:5]
		}
	} else if args.commentN == 0 {
		// Uncapped: show all
	} else if args.commentN > 0 {
		if args.commentN < len(all) {
			all = all[:args.commentN]
		}
	} else {
		// Negative: oldest |N|
		count := -args.commentN
		if count < len(all) {
			all = all[len(all)-count:]
		}
		// Reverse so oldest prints first (chronological)
		slices.Reverse(all)
	}

	// Create highlighter for multiline output (reused across comments)
	var h *highlight.Highlighter
	if !args.commentOneline {
		h = highlight.New()
		defer h.Close()
	}

	for i, c := range all {
		if args.commentOneline {
			fmt.Println(formatCommentOneline(c))
		} else {
			if i > 0 {
				fmt.Print("\n\n")
			}
			fmt.Print(formatCommentBlock(c, h))
		}
	}
	return nil
}

// ANSI color codes for comment list output, matching TUI theme defaults.
const (
	cReset       = "\033[0m"
	cBold        = "\033[1m"
	cYellow      = "\033[33m"      // fg=3 - commit SHA
	cGreen       = "\033[38;5;10m" // fg=10 - added/resolved
	cGray        = "\033[38;5;8m"  // fg=8 - labels, dim text
	cCyan        = "\033[36m"      // fg=6 - branch name
	cDirWhite    = "\033[38;5;7m"  // fg=7 - directory part of path
	cBrightWhite = "\033[38;5;15m" // fg=15 - basename, header text
)

// formatCommentOneline formats a single comment as a compact one-liner.
func formatCommentOneline(c *comments.Comment) string {
	// First line of text, truncated to 60 chars
	text := c.Text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	if len(text) > 60 {
		text = text[:57] + "..."
	}

	commitShort := c.CommitSHA
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}
	if commitShort == "" {
		commitShort = "-"
	}

	resolved := ""
	if c.Resolved {
		resolved = cGreen + " [resolved]" + cReset
	}

	return fmt.Sprintf("%s%s%s%s  %s  %s%s%s%s  %s",
		cBrightWhite, cBold, c.ID, cReset,
		styleCommentPath(c.File, c.Line),
		cYellow, commitShort, cReset,
		resolved,
		text)
}

// formatCommentBlock formats a comment as a multiline block showing the full
// serialized patch context and metadata. If h is non-nil, context lines are
// syntax-highlighted based on the comment's file extension.
func formatCommentBlock(c *comments.Comment, h *highlight.Highlighter) string {
	var b strings.Builder

	// Header line
	commitShort := c.CommitSHA
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}
	resolved := ""
	if c.Resolved {
		resolved = cGreen + " [resolved]" + cReset
	}
	fmt.Fprintf(&b, "%sID:%s     %s%s%s%s%s\n",
		cGray, cReset,
		cBrightWhite+cBold, c.ID, cReset,
		resolved, cReset)

	// Metadata
	if commitShort != "" {
		fmt.Fprintf(&b, "%sCommit:%s %s%s%s\n", cGray, cReset, cYellow, commitShort, cReset)
	}
	if c.Branch != "" {
		fmt.Fprintf(&b, "%sBranch:%s %s%s%s\n", cGray, cReset, cCyan, c.Branch, cReset)
	}
	fmt.Fprintf(&b, "%sFile:%s   %s\n", cGray, cReset, styleCommentPath(c.File, c.Line))
	fmt.Fprintf(&b, "%sDate:%s   %s\n", cGray, cReset, c.Created.Format(time.RFC3339))

	// Diff context (with optional syntax highlighting)
	b.WriteString("\n")
	contextLines := highlightContext(c, h)
	targetIdx := len(c.Context.Above)
	for i, hl := range contextLines {
		if i == targetIdx {
			fmt.Fprintf(&b, " %s+%s%s%s\n", cGreen, cReset, hl, cReset)
		} else {
			fmt.Fprintf(&b, "  %s%s\n", hl, cReset)
		}
	}

	// Comment text
	b.WriteString("\n")
	for _, line := range strings.Split(c.Text, "\n") {
		fmt.Fprintf(&b, "    %s\n", line)
	}

	// Add grey left margin bar to every line
	bar := cGray + "┃" + cReset + " "
	raw := b.String()
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var out strings.Builder
	for _, line := range lines {
		out.WriteString(bar)
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

// styleCommentPath formats a file:line with dir in dim, basename in bright white.
func styleCommentPath(path string, line int) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if dir == "." {
		return fmt.Sprintf("%s%s%s%s:%s%d%s", cBrightWhite, base, cReset, cGray, cReset, line, cReset)
	}
	return fmt.Sprintf("%s%s/%s%s%s%s%s:%s%d%s",
		cDirWhite, dir, cReset,
		cBrightWhite, base, cReset,
		cGray, cReset, line, cReset)
}

// highlightContext applies syntax highlighting to a comment's context lines.
// Returns the context lines (above + target + below) as styled strings.
// Falls back to plain text if the language isn't supported or h is nil.
func highlightContext(c *comments.Comment, h *highlight.Highlighter) []string {
	// Collect all lines
	allLines := make([]string, 0, len(c.Context.Above)+1+len(c.Context.Below))
	allLines = append(allLines, c.Context.Above...)
	allLines = append(allLines, c.Context.Line)
	allLines = append(allLines, c.Context.Below...)

	if h == nil || len(allLines) == 0 {
		return allLines
	}

	// Build single snippet for tree-sitter
	snippet := strings.Join(allLines, "\n")
	content := []byte(snippet)

	spans, _ := h.Highlight(c.File, content)
	if len(spans) == 0 {
		return allLines
	}

	theme := h.Theme()
	result := make([]string, len(allLines))
	offset := 0
	for i, line := range allLines {
		lineBytes := []byte(line)
		lineStart := offset
		lineEnd := offset + len(lineBytes)

		lineSpans := highlight.SpansForLine(spans, lineStart, lineEnd)
		if len(lineSpans) == 0 {
			result[i] = line
		} else {
			var sb strings.Builder
			pos := 0
			for _, s := range lineSpans {
				// Emit unstyled text before this span
				if s.Start > pos {
					sb.Write(lineBytes[pos:s.Start])
				}
				// Emit styled text
				style := theme.Style(s.Category)
				sb.WriteString(style.Render(string(lineBytes[s.Start:s.End])))
				pos = s.End
			}
			// Emit remainder
			if pos < len(lineBytes) {
				sb.Write(lineBytes[pos:])
			}
			result[i] = sb.String()
		}

		offset = lineEnd + 1 // +1 for newline
	}

	return result
}

// runCommentEdit opens a comment in $EDITOR for editing.
func runCommentEdit(id string) error {
	editor := editorCmd()
	if editor == "" {
		return fmt.Errorf("$VISUAL or $EDITOR must be set")
	}

	store := comments.NewStore("")

	// Read the existing comment
	original, err := store.ReadComment(id)
	if err != nil {
		return fmt.Errorf("comment %s not found: %w", id, err)
	}

	// Serialize to temp file
	serialized := original.Serialize()
	tmpFile, err := os.CreateTemp("", "dfd-comment-*.patch")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(serialized); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	// Open in editor
	cmd := exec.Command("sh", "-c", editor+` "$@"`, "--", tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	// Read back edited content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("read edited file: %w", err)
	}

	// Parse and validate
	parsed, err := comments.ParseComment(id, string(edited))
	if err != nil {
		return fmt.Errorf("invalid comment format: %w", err)
	}
	if parsed.File == "" {
		return fmt.Errorf("invalid edit: FILE metadata is required")
	}
	if parsed.Line <= 0 {
		return fmt.Errorf("invalid edit: LINE metadata must be positive")
	}

	// Update timestamp and write back
	parsed.Updated = time.Now()
	if _, err := store.WriteComment(parsed); err != nil {
		return fmt.Errorf("saving comment: %w", err)
	}

	fmt.Printf("Updated comment %s\n", id)
	return nil
}

// runClean deletes all persisted snapshot refs.
func runClean() error {
	g := git.New()

	// Delete all refs (empty strings means delete all)
	if err := g.DeleteSnapshotRefs("", ""); err != nil {
		return fmt.Errorf("delete snapshot refs: %w", err)
	}

	fmt.Println("Deleted persisted snapshots")
	return nil
}

// runLogMode handles log mode showing multiple commits.
func runLogMode(cfg config.Config, args parsedArgs) error {
	g := git.New()

	// Build extra args for git log (ref range + pathspec)
	var logArgs []string
	if len(args.refs) > 0 {
		logArgs = append(logArgs, args.refs[0])
	}
	logArgs = append(logArgs, buildPathspec(args.paths, args.excludes)...)

	// Use -n count if specified, otherwise default batching
	initialBatch := tui.DefaultCommitBatchSize
	if args.count > 0 {
		initialBatch = args.count
	}

	commits, err := g.LogPathsOnly(initialBatch, logArgs...)
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No commits")
		return nil
	}

	// Fetch stats for the first page of commits synchronously so they're visible immediately
	initialStatsCount := 30
	if _, height, err := term.GetSize(int(os.Stdout.Fd())); err == nil && height > 0 {
		initialStatsCount = height
	}
	initialLimit := initialStatsCount
	if initialLimit > len(commits) {
		initialLimit = len(commits)
	}
	initialStats, _ := g.LogMetaOnlyRange(0, initialLimit, logArgs...)

	statsMap := make(map[string]*git.CommitWithStats)
	for i := range initialStats {
		statsMap[initialStats[i].Meta.SHA] = &initialStats[i]
	}

	var commitSets []sidebyside.CommitSet
	for i, c := range commits {
		var files []sidebyside.FilePair
		var totalAdded, totalRemoved int
		statsLoaded := false

		if stats, ok := statsMap[c.Meta.SHA]; ok && i < initialLimit {
			for _, f := range stats.Files {
				files = append(files, sidebyside.SkeletonFilePair(f.Path, f.Added, f.Removed))
				if f.Added > 0 {
					totalAdded += f.Added
				}
				if f.Removed > 0 {
					totalRemoved += f.Removed
				}
			}
			statsLoaded = true
		} else {
			for _, f := range c.Files {
				files = append(files, sidebyside.SkeletonFilePairNoStats(f.Path))
			}
		}

		commitSet := sidebyside.CommitSet{
			Info: sidebyside.CommitInfo{
				SHA:     c.Meta.SHA,
				Author:  c.Meta.Author,
				Email:   c.Meta.Email,
				Date:    c.Meta.Date,
				Subject: c.Meta.Subject,
				Body:    c.Meta.Body,
				Refs:    sidebyside.ParseRefs(c.Meta.Refs),
			},
			Files:        files,
			FoldLevel:    sidebyside.CommitFolded,
			FilesLoaded:  false,
			StatsLoaded:  statsLoaded,
			TotalAdded:   totalAdded,
			TotalRemoved: totalRemoved,
		}
		commitSets = append(commitSets, commitSet)
	}

	commentStore := comments.NewStore("")

	opts := []tui.Option{
		tui.WithConfig(cfg),
		tui.WithGit(g),
		tui.WithPagination(len(commitSets), initialBatch),
		tui.WithCommentStore(commentStore),
	}
	if args.debug {
		opts = append(opts, tui.WithDebugMode())
	}
	if len(logArgs) > 0 {
		opts = append(opts, tui.WithLogArgs(logArgs))
	}

	model := tui.NewWithCommits(commitSets, opts...)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus(), tea.WithMouseCellMotion())

	_, err = p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// countFilePairStats counts added and removed lines in a file pair.
// buildSnapshotHistory builds the snapshot timeline CommitSets from persisted snapshot history.
// Returns commits in display order: [lastSnapshot→WT, S(n-1)→S(n), ..., S0→S1, base→S0].
func buildSnapshotHistory(g *git.RealGit, snapshotInfos []git.SnapshotInfo, baseSHA string, persistedSnapshots []string, pathspec []string) []sidebyside.CommitSet {
	var commits []sidebyside.CommitSet

	// Build lastSnapshot→WT diff as the top commit
	if len(persistedSnapshots) > 0 {
		lastSnapshot := persistedSnapshots[len(persistedSnapshots)-1]
		var wtDiffOutput string
		var err error
		diffArgs := []string{lastSnapshot}
		diffArgs = append(diffArgs, pathspec...)
		wtDiffOutput, err = g.Diff(diffArgs...)
		if err == nil && wtDiffOutput != "" {
			if wtParsed, err := diff.Parse(wtDiffOutput); err == nil && len(wtParsed.Files) > 0 {
				wtFiles, _ := sidebyside.TransformDiff(wtParsed)
				wtCommit := sidebyside.CommitSet{
					Info: sidebyside.CommitInfo{
						Subject: "Working tree changes",
					},
					Files:          wtFiles,
					FoldLevel:      sidebyside.CommitFileHeaders,
					FilesLoaded:    true,
					StatsLoaded:    true,
					IsSnapshot:     true,
					SnapshotOldRef: lastSnapshot,
				}
				for _, f := range wtFiles {
					added, removed := countFilePairStats(f)
					wtCommit.TotalAdded += added
					wtCommit.TotalRemoved += removed
				}
				commits = append(commits, wtCommit)
			}
		}
	}

	// Build S(n-1)→S(n) diffs (newest first)
	for i := len(snapshotInfos) - 1; i >= 1; i-- {
		oldRef := snapshotInfos[i-1].SHA
		newRef := snapshotInfos[i].SHA

		histDiff, err := g.DiffSnapshots(oldRef, newRef, pathspec...)
		if err != nil || histDiff == "" {
			continue
		}

		histParsed, err := diff.Parse(histDiff)
		if err != nil {
			continue
		}

		info := snapshotInfos[i]
		snapshotShort := info.SHA
		if len(snapshotShort) > 7 {
			snapshotShort = snapshotShort[:7]
		}

		histFiles, _ := sidebyside.TransformDiff(histParsed)
		histCommit := sidebyside.CommitSet{
			Info: sidebyside.CommitInfo{
				SHA:     snapshotShort,
				Date:    info.Date,
				Subject: info.Subject,
			},
			Files:          histFiles,
			FoldLevel:      sidebyside.CommitFolded,
			FilesLoaded:    true,
			StatsLoaded:    true,
			IsSnapshot:     true,
			SnapshotOldRef: oldRef,
			SnapshotNewRef: newRef,
		}
		for _, f := range histFiles {
			added, removed := countFilePairStats(f)
			histCommit.TotalAdded += added
			histCommit.TotalRemoved += removed
		}
		commits = append(commits, histCommit)
	}

	// Initial diff: base→S0
	if len(snapshotInfos) > 0 {
		firstInfo := snapshotInfos[0]
		histDiff, err := g.DiffSnapshots(baseSHA, firstInfo.SHA, pathspec...)
		if err == nil && histDiff != "" {
			histParsed, err := diff.Parse(histDiff)
			if err == nil {
				snapshotShort := firstInfo.SHA
				if len(snapshotShort) > 7 {
					snapshotShort = snapshotShort[:7]
				}

				histFiles, _ := sidebyside.TransformDiff(histParsed)
				histCommit := sidebyside.CommitSet{
					Info: sidebyside.CommitInfo{
						SHA:     snapshotShort,
						Date:    firstInfo.Date,
						Subject: firstInfo.Subject,
					},
					Files:          histFiles,
					FoldLevel:      sidebyside.CommitFolded,
					FilesLoaded:    true,
					StatsLoaded:    true,
					IsSnapshot:     true,
					SnapshotOldRef: baseSHA,
					SnapshotNewRef: firstInfo.SHA,
				}
				for _, f := range histFiles {
					added, removed := countFilePairStats(f)
					histCommit.TotalAdded += added
					histCommit.TotalRemoved += removed
				}
				commits = append(commits, histCommit)
			}
		}
	}

	return commits
}

func countFilePairStats(fp sidebyside.FilePair) (added, removed int) {
	for _, lp := range fp.Pairs {
		if lp.New.Type == sidebyside.Added {
			added++
		}
		if lp.Old.Type == sidebyside.Removed {
			removed++
		}
	}
	return added, removed
}

// getDiffAll generates a diff that includes all changes: staged, unstaged, and untracked files.
// This combines `git diff <baseRef>` (tracked changes) with diffs for untracked files.
// Paths and excludes filter both the tracked diff and the untracked file list.
func getDiffAll(g *git.RealGit, baseRef string, paths, excludes []string) (string, error) {
	// Start with git diff <baseRef> to get all tracked changes
	diffArgs := []string{baseRef}
	diffArgs = append(diffArgs, buildPathspec(paths, excludes)...)
	output, err := g.Diff(diffArgs...)
	if err != nil {
		return "", fmt.Errorf("git diff %s: %w", baseRef, err)
	}

	// Get list of untracked files
	untrackedFiles, err := g.ListUntrackedFiles()
	if err != nil {
		return "", fmt.Errorf("list untracked files: %w", err)
	}

	// Filter untracked files by paths and excludes
	untrackedFiles = filterFiles(untrackedFiles, paths, excludes)

	// Generate diffs for each untracked file and append
	for _, file := range untrackedFiles {
		newFileDiff, err := g.DiffNewFile(file)
		if err != nil {
			continue
		}
		if newFileDiff != "" {
			output += newFileDiff
		}
	}

	return output, nil
}

// printExitComments prints all comments as a unified diff patch to stdout
// when the TUI exits, if any comments were added during the session.
// Currently unused — comments are persisted in git refs now, so printing
// on exit is no longer needed. We may re-enable some way to print comments
// later (e.g. via a CLI flag or subcommand).
//
//nolint:unused
func printExitComments(finalModel tea.Model) {
	if m, ok := finalModel.(tui.Model); ok {
		if snippet := m.AllCommentsSnippet(); snippet != "" {
			fmt.Print(snippet)
		}
	}
}
