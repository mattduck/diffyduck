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
	hunkSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	addedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	removedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	contextStyle       = lipgloss.NewStyle()
	lineNumStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	emptyStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle        = lipgloss.NewStyle().Reverse(true)

	// Inline diff highlight: inverted (black on white)
	inlineAddedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("15"))
	inlineRemovedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("15"))

	// Search highlight styles (black text on yellow background)
	searchMatchStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3"))
	searchCurrentMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))

	// Cursor highlight style (bg=7 white, fg=0 black) for gutter areas
	cursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("7")).Foreground(lipgloss.Color("0"))
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

	// Pad with empty lines to fill viewport (so status bar is always at bottom)
	for len(visibleRows) < contentH {
		visibleRows = append(visibleRows, "")
	}

	// Add status bar
	statusBar := m.renderStatusBar()
	visibleRows = append(visibleRows, statusBar)

	return strings.Join(visibleRows, "\n")
}

// displayRow represents one row in the view (header, line pair, hunk separator, or blank)
type displayRow struct {
	fileIndex   int // index of the file this row belongs to
	isHeader    bool
	isSeparator bool
	isBlank     bool
	header      string
	foldLevel   sidebyside.FoldLevel // fold level for headers (used for icon and styling)
	pair        sidebyside.LinePair
}

// buildRows creates all displayable rows from the model data.
func (m Model) buildRows() []displayRow {
	var rows []displayRow

	for fileIdx, fp := range m.files {
		switch fp.FoldLevel {
		case sidebyside.FoldFolded:
			// Folded: just the header, no blank line before, no trailing "="
			header := formatFileHeader(fp.OldPath, fp.NewPath)
			rows = append(rows, displayRow{fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldFolded, header: header})

		case sidebyside.FoldExpanded:
			// Expanded: show full file content with diff highlighting
			// If content not loaded yet, fall back to normal view
			if fp.HasContent() {
				// Add blank line before file headers (except the first)
				// Blank line belongs to the file above, not below
				if fileIdx > 0 {
					rows = append(rows, displayRow{fileIndex: fileIdx - 1, isBlank: true})
				}

				// File header
				header := formatFileHeader(fp.OldPath, fp.NewPath)
				rows = append(rows, displayRow{fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldExpanded, header: header})

				// Build expanded rows from full file content
				expandedRows := m.buildExpandedRows(fp)
				for i := range expandedRows {
					expandedRows[i].fileIndex = fileIdx
				}
				rows = append(rows, expandedRows...)
				continue // Skip the normal view below
			}
			// Fall through to normal view if content not loaded
			fallthrough

		default: // FoldNormal
			// Add blank line before file headers (except the first)
			// Blank line belongs to the file above, not below
			if fileIdx > 0 {
				rows = append(rows, displayRow{fileIndex: fileIdx - 1, isBlank: true})
			}

			// File header
			header := formatFileHeader(fp.OldPath, fp.NewPath)
			rows = append(rows, displayRow{fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldNormal, header: header})

			// Line pairs with hunk separators
			var prevLeft, prevRight int
			for i, pair := range fp.Pairs {
				// Check for gap in line numbers (hunk boundary)
				if i > 0 && isHunkBoundary(prevLeft, prevRight, pair.Left.Num, pair.Right.Num) {
					rows = append(rows, displayRow{fileIndex: fileIdx, isSeparator: true})
				}

				rows = append(rows, displayRow{fileIndex: fileIdx, pair: pair})

				// Track previous line numbers (use non-zero values)
				if pair.Left.Num > 0 {
					prevLeft = pair.Left.Num
				}
				if pair.Right.Num > 0 {
					prevRight = pair.Right.Num
				}
			}
		}
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

		if row.isBlank {
			if isCursorRow {
				visible = append(visible, m.renderBlankWithCursor(halfWidth, lineNumWidth))
			} else {
				visible = append(visible, "")
			}
		} else if row.isHeader {
			visible = append(visible, m.renderHeader(row.header, row.foldLevel, i, isCursorRow))
		} else if row.isSeparator {
			visible = append(visible, m.renderHunkSeparator(halfWidth, isCursorRow))
		} else {
			visible = append(visible, m.renderLinePair(row.pair, row.fileIndex, halfWidth, lineNumWidth, i, isCursorRow))
		}
	}

	return visible
}

