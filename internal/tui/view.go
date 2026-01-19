package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/inlinediff"
	"github.com/user/diffyduck/pkg/sidebyside"
)

var (
	// Styles for different line types
	headerStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	headerLineStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // for ━ characters in headers
	hunkSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	addedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	removedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	changedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // blue for modified lines with word diff
	contextStyle       = lipgloss.NewStyle()
	contextDimStyle    = lipgloss.NewStyle().Faint(true) // for context on old side
	lineNumStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	emptyStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle        = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("0"))

	// Inline diff highlight: underlined, bold, and colored to match the diff side
	inlineAddedStyle   = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color("10"))
	inlineRemovedStyle = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color("9"))

	// Block-aligned indicator style (grey, used faint)
	blockIndicatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Search highlight styles (black text on yellow background)
	searchMatchStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3"))
	searchCurrentMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))

	// Cursor highlight style (bg=15 bright white, fg=0 black) for gutter areas
	cursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("15")).Foreground(lipgloss.Color("0"))

	// Cursor arrow style (fg=15, no background)
	cursorArrowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow

	// Inter-file area style (dim shading for blank lines between files)
	interFileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	contentH := m.contentHeight()

	// Build list of all displayable rows
	rows := m.buildRows()

	// Apply scroll and viewport
	visibleRows := m.getVisibleRows(rows, contentH)

	// Pad with empty lines to fill viewport (so bottom bar is always at bottom)
	for len(visibleRows) < contentH {
		visibleRows = append(visibleRows, "")
	}

	// Build final output: top bar + content + bottom bar
	var output []string

	// Add top bar (file info)
	topBar := m.renderTopBar()
	output = append(output, topBar)

	// Add content rows
	output = append(output, visibleRows...)

	// Add bottom bar (less-style indicator)
	bottomBar := m.renderStatusBar()
	output = append(output, bottomBar)

	return strings.Join(output, "\n")
}

// displayRow represents one row in the view (header, line pair, hunk separator, or blank)
type displayRow struct {
	fileIndex      int // index of the file this row belongs to (-1 for summary row)
	isHeader       bool
	isSeparator    bool
	isBlank        bool
	isHeaderSpacer bool // blank line after header (no shading, just empty)
	isSummary      bool // summary row at the end showing total stats
	isFirstLine    bool // first line pair in a file (uses ┬ separator)
	isLastLine     bool // last line pair in a file (uses ┴ separator)
	header         string
	foldLevel      sidebyside.FoldLevel // fold level for headers (used for icon and styling)
	status         FileStatus           // file status (added, deleted, renamed, modified) for headers
	pair           sidebyside.LinePair
	added          int // number of added lines (for headers)
	removed        int // number of removed lines (for headers)
	maxHeaderWidth int // max header width across all files (for alignment in folded view)
	maxCountWidth  int // max stats count width across all files (for bar alignment)
	// Summary row fields
	totalFiles   int // total number of files changed
	totalAdded   int // total insertions across all files
	totalRemoved int // total deletions across all files
}

// buildRows creates all displayable rows from the model data.
func (m Model) buildRows() []displayRow {
	var rows []displayRow

	// Calculate max header width and max count width across all files for alignment
	maxHeaderWidth := 0
	maxCountWidth := 0
	for _, fp := range m.files {
		header := formatFileHeader(fp.OldPath, fp.NewPath)
		w := displayWidth(header)
		if w > maxHeaderWidth {
			maxHeaderWidth = w
		}
		added, removed := countFileStats(fp)
		cw := statsCountWidth(added, removed)
		if cw > maxCountWidth {
			maxCountWidth = cw
		}
	}

	for fileIdx, fp := range m.files {
		// Count stats once per file for header display
		added, removed := countFileStats(fp)
		status := fileStatus(fp.OldPath, fp.NewPath)

		switch fp.FoldLevel {
		case sidebyside.FoldFolded:
			// Folded: just the header, no blank line before, no trailing "="
			header := formatFileHeader(fp.OldPath, fp.NewPath)
			rows = append(rows, displayRow{fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldFolded, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxCountWidth: maxCountWidth})

		case sidebyside.FoldExpanded:
			// Expanded: show full file content with diff highlighting
			// If content not loaded yet, fall back to normal view
			if fp.HasContent() {
				// File header with stats
				header := formatFileHeader(fp.OldPath, fp.NewPath)
				rows = append(rows, displayRow{fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldExpanded, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxCountWidth: maxCountWidth})

				// Blank line after header before content (no shading)
				rows = append(rows, displayRow{fileIndex: fileIdx, isHeaderSpacer: true})

				// Build expanded rows from full file content
				expandedRows := m.buildExpandedRows(fp)
				for i := range expandedRows {
					expandedRows[i].fileIndex = fileIdx
					if i == 0 {
						expandedRows[i].isFirstLine = true
					}
					if i == len(expandedRows)-1 {
						expandedRows[i].isLastLine = true
					}
				}
				rows = append(rows, expandedRows...)

				// Add 4 blank lines after expanded content
				// Blank lines belong to the file above, not below
				for i := 0; i < 4; i++ {
					rows = append(rows, displayRow{fileIndex: fileIdx, isBlank: true})
				}
				continue // Skip the normal view below
			}
			// Fall through to normal view if content not loaded
			fallthrough

		default: // FoldNormal
			// File header with stats
			header := formatFileHeader(fp.OldPath, fp.NewPath)
			rows = append(rows, displayRow{fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldNormal, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxCountWidth: maxCountWidth})

			// Blank line after header before content (no shading)
			rows = append(rows, displayRow{fileIndex: fileIdx, isHeaderSpacer: true})

			// Line pairs with hunk separators
			var prevLeft, prevRight int
			for i, pair := range fp.Pairs {
				// Check for gap in line numbers (hunk boundary)
				if i > 0 && isHunkBoundary(prevLeft, prevRight, pair.Left.Num, pair.Right.Num) {
					rows = append(rows, displayRow{fileIndex: fileIdx, isSeparator: true})
				}

				row := displayRow{fileIndex: fileIdx, pair: pair}
				if i == 0 {
					row.isFirstLine = true
				}
				if i == len(fp.Pairs)-1 {
					row.isLastLine = true
				}
				rows = append(rows, row)

				// Track previous line numbers (use non-zero values)
				if pair.Left.Num > 0 {
					prevLeft = pair.Left.Num
				}
				if pair.Right.Num > 0 {
					prevRight = pair.Right.Num
				}
			}

			// Add 4 blank lines after normal content
			// Blank lines belong to the file above, not below
			for i := 0; i < 4; i++ {
				rows = append(rows, displayRow{fileIndex: fileIdx, isBlank: true})
			}
		}
	}

	// Add summary row at the end
	if len(m.files) > 0 {
		totalAdded := 0
		totalRemoved := 0
		for _, fp := range m.files {
			added, removed := countFileStats(fp)
			totalAdded += added
			totalRemoved += removed
		}
		rows = append(rows, displayRow{
			fileIndex:      -1, // No file association
			isSummary:      true,
			totalFiles:     len(m.files),
			totalAdded:     totalAdded,
			totalRemoved:   totalRemoved,
			maxHeaderWidth: maxHeaderWidth,
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
	leftTypes := buildLineTypeMap(fp.Pairs, true)
	var rows []displayRow

	for i, content := range fp.OldContent {
		lineNum := i + 1
		lineType := sidebyside.Context
		if t, ok := leftTypes[lineNum]; ok {
			lineType = t
		}
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Left:  sidebyside.Line{Num: lineNum, Content: content, Type: lineType},
				Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			},
		})
	}
	return rows
}

