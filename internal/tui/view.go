package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	left := renderLine(pair.Left, contentWidth, lineNumWidth)
	right := renderLine(pair.Right, contentWidth, lineNumWidth)

	return left + " │ " + right
}

func renderLine(line sidebyside.Line, contentWidth, lineNumWidth int) string {
	// Line number
	var numStr string
	if line.Num == 0 {
		numStr = strings.Repeat(" ", lineNumWidth)
	} else {
		numStr = lineNumStyle.Render(fmt.Sprintf("%*d", lineNumWidth, line.Num))
	}

	// Content
	content := truncateOrPad(line.Content, contentWidth)

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

func truncateOrPad(s string, width int) string {
	if len(s) > width {
		if width > 3 {
			return s[:width-3] + "..."
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
