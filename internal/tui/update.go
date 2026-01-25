package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/diff"
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

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scroll -= 3
			m.clampScroll()
			m.resetSearchMatchForRow()
		case tea.MouseButtonWheelDown:
			m.scroll += 3
			m.clampScroll()
			m.resetSearchMatchForRow()
		}
		return m, nil

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
			// Check if loading this file's content affects visible rows:
			// - If the file's commit is folded, files aren't shown
			// - If the file itself isn't in FoldExpanded mode, content isn't shown
			// In these cases, skip expensive scroll preservation
			commitIdx := m.commitForFile(msg.FileIndex)
			commitFolded := commitIdx >= 0 && commitIdx < len(m.commits) &&
				m.commits[commitIdx].Info.HasMetadata() &&
				m.commits[commitIdx].FoldLevel == sidebyside.CommitFolded
			fileExpanded := m.files[msg.FileIndex].FoldLevel == sidebyside.FoldExpanded
			affectsVisibleRows := !commitFolded && fileExpanded

			// Capture cursor identity before content changes the row layout
			var identity cursorRowIdentity
			if affectsVisibleRows {
				identity = m.getCursorRowIdentity()
			}

			m.files[msg.FileIndex].OldContent = msg.OldContent
			m.files[msg.FileIndex].NewContent = msg.NewContent
			m.files[msg.FileIndex].ContentTruncated = msg.ContentTruncated
			m.files[msg.FileIndex].OldContentTruncated = msg.OldTruncated
			m.files[msg.FileIndex].NewContentTruncated = msg.NewTruncated
			m.calculateTotalLines()

			// Preserve scroll position only if the loaded file affects visible rows
			if affectsVisibleRows {
				newRowIdx := m.findRowOrNearestAbove(identity)
				m.adjustScrollToRow(newRowIdx)
			}

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

	case ClearStatusMsg:
		// Only clear if this message matches the current message time
		// (prevents clearing a newer message set after this timer started)
		if m.statusMessage != "" && m.statusMessageTime == msg.SetTime {
			m.statusMessage = ""
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle comment input mode first (highest priority)
	if m.commentMode {
		return m.handleCommentInput(msg)
	}

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
		// Enter starts comment mode on commentable lines
		if m.startComment() {
			return m, nil
		}
		return m.handleEnter()

	case matchesKey(msg, keys.Yank):
		return m.handleYank()

	case matchesKey(msg, keys.RefreshLayout):
		m.RefreshLayout()
	}

	return m, nil
}

