package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.focused = true
		return m, nil

	case tea.BlurMsg:
		m.focused = false
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		// Capture cursor row index before resize changes cursorOffset()
		// Row list is stable on resize (only rendering widths change, not row count),
		// so we can restore to the same absolute row index.
		savedRowIdx := m.cursorLine()

		m.width = msg.Width
		m.height = msg.Height

		// Set initial fold levels on first window size message
		if !m.initialFoldSet && len(m.files) > 0 {
			m.initialFoldSet = true
			// If only 1 file, or all content fits on screen, start unfolded
			if len(m.files) == 1 || m.estimateNormalRows() <= m.contentHeight() {
				for i := range m.files {
					m.files[i].FoldLevel = sidebyside.FoldNormal
				}
			} else {
				// Otherwise start folded
				for i := range m.files {
					m.files[i].FoldLevel = sidebyside.FoldFolded
				}
			}
			m.calculateTotalLines()
		}

		// Restore cursor to same row index
		m.adjustScrollToRow(savedRowIdx)

		// Start loading supported files on first window size
		cmd := m.initStartupQueue()
		return m, cmd

	case FileContentLoadedMsg:
		if msg.FileIndex >= 0 && msg.FileIndex < len(m.files) {
			// Capture cursor identity before content changes the row layout
			identity := m.getCursorRowIdentity()

			m.files[msg.FileIndex].OldContent = msg.OldContent
			m.files[msg.FileIndex].NewContent = msg.NewContent
			m.files[msg.FileIndex].ContentTruncated = msg.ContentTruncated
			m.files[msg.FileIndex].OldContentTruncated = msg.OldTruncated
			m.files[msg.FileIndex].NewContentTruncated = msg.NewTruncated
			m.calculateTotalLines()

			// Preserve scroll position
			newRowIdx := m.findRowOrNearestAbove(identity)
			m.adjustScrollToRow(newRowIdx)

			// File content loaded, but still loading until highlight is ready
			// (loading state will be cleared when HighlightReadyMsg arrives)

			// Trigger syntax highlighting for this file
			return m, m.RequestHighlight(msg.FileIndex)
		}
		return m, nil

	case HighlightReadyMsg:
		m.storeHighlightSpans(msg)
		// Clear loading state - file is fully loaded now
		m.clearFileLoading(msg.FileIndex)

		// Check if there are more files to load from startup queue
		var cmds []tea.Cmd
		if cmd := m.onStartupFileComplete(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.startSpinnerIfNeeded(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case PairsHighlightReadyMsg:
		m.storePairsHighlightSpans(msg)
		return m, nil

	case spinner.TickMsg:
		cmd := m.handleSpinnerTick(msg)
		return m, cmd

	case AllContentLoadedMsg:
		if len(msg.Contents) > 0 {
			// Capture cursor identity before content changes the row layout
			identity := m.getCursorRowIdentity()

			for _, fc := range msg.Contents {
				if fc.FileIndex >= 0 && fc.FileIndex < len(m.files) {
					m.files[fc.FileIndex].OldContent = fc.OldContent
					m.files[fc.FileIndex].NewContent = fc.NewContent
					m.files[fc.FileIndex].ContentTruncated = fc.ContentTruncated
					m.files[fc.FileIndex].OldContentTruncated = fc.OldTruncated
					m.files[fc.FileIndex].NewContentTruncated = fc.NewTruncated
				}
			}
			m.calculateTotalLines()

			// Preserve scroll position
			newRowIdx := m.findRowOrNearestAbove(identity)
			m.adjustScrollToRow(newRowIdx)

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

	// Handle multi-key sequences (e.g., gg, gj, gk)
	if m.pendingKey == "g" {
		return m.handlePendingG(msg)
	}

	// Check for prefix keys that start multi-key sequences
	if msg.String() == "g" {
		m.pendingKey = "g"
		return m, nil
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
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.Down):
		m.scroll++
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.PageUp):
		m.scroll -= m.height
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.PageDown):
		m.scroll += m.height
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.HalfUp):
		m.scroll -= m.height / 2
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.HalfDown):
		m.scroll += m.height / 2
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.Top):
		m.scroll = m.minScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.Bottom):
		m.scroll = m.maxScroll()
		m.resetSearchMatchForRow()

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

	case matchesKey(msg, keys.Enter):
		return m.handleEnter()
	}

	return m, nil
}

// cursorRowIdentity captures the "identity" of the row the cursor is on.
// This is used to preserve scroll position across fold changes and resize.
// Using RowKind ensures this stays in sync with displayRow types automatically.
type cursorRowIdentity struct {
	kind      RowKind // row type - must match for non-content rows
	fileIndex int     // file this row belongs to (-1 for summary)
	// For blank rows, which blank row within the file's blank area (0-indexed)
	blankIndex int
	// For content rows, the line numbers to match
	oldNum int
	newNum int
}

