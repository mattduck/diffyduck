package tui

import (
	"fmt"
	"strings"

	"github.com/user/diffyduck/pkg/sidebyside"
)

// SearchBaseStyler styles non-matched text segments during search highlighting.
// It receives the segment text and its byte offset within the original string,
// allowing position-aware styling (e.g., different colors for directory vs basename).
type SearchBaseStyler func(text string, byteOffset int) string

// highlightSearchInVisible highlights search matches in visible text.
// Searches on-demand in the visible text and highlights matches.
// isCursorRow indicates if this row is at the cursor position.
// currentIdx is the index of the current match (0 = first match).
// side is which side is being rendered (0=new/left, 1=old/right), currentSide is which side has the current match.
func (m Model) highlightSearchInVisible(visible string, isCursorRow bool, currentIdx, side, currentSide int) string {
	return m.highlightSearchInVisibleStyled(visible, isCursorRow, currentIdx, side, currentSide, nil)
}

// highlightSearchInVisibleStyled highlights search matches in visible text,
// applying baseStyler to non-matched segments. If baseStyler is nil,
// non-matched text is left unstyled.
func (m Model) highlightSearchInVisibleStyled(visible string, isCursorRow bool, currentIdx, side, currentSide int, baseStyler SearchBaseStyler) string {
	if m.searchQuery == "" {
		if baseStyler != nil {
			return baseStyler(visible, 0)
		}
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
		seg := visible[lastEnd:pos]
		if baseStyler != nil {
			result.WriteString(baseStyler(seg, lastEnd))
		} else {
			result.WriteString(seg)
		}

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
		seg := visible[lastEnd:]
		if baseStyler != nil {
			result.WriteString(baseStyler(seg, lastEnd))
		} else {
			result.WriteString(seg)
		}
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

// formatColoredStatsBar returns the stats display with colored +/- counts.
// Returns empty string if no changes. Format: " +N -M"
func formatColoredStatsBar(added, removed int) string {
	if added == 0 && removed == 0 {
		return ""
	}

	var parts []string

	if added > 0 {
		parts = append(parts, addedStyle.Render(fmt.Sprintf("+%d", added)))
	}

	if removed > 0 {
		parts = append(parts, removedStyle.Render(fmt.Sprintf("-%d", removed)))
	}

	return " " + strings.Join(parts, " ")
}

// fileHeaderBoxWidth computes the header box width for a single file based on its own content.
// Used for headers so the border hugs the actual content.
func fileHeaderBoxWidth(headerText string, added, removed int) int {
	iconPartWidth := 3 + 1 + 1                                                                 // "   ◐ "
	return iconPartWidth + displayWidth(headerText) + statsBarDisplayWidth(added, removed) + 1 // +1 for gap before ┏
}

// statsBarDisplayWidth returns the display width of the stats counts (without ANSI codes).
// This matches formatColoredStatsBar's output width.
func statsBarDisplayWidth(added, removed int) int {
	if added == 0 && removed == 0 {
		return 0
	}

	// Format: " +N -M"
	// Leading space
	width := 1

	if added > 0 {
		width += len(fmt.Sprintf("+%d", added))
	}

	// Space between +N and -M (only when both exist)
	if added > 0 && removed > 0 {
		width++
	}

	if removed > 0 {
		width += len(fmt.Sprintf("-%d", removed))
	}

	return width
}
