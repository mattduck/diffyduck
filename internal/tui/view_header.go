package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

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

func (m Model) renderHeader(header string, foldLevel sidebyside.FoldLevel, headerMode HeaderMode, status FileStatus, added, removed, maxHeaderWidth, maxAddWidth, maxRemWidth, headerBoxWidth, fileIndex, rowIdx int, isCursorRow bool, treePath TreePath) string {
	// Calculate header width BEFORE applying search highlighting (ANSI codes affect width calculation)
	headerTextWidth := displayWidth(header)

	// Apply search highlighting if there's a query
	// Headers are always considered "side 0" for search purposes
	hasSearch := m.searchQuery != ""
	if hasSearch {
		header = m.highlightSearchInVisible(header, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide())
	}

	// Get fold level icon and file status style (for trailing fill color)
	icon := m.foldLevelIcon(foldLevel)
	_, fileStatusStyle := fileStatusIndicator(status)

	// All headers use same format: indent + icon + header + stats + trailing
	statsBar := formatColoredStatsBar(added, removed, maxAddWidth, maxRemWidth)
	statsBarWidth := statsBarDisplayWidth(maxAddWidth, maxRemWidth)
	headerPadding := ""
	if maxHeaderWidth > headerTextWidth {
		headerPadding = strings.Repeat(" ", maxHeaderWidth-headerTextWidth)
	}

	// Calculate content width and pad to match headerBoxWidth
	// Layout: indent(3) + icon(1) + space(1) + header
	iconPartWidth := 3 + 1 + 1 // "   ◐ "
	contentWidth := iconPartWidth + headerTextWidth + len(headerPadding) + statsBarWidth
	boxPadding := ""
	if headerBoxWidth > contentWidth {
		boxPadding = strings.Repeat(" ", headerBoxWidth-contentWidth)
	}

	// Style the header text:
	// - For added/deleted files, style the basename with inline diff style (bold+underline+color)
	// - For other files, use normal headerStyle (fg=15)
	// - Skip custom styling when search highlighting was applied (preserve search fg colors)
	styledHeader := m.styleFileHeaderText(header, headerPadding, status, hasSearch)

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

	result := treeLine + " " + styledIcon + styledHeader + statsBar + boxPadding

	// Add trailing border fill for unfolded files: ┏━━━━━━━ to screen edge
	if headerMode != HeaderSingleLine && m.width > 0 {
		treePrefixWidth := treeWidth(len(treePath.Ancestors), true)
		headerLineWidth := treePrefixWidth + headerBoxWidth - 2
		trailingFill := m.width - headerLineWidth - 1 // -1 for ┏
		if trailingFill > 0 {
			result += fileStatusStyle.Render("┏" + strings.Repeat("━", trailingFill))
		}
	}

	return result
}

// styleFileHeaderText applies styling to the file header text.
// Directories are fg=7, basename is fg=15 (no bold).
// For added/deleted files, the basename gets inline diff styling (bold+underline+color).
func (m Model) styleFileHeaderText(header, headerPadding string, status FileStatus, hasSearch bool) string {
	headerWithPadding := header + headerPadding

	// When search highlighting was applied, don't wrap with any style to preserve fg color
	if hasSearch {
		return " " + headerWithPadding
	}

	// Split into directory (fg=7) and basename (fg=15)
	basename := path.Base(header)
	dir := header[:len(header)-len(basename)]

	dirStyled := headerDirStyle.Render(" " + dir)

	// Basename style depends on file status
	var basenameStyled string
	switch status {
	case FileStatusAdded:
		basenameStyled = inlineAddedStyle.Render(basename)
	case FileStatusDeleted:
		basenameStyled = inlineRemovedStyle.Render(basename)
	default:
		basenameStyled = headerBasenameStyle.Render(basename)
	}

	return dirStyled + basenameStyled + headerPadding
}

// renderCommentRow renders a single comment row (part of a comment box).