// getCursorRowIdentity returns the identity of the row at the cursor position.
func (m Model) getCursorRowIdentity() cursorRowIdentity {
	// Use cached rows if valid, otherwise rebuild
	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}
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

	// For blank rows, count which blank row this is within the file's blank area
	blankIndex := 0
	if row.kind == RowKindBlank {
		for i := cursorPos - 1; i >= 0; i-- {
			if rows[i].kind == RowKindBlank && rows[i].fileIndex == row.fileIndex {
				blankIndex++
			} else {
				break
			}
		}
	}

	return cursorRowIdentity{
		kind:       row.kind,
		fileIndex:  row.fileIndex,
		blankIndex: blankIndex,
		oldNum:     row.pair.Old.Num,
		newNum:     row.pair.New.Num,
	}
}

// findRowOrNearestAbove finds the row matching identity, or the nearest header/separator above.
// Returns the line index of the found row.
func (m Model) findRowOrNearestAbove(identity cursorRowIdentity) int {
	// Use cached rows if valid, otherwise rebuild
	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}
	if len(rows) == 0 {
		return 0
	}

	// Track blanks seen per file for matching specific blank rows
	blanksSeen := 0
	lastFileIndex := -2 // Start with invalid value

	// First, try to find an exact match
	for i, row := range rows {
		// Reset blank counter when file changes
		if row.fileIndex != lastFileIndex {
			blanksSeen = 0
			lastFileIndex = row.fileIndex
		}

		if m.rowMatchesIdentity(row, identity, blanksSeen) {
			return i
		}

		// Count blanks after checking (so first blank has index 0)
		if row.kind == RowKindBlank && row.fileIndex == identity.fileIndex {
			blanksSeen++
		}
	}

	// No exact match - find the nearest header or separator above the original position
	// Walk through rows looking for the last header/separator at or before identity.fileIndex
	lastHeaderOrSep := 0
	for i, row := range rows {
		if row.fileIndex > identity.fileIndex {
			break // Past our file, stop searching
		}
		if row.kind == RowKindHeader || row.kind == RowKindSeparator {
			lastHeaderOrSep = i
		}
	}

	return lastHeaderOrSep
}

