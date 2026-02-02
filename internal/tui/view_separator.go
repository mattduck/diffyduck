package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

// visibleLineRanges finds the contiguous new-side line ranges of the hunks
// immediately above and below a separator at chunkStartLine.
func visibleLineRanges(pairs []sidebyside.LinePair, chunkStartLine int) (aboveStart, aboveEnd, belowStart, belowEnd int) {
	// Find the pair index where the hunk below starts
	belowIdx := -1
	for i, p := range pairs {
		if p.New.Num == chunkStartLine {
			belowIdx = i
			break
		}
	}

	if belowIdx < 0 {
		return
	}

	// Hunk above: scan backward from belowIdx to find contiguous lines
	for i := belowIdx - 1; i >= 0; i-- {
		if pairs[i].New.Num > 0 {
			if aboveEnd == 0 {
				aboveEnd = pairs[i].New.Num
			}
			// Keep scanning for start, stop at gap
			if i > 0 {
				prevNew := 0
				for j := i - 1; j >= 0; j-- {
					if pairs[j].New.Num > 0 {
						prevNew = pairs[j].New.Num
						break
					}
				}
				if prevNew > 0 && pairs[i].New.Num > prevNew+1 {
					aboveStart = pairs[i].New.Num
					break
				}
			}
			aboveStart = pairs[i].New.Num
		}
	}
	// Hunk below: scan forward to find the end of this contiguous block
	belowStart = chunkStartLine
	belowEnd = chunkStartLine
	for i := belowIdx; i < len(pairs); i++ {
		if pairs[i].New.Num > 0 {
			if pairs[i].New.Num > belowEnd+1 && i > belowIdx {
				break // hit next gap
			}
			belowEnd = pairs[i].New.Num
		}
	}
	return
}

// isEntryVisible checks if an entry's StartLine falls within the given visible ranges.
func isEntryVisible(start, aboveStart, aboveEnd, belowStart, belowEnd int) bool {
	if belowStart > 0 && start >= belowStart && start <= belowEnd {
		return true
	}
	if aboveStart > 0 && start >= aboveStart && start <= aboveEnd {
		return true
	}
	return false
}

