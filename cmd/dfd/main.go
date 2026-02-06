package main

import (
	"fmt"
	"os"
	"runtime/pprof"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/internal/tui"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/pager"
	"github.com/user/diffyduck/pkg/sidebyside"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// parsedArgs contains parsed command line arguments.
type parsedArgs struct {
	cmd     string   // "diff", "show", or "log"
	gitArgs []string // args to pass to git
	mode    content.Mode
	ref1    string // commit for show, or first ref for diff
	ref2    string // second ref for diff (if comparing refs)
}

// parseArgs extracts the subcommand and remaining args from command line args.
func parseArgs(args []string) parsedArgs {
	result := parsedArgs{
		cmd:     "diff",
		gitArgs: args,
		mode:    content.ModeDiffUnstaged,
	}

	if len(args) > 0 {
		switch args[0] {
		case "diff":
			result.cmd = "diff"
			result.gitArgs = args[1:]
		case "show":
			result.cmd = "show"
			result.gitArgs = args[1:]
		case "log":
			result.cmd = "log"
			result.gitArgs = args[1:]
		case "pager":
			result.cmd = "pager"
			result.gitArgs = args[1:]
		case "clean":
			result.cmd = "clean"
			result.gitArgs = args[1:]
		}
	}

	// Determine mode and refs based on command and args
	if result.cmd == "show" || result.cmd == "log" {
		result.mode = content.ModeShow
		// First non-flag arg is the commit ref (for show; log ignores this for now)
		for _, arg := range result.gitArgs {
			if !isFlag(arg) {
				result.ref1 = arg
				break
			}
		}
		if result.ref1 == "" {
			result.ref1 = "HEAD"
		}
	} else {
		// diff command - determine mode
		result.mode = content.ModeDiffUnstaged
		for _, arg := range result.gitArgs {
			if arg == "--cached" || arg == "--staged" {
				result.mode = content.ModeDiffCached
				break
			}
		}
		// Check for ref arguments (non-flag args that look like refs)
		var refs []string
		for _, arg := range result.gitArgs {
			if !isFlag(arg) && !isPath(arg) {
				refs = append(refs, arg)
			}
		}
		if len(refs) >= 2 {
			result.mode = content.ModeDiffRefs
			result.ref1 = refs[0]
			result.ref2 = refs[1]
		} else if len(refs) == 1 {
			// Single ref means diff against working tree
			result.mode = content.ModeDiffRefs
			result.ref1 = refs[0]
			result.ref2 = "" // Will compare to working tree
		}
	}

	return result
}

// isFlag returns true if the argument looks like a flag.
func isFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}

// isPath returns true if the argument looks like a file path.
// This is a heuristic - contains / or . extension.
func isPath(arg string) bool {
	for _, c := range arg {
		if c == '/' || c == '\\' {
			return true
		}
	}
	// Check for file extension
	for i := len(arg) - 1; i >= 0; i-- {
		if arg[i] == '.' {
			return i > 0 && i < len(arg)-1
		}
	}
	return false
}

// workingTreeInvolved returns true if the diff involves the current working tree.
// This is used to determine if snapshots should be available.
// Returns true for: unstaged diff, cached diff, or single-ref diff (ref vs working tree).
// Returns false for: two-ref diff (comparing two fixed commits).
func workingTreeInvolved(args parsedArgs) bool {
	switch args.mode {
	case content.ModeDiffUnstaged:
		// git diff - compares index to working tree
		return true
	case content.ModeDiffCached:
		// git diff --cached - compares HEAD to index (index can still change)
		return true
	case content.ModeDiffRefs:
		// Check if ref2 is empty (meaning working tree is the target)
		return args.ref2 == ""
	default:
		return false
	}
}

// extractDebugFlag removes --debug from args and returns (remaining args, debugMode).
func extractDebugFlag(args []string) ([]string, bool) {
	idx := slices.Index(args, "--debug")
	if idx == -1 {
		return args, false
	}
	return slices.Delete(slices.Clone(args), idx, idx+1), true
}

// extractAllFlag removes --all/-a from args and returns (remaining args, allMode).
// --all mode shows all changes including untracked files.
func extractAllFlag(args []string) ([]string, bool) {
	for _, flag := range []string{"--all", "-a"} {
		idx := slices.Index(args, flag)
		if idx != -1 {
			return slices.Delete(slices.Clone(args), idx, idx+1), true
		}
	}
	return args, false
}