// buildExpandedRowsNewFile handles the case where the file is new.
func (m Model) buildExpandedRowsNewFile(fp sidebyside.FilePair) []displayRow {
	rightTypes := buildLineTypeMap(fp.Pairs, false)
	var rows []displayRow

	for i, content := range fp.NewContent {
		lineNum := i + 1
		lineType := sidebyside.Context
		if t, ok := rightTypes[lineNum]; ok {
			lineType = t
		}
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
				Right: sidebyside.Line{Num: lineNum, Content: content, Type: lineType},
			},
		})
	}
	return rows
}

// buildLineTypeMap extracts line types from Pairs for one side.
func buildLineTypeMap(pairs []sidebyside.LinePair, leftSide bool) map[int]sidebyside.LineType {
	types := make(map[int]sidebyside.LineType)
	for _, pair := range pairs {
		if leftSide {
			if pair.Left.Num > 0 {
				types[pair.Left.Num] = pair.Left.Type
			}
		} else {
			if pair.Right.Num > 0 {
				types[pair.Right.Num] = pair.Right.Type
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
		oldTarget := pair.Left.Num - 1  // 0-based target for old (or -1 if empty)
		newTarget := pair.Right.Num - 1 // 0-based target for new (or -1 if empty)

		if pair.Left.Num == 0 {
			// Added line - old side is empty, fill new context up to this line
			for newIdx < newTarget {
				// Find corresponding old line (context lines match 1:1 before additions)
				if oldIdx < len(fp.OldContent) {
					rows = append(rows, displayRow{
						pair: sidebyside.LinePair{
							Left:  sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
							Right: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
						},
					})
					oldIdx++
				}
				newIdx++
			}
		} else if pair.Right.Num == 0 {
			// Removed line - new side is empty, fill old context up to this line
			for oldIdx < oldTarget {
				if newIdx < len(fp.NewContent) {
					rows = append(rows, displayRow{
						pair: sidebyside.LinePair{
							Left:  sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
							Right: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
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
						Left:  sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
						Right: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
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
		if pair.Left.Num > 0 {
			oldIdx = pair.Left.Num // Now at 0-based index after this line
		}
		if pair.Right.Num > 0 {
			newIdx = pair.Right.Num
		}
	}

	// Fill remaining context after the last pair
	for oldIdx < len(fp.OldContent) && newIdx < len(fp.NewContent) {
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Left:  sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
				Right: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
			},
		})
		oldIdx++
		newIdx++
	}

	// Handle any remaining lines on one side only (shouldn't happen in normal diffs)
	for oldIdx < len(fp.OldContent) {
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Left:  sidebyside.Line{Num: oldIdx + 1, Content: fp.OldContent[oldIdx], Type: sidebyside.Context},
				Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			},
		})
		oldIdx++
	}
	for newIdx < len(fp.NewContent) {
		rows = append(rows, displayRow{
			pair: sidebyside.LinePair{
				Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
				Right: sidebyside.Line{Num: newIdx + 1, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
			},
		})
		newIdx++
	}

	return rows
}

// buildPairRow creates a displayRow from a Pair, using full file content when available.
func (m Model) buildPairRow(pair sidebyside.LinePair, fp sidebyside.FilePair) displayRow {
	left := pair.Left
	right := pair.Right

	// Use content from full file if available (it should match, but ensures consistency)
	if left.Num > 0 && left.Num <= len(fp.OldContent) {
		left.Content = fp.OldContent[left.Num-1]
	}
	if right.Num > 0 && right.Num <= len(fp.NewContent) {
		right.Content = fp.NewContent[right.Num-1]
	}

	return displayRow{pair: sidebyside.LinePair{Left: left, Right: right}}
}

// isHunkBoundary returns true if there's a gap between consecutive line pairs.
func isHunkBoundary(prevLeft, prevRight, currLeft, currRight int) bool {
	// Check left side for gap (ignoring empty lines with Num=0)
	if prevLeft > 0 && currLeft > 0 && currLeft > prevLeft+1 {
		return true
	}
	// Check right side for gap
	if prevRight > 0 && currRight > 0 && currRight > prevRight+1 {
		return true
	}
	return false
}

// getVisibleRows returns the rendered rows visible in the current viewport.
func (m Model) getVisibleRows(rows []displayRow, contentHeight int) []string {
	var visible []string

	// Calculate column widths
	halfWidth := (m.width - 3) / 2 // -3 for the separator " │ "
	lineNumWidth := m.lineNumWidth()

	// Pre-compute block-aware indicator start positions
	leftIndicatorStarts, rightIndicatorStarts := computeIndicatorStarts(rows)

	// The cursor is at a fixed viewport position
	cursorViewportRow := m.cursorOffset()

	start := m.scroll
	end := m.scroll + contentHeight

	// Handle negative scroll by adding blank padding at the top
	if start < 0 {
		for i := start; i < 0 && len(visible) < contentHeight; i++ {
			isCursorRow := len(visible) == cursorViewportRow
			if isCursorRow {
				visible = append(visible, m.renderBlankWithCursor(halfWidth, lineNumWidth))
			} else {
				visible = append(visible, "")
			}
		}
		start = 0
	}

	if end > len(rows) {
		end = len(rows)
	}

	for i := start; i < end && len(visible) < contentHeight; i++ {
		row := rows[i]
		isCursorRow := len(visible) == cursorViewportRow

		if row.isHeaderSpacer {
			if isCursorRow {
				visible = append(visible, m.renderHeaderSpacerWithCursor(halfWidth, lineNumWidth))
			} else {
				visible = append(visible, "") // empty line, no shading
			}
		} else if row.isBlank {
			if isCursorRow {
				visible = append(visible, m.renderBlankWithCursor(halfWidth, lineNumWidth))
			} else {
				visible = append(visible, m.renderInterFileBlank())
			}
		} else if row.isHeader {
			visible = append(visible, m.renderHeader(row.header, row.foldLevel, row.status, row.added, row.removed, row.maxHeaderWidth, row.maxCountWidth, i, isCursorRow))
		} else if row.isSeparator {
			visible = append(visible, m.renderHunkSeparator(halfWidth, isCursorRow))
		} else if row.isSummary {
			visible = append(visible, m.renderSummary(row.totalFiles, row.totalAdded, row.totalRemoved, row.maxHeaderWidth, isCursorRow))
		} else {
			leftStart := leftIndicatorStarts[i]
			rightStart := rightIndicatorStarts[i]
			visible = append(visible, m.renderLinePair(row.pair, row.fileIndex, halfWidth, lineNumWidth, i, isCursorRow, leftStart, rightStart, row.isFirstLine, row.isLastLine))
		}
	}

	return visible
}

// renderHunkSeparator renders a separator line between hunks.
func (m Model) renderHunkSeparator(halfWidth int, isCursorRow bool) string {
	// Fill with light shading, with cross where vertical divider meets horizontal
	leftShade := strings.Repeat("░", halfWidth)
	rightShade := strings.Repeat("░", halfWidth)
	separator := hunkSeparatorStyle.Render("┼")

	if isCursorRow {
		// Cursor row: show arrows and highlight gutter area
		lineNumWidth := m.lineNumWidth()
		contentWidth := halfWidth - lineNumWidth - 3
		if contentWidth < 0 {
			contentWidth = 0
		}
		gutterShade := cursorStyle.Render(strings.Repeat("░", lineNumWidth))
		contentShade := interFileStyle.Render(strings.Repeat("░", contentWidth))

		return cursorArrowStyle.Render("➤") + interFileStyle.Render("░") + gutterShade + interFileStyle.Render("░") + contentShade + interFileStyle.Render("░") + separator + interFileStyle.Render("░") +
			cursorArrowStyle.Render("➤") + interFileStyle.Render("░") + gutterShade + interFileStyle.Render("░") + contentShade
	}

	// Normal rendering: shading with │ in the middle
	return interFileStyle.Render(leftShade) + " " + separator + " " + interFileStyle.Render(rightShade)
}

// renderBlankWithCursor renders a blank line with highlighted gutter areas when cursor is on it.
func (m Model) renderBlankWithCursor(halfWidth, lineNumWidth int) string {
	// Highlight both gutter areas (left and right)
	leftGutter := cursorStyle.Render(strings.Repeat("░", lineNumWidth))
	rightGutter := cursorStyle.Render(strings.Repeat("░", lineNumWidth))

	// Content areas with light shading (accounting for indicator + space + gutter + space)
	contentWidth := halfWidth - lineNumWidth - 3
	if contentWidth < 0 {
		contentWidth = 0
	}
	leftContent := interFileStyle.Render(strings.Repeat("░", contentWidth))
	rightContent := interFileStyle.Render(strings.Repeat("░", contentWidth))

	separator := interFileStyle.Render("░")
	// Format: arrow + shade + gutter + shade + content
	return cursorArrowStyle.Render("➤") + interFileStyle.Render("░") + leftGutter + interFileStyle.Render("░") + leftContent + interFileStyle.Render("░") + separator + interFileStyle.Render("░") +
		cursorArrowStyle.Render("➤") + interFileStyle.Render("░") + rightGutter + interFileStyle.Render("░") + rightContent
}

// renderInterFileBlank renders a blank line between files with light shading.
func (m Model) renderInterFileBlank() string {
	// Fill the entire width with light shade characters
	return interFileStyle.Render(strings.Repeat("░", m.width))
}

// renderHeaderSpacerWithCursor renders a blank line after header with cursor indicator.
// Matches the structure of renderBlankWithCursor but with empty content (no shading).
func (m Model) renderHeaderSpacerWithCursor(halfWidth, lineNumWidth int) string {
	// Highlight both gutter areas (left and right) with cursor style
	leftGutter := cursorStyle.Render(strings.Repeat(" ", lineNumWidth))
	rightGutter := cursorStyle.Render(strings.Repeat(" ", lineNumWidth))

	// Content areas - empty spaces (no shading for header spacer)
	contentWidth := halfWidth - lineNumWidth - 3
	if contentWidth < 0 {
		contentWidth = 0
	}
	leftContent := strings.Repeat(" ", contentWidth)
	rightContent := strings.Repeat(" ", contentWidth)

	// Format: arrow + space + gutter + space + content + (3 spaces for separator area) + arrow + space + gutter + space + content
	// This matches the layout of content lines but without the │ since header spacer is above the separator area
	return cursorArrowStyle.Render("➤") + " " + leftGutter + " " + leftContent + "   " +
		cursorArrowStyle.Render("➤") + " " + rightGutter + " " + rightContent
}

// renderTopBar renders the top bar showing file info with a divider line below.
func (m Model) renderTopBar() string {
	info := m.StatusInfo()

	// Only show file info when cursor is on a file (not on summary row)
	var content string
	if info.CurrentFile > 0 {
		content = m.formatStatusFileInfo(info)
	}

	// Leading arrow indicator (matches cursor arrow in gutter)
	prefix := cursorArrowStyle.Render("➤") + " "

	// Pad to fill the width (accounting for prefix: arrow + space = 2)
	contentWidth := displayWidth(content)
	padding := m.width - contentWidth - 2
	if padding < 0 {
		padding = 0
	}

	topLine := prefix + content + strings.Repeat(" ", padding)

	// Divider line using upper 1/8 block (dim)
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	divider := dividerStyle.Render(strings.Repeat("▔", m.width))

	return topLine + "\n" + divider
}

// renderStatusBar renders the status bar at the bottom of the screen.
// This now only contains the less-style indicator (file info is in top bar).
func (m Model) renderStatusBar() string {
	// In search mode, show search prompt
	if m.searchMode {
		return m.renderSearchPrompt()
	}

	info := m.StatusInfo()

	// Build less-style line indicator (with reverse styling)
	lessIndicator := formatLessIndicator(info.CurrentLine, info.TotalLines, info.Percentage, info.AtEnd)

	// Pad to max width to prevent shrinking (maxLessWidth is computed in calculateTotalLines)
	lessWidth := displayWidth(lessIndicator)
	if lessWidth < m.maxLessWidth {
		lessIndicator += strings.Repeat(" ", m.maxLessWidth-lessWidth)
	}

	// Apply reverse style to the less indicator portion
	styledLessIndicator := statusStyle.Render(" " + lessIndicator)

	// When there's an active search query, show search info
	var searchInfo string
	if m.searchQuery != "" {
		if len(m.matches) == 0 {
			searchInfo = " No matches"
		} else {
			searchInfo = fmt.Sprintf(" %d/%d", m.currentMatch+1, len(m.matches))
		}
	}

	// Pager mode indicator (right-aligned)
	var pagerIndicator string
	if m.pagerMode {
		pagerIndicator = "PAGER"
	}

	// Combine: reversed_less_indicator + search_info + padding + pager_indicator
	content := styledLessIndicator + searchInfo
	contentWidth := displayWidth(" "+lessIndicator) + displayWidth(searchInfo)
	pagerWidth := displayWidth(pagerIndicator)

	// Calculate padding between content and pager indicator
	padding := m.width - contentWidth - pagerWidth
	if padding < 0 {
		padding = 0
	}

	return content + strings.Repeat(" ", padding) + pagerIndicator
}

// formatStatusFileInfo formats the file info for the status bar.
// Format: foldIcon statusIcon fileName +N -M
func (m Model) formatStatusFileInfo(info StatusInfo) string {
	// Get fold level icon
	icon := m.foldLevelIcon(info.FoldLevel)

	// Get status indicator
	statusSymbol, statusStyle := fileStatusIndicator(FileStatus(info.FileStatus))
	styledStatus := statusStyle.Render(statusSymbol)

	// Format stats (only show if there are changes)
	var stats string
	if info.Added > 0 || info.Removed > 0 {
		var parts []string
		if info.Added > 0 {
			parts = append(parts, addedStyle.Render(fmt.Sprintf("+%d", info.Added)))
		}
		if info.Removed > 0 {
			parts = append(parts, removedStyle.Render(fmt.Sprintf("-%d", info.Removed)))
		}
		stats = " " + strings.Join(parts, " ")
	}

	return icon + " " + styledStatus + " " + info.FileName + stats
}

// renderSearchPrompt renders the status bar as a search input prompt.
// Uses normal styling (not reversed) so the search input is visible.
func (m Model) renderSearchPrompt() string {
	// Show / for forward, ? for backward
	prefix := "/"
	if !m.searchForward {
		prefix = "?"
	}

	left := " " + prefix + m.searchInput

	// Calculate padding to fill the width
	leftWidth := displayWidth(left)
	padding := m.width - leftWidth
	if padding < 0 {
		padding = 0
	}

	// No reverse styling for search prompt - just return plain text with padding
	return left + strings.Repeat(" ", padding)
}

// hasMatchOnRow returns true if there are search matches on the given row and side.
func (m Model) hasMatchOnRow(rowIdx, side int) bool {
	for _, match := range m.matches {
		if match.Row == rowIdx && match.Side == side {
			return true
		}
	}
	return false
}

// highlightSearchInVisible highlights search matches in visible text.
// It finds the query in the visible text and applies highlighting.
func (m Model) highlightSearchInVisible(visible string, rowIdx, side int) string {
	if m.searchQuery == "" {
		return visible
	}

	query := m.searchQuery
	caseSensitive := isSmartCaseSensitive(query)

	searchIn := visible
	if !caseSensitive {
		searchIn = strings.ToLower(visible)
		query = strings.ToLower(query)
	}

	// Find the current match index for this row/side to highlight it differently
	currentMatchOnRow := -1
	for i, match := range m.matches {
		if match.Row == rowIdx && match.Side == side && i == m.currentMatch {
			currentMatchOnRow = match.Col
			break
		}
	}

	// Find and highlight all occurrences
	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(searchIn[lastEnd:], query)
		if idx == -1 {
			break
		}
		pos := lastEnd + idx

		// Add text before match
		result.WriteString(visible[lastEnd:pos])

		// Determine if this is the current match
		// We check if the position in the original content would match
		originalPos := pos + m.hscroll
		isCurrent := originalPos == currentMatchOnRow

		// Add highlighted match
		end := pos + len(m.searchQuery)
		if end > len(visible) {
			end = len(visible)
		}

		matchText := visible[pos:end]
		if isCurrent {
			result.WriteString(searchCurrentMatchStyle.Render(matchText))
		} else {
			result.WriteString(searchMatchStyle.Render(matchText))
		}
		lastEnd = end
	}

	// Add remaining text
	if lastEnd < len(visible) {
		result.WriteString(visible[lastEnd:])
	}

	return result.String()
}

// applySearchHighlight applies search highlighting to text for a given row and side.
func (m Model) applySearchHighlight(text string, rowIdx, side int) string {
	if len(m.matches) == 0 {
		return text
	}

	// Find matches for this row and side
	var rowMatches []Match
	for i, match := range m.matches {
		if match.Row == rowIdx && match.Side == side {
			rowMatches = append(rowMatches, match)
			// Mark if this is the current match
			if i == m.currentMatch {
				rowMatches[len(rowMatches)-1].Col = -rowMatches[len(rowMatches)-1].Col - 1 // negative marks current
			}
		}
	}

	if len(rowMatches) == 0 {
		return text
	}

	// Build highlighted text
	queryLen := len(m.searchQuery)
	var result strings.Builder
	lastEnd := 0

	for _, match := range rowMatches {
		col := match.Col
		isCurrent := col < 0
		if isCurrent {
			col = -col - 1
		}

		if col < lastEnd || col >= len(text) {
			continue
		}

		// Add text before match
		result.WriteString(text[lastEnd:col])

		// Add highlighted match
		end := col + queryLen
		if end > len(text) {
			end = len(text)
		}

		matchText := text[col:end]
		if isCurrent {
			result.WriteString(searchCurrentMatchStyle.Render(matchText))
		} else {
			result.WriteString(searchMatchStyle.Render(matchText))
		}
		lastEnd = end
	}

	// Add remaining text
	if lastEnd < len(text) {
		result.WriteString(text[lastEnd:])
	}

	return result.String()
}

// countFileStats counts the number of added and removed lines in a file.
// TODO: Handle binary files and renames - they should display differently
// (e.g., "Binary file changed" or show rename info without line stats).
func countFileStats(fp sidebyside.FilePair) (added, removed int) {
	for _, pair := range fp.Pairs {
		if pair.Right.Type == sidebyside.Added {
			added++
		}
		if pair.Left.Type == sidebyside.Removed {
			removed++
		}
	}
	return added, removed
}

// formatStatsBar formats the stats as "+N -M +++---" with proportional scaling.
// If total changes exceed maxWidth, the bar is scaled proportionally.
// Returns empty string if there are no changes.
func formatStatsBar(added, removed, maxWidth int) string {
	if added == 0 && removed == 0 {
		return ""
	}

	var parts []string

	// Build the count prefix: "+N" and/or "-M"
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+%d", added))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("-%d", removed))
	}

	// Calculate bar characters
	total := added + removed
	plusChars := added
	minusChars := removed

	// Scale if exceeds maxWidth
	if total > maxWidth {
		scale := float64(maxWidth) / float64(total)
		plusChars = int(float64(added) * scale)
		minusChars = int(float64(removed) * scale)
		// Ensure we don't lose representation for non-zero counts
		if added > 0 && plusChars == 0 {
			plusChars = 1
		}
		if removed > 0 && minusChars == 0 {
			minusChars = 1
		}
	}

	bar := strings.Repeat("+", plusChars) + strings.Repeat("-", minusChars)
	parts = append(parts, bar)

	return strings.Join(parts, " ")
}

func formatFileHeader(oldPath, newPath string) string {
	if oldPath == newPath || oldPath == "/dev/null" {
		return newPath
	}
	if newPath == "/dev/null" {
		return oldPath + " (deleted)"
	}
	// Strip a/ and b/ prefixes if present
	old := strings.TrimPrefix(oldPath, "a/")
	new := strings.TrimPrefix(newPath, "b/")
	if old == new {
		return old
	}
	return old + " → " + new
}

// formatLessIndicator formats the less-style line indicator.
// Returns "line N/TOTAL X%" normally, or "line N/TOTAL (END)" when at end.
func formatLessIndicator(line, total, percentage int, atEnd bool) string {
	if atEnd {
		return fmt.Sprintf("line %d/%d (END)", line, total)
	}
	return fmt.Sprintf("line %d/%d %d%%", line, total, percentage)
}

// foldLevelIcon returns the icon for a given fold level.
// ○ = Folded (empty/minimal), ◐ = Normal (half), ● = Expanded (full)
// In pager mode, FoldNormal shows ● (filled) to indicate max expansion.
func (m Model) foldLevelIcon(level sidebyside.FoldLevel) string {
	switch level {
	case sidebyside.FoldFolded:
		return "○"
	case sidebyside.FoldExpanded:
		return "●"
	default: // FoldNormal
		if m.pagerMode {
			// In pager mode, FoldNormal is max expansion (no FoldExpanded available)
			return "●"
		}
		return "◐"
	}
}

// FileStatus represents the status of a file in a diff.
type FileStatus string

const (
	FileStatusAdded    FileStatus = "added"
	FileStatusDeleted  FileStatus = "deleted"
	FileStatusRenamed  FileStatus = "renamed"
	FileStatusModified FileStatus = "modified"
)

// fileStatus determines the status of a file based on its old and new paths.
func fileStatus(oldPath, newPath string) FileStatus {
	// Added: old path is /dev/null
	if oldPath == "/dev/null" {
		return FileStatusAdded
	}
	// Deleted: new path is /dev/null
	if newPath == "/dev/null" {
		return FileStatusDeleted
	}
	// Renamed: paths differ after stripping a/ and b/ prefixes
	old := strings.TrimPrefix(oldPath, "a/")
	new := strings.TrimPrefix(newPath, "b/")
	if old != new {
		return FileStatusRenamed
	}
	// Modified: everything else
	return FileStatusModified
}

// fileStatusIndicator returns the symbol and style for a file status.
// + (green) for added, - (red) for deleted, > (blue) for renamed, ~ (blue) for modified.
func fileStatusIndicator(status FileStatus) (symbol string, style lipgloss.Style) {
	switch status {
	case FileStatusAdded:
		return "+", addedStyle
	case FileStatusDeleted:
		return "-", removedStyle
	case FileStatusRenamed:
		return ">", lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	default: // FileStatusModified
		return "~", lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	}
}

// statsCountWidth returns the display width of the count portion "+N -M" (without bar).
func statsCountWidth(added, removed int) int {
	width := 0
	if added > 0 {
		width += len(fmt.Sprintf("+%d", added))
	}
	if removed > 0 {
		if width > 0 {
			width++ // space between +N and -M
		}
		width += len(fmt.Sprintf("-%d", removed))
	}
	return width
}

// formatColoredStatsBar returns the stats display with colored +/- characters.
// Returns empty string if no changes. Format: " | +N -M +++---"
// maxCountWidth is used to pad the count portion for alignment across files.
func formatColoredStatsBar(added, removed, maxBarWidth, maxCountWidth int) string {
	if added == 0 && removed == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "|")

	// Build count string and pad for alignment
	var countParts []string
	if added > 0 {
		countParts = append(countParts, addedStyle.Render(fmt.Sprintf("+%d", added)))
	}
	if removed > 0 {
		countParts = append(countParts, removedStyle.Render(fmt.Sprintf("-%d", removed)))
	}
	countStr := strings.Join(countParts, " ")

	// Pad to align bars
	currentCountWidth := statsCountWidth(added, removed)
	if maxCountWidth > currentCountWidth {
		countStr += strings.Repeat(" ", maxCountWidth-currentCountWidth)
	}
	parts = append(parts, countStr)

	// Calculate bar characters with scaling
	// We derive minusChars from plusChars to ensure total is always exactly maxBarWidth
	total := added + removed
	plusChars := added
	minusChars := removed

	if total > maxBarWidth {
		// Calculate plusChars proportionally, derive minusChars from remainder
		plusChars = int(float64(added) / float64(total) * float64(maxBarWidth))
		minusChars = maxBarWidth - plusChars

		// Ensure minimum 1 char representation if changes exist
		if added > 0 && plusChars == 0 {
			plusChars = 1
			minusChars = maxBarWidth - 1
		}
		if removed > 0 && minusChars == 0 {
			minusChars = 1
			plusChars = maxBarWidth - 1
		}
	}

	// Build colored bar
	bar := addedStyle.Render(strings.Repeat("+", plusChars)) + removedStyle.Render(strings.Repeat("-", minusChars))
	parts = append(parts, bar)

	return " " + strings.Join(parts, " ")
}

