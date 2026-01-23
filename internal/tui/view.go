package tui

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/diff"
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
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	contentH := m.contentHeight()

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
	RowKindSeparatorTop        // top shader line above hunk separator
	RowKindSeparator           // hunk separator with breadcrumb
	RowKindSeparatorBottom     // bottom shader line below hunk separator
	RowKindSummary             // summary row at the end
	RowKindTruncationIndicator // truncation message row
	RowKindBinaryIndicator     // binary file message row
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
	isSummary             bool // summary row at the end showing total stats
	isTruncationIndicator bool // true if this row shows a truncation message

	borderVisible  bool // whether border should use normal color (true) or fg=0 (false)
	isFirstLine    bool // first line pair in a file (uses ┬ separator)
	isLastLine     bool // last line pair in a file (uses ┴ separator)
	header         string
	foldLevel      sidebyside.FoldLevel // fold level for headers (used for icon and styling)
	status         FileStatus           // file status (added, deleted, renamed, modified) for headers
	pair           sidebyside.LinePair
	added          int // number of added lines (for headers)
	removed        int // number of removed lines (for headers)
	maxHeaderWidth int // max header width across all files (for alignment in folded view)
	maxAddWidth    int // max addition count width across all files (for column alignment)
	maxRemWidth    int // max removal count width across all files (for column alignment)
	maxCountWidth  int // max stats count width across all files (for bar alignment)
	headerBoxWidth int // width of the box around header content (for border alignment)
	// Summary row fields
	totalFiles   int // total number of files changed
	totalAdded   int // total insertions across all files
	totalRemoved int // total deletions across all files
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
}

