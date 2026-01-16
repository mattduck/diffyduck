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

func run() error {
	// Determine what to show
	ref := "HEAD"
	if len(os.Args) > 1 {
		ref = os.Args[1]
	}

	// Get diff from git
	g := git.New()
	output, err := g.Show(ref)
	if err != nil {
		return fmt.Errorf("git show: %w", err)
	}

	// Parse the diff
	d, err := diff.Parse(output)
	if err != nil {
		return fmt.Errorf("parse diff: %w", err)
	}

	if len(d.Files) == 0 {
		fmt.Println("No changes in", ref)
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