// extractCPUProfileFlag removes --cpuprofile=<path> from args and returns (remaining args, profilePath).
func extractCPUProfileFlag(args []string) ([]string, string) {
	for i, arg := range args {
		if strings.HasPrefix(arg, "--cpuprofile=") {
			path := strings.TrimPrefix(arg, "--cpuprofile=")
			return slices.Delete(slices.Clone(args), i, i+1), path
		}
	}
	return args, ""
}

// extractUnstagedFlag removes --unstaged from args and returns (remaining args, unstagedMode).
// Unstaged mode diffs index vs working tree (the old default behavior).
func extractUnstagedFlag(args []string) ([]string, bool) {
	idx := slices.Index(args, "--unstaged")
	if idx != -1 {
		return slices.Delete(slices.Clone(args), idx, idx+1), true
	}
	return args, false
}

// extractSnapshotsFlag removes --snapshots/--no-snapshots from args.
// Returns (remaining args, snapshotsExplicitlyDisabled).
// Default is snapshots enabled (returns false).
func extractSnapshotsFlag(args []string) ([]string, bool) {
	// Check for --no-snapshots first
	idx := slices.Index(args, "--no-snapshots")
	if idx != -1 {
		return slices.Delete(slices.Clone(args), idx, idx+1), true
	}
	// Check for --snapshots (explicit enable, which is already the default)
	idx = slices.Index(args, "--snapshots")
	if idx != -1 {
		return slices.Delete(slices.Clone(args), idx, idx+1), false
	}
	return args, false
}

