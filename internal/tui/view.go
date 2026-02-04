package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

var (
	// Styles for different line types
	headerStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	headerDirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))  // dimmer for directory part of file headers
	headerBasenameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // bright, non-bold for file basename
	headerLineStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // for ━ characters in headers
	hunkSeparatorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	addedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	removedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	changedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // blue for modified lines with word diff
	contextStyle        = lipgloss.NewStyle()
	contextDimStyle     = lipgloss.NewStyle().Faint(true) // for context on old side
	lineNumStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	emptyStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle         = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("0"))

	// Inline diff highlight: underlined, bold, and colored to match the diff side
	inlineAddedStyle   = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color("10"))
	inlineRemovedStyle = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color("9"))

	// Inline diff highlight for whitespace characters: add background so spaces are visible
	inlineAddedWhitespaceStyle   = lipgloss.NewStyle().Background(lipgloss.Color("10"))
	inlineRemovedWhitespaceStyle = lipgloss.NewStyle().Background(lipgloss.Color("9"))

	// Search highlight styles (black text on yellow background)
	searchMatchStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3"))
	searchCurrentMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("9"))

	// Cursor highlight style (bg=7 silver, fg=0 black) for gutter areas
	cursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("7")).Foreground(lipgloss.Color("0"))

	// Cursor arrow style (fg=15 bright white, no background)
	cursorArrowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	// Unfocused cursor arrow style (fg=8 gray) - outline arrow when terminal loses focus
	unfocusedCursorArrowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Unfocused status bar style (inverted from normal: fg=8 gray on default bg)
	unfocusedStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Inter-file area style (dim shading for blank lines between files)
	interFileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)

	// Center divider style (between left and right sides)
	centerDividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)

	// Debug mode styles
	debugLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // magenta for labels
	debugValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan for values

	// Comment styles
	commentBorderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // bright white for borders
	commentTextStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // bright white for text
	commentRightDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)

	// Tree hierarchy styles
	commitTreeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow for commit level
)

// determineFileHeaderMode computes the HeaderMode for a file node based on its fold state.
//
// Rules:
//   - Folded → HeaderSingleLine (no borders)
//   - Unfolded → HeaderThreeLine (show bottom border)
func determineFileHeaderMode(isFolded bool, isFirstChild bool, prevSiblingUnfolded bool) HeaderMode {
	_, _ = isFirstChild, prevSiblingUnfolded // kept for API compatibility
	if isFolded {
		return HeaderSingleLine
	}
	return HeaderThreeLine
}

// determineCommitHeaderMode computes the HeaderMode for a commit node.
//
// Rules:
//   - Folded → HeaderSingleLine (no border)
//   - Unfolded → HeaderThreeLine (show border)
func determineCommitHeaderMode(isFolded bool, isFirstCommit bool, prevCommitUnfolded bool) HeaderMode {
	_, _ = isFirstCommit, prevCommitUnfolded // kept for API compatibility
	if isFolded {
		return HeaderSingleLine
	}
	return HeaderThreeLine
}

// buildFileTreePath creates a TreePath for rows belonging to a file.
// This is used for file headers, preview rows, and content rows.
//
// Parameters:
//   - fileIdx: index of the file
//   - isLastFileInCommit: whether this is the last file in its commit
//   - isFileFolded: whether the file is in folded state
//   - kind: TreeRowHeader, TreeRowPreview, or TreeRowContent
//
// Returns a TreePath based on row kind:
//   - TreeRowHeader: no ancestors, Current is file level (shows branch ├─── or └───)
//   - TreeRowPreview: file as ancestor with IsFolded=false (shows │ based on isLast, not fold state)
//   - TreeRowContent: file as ancestor (shows │ based on fold state)
func (m Model) buildFileTreePath(fileIdx int, isLastFileInCommit, isFileFolded bool, kind TreeRowKind) TreePath {
	// Get file status style
	var fileStyle lipgloss.Style
	if fileIdx >= 0 && fileIdx < len(m.files) {
		status := fileStatusFromPair(m.files[fileIdx])
		_, fileStyle = fileStatusIndicator(status)
	} else {
		fileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue fallback
	}

	switch kind {
	case TreeRowHeader:
		// File header: no ancestors, shows branch character (├─── or └───)
		fileLevel := TreeLevel{
			IsLast:   isLastFileInCommit,
			IsFolded: isFileFolded,
			Style:    fileStyle,
			Depth:    0,
		}
		return TreePath{
			Ancestors: []TreeLevel{},
			Current:   &fileLevel,
		}

	case TreeRowPreview, TreeRowContent:
		// Preview and content rows show sibling continuation only.
		// Shows │ if there are more sibling files below.
		//
		// This produces: │    content
		//                ^
		//                +-- sibling continuation (5 chars)
		siblingLevel := TreeLevel{
			IsLast:   isLastFileInCommit, // controls whether │ shows
			IsFolded: false,
			Style:    fileStyle,
			Depth:    0,
		}
		return TreePath{
			Ancestors: []TreeLevel{siblingLevel},
			Current:   nil,
		}

	default:
		// Fallback to content behavior
		return m.buildFileTreePath(fileIdx, isLastFileInCommit, isFileFolded, TreeRowContent)
	}
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Single window: render normally
	if len(m.windows) <= 1 {
		return m.renderSingleWindowView()
	}

	// Multiple windows: render side by side with vertical divider
	return m.renderMultiWindowView()
}

// renderSingleWindowView renders the view for a single window (original behavior).
func (m Model) renderSingleWindowView() string {
	// Render top bar first to determine its actual height.
	// Top bar height varies: 2 lines (commit + divider) when on commit section,
	// 3 lines (commit + file + divider) when on a file.
	topBar := m.renderTopBar()
	topBarLines := strings.Count(topBar, "\n") + 1

	// Calculate actual available content height based on rendered top bar
	bottomBarLines := 1
	contentH := m.height - topBarLines - bottomBarLines
	if contentH < 1 {
		contentH = 1
	}

	// Use cached rows if available, otherwise rebuild (cache should normally be valid)
	rows := m.w().cachedRows
	if !m.w().rowsCacheValid {
		rows = m.buildRows()
	}

	// Apply scroll and viewport
	visibleRows := m.getVisibleRows(rows, contentH)

	// Pad with empty lines to fill viewport (so bottom bar is always at bottom)
	for len(visibleRows) < contentH {
		visibleRows = append(visibleRows, "")
	}

	// Build final output: top bar + content + bottom bar
	var output []string
	output = append(output, topBar)
	output = append(output, visibleRows...)

	// Add bottom bar (less-style indicator)
	bottomBar := m.renderStatusBar()
	output = append(output, bottomBar)

	return strings.Join(output, "\n")
}

