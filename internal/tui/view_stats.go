package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/diffyduck/pkg/sidebyside"
)

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

// formatLessIndicator formats the less-style line indicator.
// Returns "line N/TOTAL X%" normally, or "line N/TOTAL (END)" when at end.
func formatLessIndicator(line, total, percentage int, atEnd bool) string {
	if atEnd {
		return fmt.Sprintf("line %d/%d (END)", line, total)
	}
	return fmt.Sprintf("line %d/%d %d%%", line, total, percentage)
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
		// Show just + right-aligned in dim grey (no additions)
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		addStr := strings.Repeat(" ", maxAddWidth-1) + "+"
		parts = append(parts, dimStyle.Render(addStr))
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
		// Show just - right-aligned in dim grey (no removals)
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		remStr := strings.Repeat(" ", maxRemWidth-1) + "-"
		parts = append(parts, dimStyle.Render(remStr))
	}

	return " " + strings.Join(parts, " ")
}

// fileHeaderBoxWidth computes the header box width for a single file based on its own content.
// Used for unfolded headers so the border hugs the actual content instead of aligning to shared max.
func fileHeaderBoxWidth(headerText string, added, removed, totalFilesInCommit int) int {
	numDigits := len(fmt.Sprintf("%d", totalFilesInCommit))
	fileNumWidth := 1 + numDigits
	iconPartWidth := 3 + 1 + 1 + fileNumWidth + 1 + 1 + 1
	aw := statsAddWidth(added)
	rw := statsRemWidth(removed)
	return iconPartWidth + displayWidth(headerText) + statsBarDisplayWidth(aw, rw) + 1 // +1 for gap before ┏
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