// rowMatchesIdentity checks if a row matches the given identity.
// For blank rows, blanksSeen tracks how many blanks we've seen for this file.
func (m Model) rowMatchesIdentity(row displayRow, identity cursorRowIdentity, blanksSeen int) bool {
	// File index must match
	if row.fileIndex != identity.fileIndex {
		return false
	}

	// For non-content rows, kind must match exactly
	switch identity.kind {
	case RowKindHeader:
		return row.kind == RowKindHeader
	case RowKindHeaderSpacer:
		return row.kind == RowKindHeaderSpacer
	case RowKindHeaderTopBorder:
		return row.kind == RowKindHeaderTopBorder
	case RowKindBlank:
		// Match the specific blank row by index within the file
		return row.kind == RowKindBlank && blanksSeen == identity.blankIndex
	case RowKindSeparatorTop:
		return row.kind == RowKindSeparatorTop
	case RowKindSeparator:
		return row.kind == RowKindSeparator
	case RowKindSeparatorBottom:
		return row.kind == RowKindSeparatorBottom
	case RowKindTruncationIndicator:
		return row.kind == RowKindTruncationIndicator
	case RowKindCommitHeader:
		return row.kind == RowKindCommitHeader
	case RowKindContent:
		// For content rows, match by line numbers
		// Handle cases where one side might be 0 (added/removed lines)
		if row.kind != RowKindContent {
			return false
		}
		if identity.oldNum > 0 && row.pair.Old.Num == identity.oldNum {
			return true
		}
		if identity.newNum > 0 && row.pair.New.Num == identity.newNum {
			return true
		}
		// If both are 0, no match (can't identify the row)
		return false
	}

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

// nextFoldLevel returns the next fold level, respecting pager mode.
// In pager mode, FoldExpanded is skipped since full file content is unavailable.
// Normal mode cycle: Normal -> Expanded -> Folded -> Normal
// Pager mode cycle:  Normal -> Folded -> Normal
func (m Model) nextFoldLevel(current sidebyside.FoldLevel) sidebyside.FoldLevel {
	next := current.NextLevel()
	if m.pagerMode && next == sidebyside.FoldExpanded {
		// Skip FoldExpanded in pager mode
		return next.NextLevel() // Returns FoldFolded
	}
	return next
}

// nextFoldLevelForFile returns the next fold level for a specific file.
// Like nextFoldLevel but also skips FoldExpanded for binary files.
func (m Model) nextFoldLevelForFile(fp sidebyside.FilePair) sidebyside.FoldLevel {
	next := fp.FoldLevel.NextLevel()
	if (m.pagerMode || fp.IsBinary) && next == sidebyside.FoldExpanded {
		// Skip FoldExpanded in pager mode or for binary files
		return next.NextLevel() // Returns FoldFolded
	}
	return next
}

// handleFoldToggle cycles the fold level of the current file, or commit if on commit header.
func (m Model) handleFoldToggle() (tea.Model, tea.Cmd) {
	// If cursor is on commit header, do commit fold cycle instead
	if m.isOnCommitHeader() {
		return m.handleCommitFoldCycle()
	}

	fileIdx := m.currentFileIndex()
	if fileIdx < 0 || fileIdx >= len(m.files) {
		return m, nil
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	newLevel := m.nextFoldLevelForFile(m.files[fileIdx])
	m.files[fileIdx].FoldLevel = newLevel
	m.calculateTotalLines()

	// Preserve scroll position
	newRowIdx := m.findRowOrNearestAbove(identity)
	m.adjustScrollToRow(newRowIdx)

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
		// All same - advance to next level (respecting pager mode)
		newLevel = m.nextFoldLevel(firstLevel)
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

	cursorLine := m.cursorLine()

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

// handlePendingG handles the second key after 'g' is pressed.
func (m Model) handlePendingG(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.pendingKey = "" // Always clear pending state

	switch msg.String() {
	case "g":
		// gg: go to top
		m.scroll = m.minScroll()
		m.resetSearchMatchForRow()
	case "j":
		// gj: next heading (file header)
		m.goToNextHeading()
		m.resetSearchMatchForRow()
	case "k":
		// gk: previous heading (file header)
		m.goToPrevHeading()
		m.resetSearchMatchForRow()
	}
	// Any other key just cancels the pending state without action

	return m, nil
}

// goToNextHeading moves the cursor to the next file header.
func (m *Model) goToNextHeading() {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	// Find the current file index
	currentFileIdx := -1
	if cursorPos >= 0 && cursorPos < len(rows) {
		currentFileIdx = rows[cursorPos].fileIndex
	}

	// Find the next file header after the current file
	for i, row := range rows {
		if row.isHeader && row.fileIndex > currentFileIdx {
			m.adjustScrollToRow(i)
			return
		}
	}
}

// goToPrevHeading moves the cursor to the current file's header if not already
// on it, or to the previous file's header if already on the current header.
func (m *Model) goToPrevHeading() {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	// Find the current file index and whether we're on header
	currentFileIdx := 0
	onHeader := false
	if cursorPos >= 0 && cursorPos < len(rows) {
		fi := rows[cursorPos].fileIndex
		onHeader = rows[cursorPos].isHeader
		if fi >= 0 {
			currentFileIdx = fi
		}
	}

	// If not on header, jump to current file's header first
	if !onHeader {
		for i, row := range rows {
			if row.isHeader && row.fileIndex == currentFileIdx {
				m.adjustScrollToRow(i)
				return
			}
		}
		return
	}

	// Already on header, find the header of the previous file
	targetFileIdx := currentFileIdx - 1
	if targetFileIdx < 0 {
		// Already at first file's header, stay there
		return
	}

	for i, row := range rows {
		if row.isHeader && row.fileIndex == targetFileIdx {
			m.adjustScrollToRow(i)
			return
		}
	}
}

// handleEnter handles the Enter key.
// On hunk separator: (future) expand context
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	// TODO: Handle context expansion on hunk separators

	return m, nil
}

// isOnCommitHeader returns true if the cursor is on a commit header row.
func (m Model) isOnCommitHeader() bool {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return false
	}

	return rows[cursorPos].isCommitHeader
}

// handleCommitFoldCycle cycles through 3 levels of commit visibility (org-mode style).
// Level 1 (Folded): Just the commit header row
// Level 2: File headings only (all files at FoldFolded)
// Level 3: File hunks visible (files at FoldNormal)
// Cycling: Level 1 -> Level 2 -> Level 3 -> Level 1
func (m Model) handleCommitFoldCycle() (tea.Model, tea.Cmd) {
	if len(m.commits) == 0 {
		return m, nil
	}

	commit := &m.commits[0]

	// Determine current level
	currentLevel := m.commitVisibilityLevel()

	switch currentLevel {
	case 1:
		// Level 1 -> Level 2: Show file headings only
		commit.FoldLevel = sidebyside.CommitNormal
		for i := range m.files {
			m.files[i].FoldLevel = sidebyside.FoldFolded
		}
	case 2:
		// Level 2 -> Level 3: Show file hunks
		for i := range m.files {
			m.files[i].FoldLevel = sidebyside.FoldNormal
		}
	default:
		// Level 3 -> Level 1: Collapse everything
		commit.FoldLevel = sidebyside.CommitFolded
		for i := range m.files {
			m.files[i].FoldLevel = sidebyside.FoldFolded
		}
	}

	m.calculateTotalLines()

	return m, nil
}

// commitVisibilityLevel returns the current visibility level (1, 2, or 3).
// Level 1: Commit is folded (only commit header visible)
// Level 2: Commit is normal, all files are FoldFolded (file headings only)
// Level 3: Any file is not FoldFolded (file content visible)
func (m Model) commitVisibilityLevel() int {
	if len(m.commits) == 0 {
		return 1
	}

	commit := m.commits[0]

	// Level 1: Commit itself is folded
	if commit.FoldLevel == sidebyside.CommitFolded {
		return 1
	}

	// Check if any file is expanded beyond FoldFolded
	for _, fp := range m.files {
		if fp.FoldLevel != sidebyside.FoldFolded {
			return 3
		}
	}

	// All files are FoldFolded
	return 2
}
