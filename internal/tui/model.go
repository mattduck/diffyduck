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

// maxScroll returns the maximum valid scroll offset.
// We allow scrolling until the last line is at the TOP of the viewport,
// which enables the status bar to show the correct file when viewing the end.
func (m Model) maxScroll() int {
	max := m.totalLines - 1
	if max < 0 {
		return 0
	}
	return max
}

// clampScroll ensures scroll is within valid bounds.
func (m *Model) clampScroll() {
	if m.scroll < 0 {
		m.scroll = 0
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

// StatusInfo computes information for the status bar based on current scroll position.
func (m Model) StatusInfo() StatusInfo {
	info := StatusInfo{
		TotalFiles: len(m.files),
		TotalLines: m.totalLines,
	}

	if m.totalLines == 0 || len(m.files) == 0 {
		return info
	}

	// Calculate current line (1-based, top of viewport)
	info.CurrentLine = m.scroll + 1

	// Calculate percentage
	contentH := m.contentHeight()
	if m.totalLines <= contentH {
		// Everything fits on screen
		info.Percentage = 100
		info.AtEnd = true
	} else {
		maxScroll := m.maxScroll()
		if m.scroll >= maxScroll {
			info.Percentage = 100
			info.AtEnd = true
		} else {
			info.Percentage = (m.scroll * 100) / maxScroll
		}
	}

	// Find which file contains the current scroll position
	linesSoFar := 0
	for i, fp := range m.files {
		fileLines := 1 + len(fp.Pairs) // header + pairs
		if m.scroll < linesSoFar+fileLines {
			info.CurrentFile = i + 1 // 1-based
			info.FileName = formatFilePath(fp.OldPath, fp.NewPath)
			break
		}
		linesSoFar += fileLines
	}

	// Edge case: if scroll is past all content
	if info.CurrentFile == 0 && len(m.files) > 0 {
		info.CurrentFile = len(m.files)
		lastFile := m.files[len(m.files)-1]
		info.FileName = formatFilePath(lastFile.OldPath, lastFile.NewPath)
	}

	return info
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