// renderHunkSeparator renders a separator line between hunks.
func (m Model) renderHunkSeparator(halfWidth int, isCursorRow bool) string {
	lineNumWidth := m.lineNumWidth()

	if isCursorRow {
		// Gutter area gets dashes, highlighted with cursor style
		// Format: indicator(dash) + space + gutter(dashes) + space + content(dashes)
		gutterDashes := cursorStyle.Render(strings.Repeat("─", lineNumWidth))

		// Content area with dashes
		contentWidth := halfWidth - lineNumWidth - 3
		contentDashes := strings.Repeat("─", contentWidth)

		separator := hunkSeparatorStyle.Render("│")
		// Build: dash + space + gutter_dashes + space + content_dashes
		return hunkSeparatorStyle.Render("─ ") + gutterDashes + hunkSeparatorStyle.Render(" "+contentDashes) +
			" " + separator + " " +
			hunkSeparatorStyle.Render("─ ") + gutterDashes + hunkSeparatorStyle.Render(" "+contentDashes)
	}

	// Normal rendering: full line of dashes
	leftDashes := strings.Repeat("─", halfWidth)
	rightDashes := strings.Repeat("─", halfWidth)
	return hunkSeparatorStyle.Render(leftDashes + "─┼─" + rightDashes)
}

// renderBlankWithCursor renders a blank line with highlighted gutter areas when cursor is on it.
func (m Model) renderBlankWithCursor(halfWidth, lineNumWidth int) string {
	// Highlight both gutter areas (left and right)
	leftGutter := cursorStyle.Render(strings.Repeat(" ", lineNumWidth))
	rightGutter := cursorStyle.Render(strings.Repeat(" ", lineNumWidth))

	// Empty content areas (accounting for indicator + space + gutter + space)
	contentWidth := halfWidth - lineNumWidth - 3
	leftContent := strings.Repeat(" ", contentWidth)
	rightContent := strings.Repeat(" ", contentWidth)

	separator := hunkSeparatorStyle.Render("│")
	// Format: indicator(space) + space + gutter + space + content
	return "  " + leftGutter + " " + leftContent + " " + separator + " " + "  " + rightGutter + " " + rightContent
}

// renderStatusBar renders the status bar at the bottom of the screen.
func (m Model) renderStatusBar() string {
	// In search mode, show search prompt
	if m.searchMode {
		return m.renderSearchPrompt()
	}

	info := m.StatusInfo()

	// Build left side: file name and file count
	var left string
	if info.TotalFiles > 0 {
		left = fmt.Sprintf(" %s", info.FileName)
		if info.TotalFiles > 1 {
			left += fmt.Sprintf(" [%d/%d]", info.CurrentFile, info.TotalFiles)
		}
	}

	// Build right side: position info and search match count
	var right string

	// Show search match info if there's an active query
	if m.searchQuery != "" {
		if len(m.matches) == 0 {
			right = "No matches "
		} else {
			right = fmt.Sprintf("%d/%d ", m.currentMatch+1, len(m.matches))
		}
	}

	// Add position info
	if info.AtEnd {
		right += "END "
	} else if info.CurrentLine == 1 && info.Percentage == 0 {
		right += "TOP "
	} else {
		right += fmt.Sprintf("%d%% ", info.Percentage)
	}

	// Calculate padding to fill the width
	leftWidth := displayWidth(left)
	rightWidth := displayWidth(right)
	padding := m.width - leftWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	statusContent := left + strings.Repeat(" ", padding) + right
	return statusStyle.Render(statusContent)
}

// renderSearchPrompt renders the status bar as a search input prompt.
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

	statusContent := left + strings.Repeat(" ", padding)
	return statusStyle.Render(statusContent)
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

// foldLevelIcon returns the icon for a given fold level.
// ○ = Folded (empty/minimal), ◐ = Normal (half), ● = Expanded (full)
func foldLevelIcon(level sidebyside.FoldLevel) string {
	switch level {
	case sidebyside.FoldFolded:
		return "○"
	case sidebyside.FoldExpanded:
		return "●"
	default: // FoldNormal
		return "◐"
	}
}

