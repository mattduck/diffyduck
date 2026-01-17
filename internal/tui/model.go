package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// Model represents the application state.
type Model struct {
	// Data
	files   []sidebyside.FilePair
	fetcher *content.Fetcher // for fetching full file contents (lazy)

	// Viewport state
	scroll  int // vertical scroll offset (line index at top of viewport)
	hscroll int // horizontal scroll offset (display columns)
	width   int // terminal width
	height  int // terminal height (viewport height)

	// Configuration
	keys        KeyMap
	hscrollStep int // columns to scroll horizontally per keypress

	// Search state
	searchMode       bool    // true when in search input mode
	searchForward    bool    // true for forward search (/), false for backward (?)
	searchInput      string  // current input being typed
	searchQuery      string  // executed search query
	matches          []Match // search match positions
	currentMatch     int     // index of current match in matches slice
	lastSearchScroll int     // scroll position when last search navigation occurred

	// Derived/cached
	totalLines int // total number of displayable lines across all files
}

// DefaultHScrollStep is the default number of columns to scroll horizontally.
const DefaultHScrollStep = 4

// Option is a function that configures a Model.
type Option func(*Model)

// WithFetcher sets the content fetcher for lazy file content loading.
func WithFetcher(f *content.Fetcher) Option {
	return func(m *Model) {
		m.fetcher = f
	}
}

// New creates a new Model with the given file pairs.
func New(files []sidebyside.FilePair, opts ...Option) Model {
	m := Model{
		files:       files,
		keys:        DefaultKeyMap(),
		hscrollStep: DefaultHScrollStep,
	}
	for _, opt := range opts {
		opt(&m)
	}
	m.calculateTotalLines()
	return m
}

// calculateTotalLines counts total lines including file headers and hunk separators.
func (m *Model) calculateTotalLines() {
	m.totalLines = len(m.buildRows())
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// contentHeight returns the height available for content (minus status bar).
func (m Model) contentHeight() int {
	h := m.height - 1 // Reserve 1 line for status bar
	if h < 1 {
		return 1
	}
	return h
}

// cursorOffset returns the fixed offset from the top of the viewport where the cursor sits.
// This is 20% of the content height.
func (m Model) cursorOffset() int {
	return m.contentHeight() * 20 / 100
}

// cursorLine returns the display row index that the cursor points to.
// This is the scroll position plus the cursor offset.
func (m Model) cursorLine() int {
	return m.scroll + m.cursorOffset()
}

// minScroll returns the minimum valid scroll offset.
// This is negative, allowing the cursor to reach the first line of content.
// When scroll = minScroll, cursor (at cursorOffset) points to line 0.
func (m Model) minScroll() int {
	return -m.cursorOffset()
}

// maxScroll returns the maximum valid scroll offset.
// This allows the cursor to reach the last line of content.
// When scroll = maxScroll, cursor points to (totalLines - 1).
func (m Model) maxScroll() int {
	if m.totalLines == 0 {
		return 0
	}
	// cursor = scroll + cursorOffset
	// We want cursor to be able to reach totalLines - 1
	// So: totalLines - 1 = scroll + cursorOffset
	// scroll = totalLines - 1 - cursorOffset
	max := m.totalLines - 1 - m.cursorOffset()
	// Don't go below minScroll (handles case where content is smaller than viewport)
	if max < m.minScroll() {
		return m.minScroll()
	}
	return max
}

// clampScroll ensures scroll is within valid bounds.
func (m *Model) clampScroll() {
	if min := m.minScroll(); m.scroll < min {
		m.scroll = min
	}
	if max := m.maxScroll(); m.scroll > max {
		m.scroll = max
	}
}

// StatusInfo contains information for the status bar.
type StatusInfo struct {
	CurrentFile int    // 1-based index of current file
	TotalFiles  int    // total number of files
	FileName    string // name of current file
	CurrentLine int    // 1-based line position in viewport
	TotalLines  int    // total lines in diff
	Percentage  int    // 0-100 percentage through diff
	AtEnd       bool   // true if scrolled to the end
}

// StatusInfo computes information for the status bar based on cursor position.
func (m Model) StatusInfo() StatusInfo {
	info := StatusInfo{
		TotalFiles: len(m.files),
		TotalLines: m.totalLines,
	}

	if m.totalLines == 0 || len(m.files) == 0 {
		return info
	}

	// Use cursor position (not scroll) to determine current file
	cursorPos := m.cursorLine()

	// Calculate current line (1-based, cursor position)
	info.CurrentLine = cursorPos + 1

	// Calculate percentage based on cursor position through the content
	if m.totalLines <= 1 {
		info.Percentage = 100
		info.AtEnd = true
	} else {
		maxCursor := m.totalLines - 1
		if cursorPos >= maxCursor {
			info.Percentage = 100
			info.AtEnd = true
		} else if cursorPos <= 0 {
			info.Percentage = 0
		} else {
			info.Percentage = (cursorPos * 100) / maxCursor
		}
	}

	// Find which file contains the cursor position
	info.CurrentFile, info.FileName = m.fileAtLine(cursorPos)

	return info
}

// fileAtLine returns the file index (1-based) and filename at the given display line.
// For blank separator lines between files, returns the file BELOW (except at the very end).
func (m Model) fileAtLine(line int) (int, string) {
	if len(m.files) == 0 {
		return 0, ""
	}

	// Clamp line to valid range
	if line < 0 {
		line = 0
	}
	if line >= m.totalLines {
		// Past all content - return last file
		lastFile := m.files[len(m.files)-1]
		return len(m.files), formatFilePath(lastFile.OldPath, lastFile.NewPath)
	}

	// Build rows to understand the structure
	rows := m.buildRows()
	if line >= len(rows) {
		// Shouldn't happen, but handle gracefully
		lastFile := m.files[len(m.files)-1]
		return len(m.files), formatFilePath(lastFile.OldPath, lastFile.NewPath)
	}

	row := rows[line]

	// If this is a blank separator line, look at the next row to find the file below
	if row.isBlank && line+1 < len(rows) {
		nextRow := rows[line+1]
		return nextRow.fileIndex + 1, formatFilePath(
			m.files[nextRow.fileIndex].OldPath,
			m.files[nextRow.fileIndex].NewPath,
		)
	}

	return row.fileIndex + 1, formatFilePath(
		m.files[row.fileIndex].OldPath,
		m.files[row.fileIndex].NewPath,
	)
}

// formatFilePath returns a display-friendly file path.
func formatFilePath(oldPath, newPath string) string {
	// Prefer new path unless it's /dev/null (deleted file)
	var path string
	if newPath == "/dev/null" {
		path = oldPath
	} else {
		path = newPath
	}
	// Strip common prefixes
	if len(path) > 2 && (path[:2] == "a/" || path[:2] == "b/") {
		path = path[2:]
	}
	return path
}
