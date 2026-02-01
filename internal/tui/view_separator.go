package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/sidebyside"
)

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
		breadcrumb = formatBreadcrumbs(entries, leftContentWidth)
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
