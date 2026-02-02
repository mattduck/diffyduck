package tui

import (
	"strings"
	"unicode"

	"github.com/user/diffyduck/pkg/sidebyside"
)

// isSmartCaseSensitive returns true if the query should be matched case-sensitively.
// Smart-case: lowercase query = insensitive, any uppercase = sensitive.
func isSmartCaseSensitive(query string) bool {
	for _, r := range query {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// searchableText returns the text to search for a given row and side.
// For the old side (side=1), only returns text for removed lines (- and ~ lines).
// For the new side (side=0) and headers, returns the full text.
// For comment rows, returns the specific line displayed by that row (only on side 0).
// Comment border rows (top/bottom) return empty string since they don't display text.
func searchableText(row displayRow, side int) string {
	// Comment rows - only searchable on side 0 (new/left side)
	// Only content rows are searchable, not border rows
	if row.kind == RowKindComment {
		if side == 0 && row.commentLineIndex >= 0 {
			// Return only the specific line this row displays
			lines := strings.Split(row.commentText, "\n")
			if row.commentLineIndex < len(lines) {
				return lines[row.commentLineIndex]
			}
		}
		return ""
	}

	if row.isHeader {
		if side == 0 {
			return row.header // Already formatted
		}
		return "" // Don't search headers on side 1
	}

	// Commit header rows - search the subject (side 0 only)
	if row.kind == RowKindCommitHeader {
		if side == 0 && row.commitIndex >= 0 {
			return row.commitHeaderSearchText
		}
		return ""
	}

	// Commit body rows - search the line text (side 0 only)
	if row.kind == RowKindCommitBody {
		if side == 0 {
			return row.commitBodyLine
		}
		return ""
	}

	// Commit info body rows - search the line text (side 0 only)
	if row.kind == RowKindCommitInfoBody {
		if side == 0 {
			return row.commitInfoLine
		}
		return ""
	}

	if side == 0 {
		// New side (left) - always searchable
		return row.pair.New.Content
	}

	// Old side (right) - only search if it's a removed line (includes changed lines)
	if row.pair.Old.Type == sidebyside.Removed {
		return row.pair.Old.Content
	}
	return "" // Skip context lines on old side
}

// findMatchColsOnRowSide finds all match column positions on a given row for a specific side.
// side: 0 = new/left side, 1 = old/right side
// Returns byte offsets of all matches in the searchable content for that side.
func (m Model) findMatchColsOnRowSide(rowIdx, side int) []int {
	if m.searchQuery == "" {
		return nil
	}

	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}

	if rowIdx < 0 || rowIdx >= len(rows) {
		return nil
	}

	row := rows[rowIdx]
	text := searchableText(row, side)
	if text == "" {
		return nil
	}

	caseSensitive := isSmartCaseSensitive(m.searchQuery)
	query := m.searchQuery
	if !caseSensitive {
		query = strings.ToLower(query)
	}

	searchIn := text
	if !caseSensitive {
		searchIn = strings.ToLower(text)
	}

	var cols []int
	start := 0
	for {
		idx := strings.Index(searchIn[start:], query)
		if idx == -1 {
			break
		}
		cols = append(cols, start+idx)
		start += idx + 1
	}

	return cols
}

// findMatchColsOnRow finds all match column positions on a given row (both sides combined).
// Returns byte offsets of all matches in the searchable content.
func (m Model) findMatchColsOnRow(rowIdx int) []int {
	var cols []int
	for side := 0; side <= 1; side++ {
		cols = append(cols, m.findMatchColsOnRowSide(rowIdx, side)...)
	}
	return cols
}

// rowHasMatchOnSide returns true if the given row has any matches on the specified side.
func (m Model) rowHasMatchOnSide(rowIdx, side int) bool {
	return len(m.findMatchColsOnRowSide(rowIdx, side)) > 0
}

// findNextMatchRow searches for the next row with a match starting from startRow.
// Returns (rowIndex, found). Searches in the direction specified.
// Respects fold state (only searches visible rows) and old-side filtering.
func (m Model) findNextMatchRow(startRow int, forward bool) (int, bool) {
	if m.searchQuery == "" {
		return 0, false
	}

	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}

	if len(rows) == 0 {
		return 0, false
	}

	caseSensitive := isSmartCaseSensitive(m.searchQuery)
	query := m.searchQuery
	if !caseSensitive {
		query = strings.ToLower(query)
	}

	// Determine search range and direction
	var indices []int
	if forward {
		for i := startRow; i < len(rows); i++ {
			indices = append(indices, i)
		}
	} else {
		for i := startRow; i >= 0; i-- {
			indices = append(indices, i)
		}
	}

	for _, rowIdx := range indices {
		row := rows[rowIdx]

		// Search both sides of this row
		for side := 0; side <= 1; side++ {
			text := searchableText(row, side)
			if text == "" {
				continue
			}

			searchIn := text
			if !caseSensitive {
				searchIn = strings.ToLower(text)
			}

			if strings.Contains(searchIn, query) {
				return rowIdx, true
			}
		}
	}

	return 0, false
}