// renderMultiWindowView renders multiple windows side by side with a vertical divider.
func (m Model) renderMultiWindowView() string {
	// Calculate window widths: 50/50 split with 1 char divider
	dividerWidth := 1
	totalContentWidth := m.width - dividerWidth
	leftWidth := totalContentWidth / 2
	rightWidth := totalContentWidth - leftWidth

	// Render each window's content
	leftLines := m.renderWindowContent(0, leftWidth)
	rightLines := m.renderWindowContent(1, rightWidth)

	// Ensure both have the same number of lines
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	for len(leftLines) < maxLines {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < maxLines {
		rightLines = append(rightLines, "")
	}

	// Divider character (full block, dim)
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	divider := dividerStyle.Render("█")

	// Join horizontally: left | divider | right
	var output []string
	for i := 0; i < maxLines; i++ {
		// Pad left side to exact width
		leftPadded := padToWidth(leftLines[i], leftWidth)
		// Pad right side to exact width
		rightPadded := padToWidth(rightLines[i], rightWidth)
		output = append(output, leftPadded+divider+rightPadded)
	}

	return strings.Join(output, "\n")
}

// renderWindowContent renders the content for a specific window at a given width.
// Returns the lines as a slice (top bar, content rows, bottom bar).
func (m Model) renderWindowContent(windowIdx int, windowWidth int) []string {
	// Temporarily switch to this window for rendering
	savedActiveIdx := m.activeWindowIdx
	m.activeWindowIdx = windowIdx
	savedWidth := m.width
	m.width = windowWidth

	// For inactive windows, render as unfocused (use unfocused cursor styling)
	// so the user can tell which window is active
	savedFocused := m.focused
	isActiveWindow := windowIdx == savedActiveIdx
	if !isActiveWindow {
		m.focused = false // Inactive window renders as unfocused
	}

	// Render top bar
	topBar := m.renderTopBar()
	topBarLines := strings.Split(topBar, "\n")

	// Calculate content height
	bottomBarLines := 1
	contentH := m.height - len(topBarLines) - bottomBarLines
	if contentH < 1 {
		contentH = 1
	}

	// Get rows for this window
	rows := m.windows[windowIdx].cachedRows
	if !m.windows[windowIdx].rowsCacheValid {
		rows = m.buildRows()
	}

	// Apply scroll and viewport
	visibleRows := m.getVisibleRows(rows, contentH)

	// Pad content to fill height
	for len(visibleRows) < contentH {
		visibleRows = append(visibleRows, "")
	}

	// Render bottom bar
	bottomBar := m.renderStatusBar()

	// Restore original state
	m.activeWindowIdx = savedActiveIdx
	m.width = savedWidth
	m.focused = savedFocused

	// Combine all lines
	var lines []string
	lines = append(lines, topBarLines...)
	lines = append(lines, visibleRows...)
	lines = append(lines, bottomBar)

	return lines
}

// padToWidth pads or truncates a string to exactly the specified display width.
// Uses ansi.StringWidth because the input string may contain ANSI escape codes.
func padToWidth(s string, width int) string {
	currentWidth := ansi.StringWidth(s)
	if currentWidth == width {
		return s
	}
	if currentWidth < width {
		// Pad with spaces
		return s + strings.Repeat(" ", width-currentWidth)
	}
	// Truncate (need to be careful with ANSI codes)
	return ansi.Truncate(s, width, "")
}

// RowKind identifies the type of a display row.
// Using an enum instead of multiple booleans ensures cursor identity
// logic stays in sync when new row types are added.
type RowKind int

const (
	RowKindContent RowKind = iota // default: content line pair with diff data
	RowKindHeader
	RowKindHeaderSpacer    // bottom border line after header
	RowKindHeaderTopBorder // top border line before header
	RowKindBlank
	RowKindSeparatorTop             // top shader line above hunk separator
	RowKindSeparator                // hunk separator with breadcrumb
	RowKindSeparatorBottom          // bottom shader line below hunk separator
	RowKindTruncationIndicator      // truncation message row
	RowKindBinaryIndicator          // binary file message row
	RowKindCommitHeader             // commit header row (sha, author, date, subject)
	RowKindCommitHeaderTopBorder    // top border line before commit header
	RowKindCommitHeaderBottomBorder // bottom border line after commit header
	RowKindCommitBody               // commit body row (full sha, author, date, message) - legacy, kept for separators
	RowKindCommitInfoHeader         // commit info header with yellow shaders (foldable child node)
	RowKindCommitInfoTopBorder      // top border line before commit info header
	RowKindCommitInfoBottomBorder   // bottom border line after commit info header
	RowKindCommitInfoBody           // commit info body row (Author, Date, message content)
	RowKindComment                  // inline comment row (belongs to line above)
	RowKindStructuralDiff           // structural diff row (added/modified/deleted functions/types)
	RowKindPaginationIndicator      // ellipsis row indicating more commits can be loaded
)

// displayRow represents one row in the view (header, line pair, hunk separator, or blank)
type displayRow struct {
	kind      RowKind // type of row - use this for identity matching
	fileIndex int     // index of the file this row belongs to (-1 for summary row)

	// Legacy boolean flags - kept for backward compatibility during refactor.
	// These are derived from 'kind' and will be removed in a future cleanup.
	isHeader              bool
	isSeparatorTop        bool // top shader line above hunk separator
	isSeparator           bool
	isSeparatorBottom     bool // bottom shader line below hunk separator
	isBlank               bool
	isHeaderSpacer        bool // bottom border line after header
	isHeaderTopBorder     bool // top border line before header
	isTruncationIndicator bool // true if this row shows a truncation message

	headerMode      HeaderMode // how to render the header (single/two/three line)
	isFirstLine     bool       // first line pair in a file (uses ┬ separator)
	isLastLine      bool       // last line pair in a file (uses ┴ separator)
	header          string
	foldLevel       sidebyside.FoldLevel // fold level for headers (used for icon and styling)
	status          FileStatus           // file status (added, deleted, renamed, modified) for headers
	pair            sidebyside.LinePair
	added           int // number of added lines (for headers)
	removed         int // number of removed lines (for headers)
	headerBoxWidth  int // width of the box around header content (for border alignment)
	treePrefixWidth int // width of tree prefix for border alignment
	// Truncation indicator fields
	truncationMessage string // message to display
	truncateOld       bool   // show truncation on left (old) side
	truncateNew       bool   // show truncation on right (new) side
	// Binary file indicator fields
	isBinaryIndicator bool   // true if this row shows a binary file message
	binaryMessage     string // message to display (e.g., "Binary file created")
	binaryOld         bool   // show binary message on left (old) side
	binaryNew         bool   // show binary message on right (new) side
	// Hunk separator fields
	chunkStartLine     int // first line of the following chunk (new/right side), for breadcrumbs
	prevChunkStartLine int // chunkStartLine of the previous separator in this file (for repeat detection)
	// Commit header fields
	isCommitHeader             bool                       // true if this is a commit header row
	isCommitHeaderTopBorder    bool                       // true if this is a commit header top border row
	isCommitHeaderBottomBorder bool                       // true if this is a commit header bottom border row
	commitFoldLevel            sidebyside.CommitFoldLevel // fold level for commit headers
	commitIndex                int                        // which commit this header belongs to
	maxCommitFilesWidth        int                        // max width for file count column across all commits
	maxCommitAddWidth          int                        // max width for additions column across all commits
	maxCommitRemWidth          int                        // max width for removals column across all commits
	maxCommitTimeWidth         int                        // max width for relative time column across all commits
	maxCommitSubjectWidth      int                        // max width for subject column across all commits
	commitHeaderSearchText     string                     // searchable text for commit header rows (SHA + author + subject)
	// Commit body fields (shown when commit is expanded) - legacy, kept for separators
	isCommitBody      bool   // true if this is a commit body row
	commitBodyLine    string // the text content for this body line
	commitBodyIsBlank bool   // true if this is a blank line in the body
	// Commit info fields (foldable child node under commit)
	isCommitInfoHeader       bool // true if this is a commit info header row
	isCommitInfoTopBorder    bool // true if this is a commit info top border row
	isCommitInfoBottomBorder bool // true if this is a commit info bottom border row
	isCommitInfoBody         bool // true if this is a commit info body row
	// Tree hierarchy fields (generic representation)
	treePath TreePath // full path from root to this node (for tree prefix rendering)
	// Legacy tree fields - kept during migration, will be removed in Phase 4
	isLastFileInCommit bool                 // true if this file is the last file in its commit (for tree └─ vs ├─)
	treeTerminator     bool                 // true if this blank row should render ┴ instead of │ (end of tree)
	isFileFolded       bool                 // true if the parent file is folded (hide commit-level tree line)
	commitInfoLine     string               // text content for info body lines
	dateParts          sidebyside.DateParts // structured date parts for styled rendering
	// Comment fields (for RowKindComment rows)
	commentText      string // text of the comment (for rendering)
	commentLineNum   int    // line number this comment belongs to (for association)
	commentRowIndex  int    // index within the comment box (0=top border, 1..n-2=content, n-1=bottom border)
	commentRowCount  int    // total rows in this comment box
	commentLineIndex int    // which line of comment content this is (for content rows, -1 for borders)
	// Structural diff fields (for RowKindStructuralDiff rows)
	isStructuralDiff          bool                 // true if this is a structural diff row
	structuralDiffLine        string               // the formatted line (e.g., "  func FuncA(...)")
	structuralDiffIsBlank     bool                 // true if this is a blank separator line
	structuralDiffChangeKind  structure.ChangeKind // change kind for styling the identifier
	structuralDiffAdded       int                  // lines added within this element
	structuralDiffRemoved     int                  // lines removed within this element
	structuralDiffIsTruncated bool                 // true if this is a "...N more" overflow row
}

// buildCommentRows creates displayRow entries for a comment box.
// contentWidth is the text width available inside the box; lines are word-wrapped to fit.
func buildCommentRows(fileIndex int, lineNum int, comment string, contentWidth int, treePath TreePath) []displayRow {
	if comment == "" {
		return nil
	}

	// Word-wrap each paragraph to fit inside the comment box
	var wrappedLines []string
	for _, para := range strings.Split(comment, "\n") {
		wrappedLines = append(wrappedLines, wrapComment(para, contentWidth)...)
	}

	rowCount := len(wrappedLines) + 2 // content lines + top border + bottom border

	rows := make([]displayRow, rowCount)

	// Top border
	rows[0] = displayRow{
		kind:             RowKindComment,
		fileIndex:        fileIndex,
		commentText:      comment,
		commentLineNum:   lineNum,
		commentRowIndex:  0,
		commentRowCount:  rowCount,
		commentLineIndex: -1, // border, not content
		treePath:         treePath,
	}

	// Content lines
	for i := range wrappedLines {
		rows[i+1] = displayRow{
			kind:             RowKindComment,
			fileIndex:        fileIndex,
			commentText:      comment,
			commentLineNum:   lineNum,
			commentRowIndex:  i + 1,
			commentRowCount:  rowCount,
			commentLineIndex: i,
			treePath:         treePath,
		}
	}

	// Bottom border
	rows[rowCount-1] = displayRow{
		kind:             RowKindComment,
		fileIndex:        fileIndex,
		commentText:      comment,
		commentLineNum:   lineNum,
		commentRowIndex:  rowCount - 1,
		commentRowCount:  rowCount,
		commentLineIndex: -1, // border, not content
		treePath:         treePath,
	}

	return rows
}

// buildRows creates all displayable rows from the model data.
func (m Model) buildRows() []displayRow {
	var rows []displayRow

	// Handle legacy case where Model was created without using New/NewWithCommits
	// (e.g., tests that directly set m.files)
	if len(m.commits) == 0 && len(m.files) > 0 {
		return m.buildRowsLegacy()
	}

	// Calculate consistent header box width for borders from max per-file width
	maxBoxWidth := 0
	for commitIdx, commit := range m.commits {
		if commit.Info.HasMetadata() && m.commitFoldLevel(commitIdx) == sidebyside.CommitFolded {
			continue
		}
		startIdx := m.commitFileStarts[commitIdx]
		endIdx := len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}
		for fileIdx := startIdx; fileIdx < endIdx; fileIdx++ {
			fp := m.files[fileIdx]
			header := formatFileHeader(fp)
			added, removed := countFileStats(fp)
			bw := fileHeaderBoxWidth(header, added, removed)
			if bw > maxBoxWidth {
				maxBoxWidth = bw
			}
		}
	}

	iconPartWidth := 3 + 1 + 1 // "   ◐ "
	headerContentWidth := maxBoxWidth - iconPartWidth
	if headerContentWidth < 0 {
		headerContentWidth = 0
	}

	// Use cached structural diff width (updated on 'r' refresh)
	if m.cachedStructDiffWidth > headerContentWidth {
		headerContentWidth = m.cachedStructDiffWidth
	}

	// Calculate final box width, clamped to 80% of screen width
	headerBoxWidth := iconPartWidth + headerContentWidth
	if m.width > 0 {
		maxAllowedWidth := m.width * 80 / 100
		if headerBoxWidth > maxAllowedWidth && maxAllowedWidth > iconPartWidth {
			headerBoxWidth = maxAllowedWidth
		}
	}

	// Use cached commit column widths (updated on 'r' refresh)
	// Fall back to calculating if not initialized
	maxCommitFilesWidth := m.cachedCommitFileCount
	maxCommitAddWidth := m.cachedCommitAddWidth
	maxCommitRemWidth := m.cachedCommitRemWidth
	maxCommitTimeWidth := m.cachedCommitTimeWidth
	maxCommitSubjectWidth := m.cachedCommitSubjWidth
	if maxCommitFilesWidth == 0 && len(m.commits) > 0 {
		for commitIdx, commit := range m.commits {
			startIdx := m.commitFileStarts[commitIdx]
			endIdx := len(m.files)
			if commitIdx+1 < len(m.commits) {
				endIdx = m.commitFileStarts[commitIdx+1]
			}
			commitFileCount := endIdx - startIdx

			// Calculate stats column widths (matching renderCommitHeaderRow logic)
			var commitAdded, commitRemoved int
			var statsKnown bool
			if commit.StatsLoaded {
				commitAdded = commit.TotalAdded
				commitRemoved = commit.TotalRemoved
				statsKnown = true
			} else {
				// Compute from files (same as render code)
				for i := startIdx; i < endIdx; i++ {
					added, removed := countFileStats(m.files[i])
					commitAdded += added
					commitRemoved += removed
				}
				statsKnown = commitAdded > 0 || commitRemoved > 0 || commitFileCount == 0
			}

			var aw, rw int
			if statsKnown {
				aw = len(fmt.Sprintf("+%d", commitAdded))
				rw = len(fmt.Sprintf("-%d", commitRemoved))
			} else {
				// Stats not loaded yet, use placeholder width ("+?" = 2 chars)
				aw = 2
				rw = 2
			}

			fw := len(fmt.Sprintf("%d", commitFileCount))
			if fw > maxCommitFilesWidth {
				maxCommitFilesWidth = fw
			}
			if aw > maxCommitAddWidth {
				maxCommitAddWidth = aw
			}
			if rw > maxCommitRemWidth {
				maxCommitRemWidth = rw
			}
			tw := len(formatShortRelativeDate(commit.Info.Date))
			if tw > maxCommitTimeWidth {
				maxCommitTimeWidth = tw
			}
			sw := displayWidth(commit.Info.Subject)
			if sw > 120 {
				sw = 120
			}
			if sw > maxCommitSubjectWidth {
				maxCommitSubjectWidth = sw
			}
		}
	}

	// Build rows for each commit
	// Note: The first item's top border is rendered specially in getVisibleRows,
	// not as part of the content rows (so it doesn't affect cursor line numbering).
	for commitIdx, commit := range m.commits {
		// Skip commits outside narrow scope
		if !m.w().narrow.IncludesCommit(commitIdx) {
			continue
		}
		// Add commit header row if commit has metadata
		// Skip commit-level rows when narrowed to file level or commit-info-only
		if commit.Info.HasMetadata() && !m.w().narrow.IsFileLevelOrBelow() && !m.w().narrow.IsCommitInfoOnly() {
			commitFolded := m.commitFoldLevel(commitIdx) == sidebyside.CommitFolded
			isFirstCommit := commitIdx == 0
			prevCommitUnfolded := !isFirstCommit && m.commitFoldLevel(commitIdx-1) != sidebyside.CommitFolded

			// Compute header mode for this commit
			commitHeaderMode := determineCommitHeaderMode(commitFolded, isFirstCommit, prevCommitUnfolded)
			// Border is visible when mode is ThreeLine
			// Subsequent commits get their top border from the previous commit's separator row

			// Calculate actual content width for this commit's header
			// Layout: prefix(1) + icon(1) + space(1) + sha(7) + space(1) + added + space(1)
			//         + removed + space(1) + files + space(1) + time + space(1) + author + space(1) + subject
			startIdx := m.commitFileStarts[commitIdx]
			endIdx := len(m.files)
			if commitIdx+1 < len(m.commits) {
				endIdx = m.commitFileStarts[commitIdx+1]
			}
			fileCount := endIdx - startIdx
			totalAdded := 0
			totalRemoved := 0
			for i := startIdx; i < endIdx; i++ {
				added, removed := countFileStats(m.files[i])
				totalAdded += added
				totalRemoved += removed
			}
			filesWidth := len(fmt.Sprintf("%d", fileCount))
			addedWidth := len(fmt.Sprintf("+%d", totalAdded))
			removedWidth := len(fmt.Sprintf("-%d", totalRemoved))
			timeWidth := len(formatShortRelativeDate(commit.Info.Date))
			authorWidth := displayWidth(commit.Info.Author)
			if authorWidth > 15 {
				authorWidth = 15
			}
			subjectWidth := displayWidth(commit.Info.Subject)
			if subjectWidth > 120 {
				subjectWidth = 120
			}
			// Total: prefix(1) + icon(1) + space(1) + sha(7) + space(1) + added + space(1)
			//        + removed + space(1) + files + space(1) + time + space(1) + author + space(1) + subject
			commitHeaderWidth := 1 + 1 + 1 + 7 + 1 + filesWidth + 1 + addedWidth + 1 + removedWidth + 1 + timeWidth + 1 + authorWidth + 1 + subjectWidth

			// When unfolded, keep shared column widths for alignment but use
			// per-commit subject width so the border hugs actual content
			useHeaderBoxWidth := commitHeaderWidth
			if !commitFolded {
				// Cap subject to what the render will actually display
				// (render truncates to maxCommitSubjectWidth)
				renderSubjWidth := subjectWidth
				if renderSubjWidth > maxCommitSubjectWidth {
					renderSubjWidth = maxCommitSubjectWidth
				}
				// Recompute with shared fixed columns + per-commit subject/author
				useHeaderBoxWidth = 1 + 1 + 1 + 7 + 1 + maxCommitFilesWidth + 1 + maxCommitAddWidth + 1 + maxCommitRemWidth + 1 + maxCommitTimeWidth + 1 + authorWidth + 1 + renderSubjWidth
			}

			rows = append(rows, displayRow{
				kind:                   RowKindCommitHeader,
				fileIndex:              -1,
				isCommitHeader:         true,
				commitFoldLevel:        m.commitFoldLevel(commitIdx),
				commitIndex:            commitIdx,
				maxCommitFilesWidth:    maxCommitFilesWidth,
				maxCommitAddWidth:      maxCommitAddWidth,
				maxCommitRemWidth:      maxCommitRemWidth,
				maxCommitTimeWidth:     maxCommitTimeWidth,
				maxCommitSubjectWidth:  maxCommitSubjectWidth,
				commitHeaderSearchText: commit.Info.ShortSHA() + " " + commit.Info.Author + " " + commit.Info.Subject,
				headerMode:             commitHeaderMode,
				headerBoxWidth:         useHeaderBoxWidth,
			})

			// If commit is folded, skip its files
			if commitFolded {
				continue
			}

			// Unfolded commits produce 2 margin lines before first child:
			// - Line 1: available for commit's bottom border
			// - Line 2: available for first child's top border (commit-info)
			rows = append(rows, displayRow{
				kind:                       RowKindCommitHeaderBottomBorder,
				fileIndex:                  -1,
				isCommitHeaderBottomBorder: true,
				commitIndex:                commitIdx,
				headerMode:                 commitHeaderMode,
				headerBoxWidth:             useHeaderBoxWidth,
			})

			// Calculate commit-info header box width for the top border slot
			treePrefixWidth := treeWidth(0, true) + 1
			iconPartWidth := treePrefixWidth + 2
			headerText := commit.Info.FormattedDate(time.Now())
			infoHeaderBoxWidth := iconPartWidth + displayWidth(headerText) + 2

			// Build tree path for commit-info top border.
			detailsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // grey for details
			// The border sits between commit header and details header, always showing
			// commit trunk continuation (│). Use a level that's never folded/last since
			// at least the details header always follows below this row.
			commitTrunkLevel := TreeLevel{
				IsLast:   false,
				IsFolded: false,
				Style:    detailsStyle,
				Depth:    0,
			}
			detailsBorderTreePath := TreePath{
				Ancestors: []TreeLevel{commitTrunkLevel},
				Current:   nil,
			}

			// Top border slot for commit-info - renders as border when expanded, blank when normal
			rows = append(rows, displayRow{
				kind:                  RowKindCommitInfoTopBorder,
				fileIndex:             -1,
				isCommitInfoTopBorder: true,
				commitIndex:           commitIdx,
				commitFoldLevel:       m.commitFoldLevel(commitIdx),
				headerBoxWidth:        infoHeaderBoxWidth,
				treePrefixWidth:       treePrefixWidth,
				treePath:              detailsBorderTreePath,
			})

			// Add commit info rows (foldable child node under commit)
			rows = append(rows, m.buildCommitInfoRows(&commit, commitIdx)...)
		}

		// When narrowed to commit-info-only, add just the commit info rows
		// (no parent commit header, no files)
		if commit.Info.HasMetadata() && m.w().narrow.IsCommitInfoOnly() {
			rows = append(rows, m.buildCommitInfoRows(&commit, commitIdx)...)
		}

		// Get file range for this commit
		startIdx := m.commitFileStarts[commitIdx]
		endIdx := len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}

		// Add first file's top border slot when commit info is expanded (provides margin).
		// The slot draws into the trailing blank line of the expanded commit info body.
		// Always add this row when commit-info is expanded to prevent content shift;
		// render as border or blank based on first file's mode.
		// Skip when narrowed to file level or commit info only (no files shown).
		if startIdx < endIdx && commit.Info.HasMetadata() && !m.w().narrow.IsFileLevelOrBelow() && !m.w().narrow.IsCommitInfoOnly() {
			commitInfoExpanded := m.commitFoldLevel(commitIdx) == sidebyside.CommitExpanded
			if commitInfoExpanded {
				firstFileFolded := m.fileFoldLevel(startIdx) == sidebyside.FoldFolded
				// First file's prev sibling is commit-info, not another file
				firstFileHeaderMode := determineFileHeaderMode(firstFileFolded, false, commitInfoExpanded)
				firstIsLastFile := startIdx == endIdx-1
				// Force IsLast=false so │ continuation shows on the top border row;
				// the branch point (├/└) appears on the header row below, not here.
				firstBorderTreePath := m.buildFileTreePath(startIdx, false, firstFileFolded, TreeRowContent)
				rows = append(rows, displayRow{
					kind:               RowKindHeaderTopBorder,
					fileIndex:          startIdx,
					isHeaderTopBorder:  true,
					foldLevel:          sidebyside.FoldNormal,
					status:             fileStatusFromPair(m.files[startIdx]),
					headerBoxWidth:     headerBoxWidth,
					treePrefixWidth:    treeWidth(0, true) + 1, // +1 to align with fold icon
					headerMode:         firstFileHeaderMode,
					treePath:           firstBorderTreePath,
					isLastFileInCommit: firstIsLastFile,
				})
			}
		}

		// Add file rows for this commit
		for fileIdx := startIdx; fileIdx < endIdx; fileIdx++ {
			// Skip files outside narrow scope
			if !m.w().narrow.IncludesFile(fileIdx) {
				continue
			}
			fp := m.files[fileIdx]
			rows = m.buildFileRows(rows, fileIdx, fp, startIdx, endIdx, headerBoxWidth)
		}

		// Add separator row between commits (blank line after last file, before next commit)
		// This row becomes the top border slot for the next commit when this commit is unfolded
		// Skip when narrowed to file level (no inter-commit separators needed)
		// Also skip if next commit is outside the narrow scope
		if commit.Info.HasMetadata() && commitIdx+1 < len(m.commits) && m.commits[commitIdx+1].Info.HasMetadata() && !m.w().narrow.IsFileLevelOrBelow() && m.w().narrow.IncludesCommit(commitIdx+1) {
			thisCommitUnfolded := m.commitFoldLevel(commitIdx) != sidebyside.CommitFolded

			if thisCommitUnfolded {
				// Unfolded commit produces margin; add top border slot for next commit
				// Compute next commit's header mode to determine if border should be visible
				nextCommitFolded := m.commitFoldLevel(commitIdx+1) == sidebyside.CommitFolded
				nextCommitHeaderMode := determineCommitHeaderMode(nextCommitFolded, false, true) // prev (this) is unfolded

				rows = append(rows, displayRow{
					kind:                    RowKindCommitHeaderTopBorder,
					fileIndex:               -1,
					isCommitHeaderTopBorder: true,
					commitIndex:             commitIdx + 1,
					headerMode:              nextCommitHeaderMode,
				})
			} else {
				// Folded commit produces no margin; just add a blank separator
				rows = append(rows, displayRow{
					kind:              RowKindCommitBody,
					fileIndex:         -1,
					isCommitBody:      true,
					commitBodyLine:    "",
					commitBodyIsBlank: true,
					commitIndex:       commitIdx,
				})
			}
		}
	}

	// Add truncation indicator if files were omitted
	if m.truncatedFileCount > 0 {
		rows = append(rows, displayRow{
			kind:                  RowKindTruncationIndicator,
			fileIndex:             -1,
			isTruncationIndicator: true,
			truncationMessage:     fmt.Sprintf("[%d files truncated]", m.truncatedFileCount),
		})
	}

	// Add pagination indicator if more commits may be available
	if m.hasMoreCommitsToLoad() {
		rows = append(rows, displayRow{
			kind:      RowKindPaginationIndicator,
			fileIndex: -1,
		})
	}

	return rows
}

