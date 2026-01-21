package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// Model represents the application state.
type Model struct {
	// Data
	files              []sidebyside.FilePair
	fetcher            *content.Fetcher // for fetching full file contents (lazy)
	truncatedFileCount int              // number of files omitted due to limit

	// Pager mode
	pagerMode bool // true when running as a pager (stdin input, no fetcher)

	// Syntax highlighting
	highlighter         *highlight.Highlighter
	highlightSpans      map[int]*FileHighlight      // file index -> full content highlight spans
	pairsHighlightSpans map[int]*PairsFileHighlight // file index -> pairs-based highlight spans

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

	// Multi-key sequence state
	pendingKey string // first key of a multi-key sequence (e.g., "g" waiting for second key)

	// Initial state tracking
	initialFoldSet bool // true once initial fold levels have been determined

	// Derived/cached
	totalLines     int // total number of displayable lines across all files
	maxLineNumSeen int // largest line number seen (for dynamic gutter width, only grows)
	maxLessWidth   int // max width of less indicator (never shrinks to prevent jittering)

	// Row cache - avoids rebuilding on every scroll
	cachedRows     []displayRow // cached result of buildRows()
	rowsCacheValid bool         // true if cachedRows is up to date
}

// DefaultHScrollStep is the default number of columns to scroll horizontally.
const DefaultHScrollStep = 4

// FileHighlight stores syntax highlighting spans for a file's old and new content.
type FileHighlight struct {
	OldSpans []highlight.Span // spans for old content
	NewSpans []highlight.Span // spans for new content
}

// PairsFileHighlight stores syntax highlighting spans derived from diff Pairs.
// Unlike FileHighlight which has spans for full file content, this has spans
// for the concatenated lines from Pairs, with a mapping from line numbers back
// to positions in the concatenated content.
type PairsFileHighlight struct {
	OldSpans      []highlight.Span // spans for concatenated old lines
	NewSpans      []highlight.Span // spans for concatenated new lines
	OldLineStarts map[int]int      // line number -> byte offset in concatenated old content
	NewLineStarts map[int]int      // line number -> byte offset in concatenated new content
	OldLineLens   map[int]int      // line number -> length of line content
	NewLineLens   map[int]int      // line number -> length of line content
}

// Option is a function that configures a Model.
type Option func(*Model)

// WithFetcher sets the content fetcher for lazy file content loading.
func WithFetcher(f *content.Fetcher) Option {
	return func(m *Model) {
		m.fetcher = f
	}
}

// WithPagerMode enables pager mode, which limits functionality since
// full file content cannot be fetched from git.
func WithPagerMode() Option {
	return func(m *Model) {
		m.pagerMode = true
	}
}

// WithTruncatedFileCount sets the number of files that were truncated.
func WithTruncatedFileCount(count int) Option {
	return func(m *Model) {
		m.truncatedFileCount = count
	}
}

// New creates a new Model with the given file pairs.
func New(files []sidebyside.FilePair, opts ...Option) Model {
	m := Model{
		files:               files,
		keys:                DefaultKeyMap(),
		hscrollStep:         DefaultHScrollStep,
		highlighter:         highlight.New(),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
	}
	for _, opt := range opts {
		opt(&m)
	}
	m.calculateTotalLines()

	// Synchronously highlight the first file so initial render has highlighting.
	// The rest will be highlighted async in Init().
	if len(files) > 0 {
		m.highlightPairsSync(0)
	}

	return m
}

// estimateNormalRows calculates how many rows would be displayed if all files
// were at FoldNormal level. Used to determine initial fold state.
func (m Model) estimateNormalRows() int {
	total := 0
	for i, fp := range m.files {
		// Header structure: top border (first file only) + header + spacer + trailing border
		if i == 0 {
			total++ // top border for first file
		}
		total += 3 // header + spacer + trailing border

		// Pairs plus hunk separators
		total += len(fp.Pairs)
		total += m.countHunkSeparators(fp)

		// 4 blank lines after content
		total += 4
	}
	// Summary row
	if len(m.files) > 0 {
		total++
	}
	return total
}

// countHunkSeparators counts the number of hunk boundaries in a file's pairs.
func (m Model) countHunkSeparators(fp sidebyside.FilePair) int {
	count := 0
	var prevLeft, prevRight int
	for i, pair := range fp.Pairs {
		if i > 0 {
			// Check for gap in line numbers (hunk boundary)
			leftGap := prevLeft > 0 && pair.Left.Num > 0 && pair.Left.Num > prevLeft+1
			rightGap := prevRight > 0 && pair.Right.Num > 0 && pair.Right.Num > prevRight+1
			if leftGap || rightGap {
				count++
			}
		}
		if pair.Left.Num > 0 {
			prevLeft = pair.Left.Num
		}
		if pair.Right.Num > 0 {
			prevRight = pair.Right.Num
		}
	}
	return count
}

