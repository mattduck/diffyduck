// Package ticketcli implements the command-line interface over the git-state
// ticket store (pkg/ticketdb): listing, adding and editing comments and notes.
//
// It is deliberately free of any TUI or tree-sitter (cgo) dependency so that the
// standalone tdb binary can be built with CGO_ENABLED=0. Syntax highlighting of a
// comment's code context is optional and injected via ContextHighlighter; when nil,
// context is rendered as plain text.
package ticketcli

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mattduck/diffyduck/pkg/config"
)

// CommentListStyles holds lipgloss styles for CLI comment list output.
type CommentListStyles struct {
	Header  lipgloss.Style // bold header text, file basenames
	Label   lipgloss.Style // dim labels, metadata text
	Commit  lipgloss.Style // commit SHA
	Branch  lipgloss.Style // branch name, author
	DirPart lipgloss.Style // directory part of file paths
}

// StylesFromConfig builds the CLI comment-list styles from a theme config,
// mirroring the defaults and overrides applied by the TUI's ApplyTheme. This lets
// the cgo-free tdb binary render colored output without importing internal/tui.
func StylesFromConfig(cfg config.ThemeConfig) CommentListStyles {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	if cfg.Header != "" {
		header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Header))
	}
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	if cfg.LineNumber != "" {
		label = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LineNumber))
	}
	commit := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	if cfg.CommitTree != "" {
		commit = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.CommitTree))
	}
	branch := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	if cfg.LocalRef != "" {
		branch = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.LocalRef))
	}
	dirPart := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	if cfg.HeaderDir != "" {
		dirPart = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.HeaderDir))
	}
	return CommentListStyles{
		Header:  header,
		Label:   label,
		Commit:  commit,
		Branch:  branch,
		DirPart: dirPart,
	}
}
