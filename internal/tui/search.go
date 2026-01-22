package tui

import (
	"strings"
	"unicode"
)

// Match represents a search match location.
type Match struct {
	Row  int // display row index
	Col  int // column offset within the line content
	Side int // 0 = left/header, 1 = right
}

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

// findMatches finds all matches for the current search query in the displayable content.
func (m Model) findMatches(query string) []Match {
	if query == "" {
		return nil
	}

	var matches []Match
	caseSensitive := isSmartCaseSensitive(query)

	searchQuery := query
	if !caseSensitive {
		searchQuery = strings.ToLower(query)
	}

	// Use cached rows if valid, otherwise rebuild
	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}

	for rowIdx, row := range rows {
		if row.isHeader {
			// Search in header
			header := formatFileHeader(row.header, row.header)
			matches = append(matches, findInString(header, searchQuery, caseSensitive, rowIdx, 0)...)
		} else {
			// Search in both new and old content
			// New content is on left (side 0), Old content is on right (side 1)
			newContent := row.pair.New.Content
			oldContent := row.pair.Old.Content

			matches = append(matches, findInString(newContent, searchQuery, caseSensitive, rowIdx, 0)...)

			// Only search old side if it differs from new (avoid duplicates for context lines)
			if oldContent != newContent {
				matches = append(matches, findInString(oldContent, searchQuery, caseSensitive, rowIdx, 1)...)
			}
		}
	}

	return matches
}

// findInString finds all occurrences of query in s.
func findInString(s, query string, caseSensitive bool, row, side int) []Match {
	var matches []Match

	searchIn := s
	if !caseSensitive {
		searchIn = strings.ToLower(s)
	}

	start := 0
	for {
		idx := strings.Index(searchIn[start:], query)
		if idx == -1 {
			break
		}
		matches = append(matches, Match{
			Row:  row,
			Col:  start + idx,
			Side: side,
		})
		start += idx + 1 // move past this match to find overlapping matches
	}

	return matches
}

// scrollToMatch scrolls the view so that the given match is visible.
func (m *Model) scrollToMatch(matchIdx int) {
	if matchIdx < 0 || matchIdx >= len(m.matches) {
		return
	}

	match := m.matches[matchIdx]
	contentH := m.contentHeight()

	// If match is above viewport, scroll up to it
	if match.Row < m.scroll {
		m.scroll = match.Row
	}

	// If match is below viewport, scroll down so it's visible
	if match.Row >= m.scroll+contentH {
		m.scroll = match.Row - contentH + 1
	}

	m.clampScroll()
}

// nextMatch moves to the next match in the search direction.
// If scroll position changed since last search nav, finds match from current position.
// Otherwise, moves to next match without wrap.
// Returns true if moved to a new match.
func (m *Model) nextMatch() bool {
	if len(m.matches) == 0 {
		return false
	}

	scrollChanged := m.scroll != m.lastSearchScroll

	if m.searchForward {
		if scrollChanged {
			// Scroll changed: find first match at or after current scroll
			for i, match := range m.matches {
				if match.Row >= m.scroll {
					m.currentMatch = i
					m.scrollToMatch(m.currentMatch)
					m.lastSearchScroll = m.scroll
					return true
				}
			}
		} else {
			// No scroll change: just go to next match (no wrap)
			if m.currentMatch < len(m.matches)-1 {
				m.currentMatch++
				m.scrollToMatch(m.currentMatch)
				m.lastSearchScroll = m.scroll
				return true
			}
		}
	} else {
		if scrollChanged {
			// Scroll changed: find last match at or before current scroll
			for i := len(m.matches) - 1; i >= 0; i-- {
				if m.matches[i].Row <= m.scroll+m.contentHeight()-1 {
					m.currentMatch = i
					m.scrollToMatch(m.currentMatch)
					m.lastSearchScroll = m.scroll
					return true
				}
			}
		} else {
			// No scroll change: go to previous match (no wrap)
			if m.currentMatch > 0 {
				m.currentMatch--
				m.scrollToMatch(m.currentMatch)
				m.lastSearchScroll = m.scroll
				return true
			}
		}
	}

	return false
}

// prevMatch moves to the previous match (opposite of search direction).
// If scroll position changed since last search nav, finds match from current position.
// Otherwise, moves to previous match without wrap.
// Returns true if moved to a new match.
func (m *Model) prevMatch() bool {
	if len(m.matches) == 0 {
		return false
	}

	scrollChanged := m.scroll != m.lastSearchScroll

	if m.searchForward {
		if scrollChanged {
			// Scroll changed: find last match at or before viewport bottom
			for i := len(m.matches) - 1; i >= 0; i-- {
				if m.matches[i].Row <= m.scroll+m.contentHeight()-1 {
					m.currentMatch = i
					m.scrollToMatch(m.currentMatch)
					m.lastSearchScroll = m.scroll
					return true
				}
			}
		} else {
			// No scroll change: go to previous match (no wrap)
			if m.currentMatch > 0 {
				m.currentMatch--
				m.scrollToMatch(m.currentMatch)
				m.lastSearchScroll = m.scroll
				return true
			}
		}
	} else {
		if scrollChanged {
			// Scroll changed: find first match at or after current scroll
			for i, match := range m.matches {
				if match.Row >= m.scroll {
					m.currentMatch = i
					m.scrollToMatch(m.currentMatch)
					m.lastSearchScroll = m.scroll
					return true
				}
			}
		} else {
			// No scroll change: go to next match (no wrap)
			if m.currentMatch < len(m.matches)-1 {
				m.currentMatch++
				m.scrollToMatch(m.currentMatch)
				m.lastSearchScroll = m.scroll
				return true
			}
		}
	}

	return false
}

// executeSearch runs the search with the current input.
func (m *Model) executeSearch() {
	m.searchQuery = m.searchInput
	m.searchInput = ""
	m.searchMode = false

	if m.searchQuery == "" {
		m.matches = nil
		m.currentMatch = 0
		return
	}

	m.matches = m.findMatches(m.searchQuery)
	m.currentMatch = 0

	if len(m.matches) > 0 {
		// Find first match at or after current scroll position
		for i, match := range m.matches {
			if match.Row >= m.scroll {
				m.currentMatch = i
				break
			}
		}
		m.scrollToMatch(m.currentMatch)
		m.lastSearchScroll = m.scroll
	}
}

// cancelSearch exits search mode without executing.
func (m *Model) cancelSearch() {
	m.searchMode = false
	m.searchInput = ""
}

// refreshSearch re-runs the current search query.
// This should be called when the displayable content changes (e.g., fold toggle).
func (m *Model) refreshSearch() {
	if m.searchQuery == "" {
		return
	}

	// Re-find matches with the current query
	m.matches = m.findMatches(m.searchQuery)

	// If we had a currentMatch, try to find the closest match to the current scroll
	if len(m.matches) > 0 {
		// Find first match at or after current scroll position
		found := false
		for i, match := range m.matches {
			if match.Row >= m.scroll {
				m.currentMatch = i
				found = true
				break
			}
		}
		// If no match after scroll, use the last match
		if !found {
			m.currentMatch = len(m.matches) - 1
		}
	} else {
		m.currentMatch = 0
	}
}