// RowMatch represents a match found within a single row during rendering.
type RowMatch struct {
	Col       int  // byte offset within the content
	Len       int  // length of match in bytes
	IsCurrent bool // true if this is the currently active match
}

// findMatchesInText finds all occurrences of the search query in the given text.
// Returns matches with their byte offsets.
// isCursorRow indicates if this is the cursor row.
// currentIdx is the index of the current match (0 = first match).
func (m Model) findMatchesInText(text string, isCursorRow bool, currentIdx int) []RowMatch {
	if m.searchQuery == "" || text == "" {
		return nil
	}

	caseSensitive := isSmartCaseSensitive(m.searchQuery)
	query := m.searchQuery
	searchIn := text
	if !caseSensitive {
		searchIn = strings.ToLower(text)
		query = strings.ToLower(query)
	}

	var matches []RowMatch
	start := 0
	matchIdx := 0
	for {
		idx := strings.Index(searchIn[start:], query)
		if idx == -1 {
			break
		}
		col := start + idx

		// Determine if this is the current match by index
		isCurrent := isCursorRow && matchIdx == currentIdx

		matches = append(matches, RowMatch{
			Col:       col,
			Len:       len(m.searchQuery),
			IsCurrent: isCurrent,
		})
		start = col + 1 // Move past this match to find overlapping matches
		matchIdx++
	}

	return matches
}

// executeSearch runs the search with the current input.
// Finds the first match from the cursor position and jumps to it.
func (m *Model) executeSearch() {
	m.searchQuery = m.searchInput
	m.searchInput = ""
	m.searchMode = false
	m.searchMatchIdx = 0  // Reset to first match
	m.searchMatchSide = 0 // Start on new side (left)

	if m.searchQuery == "" {
		return
	}

	// Find first match from current cursor position
	cursorRow := m.cursorLine()
	if row, found := m.findNextMatchRow(cursorRow, m.searchForward); found {
		m.adjustScrollToRow(row)
		m.searchMatchIdx = 0
		// Start on whichever side has matches (prefer new side)
		if m.rowHasMatchOnSide(row, 0) {
			m.searchMatchSide = 0
		} else {
			m.searchMatchSide = 1
		}
	}
}

// cancelSearch exits search mode without executing.
func (m *Model) cancelSearch() {
	m.searchMode = false
	m.searchInput = ""
}

// nextMatch moves to the next match in the search direction.
// Cycles through matches on current side, then other side, then next row.
// Returns true if a match was found.
func (m *Model) nextMatch() bool {
	if m.searchQuery == "" {
		return false
	}

	cursorRow := m.cursorLine()
	currentSideCount := len(m.findMatchColsOnRowSide(cursorRow, m.searchMatchSide))

	if m.searchForward {
		// Forward search: try next match on current side first
		if currentSideCount > 0 && m.searchMatchIdx < currentSideCount-1 {
			m.searchMatchIdx++
			return true
		}

		// Try other side of same row, but only if we haven't visited it yet
		// (side 0 -> side 1 is allowed, but side 1 -> side 0 means we've done both)
		otherSide := 1 - m.searchMatchSide
		if otherSide > m.searchMatchSide {
			otherSideCount := len(m.findMatchColsOnRowSide(cursorRow, otherSide))
			if otherSideCount > 0 {
				m.searchMatchSide = otherSide
				m.searchMatchIdx = 0
				return true
			}
		}

		// No more matches on current row, go to next row
		if row, found := m.findNextMatchRow(cursorRow+1, true); found {
			m.adjustScrollToRow(row)
			m.searchMatchIdx = 0
			// Start on whichever side has matches (prefer new side)
			if m.rowHasMatchOnSide(row, 0) {
				m.searchMatchSide = 0
			} else {
				m.searchMatchSide = 1
			}
			return true
		}
	} else {
		// Backward search: "next" means previous in document order
		if currentSideCount > 0 && m.searchMatchIdx > 0 {
			m.searchMatchIdx--
			return true
		}

		// Try other side of same row, but only if we haven't visited it yet
		// (backward: side 1 -> side 0 is allowed, but side 0 -> side 1 means we've done both)
		otherSide := 1 - m.searchMatchSide
		if otherSide < m.searchMatchSide {
			otherSideCount := len(m.findMatchColsOnRowSide(cursorRow, otherSide))
			if otherSideCount > 0 {
				m.searchMatchSide = otherSide
				m.searchMatchIdx = otherSideCount - 1
				return true
			}
		}

		// No more matches on current row, go to previous row
		if row, found := m.findNextMatchRow(cursorRow-1, false); found {
			m.adjustScrollToRow(row)
			// Set to last match on last side that has matches
			if m.rowHasMatchOnSide(row, 1) {
				m.searchMatchSide = 1
				m.searchMatchIdx = len(m.findMatchColsOnRowSide(row, 1)) - 1
			} else {
				m.searchMatchSide = 0
				m.searchMatchIdx = max(0, len(m.findMatchColsOnRowSide(row, 0))-1)
			}
			return true
		}
	}

	return false
}

