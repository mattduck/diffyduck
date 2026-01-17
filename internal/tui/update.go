package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScroll()
		return m, nil

	case FileContentLoadedMsg:
		if msg.FileIndex >= 0 && msg.FileIndex < len(m.files) {
			// Capture cursor identity before content changes the row layout
			identity := m.getCursorRowIdentity()

			m.files[msg.FileIndex].OldContent = msg.OldContent
			m.files[msg.FileIndex].NewContent = msg.NewContent
			m.calculateTotalLines()

			// Preserve scroll position
			newRowIdx := m.findRowOrNearestAbove(identity)
			m.adjustScrollToRow(newRowIdx)

			m.refreshSearch()

			// Trigger syntax highlighting for this file
			return m, m.RequestHighlight(msg.FileIndex)
		}
		return m, nil

	case HighlightReadyMsg:
		m.storeHighlightSpans(msg)
		return m, nil

	case PairsHighlightReadyMsg:
		m.storePairsHighlightSpans(msg)
		return m, nil

	case AllContentLoadedMsg:
		if len(msg.Contents) > 0 {
			// Capture cursor identity before content changes the row layout
			identity := m.getCursorRowIdentity()

			for _, fc := range msg.Contents {
				if fc.FileIndex >= 0 && fc.FileIndex < len(m.files) {
					m.files[fc.FileIndex].OldContent = fc.OldContent
					m.files[fc.FileIndex].NewContent = fc.NewContent
				}
			}
			m.calculateTotalLines()

			// Preserve scroll position
			newRowIdx := m.findRowOrNearestAbove(identity)
			m.adjustScrollToRow(newRowIdx)

			m.refreshSearch()

			// Trigger syntax highlighting for all files
			return m, m.RequestHighlightAll()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input mode separately
	if m.searchMode {
		return m.handleSearchInput(msg)
	}

	keys := m.keys

	switch {
	case matchesKey(msg, keys.Quit):
		return m, tea.Quit

	case matchesKey(msg, keys.SearchForward):
		m.searchMode = true
		m.searchForward = true
		m.searchInput = ""
		return m, nil

	case matchesKey(msg, keys.SearchBack):
		m.searchMode = true
		m.searchForward = false
		m.searchInput = ""
		return m, nil

	case matchesKey(msg, keys.NextMatch):
		m.nextMatch()
		return m, nil

	case matchesKey(msg, keys.PrevMatch):
		m.prevMatch()
		return m, nil

	case matchesKey(msg, keys.Up):
		m.scroll--
		m.clampScroll()

	case matchesKey(msg, keys.Down):
		m.scroll++
		m.clampScroll()

	case matchesKey(msg, keys.PageUp):
		m.scroll -= m.height
		m.clampScroll()

	case matchesKey(msg, keys.PageDown):
		m.scroll += m.height
		m.clampScroll()

	case matchesKey(msg, keys.HalfUp):
		m.scroll -= m.height / 2
		m.clampScroll()

	case matchesKey(msg, keys.HalfDown):
		m.scroll += m.height / 2
		m.clampScroll()

	case matchesKey(msg, keys.Top):
		m.scroll = m.minScroll()

	case matchesKey(msg, keys.Bottom):
		m.scroll = m.maxScroll()

	case matchesKey(msg, keys.Left):
		m.hscroll -= m.hscrollStep
		if m.hscroll < 0 {
			m.hscroll = 0
		}

	case matchesKey(msg, keys.Right):
		m.hscroll += m.hscrollStep

	case matchesKey(msg, keys.FoldToggle):
		return m.handleFoldToggle()

	case matchesKey(msg, keys.FoldToggleAll):
		return m.handleFoldToggleAll()
	}

	return m, nil
}

// cursorRowIdentity captures the "identity" of the row the cursor is on.
// This is used to preserve scroll position across fold changes.
type cursorRowIdentity struct {
	fileIndex   int
	isHeader    bool
	isBlank     bool
	isSeparator bool
	// For content rows, the line numbers to match
	leftNum  int
	rightNum int
}

// getCursorRowIdentity returns the identity of the row at the cursor position.
func (m Model) getCursorRowIdentity() cursorRowIdentity {
	rows := m.buildRows()
	cursorPos := m.cursorLine()

	// Clamp to valid range
	if cursorPos < 0 {
		cursorPos = 0
	}
	if cursorPos >= len(rows) {
		cursorPos = len(rows) - 1
	}
	if cursorPos < 0 {
		return cursorRowIdentity{}
	}

	row := rows[cursorPos]
	return cursorRowIdentity{
		fileIndex:   row.fileIndex,
		isHeader:    row.isHeader,
		isBlank:     row.isBlank,
		isSeparator: row.isSeparator,
		leftNum:     row.pair.Left.Num,
		rightNum:    row.pair.Right.Num,
	}
}