// cursorRowIdentity captures the "identity" of the row the cursor is on.
// This is used to preserve scroll position across fold changes and resize.
// Using RowKind ensures this stays in sync with displayRow types automatically.
type cursorRowIdentity struct {
	kind        RowKind // row type - must match for non-content rows
	fileIndex   int     // file this row belongs to (-1 for commit rows)
	commitIndex int     // commit this row belongs to (for commit headers/body)
	// For blank rows, which blank row within the file's blank area (0-indexed)
	blankIndex int
	// For commit body rows, which body row within the commit (0-indexed)
	commitBodyIndex int
	// For structural diff rows, which row within the file's structural diff area (0-indexed)
	structuralDiffIndex int
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

	// For commit body rows, count which body row this is within the commit
	commitBodyIndex := 0
	if row.kind == RowKindCommitBody {
		for i := cursorPos - 1; i >= 0; i-- {
			if rows[i].kind == RowKindCommitBody && rows[i].commitIndex == row.commitIndex {
				commitBodyIndex++
			} else {
				break
			}
		}
	}

	// For structural diff rows, count which row this is within the file's structural diff area
	structuralDiffIndex := 0
	if row.kind == RowKindStructuralDiff {
		for i := cursorPos - 1; i >= 0; i-- {
			if rows[i].kind == RowKindStructuralDiff && rows[i].fileIndex == row.fileIndex {
				structuralDiffIndex++
			} else {
				break
			}
		}
	}

	return cursorRowIdentity{
		kind:                row.kind,
		fileIndex:           row.fileIndex,
		commitIndex:         row.commitIndex,
		blankIndex:          blankIndex,
		commitBodyIndex:     commitBodyIndex,
		structuralDiffIndex: structuralDiffIndex,
		oldNum:              row.pair.Old.Num,
		newNum:              row.pair.New.Num,
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
	lastFileIndexForBlanks := -2 // Start with invalid value

	// Track commit body rows seen per commit for matching specific body rows
	commitBodySeen := 0
	lastCommitIndex := -2 // Start with invalid value

	// Track structural diff rows seen per file for matching specific rows
	structuralDiffSeen := 0
	lastFileIndexForStructDiff := -2 // Start with invalid value

	// First, try to find an exact match
	for i, row := range rows {
		// Reset blank counter when file changes
		if row.fileIndex != lastFileIndexForBlanks {
			blanksSeen = 0
			lastFileIndexForBlanks = row.fileIndex
		}

		// Reset commit body counter when commit changes
		if row.commitIndex != lastCommitIndex {
			commitBodySeen = 0
			lastCommitIndex = row.commitIndex
		}

		// Reset structural diff counter when file changes
		if row.fileIndex != lastFileIndexForStructDiff {
			structuralDiffSeen = 0
			lastFileIndexForStructDiff = row.fileIndex
		}

		if m.rowMatchesIdentity(row, identity, blanksSeen, commitBodySeen, structuralDiffSeen) {
			return i
		}

		// Count blanks after checking (so first blank has index 0)
		if row.kind == RowKindBlank && row.fileIndex == identity.fileIndex {
			blanksSeen++
		}

		// Count commit body rows after checking (so first body row has index 0)
		if row.kind == RowKindCommitBody && row.commitIndex == identity.commitIndex {
			commitBodySeen++
		}

		// Count structural diff rows after checking (so first row has index 0)
		if row.kind == RowKindStructuralDiff && row.fileIndex == identity.fileIndex {
			structuralDiffSeen++
		}
	}

	// No exact match - find the nearest header or separator above the original position
	// For commit rows, find the commit header; for file rows, find the file header
	lastHeaderOrSep := 0
	for i, row := range rows {
		// For commit-related rows (fileIndex == -1), stop when we pass the target commit
		if identity.fileIndex == -1 {
			if row.commitIndex > identity.commitIndex {
				break
			}
			if row.kind == RowKindCommitHeader && row.commitIndex == identity.commitIndex {
				lastHeaderOrSep = i
			}
		} else {
			// For file rows, stop when we pass the target file
			if row.fileIndex > identity.fileIndex {
				break
			}
			if row.kind == RowKindHeader || row.kind == RowKindSeparator {
				lastHeaderOrSep = i
			}
		}
	}

	return lastHeaderOrSep
}

// rowMatchesIdentity checks if a row matches the given identity.
// For blank rows, blanksSeen tracks how many blanks we've seen for this file.
// For commit body rows, commitBodySeen tracks how many body rows we've seen for this commit.
// For structural diff rows, structuralDiffSeen tracks how many rows we've seen for this file.
func (m Model) rowMatchesIdentity(row displayRow, identity cursorRowIdentity, blanksSeen, commitBodySeen, structuralDiffSeen int) bool {
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
		return row.kind == RowKindCommitHeader && row.commitIndex == identity.commitIndex
	case RowKindCommitHeaderTopBorder:
		return row.kind == RowKindCommitHeaderTopBorder && row.commitIndex == identity.commitIndex
	case RowKindCommitHeaderBottomBorder:
		return row.kind == RowKindCommitHeaderBottomBorder && row.commitIndex == identity.commitIndex
	case RowKindCommitBody:
		// Match the specific commit body row by index within the commit
		return row.kind == RowKindCommitBody && row.commitIndex == identity.commitIndex && commitBodySeen == identity.commitBodyIndex
	case RowKindStructuralDiff:
		// Match the specific structural diff row by index within the file
		return row.kind == RowKindStructuralDiff && structuralDiffSeen == identity.structuralDiffIndex
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
	// In the new cursor model, scroll directly represents the cursor line
	m.scroll = rowIndex
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

// handleFoldToggle cycles the fold level of the current file or commit,
// but only when on the respective header row.
func (m Model) handleFoldToggle() (tea.Model, tea.Cmd) {
	// If cursor is on commit header, do commit fold cycle
	if m.isOnCommitHeader() {
		return m.handleCommitFoldCycle()
	}

	// Only toggle file fold when on file header
	if !m.isOnFileHeader() {
		return m, nil
	}

	fileIdx := m.currentFileIndex()
	if fileIdx < 0 || fileIdx >= len(m.files) {
		return m, nil
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	newLevel := m.nextFoldLevelForFile(m.files[fileIdx])
	m.files[fileIdx].FoldLevel = newLevel

	// If file is expanded beyond FoldFolded, update parent commit to CommitExpanded
	// Level 2 means "file headings only" - any file content means level 3
	if newLevel != sidebyside.FoldFolded && len(m.commits) > 0 {
		commitIdx := m.commitForFile(fileIdx)
		if commitIdx >= 0 && commitIdx < len(m.commits) {
			m.commits[commitIdx].FoldLevel = sidebyside.CommitExpanded
		}
	}

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

// handleFoldToggleAll cycles the fold level for all commits.
// Commit visibility levels:
//   - Level 1: CommitFolded (just commit header)
//   - Level 2: CommitNormal with all files at FoldFolded (commit + file headers)
//   - Level 3: CommitNormal with files at FoldNormal (commit + file headers + hunks)
//
// If all commits are at the same level, advance to next level.
// If commits are at different levels (mixed), collapse all to level 1.
func (m Model) handleFoldToggleAll() (tea.Model, tea.Cmd) {
	// Fall back to legacy file-based behavior if no commits
	if len(m.commits) == 0 {
		return m.handleFoldToggleAllFiles()
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	// Check the visibility level of all commits
	firstLevel := m.commitVisibilityLevelFor(0)
	allSame := true
	for i := 1; i < len(m.commits); i++ {
		if m.commitVisibilityLevelFor(i) != firstLevel {
			allSame = false
			break
		}
	}

	var newLevel int
	if allSame {
		// All same - advance to next level (1 -> 2 -> 3 -> 1)
		newLevel = firstLevel%3 + 1
	} else {
		// Mixed levels - reset all to level 1
		newLevel = 1
	}

	// Apply the new level to all commits
	m.setAllCommitsToLevel(newLevel)

	m.calculateTotalLines()

	// Preserve scroll position
	newRowIdx := m.findRowOrNearestAbove(identity)
	m.adjustScrollToRow(newRowIdx)

	// If expanding to level 2+, queue files for all commits
	var cmd tea.Cmd
	if newLevel >= 2 {
		cmd = m.queueFilesForAllCommits()
	}

	return m, cmd
}

// setAllCommitsToLevel sets all commits and their files to the specified visibility level.
// Level 1: CommitFolded, all files FoldFolded
// Level 2: CommitNormal, all files FoldFolded
// Level 3: CommitNormal, all files FoldNormal
func (m *Model) setAllCommitsToLevel(level int) {
	var commitFold sidebyside.CommitFoldLevel
	var fileFold sidebyside.FoldLevel

	switch level {
	case 1:
		commitFold = sidebyside.CommitFolded
		fileFold = sidebyside.FoldFolded
	case 2:
		commitFold = sidebyside.CommitNormal
		fileFold = sidebyside.FoldFolded
	case 3:
		commitFold = sidebyside.CommitNormal
		fileFold = sidebyside.FoldNormal
	default:
		commitFold = sidebyside.CommitFolded
		fileFold = sidebyside.FoldFolded
	}

	for i := range m.commits {
		m.commits[i].FoldLevel = commitFold
	}
	for i := range m.files {
		m.files[i].FoldLevel = fileFold
	}
}

// handleFoldToggleAllFiles is the legacy behavior for toggling all files
// when there are no commits (e.g., pager mode or tests that bypass commits).
func (m Model) handleFoldToggleAllFiles() (tea.Model, tea.Cmd) {
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
		// gj: next node (commit header or file header)
		m.goToNextHeading()
		m.resetSearchMatchForRow()
	case "k":
		// gk: previous node (commit header or file header)
		m.goToPrevHeading()
		m.resetSearchMatchForRow()
	}
	// Any other key just cancels the pending state without action

	return m, nil
}

// goToNextHeading moves the cursor to the next node (commit header or file header).
// A node is either a commit or a file. From any position within a node, gj jumps
// to the header of the next node in sequence.
// Special case: when on a top border, gj goes to that file's header (one line down).
func (m *Model) goToNextHeading() {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return
	}

	currentRow := rows[cursorPos]

	// Special case: when on a top border, go to the file's header (visually one line down)
	if currentRow.isHeaderTopBorder {
		for i := cursorPos + 1; i < len(rows); i++ {
			if rows[i].isHeader && rows[i].fileIndex == currentRow.fileIndex {
				m.adjustScrollToRow(i)
				return
			}
		}
		return
	}

	currentFileIdx := currentRow.fileIndex
	currentCommitIdx := currentRow.commitIndex
	inCommitSection := currentRow.isCommitHeader || currentRow.isCommitBody

	// Find the next header that belongs to a different node
	for i := cursorPos + 1; i < len(rows); i++ {
		row := rows[i]

		if row.isCommitHeader {
			// A commit header is always a different node (unless we're in that commit's header/body)
			if !inCommitSection || row.commitIndex != currentCommitIdx {
				m.adjustScrollToRow(i)
				return
			}
		}

		if row.isHeader {
			// A file header is a different node if:
			// - We're in a commit section (any file header is different from commit header)
			// - Or the file has a different index
			if inCommitSection || row.fileIndex != currentFileIdx {
				m.adjustScrollToRow(i)
				return
			}
		}
	}
}

// goToPrevHeading moves the cursor to the previous node (commit header or file header).
// If not on a node header, jumps to the current node's header.
// If already on a header (or top border), jumps to the previous node's header.
func (m *Model) goToPrevHeading() {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return
	}

	currentRow := rows[cursorPos]
	// Treat top border as part of the header section for navigation purposes
	onHeader := currentRow.isCommitHeader || currentRow.isHeader || currentRow.isHeaderTopBorder

	if !onHeader {
		// Not on a header - find the current node's header (could be commit or file)
		currentFileIdx := currentRow.fileIndex
		inCommitSection := currentRow.isCommitBody

		for i := cursorPos - 1; i >= 0; i-- {
			row := rows[i]
			if inCommitSection && row.isCommitHeader {
				m.adjustScrollToRow(i)
				return
			}
			if !inCommitSection && row.isHeader && row.fileIndex == currentFileIdx {
				m.adjustScrollToRow(i)
				return
			}
		}
		return
	}

	// Already on a header (or top border) - find the previous different node's header
	currentFileIdx := currentRow.fileIndex
	currentCommitIdx := currentRow.commitIndex
	isCommitHeader := currentRow.isCommitHeader

	for i := cursorPos - 1; i >= 0; i-- {
		row := rows[i]

		if row.isCommitHeader {
			// Found a commit header - it's a different node if different commit
			if !isCommitHeader || row.commitIndex != currentCommitIdx {
				m.adjustScrollToRow(i)
				return
			}
		}

		if row.isHeader {
			// Found a file header - it's a different node if:
			// - Current is a commit header (any file is different from commit)
			// - Or different file index
			if isCommitHeader || row.fileIndex != currentFileIdx {
				m.adjustScrollToRow(i)
				return
			}
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

// isOnFileHeader returns true if the cursor is on a file header row.
func (m Model) isOnFileHeader() bool {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return false
	}

	return rows[cursorPos].isHeader
}

// cursorCommitIndex returns the commit index for the row at cursor position.
// Returns -1 if cursor is not on a commit-related row.
func (m Model) cursorCommitIndex() int {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return -1
	}

	row := rows[cursorPos]
	if row.isCommitHeader || row.isCommitBody {
		return row.commitIndex
	}
	return -1
}

// isOnCommitSection returns true if the cursor is on any commit-related row
// (commit header, commit body, or commit header borders).
func (m Model) isOnCommitSection() bool {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return false
	}

	row := rows[cursorPos]
	return row.isCommitHeader || row.isCommitBody || row.isCommitHeaderBottomBorder || row.isCommitHeaderTopBorder
}

// handleCommitFoldCycle cycles through 3 levels of commit visibility (org-mode style).
// Level 1 (Folded): Just the commit header row
// Level 2: File headings only (all files at FoldFolded)
// Level 3: File hunks visible (files at FoldNormal)
// Cycling: Level 1 -> Level 2 -> Level 3 -> Level 1
func (m Model) handleCommitFoldCycle() (tea.Model, tea.Cmd) {
	commitIdx := m.cursorCommitIndex()
	// Fall back to commit 0 for backward compatibility (when cursor isn't on a commit header)
	if commitIdx < 0 {
		commitIdx = 0
	}
	if commitIdx >= len(m.commits) {
		return m, nil
	}

	commit := &m.commits[commitIdx]

	// Get file range for this commit
	startIdx := m.commitFileStarts[commitIdx]
	endIdx := len(m.files)
	if commitIdx+1 < len(m.commits) {
		endIdx = m.commitFileStarts[commitIdx+1]
	}

	// Determine current level for this commit
	currentLevel := m.commitVisibilityLevelFor(commitIdx)

	var cmd tea.Cmd

	switch currentLevel {
	case 1:
		// Level 1 -> Level 2: Show file headings only
		// Load diff content on demand if not already loaded
		if !commit.FilesLoaded && m.git != nil {
			m.loadCommitDiff(commitIdx)
			// Update endIdx since file count may have changed
			endIdx = len(m.files)
			if commitIdx+1 < len(m.commits) {
				endIdx = m.commitFileStarts[commitIdx+1]
			}
		}
		commit.FoldLevel = sidebyside.CommitNormal
		for i := startIdx; i < endIdx; i++ {
			m.files[i].FoldLevel = sidebyside.FoldFolded
		}
		// Queue files for loading now that the commit is expanded
		cmd = m.queueFilesForCommit(commitIdx)
	case 2:
		// Level 2 -> Level 3: Show file hunks
		commit.FoldLevel = sidebyside.CommitExpanded
		for i := startIdx; i < endIdx; i++ {
			m.files[i].FoldLevel = sidebyside.FoldNormal
		}
	default:
		// Level 3 -> Level 1: Collapse everything
		commit.FoldLevel = sidebyside.CommitFolded
		for i := startIdx; i < endIdx; i++ {
			m.files[i].FoldLevel = sidebyside.FoldFolded
		}
	}

	m.calculateTotalLines()

	return m, cmd
}

// loadCommitDiff fetches and parses the diff for a commit on demand.
// This replaces the skeleton files with fully parsed FilePairs.
func (m *Model) loadCommitDiff(commitIdx int) {
	if commitIdx < 0 || commitIdx >= len(m.commits) {
		return
	}

	commit := &m.commits[commitIdx]
	if commit.FilesLoaded {
		return
	}

	// Fetch the diff for this commit
	diffStr, err := m.git.Show(commit.Info.SHA)
	if err != nil {
		// On error, mark as loaded to avoid retrying
		commit.FilesLoaded = true
		return
	}

	// Parse the diff
	d, err := diff.Parse(diffStr)
	if err != nil {
		commit.FilesLoaded = true
		return
	}

	// Transform to side-by-side format
	files, truncatedCount := sidebyside.TransformDiff(d)

	// Get the file range for this commit
	startIdx := m.commitFileStarts[commitIdx]
	endIdx := len(m.files)
	if commitIdx+1 < len(m.commits) {
		endIdx = m.commitFileStarts[commitIdx+1]
	}
	oldFileCount := endIdx - startIdx

	// Replace skeleton files with real ones
	// If file counts differ, we need to adjust the files slice and update commitFileStarts
	if len(files) != oldFileCount {
		// Build new files slice
		newFiles := make([]sidebyside.FilePair, 0, len(m.files)-oldFileCount+len(files))
		newFiles = append(newFiles, m.files[:startIdx]...)
		newFiles = append(newFiles, files...)
		newFiles = append(newFiles, m.files[endIdx:]...)
		m.files = newFiles

		// Update commitFileStarts for subsequent commits
		delta := len(files) - oldFileCount
		for i := commitIdx + 1; i < len(m.commitFileStarts); i++ {
			m.commitFileStarts[i] += delta
		}
	} else {
		// Same file count - just replace in place
		for i, f := range files {
			m.files[startIdx+i] = f
		}
	}

	commit.FilesLoaded = true
	commit.TruncatedFileCount = truncatedCount

	// Recalculate cached commit stats from loaded files
	commit.TotalAdded = 0
	commit.TotalRemoved = 0
	for _, f := range files {
		commit.TotalAdded += f.TotalAdded
		commit.TotalRemoved += f.TotalRemoved
	}

	// Invalidate caches
	m.rowsCacheValid = false
}

// commitVisibilityLevel returns the current visibility level for the first commit (1, 2, or 3).
// Deprecated: Use commitVisibilityLevelFor for multi-commit support.
func (m Model) commitVisibilityLevel() int {
	return m.commitVisibilityLevelFor(0)
}

// commitVisibilityLevelFor returns the visibility level for a specific commit (1, 2, or 3).
// Level 1: Commit is folded (only commit header visible)
// Level 2: Commit is normal, all files are FoldFolded (file headings only)
// Level 3: Any file is not FoldFolded (file content visible)
func (m Model) commitVisibilityLevelFor(commitIdx int) int {
	if commitIdx < 0 || commitIdx >= len(m.commits) {
		return 1
	}

	commit := m.commits[commitIdx]

	// Level 1: Commit itself is folded
	if commit.FoldLevel == sidebyside.CommitFolded {
		return 1
	}

	// Get file range for this commit
	startIdx := m.commitFileStarts[commitIdx]
	endIdx := len(m.files)
	if commitIdx+1 < len(m.commits) {
		endIdx = m.commitFileStarts[commitIdx+1]
	}

	// Check if any file in this commit is expanded beyond FoldFolded
	for i := startIdx; i < endIdx; i++ {
		if m.files[i].FoldLevel != sidebyside.FoldFolded {
			return 3
		}
	}

	// All files are FoldFolded
	return 2
}