// statsBarDisplayWidth returns the display width of the stats bar (without ANSI codes).
// This matches formatColoredStatsBar's output width.
func statsBarDisplayWidth(added, removed, maxBarWidth, maxCountWidth int) int {
	if added == 0 && removed == 0 {
		return 0
	}

	// Format: " | countStr barStr"
	// Leading space + | + space + count + space + bar
	width := 1 + 1 + 1 // " | "

	// Count width (padded to maxCountWidth)
	width += maxCountWidth

	// Space before bar
	width += 1

	// Bar width (capped at maxBarWidth)
	total := added + removed
	if total > maxBarWidth {
		width += maxBarWidth
	} else {
		width += total
	}

	return width
}

// formatSummaryStats returns a git-style summary string like "2 files changed, 5 insertions(+), 3 deletions(-)".
// Handles singular/plural and omits zero-count sections.
func formatSummaryStats(files, added, removed int) string {
	var parts []string

	// Files changed
	if files == 1 {
		parts = append(parts, "1 file changed")
	} else {
		parts = append(parts, fmt.Sprintf("%d files changed", files))
	}

	// Insertions
	if added > 0 {
		if added == 1 {
			parts = append(parts, "1 insertion(+)")
		} else {
			parts = append(parts, fmt.Sprintf("%d insertions(+)", added))
		}
	}

	// Deletions
	if removed > 0 {
		if removed == 1 {
			parts = append(parts, "1 deletion(-)")
		} else {
			parts = append(parts, fmt.Sprintf("%d deletions(-)", removed))
		}
	}

	return strings.Join(parts, ", ")
}