// calculateTotalLines counts total lines including file headers and hunk separators.
// Also updates maxLineNumSeen based on visible line numbers and maxLessWidth for status bar.
func (m *Model) calculateTotalLines() {
	// Rebuild the rows cache, which also updates totalLines, maxLineNumSeen, and maxLessWidth
	m.rebuildRowsCache()
}

// updateMaxLessWidth updates maxLessWidth if the current totalLines would require more width.
// This is called from calculateTotalLines to ensure the less indicator never shrinks.
func (m *Model) updateMaxLessWidth() {
	if m.totalLines == 0 {
		return
	}
	// Calculate width for worst case: "line TOTAL/TOTAL (END)"
	maxIndicator := formatLessIndicator(m.totalLines, m.totalLines, 100, true)
	width := displayWidth(maxIndicator)
	if width > m.maxLessWidth {
		m.maxLessWidth = width
	}
}

// Init implements tea.Model.
// Triggers async highlighting for all files except the first (which is highlighted sync in New).
func (m Model) Init() tea.Cmd {
	if len(m.files) <= 1 {
		return nil // First file already highlighted sync, no more to do
	}
	// Highlight remaining files async
	return m.RequestHighlightFromPairsExcept(map[int]bool{0: true})
}

// contentHeight returns the height available for content (minus top bar, divider, and bottom bar).
func (m Model) contentHeight() int {
	h := m.height - 3 // Reserve 1 line for top bar + 1 line for divider + 1 line for bottom bar
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
	CurrentFile int                  // 1-based index of current file
	TotalFiles  int                  // total number of files
	FileName    string               // name of current file
	CurrentLine int                  // 1-based line position in viewport
	TotalLines  int                  // total lines in diff
	Percentage  int                  // 0-100 percentage through diff
	AtEnd       bool                 // true if scrolled to the end
	FoldLevel   sidebyside.FoldLevel // fold level of current file
	FileStatus  string               // file status (added, deleted, renamed, modified)
	Added       int                  // number of added lines in current file
	Removed     int                  // number of removed lines in current file
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
	fileIdx := m.currentFileIndex()
	if fileIdx >= 0 && fileIdx < len(m.files) {
		fp := m.files[fileIdx]
		info.CurrentFile = fileIdx + 1
		info.FileName = formatFilePath(fp.OldPath, fp.NewPath)
		info.FoldLevel = fp.FoldLevel
		info.FileStatus = string(fileStatus(fp.OldPath, fp.NewPath))
		info.Added, info.Removed = countFileStats(fp)
	}
	// Summary row: leave file-specific fields at zero values (no file info shown)

	return info
}

// fileAtLine returns the file index (1-based) and filename at the given display line.
// Blank separator lines between files belong to the file above them.
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

	// Use cached rows if valid, otherwise rebuild
	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}
	if line >= len(rows) {
		// Shouldn't happen, but handle gracefully
		lastFile := m.files[len(m.files)-1]
		return len(m.files), formatFilePath(lastFile.OldPath, lastFile.NewPath)
	}

	row := rows[line]
	// Summary row has fileIndex = -1, return last file info
	if row.fileIndex < 0 {
		lastFile := m.files[len(m.files)-1]
		return len(m.files), formatFilePath(lastFile.OldPath, lastFile.NewPath)
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

// updateMaxLineNum updates maxLineNumSeen if n is larger (never shrinks).
func (m *Model) updateMaxLineNum(n int) {
	if n > m.maxLineNumSeen {
		m.maxLineNumSeen = n
	}
}

// lineNumWidth returns the width needed for line numbers based on the largest seen.
// Minimum width is 4 to handle typical files up to 9999 lines.
func (m Model) lineNumWidth() int {
	width := 4 // minimum
	n := m.maxLineNumSeen
	for n >= 10000 {
		width++
		n /= 10
	}
	return width
}

// invalidateRowsCache marks the row cache as needing rebuild.
// Call this after any change that affects the row structure (fold changes, content load, etc).
func (m *Model) invalidateRowsCache() {
	m.rowsCacheValid = false
}

// rebuildRowsCache unconditionally rebuilds the cached rows.
// This also updates totalLines and maxLineNumSeen.
func (m *Model) rebuildRowsCache() {
	// Pre-scan files to update maxLineNumSeen BEFORE building rows.
	// This ensures lineNumWidth() returns the correct value during buildRows().
	for _, fp := range m.files {
		for _, pair := range fp.Pairs {
			m.updateMaxLineNum(pair.Left.Num)
			m.updateMaxLineNum(pair.Right.Num)
		}
	}

	m.cachedRows = m.buildRows()
	m.rowsCacheValid = true
	m.totalLines = len(m.cachedRows)
	m.updateMaxLessWidth()
}

// getRows returns the cached rows, rebuilding if necessary.
func (m *Model) getRows() []displayRow {
	if !m.rowsCacheValid {
		m.rebuildRowsCache()
	}
	return m.cachedRows
}
