package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/inlinediff"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// renderTruncationIndicator renders a row indicating content was truncated.
// Shows truncation on left side if truncateOld is true, right side if truncateNew is true.
func (m Model) renderTruncationIndicator(message string, isCursorRow bool, truncateOld, truncateNew bool) string {
	lineNumWidth := m.lineNumWidth()
	halfWidth := m.width / 2

	// Calculate content width (same as renderLinePair)
	contentWidth := halfWidth - lineNumWidth - 3 // -3 for indicator, space after indicator, and space after line num
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Max 3 dots, right-aligned in gutter
	numDots := 3
	if numDots > lineNumWidth {
		numDots = lineNumWidth
	}
	padding := strings.Repeat(" ", lineNumWidth-numDots)
	dots := strings.Repeat("·", numDots)
	blankGutter := strings.Repeat(" ", lineNumWidth)

	// Style with fg=13 (magenta)
	truncStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))

	// Build left side
	var left string
	if truncateOld {
		// Truncate message to fit content width
		msgText := message
		if len(msgText) > contentWidth-1 {
			msgText = msgText[:contentWidth-1]
		}
		msgPadding := strings.Repeat(" ", contentWidth-len(msgText))

		if isCursorRow && m.focused {
			left = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(padding+dots) + " " + truncStyle.Render(msgText) + msgPadding
		} else if isCursorRow && !m.focused {
			left = unfocusedCursorArrowStyle.Render("▷") + " " + padding + truncStyle.Render(dots) + " " + truncStyle.Render(msgText) + msgPadding
		} else {
			left = "  " + padding + truncStyle.Render(dots) + " " + truncStyle.Render(msgText) + msgPadding
		}
	} else {
		// Blank left side
		blankContent := strings.Repeat(" ", contentWidth)
		if isCursorRow && m.focused {
			left = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(blankGutter) + " " + blankContent
		} else if isCursorRow && !m.focused {
			left = unfocusedCursorArrowStyle.Render("▷") + " " + blankGutter + " " + blankContent
		} else {
			left = "  " + blankGutter + " " + blankContent
		}
	}

	// Build right side
	var right string
	if truncateNew {
		// Truncate message to fit content width
		msgText := message
		if len(msgText) > contentWidth-1 {
			msgText = msgText[:contentWidth-1]
		}
		msgPadding := strings.Repeat(" ", contentWidth-len(msgText))

		if isCursorRow && m.focused {
			right = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(padding+dots) + " " + truncStyle.Render(msgText) + msgPadding
		} else if isCursorRow && !m.focused {
			right = unfocusedCursorArrowStyle.Render("▷") + " " + padding + truncStyle.Render(dots) + " " + truncStyle.Render(msgText) + msgPadding
		} else {
			right = "  " + padding + truncStyle.Render(dots) + " " + truncStyle.Render(msgText) + msgPadding
		}
	} else {
		// Blank right side
		blankContent := strings.Repeat(" ", contentWidth)
		if isCursorRow && m.focused {
			right = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(blankGutter) + " " + blankContent
		} else if isCursorRow && !m.focused {
			right = unfocusedCursorArrowStyle.Render("▷") + " " + blankGutter + " " + blankContent
		} else {
			right = "  " + blankGutter + " " + blankContent
		}
	}

	separator := hunkSeparatorStyle.Render("│")
	return left + " " + separator + " " + right
}

