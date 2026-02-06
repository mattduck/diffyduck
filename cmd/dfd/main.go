package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/internal/tui"
	"github.com/user/diffyduck/pkg/branches"
	"github.com/user/diffyduck/pkg/comments"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/sidebyside"
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
	cmd      string   // "diff", "show", "log", "clean"
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

	// branches-specific
	verbose bool // -v/--verbose

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

// parseArgs parses command line arguments into structured fields.
// Unknown flags produce an error. No arguments are passed through to git verbatim.
func parseArgs(args []string) (parsedArgs, error) {
	result := parsedArgs{cmd: "diff"}

	// Consume subcommand if present
	remaining := args
	if len(remaining) > 0 {
		switch remaining[0] {
		case "diff", "show", "log", "clean", "branches":
			result.cmd = remaining[0]
			result.helpCmd = remaining[0] // target for --help flag
			remaining = remaining[1:]
		case "help":
			result.showHelp = true
			remaining = remaining[1:]
			// Optional target command: "help diff", "help show", etc.
			if len(remaining) > 0 {
				result.helpCmd = remaining[0]
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
		n, err := strconv.Atoi(args[i+1])
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("-n requires a positive integer, got %q", args[i+1])
		}
		p.count = n
		return 1, nil
	case strings.HasPrefix(arg, "-n"):
		nStr := strings.TrimPrefix(arg, "-n")
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("-n requires a positive integer, got %q", nStr)
		}
		p.count = n

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
	case "clean":
		if len(p.refs) > 0 || len(p.paths) > 0 || len(p.excludes) > 0 {
			return fmt.Errorf("%s does not accept arguments", p.cmd)
		}
		if p.cached || p.unstaged || p.allMode || p.count > 0 || p.verbose {
			return fmt.Errorf("%s does not accept flags", p.cmd)
		}
	case "branches":
		if len(p.refs) > 0 || len(p.paths) > 0 || len(p.excludes) > 0 {
			return fmt.Errorf("branches does not accept arguments")
		}
		if p.cached || p.unstaged || p.allMode || p.count > 0 {
			return fmt.Errorf("branches only accepts -v/--verbose")
		}
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
	default:
		fmt.Print(usageGeneral)
	}
}

