package config

import (
	"fmt"
	"strings"
)

// Default theme colors (matching the hardcoded values in internal/tui/view.go).
var DefaultTheme = ThemeConfig{
	Added:           "10",
	Removed:         "9",
	Changed:         "12",
	LineNumber:      "8",
	Header:          "15",
	HeaderDir:       "7",
	HeaderLine:      "8",
	HunkSeparator:   "8",
	SearchMatchFg:   "0",
	SearchMatchBg:   "3",
	SearchCurrentFg: "0",
	SearchCurrentBg: "9",
	CursorBg:        "7",
	CursorFg:        "0",
	CursorArrow:     "15",
	StatusBg:        "8",
	StatusFg:        "0",
	CommitTree:      "3",
	SnapshotTree:    "5",
	LocalRef:        "6",
	RemoteRef:       "8",
	ConflictMarker:  "3",
	CommentCheckbox: "3",
}

// Default syntax highlighting colors (matching highlight.DefaultTheme()).
// Empty string means "use terminal default".
var DefaultSyntaxTheme = SyntaxConfig{
	Keyword:   "4",
	String:    "8",
	Number:    "3",
	Comment:   "8",
	Function:  "12",
	Type:      "13",
	Field:     "14",
	Operator:  "7",
	Tag:       "8",
	Namespace: "",
	Variable:  "",
}

// Default feature values (matching the hardcoded values in internal/tui/model.go).
const (
	DefaultHScrollStep     = 4
	DefaultCommitBatchSize = 100
	DefaultAutoSnapshots   = true
	DefaultShowSnapshots   = false
	DefaultExpandAllBudget = 500
	DefaultChordTimeoutMs  = 250
)

