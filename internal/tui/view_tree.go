package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tree hierarchy types for visual indicators (like the `tree` command).
// These represent the path from root to a node, allowing consistent rendering
// of continuation lines (│) and branch characters (├, └) across all row types.

// TreeLevel represents one level in the tree hierarchy.
type TreeLevel struct {
	IsLast   bool           // Is this the last child at its level?
	IsFolded bool           // Is this node folded (hiding children)?
	Style    lipgloss.Style // Color/style for tree chars at this level
	Depth    int            // 0=commit, 1=file, 2=hunk, 3=content
}

// TreePath represents the full path from root to a node.
type TreePath struct {
	Ancestors []TreeLevel // Parent levels (for continuation lines)
	Current   *TreeLevel  // This node's level (for branch char), nil for content rows
}

// TreeRowKind specifies how a row relates to its parent node in the tree.
type TreeRowKind int

const (
	// TreeRowHeader - the node's header row, shows branch (├─── or └───)
	TreeRowHeader TreeRowKind = iota
	// TreeRowPreview - preview rows below header (e.g., structural diff summary)
	// Shows sibling continuation (│ if more siblings below), not parent expansion state
	TreeRowPreview
	// TreeRowContent - actual expanded content under the node
	// Shows parent continuation (│ if parent is expanded)
	TreeRowContent
)

// HeaderMode determines how a node's header is rendered.
// The mode depends on the node's fold state and the previous sibling's fold state.
type HeaderMode int

const (
	// HeaderSingleLine - folded node: header only, no borders, no margin below
	HeaderSingleLine HeaderMode = iota
	// HeaderTwoLine - unfolded with prev sibling folded: header + bottom border (no top border)
	HeaderTwoLine
	// HeaderThreeLine - unfolded with prev sibling unfolded (or first child): top + header + bottom border
	HeaderThreeLine
)

const (
	// TreeLeftMargin is the left margin before tree characters (aligns with fold icon)
	TreeLeftMargin = 1
	// TreeLevelWidth is the width of each tree level: "│    " or "     "
	TreeLevelWidth = 5
	// TreeBranchWidth is the width of branch characters: "├───" or "└───"
	TreeBranchWidth = 4
	// TreeContentIndent is extra indent for content rows to align with file heading text
	TreeContentIndent = 2
)

// treeContinuationStyle is used for vertical continuation lines (│).
// Grey color provides visual hierarchy without competing with branch colors.
var treeContinuationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

// renderTreeContinuation renders the vertical continuation lines for ancestor levels.
// Example output: "│    │    " for two non-last ancestors.
func renderTreeContinuation(ancestors []TreeLevel) string {
	var b strings.Builder
	for _, level := range ancestors {
		if level.IsLast || level.IsFolded {
			b.WriteString("     ") // 5 spaces - no continuation needed
		} else {
			b.WriteString(treeContinuationStyle.Render("│"))
			b.WriteString("    ") // │ + 4 spaces = 5 chars
		}
	}
	return b.String()
}

// renderTreeContinuationTight renders tree continuation with minimal spacing.
// Uses just 1 space after │ instead of 4, for tighter content indentation.
func renderTreeContinuationTight(ancestors []TreeLevel) string {
	var b strings.Builder
	for _, level := range ancestors {
		if level.IsLast || level.IsFolded {
			b.WriteString("  ") // 2 spaces - no continuation needed
		} else {
			b.WriteString(treeContinuationStyle.Render("│"))
			b.WriteString(" ") // │ + 1 space = 2 chars
		}
	}
	return b.String()
}

// renderTreePrefixTight renders tree prefix for content rows with minimal spacing.
// Uses margin + tight continuation (2 chars per ancestor) for compact indentation.
func renderTreePrefixTight(path TreePath) string {
	margin := strings.Repeat(" ", TreeLeftMargin)
	return margin + renderTreeContinuationTight(path.Ancestors)
}