// renderSummary renders the summary row at the bottom of the diff view.
// Format: "➤ ━━━ ●   N files changed, N insertions(+), N deletions(-)" (when cursor)
// Uses expanded icon (●) since there's no additional content to show.
// Text is not bold, unlike file headers.
func (m Model) renderSummary(totalFiles, totalAdded, totalRemoved, maxHeaderWidth int, isCursorRow bool) string {
	lineNumWidth := m.lineNumWidth()
	equalsGutter := strings.Repeat("━", lineNumWidth)
	icon := m.foldLevelIcon(sidebyside.FoldExpanded) // Always use expanded icon
	// Space where status indicator would be (empty for summary)
	iconPart := " " + icon + "   " // icon + 3 spaces (status position + space)

	summary := formatSummaryStats(totalFiles, totalAdded, totalRemoved)

	// Use non-bold style for summary (just the foreground color)
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	if isCursorRow {
		// Format: arrow + space + gutter(━━━ with bg) + space + icon + summary
		return cursorArrowStyle.Render("➤") + " " + cursorStyle.Render(equalsGutter) + summaryStyle.Render(iconPart+summary)
	}
	// Format: space + space + gutter(━━━ dim) + space + icon + summary
	return "  " + headerLineStyle.Render(equalsGutter) + summaryStyle.Render(iconPart+summary)
}

