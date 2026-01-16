package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/sidebyside"
)

var (
	// Styles for different line types
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	addedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	removedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	contextStyle = lipgloss.NewStyle()
	lineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	emptyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Build list of all displayable rows
	rows := m.buildRows()

	// Apply scroll and viewport
	visibleRows := m.getVisibleRows(rows)

	return strings.Join(visibleRows, "\n")
}

// displayRow represents one row in the view (either a header or a line pair)
type displayRow struct {
	isHeader bool
	header   string
	pair     sidebyside.LinePair
}

// buildRows creates all displayable rows from the model data.
func (m Model) buildRows() []displayRow {
	var rows []displayRow

	for _, fp := range m.files {
		// File header
		header := formatFileHeader(fp.OldPath, fp.NewPath)
		rows = append(rows, displayRow{isHeader: true, header: header})

		// Line pairs
		for _, pair := range fp.Pairs {
			rows = append(rows, displayRow{isHeader: false, pair: pair})
		}
	}

	return rows
}

// getVisibleRows returns the rendered rows visible in the current viewport.
func (m Model) getVisibleRows(rows []displayRow) []string {
	var visible []string

	// Calculate column widths
	halfWidth := (m.width - 3) / 2 // -3 for the separator " │ "
	lineNumWidth := 4

	start := m.scroll
	end := m.scroll + m.height
	if end > len(rows) {
		end = len(rows)
	}

	for i := start; i < end; i++ {
		row := rows[i]
		if row.isHeader {
			visible = append(visible, m.renderHeader(row.header))
		} else {
			visible = append(visible, m.renderLinePair(row.pair, halfWidth, lineNumWidth))
		}
	}

	return visible
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

func (m Model) renderHeader(header string) string {
	return headerStyle.Render(header)
}

func (m Model) renderLinePair(pair sidebyside.LinePair, halfWidth, lineNumWidth int) string {
	contentWidth := halfWidth - lineNumWidth - 1 // -1 for space after line num

	left := m.renderLine(pair.Left, contentWidth, lineNumWidth)
	right := m.renderLine(pair.Right, contentWidth, lineNumWidth)

	return left + " │ " + right
}

func (m Model) renderLine(line sidebyside.Line, contentWidth, lineNumWidth int) string {
	// Line number (fixed, not affected by horizontal scroll)
	var numStr string
	if line.Num == 0 {
		numStr = strings.Repeat(" ", lineNumWidth)
	} else {
		numStr = lineNumStyle.Render(fmt.Sprintf("%*d", lineNumWidth, line.Num))
	}

	// Content - expand tabs, then apply horizontal scroll
	content := horizontalSlice(expandTabs(line.Content), m.hscroll, contentWidth)

	// Apply style based on type
	var styledContent string
	switch line.Type {
	case sidebyside.Added:
		styledContent = addedStyle.Render(content)
	case sidebyside.Removed:
		styledContent = removedStyle.Render(content)
	case sidebyside.Empty:
		styledContent = emptyStyle.Render(content)
	default:
		styledContent = contextStyle.Render(content)
	}

	return numStr + " " + styledContent
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
