package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/diffyduck/pkg/sidebyside"
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
			style := treeContinuationStyle
			if level.Faint {
				style = treeFaintContinuationStyle
			}
			treeCont = style.Render("│")
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
			arrow = cursorArrowStyle.Render("▌")
		} else {
			styledGutter = style.Render("─")
			arrow = " "
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
// For expanded files: corner ┗ aligned under header's ┓, extending to screen edge with ●.
// For normal files: underline starting at the fold icon position with ┗━━━┛.
func (m Model) renderHeaderBottomBorder(headerBoxWidth int, headerMode HeaderMode, status FileStatus, isCursorRow bool, treePrefixWidth int, treePath TreePath, foldLevel sidebyside.FoldLevel) string {
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
	var treeContWidth int
	if len(treePath.Ancestors) > 0 {
		level := treePath.Ancestors[0]
		if !level.IsLast && !level.IsFolded {
			contStyle := treeContinuationStyle
			if level.Faint {
				contStyle = treeFaintContinuationStyle
			}
			treeCont = contStyle.Render("│")
		} else {
			treeCont = " "
		}
		treeContWidth = 1
	}

	margin := strings.Repeat(" ", TreeLeftMargin)

	// Unfolded files: position content under the ┓ on the header line.
	// Expanded: ┗━━━━━━━━━━━━━━━━━━━━━━━●  (line to screen edge)
	// Normal:   ◐                          (just the fold icon)
	if headerMode == HeaderThreeLine && (foldLevel == sidebyside.FoldHunks || foldLevel == sidebyside.FoldStructure) {
		// Compute the same column as the header's ┓:
		// header uses treeWidth(headerAncestors, true) + headerBoxWidth - 2 + 4
		// Content rows have one extra ancestor (the file level itself).
		headerNumAncestors := len(treePath.Ancestors) - 1
		if headerNumAncestors < 0 {
			headerNumAncestors = 0
		}
		cornerColumn := treeWidth(headerNumAncestors, true) + headerBoxWidth - 2 + 4 // +4 for ━━━━ before ┓

		spacesBeforeCorner := cornerColumn - TreeLeftMargin - treeContWidth
		if spacesBeforeCorner < 0 {
			spacesBeforeCorner = 0
		}
		spacing := strings.Repeat(" ", spacesBeforeCorner)

		var content string
		if foldLevel == sidebyside.FoldHunks {
			corner := "┗"
			borderFill := m.width - cornerColumn - 1 // -1 for corner char
			if borderFill > 1 {
				content = corner + strings.Repeat("━", borderFill-1) + "●"
			} else if borderFill > 0 {
				content = corner + "●"
			} else {
				content = corner
			}
		} else {
			// FoldStructure: corner turning right with fold icon
			content = "┗━◐"
		}

		if isCursorRow {
			var arrow string
			if m.focused {
				arrow = cursorArrowStyle.Render("▌")
			} else {
				arrow = " "
			}
			if TreeLeftMargin > 0 {
				return arrow + margin[1:] + treeCont + spacing + style.Render(content)
			}
			return arrow + treeCont + spacing + style.Render(content)
		}

		return margin + treeCont + spacing + style.Render(content)
	}

	// Normal/folded files: underline under header content
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
	borderWidth := headerBoxWidth - treePrefixWidth + 2
	if borderWidth < 1 {
		borderWidth = 1
	}

	corner := "┗"
	borderLine := strings.Repeat("━", borderWidth) + "┛"

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▌")
		} else {
			arrow = " "
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
// The bottom border extends full-width to the screen edge.
// Uses yellow (commitTreeStyle) when visible, magenta (snapshotTreeStyle) for snapshots.
// isTop determines the tree connector: currently top border renders empty, bottom uses ╞.
func (m Model) renderCommitBorderLine(visible bool, isTop bool, isCursorRow bool, treePath TreePath, isSnapshot bool) string {
	// Top border: render as empty line with tree continuation
	if isTop {
		return renderEmptyTreeRow(treePath, isCursorRow, m.focused, false)
	}

	// Use commit tree style (yellow) for border, magenta for snapshots, or dark when not visible
	treeStyle := commitTreeStyle
	if isSnapshot {
		treeStyle = snapshotTreeStyle
	}
	borderStyle := treeStyle
	if !visible {
		borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	}

	// Tree connector at column 1 (after margin space) - double line style
	connector := "╞"
	borderChar := "═"

	// Border extends full width to the screen edge, ending with ╝
	// Layout: margin(1) + ╞(1) + ═*(borderWidth-1) + ╝(1)
	borderWidth := m.width - 2
	if borderWidth < 1 {
		borderWidth = 1
	}

	border := strings.Repeat(borderChar, borderWidth-1) + "╛"
	if borderWidth == 1 {
		border = "╛"
	}

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▌")
		} else {
			arrow = " "
		}
		// Arrow replaces margin, then connector + border + ╝
		return arrow + treeStyle.Render(connector) + borderStyle.Render(border)
	}

	// Margin space + connector + border + ╝
	return " " + treeStyle.Render(connector) + borderStyle.Render(border)
}

// renderCommitHeaderTopBorder renders the top border of the commit header.
func (m Model) renderCommitHeaderTopBorder(row displayRow, isCursorRow bool) string {
	isSnapshot := row.commitIndex < len(m.commits) && m.commits[row.commitIndex].IsSnapshot
	result := m.renderCommitBorderLine(row.headerMode == HeaderThreeLine, true, isCursorRow, row.treePath, isSnapshot)

	// When the border line is empty (no tree content), draw a horizontal
	// divider to visually separate unfolded commits.
	// Also check cursor rows: renderEmptyTreeRow returns a non-empty arrow
	// even with no tree ancestors, so the divider would be skipped.
	if result == "" || (isCursorRow && len(row.treePath.Ancestors) == 0) {
		dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
		w := m.width
		if w <= 0 {
			w = 80
		}
		if isCursorRow {
			var arrow string
			if m.focused {
				arrow = cursorArrowStyle.Render("▌")
			} else {
				arrow = " "
			}
			return arrow + dividerStyle.Render(strings.Repeat("═", w-1))
		}
		return dividerStyle.Render(strings.Repeat("═", w))
	}

	return result
}

// renderCommitHeaderBottomBorder renders the bottom border of the commit header.
// Shows tree continuation │ below the commit fold icon (border moved to header line).
func (m Model) renderCommitHeaderBottomBorder(row displayRow, isCursorRow bool) string {
	treeCont := treeContinuationStyle.Render("│")
	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▌")
		} else {
			arrow = " "
		}
		return arrow + treeCont
	}
	return " " + treeCont
}