func (m Model) renderHeader(header string, foldLevel sidebyside.FoldLevel, status FileStatus, added, removed, maxHeaderWidth, maxCountWidth, rowIdx int, isCursorRow bool) string {
	// Apply search highlighting if there are matches
	if m.searchQuery != "" {
		header = m.applySearchHighlight(header, rowIdx, 0)
	}

	// Get fold level icon and file status indicator
	icon := m.foldLevelIcon(foldLevel)
	statusSymbol, statusStyle := fileStatusIndicator(status)
	styledStatus := statusStyle.Render(statusSymbol)

	lineNumWidth := m.lineNumWidth()
	// Gutter uses ━ repeated to match line number width
	equalsGutter := strings.Repeat("━", lineNumWidth)

	// All headers use same format: gutter + icon + status + header + stats + trailing
	const maxBarWidth = 24
	statsBar := formatColoredStatsBar(added, removed, maxBarWidth, maxCountWidth)
	statsBarWidth := statsBarDisplayWidth(added, removed, maxBarWidth, maxCountWidth)

	// Pad header to align | separator across all files
	headerTextWidth := displayWidth(header)
	padding := ""
	if maxHeaderWidth > headerTextWidth {
		padding = strings.Repeat(" ", maxHeaderWidth-headerTextWidth)
	}

	// Calculate trailing spaces to fill the width
	// Format: prefix(2) + gutter + space + icon + space + status + space + header + padding + statsBar + trailing
	iconPartWidth := 1 + len(icon) + 1 + len(statusSymbol) + 1 // " icon status "
	prefixWidth := 2 + lineNumWidth + iconPartWidth + headerTextWidth + len(padding) + statsBarWidth
	trailing := m.width - prefixWidth
	if trailing < 0 {
		trailing = 0
	}
	trailingSpace := ""
	if trailing > 0 {
		trailingSpace = strings.Repeat(" ", trailing)
	}

	if isCursorRow {
		// Format: arrow + space + gutter(━━━ with bg) + space + icon + status + header + padding + stats + trailing
		styledGutter := cursorStyle.Render(equalsGutter)
		return cursorArrowStyle.Render("➤") + " " + styledGutter + headerStyle.Render(" "+icon+" ") + styledStatus + headerStyle.Render(" "+header+padding) + statsBar + trailingSpace
	}

	// Normal rendering
	// Format: space + space + gutter(━━━) + space + icon + status + header + padding + stats + trailing
	return "  " + headerLineStyle.Render(equalsGutter) + headerStyle.Render(" "+icon+" ") + styledStatus + headerStyle.Render(" "+header+padding) + statsBar + trailingSpace
}