// filterVisibleEntries removes structure entries whose StartLine is already
// visible in the pairs around the separator. This prevents showing a breadcrumb
// for a function whose signature is already on screen (e.g. first line of the hunk).
// Entries are filtered from innermost (last) outward; we stop at the first
// entry that is NOT visible, keeping it and all outer entries.
// Also returns the innermost filtered entry (if any) for use as a continuation marker.
func filterVisibleEntries(entries []structure.Entry, pairs []sidebyside.LinePair, chunkStartLine int) (kept []structure.Entry, continuation *structure.Entry) {
	if len(entries) == 0 || len(pairs) == 0 {
		return entries, nil
	}

	aboveStart, aboveEnd, belowStart, belowEnd := visibleLineRanges(pairs, chunkStartLine)

	// Filter from innermost outward: drop entries whose StartLine is visible
	cutoff := len(entries)
	for i := len(entries) - 1; i >= 0; i-- {
		if !isEntryVisible(entries[i].StartLine, aboveStart, aboveEnd, belowStart, belowEnd) {
			break
		}
		cutoff = i
	}

	if cutoff < len(entries) {
		// Continuation is the innermost filtered entry (last in the list),
		// but only if its definition is in the hunk above (scrolled past).
		// If the definition is in the hunk below, we're looking at it — no continuation needed.
		cont := entries[len(entries)-1]
		inBelow := belowStart > 0 && cont.StartLine >= belowStart && cont.StartLine <= belowEnd
		if !inBelow {
			continuation = &cont
		}
	}

	return entries[:cutoff], continuation
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

// findFirstNewLineNum finds the first non-zero New.Num starting at index start.
// Used to find the line number for breadcrumb lookup when a chunk starts with deletions.
func findFirstNewLineNum(pairs []sidebyside.LinePair, start int) int {
	for i := start; i < len(pairs); i++ {
		if pairs[i].New.Num > 0 {
			return pairs[i].New.Num
		}
		// Stop at next hunk boundary to avoid crossing into another chunk
		if i > start {
			prevOld := 0
			prevNew := 0
			if i > 0 {
				if pairs[i-1].Old.Num > 0 {
					prevOld = pairs[i-1].Old.Num
				}
				if pairs[i-1].New.Num > 0 {
					prevNew = pairs[i-1].New.Num
				}
			}
			if isHunkBoundary(prevOld, prevNew, pairs[i].Old.Num, pairs[i].New.Num) {
				break
			}
		}
	}
	return 0
}

func (m Model) renderHunkSeparator(row displayRow, leftHalfWidth, rightHalfWidth int, isCursorRow bool) string {
	shadeStyle := hunkSeparatorStyle
	lineNumWidth := m.lineNumWidth()

	// Tree prefix using tight spacing for compact content
	// Cursor arrow replaces the left margin space in the tree prefix
	treeContinuation := renderTreePrefixTightWithCursor(row.treePath, isCursorRow, m.focused)
	currentTreeWidth := treeWidthTight(len(row.treePath.Ancestors))

	// Gutter width: indicator(1) + space(1) + lineNumWidth (one less than content lines for tighter breadcrumb)
	gutterWidth := 2 + lineNumWidth

	// Content width after gutter (breadcrumb starts here, aligned with code content)
	leftContentWidth := leftHalfWidth - gutterWidth - currentTreeWidth
	if leftContentWidth < 0 {
		leftContentWidth = 0
	}

	// Try to get breadcrumb for the chunk start line (new/left side only)
	var breadcrumb string
	if row.chunkStartLine > 0 {
		entries := m.getStructureAtLine(row.fileIndex, row.chunkStartLine)
		var continuation *structure.Entry
		if row.fileIndex >= 0 && row.fileIndex < len(m.files) {
			entries, continuation = filterVisibleEntries(entries, m.files[row.fileIndex].Pairs, row.chunkStartLine)
		}
		breadcrumb = formatBreadcrumbs(entries, leftContentWidth)
		// If the innermost entry was filtered (already visible), show a continuation
		if breadcrumb == "" && continuation != nil {
			breadcrumb = " ... "
		} else if continuation != nil {
			breadcrumb += " > ... "
		}

		// If no continuation was triggered but the previous separator in this file
		// had the same innermost entry, show "..." to avoid repeating the breadcrumb.
		if continuation == nil && row.prevChunkStartLine > 0 && len(entries) > 0 {
			prevEntries := m.getStructureAtLine(row.fileIndex, row.prevChunkStartLine)
			if len(prevEntries) > 0 {
				innermost := entries[len(entries)-1]
				prevInnermost := prevEntries[len(prevEntries)-1]
				if innermost.Name == prevInnermost.Name && innermost.StartLine == prevInnermost.StartLine {
					if len(entries) > 1 {
						breadcrumb = formatBreadcrumbs(entries[:len(entries)-1], leftContentWidth) + " > ... "
					} else {
						breadcrumb = " ... "
					}
				}
			}
		}
	}
	rightContentWidth := rightHalfWidth - gutterWidth
	if rightContentWidth < 0 {
		rightContentWidth = 0
	}

	if !isCursorRow {
		// Non-cursor: tree + all shading
		leftGutter := shadeStyle.Render(strings.Repeat("░", gutterWidth))
		rightGutter := shadeStyle.Render(strings.Repeat("░", gutterWidth))

		var leftContent string
		if breadcrumb != "" && leftContentWidth > 0 {
			breadcrumb = runewidth.Truncate(breadcrumb, leftContentWidth, "")
			displayWidth := runewidth.StringWidth(breadcrumb)
			padding := leftContentWidth - displayWidth
			leftContent = shadeStyle.Render(breadcrumb + strings.Repeat("░", padding))
		} else {
			leftContent = shadeStyle.Render(strings.Repeat("░", leftContentWidth))
		}
		rightContent := shadeStyle.Render(strings.Repeat("░", rightContentWidth))

		return treeContinuation + leftGutter + leftContent + shadeStyle.Render("░░░") + rightGutter + rightContent
	}

	// Cursor row: arrow is in tree prefix, gutter shows shading with cursor bg on line num area
	// When unfocused, no background highlighting
	var leftGutter, rightGutter string
	if m.focused {
		leftGutter = shadeStyle.Render("░░") + cursorStyle.Render(strings.Repeat("░", lineNumWidth))
		rightGutter = shadeStyle.Render("░░") + cursorStyle.Render(strings.Repeat("░", lineNumWidth))
	} else {
		leftGutter = shadeStyle.Render("░░") + shadeStyle.Render(strings.Repeat("░", lineNumWidth))
		rightGutter = shadeStyle.Render("░░") + shadeStyle.Render(strings.Repeat("░", lineNumWidth))
	}

	// Left side: breadcrumb in content area
	var leftContent string
	if breadcrumb != "" && leftContentWidth > 0 {
		breadcrumb = runewidth.Truncate(breadcrumb, leftContentWidth, "")
		displayWidth := runewidth.StringWidth(breadcrumb)
		padding := leftContentWidth - displayWidth
		leftContent = shadeStyle.Render(breadcrumb + strings.Repeat("░", padding))
	} else {
		leftContent = shadeStyle.Render(strings.Repeat("░", leftContentWidth))
	}

	// Right side: all shading
	rightContent := shadeStyle.Render(strings.Repeat("░", rightContentWidth))

	return treeContinuation + leftGutter + leftContent + shadeStyle.Render("░░░") + rightGutter + rightContent
}

// renderHunkSeparatorTop renders the top line of a hunk separator (faint shader for visual separation).
func (m Model) renderHunkSeparatorTop(row displayRow, leftHalfWidth, rightHalfWidth int, isCursorRow bool) string {
	// Faint shader style - less visible than the main separator
	faintShadeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	lineNumWidth := m.lineNumWidth()
	_ = lineNumWidth // used below in cursor handling

	// Tree prefix using tight spacing for compact content
	// Cursor arrow replaces the left margin space in the tree prefix
	treeContinuation := renderTreePrefixTightWithCursor(row.treePath, isCursorRow, m.focused)
	currentTreeWidth := treeWidthTight(len(row.treePath.Ancestors))

	// Arrow column width: indicator(1) + space(1) = 2
	arrowWidth := 2

	leftContentWidth := leftHalfWidth - arrowWidth - currentTreeWidth
	if leftContentWidth < 0 {
		leftContentWidth = 0
	}
	rightContentWidth := rightHalfWidth - arrowWidth
	if rightContentWidth < 0 {
		rightContentWidth = 0
	}

	if !isCursorRow {
		// Non-cursor: tree + all faint shading
		leftArrow := faintShadeStyle.Render("░░")
		rightArrow := faintShadeStyle.Render("░░")
		leftContent := faintShadeStyle.Render(strings.Repeat("░", leftContentWidth))
		rightContent := faintShadeStyle.Render(strings.Repeat("░", rightContentWidth))
		return treeContinuation + leftArrow + leftContent + faintShadeStyle.Render("░░░") + rightArrow + rightContent
	}

	// Cursor row: arrow is in tree prefix, gutter shows faint shading with cursor bg on line num area
	// When unfocused, no background highlighting
	var leftArrow, rightArrow string
	leftArrow = faintShadeStyle.Render("░░")
	rightArrow = faintShadeStyle.Render("░░")

	// Left side: lineNumWidth chars with cursor bg (only when focused), rest faint
	var cursorPart string
	if m.focused {
		cursorPart = cursorStyle.Render(strings.Repeat("░", lineNumWidth))
	} else {
		cursorPart = faintShadeStyle.Render(strings.Repeat("░", lineNumWidth))
	}
	leftRestWidth := leftContentWidth - lineNumWidth
	var leftContent string
	if leftRestWidth > 0 {
		leftContent = cursorPart + faintShadeStyle.Render(strings.Repeat("░", leftRestWidth))
	} else {
		leftContent = cursorPart
	}

	// Right side: lineNumWidth chars with cursor bg (only when focused), rest faint
	var rightCursorPart string
	if m.focused {
		rightCursorPart = cursorStyle.Render(strings.Repeat("░", lineNumWidth))
	} else {
		rightCursorPart = faintShadeStyle.Render(strings.Repeat("░", lineNumWidth))
	}
	rightRestWidth := rightContentWidth - lineNumWidth
	var rightContent string
	if rightRestWidth > 0 {
		rightContent = rightCursorPart + faintShadeStyle.Render(strings.Repeat("░", rightRestWidth))
	} else {
		rightContent = rightCursorPart
	}

	return treeContinuation + leftArrow + leftContent + faintShadeStyle.Render("░░░") + rightArrow + rightContent
}

// renderPaginationIndicator renders an ellipsis line indicating more commits can be loaded.
func (m Model) renderPaginationIndicator(isCursorRow bool) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim
	indicator := "   …"
	if isCursorRow && m.focused {
		return cursorArrowStyle.Render("▶") + " " + style.Render("…")
	} else if isCursorRow {
		return unfocusedCursorArrowStyle.Render("▷") + " " + style.Render("…")
	}
	return style.Render(indicator)
}

// renderNodeBorder renders a top or bottom border for a tree node (file header, commit-info, etc).
// This is the shared implementation used by file headers and commit-info borders.
// Format: [adjustedPrefix]┌───────────┐ or └───────────┘