// formatBreadcrumbsStyled formats structure entries with syntax highlighting.
// Applies semantic colors: keywords blue, function names bright blue, types magenta.
// maxWidth controls param expansion (0 = compact with "...").
func formatBreadcrumbsStyled(entries []structure.Entry, theme highlight.Theme, maxWidth int) string {
	if len(entries) == 0 {
		return ""
	}

	// Styles for different semantic parts
	keywordStyle := theme.Style(highlight.CategoryKeyword)
	funcStyle := theme.Style(highlight.CategoryFunction)
	typeStyle := theme.Style(highlight.CategoryType)
	punctStyle := theme.Style(highlight.CategoryPunctuation)

	// Calculate width budget for innermost entry's signature
	// (same logic as formatBreadcrumbs in model.go)
	separatorWidth := 3 // " > "
	totalSeparators := len(entries) - 1
	reservedWidth := totalSeparators * separatorWidth

	sigWidth := 0
	if maxWidth > 0 {
		sigWidth = maxWidth - reservedWidth
		for _, e := range entries[:len(entries)-1] {
			// Estimate width for outer entries (kind + space + name + buffer)
			outerWidth := len(e.Kind) + 1 + len(e.Name) + 5
			sigWidth -= outerWidth
		}
		if sigWidth < 20 {
			sigWidth = 20
		}
	}

	var result strings.Builder
	for i, e := range entries {
		if i > 0 {
			result.WriteString(punctStyle.Render(" > "))
		}

		// Style the kind (func, type, class, def, etc.)
		result.WriteString(keywordStyle.Render(e.Kind))
		result.WriteString(" ")

		// Determine name style based on kind
		nameStyle := funcStyle
		if e.Kind == "type" || e.Kind == "class" || e.Kind == "struct" || e.Kind == "interface" {
			nameStyle = typeStyle
		}

		// Calculate width budget for this entry's signature
		entryWidth := 0
		if i == len(entries)-1 && sigWidth > 0 {
			kindPrefixLen := len(e.Kind) + 1
			entryWidth = sigWidth - kindPrefixLen
			if entryWidth < 0 {
				entryWidth = 0
			}
		}

		// Format signature with styling (build directly from Entry fields)
		if len(e.Params) > 0 || e.ReturnType != "" || e.Receiver != "" {
			result.WriteString(formatSignatureStyled(e, entryWidth, nameStyle, typeStyle, punctStyle))
		} else {
			result.WriteString(nameStyle.Render(e.Name))
		}
	}

	return result.String()
}

// formatSignatureStyled builds a styled signature directly from Entry fields.
// Output format: "[receiver] name(params) -> returnType"
// maxWidth controls param expansion (0 = compact with "...").
func formatSignatureStyled(e structure.Entry, maxWidth int, nameStyle, typeStyle, punctStyle lipgloss.Style) string {
	var result strings.Builder

	// Handle receiver if present
	if e.Receiver != "" {
		result.WriteString(punctStyle.Render(e.Receiver))
		result.WriteString(" ")
	}

	// Style the name
	result.WriteString(nameStyle.Render(e.Name))

	// Build params section - determine how many params to show based on width
	result.WriteString(punctStyle.Render("("))
	if len(e.Params) == 0 {
		// No params - empty parens
	} else if maxWidth <= 0 {
		// Compact format - just show ellipsis
		result.WriteString("...")
	} else {
		// Try to fit as many params as possible within width budget
		// Calculate base width (receiver + name + parens + return type)
		baseWidth := len(e.Receiver)
		if e.Receiver != "" {
			baseWidth++ // space after receiver
		}
		baseWidth += len(e.Name) + 2 // name + "()"
		if e.ReturnType != "" {
			baseWidth += 4 + len(e.ReturnType) // " -> " + returnType
		}

		availableForParams := maxWidth - baseWidth
		if availableForParams < 3 {
			// Not enough space, use compact
			result.WriteString("...")
		} else {
			// Try progressively adding params
			numParams := 0
			for n := 1; n <= len(e.Params); n++ {
				var testParams string
				if n >= len(e.Params) {
					testParams = strings.Join(e.Params, ", ")
				} else {
					testParams = strings.Join(e.Params[:n], ", ") + ", ..."
				}
				if len(testParams) <= availableForParams {
					numParams = n
				} else {
					break
				}
			}

			if numParams == 0 {
				result.WriteString("...")
			} else if numParams >= len(e.Params) {
				result.WriteString(strings.Join(e.Params, ", "))
			} else {
				result.WriteString(strings.Join(e.Params[:numParams], ", ") + ", ...")
			}
		}
	}
	result.WriteString(punctStyle.Render(")"))

	// Add return type if present
	if e.ReturnType != "" {
		result.WriteString(punctStyle.Render(" -> "))
		result.WriteString(typeStyle.Render(e.ReturnType))
	}

	return result.String()
}
