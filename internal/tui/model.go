package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/inlinediff"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

// inlineDiffKey identifies a specific line pair for caching inline diffs.
type inlineDiffKey struct {
	fileIndex int
	oldNum    int
	newNum    int
}

// commentKey identifies a line that can have a comment.
// Comments attach to the new/left side line number.
type commentKey struct {
	fileIndex  int
	newLineNum int // line number on new/left side
}

// inlineDiffResult stores cached inline diff spans for a line pair.
type inlineDiffResult struct {
	oldSpans []inlinediff.Span
	newSpans []inlinediff.Span
}

// Model represents the application state.
type Model struct {
	// Data - hierarchical: commits contain files
	commits []sidebyside.CommitSet

	// Flattened files from all commits for unified indexing
	// commitFileStarts[i] is the index in files where commit i's files start
	files            []sidebyside.FilePair
	commitFileStarts []int // start index in files for each commit

	fetcher            *content.Fetcher // for fetching full file contents (lazy)
	git                git.Git          // for creating on-demand fetchers in log mode
	truncatedFileCount int              // number of files omitted due to limit

	// Pager mode
	pagerMode bool // true when running as a pager (stdin input, no fetcher)

	// Debug mode
	debugMode bool // true when --debug flag is passed, shows memory/goroutine stats

	// Syntax highlighting
	highlighter         *highlight.Highlighter
	highlightSpans      map[int]*FileHighlight      // file index -> full content highlight spans
	pairsHighlightSpans map[int]*PairsFileHighlight // file index -> pairs-based highlight spans

	// Code structure for breadcrumbs
	structureMaps      map[int]*FileStructure // file index -> full content structure
	pairsStructureMaps map[int]*FileStructure // file index -> pairs-based structure

	// Viewport state
	scroll  int // vertical scroll offset (line index at top of viewport)
	hscroll int // horizontal scroll offset (display columns)
	width   int // terminal width
	height  int // terminal height (viewport height)

	// Configuration
	keys        KeyMap
	hscrollStep int // columns to scroll horizontally per keypress

	// Search state
	searchMode      bool   // true when in search input mode
	searchForward   bool   // true for forward search (/), false for backward (?)
	searchInput     string // current input being typed
	searchQuery     string // executed search query
	searchMatchIdx  int    // index of current match within cursor row on current side (0 = first)
	searchMatchSide int    // which side the current match is on (0 = new/left, 1 = old/right)

	// Multi-key sequence state
	pendingKey string // first key of a multi-key sequence (e.g., "g" waiting for second key)

	// Initial state tracking
	initialFoldSet bool // true once initial fold levels have been determined

	// Derived/cached
	totalLines         int // total number of displayable lines across all files
	maxLineNumSeen     int // largest line number seen (for dynamic gutter width, only grows)
	maxLessWidth       int // max width of less indicator (never shrinks to prevent jittering)
	maxNewContentWidth int // max display width of new-side content (left side, only grows, for dynamic divider)

	// Row cache - avoids rebuilding on every scroll
	cachedRows     []displayRow // cached result of buildRows()
	rowsCacheValid bool         // true if cachedRows is up to date

	// Inline diff cache - avoids recomputing Myers diff on every render
	inlineDiffCache map[inlineDiffKey]inlineDiffResult

	// Loading indicator - shows spinner while files are being fetched/parsed
	spinner        spinner.Model     // animated spinner for loading state
	loadingFiles   map[int]time.Time // file index -> time loading started
	spinnerTicking bool              // true if a spinner tick chain is already running

	// Startup loading - prefetch content for all supported files on startup
	startupQueue      []int // file indices waiting to be loaded on startup
	startupInFlight   int   // number of files currently being fetched
	startupQueuedInit bool  // true once startup queue has been initialized

	// Focus state - true when terminal has focus
	focused bool

	// Focus colour mode - dims content outside current hunk to reduce visual clutter
	focusColour bool

	// Comment state
	commentMode   bool                  // true when editing a comment
	commentInput  string                // text being edited
	commentCursor int                   // cursor position in commentInput (byte offset)
	commentScroll int                   // vertical scroll offset within comment editor
	commentKey    commentKey            // which line is being commented
	comments      map[commentKey]string // stored comments

	// Status message (echo area)
	statusMessage     string    // message to display in status bar
	statusMessageTime time.Time // when the message was set (for auto-clear)
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

// FileStructure stores code structure maps for breadcrumbs and structural diff.
// NewStructure is used for breadcrumbs (current version of the file).
// OldStructure is used for structural diff (comparing old vs new).
type FileStructure struct {
	OldStructure   *structure.Map            // structure for old content (for structural diff)
	NewStructure   *structure.Map            // structure for new content (for breadcrumbs)
	StructuralDiff *structure.StructuralDiff // diff between old and new structure
}

// Option is a function that configures a Model.
type Option func(*Model)

// WithFetcher sets the content fetcher for lazy file content loading.
func WithFetcher(f *content.Fetcher) Option {
	return func(m *Model) {
		m.fetcher = f
	}
}

// WithGit sets the git interface for on-demand content fetching in log mode.
// This allows creating per-commit fetchers when expanding files.
func WithGit(g git.Git) Option {
	return func(m *Model) {
		m.git = g
	}
}

// WithPagerMode enables pager mode, which limits functionality since
// full file content cannot be fetched from git.
func WithPagerMode() Option {
	return func(m *Model) {
		m.pagerMode = true
	}
}

// WithDebugMode enables debug mode, which displays memory and goroutine
// stats in the status bar.
func WithDebugMode() Option {
	return func(m *Model) {
		m.debugMode = true
	}
}

// WithTruncatedFileCount sets the number of files that were truncated.
func WithTruncatedFileCount(count int) Option {
	return func(m *Model) {
		m.truncatedFileCount = count
		// Also update the commit set if present
		if len(m.commits) > 0 {
			m.commits[0].TruncatedFileCount = count
		}
	}
}

// New creates a new Model with the given file pairs.
// This wraps files in a single CommitSet for backward compatibility.
func New(files []sidebyside.FilePair, opts ...Option) Model {
	// Wrap files in a CommitSet
	commit := sidebyside.CommitSet{
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal, // Start with files visible
		FilesLoaded: true,                    // Files are already provided
	}
	return NewWithCommits([]sidebyside.CommitSet{commit}, opts...)
}

// NewWithCommits creates a new Model with the given commit sets.
// Use this for log view or when commit metadata is available.
func NewWithCommits(commits []sidebyside.CommitSet, opts ...Option) Model {
	// Initialize spinner with compact style and slower speed
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Spinner.FPS = time.Second / 6 // 6 fps

	m := Model{
		commits:             commits,
		keys:                DefaultKeyMap(),
		hscrollStep:         DefaultHScrollStep,
		highlighter:         highlight.New(),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		spinner:             s,
		loadingFiles:        make(map[int]time.Time),
		focused:             true,
		focusColour:         true,
		comments:            make(map[commentKey]string),
	}

	// Flatten files from all commits and track boundaries
	m.commitFileStarts = make([]int, len(commits))
	for i, c := range commits {
		m.commitFileStarts[i] = len(m.files)
		m.files = append(m.files, c.Files...)
		m.truncatedFileCount += c.TruncatedFileCount
	}

	for _, opt := range opts {
		opt(&m)
	}
	m.calculateTotalLines()

	// Synchronously highlight the first file so initial render has highlighting.
	// The rest will be highlighted async in Init().
	if len(m.files) > 0 {
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
	var prevOld, prevNew int
	for i, pair := range fp.Pairs {
		if i > 0 {
			// Check for gap in line numbers (hunk boundary)
			oldGap := prevOld > 0 && pair.Old.Num > 0 && pair.Old.Num > prevOld+1
			newGap := prevNew > 0 && pair.New.Num > 0 && pair.New.Num > prevNew+1
			if oldGap || newGap {
				count++
			}
		}
		if pair.Old.Num > 0 {
			prevOld = pair.Old.Num
		}
		if pair.New.Num > 0 {
			prevNew = pair.New.Num
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

// commentMaxVisibleLines returns the maximum number of input lines to show.
// This is max(10, 20% of viewport height).
func (m Model) commentMaxVisibleLines() int {
	twentyPercent := m.height * 20 / 100
	if twentyPercent < 10 {
		return 10
	}
	return twentyPercent
}

// commentPromptHeight returns the number of lines needed for the comment prompt.
// Returns 1 when not in comment mode (for normal status bar).
func (m Model) commentPromptHeight() int {
	if !m.commentMode {
		return 1
	}
	// Count newlines in the input, plus 1 for the current line
	totalLines := strings.Count(m.commentInput, "\n") + 1
	maxVisible := m.commentMaxVisibleLines()

	// Calculate visible content lines
	visibleLines := totalLines
	if visibleLines > maxVisible {
		visibleLines = maxVisible
	}

	// Add scroll indicators if content exceeds visible area
	extraLines := 0
	if totalLines > maxVisible {
		// Check if there's content above (scroll > 0)
		if m.commentScroll > 0 {
			extraLines++
		}
		// Check if there's content below
		if m.commentScroll+maxVisible < totalLines {
			extraLines++
		}
	}

	// Add 1 for the help line at the bottom
	return visibleLines + extraLines + 1
}

// baseContentHeight returns the height available for content without the comment prompt.
// This is used for cursor calculations to keep them stable when comment mode is active.
func (m Model) baseContentHeight() int {
	reserved := 3 // file line + divider + bottom bar
	if m.hasCommitInfo() {
		reserved++ // commit info line in top bar
	}
	h := m.height - reserved
	if h < 1 {
		return 1
	}
	return h
}

// contentHeight returns the height available for rendering content.
// This accounts for the comment prompt when in comment mode.
func (m Model) contentHeight() int {
	h := m.baseContentHeight()
	// Subtract space for comment prompt when in comment mode
	// (bottom bar is already counted in baseContentHeight)
	if m.commentMode {
		h -= m.commentPromptHeight() - 1 // -1 since bottom bar already counted
	}
	if h < 1 {
		return 1
	}
	return h
}

// cursorOffset returns the preferred offset from the top of the viewport where the cursor sits.
// This is 20% of the base content height (ignores comment prompt to keep cursor stable).
// Note: The actual cursor position may be higher when near the top of content - see cursorViewportRow().
func (m Model) cursorOffset() int {
	return m.baseContentHeight() * 20 / 100
}

// cursorViewportRow returns the actual row in the viewport where the cursor appears.
// Near the top of content, the cursor moves up to keep content at the top of screen.
// Once scroll reaches cursorOffset, the cursor stays at the fixed 20% position.
func (m Model) cursorViewportRow() int {
	offset := m.cursorOffset()
	if m.scroll < offset {
		return m.scroll
	}
	return offset
}

// contentStartLine returns which line of content appears at the top of the viewport.
// Near the top, content starts at line 0. Once scroll exceeds cursorOffset,
// content scrolls up and this returns a positive value.
func (m Model) contentStartLine() int {
	offset := m.cursorOffset()
	if m.scroll <= offset {
		return 0
	}
	return m.scroll - offset
}

// cursorLine returns the display row index that the cursor points to.
// In the new cursor model, scroll directly represents the cursor line.
func (m Model) cursorLine() int {
	return m.scroll
}

// minScroll returns the minimum valid scroll offset.
// The cursor can reach the first content row (row 0, the header).
func (m Model) minScroll() int {
	return 0
}

// maxScroll returns the maximum valid scroll offset.
// This allows the cursor to reach the last line of content.
func (m Model) maxScroll() int {
	if m.totalLines == 0 {
		return 0
	}
	return m.totalLines - 1
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
	Breadcrumbs string               // code structure breadcrumb (e.g., "type MyStruct > func myMethod")
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

		// Calculate per-commit file number and total
		if len(m.commits) > 0 && len(m.commitFileStarts) > 0 {
			commitIdx := m.commitForFile(fileIdx)
			startIdx := m.commitFileStarts[commitIdx]
			endIdx := len(m.files)
			if commitIdx+1 < len(m.commits) {
				endIdx = m.commitFileStarts[commitIdx+1]
			}
			info.CurrentFile = fileIdx - startIdx + 1
			info.TotalFiles = endIdx - startIdx
		} else {
			// Legacy mode: use global file index
			info.CurrentFile = fileIdx + 1
		}

		info.FileName = formatFilePath(fp.OldPath, fp.NewPath)
		info.FoldLevel = fp.FoldLevel
		info.FileStatus = string(fileStatusFromPair(fp))
		info.Added, info.Removed = countFileStats(fp)

		// Get breadcrumbs for current source line
		info.Breadcrumbs = m.getBreadcrumbsForCursor(fileIdx, cursorPos)
	}
	// Summary row: leave file-specific fields at zero values (no file info shown)

	return info
}

// getBreadcrumbsForCursor returns formatted breadcrumbs for the cursor position.
// fileIdx is 0-based, cursorPos is the display row index.
func (m Model) getBreadcrumbsForCursor(fileIdx int, cursorPos int) string {
	// Get the display row at cursor position (use cache if valid)
	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}
	if cursorPos < 0 || cursorPos >= len(rows) {
		return ""
	}

	row := rows[cursorPos]

	// Only show breadcrumbs for content rows and certain separator rows
	// (not headers, separator tops, structural diff rows, etc.)
	if row.isHeader || row.isSeparatorTop ||
		row.isBlank || row.isHeaderSpacer || row.isHeaderTopBorder ||
		row.isStructuralDiff {
		return ""
	}

	// For separator and separator bottom rows, use the chunk's start line for breadcrumbs
	// This shows the breadcrumb when cursor is on or below the breadcrumb line in the separator
	if row.isSeparator || row.isSeparatorBottom {
		if row.chunkStartLine <= 0 {
			return ""
		}
		entries := m.getStructureAtLine(row.fileIndex, row.chunkStartLine)
		if len(entries) == 0 {
			return ""
		}
		// Use 0 for compact default (status bar will truncate as needed)
		return formatBreadcrumbs(entries, 0)
	}

	// For comment rows, use the line number the comment belongs to
	if row.kind == RowKindComment {
		if row.commentLineNum <= 0 {
			return ""
		}
		entries := m.getStructureAtLine(row.fileIndex, row.commentLineNum)
		if len(entries) == 0 {
			return ""
		}
		return formatBreadcrumbs(entries, 0)
	}

	// Get source line number from the new side only
	// Don't show breadcrumbs for deleted lines to avoid confusion
	if row.pair.New.Num <= 0 {
		return ""
	}
	sourceLine := row.pair.New.Num

	// Look up structure for this file (new side only)
	entries := m.getStructureAtLine(fileIdx, sourceLine)
	if len(entries) == 0 {
		return ""
	}

	// Use 0 for compact default (status bar will truncate as needed)
	return formatBreadcrumbs(entries, 0)
}

// getStructureAtLine returns structure entries containing the given line.
// Only works when full file content has been loaded (expanded view).
// Returns nil for files that only have pairs-based content.
func (m Model) getStructureAtLine(fileIdx int, lineNum int) []structure.Entry {
	// Only full-content structure is available - pairs-based structure can't be used
	// because line numbers would be relative to concatenated hunk content, not source.
	fs, ok := m.structureMaps[fileIdx]
	if !ok || fs == nil || fs.NewStructure == nil {
		return nil
	}

	return fs.NewStructure.AtLine(lineNum)
}

// formatBreadcrumbs formats structure entries as a breadcrumb string.
// Entries are expected to be ordered from outermost to innermost.
// maxWidth controls signature truncation (0 = use compact default).
// Output format: "type MyStruct > func (m Model) myMethod(ctx) -> error"
func formatBreadcrumbs(entries []structure.Entry, maxWidth int) string {
	if len(entries) == 0 {
		return ""
	}

	// Calculate width budget for each entry's signature
	// Reserve space for kind prefix and separators
	separatorWidth := 3 // " > "
	totalSeparators := len(entries) - 1
	reservedWidth := totalSeparators * separatorWidth

	// Rough estimate: divide remaining width among entries
	// (In practice, outer entries like "class Foo" are short, inner ones need more space)
	sigWidth := 0
	if maxWidth > 0 && len(entries) > 0 {
		// Give most of the budget to the innermost entry (last one)
		sigWidth = maxWidth - reservedWidth
		for _, e := range entries[:len(entries)-1] {
			// Estimate width for outer entries (kind + name + some buffer)
			outerWidth := len(e.Kind) + 1 + len(e.Name) + 5
			sigWidth -= outerWidth
		}
		if sigWidth < 20 {
			sigWidth = 20 // minimum reasonable width
		}
	}

	var parts []string
	for i, e := range entries {
		var part string
		// Use full width budget for innermost entry, compact for others
		entryWidth := 0
		if i == len(entries)-1 {
			// Subtract the "kind " prefix from available width
			kindPrefixLen := len(e.Kind) + 1
			entryWidth = sigWidth - kindPrefixLen
			if entryWidth < 0 {
				entryWidth = 0
			}
		}
		sig := e.FormatSignature(entryWidth)
		if sig != "" {
			part = e.Kind + " " + sig
		} else {
			part = e.Kind + " " + e.Name
		}
		parts = append(parts, part)
	}

	// Join with separator
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " > "
		}
		result += part
	}
	return result
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
	// Commit header or summary row has fileIndex = -1
	// Return 0 to indicate no specific file is selected
	if row.fileIndex < 0 {
		return 0, ""
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

// updateMaxNewContentWidth scans visible content to update maxNewContentWidth.
// Only grows, never shrinks (prevents jitter when divider position changes).
// Measures new-side content (displayed on the left).
func (m *Model) updateMaxNewContentWidth() {
	for _, fp := range m.files {
		if fp.FoldLevel == sidebyside.FoldFolded {
			continue // no content visible when folded
		}

		if fp.FoldLevel == sidebyside.FoldExpanded && fp.NewContent != nil {
			// Expanded: measure full new content
			for _, line := range fp.NewContent {
				w := displayWidth(expandTabs(line))
				if w > m.maxNewContentWidth {
					m.maxNewContentWidth = w
				}
			}
		} else {
			// Normal: measure pairs
			for _, pair := range fp.Pairs {
				if pair.New.Content != "" {
					w := displayWidth(expandTabs(pair.New.Content))
					if w > m.maxNewContentWidth {
						m.maxNewContentWidth = w
					}
				}
			}
		}
	}
}

// rebuildRowsCache unconditionally rebuilds the cached rows.
// This also updates totalLines, maxLineNumSeen, and maxNewContentWidth.
func (m *Model) rebuildRowsCache() {
	// Pre-scan files to update maxLineNumSeen BEFORE building rows.
	// This ensures lineNumWidth() returns the correct value during buildRows().
	for _, fp := range m.files {
		for _, pair := range fp.Pairs {
			m.updateMaxLineNum(pair.Old.Num)
			m.updateMaxLineNum(pair.New.Num)
		}
	}

	// Update maxNewContentWidth (only grows, never shrinks to prevent jitter).
	// This enables dynamic divider positioning.
	m.updateMaxNewContentWidth()

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

// currentCommitIndex returns the index of the commit the cursor is currently in.
// Returns 0 if there are no commits or cursor position is invalid.
func (m *Model) currentCommitIndex() int {
	if len(m.commits) == 0 {
		return 0
	}

	// Get the row at the cursor position
	rows := m.getRows()
	cursorPos := m.cursorLine()
	if cursorPos >= 0 && cursorPos < len(rows) {
		row := rows[cursorPos]
		// For file rows, use fileIndex to determine the commit
		// This works because file rows have fileIndex set but may not have commitIndex set
		if row.fileIndex >= 0 {
			return m.commitForFile(row.fileIndex)
		}
		// For commit header/body rows, use commitIndex directly
		if row.commitIndex >= 0 && row.commitIndex < len(m.commits) {
			return row.commitIndex
		}
	}

	return 0
}

// currentCommit returns the commit set the cursor is currently in.
// Uses the cursor position to determine which commit is displayed.
func (m *Model) currentCommit() *sidebyside.CommitSet {
	if len(m.commits) == 0 {
		return nil
	}
	return &m.commits[m.currentCommitIndex()]
}

// commitForFile returns the commit index that contains the given file index.
func (m Model) commitForFile(fileIdx int) int {
	for i := len(m.commitFileStarts) - 1; i >= 0; i-- {
		if fileIdx >= m.commitFileStarts[i] {
			return i
		}
	}
	return 0
}

// isFirstFileInCommit returns true if fileIdx is the first file in its commit.
func (m Model) isFirstFileInCommit(fileIdx int) bool {
	commitIdx := m.commitForFile(fileIdx)
	return fileIdx == m.commitFileStarts[commitIdx]
}

// hasCommitInfo returns true if any commit in the view has metadata.
// This is used to determine whether to reserve space for the commit line in the top bar.
func (m Model) hasCommitInfo() bool {
	for i := range m.commits {
		if m.commits[i].Info.HasMetadata() {
			return true
		}
	}
	return false
}

// getFocusPredicate returns a predicate function that determines if a row should be in focus.
// Returns nil if focus mode shouldn't apply (focusColour disabled or cursor not on focusable row).
// When non-nil, rows where the predicate returns false will be dimmed.
func (m Model) getFocusPredicate() func(rowIdx int, row displayRow) bool {
	if !m.focusColour {
		return nil
	}

	rows := m.cachedRows
	if !m.rowsCacheValid || len(rows) == 0 {
		return nil
	}

	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return nil
	}

	cursorRow := rows[cursorPos]

	switch cursorRow.kind {
	case RowKindContent, RowKindSeparatorTop, RowKindSeparator, RowKindSeparatorBottom:
		// Focus on current hunk + file header + all hunk separators for navigation
		start, end, fileIdx := m.getHunkBounds(cursorPos, cursorRow.fileIndex)
		return func(rowIdx int, row displayRow) bool {
			// Only consider rows in the current file
			if row.fileIndex != fileIdx {
				return false
			}

			switch row.kind {
			// File header rows stay in focus
			case RowKindHeader, RowKindHeaderSpacer, RowKindHeaderTopBorder:
				return true
			// Structural diff rows (under file header) stay in focus
			case RowKindStructuralDiff:
				return true
			// All hunk separators in the file stay in focus for navigation
			case RowKindSeparatorTop, RowKindSeparator, RowKindSeparatorBottom:
				return true
			}

			// Current hunk content
			return rowIdx >= start && rowIdx < end
		}

	default:
		// No focus mode for other row types (yet)
		return nil
	}
}

// focusProximityThreshold is the number of lines within which nearby hunks
// are also included in the focus area.
const focusProximityThreshold = 15

// getHunkBounds returns the range of row indices for the current hunk.
// Returns (start, end, fileIdx) where start is inclusive and end is exclusive.
// The range includes the hunk's content and its preceding separator rows.
// Special case: SeparatorTop is treated as the bottom border of the hunk ABOVE.
// Also expands to include nearby hunks within focusProximityThreshold lines.
func (m Model) getHunkBounds(cursorPos, fileIdx int) (int, int, int) {
	rows := m.cachedRows
	cursorKind := rows[cursorPos].kind

	var start, end int

	// Special case: SeparatorTop belongs to the hunk above, not below.
	// It acts as the bottom border of the previous hunk.
	if cursorKind == RowKindSeparatorTop {
		// Scan backward through content to find the start of the hunk above
		start = cursorPos
		for start > 0 {
			prevRow := rows[start-1]
			if prevRow.fileIndex != fileIdx {
				break
			}
			if prevRow.kind == RowKindHeader || prevRow.kind == RowKindHeaderSpacer || prevRow.kind == RowKindHeaderTopBorder {
				break
			}
			if prevRow.kind == RowKindContent {
				start--
				continue
			}
			// Stop at the previous separator block
			if prevRow.kind == RowKindSeparatorTop {
				start-- // include the previous SeparatorTop
				break
			}
			if prevRow.kind == RowKindSeparator || prevRow.kind == RowKindSeparatorBottom {
				// Skip past the separator block to find its SeparatorTop
				start--
				continue
			}
			break
		}
		// End is just after the SeparatorTop we're on
		end = cursorPos + 1
	} else {
		// Normal case: scan backward to find the start of the current hunk.
		// The hunk includes Separator and SeparatorBottom above it, but NOT SeparatorTop
		// (since SeparatorTop belongs to the hunk above).
		start = cursorPos
		for start > 0 {
			prevRow := rows[start-1]
			// Stop if we hit a different file
			if prevRow.fileIndex != fileIdx {
				break
			}
			// Stop if we hit a header row (we're at the first hunk)
			if prevRow.kind == RowKindHeader || prevRow.kind == RowKindHeaderSpacer || prevRow.kind == RowKindHeaderTopBorder {
				break
			}
			// Include content rows
			if prevRow.kind == RowKindContent {
				start--
				continue
			}
			// Include Separator and SeparatorBottom (they belong to this hunk)
			if prevRow.kind == RowKindSeparatorBottom || prevRow.kind == RowKindSeparator {
				start--
				continue
			}
			// Stop at SeparatorTop (it belongs to the hunk above)
			if prevRow.kind == RowKindSeparatorTop {
				break
			}
			break
		}

		// Scan forward to find the end of the current hunk.
		// If cursor is on Separator or SeparatorBottom, skip past them to reach content.
		// The hunk ends when we hit SeparatorTop (which belongs to the next hunk's above).
		end = cursorPos + 1
		inSeparatorBlock := cursorKind == RowKindSeparator || cursorKind == RowKindSeparatorBottom

		for end < len(rows) {
			nextRow := rows[end]
			// Stop if we hit a different file
			if nextRow.fileIndex != fileIdx {
				break
			}
			// Stop if we hit blank rows (inter-file spacing)
			if nextRow.kind == RowKindBlank {
				break
			}
			// SeparatorTop belongs to the hunk above, so stop here
			if nextRow.kind == RowKindSeparatorTop {
				break
			}
			// Handle Separator and SeparatorBottom
			if nextRow.kind == RowKindSeparator || nextRow.kind == RowKindSeparatorBottom {
				if inSeparatorBlock {
					// Still in the initial separator block, keep going
					end++
					continue
				}
				// Hit a new separator block, include Separator and SeparatorBottom
				// (they're the bottom border of current hunk)
				end++
				continue
			}
			// Content row - we've exited any initial separator block
			if nextRow.kind == RowKindContent {
				inSeparatorBlock = false
				end++
			} else {
				break
			}
		}
	}

	// Expand to include nearby hunks within the proximity threshold
	start, end = m.expandToNearbyHunks(start, end, fileIdx)

	return start, end, fileIdx
}

// expandToNearbyHunks expands the focus bounds to include hunks within
// focusProximityThreshold SOURCE lines of the current bounds.
// Uses actual line numbers from the code, not display row indices.
func (m Model) expandToNearbyHunks(start, end, fileIdx int) (int, int) {
	rows := m.cachedRows

	// Get the source line range of current focus area
	minLine, maxLine := m.getSourceLineRange(start, end)
	if minLine == 0 && maxLine == 0 {
		return start, end
	}

	// Keep expanding until no more nearby hunks are found
	for {
		expanded := false

		// Check for hunk above within threshold (by source line numbers)
		if start > 0 {
			// Find the last content row of the hunk above
			scanPos := start - 1
			// Skip past separator rows
			for scanPos >= 0 && rows[scanPos].fileIndex == fileIdx {
				row := rows[scanPos]
				if row.kind == RowKindSeparatorTop || row.kind == RowKindSeparator || row.kind == RowKindSeparatorBottom {
					scanPos--
				} else {
					break
				}
			}
			// Check if there's content above
			if scanPos >= 0 && rows[scanPos].fileIndex == fileIdx && rows[scanPos].kind == RowKindContent {
				// Get the max line number of the hunk above
				hunkAboveEnd := m.findHunkEnd(scanPos, fileIdx)
				_, hunkAboveMaxLine := m.getSourceLineRange(scanPos, hunkAboveEnd)
				// Check if it's within threshold of our min line
				if hunkAboveMaxLine > 0 && minLine-hunkAboveMaxLine <= focusProximityThreshold {
					newStart := m.findHunkStart(scanPos, fileIdx)
					if newStart < start {
						start = newStart
						// Update minLine for next iteration
						newMinLine, _ := m.getSourceLineRange(start, end)
						if newMinLine > 0 {
							minLine = newMinLine
						}
						expanded = true
					}
				}
			}
		}

		// Check for hunk below within threshold (by source line numbers)
		if end < len(rows) {
			// Find the first content row of the hunk below
			scanPos := end
			// Skip past separator rows
			for scanPos < len(rows) && rows[scanPos].fileIndex == fileIdx {
				row := rows[scanPos]
				if row.kind == RowKindSeparatorTop || row.kind == RowKindSeparator || row.kind == RowKindSeparatorBottom {
					scanPos++
				} else {
					break
				}
			}
			// Check if there's content below
			if scanPos < len(rows) && rows[scanPos].fileIndex == fileIdx && rows[scanPos].kind == RowKindContent {
				// Get the min line number of the hunk below
				hunkBelowStart := m.findHunkStart(scanPos, fileIdx)
				hunkBelowMinLine, _ := m.getSourceLineRange(hunkBelowStart, scanPos+1)
				// Check if it's within threshold of our max line
				if hunkBelowMinLine > 0 && hunkBelowMinLine-maxLine <= focusProximityThreshold {
					newEnd := m.findHunkEnd(scanPos, fileIdx)
					if newEnd > end {
						end = newEnd
						// Update maxLine for next iteration
						_, newMaxLine := m.getSourceLineRange(start, end)
						if newMaxLine > 0 {
							maxLine = newMaxLine
						}
						expanded = true
					}
				}
			}
		}

		if !expanded {
			break
		}
	}

	return start, end
}

// getSourceLineRange returns the min and max source line numbers in the given row range.
// Uses the New side line numbers (left side in the UI).
func (m Model) getSourceLineRange(start, end int) (minLine, maxLine int) {
	rows := m.cachedRows
	for i := start; i < end && i < len(rows); i++ {
		row := rows[i]
		if row.kind == RowKindContent {
			if row.pair.New.Num > 0 {
				if minLine == 0 || row.pair.New.Num < minLine {
					minLine = row.pair.New.Num
				}
				if row.pair.New.Num > maxLine {
					maxLine = row.pair.New.Num
				}
			}
			// Also check Old side for deleted-only lines
			if row.pair.Old.Num > 0 {
				if minLine == 0 || row.pair.Old.Num < minLine {
					minLine = row.pair.Old.Num
				}
				if row.pair.Old.Num > maxLine {
					maxLine = row.pair.Old.Num
				}
			}
		}
	}
	return minLine, maxLine
}

// findHunkStart finds the start of the hunk containing the given position.
func (m Model) findHunkStart(pos, fileIdx int) int {
	rows := m.cachedRows
	start := pos
	for start > 0 {
		prevRow := rows[start-1]
		if prevRow.fileIndex != fileIdx {
			break
		}
		if prevRow.kind == RowKindHeader || prevRow.kind == RowKindHeaderSpacer || prevRow.kind == RowKindHeaderTopBorder {
			break
		}
		if prevRow.kind == RowKindContent {
			start--
			continue
		}
		if prevRow.kind == RowKindSeparatorBottom || prevRow.kind == RowKindSeparator {
			start--
			continue
		}
		if prevRow.kind == RowKindSeparatorTop {
			break
		}
		break
	}
	return start
}

// findHunkEnd finds the end of the hunk containing the given position.
func (m Model) findHunkEnd(pos, fileIdx int) int {
	rows := m.cachedRows
	end := pos + 1
	for end < len(rows) {
		nextRow := rows[end]
		if nextRow.fileIndex != fileIdx {
			break
		}
		if nextRow.kind == RowKindBlank {
			break
		}
		if nextRow.kind == RowKindSeparatorTop {
			break
		}
		if nextRow.kind == RowKindContent {
			end++
		} else {
			break
		}
	}
	return end
}