func (m Model) renderHeader(header string, foldLevel sidebyside.FoldLevel, rowIdx int, isCursorRow bool) string {
	// Apply search highlighting if there are matches
	if m.searchQuery != "" {
		header = m.applySearchHighlight(header, rowIdx, 0)
	}

	// Get fold level icon
	icon := foldLevelIcon(foldLevel)

	// Split prefix into highlightable part and icon part
	// Only "═══" gets highlighted (not the space or icon)
	equalsPrefix := "═══"
	iconPart := " " + icon + " "
	fullPrefix := equalsPrefix + iconPart

	if foldLevel == sidebyside.FoldFolded {
		// Folded header: no trailing line, just highlight equals prefix if cursor
		if isCursorRow {
			return cursorStyle.Render(equalsPrefix) + headerStyle.Render(iconPart+header)
		}
		return headerStyle.Render(fullPrefix + header)
	}

	// Normal or Expanded: include trailing line and right gutter highlight when cursor
	lineChar := "═"
	if foldLevel == sidebyside.FoldNormal {
		lineChar = "─"
	}

	// Calculate layout to match diff line structure
	halfWidth := (m.width - 3) / 2 // -3 for " │ "
	lineNumWidth := m.lineNumWidth()

	if isCursorRow {
		// Build header with highlighted equals prefix and right gutter
		styledEqualsPrefix := cursorStyle.Render(equalsPrefix)

		// Left portion needs to fill exactly halfWidth display columns
		equalsPrefixWidth := displayWidth(equalsPrefix)
		iconPartWidth := displayWidth(iconPart)
		headerTextWidth := displayWidth(header)
		suffix := " "
		suffixWidth := displayWidth(suffix)

		contentWidth := equalsPrefixWidth + iconPartWidth + headerTextWidth + suffixWidth
		trailingBeforeSep := halfWidth - contentWidth
		if trailingBeforeSep < 0 {
			trailingBeforeSep = 0
		}

		// Right side: space + gutter (highlighted) + space + trailing
		rightGutter := cursorStyle.Render(strings.Repeat(" ", lineNumWidth))
		// Right half also has halfWidth columns: gutter(4) + space(1) + content
		trailingAfterGutter := halfWidth - lineNumWidth - 1
		if trailingAfterGutter < 0 {
			trailingAfterGutter = 0
		}

		return styledEqualsPrefix + headerStyle.Render(iconPart+header+suffix+strings.Repeat(lineChar, trailingBeforeSep)) +
			" " + hunkSeparatorStyle.Render("│") + " " + rightGutter + " " +
			headerStyle.Render(strings.Repeat(lineChar, trailingAfterGutter))
	}

	// Normal rendering: full width trailing line
	suffix := " "
	headerWidth := displayWidth(fullPrefix) + displayWidth(header) + displayWidth(suffix)
	remaining := m.width - headerWidth
	if remaining < 0 {
		remaining = 0
	}
	line := strings.Repeat(lineChar, remaining)

	return headerStyle.Render(fullPrefix + header + suffix + line)
}

func (m Model) renderLinePair(pair sidebyside.LinePair, fileIndex, halfWidth, lineNumWidth, rowIdx int, isCursorRow bool) string {
	contentWidth := halfWidth - lineNumWidth - 3 // -3 for indicator, space after indicator, and space after line num

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

	left := m.renderLineWithSpans(pair.Left, contentWidth, lineNumWidth, leftSpans, leftSyntax, rowIdx, 0, isCursorRow)
	right := m.renderLineWithSpans(pair.Right, contentWidth, lineNumWidth, rightSpans, rightSyntax, rowIdx, 1, isCursorRow)

	separator := hunkSeparatorStyle.Render("│")
	return left + " " + separator + " " + right
}

func (m Model) renderLineWithSpans(line sidebyside.Line, contentWidth, lineNumWidth int, inlineSpans []inlinediff.Span, syntaxSpans []highlight.Span, rowIdx, side int, isCursorRow bool) string {
	// Diff indicator (+/-/space) before line number
	var indicator string
	switch line.Type {
	case sidebyside.Added:
		indicator = addedStyle.Render("+")
	case sidebyside.Removed:
		indicator = removedStyle.Render("-")
	default:
		indicator = " "
	}

	// Line number (fixed, not affected by horizontal scroll)
	// Color matches the +/- indicator: green for added, red for removed, dim for context
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
			numStr = addedStyle.Render(numContent)
		case sidebyside.Removed:
			numStr = removedStyle.Render(numContent)
		default:
			numStr = lineNumStyle.Render(numContent)
		}
	}

	// Content - expand tabs
	expanded := expandTabs(line.Content)

	// Apply horizontal scroll to get visible portion
	visible := horizontalSlice(expanded, m.hscroll, contentWidth)

	// Apply styling with layers: syntax (base) -> inline diff -> search (top)
	var styledContent string
	if len(inlineSpans) > 0 && (line.Type == sidebyside.Added || line.Type == sidebyside.Removed) {
		// Apply inline diff highlighting (with search highlighting taking precedence)
		styledContent = m.applyInlineSpans(expanded, visible, inlineSpans, line.Type, contentWidth, rowIdx, side)
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

	return indicator + " " + numStr + " " + styledContent
}

// applyInlineSpans applies inline diff highlighting to visible content.
// It maps spans from the full expanded string to the visible viewport slice.
// Search highlighting takes precedence over inline diff highlighting.
func (m Model) applyInlineSpans(expanded, visible string, spans []inlinediff.Span, lineType sidebyside.LineType, _ int, rowIdx, side int) string {
	// Determine base and highlight styles
	// Base style is context (no color) since gutter shows +/- indicators
	var highlightStyle lipgloss.Style
	baseStyle := contextStyle
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