// findRowOrNearestAbove finds the row matching identity, or the nearest header/separator above.
// Returns the line index of the found row.
func (m Model) findRowOrNearestAbove(identity cursorRowIdentity) int {
	rows := m.buildRows()
	if len(rows) == 0 {
		return 0
	}

	// First, try to find an exact match
	for i, row := range rows {
		if m.rowMatchesIdentity(row, identity) {
			return i
		}
	}

	// No exact match - find the nearest header or separator above the original position
	// Walk through rows looking for the last header/separator at or before identity.fileIndex
	lastHeaderOrSep := 0
	for i, row := range rows {
		if row.fileIndex > identity.fileIndex {
			break // Past our file, stop searching
		}
		if row.isHeader || row.isSeparator {
			lastHeaderOrSep = i
		}
	}

	return lastHeaderOrSep
}

// rowMatchesIdentity checks if a row matches the given identity.
func (m Model) rowMatchesIdentity(row displayRow, identity cursorRowIdentity) bool {
	// File index must match
	if row.fileIndex != identity.fileIndex {
		return false
	}

	// Type must match
	if identity.isHeader {
		return row.isHeader
	}
	if identity.isBlank {
		return row.isBlank
	}
	if identity.isSeparator {
		return row.isSeparator
	}

	// For content rows, match by line numbers
	// Handle cases where one side might be 0 (added/removed lines)
	if identity.leftNum > 0 && row.pair.Left.Num == identity.leftNum {
		return true
	}
	if identity.rightNum > 0 && row.pair.Right.Num == identity.rightNum {
		return true
	}
	// If both are 0, no match (can't identify the row)
	return false
}

// adjustScrollToRow adjusts scroll so the cursor points to the given row index.
func (m *Model) adjustScrollToRow(rowIndex int) {
	// cursor = scroll + cursorOffset
	// We want cursor = rowIndex
	// So: scroll = rowIndex - cursorOffset
	m.scroll = rowIndex - m.cursorOffset()
	m.clampScroll()
}

// handleFoldToggle cycles the fold level of the current file.
func (m Model) handleFoldToggle() (tea.Model, tea.Cmd) {
	fileIdx := m.currentFileIndex()
	if fileIdx < 0 || fileIdx >= len(m.files) {
		return m, nil
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	newLevel := m.files[fileIdx].FoldLevel.NextLevel()
	m.files[fileIdx].FoldLevel = newLevel
	m.calculateTotalLines()

	// Preserve scroll position
	newRowIdx := m.findRowOrNearestAbove(identity)
	m.adjustScrollToRow(newRowIdx)

	m.refreshSearch()

	// If expanding to full view and content not loaded, fetch it
	if newLevel == sidebyside.FoldExpanded && !m.files[fileIdx].HasContent() {
		return m, m.FetchFileContent(fileIdx)
	}

	return m, nil
}

// handleFoldToggleAll cycles the fold level for all files.
// If all files are at the same level, advance to next level.
// If files are at different levels, collapse all to FoldFolded.
func (m Model) handleFoldToggleAll() (tea.Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m, nil
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	// Check if all files are at the same level
	firstLevel := m.files[0].FoldLevel
	allSame := true
	for _, fp := range m.files[1:] {
		if fp.FoldLevel != firstLevel {
			allSame = false
			break
		}
	}

	var newLevel sidebyside.FoldLevel
	if allSame {
		// All same - advance to next level
		newLevel = firstLevel.NextLevel()
	} else {
		// Different levels - collapse all to Folded
		newLevel = sidebyside.FoldFolded
	}

	for i := range m.files {
		m.files[i].FoldLevel = newLevel
	}

	m.calculateTotalLines()

	// Preserve scroll position
	newRowIdx := m.findRowOrNearestAbove(identity)
	m.adjustScrollToRow(newRowIdx)

	m.refreshSearch()

	// If expanding to full view, fetch content for files that don't have it
	if newLevel == sidebyside.FoldExpanded {
		// Check if any files need content
		needsFetch := false
		for _, fp := range m.files {
			if !fp.HasContent() {
				needsFetch = true
				break
			}
		}
		if needsFetch {
			return m, m.FetchAllFileContent()
		}
	}

	return m, nil
}

// currentFileIndex returns the index of the file at the cursor position.
// Returns -1 if cursor is on the summary row (no file association).
// This matches what the status bar displays.
func (m Model) currentFileIndex() int {
	if len(m.files) == 0 {
		return -1
	}

	// Build rows to check if cursor is on summary row
	rows := m.buildRows()
	cursorLine := m.cursorLine()
	if cursorLine >= 0 && cursorLine < len(rows) && rows[cursorLine].isSummary {
		return -1 // Summary row has no associated file
	}

	// Use cursor position, not scroll position
	// This ensures Tab acts on the file shown in the status bar
	fileIdx, _ := m.fileAtLine(cursorLine)
	return fileIdx - 1 // fileAtLine returns 1-based, we need 0-based
}

// handleSearchInput handles keypresses while in search input mode.
func (m Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.executeSearch()
		return m, nil

	case tea.KeyEsc:
		m.cancelSearch()
		return m, nil

	case tea.KeyBackspace:
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
		return m, nil

	case tea.KeyRunes:
		m.searchInput += string(msg.Runes)
		return m, nil
	}

	return m, nil
}
