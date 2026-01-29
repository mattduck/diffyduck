package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/inlinediff"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
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
	rows := m.cachedRows
	if !m.rowsCacheValid {
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
	maxHeaderWidth  int // max header width across all files (for alignment in folded view)
	maxAddWidth     int // max addition count width across all files (for column alignment)
	maxRemWidth     int // max removal count width across all files (for column alignment)
	maxCountWidth   int // max stats count width across all files (for bar alignment)
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
	chunkStartLine int // first line of the following chunk (new/right side), for breadcrumbs
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
	isLastFileInCommit bool   // true if this file is the last file in its commit (for tree └─ vs ├─)
	isFileFolded       bool   // true if the parent file is folded (hide commit-level tree line)
	commitInfoLine     string // text content for info body lines
	// Comment fields (for RowKindComment rows)
	commentText      string // text of the comment (for rendering)
	commentLineNum   int    // line number this comment belongs to (for association)
	commentRowIndex  int    // index within the comment box (0=top border, 1..n-2=content, n-1=bottom border)
	commentRowCount  int    // total rows in this comment box
	commentLineIndex int    // which line of comment content this is (for content rows, -1 for borders)
	// Structural diff fields (for RowKindStructuralDiff rows)
	isStructuralDiff        bool   // true if this is a structural diff row
	structuralDiffLine      string // the formatted line (e.g., "  ~ func FuncA")
	structuralDiffIsBlank   bool   // true if this is a blank separator line
	structuralDiffAdded     int    // lines added within this element
	structuralDiffRemoved   int    // lines removed within this element
	structuralDiffMaxAddLen int    // max width of add counts (for alignment)
	structuralDiffMaxRemLen int    // max width of remove counts (for alignment)
}