// GenerateExample returns a complete, commented TOML config file with all
// current defaults filled in. defaultKeys should be populated from
// tui.DefaultKeyMap() by the caller (since pkg/config cannot import internal/tui).
func GenerateExample(defaultKeys KeysConfig) string {
	var b strings.Builder

	b.WriteString("# DiffyDuck configuration file\n")
	b.WriteString("# ~/.config/dfd/config.toml (or $XDG_CONFIG_HOME/dfd/config.toml)\n")
	b.WriteString("#\n")
	b.WriteString("# All values below are the defaults. Uncomment and edit to customise.\n")
	b.WriteString("# Omitted fields keep their defaults. Partial configs are fine.\n")
	b.WriteString("#\n")
	b.WriteString("# Key sequences use space-separated strings: \"g g\" means press g then g.\n\n")

	// Keys section header
	b.WriteString("[keys]\n")
	b.WriteString("# Per-action key bindings. Specifying an action replaces its defaults entirely.\n")
	b.WriteString("# Values are arrays of Bubble Tea key strings (e.g. \"ctrl+b\", \"up\", \"pgdown\").\n")
	b.WriteString("# See https://pkg.go.dev/github.com/charmbracelet/bubbletea#KeyMsg for key names.\n\n")

	writeKeys := func(name, comment string, keys []string) {
		b.WriteString(fmt.Sprintf("# %-19s = %s", name, formatTOMLArray(keys)))
		if comment != "" {
			b.WriteString(fmt.Sprintf("  # %s", comment))
		}
		b.WriteString("\n")
	}

	// Navigation subsection
	if nav := defaultKeys.Navigation; nav != nil {
		b.WriteString("[keys.navigation]\n")
		writeKeys("up", "", nav.Up)
		writeKeys("down", "", nav.Down)
		writeKeys("page_up", "", nav.PageUp)
		writeKeys("page_down", "", nav.PageDown)
		writeKeys("half_up", "", nav.HalfUp)
		writeKeys("half_down", "", nav.HalfDown)
		writeKeys("top", "", nav.Top)
		writeKeys("bottom", "", nav.Bottom)
		writeKeys("left", "", nav.Left)
		writeKeys("right", "", nav.Right)
		writeKeys("go_to_top", "key sequence", nav.GoToTop)
		writeKeys("next_heading", "key sequence", nav.NextHeading)
		writeKeys("prev_heading", "key sequence", nav.PrevHeading)
		writeKeys("next_comment", "key sequence", nav.NextComment)
		writeKeys("prev_comment", "key sequence", nav.PrevComment)
		writeKeys("next_change", "key sequence", nav.NextChange)
		writeKeys("prev_change", "key sequence", nav.PrevChange)
		writeKeys("narrow_next", "next node in narrow mode", nav.NarrowNext)
		writeKeys("narrow_prev", "previous node in narrow mode", nav.NarrowPrev)
		b.WriteString("\n")
	}

	// Search subsection
	if s := defaultKeys.Search; s != nil {
		b.WriteString("[keys.search]\n")
		writeKeys("search_fwd", "search forward", s.SearchFwd)
		writeKeys("search_back", "search backward", s.SearchBack)
		writeKeys("next_match", "", s.NextMatch)
		writeKeys("prev_match", "", s.PrevMatch)
		writeKeys("narrow_toggle", "key sequence", s.NarrowToggle)
		b.WriteString("\n")
	}

	// Folds subsection
	if f := defaultKeys.Folds; f != nil {
		b.WriteString("[keys.folds]\n")
		writeKeys("fold", "cycle fold level", f.Fold)
		writeKeys("fold_all", "cycle fold level for all files", f.FoldAll)
		writeKeys("full_file", "toggle full file view", f.FullFile)
		b.WriteString("\n")
	}

	// Actions subsection
	if a := defaultKeys.Actions; a != nil {
		b.WriteString("[keys.actions]\n")
		writeKeys("quit", "", a.Quit)
		writeKeys("enter", "add/edit comment", a.Enter)
		writeKeys("resolve_toggle", "toggle comment resolved", a.ResolveToggle)
		writeKeys("yank", "copy item (SHA / path / comment)", a.Yank)
		writeKeys("yank_all", "copy all comments", a.YankAll)
		writeKeys("refresh", "recalculate layout", a.Refresh)
		writeKeys("snapshot", "take snapshot", a.Snapshot)
		writeKeys("snapshot_toggle", "toggle snapshot view", a.SnapshotToggle)
		writeKeys("visual", "enter visual line mode", a.Visual)
		writeKeys("help", "toggle help screen", a.Help)
		b.WriteString("\n")
	}

	// Window subsection
	if w := defaultKeys.Window; w != nil {
		b.WriteString("[keys.window]\n")
		writeKeys("split_vertical", "key sequence", w.SplitVertical)
		writeKeys("split_horizontal", "key sequence", w.SplitHorizontal)
		writeKeys("close", "key sequence", w.Close)
		writeKeys("focus_left", "key sequence", w.FocusLeft)
		writeKeys("focus_right", "key sequence", w.FocusRight)
		writeKeys("focus_up", "key sequence", w.FocusUp)
		writeKeys("focus_down", "key sequence", w.FocusDown)
		writeKeys("resize_left", "key sequence", w.ResizeLeft)
		writeKeys("resize_right", "key sequence", w.ResizeRight)
		writeKeys("resize_up", "key sequence", w.ResizeUp)
		writeKeys("resize_down", "key sequence", w.ResizeDown)
		b.WriteString("\n")
	}

	// Visual subsection
	if v := defaultKeys.Visual; v != nil {
		b.WriteString("[keys.visual]\n")
		writeKeys("exit", "exit visual mode", v.Exit)
		b.WriteString("\n")
	}

	// Theme section
	b.WriteString("[theme]\n")
	b.WriteString("# Colors use ANSI 256-color numbers (\"0\"-\"255\") or hex strings (\"#ff5733\").\n")
	b.WriteString("# Omit any field to keep its default.\n")

	writeColor := func(name, value, comment string) {
		b.WriteString(fmt.Sprintf("# %-19s = %-8s # %s\n", name, fmt.Sprintf("%q", value), comment))
	}

	d := DefaultTheme
	writeColor("added", d.Added, "green — added lines")
	writeColor("removed", d.Removed, "red — removed lines")
	writeColor("changed", d.Changed, "blue — modified lines (word diff)")
	writeColor("context", d.Context, "default — unchanged context lines")
	writeColor("line_number", d.LineNumber, "gray — gutter line numbers")
	writeColor("header", d.Header, "bright white — file header text")
	writeColor("header_dir", d.HeaderDir, "dim white — directory part of path")
	writeColor("header_line", d.HeaderLine, "gray — header border characters")
	writeColor("hunk_separator", d.HunkSeparator, "gray — @@ hunk headers")
	writeColor("search_match_fg", d.SearchMatchFg, "black text on search matches")
	writeColor("search_match_bg", d.SearchMatchBg, "yellow background on search matches")
	writeColor("search_current_fg", d.SearchCurrentFg, "black text on current match")
	writeColor("search_current_bg", d.SearchCurrentBg, "red background on current match")
	writeColor("cursor_bg", d.CursorBg, "silver — cursor line background")
	writeColor("cursor_fg", d.CursorFg, "black — cursor line foreground")
	writeColor("cursor_arrow", d.CursorArrow, "bright white — the > arrow")
	writeColor("status_bg", d.StatusBg, "gray — status/less bar background")
	writeColor("status_fg", d.StatusFg, "black — status/less bar text")
	writeColor("commit_tree", d.CommitTree, "yellow — commit tree glyphs")
	writeColor("snapshot_tree", d.SnapshotTree, "magenta — snapshot tree glyphs")
	writeColor("local_ref", d.LocalRef, "cyan — local branch decorations")
	writeColor("remote_ref", d.RemoteRef, "gray — remote branch decorations")
	writeColor("conflict_marker", d.ConflictMarker, "yellow — merge conflict markers")
	writeColor("comment_checkbox", d.CommentCheckbox, "yellow — comment checkbox indicator")

	// Syntax highlighting subsection
	b.WriteString("\n[theme.syntax]\n")
	b.WriteString("# Syntax highlighting colors for code. Same format as above.\n")
	b.WriteString("# Each field controls a group of related categories.\n")

	s := DefaultSyntaxTheme
	writeColor("keyword", s.Keyword, "blue — keywords (if, func, return, etc.)")
	writeColor("string", s.String, "gray — string literals")
	writeColor("number", s.Number, "yellow bold — numbers, booleans, nil, constants")
	writeColor("comment", s.Comment, "gray — comments and doc comments")
	writeColor("function", s.Function, "bright blue — function defs and calls")
	writeColor("type", s.Type, "bright magenta — type names")
	writeColor("field", s.Field, "bright cyan — struct/object fields")
	writeColor("operator", s.Operator, "white — operators and punctuation")
	writeColor("tag", s.Tag, "gray — struct tags, attributes, decorators")
	writeColor("namespace", s.Namespace, "default — package/module names")
	writeColor("variable", s.Variable, "default — variables and parameters")

	// Features section
	b.WriteString("\n[features]\n")
	b.WriteString("# Behavioral settings. CLI flags override these.\n")
	b.WriteString(fmt.Sprintf("# hscroll_step       = %d    # columns per horizontal scroll keypress\n", DefaultHScrollStep))
	b.WriteString(fmt.Sprintf("# commit_batch_size  = %d  # commits loaded per batch in log mode\n", DefaultCommitBatchSize))
	b.WriteString(fmt.Sprintf("# auto_snapshots     = %v  # take snapshots automatically (--no-snapshots overrides)\n", DefaultAutoSnapshots))
	b.WriteString(fmt.Sprintf("# show_snapshots     = %v # show snapshot view by default (--snapshots overrides)\n", DefaultShowSnapshots))
	b.WriteString(fmt.Sprintf("# expand_all_budget  = %d  # max total files for full shift-tab expansion\n", DefaultExpandAllBudget))
	b.WriteString(fmt.Sprintf("# chord_timeout_ms   = %d  # milliseconds before a prefix key fires its solo binding\n", DefaultChordTimeoutMs))

	return b.String()
}

// formatTOMLArray formats a string slice as a TOML array literal.
func formatTOMLArray(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
