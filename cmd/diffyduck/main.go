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
	cmd     string   // "diff" or "show"
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
		case "pager":
			result.cmd = "pager"
			result.gitArgs = args[1:]
		}
	}

	// Determine mode and refs based on command and args
	if result.cmd == "show" {
		result.mode = content.ModeShow
		// First non-flag arg is the commit ref
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

func run() error {
	rawArgs, debugMode := extractDebugFlag(os.Args[1:])
	args := parseArgs(rawArgs)

	// Check for pager mode: explicit "pager" command or piped stdin
	if args.cmd == "pager" || pager.IsStdinPipe() {
		return runPagerMode(debugMode)
	}

	g := git.New()

	// Get diff from git
	var output string
	var err error
	switch args.cmd {
	case "diff":
		output, err = g.Diff(args.gitArgs...)
		if err != nil {
			return fmt.Errorf("git diff: %w", err)
		}
	case "show":
		output, err = g.Show(args.gitArgs...)
		if err != nil {
			return fmt.Errorf("git show: %w", err)
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

	// Create and run the TUI
	opts := []tui.Option{tui.WithFetcher(fetcher), tui.WithTruncatedFileCount(truncatedFileCount)}
	if debugMode {
		opts = append(opts, tui.WithDebugMode())
	}
	model := tui.New(files, opts...)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
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
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