// buildRows creates all displayable rows from the model data.
func (m Model) buildRows() []displayRow {
	var rows []displayRow

	// Calculate max header width and max add/rem widths across all files for alignment
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
	// Box contains: lineNumWidth + " ◐ ~ " (5 chars) + maxHeaderWidth + maxStatsBarWidth
	lineNumWidth := m.lineNumWidth()
	iconPartWidth := 5 // " ◐ ~ " = space + icon(1) + space + status(1) + space
	maxStatsBarWidth := 0
	if maxAddWidth > 0 || maxRemWidth > 0 {
		maxStatsBarWidth = 1 + maxAddWidth // leading space + add column
		if maxAddWidth > 0 {
			maxStatsBarWidth++ // space between + and -
		}
		maxStatsBarWidth += maxRemWidth // removal column
	}
	headerBoxWidth := lineNumWidth + iconPartWidth + maxHeaderWidth + maxStatsBarWidth

	for fileIdx, fp := range m.files {
		// Count stats once per file for header display
		added, removed := countFileStats(fp)
		status := fileStatusFromPair(fp)

		// Check if this is the first file and if it's unfolded - needs a top border
		isFirstFile := fileIdx == 0
		isUnfolded := fp.FoldLevel != sidebyside.FoldFolded

		// Check if previous file is unfolded (for header/bottom border visibility)
		// First file counts as "previous unfolded" since there's nothing above
		prevFileUnfolded := isFirstFile
		if fileIdx > 0 {
			prevFileUnfolded = m.files[fileIdx-1].FoldLevel != sidebyside.FoldFolded
		}

		// Check if next file exists and is unfolded (for trailing border visibility)
		nextFileUnfolded := false
		if fileIdx+1 < len(m.files) {
			nextFileUnfolded = m.files[fileIdx+1].FoldLevel != sidebyside.FoldFolded
		}

		switch fp.FoldLevel {
		case sidebyside.FoldFolded:
			// Folded: just the header, no borders - files stack tightly together
			header := formatFileHeader(fp)
			rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldFolded, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxAddWidth: maxAddWidth, maxRemWidth: maxRemWidth, maxCountWidth: statsCountWidth(added, removed, maxAddWidth), headerBoxWidth: headerBoxWidth})

		case sidebyside.FoldExpanded:
			// Expanded: show full file content with diff highlighting
			// If content not loaded yet, fall back to normal view
			if fp.HasContent() {
				// First file gets a top border before header (visible since no file above)
				if isFirstFile && isUnfolded {
					rows = append(rows, displayRow{kind: RowKindHeaderTopBorder, fileIndex: fileIdx, isHeaderTopBorder: true, foldLevel: sidebyside.FoldExpanded, status: status, headerBoxWidth: headerBoxWidth, borderVisible: true})
				}

				// File header with stats
				// Border visible only if previous file is also unfolded (or this is first file)
				header := formatFileHeader(fp)
				rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: sidebyside.FoldExpanded, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxAddWidth: maxAddWidth, maxRemWidth: maxRemWidth, maxCountWidth: statsCountWidth(added, removed, maxAddWidth), headerBoxWidth: headerBoxWidth, borderVisible: prevFileUnfolded})

				// Bottom border of header box (visible only if previous file is also unfolded)
				rows = append(rows, displayRow{kind: RowKindHeaderSpacer, fileIndex: fileIdx, isHeaderSpacer: true, foldLevel: sidebyside.FoldExpanded, status: status, headerBoxWidth: headerBoxWidth, borderVisible: prevFileUnfolded})

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

				// Add file truncation indicator if this file was truncated
				if fp.Truncated || fp.ContentTruncated || fp.OldContentTruncated || fp.NewContentTruncated {
					// Determine which sides to show truncation on
					// For expanded view, use content truncation flags; fall back to diff truncation flags
					oldTrunc := fp.OldContentTruncated || fp.OldTruncated
					newTrunc := fp.NewContentTruncated || fp.NewTruncated
					// Legacy: if only ContentTruncated is set (old code path), show on both sides
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

				// Add 4 blank lines after expanded content
				for i := 0; i < 4; i++ {
					rows = append(rows, displayRow{kind: RowKindBlank, fileIndex: fileIdx, isBlank: true})
				}

				// Trailing top border (visually looks like top of next file, but belongs to this file)
				// Only visible if next file is also unfolded
				rows = append(rows, displayRow{kind: RowKindHeaderTopBorder, fileIndex: fileIdx, isHeaderTopBorder: true, foldLevel: sidebyside.FoldExpanded, status: status, headerBoxWidth: headerBoxWidth, borderVisible: nextFileUnfolded})
				continue // Skip the normal view below
			}
			// Fall through to normal view if content not loaded
			fallthrough

		default: // FoldNormal (or FoldExpanded falling through while content loads)
			// First file gets a top border before header (visible since no file above)
			if isFirstFile && isUnfolded {
				rows = append(rows, displayRow{kind: RowKindHeaderTopBorder, fileIndex: fileIdx, isHeaderTopBorder: true, foldLevel: fp.FoldLevel, status: status, headerBoxWidth: headerBoxWidth, borderVisible: true})
			}

			// File header with stats
			// Border visible only if previous file is also unfolded (or this is first file)
			header := formatFileHeader(fp)
			rows = append(rows, displayRow{kind: RowKindHeader, fileIndex: fileIdx, isHeader: true, foldLevel: fp.FoldLevel, status: status, header: header, added: added, removed: removed, maxHeaderWidth: maxHeaderWidth, maxAddWidth: maxAddWidth, maxRemWidth: maxRemWidth, maxCountWidth: statsCountWidth(added, removed, maxAddWidth), headerBoxWidth: headerBoxWidth, borderVisible: prevFileUnfolded})

			// Bottom border of header box (visible only if previous file is also unfolded)
			rows = append(rows, displayRow{kind: RowKindHeaderSpacer, fileIndex: fileIdx, isHeaderSpacer: true, foldLevel: fp.FoldLevel, status: status, headerBoxWidth: headerBoxWidth, borderVisible: prevFileUnfolded})

			// Binary files: show message instead of content
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
				// Line pairs with hunk separators
				var prevLeft, prevRight int
				for i, pair := range fp.Pairs {
					// Add separator before first chunk if it starts after line 1
					// (when starting at line 1, user can see they're at the top - no breadcrumb needed)
					if i == 0 && (pair.Old.Num > 1 || pair.New.Num > 1) {
						chunkStartLine := findFirstNewLineNum(fp.Pairs, i)
						rows = append(rows, displayRow{kind: RowKindSeparatorTop, fileIndex: fileIdx, isSeparatorTop: true})
						rows = append(rows, displayRow{kind: RowKindSeparator, fileIndex: fileIdx, isSeparator: true, chunkStartLine: chunkStartLine})
						rows = append(rows, displayRow{kind: RowKindSeparatorBottom, fileIndex: fileIdx, isSeparatorBottom: true, chunkStartLine: chunkStartLine})
					}

					// Check for gap in line numbers (hunk boundary)
					if i > 0 && isHunkBoundary(prevLeft, prevRight, pair.Old.Num, pair.New.Num) {
						// Find first non-zero New.Num in this chunk for breadcrumb lookup
						chunkStartLine := findFirstNewLineNum(fp.Pairs, i)
						// Add three-line separator: top shader + breadcrumb + bottom shader
						rows = append(rows, displayRow{kind: RowKindSeparatorTop, fileIndex: fileIdx, isSeparatorTop: true})
						rows = append(rows, displayRow{kind: RowKindSeparator, fileIndex: fileIdx, isSeparator: true, chunkStartLine: chunkStartLine})
						rows = append(rows, displayRow{kind: RowKindSeparatorBottom, fileIndex: fileIdx, isSeparatorBottom: true, chunkStartLine: chunkStartLine})
					}

					row := displayRow{kind: RowKindContent, fileIndex: fileIdx, pair: pair}
					if i == 0 {
						row.isFirstLine = true
					}
					if i == len(fp.Pairs)-1 {
						row.isLastLine = true
					}
					rows = append(rows, row)

					// Track previous line numbers (use non-zero values)
					if pair.Old.Num > 0 {
						prevLeft = pair.Old.Num
					}
					if pair.New.Num > 0 {
						prevRight = pair.New.Num
					}
				}

				// Add file truncation indicator if this file was truncated
				if fp.Truncated || fp.OldTruncated || fp.NewTruncated {
					// Determine which sides to show truncation on
					oldTrunc := fp.OldTruncated
					newTrunc := fp.NewTruncated
					// Legacy: if only Truncated is set (old code path), show on both sides
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

			// Add 4 blank lines after normal content
			for i := 0; i < 4; i++ {
				rows = append(rows, displayRow{kind: RowKindBlank, fileIndex: fileIdx, isBlank: true})
			}

			// Trailing top border (visually looks like top of next file, but belongs to this file)
			// Only visible if next file is also unfolded
			rows = append(rows, displayRow{kind: RowKindHeaderTopBorder, fileIndex: fileIdx, isHeaderTopBorder: true, foldLevel: fp.FoldLevel, status: status, headerBoxWidth: headerBoxWidth, borderVisible: nextFileUnfolded})
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
			kind:           RowKindSummary,
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

	// The cursor is at a fixed viewport position
	cursorViewportRow := m.cursorOffset()

	start := m.scroll
	end := m.scroll + contentHeight

	// Handle negative scroll by adding blank padding at the top
	if start < 0 {
		for i := start; i < 0 && len(visible) < contentHeight; i++ {
			isCursorRow := len(visible) == cursorViewportRow
			if isCursorRow {
				visible = append(visible, m.renderBlankWithCursor(leftHalfWidth, rightHalfWidth, lineNumWidth))
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

		if row.isHeaderTopBorder {
			visible = append(visible, m.renderHeaderTopBorder(row.headerBoxWidth, row.borderVisible, row.status, isCursorRow))
		} else if row.isHeaderSpacer {
			visible = append(visible, m.renderHeaderBottomBorder(row.headerBoxWidth, row.borderVisible, row.status, isCursorRow))
		} else if row.isBlank {
			if isCursorRow {
				visible = append(visible, m.renderBlankWithCursor(leftHalfWidth, rightHalfWidth, lineNumWidth))
			} else {
				visible = append(visible, m.renderInterFileBlank())
			}
		} else if row.isHeader {
			visible = append(visible, m.renderHeader(row.header, row.foldLevel, row.borderVisible, row.status, row.added, row.removed, row.maxHeaderWidth, row.maxAddWidth, row.maxRemWidth, row.headerBoxWidth, row.fileIndex, i, isCursorRow))
		} else if row.isSeparatorTop {
			visible = append(visible, m.renderHunkSeparatorTop(leftHalfWidth, rightHalfWidth, isCursorRow))
		} else if row.isSeparator {
			visible = append(visible, m.renderHunkSeparator(row, leftHalfWidth, rightHalfWidth, isCursorRow))
		} else if row.isSeparatorBottom {
			visible = append(visible, m.renderHunkSeparatorTop(leftHalfWidth, rightHalfWidth, isCursorRow)) // same as top
		} else if row.isSummary {
			visible = append(visible, m.renderSummary(row.totalFiles, row.totalAdded, row.totalRemoved, row.maxHeaderWidth, isCursorRow))
		} else if row.isTruncationIndicator {
			visible = append(visible, m.renderTruncationIndicator(row.truncationMessage, isCursorRow, row.truncateOld, row.truncateNew))
		} else if row.isBinaryIndicator {
			visible = append(visible, m.renderBinaryIndicator(row.binaryMessage, isCursorRow, row.binaryOld, row.binaryNew))
		} else {
			visible = append(visible, m.renderLinePair(row.pair, row.fileIndex, leftHalfWidth, rightHalfWidth, lineNumWidth, i, isCursorRow, row.isFirstLine, row.isLastLine, hideRightTrailingGutter))
		}
	}

	return visible
}

// renderHunkSeparator renders a separator line between hunks.
// If structure data is available, shows breadcrumbs on the left side (new content).
func (m Model) renderHunkSeparator(row displayRow, leftHalfWidth, rightHalfWidth int, isCursorRow bool) string {
	shadeStyle := hunkSeparatorStyle
	lineNumWidth := m.lineNumWidth()

	// Gutter width: indicator(1) + space(1) + lineNumWidth (one less than content lines for tighter breadcrumb)
	gutterWidth := 2 + lineNumWidth

	// Content width after gutter (breadcrumb starts here, aligned with code content)
	leftContentWidth := leftHalfWidth - gutterWidth
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
		// Non-cursor: all shading
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

		return leftGutter + leftContent + shadeStyle.Render("░░░") + rightGutter + rightContent
	}

	// Cursor row: arrow in gutter, lineNumWidth chars with cursor bg, then breadcrumb in content area
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

	return leftGutter + leftContent + shadeStyle.Render("░░░") + rightGutter + rightContent
}

// renderHunkSeparatorTop renders the top line of a hunk separator (faint shader for visual separation).
func (m Model) renderHunkSeparatorTop(leftHalfWidth, rightHalfWidth int, isCursorRow bool) string {
	// Faint shader style - less visible than the main separator
	faintShadeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	lineNumWidth := m.lineNumWidth()

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

	if !isCursorRow {
		// Non-cursor: all faint shading
		leftArrow := faintShadeStyle.Render("░░")
		rightArrow := faintShadeStyle.Render("░░")
		leftContent := faintShadeStyle.Render(strings.Repeat("░", leftContentWidth))
		rightContent := faintShadeStyle.Render(strings.Repeat("░", rightContentWidth))
		return leftArrow + leftContent + faintShadeStyle.Render("░░░") + rightArrow + rightContent
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

// renderHeaderTopBorder renders the top border of the file header box.
// Format: ─────────────────────┐ (horizontal lines on left, corner on right)
func (m Model) renderHeaderTopBorder(headerBoxWidth int, borderVisible bool, status FileStatus, isCursorRow bool) string {
	lineNumWidth := m.lineNumWidth()
	_ = status // status not used for top border (no shading)

	// Use darker color when border should not be visible (fg=0)
	borderStyle := headerLineStyle
	if !borderVisible {
		borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	}

	innerWidth := headerBoxWidth
	if innerWidth < 0 {
		innerWidth = 0
	}

	if isCursorRow {
		// Arrow at position 0, then ─ for position 1, then gutter area highlighted (when focused)
		// +1 at end for the space gap before the border corner
		var styledGutter, arrow string
		if m.focused {
			styledGutter = cursorStyle.Render(strings.Repeat("─", lineNumWidth))
			arrow = cursorArrowStyle.Render("▶")
		} else {
			styledGutter = borderStyle.Render(strings.Repeat("─", lineNumWidth))
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		restWidth := innerWidth - lineNumWidth + 1
		if restWidth < 0 {
			restWidth = 0
		}
		return arrow + borderStyle.Render("─") + styledGutter + borderStyle.Render(strings.Repeat("─", restWidth)+"┐")
	}

	border := strings.Repeat("─", 2+innerWidth+1) // +1 for space gap before corner
	return borderStyle.Render(border + "┐")
}

// renderHeaderBottomBorder renders the bottom border of the file header box.
// Format: ─────────────────────┘ (horizontal lines on left, corner on right)
func (m Model) renderHeaderBottomBorder(headerBoxWidth int, borderVisible bool, status FileStatus, isCursorRow bool) string {
	lineNumWidth := m.lineNumWidth()
	_ = status // status not used for bottom border

	// Use darker color when border should not be visible (fg=0)
	borderStyle := headerLineStyle
	if !borderVisible {
		borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	}

	innerWidth := headerBoxWidth
	if innerWidth < 0 {
		innerWidth = 0
	}

	if isCursorRow {
		// Arrow at position 0, then ─ for position 1, then gutter area highlighted (when focused)
		// +1 at end for the space gap before the border corner
		var styledGutter, arrow string
		if m.focused {
			styledGutter = cursorStyle.Render(strings.Repeat("─", lineNumWidth))
			arrow = cursorArrowStyle.Render("▶")
		} else {
			styledGutter = borderStyle.Render(strings.Repeat("─", lineNumWidth))
			arrow = unfocusedCursorArrowStyle.Render("▷")
		}
		restWidth := innerWidth - lineNumWidth + 1
		if restWidth < 0 {
			restWidth = 0
		}
		return arrow + borderStyle.Render("─") + styledGutter + borderStyle.Render(strings.Repeat("─", restWidth)+"┘")
	}

	border := strings.Repeat("─", 2+innerWidth+1) // +1 for space gap before corner
	return borderStyle.Render(border + "┘")
}

// renderTopBar renders the top bar showing file info with a divider line below.
func (m Model) renderTopBar() string {
	info := m.StatusInfo()

	// Only show file info when cursor is on a file (not on summary row)
	var content string
	if info.CurrentFile > 0 {
		content = m.formatStatusFileInfo(info)
	}

	// Calculate total stats for all files
	totalAdded := 0
	totalRemoved := 0
	for _, fp := range m.files {
		added, removed := countFileStats(fp)
		totalAdded += added
		totalRemoved += removed
	}

	// Left section: file counter [01/27] - colored to match current file's status
	_, fileCounterStyle := fileStatusIndicator(FileStatus(info.FileStatus))
	totalWidth := len(fmt.Sprintf("%d", info.TotalFiles))
	counterText := fmt.Sprintf("[%0*d/%d]", totalWidth, info.CurrentFile, info.TotalFiles)
	fileCounter := fileCounterStyle.Render(counterText) + " "
	counterDisplayWidth := len(counterText) + 1 // +1 for trailing space

	// Right section: total stats +123 -123 (only if there are changes)
	var rightText string
	var rightSection string
	if totalAdded > 0 || totalRemoved > 0 {
		addedText := fmt.Sprintf("+%d", totalAdded)
		removedText := fmt.Sprintf("-%d", totalRemoved)
		rightText = addedText + " " + removedText
		rightSection = addedStyle.Render(addedText) + " " + removedStyle.Render(removedText)
	}

	// Calculate widths for padding
	// Layout: prefix + counter + content + padding + rightSection
	prefixWidth := 2 // "▶ "
	contentWidth := lipgloss.Width(content)
	rightWidth := len(rightText)
	padding := m.width - prefixWidth - counterDisplayWidth - contentWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	// Leading arrow indicator (matches cursor arrow in gutter)
	// Use outline arrow when unfocused
	var prefix string
	if m.focused {
		prefix = cursorArrowStyle.Render("▶") + " "
	} else {
		prefix = unfocusedCursorArrowStyle.Render("▷") + " "
	}
	topLine := prefix + fileCounter + content + strings.Repeat(" ", padding) + rightSection

	// Divider line using upper 1/8 block (dim, faint when unfocused)
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	if !m.focused {
		dividerStyle = dividerStyle.Faint(true)
	}
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
	// Use unfocused style when terminal loses focus
	var styledLessIndicator string
	if m.focused {
		styledLessIndicator = statusStyle.Render(" " + lessIndicator)
	} else {
		styledLessIndicator = unfocusedStatusStyle.Render(" " + lessIndicator)
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

	// Combine: reversed_less_indicator + loading + padding + debug_stats + pager_indicator
	content := styledLessIndicator + loadingIndicator
	contentWidth := displayWidth(" "+lessIndicator) + loadingWidth
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
// Format: foldIcon statusIcon fileName +N -M
func (m Model) formatStatusFileInfo(info StatusInfo) string {
	// Get fold level icon
	icon := m.foldLevelIcon(info.FoldLevel)

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

	return icon + " " + styledStatus + " " + info.FileName + stats + breadcrumbs
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

	// Space before removal + removal column (always padded to maxRemWidth)
	if maxAddWidth > 0 {
		width++ // space between +N and -M
	}
	width += maxRemWidth

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
// Format: "▶ ━━━ ●   N files changed, N insertions(+), N deletions(-)" (when cursor)
// Uses expanded icon (●) since there's no additional content to show.
// Text is not bold, unlike file headers.
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

func (m Model) renderSummary(totalFiles, totalAdded, totalRemoved, maxHeaderWidth int, isCursorRow bool) string {
	lineNumWidth := m.lineNumWidth()
	equalsGutter := strings.Repeat("━", lineNumWidth)
	icon := m.foldLevelIcon(sidebyside.FoldExpanded) // Always use expanded icon
	// Space where status indicator would be (empty for summary)
	iconPart := " " + icon + "   " // icon + 3 spaces (status position + space)

	summary := formatSummaryStats(totalFiles, totalAdded, totalRemoved)

	// Use non-bold style for summary (just the foreground color)
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	if isCursorRow && m.focused {
		// Format: arrow + space + gutter(━━━ with bg) + space + icon + summary
		return cursorArrowStyle.Render("▶") + " " + cursorStyle.Render(equalsGutter) + summaryStyle.Render(iconPart+summary)
	}
	if isCursorRow && !m.focused {
		// Unfocused: outline arrow, no background highlight
		return unfocusedCursorArrowStyle.Render("▷") + " " + headerLineStyle.Render(equalsGutter) + summaryStyle.Render(iconPart+summary)
	}
	// Format: space + space + gutter(━━━ dim) + space + icon + summary
	return "  " + headerLineStyle.Render(equalsGutter) + summaryStyle.Render(iconPart+summary)
}

func (m Model) renderHeader(header string, foldLevel sidebyside.FoldLevel, borderVisible bool, status FileStatus, added, removed, maxHeaderWidth, maxAddWidth, maxRemWidth, headerBoxWidth, fileIndex, rowIdx int, isCursorRow bool) string {
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

	lineNumWidth := m.lineNumWidth()

	// File number with hash prefix and leading zeros, left-aligned with padding to match lineNumWidth
	// Color matches the file status (green=added, red=deleted, blue=modified/renamed)
	totalFiles := len(m.files)
	numDigits := len(fmt.Sprintf("%d", totalFiles))
	fileNum := fmt.Sprintf("#%0*d", numDigits, fileIndex+1) // #01
	fileNumPadded := fileNum + strings.Repeat(" ", lineNumWidth-len(fileNum))

	// All headers use same format: gutter + icon + status + header + stats + │ + trailing
	statsBar := formatColoredStatsBar(added, removed, maxAddWidth, maxRemWidth)
	statsBarWidth := statsBarDisplayWidth(maxAddWidth, maxRemWidth)
	headerPadding := ""
	if maxHeaderWidth > headerTextWidth {
		headerPadding = strings.Repeat(" ", maxHeaderWidth-headerTextWidth)
	}

	// Calculate content width and pad to match headerBoxWidth
	// Status symbol is always 1 character (or spinner which is also 1 char)
	iconPartWidth := 1 + len(icon) + 1 + 1 + 1 // " icon status "
	contentWidth := lineNumWidth + iconPartWidth + headerTextWidth + len(headerPadding) + statsBarWidth
	boxPadding := ""
	if headerBoxWidth > contentWidth {
		boxPadding = strings.Repeat(" ", headerBoxWidth-contentWidth)
	}

	// Calculate trailing fill to fill the width with status-colored shading
	// Format: prefix(2) + content(headerBoxWidth) + space(1) + │(1) + space + trailing
	prefixWidth := 2 + headerBoxWidth + 1 + 1 // prefix + content + space + │
	trailing := m.width - prefixWidth
	if trailing < 1 {
		trailing = 0
	}
	trailingFill := ""
	if trailing > 0 {
		trailingFill = fileStatusStyle.Render(strings.Repeat("▒", trailing+1))
	}

	// Use darker color for border when not visible (file above is folded)
	borderStyle := headerLineStyle
	if !borderVisible {
		borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	}

	// Style the header text - but if search highlighting was applied, don't override it
	// (search highlighting sets fg=0 which would be overridden by headerStyle's fg=15)
	styledHeader := headerStyle.Render(" " + header + headerPadding)
	if m.searchQuery != "" {
		// Search highlighting was applied; don't wrap with headerStyle to preserve fg color
		styledHeader = " " + header + headerPadding
	}

	if isCursorRow && m.focused {
		// Format: arrow + space + fileNum(with bg) + space + icon + status + header + padding + stats + boxPadding + space + │ + trailing
		styledFileNum := cursorStyle.Render(fileNumPadded)
		return cursorArrowStyle.Render("▶") + " " + styledFileNum + headerStyle.Render(" "+icon+" ") + styledStatus + styledHeader + statsBar + boxPadding + " " + borderStyle.Render("│") + trailingFill
	}

	if isCursorRow && !m.focused {
		// Unfocused: outline arrow, no background highlight (use same style as non-cursor row)
		return unfocusedCursorArrowStyle.Render("▷") + " " + fileStatusStyle.Render(fileNumPadded) + headerStyle.Render(" "+icon+" ") + styledStatus + styledHeader + statsBar + boxPadding + " " + borderStyle.Render("│") + trailingFill
	}

	// Normal rendering
	// Format: space + space + fileNum + space + icon + status + header + padding + stats + boxPadding + space + │ + trailing
	return "  " + fileStatusStyle.Render(fileNumPadded) + headerStyle.Render(" "+icon+" ") + styledStatus + styledHeader + statsBar + boxPadding + " " + borderStyle.Render("│") + trailingFill
}

func (m Model) renderLinePair(pair sidebyside.LinePair, fileIndex, leftHalfWidth, rightHalfWidth, lineNumWidth, rowIdx int, isCursorRow bool, isFirstLine, isLastLine, hideRightTrailingGutter bool) string {
	leftContentWidth := leftHalfWidth - lineNumWidth - 3   // -3 for indicator, space after indicator, and space after line num
	rightContentWidth := rightHalfWidth - lineNumWidth - 3 // same layout on right side

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
	return left + " " + separator + " " + right
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
		// Apply inline diff highlighting (with search highlighting taking precedence)
		styledContent = m.applyInlineSpans(expanded, visible, inlineSpans, line.Type, isCursorRow, shouldSearch, m.currentMatchIdx(), side, m.currentMatchSide())
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
func (m Model) applyInlineSpans(expanded, visible string, spans []inlinediff.Span, lineType sidebyside.LineType, isCursorRow, shouldSearch bool, currentIdx, side, currentSide int) string {
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