// renderBinaryIndicator renders a row indicating a binary file.
// Shows message on left side if binaryOld is true, right side if binaryNew is true.
// Uses the same visual style as truncation indicator (fg=13, dots in gutter).
func (m Model) renderBinaryIndicator(message string, isCursorRow bool, binaryOld, binaryNew bool) string {
	lineNumWidth := m.lineNumWidth()
	halfWidth := m.width / 2

	// Calculate content width (same as renderLinePair)
	contentWidth := halfWidth - lineNumWidth - 3 // -3 for indicator, space after indicator, and space after line num
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Max 3 dots, right-aligned in gutter
	numDots := 3
	if numDots > lineNumWidth {
		numDots = lineNumWidth
	}
	padding := strings.Repeat(" ", lineNumWidth-numDots)
	dots := strings.Repeat("·", numDots)
	blankGutter := strings.Repeat(" ", lineNumWidth)

	// Style with fg=13 (magenta) - same as truncation indicator
	binaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))

	// Build left side
	var left string
	if binaryOld {
		// Truncate message to fit content width
		msgText := message
		if len(msgText) > contentWidth-1 {
			msgText = msgText[:contentWidth-1]
		}
		msgPadding := strings.Repeat(" ", contentWidth-len(msgText))

		if isCursorRow && m.focused {
			left = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(padding+dots) + " " + binaryStyle.Render(msgText) + msgPadding
		} else if isCursorRow && !m.focused {
			left = unfocusedCursorArrowStyle.Render("▷") + " " + padding + binaryStyle.Render(dots) + " " + binaryStyle.Render(msgText) + msgPadding
		} else {
			left = "  " + padding + binaryStyle.Render(dots) + " " + binaryStyle.Render(msgText) + msgPadding
		}
	} else {
		// Blank left side
		blankContent := strings.Repeat(" ", contentWidth)
		if isCursorRow && m.focused {
			left = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(blankGutter) + " " + blankContent
		} else if isCursorRow && !m.focused {
			left = unfocusedCursorArrowStyle.Render("▷") + " " + blankGutter + " " + blankContent
		} else {
			left = "  " + blankGutter + " " + blankContent
		}
	}

	// Build right side
	var right string
	if binaryNew {
		// Truncate message to fit content width
		msgText := message
		if len(msgText) > contentWidth-1 {
			msgText = msgText[:contentWidth-1]
		}
		msgPadding := strings.Repeat(" ", contentWidth-len(msgText))

		if isCursorRow && m.focused {
			right = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(padding+dots) + " " + binaryStyle.Render(msgText) + msgPadding
		} else if isCursorRow && !m.focused {
			right = unfocusedCursorArrowStyle.Render("▷") + " " + padding + binaryStyle.Render(dots) + " " + binaryStyle.Render(msgText) + msgPadding
		} else {
			right = "  " + padding + binaryStyle.Render(dots) + " " + binaryStyle.Render(msgText) + msgPadding
		}
	} else {
		// Blank right side
		blankContent := strings.Repeat(" ", contentWidth)
		if isCursorRow && m.focused {
			right = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(blankGutter) + " " + blankContent
		} else if isCursorRow && !m.focused {
			right = unfocusedCursorArrowStyle.Render("▷") + " " + blankGutter + " " + blankContent
		} else {
			right = "  " + blankGutter + " " + blankContent
		}
	}

	separator := hunkSeparatorStyle.Render("│")
	return left + " " + separator + " " + right
}

func (m Model) renderCommentRow(row displayRow, leftHalfWidth, rightHalfWidth, lineNumWidth int, isCursorRow bool) string {
	// Tree prefix using tight spacing (same as content rows)
	treeContinuation := renderTreePrefixTight(row.treePath)
	currentTreeWidth := treeWidthTight(len(row.treePath.Ancestors))

	// Gutter: arrow(1) + space(1) + lineNum area
	gutterWidth := 2 + lineNumWidth

	// Box spans from after gutter and tree prefix to the left half width
	boxWidth := leftHalfWidth - gutterWidth - currentTreeWidth
	if boxWidth < 6 {
		boxWidth = 6
	}

	// Content width inside the box (minus borders and padding)
	contentWidth := boxWidth - 4 // 4 = │ + space + space + │
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Build left gutter with cursor indicator if applicable
	var leftGutter string
	if isCursorRow && m.focused {
		leftGutter = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(strings.Repeat(" ", lineNumWidth))
	} else if isCursorRow && !m.focused {
		leftGutter = unfocusedCursorArrowStyle.Render("▷") + " " + strings.Repeat(" ", lineNumWidth)
	} else {
		leftGutter = strings.Repeat(" ", gutterWidth)
	}

	// Build right gutter with cursor indicator if applicable
	var rightGutter string
	if isCursorRow && m.focused {
		rightGutter = cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(strings.Repeat(" ", lineNumWidth))
	} else if isCursorRow && !m.focused {
		rightGutter = unfocusedCursorArrowStyle.Render("▷") + " " + strings.Repeat(" ", lineNumWidth)
	} else {
		rightGutter = strings.Repeat(" ", gutterWidth)
	}

	sep := centerDividerStyle.Render(" │ ")

	// Right side content area (dim shading)
	rightContentWidth := rightHalfWidth - gutterWidth
	if rightContentWidth < 0 {
		rightContentWidth = 0
	}
	rightContent := commentRightDimStyle.Render(strings.Repeat("░", rightContentWidth))

	// Determine which part of the comment box this row is
	isTopBorder := row.commentRowIndex == 0
	isBottomBorder := row.commentRowIndex == row.commentRowCount-1

	if isTopBorder {
		topBorder := "╓" + strings.Repeat("─", boxWidth-2) + "╖"
		return treeContinuation + leftGutter + commentBorderStyle.Render(topBorder) + sep + rightGutter + rightContent
	}

	if isBottomBorder {
		bottomBorder := "╙" + strings.Repeat("─", boxWidth-2) + "╜"
		return treeContinuation + leftGutter + commentBorderStyle.Render(bottomBorder) + sep + rightGutter + rightContent
	}

	// Content line - wrap the comment text the same way buildCommentRows does,
	// then index into the wrapped lines.
	var wrappedLines []string
	for _, para := range strings.Split(row.commentText, "\n") {
		wrappedLines = append(wrappedLines, wrapComment(para, contentWidth)...)
	}
	lineIdx := row.commentLineIndex
	var lineText string
	if lineIdx >= 0 && lineIdx < len(wrappedLines) {
		lineText = wrappedLines[lineIdx]
	}

	// Apply search highlighting to the comment text
	// Comments are always on side 0 (new/left side)
	highlightedText := m.highlightSearchInVisible(lineText, isCursorRow, m.searchMatchIdx, 0, m.searchMatchSide)

	lineWidth := displayWidth(lineText)
	padding := contentWidth - lineWidth
	if padding < 0 {
		padding = 0
	}

	// Build the content with highlighting and padding
	// Note: padding must be added after highlighting to maintain box alignment
	paddedText := highlightedText + strings.Repeat(" ", padding)

	return treeContinuation + leftGutter + commentBorderStyle.Render("║ ") + paddedText + " " + commentBorderStyle.Render("║") + sep + rightGutter + rightContent
}

