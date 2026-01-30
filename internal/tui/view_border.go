package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderNodeBorder renders a top or bottom border for a tree node (file header, commit-info, etc).
// This is the shared implementation used by file headers and commit-info borders.
// Format: [adjustedPrefix]┌───────────┐ or └───────────┘
// The prefix is shifted left by 2 so the corner aligns with │ on the header line.
func (m Model) renderNodeBorder(headerBoxWidth int, treePrefixWidth int, style lipgloss.Style, isTop bool, isCursorRow bool, treePath TreePath) string {
	innerWidth := headerBoxWidth
	if innerWidth < 0 {
		innerWidth = 0
	}

	// Build tree continuation from ancestors (│ if more siblings, space otherwise)
	var treeCont string
	var treeContWidth int
	if len(treePath.Ancestors) > 0 {
		level := treePath.Ancestors[0]
		if !level.IsLast && !level.IsFolded {
			treeCont = treeContinuationStyle.Render("│")
		} else {
			treeCont = " "
		}
		treeContWidth = 1
	}

	// Tree prefix shifted left by 2 to align corner with │ on header
	adjustedPrefixWidth := treePrefixWidth - 2
	if adjustedPrefixWidth < 0 {
		adjustedPrefixWidth = 0
	}

	// Calculate spacing: margin + continuation + spaces = adjustedPrefixWidth
	spacesBeforeCorner := adjustedPrefixWidth - TreeLeftMargin - treeContWidth
	if spacesBeforeCorner < 0 {
		spacesBeforeCorner = 0
	}

	margin := strings.Repeat(" ", TreeLeftMargin)
	spacing := strings.Repeat(" ", spacesBeforeCorner)

	// Border width to align right corner with │ on header's right side
	borderWidth := headerBoxWidth - adjustedPrefixWidth
	if borderWidth < 2 {
		borderWidth = 2
	}
	innerBorderWidth := borderWidth - 2 // minus corners
	if innerBorderWidth < 0 {
		innerBorderWidth = 0
	}

	// Choose corner characters
	leftCorner := "┌"
	rightCorner := "┐"
	if !isTop {
		leftCorner = "└"
		rightCorner = "┘"
	}

	if isCursorRow {
		var styledGutter, arrow string
		if m.focused {
			styledGutter = cursorStyle.Render("─")
			arrow = cursorArrowStyle.Render("▶")
		} else {
			styledGutter = style.Render("─")
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		// Replace first char of margin with arrow
		if TreeLeftMargin > 0 {
			return arrow + margin[1:] + treeCont + spacing + style.Render(leftCorner) + styledGutter + style.Render(strings.Repeat("─", innerBorderWidth)+rightCorner)
		}
		return arrow + treeCont + spacing + style.Render(leftCorner) + styledGutter + style.Render(strings.Repeat("─", innerBorderWidth)+rightCorner)
	}

	// Normal: margin + continuation + spacing + border
	return margin + treeCont + spacing + style.Render(leftCorner+strings.Repeat("─", innerBorderWidth)+rightCorner)
}

// renderHeaderTopBorder renders the top border row above the file header.
// Renders as empty space (keeping the row for layout consistency).
func (m Model) renderHeaderTopBorder(headerBoxWidth int, headerMode HeaderMode, status FileStatus, isCursorRow bool, treePrefixWidth int, treePath TreePath) string {
	_, _ = status, headerBoxWidth // not used
	return renderEmptyTreeRow(treePath, isCursorRow, m.focused, false)
}

// renderHeaderBottomBorder renders the bottom border of the file header.
// Renders as an underline starting at the fold icon position with a corner character.
func (m Model) renderHeaderBottomBorder(headerBoxWidth int, headerMode HeaderMode, status FileStatus, isCursorRow bool, treePrefixWidth int, treePath TreePath) string {
	// Use file status color for border (matches branch color)
	var style lipgloss.Style
	if headerMode != HeaderThreeLine {
		// Use darker color when border should not be visible
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	} else {
		// Use status-based color
		_, style = fileStatusIndicator(status)
	}

	// Build tree continuation from ancestors
	var treeCont string
	if len(treePath.Ancestors) > 0 {
		level := treePath.Ancestors[0]
		if !level.IsLast && !level.IsFolded {
			treeCont = treeContinuationStyle.Render("│")
		} else {
			treeCont = " "
		}
	}

	margin := strings.Repeat(" ", TreeLeftMargin)

	// Calculate spacing to position corner at fold icon column
	// Header layout: margin(1) + branch(4) + space(1) + fold_icon
	// treePrefixWidth = margin + branch + 1 = 6
	// Corner should be at position treePrefixWidth (same column as fold icon)
	// With continuation: margin(1) + cont(1) + spaces + corner at treePrefixWidth
	// Without continuation: margin(1) + spaces + corner at treePrefixWidth
	var spacesBeforeCorner int
	if len(treePath.Ancestors) > 0 {
		spacesBeforeCorner = treePrefixWidth - TreeLeftMargin - 1 // -1 for continuation char
	} else {
		spacesBeforeCorner = treePrefixWidth - TreeLeftMargin
	}
	if spacesBeforeCorner < 0 {
		spacesBeforeCorner = 0
	}
	spacing := strings.Repeat(" ", spacesBeforeCorner)

	// Border width: from corner to end of header content
	// headerBoxWidth includes the full header width from tree prefix to right edge
	borderWidth := headerBoxWidth - treePrefixWidth + 2
	if borderWidth < 1 {
		borderWidth = 1
	}

	// Use heavy box-drawing characters for underline: ┗ corner and ━ horizontal
	corner := "┗"
	borderLine := strings.Repeat("━", borderWidth) + "┛"

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▶")
		} else {
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		if TreeLeftMargin > 0 {
			return arrow + margin[1:] + treeCont + spacing + style.Render(corner+borderLine)
		}
		return arrow + treeCont + spacing + style.Render(corner+borderLine)
	}

	return margin + treeCont + spacing + style.Render(corner+borderLine)
}