// renderTreePrefixTightWithCursor renders tree prefix for content rows, with cursor arrow
// replacing the left margin space when isCursorRow is true.
func renderTreePrefixTightWithCursor(path TreePath, isCursorRow bool, focused bool) string {
	continuation := renderTreeContinuationTight(path.Ancestors)
	if isCursorRow {
		var arrow string
		if focused {
			arrow = cursorArrowStyle.Render("▶")
		} else {
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		return arrow + continuation
	}
	margin := strings.Repeat(" ", TreeLeftMargin)
	return margin + continuation
}

// renderEmptyTreeRow renders an empty/spacer row with tree continuation and optional cursor.
// Used for blank rows, top borders, and other visually-empty rows that still need to
// maintain the tree branch. The cursor arrow replaces the left margin space.
// When terminator is true, renders ┴ instead of │ to signal the end of the tree.
func renderEmptyTreeRow(treePath TreePath, isCursorRow bool, focused bool, terminator bool) string {
	continuation := renderTreeContinuationTight(treePath.Ancestors)
	if terminator {
		continuation = renderTreeTerminatorTight(treePath.Ancestors)
	}
	if isCursorRow {
		var arrow string
		if focused {
			arrow = cursorArrowStyle.Render("▶")
		} else {
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		// Arrow replaces the left margin space
		return arrow + continuation
	}
	if len(treePath.Ancestors) > 0 {
		margin := strings.Repeat(" ", TreeLeftMargin)
		return margin + continuation
	}
	return ""
}

// renderTreeTerminatorTight renders tree termination with ┴ instead of │.
// Used on the last blank row to visually signal the end of the tree.
func renderTreeTerminatorTight(ancestors []TreeLevel) string {
	var b strings.Builder
	for _, level := range ancestors {
		if level.IsLast || level.IsFolded {
			b.WriteString("  ") // 2 spaces - no continuation needed
		} else {
			b.WriteString(treeContinuationStyle.Render("┴"))
			b.WriteString(" ") // ┴ + 1 space = 2 chars
		}
	}
	return b.String()
}

// treeWidthTight calculates the character width of tight tree prefixes.
// Uses 2 chars per ancestor instead of 5, for compact content rows.
func treeWidthTight(numAncestors int) int {
	return TreeLeftMargin + numAncestors*2
}

// renderTreeBranch renders the branch character for a header node.
// T-connectors (├/└) use grey to match vertical lines, horizontal (━━━) uses status color.
// Example output: "├━━━" or "└━━━" with mixed colors.
func renderTreeBranch(level TreeLevel) string {
	if level.IsLast {
		return treeContinuationStyle.Render("└") + level.Style.Render("━━━")
	}
	return treeContinuationStyle.Render("├") + level.Style.Render("━━━")
}

// renderTreePrefix renders the full tree prefix for any row.
// For headers (isHeader=true): margin + continuation + branch (e.g., " ├───")
// For content (isHeader=false): margin + continuation + innermost │ (e.g., " │ ")
//
// The left margin aligns the tree with the fold icon column on commit headers.
func renderTreePrefix(path TreePath, isHeader bool) string {
	margin := strings.Repeat(" ", TreeLeftMargin)

	if len(path.Ancestors) == 0 {
		if isHeader && path.Current != nil {
			return margin + renderTreeBranch(*path.Current)
		}
		// Content with no ancestors: margin + content indent
		return margin + strings.Repeat(" ", TreeContentIndent)
	}

	// Split: outer ancestors get 5-char treatment, innermost gets 2-char
	outerAncestors := path.Ancestors[:len(path.Ancestors)-1]
	innermost := path.Ancestors[len(path.Ancestors)-1]
	continuation := renderTreeContinuation(outerAncestors)

	if isHeader && path.Current != nil {
		// Header row: margin + outer continuation + innermost continuation (5 chars) + branch (4 chars)
		var innermostCont string
		if innermost.IsLast || innermost.IsFolded {
			innermostCont = "     " // 5 spaces
		} else {
			innermostCont = treeContinuationStyle.Render("│") + "    " // │ + 4 spaces
		}
		return margin + continuation + innermostCont + renderTreeBranch(*path.Current)
	}

	// Content row: margin + outer continuation + innermost continuation (5 chars) + content indent (2 chars)
	contentIndent := strings.Repeat(" ", TreeContentIndent)
	if innermost.IsLast || innermost.IsFolded {
		return margin + continuation + "     " + contentIndent // 5 spaces + indent
	}
	return margin + continuation + treeContinuationStyle.Render("│") + "    " + contentIndent // │ + 4 spaces + indent
}

// treeWidth calculates the character width of tree prefixes.
// numAncestors is the number of ancestor levels (e.g., 1 for content under file).
// For headers, the Current level adds a branch (4 chars).
// All ancestor levels use 5-char treatment (│ + 4 spaces or 5 spaces).
// Content rows get additional TreeContentIndent (2 chars) for alignment.
// All widths include TreeLeftMargin.
func treeWidth(numAncestors int, isHeader bool) int {
	if numAncestors == 0 {
		if isHeader {
			return TreeLeftMargin + TreeBranchWidth // margin + branch: " ├───"
		}
		return TreeLeftMargin + TreeContentIndent // just the margin + content indent for content with no ancestors
	}
	// All ancestors get 5-char treatment
	ancestorWidth := numAncestors * TreeLevelWidth
	if isHeader {
		return TreeLeftMargin + ancestorWidth + TreeBranchWidth // margin + ancestors(5 each) + branch(4)
	}
	return TreeLeftMargin + ancestorWidth + TreeContentIndent // margin + ancestors(5 each) + content indent
}