// wrapComment wraps a single line of text to fit within maxWidth using
// ansi.Wordwrap from the charmbracelet/x library.
func wrapComment(line string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{line}
	}
	wrapped := ansi.Wordwrap(line, maxWidth, "")
	return strings.Split(wrapped, "\n")
}

func (m Model) renderLinePair(pair sidebyside.LinePair, fileIndex, leftHalfWidth, rightHalfWidth, lineNumWidth, rowIdx int, isCursorRow bool, isFirstLine, isLastLine, hideRightTrailingGutter bool, treePath TreePath) string {
	// Tree prefix using tight spacing for compact content indentation
	treeContinuation := renderTreePrefixTight(treePath)
	currentTreeWidth := treeWidthTight(len(treePath.Ancestors))

	leftContentWidth := leftHalfWidth - lineNumWidth - 3 - currentTreeWidth // -3 for indicator, space after indicator, space after line num
	rightContentWidth := rightHalfWidth - lineNumWidth - 3                  // same layout on right side

	// Vertical divider between left and right sides
	separatorChar := "┃"

	// Check if this is a modified pair where we should show inline diff
	isModifiedPair := pair.Old.Type == sidebyside.Removed && pair.New.Type == sidebyside.Added

	var oldSpans, newSpans []inlinediff.Span
	if isModifiedPair {
		oldSpans, newSpans = m.getInlineDiff(fileIndex, pair)
	}

	// Get syntax highlight spans for each side
	// New content on left (side 0), Old content on right (side 1)
	newSyntax := m.getLineSpans(fileIndex, pair.New.Num, false)
	oldSyntax := m.getLineSpans(fileIndex, pair.Old.Num, true)

	// Use blue "changed" styling when we have word-level diff (both sides modified)
	hasWordDiff := len(oldSpans) > 0

	// Render: New on left (side 0), Old on right (side 1)
	left := m.renderLineWithSpans(pair.New, leftContentWidth, lineNumWidth, newSpans, newSyntax, 0, isCursorRow, hasWordDiff, false)
	right := m.renderLineWithSpans(pair.Old, rightContentWidth, lineNumWidth, oldSpans, oldSyntax, 1, isCursorRow, hasWordDiff, hideRightTrailingGutter)

	separator := centerDividerStyle.Render(separatorChar)
	return treeContinuation + left + " " + separator + " " + right
}