// renderCommitBorderLine renders a commit-level horizontal border line.
// Used for commit header top/bottom borders, including the first commit's margin border.
// The border spans contentWidth if provided (>0), otherwise full width minus margin.
// Uses yellow (commitTreeStyle) when visible.
// isTop determines the tree connector: currently top border renders empty, bottom uses ╞.
func (m Model) renderCommitBorderLine(visible bool, isTop bool, contentWidth int, isCursorRow bool, treePath TreePath) string {
	// Top border: render as empty line with tree continuation
	if isTop {
		return renderEmptyTreeRow(treePath, isCursorRow, m.focused, false)
	}

	// Use commit tree style (yellow) for border, or dark when not visible
	borderStyle := commitTreeStyle
	if !visible {
		borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	}

	// Tree connector at column 1 (after margin space) - double line style
	connector := "╞"
	borderChar := "═"

	// Border width: use contentWidth if provided, otherwise full width
	// Subtract 1 so ╝ aligns with ╔ on the header line
	// (border = margin(1) + ╞(1) + ═*(borderWidth-1) + ╝(1), ╝ at column borderWidth+1)
	borderWidth := contentWidth - 1
	if contentWidth <= 0 {
		borderWidth = m.width - 2
	}
	if borderWidth < 1 {
		borderWidth = 1
	}

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▶")
		} else {
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		// Arrow replaces margin, then connector (grey) + border + closing corner
		return arrow + treeContinuationStyle.Render(connector) + borderStyle.Render(strings.Repeat(borderChar, borderWidth-1)+"╝")
	}

	// Margin space + connector (grey) + border + closing corner
	return " " + treeContinuationStyle.Render(connector) + borderStyle.Render(strings.Repeat(borderChar, borderWidth-1)+"╝")
}

// renderCommitHeaderTopBorder renders the top border of the commit header.
func (m Model) renderCommitHeaderTopBorder(row displayRow, isCursorRow bool) string {
	return m.renderCommitBorderLine(row.headerMode == HeaderThreeLine, true, 0, isCursorRow, row.treePath)
}

// renderCommitHeaderBottomBorder renders the bottom border of the commit header.
// The border width matches the commit header content width.
func (m Model) renderCommitHeaderBottomBorder(row displayRow, isCursorRow bool) string {
	return m.renderCommitBorderLine(row.headerMode == HeaderThreeLine, false, row.headerBoxWidth, isCursorRow, row.treePath)
}