// buildRowsLegacy handles the case where Model was created without using New/NewWithCommits.
// This maintains backward compatibility with tests that directly set m.files.
func (m Model) buildRowsLegacy() []displayRow {
	var rows []displayRow

	// Calculate consistent header box width for borders from max per-file width
	maxBoxWidth := 0
	for _, fp := range m.files {
		header := formatFileHeader(fp)
		added, removed := countFileStats(fp)
		bw := fileHeaderBoxWidth(header, added, removed)
		if bw > maxBoxWidth {
			maxBoxWidth = bw
		}
	}

	iconPartWidth := 3 + 1 + 1 // "   ◐ "
	headerContentWidth := maxBoxWidth - iconPartWidth
	if headerContentWidth < 0 {
		headerContentWidth = 0
	}

	// Use cached structural diff width, or calculate if not set (for tests)
	maxStructuralDiffWidth := m.cachedStructDiffWidth
	if maxStructuralDiffWidth == 0 {
		for fileIdx := range m.files {
			w := m.structuralDiffMaxContentWidth(fileIdx)
			if w > maxStructuralDiffWidth {
				maxStructuralDiffWidth = w
			}
		}
	}
	if maxStructuralDiffWidth > headerContentWidth {
		headerContentWidth = maxStructuralDiffWidth
	}

	// Calculate final box width, clamped to 80% of screen width
	headerBoxWidth := iconPartWidth + headerContentWidth
	if m.width > 0 {
		maxAllowedWidth := m.width * 80 / 100
		if headerBoxWidth > maxAllowedWidth && maxAllowedWidth > iconPartWidth {
			headerBoxWidth = maxAllowedWidth
		}
	}

	// Add file rows (no commit headers in legacy mode)
	// Note: The first file's top border is rendered specially in getVisibleRows,
	// not as part of the content rows (so it doesn't affect cursor line numbering).
	for fileIdx, fp := range m.files {
		// Skip files outside narrow scope
		if !m.w().narrow.IncludesFile(fileIdx) {
			continue
		}
		rows = m.buildFileRows(rows, fileIdx, fp, 0, len(m.files), headerBoxWidth)
	}

	// Add truncation indicator if files were omitted
	if m.truncatedFileCount > 0 {
		rows = append(rows, displayRow{
			kind:                  RowKindTruncationIndicator,
			fileIndex:             -1,
			isTruncationIndicator: true,
			truncationMessage:     fmt.Sprintf("[%d files truncated]", m.truncatedFileCount),
		})
	}

	return rows
}