// prevMatch moves to the previous match (opposite of search direction).
// Cycles through matches on current side, then other side, then previous row.
// Returns true if a match was found.
func (m *Model) prevMatch() bool {
	if m.searchQuery == "" {
		return false
	}

	cursorRow := m.cursorLine()
	currentSideCount := len(m.findMatchColsOnRowSide(cursorRow, m.searchMatchSide))

	if m.searchForward {
		// Forward search mode: "prev" means go backward in matches
		if currentSideCount > 0 && m.searchMatchIdx > 0 {
			m.searchMatchIdx--
			return true
		}

		// Try other side of same row, but only if we haven't visited it yet
		// (backward: side 1 -> side 0 is allowed, but side 0 -> side 1 means we've done both)
		otherSide := 1 - m.searchMatchSide
		if otherSide < m.searchMatchSide {
			otherSideCount := len(m.findMatchColsOnRowSide(cursorRow, otherSide))
			if otherSideCount > 0 {
				m.searchMatchSide = otherSide
				m.searchMatchIdx = otherSideCount - 1
				return true
			}
		}

		// No more matches on current row, go to previous row
		if row, found := m.findNextMatchRow(cursorRow-1, false); found {
			m.adjustScrollToRow(row)
			// Set to last match on last side that has matches
			if m.rowHasMatchOnSide(row, 1) {
				m.searchMatchSide = 1
				m.searchMatchIdx = len(m.findMatchColsOnRowSide(row, 1)) - 1
			} else {
				m.searchMatchSide = 0
				m.searchMatchIdx = max(0, len(m.findMatchColsOnRowSide(row, 0))-1)
			}
			return true
		}
	} else {
		// Backward search mode: "prev" means go forward in matches
		if currentSideCount > 0 && m.searchMatchIdx < currentSideCount-1 {
			m.searchMatchIdx++
			return true
		}

		// Try other side of same row, but only if we haven't visited it yet
		// (forward: side 0 -> side 1 is allowed, but side 1 -> side 0 means we've done both)
		otherSide := 1 - m.searchMatchSide
		if otherSide > m.searchMatchSide {
			otherSideCount := len(m.findMatchColsOnRowSide(cursorRow, otherSide))
			if otherSideCount > 0 {
				m.searchMatchSide = otherSide
				m.searchMatchIdx = 0
				return true
			}
		}

		// No more matches on current row, go to next row
		if row, found := m.findNextMatchRow(cursorRow+1, true); found {
			m.adjustScrollToRow(row)
			m.searchMatchIdx = 0
			// Start on whichever side has matches (prefer new side)
			if m.rowHasMatchOnSide(row, 0) {
				m.searchMatchSide = 0
			} else {
				m.searchMatchSide = 1
			}
			return true
		}
	}

	return false
}

// resetSearchMatchForRow resets the search match state for the current cursor row.
// Call this when the cursor moves to a new row via j/k navigation.
// Sets matchIdx to 0 and matchSide to whichever side has the first match.
func (m *Model) resetSearchMatchForRow() {
	if m.searchQuery == "" {
		return
	}

	cursorRow := m.cursorLine()

	// Reset to first match on this row (prefer side 0)
	m.searchMatchIdx = 0
	if m.rowHasMatchOnSide(cursorRow, 0) {
		m.searchMatchSide = 0
	} else if m.rowHasMatchOnSide(cursorRow, 1) {
		m.searchMatchSide = 1
	}
}

// currentMatchIdx returns the current match index for rendering.
func (m Model) currentMatchIdx() int {
	return m.searchMatchIdx
}

// currentMatchSide returns the current match side for rendering (0=new/left, 1=old/right).
func (m Model) currentMatchSide() int {
	return m.searchMatchSide
}