func (m Model) renderLineWithSpans(line sidebyside.Line, contentWidth, lineNumWidth int, inlineSpans []inlinediff.Span, syntaxSpans []highlight.Span, side int, isCursorRow bool, hasWordDiff bool, hideTrailingGutter bool) string {
	// Diff indicator (+/-/~/space) before line number
	// On cursor row, show arrowhead instead (outline arrow when unfocused)
	// When hasWordDiff is true, use blue "~" instead of green/red +/-
	var indicator string
	if isCursorRow && m.focused {
		indicator = cursorArrowStyle.Render("▶")
	} else if isCursorRow && !m.focused {
		indicator = unfocusedCursorArrowStyle.Render("▷")
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
	// When focused and on cursor row, highlight with cursor background
	// When unfocused, show normal colors (no background highlight)
	var numStr string
	numContent := fmt.Sprintf("%*d", lineNumWidth, line.Num)
	if line.Num == 0 {
		numContent = strings.Repeat(" ", lineNumWidth)
	}
	if isCursorRow && m.focused {
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

	// Reduce content width to make room for gutter columns
	// Layout: [gutter char] + space + content + space + [gutter char] (4 chars total)
	// When hideTrailingGutter is true, only left gutter: [gutter char] + space (2 chars)
	// Added/removed lines get ░, context/empty lines get spaces
	gutterWidth := 4
	if hideTrailingGutter {
		gutterWidth = 2
	}
	actualContentWidth := contentWidth - gutterWidth

	// Apply horizontal scroll to get visible portion
	visible := horizontalSlice(expanded, m.hscroll, actualContentWidth)

	// Apply styling with layers: syntax (base) -> inline diff -> search (top)
	// Exception: context lines on old side are dimmed (no syntax highlighting)
	// Old side is now on the right (side == 1)
	var styledContent string
	isOldSideContext := side == 1 && line.Type == sidebyside.Context

	// Determine if this side should be searched
	// New side (0): always searchable
	// Old side (1): only searchable for removed lines (- and ~ lines)
	shouldSearch := side == 0 || line.Type == sidebyside.Removed

	if isOldSideContext {
		// Dim context lines on the old side - they're duplicates of the new side
		// Don't search these (shouldSearch will be false for old side context)
		styledContent = contextDimStyle.Render(visible)
	} else if len(inlineSpans) > 0 && (line.Type == sidebyside.Added || line.Type == sidebyside.Removed) {
		// Apply inline diff highlighting with syntax as base layer, search on top
		styledContent = m.applyInlineSpans(line.Content, expanded, visible, inlineSpans, syntaxSpans, line.Type, isCursorRow, shouldSearch, m.currentMatchIdx(), side, m.currentMatchSide())
	} else if len(syntaxSpans) > 0 {
		// Apply syntax highlighting as base, with search on top
		styledContent = m.applySyntaxHighlight(line.Content, expanded, visible, syntaxSpans, isCursorRow, shouldSearch, m.currentMatchIdx(), side, m.currentMatchSide())
	} else {
		// Apply search highlighting first if applicable
		displayContent := visible
		if m.searchQuery != "" && shouldSearch {
			displayContent = m.highlightSearchInVisible(visible, isCursorRow, m.currentMatchIdx(), side, m.currentMatchSide())
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
	styledContent = m.applyColumnIndicators(styledContent, line.Type, hasWordDiff, hideTrailingGutter)

	// Style truncation indicator with fg=13 if present
	if strings.Contains(visible, diff.LineTruncationText) {
		truncStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
		styledContent = strings.ReplaceAll(styledContent, diff.LineTruncationText, truncStyle.Render(diff.LineTruncationText))
	}

	return indicator + " " + numStr + " " + styledContent
}

// applyInlineSpans applies inline diff highlighting to visible content.
// It maps spans from the full expanded string to the visible viewport slice.
// Search highlighting takes precedence over inline diff highlighting.
// isCursorRow indicates if this is the cursor row (for "current match" styling).
// shouldSearch indicates if search highlighting should be applied to this content.
// side is which side is being rendered, currentSide is which side has the current match.
func (m Model) applyInlineSpans(original, expanded, visible string, spans []inlinediff.Span, syntaxSpans []highlight.Span, lineType sidebyside.LineType, isCursorRow, shouldSearch bool, currentIdx, side, currentSide int) string {
	// Highlight style matches the line type (green for added, red for removed)
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

	// Map byte positions from ORIGINAL content to display columns for syntax spans
	// Syntax spans have offsets into the original (non-tab-expanded) content
	syntaxByteToCol := make([]int, len(original)+1)
	col = 0
	bytePos = 0
	for _, r := range original {
		syntaxByteToCol[bytePos] = col
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
	syntaxByteToCol[len(original)] = col

	// Build search match ranges (in visible coordinates) if search is active
	type searchRange struct {
		start, end int
		isCurrent  bool
	}
	var searchRanges []searchRange
	if m.searchQuery != "" && shouldSearch {
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
		matchIdx := 0
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

			// Determine if this is the current match by index and side
			isCurrent := isCursorRow && matchIdx == currentIdx && side == currentSide

			searchRanges = append(searchRanges, searchRange{start: start, end: end, isCurrent: isCurrent})
			pos = start + 1
			matchIdx++
		}
	}

	// Get theme for syntax coloring (used as base layer for non-diff chars)
	var theme highlight.Theme
	hasTheme := m.highlighter != nil
	if hasTheme {
		theme = m.highlighter.Theme()
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
				// Use syntax highlighting as base layer
				foundStyle := false
				if hasTheme {
					for _, span := range syntaxSpans {
						spanStartCol := 0
						spanEndCol := 0
						if span.Start < len(syntaxByteToCol) {
							spanStartCol = syntaxByteToCol[span.Start]
						}
						if span.End < len(syntaxByteToCol) {
							spanEndCol = syntaxByteToCol[span.End]
						} else if span.End >= len(syntaxByteToCol) {
							spanEndCol = syntaxByteToCol[len(syntaxByteToCol)-1]
						}

						if actualCol >= spanStartCol && actualCol < spanEndCol {
							style := theme.Style(span.Category)
							result.WriteString(style.Render(string(vr)))
							foundStyle = true
							break
						}
					}
				}
				if !foundStyle {
					result.WriteString(string(vr))
				}
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
// isCursorRow indicates if this is the cursor row (for "current match" styling).
// shouldSearch indicates if search highlighting should be applied to this content.
// side is which side is being rendered, currentSide is which side has the current match.
func (m Model) applySyntaxHighlight(original, _, visible string, syntaxSpans []highlight.Span, isCursorRow, shouldSearch bool, currentIdx, side, currentSide int) string {
	if len(syntaxSpans) == 0 {
		// No syntax spans, just apply search if applicable
		if m.searchQuery != "" && shouldSearch {
			return m.highlightSearchInVisible(visible, isCursorRow, m.currentMatchIdx(), side, currentSide)
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

	// Build search match ranges (in visible coordinates) if search is active
	type searchRange struct {
		start, end int
		isCurrent  bool
	}
	var searchRanges []searchRange
	if m.searchQuery != "" && shouldSearch {
		queryLen := len(m.searchQuery)
		caseSensitive := isSmartCaseSensitive(m.searchQuery)
		query := m.searchQuery
		searchIn := visible
		if !caseSensitive {
			searchIn = strings.ToLower(visible)
			query = strings.ToLower(query)
		}

		pos := 0
		matchIdx := 0
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

			// Determine if this is the current match by index and side
			isCurrent := isCursorRow && matchIdx == currentIdx && side == currentSide

			searchRanges = append(searchRanges, searchRange{start: start, end: end, isCurrent: isCurrent})
			pos = start + 1
			matchIdx++
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

func (m Model) applyColumnIndicators(styledContent string, lineType sidebyside.LineType, hasWordDiff bool, hideTrailingGutter bool) string {
	isAddedOrRemoved := lineType == sidebyside.Added || lineType == sidebyside.Removed

	// For context/empty lines, just wrap with spaces to align with added/removed
	if !isAddedOrRemoved {
		if hideTrailingGutter {
			return "  " + styledContent
		}
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

	if hideTrailingGutter {
		return startIndicator + " " + styledContent
	}

	endIndicator := colorStyle.Render("░")
	return startIndicator + " " + styledContent + " " + endIndicator
}

// getInlineDiff returns cached inline diff spans for a modified line pair,
// computing and caching them if not already cached.
// Returns (oldSpans, newSpans) for the old and new content respectively.
func (m Model) getInlineDiff(fileIndex int, pair sidebyside.LinePair) ([]inlinediff.Span, []inlinediff.Span) {
	cacheKey := inlineDiffKey{fileIndex: fileIndex, oldNum: pair.Old.Num, newNum: pair.New.Num}

	// Check cache first (if cache exists)
	if m.inlineDiffCache != nil {
		if cached, ok := m.inlineDiffCache[cacheKey]; ok {
			return cached.oldSpans, cached.newSpans
		}
	}

	// Compute inline diff
	var oldSpans, newSpans []inlinediff.Span
	oldContent := expandTabs(pair.Old.Content)
	newContent := expandTabs(pair.New.Content)

	// Only do inline diff if lines are similar enough
	if !inlinediff.ShouldSkipInlineDiff(oldContent, newContent) {
		oldSpans, newSpans = inlinediff.Diff(oldContent, newContent)

		// Also skip if too much would be highlighted (not useful)
		if inlinediff.ShouldSkipBasedOnSpans(oldSpans, len(oldContent)) ||
			inlinediff.ShouldSkipBasedOnSpans(newSpans, len(newContent)) {
			oldSpans, newSpans = nil, nil
		}
	}

	// Cache the result (even if nil - means we computed and found nothing useful)
	if m.inlineDiffCache != nil {
		m.inlineDiffCache[cacheKey] = inlineDiffResult{oldSpans: oldSpans, newSpans: newSpans}
	}

	return oldSpans, newSpans
}
