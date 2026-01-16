package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/internal/tui"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs extracts the subcommand and remaining args from command line args.
// Returns (command, gitArgs) where command is "diff" or "show".
func parseArgs(args []string) (cmd string, gitArgs []string) {
	cmd = "diff" // default
	gitArgs = args

	if len(args) > 0 {
		switch args[0] {
		case "diff":
			cmd = "diff"
			gitArgs = args[1:]
		case "show":
			cmd = "show"
			gitArgs = args[1:]
		}
	}

	return cmd, gitArgs
}

func run() error {
	g := git.New()
	cmd, gitArgs := parseArgs(os.Args[1:])

	// Get diff from git
	var output string
	var err error
	switch cmd {
	case "diff":
		output, err = g.Diff(gitArgs...)
		if err != nil {
			return fmt.Errorf("git diff: %w", err)
		}
	case "show":
		output, err = g.Show(gitArgs...)
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
	files := sidebyside.TransformDiff(d)

	// Create and run the TUI
	model := tui.New(files)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
