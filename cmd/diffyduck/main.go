package main

import (
	"fmt"
	"os"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/internal/tui"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/pager"
	"github.com/user/diffyduck/pkg/sidebyside"
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

func run() error {
	rawArgs, debugMode := extractDebugFlag(os.Args[1:])
	rawArgs, allMode := extractAllFlag(rawArgs)
	args := parseArgs(rawArgs)

	// Check for pager mode: explicit "pager" command or piped stdin
	if args.cmd == "pager" || pager.IsStdinPipe() {
		return runPagerMode(debugMode)
	}

	// Handle log command separately
	if args.cmd == "log" {
		return runLogMode(debugMode)
	}

	g := git.New()

	// Get diff from git, with optional commit metadata
	var output string
	var commitInfo sidebyside.CommitInfo
	var err error

	switch args.cmd {
	case "diff":
		if allMode {
			// --all mode: diff HEAD (staged + unstaged) and include untracked files
			output, err = getDiffAll(g, args.gitArgs)
			if err != nil {
				return err
			}
			// Override mode for content fetching: compare HEAD to working tree
			args.mode = content.ModeDiffRefs
			args.ref1 = "HEAD"
			args.ref2 = ""
		} else {
			output, err = g.Diff(args.gitArgs...)
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

	if len(d.Files) == 0 {
		fmt.Println("No changes")
		return nil
	}

	// Transform to side-by-side format
	files, truncatedFileCount := sidebyside.TransformDiff(d)

	// Create content fetcher for lazy file loading
	fetcher := content.NewFetcher(g, args.mode, args.ref1, args.ref2)

	// Build commit set with files and optional metadata
	commit := sidebyside.CommitSet{
		Info:               commitInfo,
		Files:              files,
		FoldLevel:          sidebyside.CommitNormal,
		FilesLoaded:        true,
		TruncatedFileCount: truncatedFileCount,
	}

	// Create and run the TUI
	opts := []tui.Option{tui.WithFetcher(fetcher)}
	if debugMode {
		opts = append(opts, tui.WithDebugMode())
	}
	model := tui.NewWithCommits([]sidebyside.CommitSet{commit}, opts...)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// runLogMode handles log mode showing multiple commits.
func runLogMode(debugMode bool) error {
	g := git.New()

	// Fetch commits with diffs (TODO: add pagination)
	commits, err := g.LogWithMeta(500)
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No commits")
		return nil
	}

	// Convert to CommitSets
	var commitSets []sidebyside.CommitSet
	for _, c := range commits {
		// Parse the diff for this commit
		d, err := diff.Parse(c.Diff)
		if err != nil {
			return fmt.Errorf("parse diff for %s: %w", c.Meta.SHA[:7], err)
		}

		// Transform to side-by-side format
		files, truncatedFileCount := sidebyside.TransformDiff(d)

		commitSet := sidebyside.CommitSet{
			Info: sidebyside.CommitInfo{
				SHA:     c.Meta.SHA,
				Author:  c.Meta.Author,
				Email:   c.Meta.Email,
				Date:    c.Meta.Date,
				Subject: c.Meta.Subject,
				Body:    c.Meta.Body,
			},
			Files:              files,
			FoldLevel:          sidebyside.CommitFolded, // Start folded
			FilesLoaded:        true,
			TruncatedFileCount: truncatedFileCount,
		}
		commitSets = append(commitSets, commitSet)
	}

	// Pass git object for on-demand content fetching (per-commit fetchers)
	opts := []tui.Option{tui.WithGit(g)}
	if debugMode {
		opts = append(opts, tui.WithDebugMode())
	}

	model := tui.NewWithCommits(commitSets, opts...)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
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

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