func (m Model) renderLinePair(pair sidebyside.LinePair, fileIndex, halfWidth, lineNumWidth, rowIdx int, isCursorRow bool, leftIndicatorStart, rightIndicatorStart int, isFirstLine, isLastLine bool) string {
	contentWidth := halfWidth - lineNumWidth - 3 // -3 for indicator, space after indicator, and space after line num

	// Choose separator based on position: ┬ for first line, ┴ for last line, │ for middle
	separatorChar := "│"
	if isFirstLine {
		separatorChar = "┬"
	} else if isLastLine {
		separatorChar = "┴"
	}

	// Check if this is a modified pair where we should show inline diff
	isModifiedPair := pair.Left.Type == sidebyside.Removed && pair.Right.Type == sidebyside.Added

	var leftSpans, rightSpans []inlinediff.Span
	if isModifiedPair {
		// Expand tabs first since that's what we'll render
		leftContent := expandTabs(pair.Left.Content)
		rightContent := expandTabs(pair.Right.Content)

		// Only do inline diff if lines are similar enough
		if !inlinediff.ShouldSkipInlineDiff(leftContent, rightContent) {
			leftSpans, rightSpans = inlinediff.Diff(leftContent, rightContent)

			// Also skip if too much would be highlighted (not useful)
			if inlinediff.ShouldSkipBasedOnSpans(leftSpans, len(leftContent)) ||
				inlinediff.ShouldSkipBasedOnSpans(rightSpans, len(rightContent)) {
				leftSpans, rightSpans = nil, nil
			}
		}
	}

	// Get syntax highlight spans for each side
	leftSyntax := m.getLineSpans(fileIndex, pair.Left.Num, true)
	rightSyntax := m.getLineSpans(fileIndex, pair.Right.Num, false)

	// Use blue "changed" styling when we have word-level diff (both sides modified)
	hasWordDiff := len(leftSpans) > 0

	left := m.renderLineWithSpans(pair.Left, contentWidth, lineNumWidth, leftSpans, leftSyntax, rowIdx, 0, isCursorRow, leftIndicatorStart, hasWordDiff)
	right := m.renderLineWithSpans(pair.Right, contentWidth, lineNumWidth, rightSpans, rightSyntax, rowIdx, 1, isCursorRow, rightIndicatorStart, hasWordDiff)

	separator := hunkSeparatorStyle.Render(separatorChar)
	return left + " " + separator + " " + right
}

func (m Model) renderLineWithSpans(line sidebyside.Line, contentWidth, lineNumWidth int, inlineSpans []inlinediff.Span, syntaxSpans []highlight.Span, rowIdx, side int, isCursorRow bool, indicatorStart int, hasWordDiff bool) string {
	// Diff indicator (+/-/~/space) before line number
	// On cursor row, show arrowhead instead
	// When hasWordDiff is true, use blue "~" instead of green/red +/-
	var indicator string
	if isCursorRow {
		indicator = cursorArrowStyle.Render("➤")
	} else if hasWordDiff && (line.Type == sidebyside.Added || line.Type == sidebyside.Removed) {
		indicator = changedStyle.Render("~")
	} else {
		switch line.Type {
		case sidebyside.Added:
			indicator = addedStyle.Render("+")
		case sidebyside.Removed:
			indicator = removedStyle.Render("-")
		default:
			indicator = " "
		}
	}

	// Line number (fixed, not affected by horizontal scroll)
	// Color matches the +/- indicator: green for added, red for removed, blue for changed, dim for context
	var numStr string
	numContent := fmt.Sprintf("%*d", lineNumWidth, line.Num)
	if line.Num == 0 {
		numContent = strings.Repeat(" ", lineNumWidth)
	}
	if isCursorRow {
		numStr = cursorStyle.Render(numContent)
	} else {
		switch line.Type {
		case sidebyside.Added:
			if hasWordDiff {
				numStr = changedStyle.Render(numContent)
			} else {
				numStr = addedStyle.Render(numContent)
			}
		case sidebyside.Removed:
			if hasWordDiff {
				numStr = changedStyle.Render(numContent)
			} else {
				numStr = removedStyle.Render(numContent)
			}
		default:
			numStr = lineNumStyle.Render(numContent)
		}
	}

	// Content - expand tabs
	expanded := expandTabs(line.Content)

	// Reduce content width to make room for gutter columns on both sides
	// Layout: [gutter char] + space + content + space + [gutter char] (4 chars total)
	// Added/removed lines get ░, context/empty lines get spaces
	actualContentWidth := contentWidth - 4

	// Apply horizontal scroll to get visible portion
	visible := horizontalSlice(expanded, m.hscroll, actualContentWidth)

	// Apply styling with layers: syntax (base) -> inline diff -> search (top)
	// Exception: context lines on old side are dimmed (no syntax highlighting)
	var styledContent string
	isOldSideContext := side == 0 && line.Type == sidebyside.Context

	if isOldSideContext {
		// Dim context lines on the old side - they're duplicates of the new side
		displayContent := visible
		if m.searchQuery != "" && m.hasMatchOnRow(rowIdx, side) {
			displayContent = m.highlightSearchInVisible(visible, rowIdx, side)
		}
		styledContent = contextDimStyle.Render(displayContent)
	} else if len(inlineSpans) > 0 && (line.Type == sidebyside.Added || line.Type == sidebyside.Removed) {
		// Apply inline diff highlighting (with search highlighting taking precedence)
		styledContent = m.applyInlineSpans(expanded, visible, inlineSpans, line.Type, rowIdx, side)
	} else if len(syntaxSpans) > 0 {
		// Apply syntax highlighting as base, with search on top
		styledContent = m.applySyntaxHighlight(line.Content, expanded, visible, syntaxSpans, rowIdx, side)
	} else {
		// Apply search highlighting first if applicable
		displayContent := visible
		if m.searchQuery != "" && m.hasMatchOnRow(rowIdx, side) {
			displayContent = m.highlightSearchInVisible(visible, rowIdx, side)
		}

		// Apply simple style based on type
		switch line.Type {
		case sidebyside.Empty:
			styledContent = emptyStyle.Render(displayContent)
		default:
			styledContent = contextStyle.Render(displayContent)
		}
	}

	// Wrap added/removed lines with gutter indicators
	// Use blue for changed lines (hasWordDiff), otherwise green/red
	styledContent = m.applyColumnIndicators(styledContent, actualContentWidth, line.Type, indicatorStart, hasWordDiff)

	return indicator + " " + numStr + " " + styledContent
}