// buildCommentRows creates displayRow entries for a comment box.
func buildCommentRows(fileIndex int, lineNum int, comment string) []displayRow {
	if comment == "" {
		return nil
	}

	// Split comment into lines
	lines := strings.Split(comment, "\n")
	rowCount := len(lines) + 2 // content lines + top border + bottom border

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
	}

	// Content lines
	for i := range lines {
		rows[i+1] = displayRow{
			kind:             RowKindComment,
			fileIndex:        fileIndex,
			commentText:      comment,
			commentLineNum:   lineNum,
			commentRowIndex:  i + 1,
			commentRowCount:  rowCount,
			commentLineIndex: i,
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

	// Calculate max header width and max add/rem widths across all visible files
	maxHeaderWidth := 0
	maxAddWidth := 0
	maxRemWidth := 0
	for commitIdx, commit := range m.commits {
		// Skip files in folded commits for width calculation
		if commit.Info.HasMetadata() && commit.FoldLevel == sidebyside.CommitFolded {
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
			w := displayWidth(header)
			if w > maxHeaderWidth {
				maxHeaderWidth = w
			}
			added, removed := countFileStats(fp)
			aw := statsAddWidth(added)
			if aw > maxAddWidth {
				maxAddWidth = aw
			}
			rw := statsRemWidth(removed)
			if rw > maxRemWidth {
				maxRemWidth = rw
			}
		}
	}

	// Calculate consistent header box width for borders
	totalFiles := len(m.files)
	numDigits := len(fmt.Sprintf("%d", totalFiles))
	fileNumWidth := 1 + numDigits
	iconPartWidth := 3 + 1 + 1 + fileNumWidth + 1 + 1 + 1 // "   ◐ #01 ~ "
	maxStatsBarWidth := statsBarDisplayWidth(maxAddWidth, maxRemWidth)
	headerContentWidth := maxHeaderWidth + maxStatsBarWidth

	// Check if structural diff content is wider than header content
	maxStructuralDiffWidth := 0
	for fileIdx := range m.files {
		w := m.structuralDiffMaxContentWidth(fileIdx)
		if w > maxStructuralDiffWidth {
			maxStructuralDiffWidth = w
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

	// Calculate max commit header column widths for alignment
	maxCommitFilesWidth := 0
	maxCommitAddWidth := 0
	maxCommitRemWidth := 0
	maxCommitTimeWidth := 0
	maxCommitSubjectWidth := 0
	for commitIdx := range m.commits {
		// Get file range for this commit
		startIdx := m.commitFileStarts[commitIdx]
		endIdx := len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}
		// Calculate stats for this commit
		commitFileCount := endIdx - startIdx
		commitAdded := 0
		commitRemoved := 0
		for i := startIdx; i < endIdx; i++ {
			added, removed := countFileStats(m.files[i])
			commitAdded += added
			commitRemoved += removed
		}
		// Track max widths
		fw := len(fmt.Sprintf("%d", commitFileCount))
		if fw > maxCommitFilesWidth {
			maxCommitFilesWidth = fw
		}
		aw := len(fmt.Sprintf("+%d", commitAdded))
		if aw > maxCommitAddWidth {
			maxCommitAddWidth = aw
		}
		rw := len(fmt.Sprintf("-%d", commitRemoved))
		if rw > maxCommitRemWidth {
			maxCommitRemWidth = rw
		}
		tw := len(formatShortRelativeDate(m.commits[commitIdx].Info.Date))
		if tw > maxCommitTimeWidth {
			maxCommitTimeWidth = tw
		}
		// Subject width (capped at 120) - use displayWidth for Unicode
		sw := displayWidth(m.commits[commitIdx].Info.Subject)
		if sw > 120 {
			sw = 120
		}
		if sw > maxCommitSubjectWidth {
			maxCommitSubjectWidth = sw
		}
	}

	// Build rows for each commit
	// Note: The first item's top border is rendered specially in getVisibleRows,
	// not as part of the content rows (so it doesn't affect cursor line numbering).
	for commitIdx, commit := range m.commits {
		// Add commit header row if commit has metadata
		if commit.Info.HasMetadata() {
			commitFolded := commit.FoldLevel == sidebyside.CommitFolded
			isFirstCommit := commitIdx == 0
			prevCommitUnfolded := !isFirstCommit && m.commits[commitIdx-1].FoldLevel != sidebyside.CommitFolded

			// Compute header mode for this commit
			commitHeaderMode := determineCommitHeaderMode(commitFolded, isFirstCommit, prevCommitUnfolded)
			// Border is visible when mode is ThreeLine
			// Subsequent commits get their top border from the previous commit's separator row

			// Calculate actual content width for this commit's header
			// Layout: prefix(1) + icon(1) + space(1) + sha(7) + space(1) + files + space(1)
			//         + added + space(1) + removed + space(1) + time + space(1) + author + space(1) + subject
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
			// Total: prefix(1) + icon(1) + space(1) + sha(7) + space(1) + files + space(1)
			//        + added + space(1) + removed + space(1) + time + space(1) + author + space(1) + subject
			commitHeaderWidth := 1 + 1 + 1 + 7 + 1 + filesWidth + 1 + addedWidth + 1 + removedWidth + 1 + timeWidth + 1 + authorWidth + 1 + subjectWidth

			rows = append(rows, displayRow{
				kind:                  RowKindCommitHeader,
				fileIndex:             -1,
				isCommitHeader:        true,
				commitFoldLevel:       commit.FoldLevel,
				commitIndex:           commitIdx,
				maxCommitFilesWidth:   maxCommitFilesWidth,
				maxCommitAddWidth:     maxCommitAddWidth,
				maxCommitRemWidth:     maxCommitRemWidth,
				maxCommitTimeWidth:    maxCommitTimeWidth,
				maxCommitSubjectWidth: maxCommitSubjectWidth,
				headerMode:            commitHeaderMode,
				headerBoxWidth:        commitHeaderWidth,
			})

			// If commit is folded, skip its files
			if commit.FoldLevel == sidebyside.CommitFolded {
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
				headerBoxWidth:             commitHeaderWidth,
			})

			// Calculate commit-info header box width for the top border slot
			treePrefixWidth := treeWidth(0, true) + 1
			iconPartWidth := treePrefixWidth + 2
			headerText := "details"
			infoHeaderBoxWidth := iconPartWidth + len(headerText) + 2

			// Build tree path for commit-info borders
			// Details is a child of the commit; check if there are files to determine if it's last
			startFileIdx := 0
			if commitIdx < len(m.commitFileStarts) {
				startFileIdx = m.commitFileStarts[commitIdx]
			}
			endFileIdx := len(m.files)
			if commitIdx+1 < len(m.commitFileStarts) {
				endFileIdx = m.commitFileStarts[commitIdx+1]
			}
			hasFiles := startFileIdx < endFileIdx
			detailsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // grey for details
			detailsLevel := TreeLevel{
				IsLast:   !hasFiles,
				IsFolded: commit.FoldLevel == sidebyside.CommitNormal,
				Style:    detailsStyle,
				Depth:    0,
			}
			detailsBorderTreePath := TreePath{
				Ancestors: []TreeLevel{detailsLevel},
				Current:   nil,
			}

			// Top border slot for commit-info - renders as border when expanded, blank when normal
			rows = append(rows, displayRow{
				kind:                  RowKindCommitInfoTopBorder,
				fileIndex:             -1,
				isCommitInfoTopBorder: true,
				commitIndex:           commitIdx,
				commitFoldLevel:       commit.FoldLevel,
				headerBoxWidth:        infoHeaderBoxWidth,
				treePrefixWidth:       treePrefixWidth,
				treePath:              detailsBorderTreePath,
			})

			// Add commit info rows (foldable child node under commit)
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
		if startIdx < endIdx && commit.Info.HasMetadata() {
			commitInfoExpanded := commit.FoldLevel == sidebyside.CommitExpanded
			if commitInfoExpanded {
				firstFileFolded := m.files[startIdx].FoldLevel == sidebyside.FoldFolded
				// First file's prev sibling is commit-info, not another file
				firstFileHeaderMode := determineFileHeaderMode(firstFileFolded, false, commitInfoExpanded)
				firstIsLastFile := startIdx == endIdx-1
				firstBorderTreePath := m.buildFileTreePath(startIdx, firstIsLastFile, firstFileFolded, TreeRowContent)
				rows = append(rows, displayRow{
					kind:              RowKindHeaderTopBorder,
					fileIndex:         startIdx,
					isHeaderTopBorder: true,
					foldLevel:         sidebyside.FoldNormal,
					status:            fileStatusFromPair(m.files[startIdx]),
					headerBoxWidth:    headerBoxWidth,
					treePrefixWidth:   treeWidth(0, true) + 1, // +1 to align with fold icon
					headerMode:        firstFileHeaderMode,
					treePath:          firstBorderTreePath,
				})
			}
		}

		// Add file rows for this commit
		for fileIdx := startIdx; fileIdx < endIdx; fileIdx++ {
			fp := m.files[fileIdx]
			rows = m.buildFileRows(rows, fileIdx, fp, startIdx, endIdx, maxHeaderWidth, maxAddWidth, maxRemWidth, headerBoxWidth)
		}

		// Add separator row between commits (blank line after last file, before next commit)
		// This row becomes the top border slot for the next commit when this commit is unfolded
		if commit.Info.HasMetadata() && commitIdx+1 < len(m.commits) && m.commits[commitIdx+1].Info.HasMetadata() {
			nextCommit := m.commits[commitIdx+1]
			thisCommitUnfolded := commit.FoldLevel != sidebyside.CommitFolded

			if thisCommitUnfolded {
				// Unfolded commit produces margin; add top border slot for next commit
				// Compute next commit's header mode to determine if border should be visible
				nextCommitFolded := nextCommit.FoldLevel == sidebyside.CommitFolded
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

	return rows
}

// buildRowsLegacy handles the case where Model was created without using New/NewWithCommits.
// This maintains backward compatibility with tests that directly set m.files.
func (m Model) buildRowsLegacy() []displayRow {
	var rows []displayRow

	// Calculate max header width and max add/rem widths across all files
	maxHeaderWidth := 0
	maxAddWidth := 0
	maxRemWidth := 0
	for _, fp := range m.files {
		header := formatFileHeader(fp)
		w := displayWidth(header)
		if w > maxHeaderWidth {
			maxHeaderWidth = w
		}
		added, removed := countFileStats(fp)
		aw := statsAddWidth(added)
		if aw > maxAddWidth {
			maxAddWidth = aw
		}
		rw := statsRemWidth(removed)
		if rw > maxRemWidth {
			maxRemWidth = rw
		}
	}

	// Calculate consistent header box width for borders
	totalFiles := len(m.files)
	numDigits := len(fmt.Sprintf("%d", totalFiles))
	fileNumWidth := 1 + numDigits
	iconPartWidth := 3 + 1 + 1 + fileNumWidth + 1 + 1 + 1
	maxStatsBarWidth := statsBarDisplayWidth(maxAddWidth, maxRemWidth)
	headerContentWidth := maxHeaderWidth + maxStatsBarWidth

	// Check if structural diff content is wider than header content
	maxStructuralDiffWidth := 0
	for fileIdx := range m.files {
		w := m.structuralDiffMaxContentWidth(fileIdx)
		if w > maxStructuralDiffWidth {
			maxStructuralDiffWidth = w
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
		rows = m.buildFileRows(rows, fileIdx, fp, 0, len(m.files), maxHeaderWidth, maxAddWidth, maxRemWidth, headerBoxWidth)
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
func (m Model) buildFileRows(rows []displayRow, fileIdx int, fp sidebyside.FilePair, commitStartIdx, commitEndIdx int, maxHeaderWidth, maxAddWidth, maxRemWidth, headerBoxWidth int) []displayRow {
	added, removed := countFileStats(fp)
	status := fileStatusFromPair(fp)

	// Check if this is the first file in the commit
	isFirstFile := fileIdx == commitStartIdx

	// Check if previous sibling is unfolded (for determining header mode)
	prevSiblingUnfolded := false
	if fileIdx > commitStartIdx {
		prevSiblingUnfolded = m.files[fileIdx-1].FoldLevel != sidebyside.FoldFolded
	}

	// Compute header mode based on fold state and prev sibling
	isFolded := fp.FoldLevel == sidebyside.FoldFolded
	headerMode := determineFileHeaderMode(isFolded, isFirstFile, prevSiblingUnfolded)

	isLastFile := fileIdx == commitEndIdx-1

	// Build tree paths for this file (header vs content have different paths)
	headerTreePath := m.buildFileTreePath(fileIdx, isLastFile, fp.FoldLevel == sidebyside.FoldFolded, TreeRowHeader)

	switch fp.FoldLevel {
	case sidebyside.FoldFolded:
		header := formatFileHeader(fp)
		rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldFolded, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxAddWidth: maxAddWidth, maxRemWidth: maxRemWidth, maxCountWidth: statsCountWidth(added, removed, maxAddWidth), headerBoxWidth: headerBoxWidth, isLastFileInCommit: isLastFile, treePath: headerTreePath, headerMode: headerMode})

		// Add structural diff rows (no borders in folded mode, file is folded)
		structuralRows := m.buildStructuralDiffRows(fileIdx, headerBoxWidth, isLastFile, true)
		rows = append(rows, structuralRows...)

		// Only add bottom margin if there was preview content (structural diff rows)
		if len(structuralRows) > 0 {
			marginTreePath := m.buildFileTreePath(fileIdx, isLastFile, true, TreeRowContent)
			rows = append(rows, displayRow{
				kind:               RowKindBlank,
				fileIndex:          fileIdx,
				isBlank:            true,
				isLastFileInCommit: isLastFile,
				treePath:           marginTreePath,
			})
		}

	case sidebyside.FoldExpanded:
		if fp.HasContent() {
			// Note: First file's top border is added after commit body rows, not here
			// This prevents content shift when first file is unfolded
			expandedHeaderTreePath := m.buildFileTreePath(fileIdx, isLastFile, false, TreeRowHeader)
			expandedContentTreePath := m.buildFileTreePath(fileIdx, isLastFile, false, TreeRowContent)

			header := formatFileHeader(fp)
			rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldExpanded, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxAddWidth: maxAddWidth, maxRemWidth: maxRemWidth, maxCountWidth: statsCountWidth(added, removed, maxAddWidth), headerBoxWidth: headerBoxWidth, isLastFileInCommit: isLastFile, treePath: expandedHeaderTreePath, headerMode: headerMode})

			// No structural diff preview in expanded mode - the full diff content is shown instead

			rows = append(rows, displayRow{kind: RowKindHeaderSpacer, fileIndex: fileIdx, isHeaderSpacer: true, foldLevel: sidebyside.FoldExpanded, status: status, headerBoxWidth: headerBoxWidth, treePrefixWidth: treeWidth(0, true) + 1, headerMode: headerMode, treePath: expandedContentTreePath})

			expandedRows := m.buildExpandedRows(fp)
			for i := range expandedRows {
				expandedRows[i].fileIndex = fileIdx
				expandedRows[i].isLastFileInCommit = isLastFile
				expandedRows[i].treePath = expandedContentTreePath
				if i == 0 {
					expandedRows[i].isFirstLine = true
				}
				if i == len(expandedRows)-1 {
					expandedRows[i].isLastLine = true
				}
			}
			// Append expanded rows with comment rows interleaved
			for _, expRow := range expandedRows {
				rows = append(rows, expRow)
				// Add comment rows if this is a content row with a comment
				if expRow.kind == RowKindContent && expRow.pair.New.Num > 0 {
					key := commentKey{fileIndex: fileIdx, newLineNum: expRow.pair.New.Num}
					if comment, ok := m.comments[key]; ok {
						commentRows := buildCommentRows(fileIdx, expRow.pair.New.Num, comment)
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

			// No bottom margin in expanded mode - content flows directly to next file

			if !isLastFile {
				// Top border slot belongs to the NEXT file (fileIdx+1), not the current file
				// Current file is unfolded (FoldExpanded), so next file's prev sibling is unfolded
				// Always add this row to prevent content shift; render as border or blank based on next file's mode
				nextFileFolded := m.files[fileIdx+1].FoldLevel == sidebyside.FoldFolded
				nextFileHeaderMode := determineFileHeaderMode(nextFileFolded, false, true)
				nextIsLastFile := fileIdx+1 == commitEndIdx-1
				nextBorderTreePath := m.buildFileTreePath(fileIdx+1, nextIsLastFile, nextFileFolded, TreeRowContent)
				rows = append(rows, displayRow{kind: RowKindHeaderTopBorder, fileIndex: fileIdx + 1, isHeaderTopBorder: true, foldLevel: sidebyside.FoldExpanded, status: status, headerBoxWidth: headerBoxWidth, treePrefixWidth: treeWidth(0, true) + 1, headerMode: nextFileHeaderMode, treePath: nextBorderTreePath})
			}
			return rows
		}
		fallthrough

	default: // FoldNormal
		// Note: First file's top border is added after commit body rows, not here
		// This prevents content shift when first file is unfolded
		normalHeaderTreePath := m.buildFileTreePath(fileIdx, isLastFile, false, TreeRowHeader)
		normalContentTreePath := m.buildFileTreePath(fileIdx, isLastFile, false, TreeRowContent)

		header := formatFileHeader(fp)
		rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: fp.FoldLevel, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxAddWidth: maxAddWidth, maxRemWidth: maxRemWidth, maxCountWidth: statsCountWidth(added, removed, maxAddWidth), headerBoxWidth: headerBoxWidth, isLastFileInCommit: isLastFile, treePath: normalHeaderTreePath, headerMode: headerMode})

		// No structural diff preview in normal/hunk mode - diff content is shown instead

		rows = append(rows, displayRow{kind: RowKindHeaderSpacer, fileIndex: fileIdx, isHeaderSpacer: true, foldLevel: fp.FoldLevel, status: status, headerBoxWidth: headerBoxWidth, treePrefixWidth: treeWidth(0, true) + 1, headerMode: headerMode, treePath: normalContentTreePath})

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
		} else {
			var prevLeft, prevRight int
			contentTreePath := m.buildFileTreePath(fileIdx, isLastFile, false, TreeRowContent)
			for i, pair := range fp.Pairs {
				if i == 0 && (pair.Old.Num > 1 || pair.New.Num > 1) {
					chunkStartLine := findFirstNewLineNum(fp.Pairs, i)
					rows = append(rows, displayRow{kind: RowKindSeparatorTop, fileIndex: fileIdx, isSeparatorTop: true, isLastFileInCommit: isLastFile, treePath: contentTreePath})
					rows = append(rows, displayRow{kind: RowKindSeparator, fileIndex: fileIdx, isSeparator: true, chunkStartLine: chunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
					rows = append(rows, displayRow{kind: RowKindSeparatorBottom, fileIndex: fileIdx, isSeparatorBottom: true, chunkStartLine: chunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
				}

				if i > 0 && isHunkBoundary(prevLeft, prevRight, pair.Old.Num, pair.New.Num) {
					chunkStartLine := findFirstNewLineNum(fp.Pairs, i)
					rows = append(rows, displayRow{kind: RowKindSeparatorTop, fileIndex: fileIdx, isSeparatorTop: true, isLastFileInCommit: isLastFile, treePath: contentTreePath})
					rows = append(rows, displayRow{kind: RowKindSeparator, fileIndex: fileIdx, isSeparator: true, chunkStartLine: chunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
					rows = append(rows, displayRow{kind: RowKindSeparatorBottom, fileIndex: fileIdx, isSeparatorBottom: true, chunkStartLine: chunkStartLine, isLastFileInCommit: isLastFile, treePath: contentTreePath})
				}

				row := displayRow{kind: RowKindContent, fileIndex: fileIdx, pair: pair, isLastFileInCommit: isLastFile, treePath: m.buildFileTreePath(fileIdx, isLastFile, false, TreeRowContent)}
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
						commentRows := buildCommentRows(fileIdx, pair.New.Num, comment)
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
		}

		// No bottom margin in normal mode - content flows directly to next file

		if !isLastFile {
			// Top border slot belongs to the NEXT file (fileIdx+1), not the current file
			// Current file is unfolded (FoldNormal), so next file's prev sibling is unfolded
			// Always add this row to prevent content shift; render as border or blank based on next file's mode
			nextFileFolded := m.files[fileIdx+1].FoldLevel == sidebyside.FoldFolded
			nextFileHeaderMode := determineFileHeaderMode(nextFileFolded, false, true)
			nextIsLastFile := fileIdx+1 == commitEndIdx-1
			nextBorderTreePath := m.buildFileTreePath(fileIdx+1, nextIsLastFile, nextFileFolded, TreeRowContent)
			rows = append(rows, displayRow{kind: RowKindHeaderTopBorder, fileIndex: fileIdx + 1, isHeaderTopBorder: true, foldLevel: fp.FoldLevel, status: status, headerBoxWidth: headerBoxWidth, treePrefixWidth: treeWidth(0, true) + 1, headerMode: nextFileHeaderMode, treePath: nextBorderTreePath})
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
			m.commits[0].FoldLevel != sidebyside.CommitFolded

		// Check if first file is unfolded and we're in diff view (no commit metadata)
		firstFileUnfolded := len(m.files) > 0 &&
			m.files[0].FoldLevel != sidebyside.FoldFolded
		isDiffView := len(m.commits) == 0 ||
			(len(m.commits) > 0 && !m.commits[0].Info.HasMetadata())

		if firstCommitUnfolded {
			// Render first commit's top border in the margin (not cursor-selectable)
			visible = append(visible, m.renderCommitBorderLine(true, true, 0, false))
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
			if isCursorRow {
				rendered = m.renderBlankWithCursor(leftHalfWidth, rightHalfWidth, lineNumWidth)
			} else if len(row.treePath.Ancestors) > 0 {
				// Blank row with tree continuation (e.g., after preview rows)
				rendered = renderTreePrefixTight(row.treePath)
			} else {
				rendered = m.renderInterFileBlank()
			}
		} else if row.isHeader {
			rendered = m.renderHeader(row.header, row.foldLevel, row.headerMode, row.status, row.added, row.removed, row.maxHeaderWidth, row.maxAddWidth, row.maxRemWidth, row.headerBoxWidth, row.fileIndex, i, isCursorRow, row.treePath)
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
// If structure data is available, shows breadcrumbs on the left side (new content).
func (m Model) renderHunkSeparator(row displayRow, leftHalfWidth, rightHalfWidth int, isCursorRow bool) string {
	shadeStyle := hunkSeparatorStyle
	lineNumWidth := m.lineNumWidth()

	// Tree prefix using tight spacing for compact content
	treeContinuation := renderTreePrefixTight(row.treePath)
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

	// Cursor row: tree + arrow in gutter, lineNumWidth chars with cursor bg, then breadcrumb in content area
	// When unfocused, use outline arrow and no background highlighting
	var leftGutter, rightGutter string
	if m.focused {
		leftGutter = cursorArrowStyle.Render("▶") + shadeStyle.Render("░") + cursorStyle.Render(strings.Repeat("░", lineNumWidth))
		rightGutter = cursorArrowStyle.Render("▶") + shadeStyle.Render("░") + cursorStyle.Render(strings.Repeat("░", lineNumWidth))
	} else {
		leftGutter = unfocusedCursorArrowStyle.Render("▷") + shadeStyle.Render("░") + shadeStyle.Render(strings.Repeat("░", lineNumWidth))
		rightGutter = unfocusedCursorArrowStyle.Render("▷") + shadeStyle.Render("░") + shadeStyle.Render(strings.Repeat("░", lineNumWidth))
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
	treeContinuation := renderTreePrefixTight(row.treePath)
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

	// Cursor row: tree + arrow + faint shade, then lineNumWidth chars with cursor bg, rest is faint shading
	// When unfocused, use outline arrow and no background highlighting
	var leftArrow, rightArrow string
	if m.focused {
		leftArrow = cursorArrowStyle.Render("▶") + faintShadeStyle.Render("░")
		rightArrow = cursorArrowStyle.Render("▶") + faintShadeStyle.Render("░")
	} else {
		leftArrow = unfocusedCursorArrowStyle.Render("▷") + faintShadeStyle.Render("░")
		rightArrow = unfocusedCursorArrowStyle.Render("▷") + faintShadeStyle.Render("░")
	}

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

// renderBlankWithCursor renders a blank line with highlighted gutter areas when cursor is on it.
// Uses shader characters (░) consistent with other row types like hunk separators.
func (m Model) renderBlankWithCursor(leftHalfWidth, rightHalfWidth, lineNumWidth int) string {
	// Use faint shader style consistent with hunk separator rows
	faintShadeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)

	// Arrow column width: indicator(1) + space(1) = 2
	arrowWidth := 2

	leftContentWidth := leftHalfWidth - arrowWidth
	if leftContentWidth < 0 {
		leftContentWidth = 0
	}
	rightContentWidth := rightHalfWidth - arrowWidth
	if rightContentWidth < 0 {
		rightContentWidth = 0
	}

	// Cursor row: arrow + faint shade, then lineNumWidth chars with cursor bg, rest is faint shading
	// When unfocused, use outline arrow and no background highlighting
	var leftArrow, rightArrow string
	if m.focused {
		leftArrow = cursorArrowStyle.Render("▶") + faintShadeStyle.Render("░")
		rightArrow = cursorArrowStyle.Render("▶") + faintShadeStyle.Render("░")
	} else {
		leftArrow = unfocusedCursorArrowStyle.Render("▷") + faintShadeStyle.Render("░")
		rightArrow = unfocusedCursorArrowStyle.Render("▷") + faintShadeStyle.Render("░")
	}

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

	return leftArrow + leftContent + faintShadeStyle.Render("░░░") + rightArrow + rightContent
}

// renderInterFileBlank renders a blank line between files.
func (m Model) renderInterFileBlank() string {
	return ""
}

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

	// Render tree continuation for empty row
	if isCursorRow {
		if m.focused {
			return cursorArrowStyle.Render("▶")
		}
		return unfocusedCursorArrowStyle.Render("▷")
	}

	// Empty row with tree continuation if needed
	if len(treePath.Ancestors) > 0 {
		return renderTreePrefixTight(treePath)
	}
	return ""
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
	borderLine := strings.Repeat("━", borderWidth)

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
func (m Model) renderCommitBorderLine(visible bool, isTop bool, contentWidth int, isCursorRow bool) string {
	// Top border: render as empty line (temporary)
	if isTop {
		if isCursorRow {
			if m.focused {
				return cursorArrowStyle.Render("▶")
			}
			return unfocusedCursorArrowStyle.Render("▷")
		}
		return ""
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
	borderWidth := contentWidth
	if borderWidth <= 0 {
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
		// Arrow replaces margin, then connector + border
		return arrow + borderStyle.Render(connector+strings.Repeat(borderChar, borderWidth-1))
	}

	// Margin space + connector + border
	return " " + borderStyle.Render(connector+strings.Repeat(borderChar, borderWidth-1))
}

// renderCommitHeaderTopBorder renders the top border of the commit header.
func (m Model) renderCommitHeaderTopBorder(row displayRow, isCursorRow bool) string {
	return m.renderCommitBorderLine(row.headerMode == HeaderThreeLine, true, 0, isCursorRow)
}

// renderCommitHeaderBottomBorder renders the bottom border of the commit header.
// The border width matches the commit header content width.
func (m Model) renderCommitHeaderBottomBorder(row displayRow, isCursorRow bool) string {
	return m.renderCommitBorderLine(row.headerMode == HeaderThreeLine, false, row.headerBoxWidth, isCursorRow)
}

// renderCommitHeaderRow renders a commit header row in the content area.
// This is shown when viewing a commit and can be folded/unfolded.
//
// Layout: [cursor] [fold] [sha] [files] [+added] [-removed] [time] [author] [subject]
// Fixed columns (left): sha, files, +added, -removed, time, author (max 15 chars)
// Dynamic column (right): subject (max 120 chars)
func (m Model) renderCommitHeaderRow(row displayRow, isCursorRow bool) string {
	// Use commitIndex from the row to get the correct commit
	if row.commitIndex < 0 || row.commitIndex >= len(m.commits) {
		return ""
	}
	commit := &m.commits[row.commitIndex]
	commitInfo := commit.Info

	// Styles
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	authorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	foldIconColor := "8"
	if isCursorRow {
		foldIconColor = "15"
	}
	foldIconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(foldIconColor))

	// Fold icon
	var foldIcon string
	switch row.commitFoldLevel {
	case sidebyside.CommitFolded:
		foldIcon = "◯"
	case sidebyside.CommitNormal:
		foldIcon = "◐"
	case sidebyside.CommitExpanded:
		foldIcon = "●"
	}

	// Calculate file stats for this commit only
	startIdx := m.commitFileStarts[row.commitIndex]
	endIdx := len(m.files)
	if row.commitIndex+1 < len(m.commits) {
		endIdx = m.commitFileStarts[row.commitIndex+1]
	}
	totalAdded := 0
	totalRemoved := 0
	for i := startIdx; i < endIdx; i++ {
		added, removed := countFileStats(m.files[i])
		totalAdded += added
		totalRemoved += removed
	}
	fileCount := endIdx - startIdx

	// Cursor prefix
	var prefix string
	if isCursorRow {
		if m.focused {
			prefix = cursorArrowStyle.Render("▶")
		} else {
			prefix = unfocusedCursorArrowStyle.Render("▷")
		}
	} else {
		// Single space margin (fold icon aligns with tree branch column)
		prefix = " "
	}

	// Build fixed columns
	// Format: [prefix][fold] [sha] [files] [+added] [-removed] [time] [author] [subject]

	shaText := commitInfo.ShortSHA()
	filesText := fmt.Sprintf("%d", fileCount)
	addedText := fmt.Sprintf("+%d", totalAdded)
	removedText := fmt.Sprintf("-%d", totalRemoved)
	timeText := formatShortRelativeDate(commitInfo.Date)

	// Pad columns to max widths for alignment across commits (right-align numbers)
	if len(filesText) < row.maxCommitFilesWidth {
		filesText = strings.Repeat(" ", row.maxCommitFilesWidth-len(filesText)) + filesText
	}
	if len(addedText) < row.maxCommitAddWidth {
		addedText = strings.Repeat(" ", row.maxCommitAddWidth-len(addedText)) + addedText
	}
	if len(removedText) < row.maxCommitRemWidth {
		removedText = strings.Repeat(" ", row.maxCommitRemWidth-len(removedText)) + removedText
	}
	if len(timeText) < row.maxCommitTimeWidth {
		timeText = strings.Repeat(" ", row.maxCommitTimeWidth-len(timeText)) + timeText
	}

	// Author: max 15 display columns, truncate with "..." if longer
	// Use displayWidth for Unicode-aware width calculation
	author := commitInfo.Author
	maxAuthorLen := 15
	authorWidth := displayWidth(author)
	if authorWidth > maxAuthorLen {
		author = runewidth.Truncate(author, maxAuthorLen, "...")
		authorWidth = maxAuthorLen
	}

	// Build the fixed part with styling
	fixedPart := prefix +
		foldIconStyle.Render(foldIcon) + " " +
		shaStyle.Render(shaText) + " " +
		dimStyle.Render(filesText) + " " +
		addedStyle.Render(addedText) + " " +
		removedStyle.Render(removedText) + " " +
		dimStyle.Render(timeText) + " " +
		authorStyle.Render(author)

	// Subject: truncate to subjectDisplayWidth with Unicode-aware width
	subject := commitInfo.Subject
	subjectDisplayWidth := row.maxCommitSubjectWidth
	subjectWidth := displayWidth(subject)

	// Truncate if needed (capped at 120 display columns, then to subjectDisplayWidth)
	if subjectWidth > 120 {
		subject = runewidth.Truncate(subject, 120, "...")
		subjectWidth = displayWidth(subject)
	}
	if subjectWidth > subjectDisplayWidth {
		if subjectDisplayWidth > 3 {
			subject = runewidth.Truncate(subject, subjectDisplayWidth, "...")
		} else if subjectDisplayWidth > 0 {
			subject = runewidth.Truncate(subject, subjectDisplayWidth, "")
		} else {
			subject = ""
		}
		subjectWidth = displayWidth(subject)
	}

	// Pad subject to max width for alignment
	subjectPadding := ""
	if subjectWidth < subjectDisplayWidth {
		subjectPadding = strings.Repeat(" ", subjectDisplayWidth-subjectWidth)
	}

	// Build the dynamic part with padding
	var dynamicPart string
	if subjectDisplayWidth > 0 {
		dynamicPart = " " + subject + subjectPadding
	}

	return fixedPart + dynamicPart
}

// renderCommitInfoHeader renders the commit info header row (foldable child node).
// Uses yellow styling. When expanded, renders with vertical borders like file headers.
// Layout folded: [tree prefix] [fold icon] [header text]
// Layout expanded: [tree prefix] │ [fold icon] [header text] [padding] │
func (m Model) renderCommitInfoHeader(row displayRow, isCursorRow bool) string {
	// Fold icon: ◯ for CommitNormal (header only), ● for CommitExpanded (full content)
	var foldIcon string
	switch row.commitFoldLevel {
	case sidebyside.CommitExpanded:
		foldIcon = "●"
	default:
		foldIcon = "◯"
	}

	// Icon color: fg=8 normally, fg=15 when cursor is on row
	iconColor := "8"
	if isCursorRow {
		iconColor = "15"
	}
	iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(iconColor))
	styledIcon := iconStyle.Render(foldIcon)

	// Header text (e.g., "details") - no bold
	styledHeader := row.header

	// Tree prefix using the generic tree infrastructure
	treeLine := renderTreePrefix(row.treePath, true) // true = header row

	// Cursor handling: just replace first char with arrow, like commit headers
	if isCursorRow {
		var prefix string
		if m.focused {
			prefix = cursorArrowStyle.Render("▶")
		} else {
			prefix = unfocusedCursorArrowStyle.Render("▷")
		}
		// Replace first char of treeLine with arrow
		if len(treeLine) > 0 {
			treeLine = prefix + treeLine[1:]
		} else {
			treeLine = prefix
		}
	}

	return treeLine + " " + styledIcon + " " + styledHeader
}

// renderCommitInfoTopBorder renders the top border row above the commit info header.
// Renders as empty space with tree continuation (keeping the row for layout consistency).
func (m Model) renderCommitInfoTopBorder(row displayRow, isCursorRow bool) string {
	// Always show tree continuation on this line to connect commit border to children
	margin := strings.Repeat(" ", TreeLeftMargin)
	treeCont := treeContinuationStyle.Render("│")

	if isCursorRow {
		if m.focused {
			return cursorArrowStyle.Render("▶") + treeCont
		}
		return unfocusedCursorArrowStyle.Render("▷") + treeCont
	}

	return margin + treeCont
}

// renderCommitInfoBottomBorder renders the bottom border of the commit info header.
// Renders as an underline starting at the fold icon position with a corner character.
func (m Model) renderCommitInfoBottomBorder(row displayRow, isCursorRow bool) string {
	greyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	// Build tree continuation from ancestors
	var treeCont string
	if len(row.treePath.Ancestors) > 0 {
		level := row.treePath.Ancestors[0]
		if !level.IsLast && !level.IsFolded {
			treeCont = treeContinuationStyle.Render("│")
		} else {
			treeCont = " "
		}
	}

	margin := strings.Repeat(" ", TreeLeftMargin)

	// Calculate spacing to position corner at fold icon column
	// treePrefixWidth = margin + branch + alignment
	// Corner should be at position treePrefixWidth (same column as fold icon)
	var spacesBeforeCorner int
	if len(row.treePath.Ancestors) > 0 {
		spacesBeforeCorner = row.treePrefixWidth - TreeLeftMargin - 1
	} else {
		spacesBeforeCorner = row.treePrefixWidth - TreeLeftMargin
	}
	if spacesBeforeCorner < 0 {
		spacesBeforeCorner = 0
	}
	spacing := strings.Repeat(" ", spacesBeforeCorner)

	// Border width: from corner to end of header content
	borderWidth := row.headerBoxWidth - row.treePrefixWidth
	if borderWidth < 1 {
		borderWidth = 1
	}

	// Use heavy box-drawing characters for underline: ┗ corner and ━ horizontal
	corner := "┗"
	borderLine := strings.Repeat("━", borderWidth)

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▶")
		} else {
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		if TreeLeftMargin > 0 {
			return arrow + margin[1:] + treeCont + spacing + greyStyle.Render(corner+borderLine)
		}
		return arrow + treeCont + spacing + greyStyle.Render(corner+borderLine)
	}

	return margin + treeCont + spacing + greyStyle.Render(corner+borderLine)
}

// renderCommitInfoBody renders a commit info body row (Author, Date, message content).
// Uses the same styling as the legacy commit body rows.
func (m Model) renderCommitInfoBody(row displayRow, isCursorRow bool) string {
	// Styles
	gutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	line := row.commitInfoLine
	styledLine := line

	// Apply syntax highlighting for commit/Author/Date lines
	if strings.HasPrefix(line, "commit ") {
		styledLine = gutterStyle.Render("commit ") + shaStyle.Render(line[7:])
	} else if strings.HasPrefix(line, "Author: ") {
		styledLine = gutterStyle.Render("Author: ") + line[8:]
	} else if strings.HasPrefix(line, "Date:   ") {
		styledLine = gutterStyle.Render("Date:   ") + line[8:]
	}

	// Tree prefix with tight spacing (just 1 space after │)
	margin := strings.Repeat(" ", TreeLeftMargin)
	treePrefix := margin + renderTreeContinuationTight(row.treePath.Ancestors)

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▶")
		} else {
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		// Replace first char of margin with arrow
		if TreeLeftMargin > 0 {
			return arrow + margin[1:] + renderTreeContinuationTight(row.treePath.Ancestors) + styledLine
		}
		return arrow + renderTreeContinuationTight(row.treePath.Ancestors) + styledLine
	}

	return treePrefix + styledLine
}

// buildCommitBodyRows creates display rows for the commit body (shown when expanded).
// Format is similar to git log: full SHA, author, date, then message body.
func (m Model) buildCommitBodyRows(commit *sidebyside.CommitSet, commitIdx int) []displayRow {
	var rows []displayRow
	info := commit.Info

	// Blank line after commit header row
	rows = append(rows, displayRow{
		kind:              RowKindCommitBody,
		fileIndex:         -1,
		isCommitBody:      true,
		commitBodyLine:    "",
		commitBodyIsBlank: true,
		commitIndex:       commitIdx,
	})

	// Line 1: "commit <full sha>" (indented 2 spaces)
	commitLine := "commit " + info.SHA
	rows = append(rows, displayRow{
		kind:           RowKindCommitBody,
		fileIndex:      -1,
		isCommitBody:   true,
		commitBodyLine: commitLine,
		commitIndex:    commitIdx,
	})

	// Line 2: "Author: <author> <email>"
	authorLine := "Author: " + info.Author
	if info.Email != "" {
		authorLine += " <" + info.Email + ">"
	}
	rows = append(rows, displayRow{
		kind:           RowKindCommitBody,
		fileIndex:      -1,
		isCommitBody:   true,
		commitBodyLine: authorLine,
		commitIndex:    commitIdx,
	})

	// Line 3: "Date:   <date>"
	dateLine := "Date:   " + info.Date
	rows = append(rows, displayRow{
		kind:           RowKindCommitBody,
		fileIndex:      -1,
		isCommitBody:   true,
		commitBodyLine: dateLine,
		commitIndex:    commitIdx,
	})

	// Blank line before message
	rows = append(rows, displayRow{
		kind:              RowKindCommitBody,
		fileIndex:         -1,
		isCommitBody:      true,
		commitBodyLine:    "",
		commitBodyIsBlank: true,
		commitIndex:       commitIdx,
	})

	// Subject line (first line of message, indented)
	if info.Subject != "" {
		rows = append(rows, displayRow{
			kind:           RowKindCommitBody,
			fileIndex:      -1,
			isCommitBody:   true,
			commitBodyLine: "    " + info.Subject,
			commitIndex:    commitIdx,
		})
	}

	// Body lines (rest of message, indented)
	if info.Body != "" {
		// Blank line between subject and body (traditional git log format)
		rows = append(rows, displayRow{
			kind:              RowKindCommitBody,
			fileIndex:         -1,
			isCommitBody:      true,
			commitBodyLine:    "",
			commitBodyIsBlank: true,
			commitIndex:       commitIdx,
		})
		bodyLines := strings.Split(info.Body, "\n")
		for _, line := range bodyLines {
			// Skip empty lines at the start of body
			if line == "" {
				rows = append(rows, displayRow{
					kind:              RowKindCommitBody,
					fileIndex:         -1,
					isCommitBody:      true,
					commitBodyLine:    "",
					commitBodyIsBlank: true,
					commitIndex:       commitIdx,
				})
			} else {
				rows = append(rows, displayRow{
					kind:           RowKindCommitBody,
					fileIndex:      -1,
					isCommitBody:   true,
					commitBodyLine: "    " + line,
					commitIndex:    commitIdx,
				})
			}
		}
	}

	// Trailing blank line for separation from files
	rows = append(rows, displayRow{
		kind:              RowKindCommitBody,
		fileIndex:         -1,
		isCommitBody:      true,
		commitBodyLine:    "",
		commitBodyIsBlank: true,
		commitIndex:       commitIdx,
	})

	return rows
}

// buildCommitBodyRowsSkipFirstBlank creates body rows but skips the first blank line.
// The first blank is replaced by the commit header bottom border.
func (m Model) buildCommitBodyRowsSkipFirstBlank(commit *sidebyside.CommitSet, commitIdx int) []displayRow {
	rows := m.buildCommitBodyRows(commit, commitIdx)
	if len(rows) > 0 && rows[0].commitBodyIsBlank {
		return rows[1:]
	}
	return rows
}

// buildCommitInfoRows creates the foldable commit info node rows.
// This node appears as the first child under a commit, before any files.
// - CommitFolded: returns empty (node hidden)
// - CommitNormal: returns only header row ("commit abc1234")
// - CommitExpanded: returns header + body rows (Author, Date, message)
func (m Model) buildCommitInfoRows(commit *sidebyside.CommitSet, commitIdx int) []displayRow {
	var rows []displayRow
	info := commit.Info

	// No info rows if commit has no metadata or is folded
	if !info.HasMetadata() || commit.FoldLevel == sidebyside.CommitFolded {
		return rows
	}

	// Build tree path for the details node.
	// Details is a sibling to the files under this commit, not their parent.
	// Note: Commits are tree roots with no siblings, so we don't include commit
	// level in Ancestors (no continuation line needed above details/files).

	// Check if this commit has any files - if so, details is not the last child
	startIdx := 0
	if commitIdx < len(m.commitFileStarts) {
		startIdx = m.commitFileStarts[commitIdx]
	}
	endIdx := len(m.files)
	if commitIdx+1 < len(m.commitFileStarts) {
		endIdx = m.commitFileStarts[commitIdx+1]
	}
	hasFiles := startIdx < endIdx

	// Details level - grey style (Color 7) for subdued appearance
	detailsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	detailsLevel := TreeLevel{
		IsLast:   !hasFiles,                                   // details is last only if no files follow
		IsFolded: commit.FoldLevel == sidebyside.CommitNormal, // folded when just showing header
		Style:    detailsStyle,
		Depth:    0, // depth 0 since commit is the root (not in tree)
	}

	detailsTreePath := TreePath{
		Ancestors: []TreeLevel{}, // no ancestors - commit is root, not a tree node
		Current:   &detailsLevel,
	}

	// Header text for the commit-info node
	headerText := "details"

	// Calculate header box width for borders
	treePrefixWidth := treeWidth(0, true) + 1
	iconPartWidth := treePrefixWidth + 2
	headerBoxWidth := iconPartWidth + len(headerText) + 2

	// Commit info header (child node showing commit metadata)
	rows = append(rows, displayRow{
		kind:               RowKindCommitInfoHeader,
		fileIndex:          -1,
		isCommitInfoHeader: true,
		header:             headerText,
		commitFoldLevel:    commit.FoldLevel,
		commitIndex:        commitIdx,
		treePath:           detailsTreePath,
		headerBoxWidth:     headerBoxWidth,
		treePrefixWidth:    treePrefixWidth,
	})

	// If parent is CommitNormal, only show header (info folded)
	if commit.FoldLevel == sidebyside.CommitNormal {
		return rows
	}

	// CommitExpanded: show full info body

	// Body rows have detailsLevel as ancestor (shows continuation │)
	bodyTreePath := TreePath{
		Ancestors: []TreeLevel{detailsLevel},
		Current:   nil,
	}

	// Bottom border after header - uses bodyTreePath for tree continuation
	rows = append(rows, displayRow{
		kind:                     RowKindCommitInfoBottomBorder,
		fileIndex:                -1,
		isCommitInfoBottomBorder: true,
		commitIndex:              commitIdx,
		headerBoxWidth:           headerBoxWidth,
		treePrefixWidth:          treePrefixWidth,
		treePath:                 bodyTreePath,
	})

	// Blank line after header border
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "",
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// commit <full sha>
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "commit " + info.SHA,
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Author line
	authorLine := "Author: " + info.Author
	if info.Email != "" {
		authorLine += " <" + info.Email + ">"
	}
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   authorLine,
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Date line
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "Date:   " + info.Date,
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Blank line before message
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "",
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Subject line (indented)
	if info.Subject != "" {
		rows = append(rows, displayRow{
			kind:             RowKindCommitInfoBody,
			fileIndex:        -1,
			isCommitInfoBody: true,
			commitInfoLine:   "    " + info.Subject,
			commitIndex:      commitIdx,
			treePath:         bodyTreePath,
		})
	}

	// Body lines (if present)
	if info.Body != "" {
		// Blank line between subject and body
		rows = append(rows, displayRow{
			kind:             RowKindCommitInfoBody,
			fileIndex:        -1,
			isCommitInfoBody: true,
			commitInfoLine:   "",
			commitIndex:      commitIdx,
			treePath:         bodyTreePath,
		})
		bodyLines := strings.Split(info.Body, "\n")
		for _, line := range bodyLines {
			indentedLine := ""
			if line != "" {
				indentedLine = "    " + line
			}
			rows = append(rows, displayRow{
				kind:             RowKindCommitInfoBody,
				fileIndex:        -1,
				isCommitInfoBody: true,
				commitInfoLine:   indentedLine,
				commitIndex:      commitIdx,
				treePath:         bodyTreePath,
			})
		}
	}

	// Trailing blank line for the file border to draw into
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "",
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	return rows
}

// structuralDiffMaxContentWidth calculates the maximum content width needed for
// structural diff lines for a file. This is used to expand the header box width
// if structural diff entries are wider than the filename. Returns 0 if no
// structural diff or no changes. The width is the content after the icon prefix:
// extraIndent(2) + symbol(1) + space(1) + kind + space(1) + name + stats = 5 + kind_len + name_len + stats_len
// For child items, add 2 more for child indent.
func (m Model) structuralDiffMaxContentWidth(fileIdx int) int {
	fs := m.structureMaps[fileIdx]
	if fs == nil || fs.StructuralDiff == nil {
		return 0
	}

	diff := fs.StructuralDiff
	if !diff.HasChanges() {
		return 0
	}

	changes := diff.ChangedOnly()
	if len(changes) == 0 {
		return 0
	}

	// Calculate max widths for line count stats
	maxAddLen, maxRemLen := 0, 0
	for _, c := range changes {
		if c.LinesAdded > 0 {
			w := len(fmt.Sprintf("%d", c.LinesAdded))
			if w > maxAddLen {
				maxAddLen = w
			}
		}
		if c.LinesRemoved > 0 {
			w := len(fmt.Sprintf("%d", c.LinesRemoved))
			if w > maxRemLen {
				maxRemLen = w
			}
		}
	}

	// Stats width: " N M" = 1 + maxAddLen + 1 + maxRemLen (if both present)
	statsWidth := 0
	if maxAddLen > 0 {
		statsWidth += 1 + maxAddLen // " N"
	}
	if maxRemLen > 0 {
		statsWidth += 1 + maxRemLen // " M"
	}

	maxWidth := 0

	// Build tree structure to identify children (same logic as buildStructuralDiffRows)
	methodsAssigned := make(map[int]bool)

	// First pass: find types and their children
	for i, c := range changes {
		entry := c.Entry()
		if entry == nil {
			continue
		}
		if entry.Kind == "type" || entry.Kind == "class" {
			// Width for parent: extraIndent(2) + symbol(1) + space(1) + kind + space(1) + name + stats
			// extraIndent = symbolPrefix (11+numDigits) - iconPartWidth (9+numDigits) = 2
			width := 2 + 1 + 1 + len(entry.Kind) + 1 + len(entry.Name) + statsWidth
			if width > maxWidth {
				maxWidth = width
			}

			// Find children
			for j, other := range changes {
				if i == j {
					continue
				}
				otherEntry := other.Entry()
				if otherEntry == nil {
					continue
				}
				if otherEntry.Kind == "func" || otherEntry.Kind == "def" {
					typeStart, typeEnd := entry.StartLine, entry.EndLine
					otherStart := otherEntry.StartLine
					if otherStart >= typeStart && otherStart <= typeEnd {
						// Width for child: extraIndent(2) + symbol(1) + space(1) + childIndent(2) + kind + space(1) + name + stats
						childWidth := 2 + 1 + 1 + 2 + len(otherEntry.Kind) + 1 + len(otherEntry.Name) + statsWidth
						if childWidth > maxWidth {
							maxWidth = childWidth
						}
						methodsAssigned[j] = true
					}
				}
			}
			methodsAssigned[i] = true
		}
	}

	// Second pass: remaining top-level items
	for i, c := range changes {
		if !methodsAssigned[i] {
			entry := c.Entry()
			if entry == nil {
				continue
			}
			// Width for top-level: extraIndent(2) + symbol(1) + space(1) + kind + space(1) + name + stats
			width := 2 + 1 + 1 + len(entry.Kind) + 1 + len(entry.Name) + statsWidth
			if width > maxWidth {
				maxWidth = width
			}
		}
	}

	return maxWidth
}

// buildStructuralDiffRows creates display rows for the structural diff summary.
// Shows which functions, methods, and types were added, modified, or deleted.
// The rows are rendered inside the file header box, so they receive the same
// headerBoxWidth setting as the header line.
func (m Model) buildStructuralDiffRows(fileIdx int, headerBoxWidth int, isLastFileInCommit bool, isFileFolded bool) []displayRow {
	fs := m.structureMaps[fileIdx]
	if fs == nil || fs.StructuralDiff == nil {
		return nil
	}

	diff := fs.StructuralDiff
	if !diff.HasChanges() {
		return nil
	}

	// Get only the changed elements
	changes := diff.ChangedOnly()
	if len(changes) == 0 {
		return nil
	}

	// Build tree path for structural diff rows (content-level, inside file)
	structuralTreePath := m.buildFileTreePath(fileIdx, isLastFileInCommit, isFileFolded, TreeRowPreview)

	var rows []displayRow

	// Minimal prefix for symbol - just enough spacing for visual separation
	// With tight tree continuation, content starts right after │
	symbolPrefix := ""  // symbol starts immediately after tree continuation
	childPrefix := "  " // 2 extra spaces for child indent

	// Build a tree structure: top-level items and their children
	// Types/classes can contain methods
	type treeNode struct {
		change   structure.ElementChange
		children []structure.ElementChange
	}

	// Group methods under their parent types (by checking line containment)
	var topLevel []treeNode
	methodsAssigned := make(map[int]bool) // track which changes are assigned as children

	// First pass: find all types/classes that could be parents
	for i, c := range changes {
		entry := c.Entry()
		if entry == nil {
			continue
		}
		if entry.Kind == "type" || entry.Kind == "class" {
			node := treeNode{change: c}
			// Find methods that are within this type's range
			for j, other := range changes {
				if i == j {
					continue
				}
				otherEntry := other.Entry()
				if otherEntry == nil {
					continue
				}
				// Check if this is a method/function within the type's lines
				if otherEntry.Kind == "func" || otherEntry.Kind == "def" {
					// Use the entry that has line info (prefer new, fall back to old)
					typeStart, typeEnd := entry.StartLine, entry.EndLine
					otherStart := otherEntry.StartLine

					if otherStart >= typeStart && otherStart <= typeEnd {
						node.children = append(node.children, other)
						methodsAssigned[j] = true
					}
				}
			}
			topLevel = append(topLevel, node)
			methodsAssigned[i] = true
		}
	}

	// Second pass: add remaining items as top-level
	for i, c := range changes {
		if !methodsAssigned[i] {
			topLevel = append(topLevel, treeNode{change: c})
		}
	}

	// Calculate max widths for alignment of line counts
	maxAddLen, maxRemLen := 0, 0
	for _, node := range topLevel {
		if node.change.LinesAdded > 0 {
			w := len(fmt.Sprintf("%d", node.change.LinesAdded))
			if w > maxAddLen {
				maxAddLen = w
			}
		}
		if node.change.LinesRemoved > 0 {
			w := len(fmt.Sprintf("%d", node.change.LinesRemoved))
			if w > maxRemLen {
				maxRemLen = w
			}
		}
		for _, child := range node.children {
			if child.LinesAdded > 0 {
				w := len(fmt.Sprintf("%d", child.LinesAdded))
				if w > maxAddLen {
					maxAddLen = w
				}
			}
			if child.LinesRemoved > 0 {
				w := len(fmt.Sprintf("%d", child.LinesRemoved))
				if w > maxRemLen {
					maxRemLen = w
				}
			}
		}
	}

	// Render tree
	for _, node := range topLevel {
		c := node.change
		entry := c.Entry()
		if entry == nil {
			continue
		}

		// Format: "<prefix>~ type MyStruct" where symbol aligns with file status
		symbol := c.Kind.Symbol()
		line := symbolPrefix + symbol + " " + entry.Kind + " " + entry.Name

		rows = append(rows, displayRow{
			kind:                    RowKindStructuralDiff,
			fileIndex:               fileIdx,
			isStructuralDiff:        true,
			structuralDiffLine:      line,
			structuralDiffAdded:     c.LinesAdded,
			structuralDiffRemoved:   c.LinesRemoved,
			structuralDiffMaxAddLen: maxAddLen,
			structuralDiffMaxRemLen: maxRemLen,
			headerBoxWidth:          headerBoxWidth,
			isLastFileInCommit:      isLastFileInCommit,
			isFileFolded:            isFileFolded,
			treePath:                structuralTreePath,
		})

		// Add children (methods within types) with extra indentation
		for _, child := range node.children {
			childEntry := child.Entry()
			if childEntry == nil {
				continue
			}
			childSymbol := child.Kind.Symbol()
			childLine := childPrefix + childSymbol + " " + childEntry.Kind + " " + childEntry.Name

			rows = append(rows, displayRow{
				kind:                    RowKindStructuralDiff,
				fileIndex:               fileIdx,
				isStructuralDiff:        true,
				structuralDiffLine:      childLine,
				structuralDiffAdded:     child.LinesAdded,
				structuralDiffRemoved:   child.LinesRemoved,
				structuralDiffMaxAddLen: maxAddLen,
				structuralDiffMaxRemLen: maxRemLen,
				headerBoxWidth:          headerBoxWidth,
				isLastFileInCommit:      isLastFileInCommit,
				isFileFolded:            isFileFolded,
				treePath:                structuralTreePath,
			})
		}
	}

	// No trailing blank here - margins are handled at the file level in buildFileRows

	return rows
}

// renderCommitBodyRow renders a single line of the commit body.
func (m Model) renderCommitBodyRow(row displayRow, isCursorRow bool) string {
	// Style the content
	var content string
	if strings.HasPrefix(row.commitBodyLine, "commit ") {
		// SHA line - "commit" in normal text, SHA in yellow
		shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		sha := strings.TrimPrefix(row.commitBodyLine, "commit ")
		content = "commit " + shaStyle.Render(sha)
	} else {
		content = row.commitBodyLine
	}

	// Cursor handling with 1-char bg highlight (like file headers)
	if isCursorRow && m.focused {
		// Format: arrow + space + [1 char bg] + space + content
		styledGutter := cursorStyle.Render(" ")
		return cursorArrowStyle.Render("▶") + " " + styledGutter + " " + content
	}

	if isCursorRow && !m.focused {
		// Unfocused: outline arrow, no background highlight
		return unfocusedCursorArrowStyle.Render("▷") + "   " + content
	}

	// Non-cursor: 2-space prefix + 2-space indent
	return "    " + content
}

// renderStructuralDiffRow renders a single line of the structural diff summary.
// The row is rendered inside the header box with proper padding and border.
// Structural diff rows are always shown under folded files, so borders are always dark.
func (m Model) renderStructuralDiffRow(row displayRow, isCursorRow bool) string {
	content := row.structuralDiffLine
	headerBoxWidth := row.headerBoxWidth

	// Tree prefix using tight spacing for compact content
	treeContinuation := renderTreePrefixTight(row.treePath)

	// Extract parts: prefix (spaces for child indent), symbol, rest (kind + name)
	// Format: "" + symbol + " kind name" (top-level) or "  " + symbol + " kind name" (child)
	var prefix, symbol, rest string
	// Find first non-space character (the symbol)
	symbolPos := 0
	for i, c := range content {
		if c != ' ' {
			symbolPos = i
			break
		}
	}
	if symbolPos < len(content) {
		prefix = content[:symbolPos]
		symbol = string(content[symbolPos])
		rest = content[symbolPos+1:]
	} else {
		// Fallback for malformed content
		prefix = content
		symbol = ""
		rest = ""
	}

	// Style the symbol based on change kind (use dark color variants)
	darkAddedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	darkRemovedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	var styledSymbol string
	switch symbol {
	case "+":
		styledSymbol = darkAddedStyle.Render("+")
	case "-":
		styledSymbol = darkRemovedStyle.Render("-")
	case "~":
		styledSymbol = changedStyle.Render("~")
	default:
		styledSymbol = symbol
	}

	// Style the rest: " kind name" -> space + styled kind (fg=8) + space + styled name (fg=7)
	kindStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	var styledRest string
	if strings.HasPrefix(rest, " ") {
		// Parse: " kind name"
		trimmed := strings.TrimPrefix(rest, " ")
		parts := strings.SplitN(trimmed, " ", 2)
		if len(parts) == 2 {
			styledRest = " " + kindStyle.Render(parts[0]) + " " + nameStyle.Render(parts[1])
		} else {
			styledRest = rest // fallback
		}
	} else {
		styledRest = rest // fallback
	}

	// Format line count stats (e.g., "3 1") - placed after symbol, before kind
	// Show stats if any element in the file has stats (for alignment)
	// Numbers only (no +/- prefix), colors distinguish add vs remove
	var statsStr string
	var statsWidth int
	if row.structuralDiffMaxAddLen > 0 || row.structuralDiffMaxRemLen > 0 {
		// Build stats with padding for alignment
		addPart := ""
		remPart := ""
		if row.structuralDiffMaxAddLen > 0 {
			if row.structuralDiffAdded > 0 {
				addPart = fmt.Sprintf("%*d", row.structuralDiffMaxAddLen, row.structuralDiffAdded)
			} else {
				addPart = strings.Repeat(" ", row.structuralDiffMaxAddLen)
			}
		}
		if row.structuralDiffMaxRemLen > 0 {
			if row.structuralDiffRemoved > 0 {
				remPart = fmt.Sprintf("%*d", row.structuralDiffMaxRemLen, row.structuralDiffRemoved)
			} else {
				remPart = strings.Repeat(" ", row.structuralDiffMaxRemLen)
			}
		}

		// Style them with dark colors
		addStyled := darkAddedStyle.Render(addPart)
		remStyled := darkRemovedStyle.Render(remPart)

		if addPart != "" && remPart != "" {
			statsStr = " " + addStyled + " " + remStyled
			statsWidth = 1 + len(addPart) + 1 + len(remPart) // space + add + space + rem
		} else if addPart != "" {
			statsStr = " " + addStyled
			statsWidth = 1 + len(addPart)
		} else if remPart != "" {
			statsStr = " " + remStyled
			statsWidth = 1 + len(remPart)
		}
	}

	// Calculate padding to reach headerBoxWidth (based on original content width)
	// Calculate padding to reach headerBoxWidth, plus 2 extra to align with header border
	// (the header has a 2-space prefix before headerBoxWidth content)
	originalWidth := len(content) + statsWidth // All ASCII so len() works
	paddingNeeded := headerBoxWidth - originalWidth + 2
	padding := ""
	if paddingNeeded > 0 {
		padding = strings.Repeat(" ", paddingNeeded)
	}

	// Border style - always dark for structural diff rows (shown under folded files)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("0"))

	// Build the line content (stats go after symbol, before kind/name)
	// With tight spacing, prefix is minimal (0 for top-level, 2 for children)
	var result string
	if isCursorRow && m.focused {
		result = cursorArrowStyle.Render("▶") + treeContinuation[1:] + prefix + styledSymbol + statsStr + styledRest + padding
	} else if isCursorRow && !m.focused {
		result = unfocusedCursorArrowStyle.Render("▷") + treeContinuation[1:] + prefix + styledSymbol + statsStr + styledRest + padding
	} else {
		result = treeContinuation + prefix + styledSymbol + statsStr + styledRest + padding
	}

	// Add border (│) - always present but no trailing fill unlike header
	result += " " + borderStyle.Render("│")

	return result
}

// renderTopBar renders the top bar showing file info with a divider line below.
func (m Model) renderTopBar() string {
	info := m.StatusInfo()

	var lines []string

	// Commit info line (only if we have commit metadata)
	if m.hasCommitInfo() {
		commitLine := m.renderCommitLine()
		lines = append(lines, commitLine)
	}

	// File info line - show when:
	// - No commit info (file line contains total stats)
	// - OR cursor is on a file (info.CurrentFile > 0)
	if !m.hasCommitInfo() || info.CurrentFile > 0 {
		fileLine := m.renderFileLine(info)
		lines = append(lines, fileLine)
	}

	// Divider line using upper 1/8 block (dim, faint when unfocused)
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	if !m.focused {
		dividerStyle = dividerStyle.Faint(true)
	}
	divider := dividerStyle.Render(strings.Repeat("▔", m.width))
	lines = append(lines, divider)

	return strings.Join(lines, "\n")
}

// renderCommitLine renders the commit info line for the top bar.
// Shows fold icon, SHA, subject, and file stats for a compact display.
func (m *Model) renderCommitLine() string {
	commit := m.currentCommit()
	if commit == nil {
		return ""
	}
	commitInfo := commit.Info

	// Style for SHA (yellow/gold)
	shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	// Build commit line: ▶ ◐ a1b2c3d Subject line    N files +X -Y
	// Arrow shows when cursor is on any commit section (header or body)
	var prefix string
	if m.isOnCommitSection() {
		if m.focused {
			prefix = cursorArrowStyle.Render("▶") + " "
		} else {
			prefix = unfocusedCursorArrowStyle.Render("▷") + " "
		}
	} else {
		prefix = "  " // Same width as arrow + space
	}

	// Fold level icon: ◯ = folded, ◐ = normal, ● = expanded
	var foldIcon string
	switch commit.FoldLevel {
	case sidebyside.CommitFolded:
		foldIcon = "◯"
	case sidebyside.CommitNormal:
		foldIcon = "◐"
	case sidebyside.CommitExpanded:
		foldIcon = "●"
	}
	// Fold icon - always fg=8, no faint
	foldIconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	foldIconRendered := foldIconStyle.Render(foldIcon) + " "

	sha := shaStyle.Render(commitInfo.ShortSHA())
	subject := commitInfo.Subject

	// Calculate stats for the current commit's files only
	commitIdx := m.currentCommitIndex()
	var startIdx, endIdx int
	if len(m.commits) > 0 && len(m.commitFileStarts) > 0 {
		startIdx = m.commitFileStarts[commitIdx]
		endIdx = len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}
	} else {
		// Legacy mode: use all files
		startIdx = 0
		endIdx = len(m.files)
	}
	totalAdded := 0
	totalRemoved := 0
	for i := startIdx; i < endIdx; i++ {
		added, removed := countFileStats(m.files[i])
		totalAdded += added
		totalRemoved += removed
	}

	// Build right section: N files +X -Y
	var rightText string
	var rightSection string
	fileCount := endIdx - startIdx
	if fileCount == 1 {
		rightText = "1 file"
	} else {
		rightText = fmt.Sprintf("%d files", fileCount)
	}
	if totalAdded > 0 || totalRemoved > 0 {
		addedText := fmt.Sprintf("+%d", totalAdded)
		removedText := fmt.Sprintf("-%d", totalRemoved)
		rightText += " " + addedText + " " + removedText
		rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText[:len(rightText)-len(addedText)-len(removedText)-2]) + " " + addedStyle.Render(addedText) + " " + removedStyle.Render(removedText)
	} else {
		rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText)
	}
	rightWidth := len(rightText)

	// Calculate available width for subject
	// Layout: prefix(2) + foldIcon(2) + sha(7) + sep(1) + subject + padding(1+) + rightSection
	fixedWidth := 2 + 2 + 7 + 1 + 1 + rightWidth
	availableWidth := m.width - fixedWidth
	if availableWidth < 0 {
		availableWidth = 0
	}

	// Truncate subject if needed
	if len(subject) > availableWidth {
		if availableWidth > 3 {
			subject = subject[:availableWidth-3] + "..."
		} else if availableWidth > 0 {
			subject = subject[:availableWidth]
		} else {
			subject = ""
		}
	}

	// Calculate padding between subject and right section
	padding := m.width - 2 - 2 - 7 - 1 - len(subject) - rightWidth
	if padding < 1 {
		padding = 1
	}

	return prefix + foldIconRendered + sha + " " + subject + strings.Repeat(" ", padding) + rightSection
}

// renderFileLine renders the file info line for the top bar.
func (m Model) renderFileLine(info StatusInfo) string {
	// Only show file info when cursor is on a file (not on commit header)
	var content string
	var foldIcon string
	var fileNum string
	var leftSectionWidth int
	if info.CurrentFile > 0 {
		content = m.formatStatusFileInfo(info)

		// Fold icon
		foldIcon = m.foldLevelIcon(info.FoldLevel)

		// File number with # prefix
		_, fileCounterStyle := fileStatusIndicator(FileStatus(info.FileStatus))
		totalWidth := len(fmt.Sprintf("%d", info.TotalFiles))
		fileNumText := fmt.Sprintf("#%0*d", totalWidth, info.CurrentFile)
		fileNum = fileCounterStyle.Render(fileNumText) + " "

		// Layout: indent(3) + icon(1) + space(1) + #fileNum + space(1)
		leftSectionWidth = 3 + 1 + 1 + 1 + totalWidth + 1
	}

	// Right section: N files +123 -123 (only when no commit info - stats move to commit line otherwise)
	var rightText string
	var rightSection string
	var rightWidth int
	if !m.hasCommitInfo() {
		totalAdded := 0
		totalRemoved := 0
		for _, fp := range m.files {
			added, removed := countFileStats(fp)
			totalAdded += added
			totalRemoved += removed
		}

		fileCount := len(m.files)
		if fileCount == 1 {
			rightText = "1 file"
		} else {
			rightText = fmt.Sprintf("%d files", fileCount)
		}
		if totalAdded > 0 || totalRemoved > 0 {
			addedText := fmt.Sprintf("+%d", totalAdded)
			removedText := fmt.Sprintf("-%d", totalRemoved)
			rightText += " " + addedText + " " + removedText
			rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText[:len(rightText)-len(addedText)-len(removedText)-2]) + " " + addedStyle.Render(addedText) + " " + removedStyle.Render(removedText)
		} else {
			rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText)
		}
		rightWidth = len(rightText)
	}

	// Leading arrow indicator - only show when NOT on commit section (or no commit info)
	// When on commit header or body, the arrow shows on the commit line instead
	var prefix string
	showArrow := !m.hasCommitInfo() || !m.isOnCommitSection()
	if showArrow {
		if m.focused {
			prefix = cursorArrowStyle.Render("▶") + " "
		} else {
			prefix = unfocusedCursorArrowStyle.Render("▷") + " "
		}
	} else {
		prefix = "  " // Same width as arrow + space
	}

	// Calculate widths for padding
	// Layout: prefix(2) + leftSection + content + padding + rightSection
	prefixWidth := 2 // "▶ " or "  "
	contentWidth := lipgloss.Width(content)
	padding := m.width - prefixWidth - leftSectionWidth - contentWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	// Build the left section: indent(3) + icon + space + fileNum
	var leftSection string
	if info.CurrentFile > 0 {
		// Style fold icon with fg=8 to match commit header fold icon
		foldIconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		leftSection = "  " + foldIconStyle.Render(foldIcon) + " " + fileNum
	}

	return prefix + leftSection + content + strings.Repeat(" ", padding) + rightSection
}

// formatRelativeDate converts an ISO 8601 date string to a relative format like "2d ago".
func formatRelativeDate(isoDate string) string {
	if isoDate == "" {
		return ""
	}

	// Try to parse ISO 8601 format
	t, err := time.Parse(time.RFC3339, isoDate)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", isoDate)
		if err != nil {
			return isoDate // Return as-is if can't parse
		}
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1w ago"
		}
		return fmt.Sprintf("%dw ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1mo ago"
		}
		return fmt.Sprintf("%dmo ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1y ago"
		}
		return fmt.Sprintf("%dy ago", years)
	}
}

// formatShortRelativeDate returns abbreviated relative time without "ago".
// Used in commit header rows for compact display.
// Format: "now", "1m", "4h", "2d", "3w", "1mo", "1y"
func formatShortRelativeDate(isoDate string) string {
	if isoDate == "" {
		return ""
	}

	// Try to parse ISO 8601 format
	t, err := time.Parse(time.RFC3339, isoDate)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", isoDate)
		if err != nil {
			// Try git log format: "Mon Jan 15 10:30:00 2024 -0500"
			t, err = time.Parse("Mon Jan 2 15:04:05 2006 -0700", isoDate)
			if err != nil {
				return isoDate // Return as-is if can't parse
			}
		}
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(diff.Hours()/24/7))
	case diff < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(diff.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy", int(diff.Hours()/24/365))
	}
}

// renderStatusBar renders the status bar at the bottom of the screen.
// This now only contains the less-style indicator (file info is in top bar).
func (m Model) renderStatusBar() string {
	// In comment mode, show comment prompt
	if m.commentMode {
		return m.renderCommentPrompt()
	}

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
	// Use unfocused style when terminal loses focus
	var styledLessIndicator string
	if m.focused {
		styledLessIndicator = statusStyle.Render(" " + lessIndicator)
	} else {
		styledLessIndicator = unfocusedStatusStyle.Render(" " + lessIndicator)
	}

	// Status message (echo area) - shown after less indicator
	var statusMsg string
	var statusMsgWidth int
	if m.statusMessage != "" {
		statusMsgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		statusMsg = " " + statusMsgStyle.Render(m.statusMessage)
		statusMsgWidth = 1 + displayWidth(m.statusMessage)
	}

	// Loading indicator (grey, shown when any files are loading)
	var loadingIndicator string
	var loadingWidth int
	if m.hasAnyLoadingFiles() {
		loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		loadingIndicator = " " + loadingStyle.Render(m.spinner.View()+" Loading...")
		loadingWidth = 1 + 1 + len(" Loading...") // space + spinner + text
	}

	// Pager mode indicator (right-aligned)
	var pagerIndicator string
	if m.pagerMode {
		pagerIndicator = "PAGER"
	}

	// Debug stats (right-aligned, before pager indicator)
	var debugStats string
	var debugWidth int
	if m.debugMode {
		debugStats, debugWidth = m.formatDebugStats()
	}

	// Combine: reversed_less_indicator + status_msg + loading + padding + debug_stats + pager_indicator
	content := styledLessIndicator + statusMsg + loadingIndicator
	contentWidth := displayWidth(" "+lessIndicator) + statusMsgWidth + loadingWidth
	pagerWidth := displayWidth(pagerIndicator)

	// Calculate padding between content and right-side indicators
	rightWidth := debugWidth + pagerWidth
	if debugWidth > 0 && pagerWidth > 0 {
		rightWidth++ // space between debug and pager
	}
	padding := m.width - contentWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	// Build right side
	var rightSide string
	if debugStats != "" && pagerIndicator != "" {
		rightSide = debugStats + " " + pagerIndicator
	} else {
		rightSide = debugStats + pagerIndicator
	}

	return content + strings.Repeat(" ", padding) + rightSide
}

// formatDebugStats returns formatted memory and goroutine stats for debug mode.
// Returns (styled string, display width).
func (m Model) formatDebugStats() (string, int) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	heapMB := float64(mem.Alloc) / 1024 / 1024
	goroutines := runtime.NumGoroutine()

	// Build raw values for width calculation
	heapVal := fmt.Sprintf("%.1fMB", heapMB)
	grVal := fmt.Sprintf("%d", goroutines)
	// "Heap: XXX GR: YYY"
	rawWidth := displayWidth("Heap: " + heapVal + " GR: " + grVal)

	// Build styled output
	heapLabel := debugLabelStyle.Render("Heap:")
	heapValue := debugValueStyle.Render(heapVal)
	grLabel := debugLabelStyle.Render("GR:")
	grValue := debugValueStyle.Render(grVal)

	return heapLabel + " " + heapValue + " " + grLabel + " " + grValue, rawWidth
}

// formatStatusFileInfo formats the file info for the status bar.
// Format: statusIcon fileName +N -M (icon handled separately by caller)
func (m Model) formatStatusFileInfo(info StatusInfo) string {
	// Get status indicator - shows spinner if file is loading
	fileIndex := info.CurrentFile - 1 // CurrentFile is 1-based
	styledStatus := m.fileStatusSymbolStyled(fileIndex, FileStatus(info.FileStatus))

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

	// Format breadcrumbs (dimmed, after stats)
	var breadcrumbs string
	if info.Breadcrumbs != "" {
		breadcrumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		breadcrumbs = "  " + breadcrumbStyle.Render(info.Breadcrumbs)
	}

	return styledStatus + " " + info.FileName + stats + breadcrumbs
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

// renderCommentPrompt renders the comment input as a multi-line prompt.
func (m Model) renderCommentPrompt() string {
	// Split input into lines
	lines := strings.Split(m.commentInput, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	// Find which line the cursor is on and the position within that line
	cursorLine := 0
	cursorCol := m.commentCursor
	pos := 0
	for i, line := range lines {
		lineEnd := pos + len(line)
		if i < len(lines)-1 {
			lineEnd++ // account for newline
		}
		// Use < so cursor right after newline is on the next line
		if m.commentCursor < lineEnd || i == len(lines)-1 {
			cursorLine = i
			cursorCol = m.commentCursor - pos
			break
		}
		pos = lineEnd
	}

	// Calculate visible range based on scroll
	maxVisible := m.commentMaxVisibleLines()
	startLine := m.commentScroll
	endLine := startLine + maxVisible
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Check if there's content above/below the visible area
	hasMoreAbove := startLine > 0
	hasMoreBelow := endLine < len(lines)

	var result []string

	// Show scroll indicator if there's content above
	if hasMoreAbove {
		indicator := fmt.Sprintf(" ↑ %d more line", startLine)
		if startLine > 1 {
			indicator += "s"
		}
		indicatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		indicatorPadding := m.width - lipgloss.Width(indicator)
		if indicatorPadding < 0 {
			indicatorPadding = 0
		}
		result = append(result, indicatorStyle.Render(indicator)+strings.Repeat(" ", indicatorPadding))
	}

	// Render visible lines of input
	for i := startLine; i < endLine; i++ {
		line := lines[i]
		var prefix string
		if i == cursorLine {
			prefix = " > " // cursor line gets the main prompt
		} else {
			prefix = " . " // other lines get continuation indicator
		}

		var renderedLine string
		if i == cursorLine {
			// This line has the cursor
			if cursorCol > len(line) {
				cursorCol = len(line)
			}

			beforeCursor := line[:cursorCol]
			var cursorChar string
			var afterCursor string

			if cursorCol < len(line) {
				runes := []rune(line[cursorCol:])
				cursorChar = string(runes[0])
				afterCursor = string(runes[1:])
			} else {
				cursorChar = " "
				afterCursor = ""
			}

			styledCursor := statusStyle.Render(cursorChar)
			renderedLine = prefix + beforeCursor + styledCursor + afterCursor
		} else {
			renderedLine = prefix + line
		}

		// Pad to full width (use lipgloss.Width to handle ANSI escape codes correctly)
		lineWidth := lipgloss.Width(renderedLine)
		padding := m.width - lineWidth
		if padding < 0 {
			padding = 0
		}
		result = append(result, renderedLine+strings.Repeat(" ", padding))
	}

	// Show scroll indicator if there's content below
	if hasMoreBelow {
		remaining := len(lines) - endLine
		indicator := fmt.Sprintf(" ↓ %d more line", remaining)
		if remaining > 1 {
			indicator += "s"
		}
		indicatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		indicatorPadding := m.width - lipgloss.Width(indicator)
		if indicatorPadding < 0 {
			indicatorPadding = 0
		}
		result = append(result, indicatorStyle.Render(indicator)+strings.Repeat(" ", indicatorPadding))
	}

	// Add help line at the bottom
	help := " (C-j to submit, C-c to cancel)"
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpPadding := m.width - lipgloss.Width(help)
	if helpPadding < 0 {
		helpPadding = 0
	}
	result = append(result, helpStyle.Render(help)+strings.Repeat(" ", helpPadding))

	return strings.Join(result, "\n")
}

// highlightSearchInVisible highlights search matches in visible text.
// Searches on-demand in the visible text and highlights matches.
// isCursorRow indicates if this row is at the cursor position.
// currentIdx is the index of the current match (0 = first match).
// side is which side is being rendered (0=new/left, 1=old/right), currentSide is which side has the current match.
func (m Model) highlightSearchInVisible(visible string, isCursorRow bool, currentIdx, side, currentSide int) string {
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

	// Find and highlight all occurrences
	var result strings.Builder
	lastEnd := 0
	matchIdx := 0

	for {
		idx := strings.Index(searchIn[lastEnd:], query)
		if idx == -1 {
			break
		}
		pos := lastEnd + idx

		// Add text before match
		result.WriteString(visible[lastEnd:pos])

		// Add highlighted match
		end := pos + len(m.searchQuery)
		if end > len(visible) {
			end = len(visible)
		}

		// Determine if this is the current match (must match both index and side)
		isCurrent := isCursorRow && matchIdx == currentIdx && side == currentSide

		matchText := visible[pos:end]
		if isCurrent {
			result.WriteString(searchCurrentMatchStyle.Render(matchText))
		} else {
			result.WriteString(searchMatchStyle.Render(matchText))
		}
		lastEnd = end
		matchIdx++
	}

	// Add remaining text
	if lastEnd < len(visible) {
		result.WriteString(visible[lastEnd:])
	}

	return result.String()
}

// countFileStats returns the number of added and removed lines in a file.
// Uses pre-computed totals from diff parsing, which are accurate even when
// the file was truncated due to size limits. Falls back to counting from
// Pairs if totals aren't set (e.g., in tests).
// For binary files, returns +1/-1 style counts to indicate presence of changes.
func countFileStats(fp sidebyside.FilePair) (added, removed int) {
	// Binary files show +1/-1 to indicate presence of change
	if fp.IsBinary {
		if fp.OldPath == "/dev/null" {
			return 1, 0 // Binary file created
		}
		if fp.NewPath == "/dev/null" {
			return 0, 1 // Binary file deleted
		}
		return 1, 1 // Binary file changed
	}

	// Use pre-computed totals if available
	if fp.TotalAdded > 0 || fp.TotalRemoved > 0 {
		return fp.TotalAdded, fp.TotalRemoved
	}

	// Fall back to counting from Pairs (for tests or edge cases)
	for _, pair := range fp.Pairs {
		if pair.New.Type == sidebyside.Added {
			added++
		}
		if pair.Old.Type == sidebyside.Removed {
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

func formatFileHeader(fp sidebyside.FilePair) string {
	// Strip a/ and b/ prefixes if present
	old := strings.TrimPrefix(fp.OldPath, "a/")
	new := strings.TrimPrefix(fp.NewPath, "b/")

	if old == new || fp.OldPath == "/dev/null" {
		return new
	}
	if fp.NewPath == "/dev/null" {
		return old
	}
	// Show similarity percentage for renames/copies if available
	if fp.Similarity >= 0 {
		return fmt.Sprintf("%s → %s (%d%%)", old, new, fp.Similarity)
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
	FileStatusCopied   FileStatus = "copied"
	FileStatusModified FileStatus = "modified"
)

// fileStatusFromPair determines the status of a file from a FilePair.
func fileStatusFromPair(fp sidebyside.FilePair) FileStatus {
	// Added: old path is /dev/null
	if fp.OldPath == "/dev/null" {
		return FileStatusAdded
	}
	// Deleted: new path is /dev/null
	if fp.NewPath == "/dev/null" {
		return FileStatusDeleted
	}
	// Explicit copy from git metadata
	if fp.IsCopy {
		return FileStatusCopied
	}
	// Explicit rename from git metadata
	if fp.IsRename {
		return FileStatusRenamed
	}
	// Renamed: paths differ after stripping a/ and b/ prefixes (fallback detection)
	old := strings.TrimPrefix(fp.OldPath, "a/")
	new := strings.TrimPrefix(fp.NewPath, "b/")
	if old != new {
		return FileStatusRenamed
	}
	// Modified: everything else
	return FileStatusModified
}

// renamedStyle is cyan for renamed/copied files.
var renamedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

// fileStatusIndicator returns the symbol and style for a file status.
// + (green) for added, - (red) for deleted, → (cyan) for renamed/copied, ~ (blue) for modified.
func fileStatusIndicator(status FileStatus) (symbol string, style lipgloss.Style) {
	switch status {
	case FileStatusAdded:
		return "+", addedStyle
	case FileStatusDeleted:
		return "-", removedStyle
	case FileStatusRenamed, FileStatusCopied:
		return "→", renamedStyle
	default: // FileStatusModified
		return "~", lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	}
}

// statsAddWidth returns the display width of just the addition portion "+N".
func statsAddWidth(added int) int {
	if added > 0 {
		return len(fmt.Sprintf("+%d", added))
	}
	return 0
}

// statsRemWidth returns the display width of just the removal portion "-N".
func statsRemWidth(removed int) int {
	if removed > 0 {
		return len(fmt.Sprintf("-%d", removed))
	}
	return 0
}

// statsCountWidth returns the display width of the count portion "+N -M" (without bar).
func statsCountWidth(added, removed, maxAddWidth int) int {
	width := 0
	if added > 0 || maxAddWidth > 0 {
		// Use the max add width for alignment
		if maxAddWidth > 0 {
			width += maxAddWidth
		} else {
			width += len(fmt.Sprintf("+%d", added))
		}
	}
	if removed > 0 {
		if width > 0 {
			width++ // space between +N and -M
		}
		width += len(fmt.Sprintf("-%d", removed))
	}
	return width
}

// formatColoredStatsBar returns the stats display with colored +/- counts.
// Returns empty string if no changes. Format: " +N -M"
// maxAddWidth/maxRemWidth are used to pad columns so they align across files.
func formatColoredStatsBar(added, removed, maxAddWidth, maxRemWidth int) string {
	// If no stats columns needed at all (no files have changes), return empty
	if maxAddWidth == 0 && maxRemWidth == 0 {
		return ""
	}

	var parts []string

	// Build addition string with padding for alignment
	if added > 0 {
		addStr := fmt.Sprintf("+%d", added)
		currentAddWidth := len(addStr)
		if maxAddWidth > currentAddWidth {
			addStr += strings.Repeat(" ", maxAddWidth-currentAddWidth)
		}
		parts = append(parts, addedStyle.Render(addStr))
	} else if maxAddWidth > 0 {
		// No additions but need to reserve space for alignment
		parts = append(parts, strings.Repeat(" ", maxAddWidth))
	}

	// Build removal string with padding for alignment
	if removed > 0 {
		remStr := fmt.Sprintf("-%d", removed)
		currentRemWidth := len(remStr)
		if maxRemWidth > currentRemWidth {
			remStr += strings.Repeat(" ", maxRemWidth-currentRemWidth)
		}
		parts = append(parts, removedStyle.Render(remStr))
	} else if maxRemWidth > 0 {
		// No removals but need to reserve space for alignment
		parts = append(parts, strings.Repeat(" ", maxRemWidth))
	}

	return " " + strings.Join(parts, " ")
}

// statsBarDisplayWidth returns the display width of the stats counts (without ANSI codes).
// This matches formatColoredStatsBar's output width with fixed column widths.
func statsBarDisplayWidth(maxAddWidth, maxRemWidth int) int {
	// If no stats columns needed at all (no files have changes), return 0
	if maxAddWidth == 0 && maxRemWidth == 0 {
		return 0
	}

	// Format: " +N__ -M__" (with padding to fixed widths)
	// Leading space
	width := 1

	// Addition column (always padded to maxAddWidth)
	width += maxAddWidth

	// Space between +N and -M (only when both exist)
	if maxAddWidth > 0 && maxRemWidth > 0 {
		width++
	}
	width += maxRemWidth

	return width
}

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

func (m Model) renderHeader(header string, foldLevel sidebyside.FoldLevel, headerMode HeaderMode, status FileStatus, added, removed, maxHeaderWidth, maxAddWidth, maxRemWidth, headerBoxWidth, fileIndex, rowIdx int, isCursorRow bool, treePath TreePath) string {
	// Calculate header width BEFORE applying search highlighting (ANSI codes affect width calculation)
	headerTextWidth := displayWidth(header)

	// Apply search highlighting if there's a query
	// Headers are always considered "side 0" for search purposes
	if m.searchQuery != "" {
		header = m.highlightSearchInVisible(header, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide())
	}

	// Get fold level icon and file status indicator
	// Shows spinner if file is loading
	icon := m.foldLevelIcon(foldLevel)
	_, fileStatusStyle := fileStatusIndicator(status) // for coloring file number and trailing fill
	styledStatus := m.fileStatusSymbolStyled(fileIndex, status)

	// File number with # prefix and leading zeros
	// Color matches the file status (green=added, red=deleted, blue=modified/renamed)
	// File numbers reset to 1 for each commit
	var totalFilesInCommit int
	var fileNumInCommit int
	if len(m.commits) > 0 && len(m.commitFileStarts) > 0 {
		commitIdx := m.commitForFile(fileIndex)
		startIdx := m.commitFileStarts[commitIdx]
		endIdx := len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}
		totalFilesInCommit = endIdx - startIdx
		fileNumInCommit = fileIndex - startIdx + 1
	} else {
		// Legacy mode: no commits, use global file index
		totalFilesInCommit = len(m.files)
		fileNumInCommit = fileIndex + 1
	}
	numDigits := len(fmt.Sprintf("%d", totalFilesInCommit))
	fileNum := fmt.Sprintf("#%0*d", numDigits, fileNumInCommit) // #01
	fileNumWidth := 1 + numDigits                               // # + digits

	// All headers use same format: indent + icon + fileNum + status + header + stats + │ + trailing
	statsBar := formatColoredStatsBar(added, removed, maxAddWidth, maxRemWidth)
	statsBarWidth := statsBarDisplayWidth(maxAddWidth, maxRemWidth)
	headerPadding := ""
	if maxHeaderWidth > headerTextWidth {
		headerPadding = strings.Repeat(" ", maxHeaderWidth-headerTextWidth)
	}

	// Calculate content width and pad to match headerBoxWidth
	// Layout: indent(3) + icon(1) + space(1) + fileNum + space(1) + status(1) + space(1) + header
	iconPartWidth := 3 + 1 + 1 + fileNumWidth + 1 + 1 + 1 // "   ◐ #01 ~ "
	contentWidth := iconPartWidth + headerTextWidth + len(headerPadding) + statsBarWidth
	boxPadding := ""
	if headerBoxWidth > contentWidth {
		boxPadding = strings.Repeat(" ", headerBoxWidth-contentWidth)
	}

	// Style the header text - but if search highlighting was applied, don't override it
	// (search highlighting sets fg=0 which would be overridden by headerStyle's fg=15)
	styledHeader := headerStyle.Render(" " + header + headerPadding)
	if m.searchQuery != "" {
		// Search highlighting was applied; don't wrap with headerStyle to preserve fg color
		styledHeader = " " + header + headerPadding
	}

	// Style the fold icon with fg=8 (same as commit header), fg=15 when cursor is on row
	iconColor := "8"
	if isCursorRow {
		iconColor = "15"
	}
	iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(iconColor))
	styledIcon := iconStyle.Render(icon)

	// Tree prefix using the generic tree infrastructure
	treeLine := renderTreePrefix(treePath, true) // true = header row

	// Cursor handling: just replace first char with arrow, like commit headers
	var prefix string
	if isCursorRow {
		if m.focused {
			prefix = cursorArrowStyle.Render("▶")
		} else {
			prefix = unfocusedCursorArrowStyle.Render("▷")
		}
		// Replace first char of treeLine with arrow
		if len(treeLine) > 0 {
			treeLine = prefix + treeLine[1:]
		} else {
			treeLine = prefix
		}
	}

	return treeLine + " " + styledIcon + " " + fileStatusStyle.Render(fileNum) + " " + styledStatus + styledHeader + statsBar + boxPadding
}

// renderCommentRow renders a single comment row (part of a comment box).
// The row knows its position within the box (top border, content, or bottom border).
func (m Model) renderCommentRow(row displayRow, leftHalfWidth, rightHalfWidth, lineNumWidth int, isCursorRow bool) string {
	// Gutter: arrow(1) + space(1) + lineNum area
	gutterWidth := 2 + lineNumWidth

	// Box spans from after gutter to the left half width
	boxWidth := leftHalfWidth - gutterWidth
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
		topBorder := "┌" + strings.Repeat("─", boxWidth-2) + "┐"
		return leftGutter + commentBorderStyle.Render(topBorder) + sep + rightGutter + rightContent
	}

	if isBottomBorder {
		bottomBorder := "└" + strings.Repeat("─", boxWidth-2) + "┘"
		return leftGutter + commentBorderStyle.Render(bottomBorder) + sep + rightGutter + rightContent
	}

	// Content line - get the specific line from the comment
	lines := strings.Split(row.commentText, "\n")
	lineIdx := row.commentLineIndex
	var lineText string
	if lineIdx >= 0 && lineIdx < len(lines) {
		lineText = lines[lineIdx]
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

	return leftGutter + commentBorderStyle.Render("│ ") + paddedText + " " + commentBorderStyle.Render("│") + sep + rightGutter + rightContent
}

// wrapText wraps text to fit within maxWidth, preserving words where possible.
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	var lines []string
	// Split by explicit newlines first
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			testLine := currentLine + " " + word
			if displayWidth(testLine) <= maxWidth {
				currentLine = testLine
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		lines = append(lines, currentLine)
	}

	return lines
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

// displayWidth returns the display width of a string, accounting for
// wide characters (CJK, emoji) that take 2 cells.
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// truncateLeft removes the first width display columns from a string.
func truncateLeft(s string, width int) string {
	if width <= 0 {
		return s
	}
	currentWidth := 0
	for i, r := range s {
		w := runewidth.RuneWidth(r)
		currentWidth += w
		if currentWidth >= width {
			return s[i+len(string(r)):]
		}
	}
	return ""
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

// applyColumnIndicators wraps lines with gutter columns:
// - Added/removed: ░ + space + content + space + ░
// - Context/empty: space + space + content + space + space
// When hideTrailingGutter is true, omits the trailing space + indicator.
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
