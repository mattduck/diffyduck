package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// Model represents the application state.
type Model struct {
	// Data
	files []sidebyside.FilePair

	// Viewport state
	scroll  int // vertical scroll offset (line index at top of viewport)
	hscroll int // horizontal scroll offset (display columns)
	width   int // terminal width
	height  int // terminal height (viewport height)

	// Configuration
	keys        KeyMap
	hscrollStep int // columns to scroll horizontally per keypress

	// Derived/cached
	totalLines int // total number of displayable lines across all files
}

// DefaultHScrollStep is the default number of columns to scroll horizontally.
const DefaultHScrollStep = 4

// New creates a new Model with the given file pairs.
func New(files []sidebyside.FilePair) Model {
	m := Model{
		files:       files,
		keys:        DefaultKeyMap(),
		hscrollStep: DefaultHScrollStep,
	}
	m.calculateTotalLines()
	return m
}

// calculateTotalLines counts total lines including file headers.
func (m *Model) calculateTotalLines() {
	total := 0
	for _, fp := range m.files {
		total++ // file header line
		total += len(fp.Pairs)
	}
	m.totalLines = total
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// maxScroll returns the maximum valid scroll offset.
func (m Model) maxScroll() int {
	max := m.totalLines - m.height
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