const usageGeneral = `dfd - terminal side-by-side diff viewer

Usage:
  dfd [flags] [refs] [-- paths]
  dfd <command> [flags] [args]

Commands:
  diff       Compare changes (default)
  show       Show a commit
  log        Browse commit history
  clean      Delete persisted snapshots

Global flags:
  -h, --help       Show help
      --version    Show version

Diff flags:
      --cached     Diff staged changes (alias: --staged)
      --unstaged   Diff unstaged changes only
  -a, --all        Include untracked files
      --snapshots  Enable snapshots
      --no-snapshots
                   Disable snapshots
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
      --snapshots  Enable snapshots
      --no-snapshots
                   Disable snapshots
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

	// Handle clean command - deletes all persisted snapshot refs
	if args.cmd == "clean" {
		return runClean()
	}

	// Handle branches command - show branch dependency tree
	if args.cmd == "branches" {
		return runBranches(args.verbose)
	}

	// Handle log command separately
	if args.cmd == "log" {
		return runLogMode(args)
	}

	g := git.New()

	// Run async expiry of old snapshot refs (doesn't block startup)
	go func() {
		_, _ = g.ExpireOldSnapshotRefs(14)
	}()

	// Build pathspec for git commands
	pathspec := buildPathspec(args.paths, args.excludes)

	// Determine base ref for the diff (used for snapshot keying)
	var baseSHA string
	if args.cmd == "diff" && !args.unstaged {
		baseRef := args.ref1
		if baseRef == "" {
			baseRef = "HEAD"
		}
		sha, err := g.Show("--format=%H", "-s", baseRef)
		if err == nil {
			baseSHA = strings.TrimSpace(sha)
		}
	}

	// Determine if snapshots should be enabled
	snapshotsDisabled := args.snapshots != nil && !*args.snapshots
	snapshotsEnabled := args.cmd == "diff" &&
		!snapshotsDisabled &&
		!args.unstaged &&
		workingTreeInvolved(args)

	// Auto-continue: check for existing snapshots at this base SHA
	var snapshotInfos []git.SnapshotInfo
	var persistedSnapshots []string
	if snapshotsEnabled && baseSHA != "" {
		infos, err := g.ListSnapshotRefs(baseSHA)
		if err == nil {
			snapshotInfos = infos
			persistedSnapshots = make([]string, len(infos))
			for i, info := range infos {
				persistedSnapshots[i] = info.SHA
			}
		}
	}
	continueMode := len(persistedSnapshots) > 0

	// Detect merge/rebase conflict state (used to adjust diff commands and enable
	// conflict marker highlighting in the TUI).
	hasConflicts := g.HasConflicts()

	// Get diff from git, with optional commit metadata
	var output string
	var commitInfo sidebyside.CommitInfo

	switch args.cmd {
	case "diff":
		if continueMode && len(persistedSnapshots) > 0 {
			lastSnapshot := persistedSnapshots[len(persistedSnapshots)-1]
			if args.allMode {
				output, err = getDiffAll(g, lastSnapshot, args.paths, args.excludes)
			} else {
				diffArgs := []string{lastSnapshot}
				diffArgs = append(diffArgs, pathspec...)
				output, err = g.Diff(diffArgs...)
			}
			if err != nil {
				return fmt.Errorf("git diff from snapshot: %w", err)
			}
			args.mode = content.ModeDiffRefs
			args.ref1 = lastSnapshot
			args.ref2 = ""
		} else if args.allMode {
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

	// Build commit sets - in continue mode with history, we show historical diffs too
	var commits []sidebyside.CommitSet

	if continueMode && len(snapshotInfos) > 0 {
		for i := len(snapshotInfos) - 1; i >= 1; i-- {
			oldRef := snapshotInfos[i-1].SHA
			newRef := snapshotInfos[i].SHA

			histDiff, err := g.DiffSnapshots(oldRef, newRef, pathspec...)
			if err != nil {
				continue
			}
			if histDiff == "" {
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

	if len(d.Files) > 0 {
		files, truncatedFileCount := sidebyside.TransformDiff(d)

		commit := sidebyside.CommitSet{
			Info:               commitInfo,
			Files:              files,
			FoldLevel:          sidebyside.CommitNormal,
			FilesLoaded:        true,
			TruncatedFileCount: truncatedFileCount,
			IsSnapshot:         snapshotsEnabled,
		}

		if snapshotsEnabled && baseSHA != "" {
			baseShort := baseSHA
			if len(baseShort) > 7 {
				baseShort = baseShort[:7]
			}
			now := time.Now()
			dateStr := now.Format("Jan 2 15:04")
			commit.Info.Date = dateStr
			commit.Info.SHA = ""
			commit.Info.Subject = fmt.Sprintf("dfd: %s @ %s", baseShort, dateStr)
			if continueMode && len(persistedSnapshots) > 0 {
				commit.SnapshotOldRef = persistedSnapshots[len(persistedSnapshots)-1]
			}
		}

		if continueMode {
			commits = append([]sidebyside.CommitSet{commit}, commits...)
		} else {
			commits = append(commits, commit)
		}
	} else if len(commits) == 0 {
		fmt.Println("No changes")
		return nil
	}

	// Create content fetcher for lazy file loading
	fetcher := content.NewFetcher(g, args.mode, args.ref1, args.ref2)

	// Create comment store for persistence
	commentStore := comments.NewStore("")

	// Create and run the TUI
	opts := []tui.Option{tui.WithFetcher(fetcher), tui.WithGit(g), tui.WithCommentStore(commentStore)}
	if args.debug {
		opts = append(opts, tui.WithDebugMode())
	}
	if hasConflicts {
		opts = append(opts, tui.WithConflicts())
	}
	if args.allMode {
		opts = append(opts, tui.WithAllMode(true))
	}
	if snapshotsEnabled {
		opts = append(opts, tui.WithSnapshotsEnabled(true))
	}
	if continueMode {
		opts = append(opts, tui.WithContinueMode(true))
		if len(persistedSnapshots) > 0 {
			opts = append(opts, tui.WithPersistedSnapshots(persistedSnapshots))
		}
	}
	if baseSHA != "" {
		opts = append(opts, tui.WithBaseSHA(baseSHA))
	}

	model := tui.NewWithCommits(commits, opts...)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus(), tea.WithMouseCellMotion())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	printExitComments(finalModel)

	return nil
}

// runBranches prints a tree view of local branch dependencies.
func runBranches(verbose bool) error {
	g := git.New()
	branchList, err := g.LocalBranches()
	if err != nil {
		return fmt.Errorf("list branches: %w", err)
	}
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

// runClean deletes all persisted snapshot refs.
func runClean() error {
	g := git.New()

	// First list refs to see how many we're deleting
	// Empty string means list all refs across all bases
	refs, err := g.ListSnapshotRefs("")
	if err != nil {
		return fmt.Errorf("list snapshot refs: %w", err)
	}

	if len(refs) == 0 {
		fmt.Println("No persisted snapshots to clean")
		return nil
	}

	// Delete all refs (empty string means delete all)
	if err := g.DeleteSnapshotRefs(""); err != nil {
		return fmt.Errorf("delete snapshot refs: %w", err)
	}

	fmt.Printf("Deleted %d persisted snapshot(s)\n", len(refs))
	return nil
}

// runLogMode handles log mode showing multiple commits.
func runLogMode(args parsedArgs) error {
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

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	printExitComments(finalModel)

	return nil
}

// countFilePairStats counts added and removed lines in a file pair.
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
func printExitComments(finalModel tea.Model) {
	if m, ok := finalModel.(tui.Model); ok {
		if snippet := m.AllCommentsSnippet(); snippet != "" {
			fmt.Print(snippet)
		}
	}
}