// applyInlineSpans applies inline diff highlighting to visible content.
// It maps spans from the full expanded string to the visible viewport slice.
// Search highlighting takes precedence over inline diff highlighting.
func (m Model) applyInlineSpans(expanded, visible string, spans []inlinediff.Span, lineType sidebyside.LineType, rowIdx, side int) string {
	// Base style is context (no color) since gutter shows +/- indicators
	// Highlight style matches the line type (green for added, red for removed)
	baseStyle := contextStyle
	var highlightStyle lipgloss.Style
	if lineType == sidebyside.Added {
		highlightStyle = inlineAddedStyle
	} else {
		highlightStyle = inlineRemovedStyle
	}

	// Map byte positions to display columns in the expanded string
	byteToCol := make([]int, len(expanded)+1)
	col := 0
	bytePos := 0
	for _, r := range expanded {
		byteToCol[bytePos] = col
		rw := runewidth.RuneWidth(r)
		col += rw
		bytePos += len(string(r))
	}
	byteToCol[len(expanded)] = col

	// Build search match ranges (in visible coordinates) if we have matches
	type searchRange struct {
		start, end int
		isCurrent  bool
	}
	var searchRanges []searchRange
	if m.searchQuery != "" && m.hasMatchOnRow(rowIdx, side) {
		queryLen := len(m.searchQuery)
		caseSensitive := isSmartCaseSensitive(m.searchQuery)
		query := m.searchQuery
		searchIn := visible
		if !caseSensitive {
			searchIn = strings.ToLower(visible)
			query = strings.ToLower(query)
		}

		// Find all occurrences in visible text
		pos := 0
		for {
			idx := strings.Index(searchIn[pos:], query)
			if idx == -1 {
				break
			}
			start := pos + idx
			end := start + queryLen
			if end > len(visible) {
				end = len(visible)
			}

			// Check if this is the current match
			originalPos := start + m.hscroll
			isCurrent := false
			for i, match := range m.matches {
				if match.Row == rowIdx && match.Side == side && match.Col == originalPos && i == m.currentMatch {
					isCurrent = true
					break
				}
			}

			searchRanges = append(searchRanges, searchRange{start: start, end: end, isCurrent: isCurrent})
			pos = start + 1
		}
	}

	// Build styled output for each visible column
	var result strings.Builder
	visibleRunes := []rune(visible)
	visibleCol := 0
	visibleBytePos := 0

	for _, vr := range visibleRunes {
		vrWidth := runewidth.RuneWidth(vr)
		actualCol := m.hscroll + visibleCol

		// Check if in search match first (takes precedence)
		inSearch := false
		isCurrentSearch := false
		for _, sr := range searchRanges {
			if visibleBytePos >= sr.start && visibleBytePos < sr.end {
				inSearch = true
				isCurrentSearch = sr.isCurrent
				break
			}
		}

		if inSearch {
			// Search highlight takes precedence
			if isCurrentSearch {
				result.WriteString(searchCurrentMatchStyle.Render(string(vr)))
			} else {
				result.WriteString(searchMatchStyle.Render(string(vr)))
			}
		} else {
			// Check inline diff highlight
			inHighlight := false
			for _, span := range spans {
				spanStartCol := byteToCol[span.Start]
				spanEndCol := byteToCol[span.End]
				if actualCol >= spanStartCol && actualCol < spanEndCol {
					if span.Type == inlinediff.Added || span.Type == inlinediff.Removed {
						inHighlight = true
					}
					break
				}
			}

			if inHighlight {
				result.WriteString(highlightStyle.Render(string(vr)))
			} else {
				result.WriteString(baseStyle.Render(string(vr)))
			}
		}

		visibleCol += vrWidth
		visibleBytePos += len(string(vr))
	}

	return result.String()
}

// applySyntaxHighlight applies syntax highlighting to visible content.
// It maps spans from the original line to the visible viewport slice,
// with search highlighting taking precedence.
// The `original` parameter is the original line content (before tab expansion).
func (m Model) applySyntaxHighlight(original, _, visible string, syntaxSpans []highlight.Span, rowIdx, side int) string {
	if len(syntaxSpans) == 0 {
		// No syntax spans, just apply search if applicable
		if m.searchQuery != "" && m.hasMatchOnRow(rowIdx, side) {
			return m.highlightSearchInVisible(visible, rowIdx, side)
		}
		return visible
	}

	// Map byte positions from ORIGINAL content to display columns
	// Syntax spans have offsets into the original (non-tab-expanded) content
	byteToCol := make([]int, len(original)+1)
	col := 0
	bytePos := 0
	for _, r := range original {
		byteToCol[bytePos] = col
		var rw int
		if r == '\t' {
			// Tab expands to next tab stop
			rw = TabWidth - (col % TabWidth)
		} else {
			rw = runewidth.RuneWidth(r)
		}
		col += rw
		bytePos += len(string(r))
	}
	byteToCol[len(original)] = col

	// Build search match ranges (in visible coordinates) if we have matches
	type searchRange struct {
		start, end int
		isCurrent  bool
	}
	var searchRanges []searchRange
	if m.searchQuery != "" && m.hasMatchOnRow(rowIdx, side) {
		queryLen := len(m.searchQuery)
		caseSensitive := isSmartCaseSensitive(m.searchQuery)
		query := m.searchQuery
		searchIn := visible
		if !caseSensitive {
			searchIn = strings.ToLower(visible)
			query = strings.ToLower(query)
		}

		pos := 0
		for {
			idx := strings.Index(searchIn[pos:], query)
			if idx == -1 {
				break
			}
			start := pos + idx
			end := start + queryLen
			if end > len(visible) {
				end = len(visible)
			}

			// Check if this is the current match
			originalPos := start + m.hscroll
			isCurrent := false
			for i, match := range m.matches {
				if match.Row == rowIdx && match.Side == side && match.Col == originalPos && i == m.currentMatch {
					isCurrent = true
					break
				}
			}

			searchRanges = append(searchRanges, searchRange{start: start, end: end, isCurrent: isCurrent})
			pos = start + 1
		}
	}

	// Get theme for syntax coloring
	theme := m.highlighter.Theme()

	// Build styled output for each visible character
	var result strings.Builder
	visibleRunes := []rune(visible)
	visibleCol := 0
	visibleBytePos := 0

	for _, vr := range visibleRunes {
		vrWidth := runewidth.RuneWidth(vr)
		actualCol := m.hscroll + visibleCol

		// Check if in search match first (takes precedence)
		inSearch := false
		isCurrentSearch := false
		for _, sr := range searchRanges {
			if visibleBytePos >= sr.start && visibleBytePos < sr.end {
				inSearch = true
				isCurrentSearch = sr.isCurrent
				break
			}
		}

		if inSearch {
			// Search highlight takes precedence
			if isCurrentSearch {
				result.WriteString(searchCurrentMatchStyle.Render(string(vr)))
			} else {
				result.WriteString(searchMatchStyle.Render(string(vr)))
			}
		} else {
			// Find syntax category for this position
			foundStyle := false
			for _, span := range syntaxSpans {
				spanStartCol := 0
				spanEndCol := 0
				if span.Start < len(byteToCol) {
					spanStartCol = byteToCol[span.Start]
				}
				if span.End < len(byteToCol) {
					spanEndCol = byteToCol[span.End]
				} else if span.End >= len(byteToCol) {
					spanEndCol = byteToCol[len(byteToCol)-1]
				}

				if actualCol >= spanStartCol && actualCol < spanEndCol {
					style := theme.Style(span.Category)
					result.WriteString(style.Render(string(vr)))
					foundStyle = true
					break
				}
			}

			if !foundStyle {
				result.WriteString(string(vr))
			}
		}

		visibleCol += vrWidth
		visibleBytePos += len(string(vr))
	}

	return result.String()
}

// TabWidth is the number of spaces a tab character expands to.
const TabWidth = 4

// expandTabs replaces tab characters with spaces.
// This is necessary because runewidth treats tabs as width 0,
// but terminals render them with variable width.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var result strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			// Expand to next tab stop
			spaces := TabWidth - (col % TabWidth)
			result.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			result.WriteRune(r)
			col += runewidth.RuneWidth(r)
		}
	}
	return result.String()
}

// displayWidth returns the display width of a string, accounting for
// wide characters (CJK, emoji) that take 2 cells.
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// truncateOrPad truncates or pads a string to exactly the given display width.
// It properly handles multi-byte and wide characters.
func truncateOrPad(s string, width int) string {
	sw := displayWidth(s)

	if sw <= width {
		// Pad with spaces
		return s + strings.Repeat(" ", width-sw)
	}

	// Need to truncate
	if width <= 3 {
		// Too narrow for ellipsis, just truncate
		return runewidth.Truncate(s, width, "")
	}

	// Truncate with ellipsis
	return runewidth.Truncate(s, width, "...")
}