// buildFileRows adds all rows for a single file to the rows slice.
func (m Model) buildFileRows(rows []displayRow, fileIdx int, fp sidebyside.FilePair, commitStartIdx, commitEndIdx int, headerBoxWidth int) []displayRow {
	added, removed := countFileStats(fp)
	status := fileStatusFromPair(fp)

	// Check if this is the first file in the commit
	isFirstFile := fileIdx == commitStartIdx

	// Check if previous sibling is unfolded (for determining header mode)
	prevSiblingUnfolded := false
	if fileIdx > commitStartIdx {
		prevSiblingUnfolded = m.fileFoldLevel(fileIdx-1) != sidebyside.FoldFolded
	}

	// Compute header mode based on fold state and prev sibling
	isFolded := m.fileFoldLevel(fileIdx) == sidebyside.FoldFolded
	headerMode := determineFileHeaderMode(isFolded, isFirstFile, prevSiblingUnfolded)

	isLastFile := fileIdx == commitEndIdx-1

	// Content rows of the last file always show │ continuation because the
	// ┴ terminator row (added after content) provides the visual end-of-tree.
	contentIsLast := false

	// Per-file header box width for unfolded headers (tighter border around own content)
	header := formatFileHeader(fp)
	ownBoxWidth := fileHeaderBoxWidth(header, added, removed)

	// The last file's header always uses ├ (not └) because there is always something
	// below it: either content rows or a ┴ terminator row.
	headerIsLast := false

	if isFolded {
		// Folded path: header (no border) → body content → margin
		headerTreePath := m.buildFileTreePath(fileIdx, headerIsLast, true, TreeRowHeader)
		rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldFolded, status: status, header: header, added: added, removed: removed, headerBoxWidth: headerBoxWidth, isLastFileInCommit: isLastFile, treePath: headerTreePath, headerMode: headerMode})

		bodyRows := m.buildFileBodyRows(fp, fileIdx, contentIsLast, isLastFile, isFolded, headerBoxWidth)
		rows = append(rows, bodyRows...)

		if len(bodyRows) > 0 {
			// Bottom margin after body content
			marginTreePath := m.buildFileTreePath(fileIdx, contentIsLast, true, TreeRowContent)
			rows = append(rows, displayRow{
				kind:               RowKindBlank,
				fileIndex:          fileIdx,
				isBlank:            true,
				isLastFileInCommit: isLastFile,
				treeTerminator:     isLastFile,
				treePath:           marginTreePath,
			})
		} else if isLastFile {
			// Last file with no preview content: add ┴ terminator after the bare header.
			// Force IsLast=false so the ancestor renders ┴ (not blank space).
			terminatorPath := m.buildFileTreePath(fileIdx, false, true, TreeRowContent)
			rows = append(rows, displayRow{
				kind:               RowKindBlank,
				fileIndex:          fileIdx,
				isBlank:            true,
				isLastFileInCommit: isLastFile,
				treeTerminator:     true,
				treePath:           terminatorPath,
			})
		}

	} else {
		// Unfolded path: header (with border) → spacer → body content → margin → next top border
		// Note: First file's top border is added after commit body rows, not here
		// This prevents content shift when first file is unfolded
		headerTreePath := m.buildFileTreePath(fileIdx, headerIsLast, false, TreeRowHeader)
		// Use contentIsLast so │ continuation shows in log mode on content rows of the last file.
		contentTreePath := m.buildFileTreePath(fileIdx, contentIsLast, false, TreeRowContent)

		rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: m.fileFoldLevel(fileIdx), status: status, header: header, added: added, removed: removed, headerBoxWidth: ownBoxWidth, isLastFileInCommit: isLastFile, treePath: headerTreePath, headerMode: headerMode})

		rows = append(rows, displayRow{kind: RowKindHeaderSpacer, fileIndex: fileIdx, isHeaderSpacer: true, foldLevel: m.fileFoldLevel(fileIdx), status: status, headerBoxWidth: ownBoxWidth, treePrefixWidth: treeWidth(0, true) + 1, headerMode: headerMode, treePath: contentTreePath})

		bodyRows := m.buildFileBodyRows(fp, fileIdx, contentIsLast, isLastFile, isFolded, headerBoxWidth)
		rows = append(rows, bodyRows...)

		// Bottom margin: one blank row after content.
		// The next file's HeaderTopBorder or next commit's top border provides a second line of spacing.
		marginTreePath := m.buildFileTreePath(fileIdx, false, false, TreeRowContent)
		rows = append(rows, displayRow{
			kind:               RowKindBlank,
			fileIndex:          fileIdx,
			isBlank:            true,
			isLastFileInCommit: isLastFile,
			treeTerminator:     isLastFile,
			treePath:           marginTreePath,
		})

		// Skip next file's top border if next file is outside narrow scope
		if !isLastFile && m.w().narrow.IncludesFile(fileIdx+1) {
			// Top border slot belongs to the NEXT file (fileIdx+1), not the current file
			// Current file is unfolded, so next file's prev sibling is unfolded
			// Always add this row to prevent content shift; render as border or blank based on next file's mode
			nextFileFolded := m.fileFoldLevel(fileIdx+1) == sidebyside.FoldFolded
			nextFileHeaderMode := determineFileHeaderMode(nextFileFolded, false, true)
			nextIsLastFile := fileIdx+1 == commitEndIdx-1
			// Force IsLast=false so │ continuation shows on the top border row;
			// the branch point (├/└) appears on the header row below, not here.
			nextBorderTreePath := m.buildFileTreePath(fileIdx+1, false, nextFileFolded, TreeRowContent)
			rows = append(rows, displayRow{kind: RowKindHeaderTopBorder, fileIndex: fileIdx + 1, isHeaderTopBorder: true, foldLevel: m.fileFoldLevel(fileIdx), status: status, headerBoxWidth: headerBoxWidth, treePrefixWidth: treeWidth(0, true) + 1, headerMode: nextFileHeaderMode, treePath: nextBorderTreePath, isLastFileInCommit: nextIsLastFile})
		}
	}

	return rows
}