func run() error {
	rawArgs, debugMode := extractDebugFlag(os.Args[1:])
	rawArgs, allMode := extractAllFlag(rawArgs)
	rawArgs, cpuProfile := extractCPUProfileFlag(rawArgs)
	rawArgs, unstagedMode := extractUnstagedFlag(rawArgs)
	rawArgs, snapshotsDisabled := extractSnapshotsFlag(rawArgs)
	args := parseArgs(rawArgs)

	// Start CPU profiling if requested
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
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

	// Check for pager mode: explicit "pager" command or piped stdin
	if args.cmd == "pager" || pager.IsStdinPipe() {
		return runPagerMode(debugMode)
	}

	// Handle log command separately
	if args.cmd == "log" {
		return runLogMode(debugMode)
	}

	g := git.New()

	// Run async expiry of old snapshot refs (doesn't block startup)
	go func() {
		// Expire snapshot refs older than 14 days
		_, _ = g.ExpireOldSnapshotRefs(14)
	}()

	// Determine base ref for the diff (used for snapshot keying)
	// Default behavior: diff HEAD vs working tree (instead of index vs working tree)
	// --unstaged: diff index vs working tree (old default)
	// --cached: diff HEAD vs index
	// <ref>: diff <ref> vs working tree
	var baseSHA string
	if args.cmd == "diff" && !unstagedMode {
		// Resolve the base ref to a SHA for snapshot keying
		baseRef := args.ref1
		if baseRef == "" {
			baseRef = "HEAD" // default to HEAD
		}
		// Resolve ref to SHA
		sha, err := g.Show("--format=%H", "-s", baseRef)
		if err == nil {
			baseSHA = strings.TrimSpace(sha)
		}
	}

	// Determine if snapshots should be enabled
	// Disabled for: --no-snapshots, --unstaged, show command, two-ref diffs
	snapshotsEnabled := args.cmd == "diff" &&
		!snapshotsDisabled &&
		!unstagedMode &&
		workingTreeInvolved(args)

	// Auto-continue: check for existing snapshots at this base SHA
	var snapshotInfos []git.SnapshotInfo // full info (SHA, Subject, Date)
	var persistedSnapshots []string      // just SHAs for TUI
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

	// Get diff from git, with optional commit metadata
	var output string
	var commitInfo sidebyside.CommitInfo
	var err error

	switch args.cmd {
	case "diff":
		// In continue mode with persisted snapshots, diff from last snapshot
		if continueMode && len(persistedSnapshots) > 0 {
			lastSnapshot := persistedSnapshots[len(persistedSnapshots)-1]
			if allMode {
				// --all mode with continue: diff last snapshot to current working tree
				// Note: untracked files appear as new files from the snapshot's perspective
				output, err = getDiffAll(g, append([]string{lastSnapshot}, args.gitArgs...))
			} else {
				output, err = g.Diff(append([]string{lastSnapshot}, args.gitArgs...)...)
			}
			if err != nil {
				return fmt.Errorf("git diff from snapshot: %w", err)
			}
			// Set refs for content fetching
			args.mode = content.ModeDiffRefs
			args.ref1 = lastSnapshot
			args.ref2 = "" // working tree
		} else if allMode {
			// --all mode: diff HEAD (staged + unstaged) and include untracked files
			output, err = getDiffAll(g, args.gitArgs)
			if err != nil {
				return err
			}
			// Override mode for content fetching: compare HEAD to working tree
			args.mode = content.ModeDiffRefs
			args.ref1 = "HEAD"
			args.ref2 = ""
		} else if unstagedMode {
			// --unstaged mode: diff index vs working tree (old default behavior)
			output, err = g.Diff(args.gitArgs...)
			if err != nil {
				return fmt.Errorf("git diff: %w", err)
			}
		} else {
			// Default: diff HEAD vs working tree
			// Unless explicit refs are provided in args
			if args.ref1 == "" && args.mode == content.ModeDiffUnstaged {
				// No ref specified, use HEAD as default
				output, err = g.Diff(append([]string{"HEAD"}, args.gitArgs...)...)
				args.mode = content.ModeDiffRefs
				args.ref1 = "HEAD"
				args.ref2 = "" // working tree
			} else {
				output, err = g.Diff(args.gitArgs...)
			}
			if err != nil {
				return fmt.Errorf("git diff: %w", err)
			}
		}
		// diff command has no commit metadata
	case "show":
		var meta *git.CommitMeta
		meta, output, err = g.ShowWithMeta(args.gitArgs...)
		if err != nil {
			return fmt.Errorf("git show: %w", err)
		}
		// Convert git metadata to sidebyside format
		if meta != nil {
			commitInfo = sidebyside.CommitInfo{
				SHA:     meta.SHA,
				Author:  meta.Author,
				Email:   meta.Email,
				Date:    meta.Date,
				Subject: meta.Subject,
				Body:    meta.Body,
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

	// In continue mode, reconstruct historical diffs (newest first, like log view)
	if continueMode && len(snapshotInfos) > 0 {
		// Build historical diffs in reverse order (newest first)
		// S(n-1)→S(n), S(n-2)→S(n-1), ..., S0→S1, base→S0
		for i := len(snapshotInfos) - 1; i >= 1; i-- {
			oldRef := snapshotInfos[i-1].SHA
			newRef := snapshotInfos[i].SHA

			histDiff, err := g.DiffSnapshots(oldRef, newRef)
			if err != nil {
				continue // skip if we can't get the diff
			}
			if histDiff == "" {
				continue // skip empty diffs
			}

			histParsed, err := diff.Parse(histDiff)
			if err != nil {
				continue
			}

			// Use info from git log (single source of truth)
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
				FoldLevel:      sidebyside.CommitFolded, // Start folded so user can expand
				FilesLoaded:    true,
				StatsLoaded:    true,
				IsSnapshot:     true,
				SnapshotOldRef: oldRef,
				SnapshotNewRef: newRef,
			}
			// Calculate stats
			for _, f := range histFiles {
				added, removed := countFilePairStats(f)
				histCommit.TotalAdded += added
				histCommit.TotalRemoved += removed
			}
			commits = append(commits, histCommit)
		}

		// Finally, add the initial diff: base→S0 (oldest, so last in list)
		firstInfo := snapshotInfos[0]
		histDiff, err := g.DiffSnapshots(baseSHA, firstInfo.SHA)
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

	// Add the current working tree diff (or the main diff for non-continue mode)
	// In continue mode, prepend at top (newest first); otherwise append
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
			commit.Info.SHA = "" // No snapshot SHA yet (created in background)
			commit.Info.Subject = fmt.Sprintf("dfd: %s @ %s", baseShort, dateStr)
			if continueMode && len(persistedSnapshots) > 0 {
				commit.SnapshotOldRef = persistedSnapshots[len(persistedSnapshots)-1]
			}
		}

		if continueMode {
			// Prepend current diff at top (newest first)
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

	// Create and run the TUI
	opts := []tui.Option{tui.WithFetcher(fetcher), tui.WithGit(g)}
	if debugMode {
		opts = append(opts, tui.WithDebugMode())
	}
	if allMode {
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
func runLogMode(debugMode bool) error {
	g := git.New()

	// Fast startup: fetch only first batch of commits (pagination loads more on demand)
	// Stats are loaded asynchronously after the UI renders
	initialBatch := tui.DefaultCommitBatchSize
	commits, err := g.LogPathsOnly(initialBatch)
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No commits")
		return nil
	}

	// Fetch stats for the first page of commits synchronously so they're visible immediately
	// The rest will load asynchronously after the UI starts
	// Use terminal height to determine how many commits are initially visible
	initialStatsCount := 30 // fallback if we can't get terminal size
	if _, height, err := term.GetSize(int(os.Stdout.Fd())); err == nil && height > 0 {
		initialStatsCount = height
	}
	initialLimit := initialStatsCount
	if initialLimit > len(commits) {
		initialLimit = len(commits)
	}
	initialStats, _ := g.LogMetaOnlyRange(0, initialLimit) // ignore error, stats are optional

	// Build a map of SHA -> stats for quick lookup
	statsMap := make(map[string]*git.CommitWithStats)
	for i := range initialStats {
		statsMap[initialStats[i].Meta.SHA] = &initialStats[i]
	}

	// Convert to CommitSets with skeleton files (stats and diffs loaded on demand)
	var commitSets []sidebyside.CommitSet
	for i, c := range commits {
		// Check if we have stats for this commit
		var files []sidebyside.FilePair
		var totalAdded, totalRemoved int
		statsLoaded := false

		if stats, ok := statsMap[c.Meta.SHA]; ok && i < initialLimit {
			// We have stats - create files with stats
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
			// No stats yet - create skeleton files without stats
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
			},
			Files:        files,
			FoldLevel:    sidebyside.CommitFolded, // Start folded
			FilesLoaded:  false,                   // Diff content loaded on demand when unfolded
			StatsLoaded:  statsLoaded,
			TotalAdded:   totalAdded,
			TotalRemoved: totalRemoved,
		}
		commitSets = append(commitSets, commitSet)
	}

	// Pass git object for on-demand content fetching and pagination
	opts := []tui.Option{
		tui.WithGit(g),
		tui.WithPagination(len(commitSets), initialBatch),
	}
	if debugMode {
		opts = append(opts, tui.WithDebugMode())
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
// This combines `git diff HEAD` (tracked changes) with diffs for untracked files.
//
// TODO: Instead of passing all flags through to git, we should implement our own
// whitelisted flags. Currently path filters don't apply to untracked file listing,
// and some git flags may not make sense in this context.
func getDiffAll(g *git.RealGit, extraArgs []string) (string, error) {
	// Start with git diff HEAD to get all tracked changes
	diffArgs := append([]string{"HEAD"}, extraArgs...)
	output, err := g.Diff(diffArgs...)
	if err != nil {
		return "", fmt.Errorf("git diff HEAD: %w", err)
	}

	// Get list of untracked files
	untrackedFiles, err := g.ListUntrackedFiles()
	if err != nil {
		return "", fmt.Errorf("list untracked files: %w", err)
	}

	// Generate diffs for each untracked file and append
	for _, file := range untrackedFiles {
		newFileDiff, err := g.DiffNewFile(file)
		if err != nil {
			// Skip files that fail (e.g., binary files, permission issues)
			continue
		}
		if newFileDiff != "" {
			output += newFileDiff
		}
	}

	return output, nil
}

// runPagerMode handles pager mode where diff input comes from stdin.
func runPagerMode(debugMode bool) error {
	// Read and strip ANSI codes from stdin
	input, err := pager.ReadStdin()
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	// Parse the diff
	d, err := diff.Parse(input)
	if err != nil {
		return fmt.Errorf("parse diff: %w", err)
	}

	if len(d.Files) == 0 {
		fmt.Println("No changes")
		return nil
	}

	// Transform to side-by-side format
	files, truncatedFileCount := sidebyside.TransformDiff(d)

	// Create and run the TUI in pager mode (no fetcher available)
	opts := []tui.Option{tui.WithPagerMode(), tui.WithTruncatedFileCount(truncatedFileCount)}
	if debugMode {
		opts = append(opts, tui.WithDebugMode())
	}
	model := tui.New(files, opts...)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus(), tea.WithMouseCellMotion())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	printExitComments(finalModel)

	return nil
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