// horizontalSlice returns a slice of a string starting at the given display
// column offset and spanning the given width. It handles wide characters
// (CJK, emoji) properly - if the offset lands in the middle of a wide char,
// that position is replaced with a space. The result is always exactly `width`
// display columns, padded with spaces if needed.
func horizontalSlice(s string, offset, width int) string {
	if width <= 0 {
		return ""
	}

	var result strings.Builder
	col := 0             // current display column
	resultWidth := 0     // width of result so far
	skippedHalf := false // true if we skipped half of a wide char at offset

	for _, r := range s {
		rw := runewidth.RuneWidth(r)

		// Still in the skip zone (before offset)
		if col < offset {
			// Check if this wide char spans the offset boundary
			if col+rw > offset && rw > 1 {
				// Wide char straddles the offset - we're cutting it in half
				skippedHalf = true
			}
			col += rw
			continue
		}

		// We're at or past the offset - start collecting

		// If we just started and skipped half a wide char, emit a space
		if col == offset && skippedHalf {
			result.WriteRune(' ')
			resultWidth++
			skippedHalf = false
		}

		// Check if this rune fits in remaining width
		if resultWidth+rw > width {
			break
		}

		result.WriteRune(r)
		resultWidth += rw
		col += rw
	}

	// Handle case where offset was past content, or we skipped into empty
	if col <= offset && skippedHalf {
		// We never emitted anything but owe a space for half-wide-char
		result.WriteRune(' ')
		resultWidth++
	}

	// Pad to exact width
	if resultWidth < width {
		result.WriteString(strings.Repeat(" ", width-resultWidth))
	}

	return result.String()
}

// findLastNonWhitespaceCol returns the display column (0-indexed) after the
// last non-whitespace character in a string. Returns 0 if string is empty or
// all whitespace.
func findLastNonWhitespaceCol(s string) int {
	lastNonWsCol := 0
	col := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		col += rw
		if r != ' ' && r != '\t' {
			lastNonWsCol = col
		}
	}
	return lastNonWsCol
}

// indicatorStartCol calculates where the block-aligned indicator column should be.
// Returns lastNonWhitespaceCol + 1, or -1 if the line is all whitespace.
func indicatorStartCol(content string) int {
	expanded := expandTabs(content)
	lastNonWsCol := findLastNonWhitespaceCol(expanded)
	if lastNonWsCol == 0 {
		return -1 // all whitespace
	}
	return lastNonWsCol + 1
}

// computeIndicatorStarts calculates the block-aware indicator start positions.
// Within consecutive runs of Added (right) or Removed (left) lines, all lines
// in the block share the same start position (the max of the block).
// Returns maps from row index to indicator start column for left and right sides.
func computeIndicatorStarts(rows []displayRow) (leftStarts, rightStarts map[int]int) {
	leftStarts = make(map[int]int)
	rightStarts = make(map[int]int)

	// Process left side (Removed lines)
	var leftBlockIndices []int
	var leftBlockMaxStart int

	for i, row := range rows {
		if !row.isHeader && !row.isSeparator && !row.isBlank && !row.isHeaderSpacer && !row.isSummary {
			if row.pair.Left.Type == sidebyside.Removed {
				leftBlockIndices = append(leftBlockIndices, i)
				start := indicatorStartCol(row.pair.Left.Content)
				if start > leftBlockMaxStart {
					leftBlockMaxStart = start
				}
			} else {
				// End of block - assign max to all in block
				for _, idx := range leftBlockIndices {
					leftStarts[idx] = leftBlockMaxStart
				}
				leftBlockIndices = nil
				leftBlockMaxStart = 0
			}
		} else {
			// Non-line-pair row breaks the block
			for _, idx := range leftBlockIndices {
				leftStarts[idx] = leftBlockMaxStart
			}
			leftBlockIndices = nil
			leftBlockMaxStart = 0
		}
	}
	// Handle final block
	for _, idx := range leftBlockIndices {
		leftStarts[idx] = leftBlockMaxStart
	}

	// Process right side (Added lines)
	var rightBlockIndices []int
	var rightBlockMaxStart int

	for i, row := range rows {
		if !row.isHeader && !row.isSeparator && !row.isBlank && !row.isHeaderSpacer && !row.isSummary {
			if row.pair.Right.Type == sidebyside.Added {
				rightBlockIndices = append(rightBlockIndices, i)
				start := indicatorStartCol(row.pair.Right.Content)
				if start > rightBlockMaxStart {
					rightBlockMaxStart = start
				}
			} else {
				// End of block - assign max to all in block
				for _, idx := range rightBlockIndices {
					rightStarts[idx] = rightBlockMaxStart
				}
				rightBlockIndices = nil
				rightBlockMaxStart = 0
			}
		} else {
			// Non-line-pair row breaks the block
			for _, idx := range rightBlockIndices {
				rightStarts[idx] = rightBlockMaxStart
			}
			rightBlockIndices = nil
			rightBlockMaxStart = 0
		}
	}
	// Handle final block
	for _, idx := range rightBlockIndices {
		rightStarts[idx] = rightBlockMaxStart
	}

	return leftStarts, rightStarts
}

// applyColumnIndicators wraps lines with gutter columns:
// - Added/removed: ░ + space + content + space + ░
// - Context/empty: space + space + content + space + space
// Also optionally inserts a block-aligned indicator for added/removed lines.
func (m Model) applyColumnIndicators(styledContent string, contentWidth int, lineType sidebyside.LineType, indicatorStartAbs int, hasWordDiff bool) string {
	isAddedOrRemoved := lineType == sidebyside.Added || lineType == sidebyside.Removed

	// For context/empty lines, just wrap with spaces to align with added/removed
	if !isAddedOrRemoved {
		return "  " + styledContent + "  "
	}

	// Get indicator styles for added/removed lines
	// Start and end indicators: blue for changed (word diff), green/red otherwise
	var colorStyle lipgloss.Style
	if hasWordDiff {
		colorStyle = changedStyle // blue for modified lines with word diff
	} else if lineType == sidebyside.Added {
		colorStyle = addedStyle // green
	} else {
		colorStyle = removedStyle // red
	}
	startIndicator := colorStyle.Render("░")
	endIndicator := colorStyle.Render("░")
	// Block-aligned indicator is faint grey
	faintIndicator := blockIndicatorStyle.Faint(true).Render("░")

	// Calculate block-aligned position in visible coordinates (relative to content area)
	blockAlignedVisible := -1
	if indicatorStartAbs > 0 {
		blockAlignedAbs := indicatorStartAbs
		// End position is relative to the reduced content area
		endAbs := m.hscroll + contentWidth
		// Only show if at least 20 chars from end
		if endAbs-blockAlignedAbs >= 20 {
			blockAlignedVisible = blockAlignedAbs - m.hscroll
			// Must be within visible content area
			if blockAlignedVisible < 0 || blockAlignedVisible >= contentWidth {
				blockAlignedVisible = -1
			}
		}
	}

	// If we have a block-aligned indicator, insert it into the content
	var contentWithBlockIndicator string
	if blockAlignedVisible >= 0 {
		// Replace the character at blockAlignedVisible with the indicator
		var result strings.Builder
		inEscape := false
		visiblePos := 0

		for _, r := range styledContent {
			if inEscape {
				result.WriteRune(r)
				if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
					inEscape = false
				}
				continue
			}

			if r == '\x1b' {
				inEscape = true
				result.WriteRune(r)
				continue
			}

			if visiblePos == blockAlignedVisible {
				result.WriteString(faintIndicator)
			} else {
				result.WriteRune(r)
			}
			visiblePos++
		}
		contentWithBlockIndicator = result.String()
	} else {
		contentWithBlockIndicator = styledContent
	}

	// Wrap with start and end indicators: ░ + space + content + space + ░
	return startIndicator + " " + contentWithBlockIndicator + " " + endIndicator
}