// buildFileBodyRows dispatches to the appropriate content builder based on fold level.
// Returns content rows (structural diff or hunks) without header/spacer/margin.
func (m Model) buildFileBodyRows(fp sidebyside.FilePair, fileIdx int, contentIsLast bool, isLastFile bool, isFolded bool, headerBoxWidth int) []displayRow {
	foldLevel := m.fileFoldLevel(fileIdx)
	switch foldLevel {
	case sidebyside.FoldFolded:
		// Folded: header only, no content
		return nil
	case sidebyside.FoldNormal:
		// Part-expanded: structural diff preview
		return m.buildStructuralDiffRows(fileIdx, headerBoxWidth, contentIsLast, isFolded)
	default: // FoldExpanded
		// Full-file content view (Shift+F) when content is available
		if fp.ShowFullFile && fp.HasContent() {
			return m.buildExpandedBodyRows(fp, fileIdx, contentIsLast, isLastFile)
		}
		// Default: diff hunks
		return m.buildHunkRows(fp, fileIdx, contentIsLast, isLastFile)
	}
}

// buildHunkRows creates content rows from diff hunks (Pairs), including hunk separators,
// comment rows, binary indicators, and truncation indicators.
func (m Model) buildHunkRows(fp sidebyside.FilePair, fileIdx int, contentIsLast bool, isLastFile bool) []displayRow {
	var rows []displayRow

	if fp.IsBinary {
		var msg string
		var showOld, showNew bool
		if fp.OldPath == "/dev/null" {
			msg = "Binary file created"
			showNew = true
		} else if fp.NewPath == "/dev/null" {
			msg = "Binary file deleted"
			showOld = true
		} else {
			msg = "Binary file changed"
			showOld = true
			showNew = true
		}
		rows = append(rows, displayRow{
			kind:              RowKindBinaryIndicator,
			fileIndex:         fileIdx,
			isBinaryIndicator: true,
			binaryMessage:     msg,
			binaryOld:         showOld,
			binaryNew:         showNew,
			isFirstLine:       true,
			isLastLine:        true,
		})
		return rows
	}

	var prevLeft, prevRight int
	var lastChunkStartLine int // tracks previous separator's chunkStartLine for repeat detection
	contentTreePath := m.buildFileTreePath(fileIdx, contentIsLast, false, TreeRowContent)
	for i, pair := range fp.Pairs {
		if i == 0 && (pair.Old.Num > 1 || pair.New.Num > 1) {
			chunkStartLine := findFirstNewLineNum(fp.Pairs, i)
			rows = append(rows, displayRow{kind: RowKindSeparator, fileIndex: fileIdx, isSeparator: true, chunkStartLine: chunkStartLine, prevChunkStartLine: lastChunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
			rows = append(rows, displayRow{kind: RowKindSeparatorBottom, fileIndex: fileIdx, isSeparatorBottom: true, chunkStartLine: chunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
			lastChunkStartLine = chunkStartLine
		}

		if i > 0 && isHunkBoundary(prevLeft, prevRight, pair.Old.Num, pair.New.Num) {
			chunkStartLine := findFirstNewLineNum(fp.Pairs, i)
			rows = append(rows, displayRow{kind: RowKindSeparatorTop, fileIndex: fileIdx, isSeparatorTop: true, isLastFileInCommit: isLastFile, treePath: contentTreePath})
			rows = append(rows, displayRow{kind: RowKindSeparator, fileIndex: fileIdx, isSeparator: true, chunkStartLine: chunkStartLine, prevChunkStartLine: lastChunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
			rows = append(rows, displayRow{kind: RowKindSeparatorBottom, fileIndex: fileIdx, isSeparatorBottom: true, chunkStartLine: chunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
			lastChunkStartLine = chunkStartLine
		}

		row := displayRow{kind: RowKindContent, fileIndex: fileIdx, pair: pair, isLastFileInCommit: isLastFile, treePath: contentTreePath}
		if i == 0 {
			row.isFirstLine = true
		}
		if i == len(fp.Pairs)-1 {
			row.isLastLine = true
		}
		rows = append(rows, row)

		// Add comment rows if this line has a comment
		if pair.New.Num > 0 {
			key := commentKey{fileIndex: fileIdx, newLineNum: pair.New.Num}
			if comment, ok := m.comments[key]; ok {
				commentRows := buildCommentRows(fileIdx, pair.New.Num, comment, m.commentContentWidth(), contentTreePath)
				rows = append(rows, commentRows...)
			}
		}

		if pair.Old.Num > 0 {
			prevLeft = pair.Old.Num
		}
		if pair.New.Num > 0 {
			prevRight = pair.New.Num
		}
	}

	// Add a trailing separator if there's more file content below the last hunk
	if len(fp.NewContent) > 0 && prevRight > 0 && prevRight < len(fp.NewContent) {
		rows = append(rows, displayRow{kind: RowKindSeparatorTop, fileIndex: fileIdx, isSeparatorTop: true, isLastFileInCommit: isLastFile, treePath: contentTreePath})
	}

	if fp.Truncated || fp.OldTruncated || fp.NewTruncated {
		oldTrunc := fp.OldTruncated
		newTrunc := fp.NewTruncated
		if fp.Truncated && !fp.OldTruncated && !fp.NewTruncated {
			oldTrunc = true
			newTrunc = true
		}
		rows = append(rows, displayRow{
			kind:                  RowKindTruncationIndicator,
			fileIndex:             fileIdx,
			isTruncationIndicator: true,
			truncationMessage:     "[truncated due to file size limit]",
			truncateOld:           oldTrunc,
			truncateNew:           newTrunc,
		})
	}

	return rows
}

// buildExpandedBodyRows creates content rows from full file content, with comment rows interleaved
// and truncation indicators appended.
func (m Model) buildExpandedBodyRows(fp sidebyside.FilePair, fileIdx int, contentIsLast bool, isLastFile bool) []displayRow {
	contentTreePath := m.buildFileTreePath(fileIdx, contentIsLast, false, TreeRowContent)

	expandedRows := m.buildExpandedRows(fp)
	for i := range expandedRows {
		expandedRows[i].fileIndex = fileIdx
		expandedRows[i].isLastFileInCommit = isLastFile
		expandedRows[i].treePath = contentTreePath
		if i == 0 {
			expandedRows[i].isFirstLine = true
		}
		if i == len(expandedRows)-1 {
			expandedRows[i].isLastLine = true
		}
	}

	// Append expanded rows with comment rows interleaved
	var rows []displayRow
	for _, expRow := range expandedRows {
		rows = append(rows, expRow)
		// Add comment rows if this is a content row with a comment
		if expRow.kind == RowKindContent && expRow.pair.New.Num > 0 {
			key := commentKey{fileIndex: fileIdx, newLineNum: expRow.pair.New.Num}
			if comment, ok := m.comments[key]; ok {
				commentRows := buildCommentRows(fileIdx, expRow.pair.New.Num, comment, m.commentContentWidth(), contentTreePath)
				rows = append(rows, commentRows...)
			}
		}
	}

	if fp.Truncated || fp.ContentTruncated || fp.OldContentTruncated || fp.NewContentTruncated {
		oldTrunc := fp.OldContentTruncated || fp.OldTruncated
		newTrunc := fp.NewContentTruncated || fp.NewTruncated
		if fp.ContentTruncated && !fp.OldContentTruncated && !fp.NewContentTruncated {
			oldTrunc = true
			newTrunc = true
		}
		rows = append(rows, displayRow{
			kind:                  RowKindTruncationIndicator,
			fileIndex:             fileIdx,
			isTruncationIndicator: true,
			truncationMessage:     "[truncated due to file size limit]",
			truncateOld:           oldTrunc,
			truncateNew:           newTrunc,
		})
	}

	return rows
}

// buildExpandedRows creates line pairs from full file content.
// It uses the Pairs as alignment anchors to properly align added/removed lines,
// then fills in context lines from the full file content.
func (m Model) buildExpandedRows(fp sidebyside.FilePair) []displayRow {
	oldLen := len(fp.OldContent)
	newLen := len(fp.NewContent)

	// Handle deleted file (no new content)
	if newLen == 0 && oldLen > 0 {
		return m.buildExpandedRowsDeletedFile(fp)
	}

	// Handle new file (no old content)
	if oldLen == 0 && newLen > 0 {
		return m.buildExpandedRowsNewFile(fp)
	}

	// Both files have content - use Pairs as alignment skeleton
	return m.buildExpandedRowsWithAlignment(fp)
}

// buildExpandedRowsDeletedFile handles the case where the file was deleted.
func (m Model) buildExpandedRowsDeletedFile(fp sidebyside.FilePair) []displayRow {
	oldTypes := buildLineTypeMap(fp.Pairs, true)
	var rows []displayRow

	for i, content := range fp.OldContent {
		lineNum := i + 1
		lineType := sidebyside.Context
		if t, ok := oldTypes[lineNum]; ok {
			lineType = t
		}
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: lineNum, Content: content, Type: lineType},
				New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			},
		})
	}
	return rows
}

// buildExpandedRowsNewFile handles the case where the file is new.
func (m Model) buildExpandedRowsNewFile(fp sidebyside.FilePair) []displayRow {
	newTypes := buildLineTypeMap(fp.Pairs, false)
	var rows []displayRow

	for i, content := range fp.NewContent {
		lineNum := i + 1
		lineType := sidebyside.Context
		if t, ok := newTypes[lineNum]; ok {
			lineType = t
		}
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
				New: sidebyside.Line{Num: lineNum, Content: content, Type: lineType},
			},
		})
	}
	return rows
}

// buildLineTypeMap extracts line types from Pairs for one side.
func buildLineTypeMap(pairs []sidebyside.LinePair, oldSide bool) map[int]sidebyside.LineType {
	types := make(map[int]sidebyside.LineType)
	for _, pair := range pairs {
		if oldSide {
			if pair.Old.Num > 0 {
				types[pair.Old.Num] = pair.Old.Type
			}
		} else {
			if pair.New.Num > 0 {
				types[pair.New.Num] = pair.New.Type
			}
		}
	}
	return types
}

// buildExpandedRowsWithAlignment uses Pairs as alignment anchors and fills gaps.
func (m Model) buildExpandedRowsWithAlignment(fp sidebyside.FilePair) []displayRow {
	var rows []displayRow
	oldIdx := 0 // 0-based index into OldContent
	newIdx := 0 // 0-based index into NewContent

	// Process each pair from the diff, filling in context gaps
	for _, pair := range fp.Pairs {
		// Fill context lines before this pair
		// These are lines that exist in both files but weren't in the diff context
		oldTarget := pair.Old.Num - 1 // 0-based target for old (or -1 if empty)
		newTarget := pair.New.Num - 1 // 0-based target for new (or -1 if empty)

		if pair.Old.Num == 0 {
			// Added line - old side is empty, fill new context up to this line
			for newIdx < newTarget {
				// Find corresponding old line (context lines match 1:1 before additions)
				if oldIdx < len(fp.OldContent) {
					rows = append(rows, displayRow{
						pair: sidebyside.LinePair{
							Old: sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
							New: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
						},
					})
					oldIdx++
				}
				newIdx++
			}
		} else if pair.New.Num == 0 {
			// Removed line - new side is empty, fill old context up to this line
			for oldIdx < oldTarget {
				if newIdx < len(fp.NewContent) {
					rows = append(rows, displayRow{
						pair: sidebyside.LinePair{
							Old: sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
							New: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
						},
					})
					newIdx++
				}
				oldIdx++
			}
		} else {
			// Context or modified line - fill gaps on both sides
			for oldIdx < oldTarget && newIdx < newTarget &&
				oldIdx < len(fp.OldContent) && newIdx < len(fp.NewContent) {
				rows = append(rows, displayRow{
					pair: sidebyside.LinePair{
						Old: sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
						New: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
					},
				})
				oldIdx++
				newIdx++
			}
		}

		// Add the pair itself (with content from full file if available)
		pairRow := m.buildPairRow(pair, fp)
		rows = append(rows, pairRow)

		// Advance indices past this pair
		if pair.Old.Num > 0 {
			oldIdx = pair.Old.Num // Now at 0-based index after this line
		}
		if pair.New.Num > 0 {
			newIdx = pair.New.Num
		}
	}

	// Fill remaining context after the last pair
	for oldIdx < len(fp.OldContent) && newIdx < len(fp.NewContent) {
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
				New: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
			},
		})
		oldIdx++
		newIdx++
	}

	// Handle any remaining lines on one side only (shouldn't happen in normal diffs)
	for oldIdx < len(fp.OldContent) {
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
				New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			},
		})
		oldIdx++
	}
	for newIdx < len(fp.NewContent) {
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
				New: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
			},
		})
		newIdx++
	}

	return rows
}

// buildPairRow creates a displayRow from a Pair, using full file content when available.
func (m Model) buildPairRow(pair sidebyside.LinePair, fp sidebyside.FilePair) displayRow {
	old := pair.Old
	new := pair.New

	// Use content from full file if available (it should match, but ensures consistency)
	if old.Num > 0 && old.Num <= len(fp.OldContent) {
		old.Content = fp.OldContent[old.Num-1]
	}
	if new.Num > 0 && new.Num <= len(fp.NewContent) {
		new.Content = fp.NewContent[new.Num-1]
	}

	return displayRow{pair: sidebyside.LinePair{Old: old, New: new}}
}

// getVisibleRows returns the rendered rows visible in the current viewport.
func (m Model) getVisibleRows(rows []displayRow, contentHeight int) []string {
	var visible []string

	// Calculate column widths - prioritize left (new) side on narrow terminals
	lineNumWidth := m.lineNumWidth()

	// Layout per side: indicator(1) + space(1) + lineNum + space(1) + content + gutter(4)
	// Gutter is: ░ + space + content + space + ░ (2 chars on each side)
	gutterOverhead := 1 + 1 + lineNumWidth + 1 + 4 // everything except content

	// Right side minimum: show only left gutter (no trailing gutter when squeezed)
	// indicator(1) + space(1) + lineNum + space(1) + leftGutter(2) = 5 + lineNumWidth
	minRightWidth := 1 + 1 + lineNumWidth + 1 + 2

	// Left side wants up to 90 chars of content (or actual max content width if smaller)
	// If there's no new content (e.g., deleted file), use 50/50
	targetLeftContent := 90
	if m.maxNewContentWidth < targetLeftContent {
		targetLeftContent = m.maxNewContentWidth
	}

	defaultHalf := (m.width - 3) / 2 // -3 for separator " │ "
	leftContentAt50 := defaultHalf - gutterOverhead

	var leftHalfWidth int
	if leftContentAt50 >= targetLeftContent {
		// 50/50 split gives left side enough content room
		leftHalfWidth = defaultHalf
	} else {
		// Terminal is narrow - prioritize left side
		targetLeftWidth := gutterOverhead + targetLeftContent
		maxLeftWidth := m.width - 3 - minRightWidth
		leftHalfWidth = targetLeftWidth
		if leftHalfWidth > maxLeftWidth {
			leftHalfWidth = maxLeftWidth
		}
	}

	// Right side gets whatever is left after left side and separator
	rightHalfWidth := m.width - 3 - leftHalfWidth

	// Hide right trailing gutter when right side is squeezed (no content visible)
	// Right content area = rightHalfWidth - lineNumWidth - 3 (indicator, spaces) - 4 (gutter)
	rightContentArea := rightHalfWidth - lineNumWidth - 3 - 4
	hideRightTrailingGutter := rightContentArea <= 0

	// The cursor position on screen - moves up when near top of content
	cursorViewportRow := m.cursorViewportRow()

	// Get focus predicate for dimming out-of-focus content
	focusPredicate := m.getFocusPredicate()

	// Content starts at line 0 when near the top, then scrolls up
	start := m.contentStartLine()
	end := start + contentHeight

	// When showing from the very beginning, render the first item's top border
	// as an extra line before the content (not cursor-selectable)
	if start == 0 && len(rows) > 0 {
		// Check if first commit is unfolded (for rendering commit top border)
		firstCommitUnfolded := len(m.commits) > 0 &&
			m.commits[0].Info.HasMetadata() &&
			m.commitFoldLevel(0) != sidebyside.CommitFolded

		// Check if first file is unfolded and we're in diff view (no commit metadata)
		firstFileUnfolded := len(m.files) > 0 &&
			m.fileFoldLevel(0) != sidebyside.FoldFolded
		isDiffView := len(m.commits) == 0 ||
			(len(m.commits) > 0 && !m.commits[0].Info.HasMetadata())

		if firstCommitUnfolded {
			// Render first commit's top border in the margin (not cursor-selectable)
			visible = append(visible, m.renderCommitBorderLine(true, true, false, TreePath{}))
		} else if isDiffView && firstFileUnfolded && rows[0].isHeader {
			// Render first file's top border (matches the header box style)
			// In diff view, files are roots so no tree ancestors
			visible = append(visible, m.renderHeaderTopBorder(rows[0].headerBoxWidth, HeaderThreeLine, rows[0].status, false, treeWidth(0, true)+1, TreePath{}))
		} else {
			// Render blank line (border slot when folded)
			visible = append(visible, "")
		}
		// Adjust cursor position to account for the border line
		cursorViewportRow++
	}

	if end > len(rows) {
		end = len(rows)
	}

	for i := start; i < end && len(visible) < contentHeight; i++ {
		row := rows[i]
		isCursorRow := len(visible) == cursorViewportRow

		var rendered string
		if row.isCommitHeader {
			rendered = m.renderCommitHeaderRow(row, isCursorRow)
		} else if row.isCommitHeaderTopBorder {
			rendered = m.renderCommitHeaderTopBorder(row, isCursorRow)
		} else if row.isCommitHeaderBottomBorder {
			rendered = m.renderCommitHeaderBottomBorder(row, isCursorRow)
		} else if row.isCommitBody {
			rendered = m.renderCommitBodyRow(row, isCursorRow)
		} else if row.isCommitInfoHeader {
			rendered = m.renderCommitInfoHeader(row, isCursorRow)
		} else if row.isCommitInfoTopBorder {
			rendered = m.renderCommitInfoTopBorder(row, isCursorRow)
		} else if row.isCommitInfoBottomBorder {
			rendered = m.renderCommitInfoBottomBorder(row, isCursorRow)
		} else if row.isCommitInfoBody {
			rendered = m.renderCommitInfoBody(row, isCursorRow)
		} else if row.isStructuralDiff {
			rendered = m.renderStructuralDiffRow(row, isCursorRow)
		} else if row.isHeaderTopBorder {
			rendered = m.renderHeaderTopBorder(row.headerBoxWidth, row.headerMode, row.status, isCursorRow, row.treePrefixWidth, row.treePath)
		} else if row.isHeaderSpacer {
			rendered = m.renderHeaderBottomBorder(row.headerBoxWidth, row.headerMode, row.status, isCursorRow, row.treePrefixWidth, row.treePath)
		} else if row.isBlank {
			rendered = renderEmptyTreeRow(row.treePath, isCursorRow, m.focused, row.treeTerminator)
		} else if row.isHeader {
			rendered = m.renderHeader(row.header, row.foldLevel, row.headerMode, row.status, row.added, row.removed, row.headerBoxWidth, row.fileIndex, i, isCursorRow, row.treePath)
		} else if row.isSeparatorTop {
			rendered = m.renderHunkSeparatorTop(row, leftHalfWidth, rightHalfWidth, isCursorRow)
		} else if row.isSeparator {
			rendered = m.renderHunkSeparator(row, leftHalfWidth, rightHalfWidth, isCursorRow)
		} else if row.isSeparatorBottom {
			rendered = m.renderHunkSeparatorTop(row, leftHalfWidth, rightHalfWidth, isCursorRow) // same as top
		} else if row.isTruncationIndicator {
			rendered = m.renderTruncationIndicator(row.truncationMessage, isCursorRow, row.truncateOld, row.truncateNew)
		} else if row.isBinaryIndicator {
			rendered = m.renderBinaryIndicator(row.binaryMessage, isCursorRow, row.binaryOld, row.binaryNew)
		} else if row.kind == RowKindPaginationIndicator {
			rendered = m.renderPaginationIndicator(isCursorRow)
		} else if row.kind == RowKindComment {
			rendered = m.renderCommentRow(row, leftHalfWidth, rightHalfWidth, lineNumWidth, isCursorRow)
		} else {
			rendered = m.renderLinePair(row.pair, row.fileIndex, leftHalfWidth, rightHalfWidth, lineNumWidth, i, isCursorRow, row.isFirstLine, row.isLastLine, hideRightTrailingGutter, row.treePath)
		}

		// Apply focus colour dimming to rows outside the focus area
		// Strip all ANSI codes and render as dim text for a clean muted appearance
		if focusPredicate != nil && !focusPredicate(i, row) {
			rendered = "\x1b[2m" + ansi.Strip(rendered) + "\x1b[22m"
		}

		visible = append(visible, rendered)
	}

	return visible
}

// renderHunkSeparator renders a separator line between hunks.
