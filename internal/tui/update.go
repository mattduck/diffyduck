package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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
		// Any mouse activity implies we're focused (fixes tmux pane click not sending FocusMsg)
		m.focused = true
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.w().scroll -= 3
			m.clampScroll()
			m.resetSearchMatchForRow()
		case tea.MouseButtonWheelDown:
			m.w().scroll += 3
			m.clampScroll()
			m.resetSearchMatchForRow()
			// Check if we should load more commits after scrolling down
			if m.shouldLoadMoreCommits() {
				return m, m.fetchMoreCommits()
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		// Capture cursor row index before resize changes cursorOffset()
		// Row list is stable on resize (only rendering widths change, not row count),
		// so we can restore to the same absolute row index.
		savedRowIdx := m.cursorLine()

		m.width = msg.Width
		m.height = msg.Height

		// Rebuild help content on resize (column layout depends on width)
		if m.helpMode {
			m.helpLines = m.buildHelpLines()
			m.clampHelpScroll()
		}

		// Set initial fold levels on first window size message
		if !m.initialFoldSet && len(m.files) > 0 {
			m.initialFoldSet = true
			// If only 1 file, or all content fits on screen, start fully expanded (hunks)
			if len(m.files) == 1 || m.estimateNormalRows() <= m.contentHeight() {
				for i := range m.files {
					m.setFileFoldLevel(i, sidebyside.FoldHunks)
				}
			} else {
				// Otherwise start folded
				for i := range m.files {
					m.setFileFoldLevel(i, sidebyside.FoldHeader)
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
			// Check if loading this file's content affects visible rows.
			// In full-file view, content arrival changes the row layout directly.
			// Otherwise, content loading only affects layout after highlighting
			// completes via storeHighlightSpans.
			affectsVisibleRows := m.files[msg.FileIndex].ShowFullFile

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

	case CommitStatsLoadedMsg:
		if msg.Stats != nil {
			// Apply stats to commits
			for i := range m.commits {
				sha := m.commits[i].Info.SHA
				if stats, ok := msg.Stats[sha]; ok {
					m.commits[i].TotalAdded = stats.TotalAdded
					m.commits[i].TotalRemoved = stats.TotalRemoved
					m.commits[i].StatsLoaded = true

					// Apply per-file stats if they match
					startIdx := m.commitFileStarts[i]
					endIdx := len(m.files)
					if i+1 < len(m.commits) {
						endIdx = m.commitFileStarts[i+1]
					}
					for j, fs := range stats.FileStats {
						fileIdx := startIdx + j
						if fileIdx < endIdx {
							m.files[fileIdx].TotalAdded = fs.Added
							m.files[fileIdx].TotalRemoved = fs.Removed
						}
					}
				}
			}
			// Invalidate row cache for all windows (widths stay at defaults until 'r' refresh)
			m.invalidateAllRowCaches()
		}

		return m, nil

	case SnapshotCreatedMsg:
		if msg.Err != nil {
			// Snapshot creation failed - disable snapshots
			m.autoSnapshots = false
			return m, nil
		}
		m.snapshots = append(m.snapshots, msg.SHA)

		// Persist the snapshot as a git ref
		if m.autoSnapshots && m.git != nil && m.baseSHA != "" {
			_ = m.git.UpdateSnapshotRef(m.branch, m.baseSHA, msg.SHA)
		}

		// If we're supposed to be in snapshot view but haven't built it yet
		// (no persisted history existed at startup), build it now that the
		// initial snapshot exists.
		if m.showSnapshots && m.snapshotViewCommits == nil {
			m.logf("SnapshotCreatedMsg: showSnapshots=true but no snapshot view yet, building")
			if m.normalViewCommits == nil {
				m.normalViewCommits = make([]sidebyside.CommitSet, len(m.commits))
				copy(m.normalViewCommits, m.commits)
			}
			return m, m.buildSnapshotHistoryCmd()
		}

		// Update the first commit's info if already in snapshot view
		if m.showSnapshots && len(m.commits) > 0 && m.commits[0].IsSnapshot && m.commits[0].Info.SHA == "" {
			if len(msg.SHA) > 7 {
				m.commits[0].Info.SHA = msg.SHA[:7]
			} else {
				m.commits[0].Info.SHA = msg.SHA
			}
			m.commits[0].Info.Subject = msg.Subject
			m.commits[0].Info.Date = msg.Date
			m.invalidateAllRowCaches()
		}
		return m, nil

	case SnapshotDiffReadyMsg:
		if msg.Err != nil {
			now := time.Now()
			m.statusMessage = "Snapshot diff failed"
			m.statusMessageTime = now
			return m, m.clearStatusAfter(now)
		}

		// Store the new snapshot SHA (even if no changes, so next diff starts from this point)
		if msg.SnapshotSHA != "" {
			m.snapshots = append(m.snapshots, msg.SnapshotSHA)

			// Persist the snapshot as a git ref (for automatic continuation)
			if m.autoSnapshots && m.git != nil && m.baseSHA != "" {
				if err := m.git.UpdateSnapshotRef(m.branch, m.baseSHA, msg.SnapshotSHA); err != nil {
					// Log error but don't fail - the snapshot is still in memory
					now := time.Now()
					m.statusMessage = "Warning: failed to persist snapshot"
					m.statusMessageTime = now
				}
			}
		}

		// Check for "no changes" case
		if len(msg.CommitSet.Files) == 0 {
			now := time.Now()
			m.statusMessage = "No changes since last snapshot"
			m.statusMessageTime = now
			return m, m.clearStatusAfter(now)
		}

		// Increment the snapshot count (only for actual diffs, not empty ones)
		m.snapshotCount++

		// Insert the new diff at the beginning of commits
		m.insertSnapshotCommit(msg.CommitSet)

		// Fetch content and request highlighting for the new files (indices 0 to newFileCount-1)
		newFileCount := len(msg.CommitSet.Files)
		var cmds []tea.Cmd

		// Fetch full file content for context expansion
		if msg.CommitSet.SnapshotOldRef != "" && msg.CommitSet.SnapshotNewRef != "" {
			cmds = append(cmds, m.FetchSnapshotFilesContent(
				msg.CommitSet.SnapshotOldRef,
				msg.CommitSet.SnapshotNewRef,
				0, newFileCount,
			))
		}

		// Request syntax highlighting
		for i := 0; i < newFileCount; i++ {
			cmds = append(cmds, m.RequestHighlightFromPairs(i))
		}
		return m, tea.Batch(cmds...)

	case SnapshotCreatedSilentMsg:
		if msg.Err != nil {
			m.autoSnapshots = false
			return m, nil
		}
		m.snapshots = append(m.snapshots, msg.SHA)
		if m.autoSnapshots && m.git != nil && m.baseSHA != "" {
			_ = m.git.UpdateSnapshotRef(m.branch, m.baseSHA, msg.SHA)
		}
		// Invalidate cached snapshot view (new snapshot exists)
		m.snapshotViewCommits = nil
		now := time.Now()
		m.statusMessage = "Snapshot taken"
		m.statusMessageTime = now
		return m, m.clearStatusAfter(now)

	case SnapshotHistoryReadyMsg:
		m.logf("SnapshotHistoryReadyMsg: err=%v commits=%d showSnapshots=%v", msg.Err, len(msg.Commits), m.showSnapshots)
		if msg.Err != nil {
			now := time.Now()
			m.statusMessage = "Failed to load snapshot history"
			m.statusMessageTime = now
			m.showSnapshots = false
			return m, m.clearStatusAfter(now)
		}
		if len(msg.Commits) == 0 {
			now := time.Now()
			m.statusMessage = "No snapshot history"
			m.statusMessageTime = now
			m.showSnapshots = false
			return m, m.clearStatusAfter(now)
		}
		m.snapshotViewCommits = msg.Commits
		cmd := m.swapToView(msg.Commits)
		m.logf("SnapshotHistoryReadyMsg: swapped to %d commits, %d files", len(m.commits), len(m.files))
		return m, cmd

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

	case MoreCommitsLoadedMsg:
		m.loadingMoreCommits = false
		if msg.Err != nil {
			// Remove the ellipsis since we can't load more
			m.invalidateAllRowCaches()
			return m, nil
		}
		if len(msg.Commits) == 0 {
			// No more commits - we've loaded everything
			if m.totalCommitCount <= 0 {
				m.totalCommitCount = m.loadedCommitCount
			}
			// Rebuild rows to remove the pagination indicator
			m.invalidateAllRowCaches()
			m.calculateTotalLines()
			return m, nil
		}

		// Append the new commits. Since rows are only appended at the end,
		// existing row indices are unchanged — no scroll adjustment needed.
		m.appendCommits(msg.Commits)

		// Queue stats loading for the new commits
		return m, m.fetchCommitStats()

	case TotalCommitCountMsg:
		m.totalCommitCount = msg.Count
		// Rebuild rows in case the ellipsis should appear or disappear
		m.invalidateAllRowCaches()
		m.calculateTotalLines()
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear stale status message on any keypress (after minimum display time)
	if m.statusMessage != "" && time.Since(m.statusMessageTime) >= statusMessageMinDuration {
		m.statusMessage = ""
	}

	// Handle comment input mode first (highest priority)
	if m.w().commentMode {
		return m.handleCommentInput(msg)
	}

	// Handle search input mode separately
	if m.searchMode {
		return m.handleSearchInput(msg)
	}

	// Handle multi-key sequences first (before mode checks, since prefix
	// could have been set in any mode that supports sequences)
	if m.pendingKey != "" {
		return m.handlePendingKey(msg)
	}

	// Toggle help screen — available except in text-editing modes
	if matchesKey(msg, m.keys.Help) {
		m.helpMode = !m.helpMode
		m.helpScroll = 0
		if m.helpMode {
			m.helpLines = m.buildHelpLines()
		}
		return m, nil
	}

	// Handle help screen navigation when active
	if m.helpMode {
		return m.handleHelpKey(msg)
	}

	// Handle visual mode - exit keys, yank, otherwise delegate to normal keys
	if m.w().visualSelection.Active {
		if matchesKey(msg, m.keys.VisualExit) {
			m.w().visualSelection.Active = false
			return m, nil
		}
		if matchesKey(msg, m.keys.Yank) {
			return m.handleVisualYank()
		}
		// Fall through to normal key handling for movement, quit, etc.
	}

	// Check for prefix keys that start multi-key sequences
	if m.keys.prefixSet[msg.String()] {
		m.pendingKey = msg.String()
		return m, nil
	}

	keys := m.keys

	switch {
	case matchesKey(msg, keys.Quit):
		// Close window if multiple, quit if last
		if len(m.windows) > 1 {
			return m.windowClose()
		}
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
		// When no active search, N toggles narrow mode instead
		if m.searchQuery == "" {
			m.toggleNarrow()
			return m, nil
		}
		m.prevMatch()
		return m, nil

	case matchesKey(msg, keys.Up):
		m.w().scroll--
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.Down):
		m.w().scroll++
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.PageUp):
		m.w().scroll -= m.height
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.PageDown):
		m.w().scroll += m.height
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.HalfUp):
		m.w().scroll -= m.height / 2
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.HalfDown):
		m.w().scroll += m.height / 2
		m.clampScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.Top):
		m.w().scroll = m.minScroll()
		m.resetSearchMatchForRow()

	case matchesKey(msg, keys.Bottom):
		m.w().scroll = m.maxScroll()
		m.resetSearchMatchForRow()
		// Trigger loading more commits if available
		if m.shouldLoadMoreCommits() {
			return m, m.fetchMoreCommits()
		}

	case matchesKey(msg, keys.Left):
		m.w().hscroll -= m.hscrollStep
		if m.w().hscroll < 0 {
			m.w().hscroll = 0
		}

	case matchesKey(msg, keys.Right):
		m.w().hscroll += m.hscrollStep

	case matchesKey(msg, keys.FoldToggle):
		return m.handleFoldToggle()

	case matchesKey(msg, keys.FoldToggleAll):
		return m.handleFoldToggleAll()

	case matchesKey(msg, keys.FullFileToggle):
		return m.handleFullFileToggle()

	case matchesKey(msg, keys.Enter):
		// Enter starts comment mode on commentable lines
		if m.startComment() {
			return m, nil
		}

	case matchesKey(msg, keys.YankAll):
		return m.handleYankAll()

	case matchesKey(msg, keys.Yank):
		return m.handleYank()

	case matchesKey(msg, keys.RefreshLayout):
		m.RefreshLayout()

	case matchesKey(msg, keys.Snapshot):
		if cmd := m.handleSnapshot(); cmd != nil {
			return m, cmd
		}

	case matchesKey(msg, keys.SnapshotToggle):
		if cmd := m.handleSnapshotToggle(); cmd != nil {
			return m, cmd
		}

	case matchesKey(msg, keys.VisualMode):
		m.w().visualSelection.Active = true
		m.w().visualSelection.AnchorRow = m.w().scroll
		return m, nil
	}

	// Check if we should load more commits after scroll changes
	if m.shouldLoadMoreCommits() {
		return m, m.fetchMoreCommits()
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
	// For commit info body rows, which body row within the info section (0-indexed)
	commitInfoBodyIndex int
	// For structural diff rows, which row within the file's structural diff area (0-indexed)
	structuralDiffIndex int
	// For content rows, the line numbers to match
	oldNum int
	newNum int
}

// getCursorRowIdentity returns the identity of the row at the cursor position.
func (m Model) getCursorRowIdentity() cursorRowIdentity {
	// Use cached rows if valid, otherwise rebuild
	rows := m.w().cachedRows
	if !m.w().rowsCacheValid {
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

	// For commit info body rows, count which row this is within the info section
	commitInfoBodyIndex := 0
	if row.kind == RowKindCommitInfoBody {
		for i := cursorPos - 1; i >= 0; i-- {
			if rows[i].kind == RowKindCommitInfoBody && rows[i].commitIndex == row.commitIndex {
				commitInfoBodyIndex++
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
		commitInfoBodyIndex: commitInfoBodyIndex,
		structuralDiffIndex: structuralDiffIndex,
		oldNum:              row.pair.Old.Num,
		newNum:              row.pair.New.Num,
	}
}

// findRowOrNearestAbove finds the row matching identity, or the nearest header/separator above.
// Returns the line index of the found row.
func (m Model) findRowOrNearestAbove(identity cursorRowIdentity) int {
	// Use cached rows if valid, otherwise rebuild
	rows := m.w().cachedRows
	if !m.w().rowsCacheValid {
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

	// Track commit info body rows seen per commit for matching specific body rows
	commitInfoBodySeen := 0
	lastCommitIndexForInfo := -2 // Start with invalid value

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

		// Reset commit info body counter when commit changes
		if row.commitIndex != lastCommitIndexForInfo {
			commitInfoBodySeen = 0
			lastCommitIndexForInfo = row.commitIndex
		}

		// Reset structural diff counter when file changes
		if row.fileIndex != lastFileIndexForStructDiff {
			structuralDiffSeen = 0
			lastFileIndexForStructDiff = row.fileIndex
		}

		if m.rowMatchesIdentity(row, identity, blanksSeen, commitBodySeen, commitInfoBodySeen, structuralDiffSeen) {
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

		// Count commit info body rows after checking (so first row has index 0)
		if row.kind == RowKindCommitInfoBody && row.commitIndex == identity.commitIndex {
			commitInfoBodySeen++
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
func (m Model) rowMatchesIdentity(row displayRow, identity cursorRowIdentity, blanksSeen, commitBodySeen, commitInfoBodySeen, structuralDiffSeen int) bool {
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
	case RowKindCommitInfoHeader:
		return row.kind == RowKindCommitInfoHeader && row.commitIndex == identity.commitIndex
	case RowKindCommitInfoBody:
		// Match the specific commit info body row by index within the info section
		return row.kind == RowKindCommitInfoBody && row.commitIndex == identity.commitIndex && commitInfoBodySeen == identity.commitInfoBodyIndex
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
	m.w().scroll = rowIndex
	m.clampScroll()
}

// nextFoldLevel returns the next fold level.
// Cycle: Normal -> Expanded -> Folded -> Normal
// (structural diff -> hunks -> header only -> structural diff)
func (m Model) nextFoldLevel(current sidebyside.FoldLevel) sidebyside.FoldLevel {
	return current.NextLevel()
}

// nextFoldLevelForFile returns the next fold level for a specific file.
// Like nextFoldLevel but also skips FoldStructure for binary files
// (binary files have no structural diff).
func (m Model) nextFoldLevelForFile(currentLevel sidebyside.FoldLevel, fp sidebyside.FilePair) sidebyside.FoldLevel {
	next := currentLevel.NextLevel()
	if fp.IsBinary && next == sidebyside.FoldStructure {
		// Skip FoldStructure for binary files
		return next.NextLevel() // Returns FoldHunks
	}
	return next
}

// handleFoldToggle cycles the fold level of the current file or commit,
// but only when on the respective header row.
func (m Model) handleFoldToggle() (tea.Model, tea.Cmd) {
	// If cursor is on commit info header, toggle commit info expanded
	if m.isOnCommitInfoHeader() {
		return m.handleCommitInfoFoldToggle()
	}

	// If cursor is on commit header, do commit fold cycle
	if m.isOnCommitHeader() {
		return m.handleCommitFoldCycle()
	}

	// If not on a header, try context expansion on separators
	if !m.isOnFileHeader() {
		return m.handleContextExpand()
	}

	fileIdx := m.currentFileIndex()
	if fileIdx < 0 || fileIdx >= len(m.files) {
		return m, nil
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	newLevel := m.nextFoldLevelForFile(m.fileFoldLevel(fileIdx), m.files[fileIdx])
	m.setFileFoldLevel(fileIdx, newLevel)
	// Clear full-file view when cycling away from FoldHunks
	if newLevel != sidebyside.FoldHunks {
		m.files[fileIdx].ShowFullFile = false
	}
	// Reset pairs to original state (undo any user context expansion)
	m.files[fileIdx].ResetPairs()

	// If file is expanded beyond FoldHeader, ensure parent commit is at least CommitFileHeaders
	// (so the file is visible), but don't force CommitFileHunks (keep commit info fold state independent)
	if newLevel != sidebyside.FoldHeader && len(m.commits) > 0 {
		commitIdx := m.commitForFile(fileIdx)
		if commitIdx >= 0 && commitIdx < len(m.commits) {
			if m.commitFoldLevel(commitIdx) == sidebyside.CommitFolded {
				m.setCommitFoldLevel(commitIdx, sidebyside.CommitFileHeaders)
			}
		}
	}

	// If transitioning to FoldStructure, ensure structural data is available for the
	// code-stats view. If not, either trigger highlighting (content already loaded)
	// or fetch the content first (which triggers highlighting on arrival).
	var cmd tea.Cmd
	if newLevel == sidebyside.FoldStructure && m.structureMaps[fileIdx] == nil {
		fp := m.files[fileIdx]
		if len(fp.OldContent) > 0 || len(fp.NewContent) > 0 {
			// Content available but not yet highlighted with structure
			cmd = m.RequestHighlight(fileIdx)
		} else if !m.isFileLoading(fileIdx) {
			// No content — fetch it (highlighting happens on arrival)
			if fetchCmd := m.FetchFileContent(fileIdx); fetchCmd != nil {
				m.markFileLoading(fileIdx)
				cmd = fetchCmd
			}
		}
	}

	m.calculateTotalLines()

	// Preserve scroll position
	newRowIdx := m.findRowOrNearestAbove(identity)
	m.adjustScrollToRow(newRowIdx)

	return m, cmd
}

// handleFullFileToggle toggles the full-file content view for the current file.
// When enabled, FoldHunks shows full file content (with diff alignment) instead of hunks.
// This is independent of the fold cycle — Tab still cycles normally.
func (m Model) handleFullFileToggle() (tea.Model, tea.Cmd) {
	fileIdx := m.currentFileIndex()
	if fileIdx < 0 || fileIdx >= len(m.files) {
		return m, nil
	}

	// Capture cursor identity before layout change
	identity := m.getCursorRowIdentity()

	// When cursor is on a hunk separator, determine a target line number to
	// navigate to after the layout changes (separators disappear in full-file view).
	targetNewLineNum := m.fullFileToggleSeparatorTarget(fileIdx)

	// If file is not at FoldHunks, expand it first
	if m.fileFoldLevel(fileIdx) != sidebyside.FoldHunks {
		m.setFileFoldLevel(fileIdx, sidebyside.FoldHunks)
		// Ensure parent commit is visible
		if len(m.commits) > 0 {
			commitIdx := m.commitForFile(fileIdx)
			if commitIdx >= 0 && commitIdx < len(m.commits) {
				if m.commitFoldLevel(commitIdx) == sidebyside.CommitFolded {
					m.setCommitFoldLevel(commitIdx, sidebyside.CommitFileHeaders)
				}
			}
		}
	}

	// Toggle full-file view
	m.files[fileIdx].ShowFullFile = !m.files[fileIdx].ShowFullFile

	// When enabling full-file view, also narrow to this file if not already narrowed
	if m.files[fileIdx].ShowFullFile && !m.w().narrow.Active {
		m.w().narrow = NarrowScope{
			Active:    true,
			CommitIdx: m.commitForFile(fileIdx),
			FileIdx:   fileIdx,
			HunkIdx:   -1,
		}
	}

	m.calculateTotalLines()

	// Position cursor after layout change
	if targetNewLineNum > 0 && m.files[fileIdx].ShowFullFile {
		// Cursor was on a separator — find the target line in the new layout
		newRowIdx := m.findRowByNewLineNum(fileIdx, targetNewLineNum)
		m.adjustScrollToRow(newRowIdx)
	} else {
		// Normal identity-based scroll preservation
		newRowIdx := m.findRowOrNearestAbove(identity)
		m.adjustScrollToRow(newRowIdx)
	}

	// If enabling full-file and content not yet loaded, fetch it
	if m.files[fileIdx].ShowFullFile && !m.files[fileIdx].HasContent() {
		return m, m.FetchFileContent(fileIdx)
	}

	return m, nil
}

// fullFileToggleSeparatorTarget determines the target new-side line number when
// toggling full-file view while the cursor is on a hunk separator.
// Returns 0 if cursor is not on a separator.
//
// Rules:
//   - SeparatorTop: go to last content line above the separator.
//   - SeparatorBottom: go to first content line below the separator.
//   - Separator (middle): if a breadcrumb exists, go to the innermost entry's
//     start line; otherwise go to the first content line below.
func (m Model) fullFileToggleSeparatorTarget(fileIdx int) int {
	rows := m.w().cachedRows
	if !m.w().rowsCacheValid {
		rows = m.buildRows()
	}
	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return 0
	}
	row := rows[cursorPos]

	switch row.kind {
	case RowKindSeparatorTop:
		// Go to last content line above the separator
		if lineNum := m.separatorContentAbove(rows, cursorPos, fileIdx); lineNum > 0 {
			return lineNum
		}
		return 0

	case RowKindSeparatorBottom:
		return m.separatorContentBelow(rows, cursorPos, fileIdx)

	case RowKindSeparator:
		return m.separatorMiddleTarget(rows, cursorPos, fileIdx)
	}

	return 0
}

// separatorContentAbove returns the new-side line number of the last content row
// above cursorPos in the same file, or 0 if none found.
func (m Model) separatorContentAbove(rows []displayRow, cursorPos int, fileIdx int) int {
	for i := cursorPos - 1; i >= 0; i-- {
		if rows[i].kind == RowKindContent && rows[i].fileIndex == fileIdx {
			if rows[i].pair.New.Num > 0 {
				return rows[i].pair.New.Num
			}
			if rows[i].pair.Old.Num > 0 {
				return rows[i].pair.Old.Num
			}
		}
	}
	return 0
}

// separatorContentBelow returns the new-side line number of the first content row
// below cursorPos in the same file, or 0 if none found.
func (m Model) separatorContentBelow(rows []displayRow, cursorPos int, fileIdx int) int {
	for i := cursorPos + 1; i < len(rows); i++ {
		if rows[i].kind == RowKindContent && rows[i].fileIndex == fileIdx {
			if rows[i].pair.New.Num > 0 {
				return rows[i].pair.New.Num
			}
			if rows[i].pair.Old.Num > 0 {
				return rows[i].pair.Old.Num
			}
		}
		if rows[i].fileIndex != fileIdx {
			break
		}
	}
	return 0
}

// separatorMiddleTarget returns the target line for a middle separator row:
// use the breadcrumb's innermost entry start line if available, otherwise
// fall back to the first content line below.
func (m Model) separatorMiddleTarget(rows []displayRow, cursorPos int, fileIdx int) int {
	// Find the Separator row (middle) in this separator block for its chunkStartLine
	sepRow := rows[cursorPos]
	if sepRow.kind != RowKindSeparator {
		// We're on Top or Bottom — scan for the middle row in this block
		for i := cursorPos - 1; i <= cursorPos+2 && i < len(rows); i++ {
			if i >= 0 && rows[i].kind == RowKindSeparator && rows[i].fileIndex == fileIdx {
				sepRow = rows[i]
				break
			}
		}
	}

	if sepRow.chunkStartLine > 0 {
		entries := m.getStructureAtLine(fileIdx, sepRow.chunkStartLine)
		if len(entries) > 0 {
			return entries[len(entries)-1].StartLine
		}
	}
	// No breadcrumb — go to first content line below
	return m.separatorContentBelow(rows, cursorPos, fileIdx)
}

// findRowByNewLineNum finds the row in the current layout that matches the given
// new-side line number for a specific file. Falls back to the nearest row above.
func (m Model) findRowByNewLineNum(fileIdx int, targetLineNum int) int {
	rows := m.w().cachedRows
	if !m.w().rowsCacheValid {
		rows = m.buildRows()
	}

	bestIdx := 0
	for i, row := range rows {
		if row.fileIndex != fileIdx {
			continue
		}
		if row.kind != RowKindContent {
			continue
		}
		if row.pair.New.Num == targetLineNum {
			return i
		}
		// Track the closest content row at or before the target
		if row.pair.New.Num > 0 && row.pair.New.Num < targetLineNum {
			bestIdx = i
		}
	}
	return bestIdx
}

// handleFoldToggleAll cycles the fold level for all commits.
// When in narrow mode, only affects the narrowed scope.
// Commit visibility levels:
//   - CommitFolded (just commit header)
//   - CommitFileHeaders (commit + file headers)
//   - CommitFileStructure (commit + file headers + structural diff)
//   - CommitFileHunks (commit + file headers + hunks + info expanded)
//
// If all commits are at the same level, advance to next level.
// If commits are at different levels (mixed), collapse all to level 1.
func (m Model) handleFoldToggleAll() (tea.Model, tea.Cmd) {
	// Fall back to legacy file-based behavior if no commits
	if len(m.commits) == 0 {
		return m.handleFoldToggleAllFiles()
	}

	// When narrowed to a file, delegate to file-based toggle (single file)
	if m.w().narrow.Active && m.w().narrow.FileIdx >= 0 {
		return m.handleFoldToggleAllFiles()
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	// Determine which commits to operate on
	startCommit, endCommit := 0, len(m.commits)
	if m.w().narrow.Active && m.w().narrow.CommitIdx >= 0 {
		// Narrowed to a commit: only toggle that commit
		startCommit = m.w().narrow.CommitIdx
		endCommit = m.w().narrow.CommitIdx + 1
	}

	// Check the fold level of commits in scope
	firstLevel := m.commitFoldLevel(startCommit)
	allSame := true
	for i := startCommit + 1; i < endCommit; i++ {
		if m.commitFoldLevel(i) != firstLevel {
			allSame = false
			break
		}
	}

	var newLevel sidebyside.CommitFoldLevel
	if allSame {
		// All same — advance to next level in the cycle
		newLevel = firstLevel.NextLevel()
	} else {
		// Mixed levels — reset all to folded
		newLevel = sidebyside.CommitFolded
	}

	// Apply the new level to commits in scope
	m.setCommitsToLevel(startCommit, endCommit, newLevel)

	m.calculateTotalLines()

	// Preserve scroll position
	newRowIdx := m.findRowOrNearestAbove(identity)
	m.adjustScrollToRow(newRowIdx)

	// If expanding to structural diff or beyond, queue files for content loading
	var cmd tea.Cmd
	if newLevel >= sidebyside.CommitFileStructure {
		if m.w().narrow.Active && m.w().narrow.CommitIdx >= 0 {
			cmd = m.queueFilesForCommit(m.w().narrow.CommitIdx)
		} else {
			cmd = m.queueFilesForAllCommits()
		}
	}

	return m, cmd
}

// setAllCommitsToLevel sets all commits and their files to the specified commit fold level.
func (m *Model) setAllCommitsToLevel(level sidebyside.CommitFoldLevel) {
	m.setCommitsToLevel(0, len(m.commits), level)
}

// setCommitsToLevel sets commits in range [start, end) and their files to the specified level.
// Uses CommitFileFold and CommitInfoExpandedAt tables for the mapping.
func (m *Model) setCommitsToLevel(start, end int, level sidebyside.CommitFoldLevel) {
	fileFold := sidebyside.CommitFileFold[level]
	infoExpanded := sidebyside.CommitInfoExpandedAt[level]

	for i := start; i < end && i < len(m.commits); i++ {
		m.setCommitFoldLevel(i, level)
		m.setCommitInfoExpanded(i, infoExpanded)

		// Set fold level for files belonging to this commit
		fileStart := m.commitFileStarts[i]
		fileEnd := len(m.files)
		if i+1 < len(m.commitFileStarts) {
			fileEnd = m.commitFileStarts[i+1]
		}
		for j := fileStart; j < fileEnd; j++ {
			m.setFileFoldLevel(j, fileFold)
		}
	}
}

// handleFoldToggleAllFiles is the legacy behavior for toggling all files
// when there are no commits (e.g., pager mode or tests that bypass commits).
// When in narrow mode with a file scope, only toggles that file.
func (m Model) handleFoldToggleAllFiles() (tea.Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m, nil
	}

	// Capture cursor identity before fold change
	identity := m.getCursorRowIdentity()

	// Determine file range to operate on
	startFile, endFile := 0, len(m.files)
	if m.w().narrow.Active && m.w().narrow.FileIdx >= 0 {
		// Narrowed to a single file: only toggle that file
		startFile = m.w().narrow.FileIdx
		endFile = m.w().narrow.FileIdx + 1
	}

	// Check if all files in scope are at the same level
	firstLevel := m.fileFoldLevel(startFile)
	allSame := true
	for i := startFile + 1; i < endFile; i++ {
		if m.fileFoldLevel(i) != firstLevel {
			allSame = false
			break
		}
	}

	var newLevel sidebyside.FoldLevel
	if allSame {
		// All same - advance to next level (respecting pager mode)
		newLevel = m.nextFoldLevel(firstLevel)
	} else {
		// Different levels - collapse all to Header
		newLevel = sidebyside.FoldHeader
	}

	for i := startFile; i < endFile; i++ {
		m.setFileFoldLevel(i, newLevel)
	}

	m.calculateTotalLines()

	// Preserve scroll position
	newRowIdx := m.findRowOrNearestAbove(identity)
	m.adjustScrollToRow(newRowIdx)

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

// handlePendingKey handles the second key of a multi-key sequence.
// This replaces the per-prefix handlers (handlePendingG, handlePendingCtrlW)
// with a single generic handler that uses matchesSequence.
func (m Model) handlePendingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prefix := m.pendingKey
	m.pendingKey = "" // Always clear pending state

	keys := m.keys

	// In help mode, only GoToTop (gg) is meaningful
	if m.helpMode {
		if matchesSequence(prefix, msg, keys.GoToTop) {
			m.helpScroll = 0
		}
		return m, nil
	}

	// Help toggle (supports sequence bindings like "g h")
	if matchesSequence(prefix, msg, keys.Help) {
		m.helpMode = !m.helpMode
		m.helpScroll = 0
		if m.helpMode {
			m.helpLines = m.buildHelpLines()
		}
		return m, nil
	}

	// Navigation sequences
	if matchesSequence(prefix, msg, keys.GoToTop) {
		m.w().scroll = m.minScroll()
		m.resetSearchMatchForRow()
		return m, nil
	}
	if matchesSequence(prefix, msg, keys.NextHeading) {
		m.goToNextHeading()
		m.resetSearchMatchForRow()
		return m, nil
	}
	if matchesSequence(prefix, msg, keys.PrevHeading) {
		m.goToPrevHeading()
		m.resetSearchMatchForRow()
		return m, nil
	}

	// Window management sequences
	if matchesSequence(prefix, msg, keys.WinSplitV) {
		return m.windowSplitVertical()
	}
	if matchesSequence(prefix, msg, keys.WinSplitH) {
		return m.windowSplitHorizontal()
	}
	if matchesSequence(prefix, msg, keys.WinClose) {
		return m.windowClose()
	}
	if matchesSequence(prefix, msg, keys.WinFocusLeft) {
		return m.windowFocusLeft()
	}
	if matchesSequence(prefix, msg, keys.WinFocusRight) {
		return m.windowFocusRight()
	}
	if matchesSequence(prefix, msg, keys.WinFocusUp) {
		return m.windowFocusUp()
	}
	if matchesSequence(prefix, msg, keys.WinFocusDown) {
		return m.windowFocusDown()
	}
	if matchesSequence(prefix, msg, keys.WinResizeLeft) {
		return m.windowResizeLeft()
	}
	if matchesSequence(prefix, msg, keys.WinResizeRight) {
		return m.windowResizeRight()
	}
	if matchesSequence(prefix, msg, keys.WinResizeUp) {
		return m.windowResizeUp()
	}
	if matchesSequence(prefix, msg, keys.WinResizeDown) {
		return m.windowResizeDown()
	}

	// Visual exit (supports sequence bindings)
	if matchesSequence(prefix, msg, keys.VisualExit) {
		if m.w().visualSelection.Active {
			m.w().visualSelection.Active = false
		}
		return m, nil
	}

	// Any other key just cancels the pending state without action
	return m, nil
}

// createWindowCopy creates a new window copying state from the current window.
func (m Model) createWindowCopy() *Window {
	currentWindow := m.w()
	newWindow := &Window{
		scroll:           currentWindow.scroll,
		hscroll:          currentWindow.hscroll,
		narrow:           currentWindow.narrow,
		fileFoldLevels:   make(map[int]sidebyside.FoldLevel),
		commitFoldLevels: make(map[int]sidebyside.CommitFoldLevel),
		cachedRows:       nil,
		rowsCacheValid:   false,
		totalLines:       currentWindow.totalLines,
		searchMatchIdx:   currentWindow.searchMatchIdx,
		searchMatchSide:  currentWindow.searchMatchSide,
	}

	// Copy fold state from current window
	for k, v := range currentWindow.fileFoldLevels {
		newWindow.fileFoldLevels[k] = v
	}
	for k, v := range currentWindow.commitFoldLevels {
		newWindow.commitFoldLevels[k] = v
	}

	return newWindow
}

// windowSplitVertical creates a new window as a vertical split (side-by-side).
// Maximum 2 windows.
func (m Model) windowSplitVertical() (tea.Model, tea.Cmd) {
	if len(m.windows) >= 2 {
		m.statusMessage = "Maximum 2 windows"
		m.statusMessageTime = time.Now()
		return m, nil
	}

	m.windows = append(m.windows, m.createWindowCopy())
	m.activeWindowIdx = len(m.windows) - 1 // Focus the new window
	m.windowSplitV = true                  // Mark as vertical split

	return m, nil
}

// windowSplitHorizontal creates a new window as a horizontal split (stacked top/bottom).
// Maximum 2 windows.
func (m Model) windowSplitHorizontal() (tea.Model, tea.Cmd) {
	if len(m.windows) >= 2 {
		m.statusMessage = "Maximum 2 windows"
		m.statusMessageTime = time.Now()
		return m, nil
	}

	m.windows = append(m.windows, m.createWindowCopy())
	m.activeWindowIdx = len(m.windows) - 1 // Focus the new window
	m.windowSplitV = false                 // Mark as horizontal split

	return m, nil
}

// windowClose closes the current window.
// Cannot close the last remaining window.
func (m Model) windowClose() (tea.Model, tea.Cmd) {
	if len(m.windows) <= 1 {
		m.statusMessage = "Cannot close last window"
		m.statusMessageTime = time.Now()
		return m, nil
	}

	// Remove current window
	m.windows = append(m.windows[:m.activeWindowIdx], m.windows[m.activeWindowIdx+1:]...)

	// Adjust active index if needed
	if m.activeWindowIdx >= len(m.windows) {
		m.activeWindowIdx = len(m.windows) - 1
	}

	return m, nil
}

// windowFocusLeft moves focus to the left window (vertical split only).
func (m Model) windowFocusLeft() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || !m.windowSplitV {
		return m, nil // Only works for vertical split
	}
	if m.activeWindowIdx > 0 {
		m.activeWindowIdx--
	}
	return m, nil
}

// windowFocusRight moves focus to the right window (vertical split only).
func (m Model) windowFocusRight() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || !m.windowSplitV {
		return m, nil // Only works for vertical split
	}
	if m.activeWindowIdx < len(m.windows)-1 {
		m.activeWindowIdx++
	}
	return m, nil
}

// windowFocusUp moves focus to the upper window (horizontal split only).
func (m Model) windowFocusUp() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || m.windowSplitV {
		return m, nil // Only works for horizontal split
	}
	if m.activeWindowIdx > 0 {
		m.activeWindowIdx--
	}
	return m, nil
}

// windowFocusDown moves focus to the lower window (horizontal split only).
func (m Model) windowFocusDown() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || m.windowSplitV {
		return m, nil // Only works for horizontal split
	}
	if m.activeWindowIdx < len(m.windows)-1 {
		m.activeWindowIdx++
	}
	return m, nil
}

// windowResizeStep is the number of characters/lines to move the divider per resize.
const windowResizeStep = 8

// windowResizeLeft shrinks the left window (vertical split only).
func (m Model) windowResizeLeft() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || !m.windowSplitV {
		return m, nil // Only works for vertical split
	}

	if m.width > 0 {
		stepRatio := float64(windowResizeStep) / float64(m.width)
		m.windowSplitRatio -= stepRatio
		if m.windowSplitRatio < 0.2 {
			m.windowSplitRatio = 0.2
		}
	}
	return m, nil
}

// windowResizeRight grows the left window (vertical split only).
func (m Model) windowResizeRight() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || !m.windowSplitV {
		return m, nil // Only works for vertical split
	}

	if m.width > 0 {
		stepRatio := float64(windowResizeStep) / float64(m.width)
		m.windowSplitRatio += stepRatio
		if m.windowSplitRatio > 0.8 {
			m.windowSplitRatio = 0.8
		}
	}
	return m, nil
}

// windowResizeUp shrinks the top window (horizontal split only).
func (m Model) windowResizeUp() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || m.windowSplitV {
		return m, nil // Only works for horizontal split
	}

	if m.height > 0 {
		stepRatio := float64(windowResizeStep) / float64(m.height)
		m.windowSplitRatioH -= stepRatio
		if m.windowSplitRatioH < 0.2 {
			m.windowSplitRatioH = 0.2
		}
	}
	return m, nil
}

// windowResizeDown grows the top window (horizontal split only).
func (m Model) windowResizeDown() (tea.Model, tea.Cmd) {
	if len(m.windows) < 2 || m.windowSplitV {
		return m, nil // Only works for horizontal split
	}

	if m.height > 0 {
		stepRatio := float64(windowResizeStep) / float64(m.height)
		m.windowSplitRatioH += stepRatio
		if m.windowSplitRatioH > 0.8 {
			m.windowSplitRatioH = 0.8
		}
	}
	return m, nil
}

// isNavigationTarget returns true if the row at the given index is a valid
// stop for gj/gk navigation. Targets are: commit headers, commit info headers,
// file headers, hunk separator middle lines (breadcrumb), and trailing
// separator-top lines (the single-line separator at the end of a file).
func isNavigationTarget(rows []displayRow, i int) bool {
	row := rows[i]
	if row.isCommitHeader || row.isCommitInfoHeader || row.isHeader {
		return true
	}
	if row.isSeparator {
		return true
	}
	// A SeparatorTop is a target only when it's a trailing separator (not
	// followed by a Separator middle line, i.e. it's the lone line at EOF).
	if row.isSeparatorTop {
		if i+1 >= len(rows) || !rows[i+1].isSeparator {
			return true
		}
	}
	return false
}

// goToNextHeading moves the cursor to the next navigation target.
// Targets include commit headers, commit info headers, file headers,
// and hunk separators (breadcrumb lines and trailing EOF separators).
func (m *Model) goToNextHeading() {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return
	}

	for i := cursorPos + 1; i < len(rows); i++ {
		if isNavigationTarget(rows, i) {
			m.adjustScrollToRow(i)
			return
		}
	}
}

// goToPrevHeading moves the cursor to the previous navigation target.
// Targets include commit headers, commit info headers, file headers,
// and hunk separators (breadcrumb lines and trailing EOF separators).
func (m *Model) goToPrevHeading() {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return
	}

	for i := cursorPos - 1; i >= 0; i-- {
		if isNavigationTarget(rows, i) {
			m.adjustScrollToRow(i)
			return
		}
	}
}

// handleContextExpand handles Tab on hunk separators by expanding context.
// SeparatorTop: expand 15 lines downward from the hunk above.
// SeparatorBottom: expand 15 lines upward from the hunk below.
// Separator (middle): expand upward to the breadcrumb's signature start - 2 lines.
func (m Model) handleContextExpand() (tea.Model, tea.Cmd) {
	rows := m.getRows()
	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return m, nil
	}

	row := rows[cursorPos]
	if row.kind != RowKindSeparatorTop && row.kind != RowKindSeparator && row.kind != RowKindSeparatorBottom {
		return m, nil
	}

	fileIdx := row.fileIndex
	if fileIdx < 0 || fileIdx >= len(m.files) {
		return m, nil
	}
	fp := &m.files[fileIdx]

	// Require full file content to be loaded
	if !fp.HasContent() {
		return m, nil
	}

	boundaries := findHunkBoundaries(fp.Pairs)
	if len(boundaries) == 0 {
		return m, nil
	}

	// Find which hunk boundary this separator represents.
	// The separator's chunkStartLine is the first new-side line of the hunk below.
	// Find the Separator (middle) row in this separator block for chunkStartLine.
	sepRow := row
	if sepRow.kind != RowKindSeparator {
		for i := cursorPos - 2; i <= cursorPos+2 && i < len(rows); i++ {
			if i >= 0 && rows[i].kind == RowKindSeparator && rows[i].fileIndex == fileIdx {
				sepRow = rows[i]
				break
			}
		}
	}

	// Match chunkStartLine to a hunk boundary
	hunkBelow := -1
	for i, b := range boundaries {
		hunkPairs := fp.Pairs[b.startIdx:b.endIdx]
		firstNew := getFirstNewLineNum(hunkPairs)
		if firstNew == sepRow.chunkStartLine {
			hunkBelow = i
			break
		}
	}

	// For the first separator (before first hunk), chunkStartLine matches boundary 0
	// and there is no hunk above.
	hunkAbove := hunkBelow - 1

	var targetNewLine int

	switch row.kind {
	case RowKindSeparatorTop:
		// Determine which hunk to expand down from.
		// For inter-hunk separators, expand from the hunk above.
		// For the trailing separator (after last hunk), hunkBelow is -1,
		// so expand from the last boundary.
		expandIdx := hunkAbove
		if hunkBelow < 0 && len(boundaries) > 0 {
			expandIdx = len(boundaries) - 1
		}
		if expandIdx >= 0 {
			hunkPairs := fp.Pairs[boundaries[expandIdx].startIdx:boundaries[expandIdx].endIdx]
			lastNew := getLastNewLineNum(hunkPairs)
			inserted := expandContextDown(fp, boundaries, expandIdx)
			if inserted > 0 {
				// Land on the first newly-inserted line (just below where we clicked)
				targetNewLine = lastNew + 1
			}
		}

	case RowKindSeparatorBottom:
		if hunkBelow >= 0 {
			hunkPairs := fp.Pairs[boundaries[hunkBelow].startIdx:boundaries[hunkBelow].endIdx]
			firstNew := getFirstNewLineNum(hunkPairs)
			inserted := expandContextUp(fp, boundaries, hunkBelow)
			if inserted > 0 {
				// Land on the last newly-inserted line (just above the hunk below)
				targetNewLine = firstNew - 1
			}
		}

	case RowKindSeparator:
		targetNewLine = m.enterExpandMiddle(fp, boundaries, hunkBelow, sepRow)
	}

	m.calculateTotalLines()

	// Position cursor on the target line (or nearest content row)
	if targetNewLine > 0 {
		newRowIdx := m.findRowByNewLineNum(fileIdx, targetNewLine)
		m.adjustScrollToRow(newRowIdx)
	}

	return m, nil
}

// enterExpandMiddle handles the middle-separator Enter expansion.
// If a breadcrumb signature exists, expands to include it (+ 2 lines above).
// Returns the target new-side line number for cursor positioning, or 0 for no-op.
func (m Model) enterExpandMiddle(fp *sidebyside.FilePair, boundaries []hunkBoundary, hunkBelow int, sepRow displayRow) int {
	if hunkBelow < 0 || sepRow.chunkStartLine <= 0 {
		return 0
	}

	entries := m.getStructureAtLine(sepRow.fileIndex, sepRow.chunkStartLine)
	if len(entries) == 0 {
		return 0
	}

	innermost := findInnermostEntry(entries)
	if innermost == nil {
		return 0
	}

	// Only expand if the signature is above the current hunk start
	hunkPairs := fp.Pairs[boundaries[hunkBelow].startIdx:boundaries[hunkBelow].endIdx]
	firstNew := getFirstNewLineNum(hunkPairs)
	if innermost.StartLine >= firstNew {
		return 0 // signature is already visible
	}

	expandContextToSignature(fp, boundaries, hunkBelow, innermost.StartLine)
	// Land on the last inserted line (just above the hunk below), like SeparatorBottom
	return firstNew - 1
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

// isOnCommitInfoHeader returns true if the cursor is on a commit info header row.
func (m Model) isOnCommitInfoHeader() bool {
	rows := m.getRows()
	cursorPos := m.cursorLine()

	if cursorPos < 0 || cursorPos >= len(rows) {
		return false
	}

	return rows[cursorPos].isCommitInfoHeader
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

// handleCommitFoldCycle cycles through commit visibility levels (org-mode style).
// Uses CommitCycle for the transition order:
// CommitFolded -> CommitFileHeaders -> CommitFileStructure -> CommitFileHunks -> CommitFolded
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
	currentLevel := m.commitFoldLevel(commitIdx)
	newLevel := currentLevel.NextLevel()

	// Load diff content on demand when expanding from folded
	if currentLevel == sidebyside.CommitFolded && !commit.FilesLoaded && m.git != nil {
		m.loadCommitDiff(commitIdx)
	}

	// Apply new level via the shared helper
	m.setCommitsToLevel(commitIdx, commitIdx+1, newLevel)

	m.calculateTotalLines()

	// Queue files for content loading if expanding to structural diff or beyond
	var cmd tea.Cmd
	if newLevel >= sidebyside.CommitFileStructure {
		cmd = m.queueFilesForCommit(commitIdx)
	}

	return m, cmd
}

// handleCommitInfoFoldToggle toggles the commit info between header-only and expanded.
// This toggles the commitInfoExpanded flag without changing the commit fold level.
func (m Model) handleCommitInfoFoldToggle() (tea.Model, tea.Cmd) {
	rows := m.getRows()
	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return m, nil
	}

	commitIdx := rows[cursorPos].commitIndex
	if commitIdx < 0 || commitIdx >= len(m.commits) {
		return m, nil
	}

	// Only toggle if commit is not folded (info header must be visible)
	if m.commitFoldLevel(commitIdx) == sidebyside.CommitFolded {
		return m, nil
	}

	// Toggle info expanded state independently
	m.setCommitInfoExpanded(commitIdx, !m.isCommitInfoExpanded(commitIdx))

	m.calculateTotalLines()

	return m, nil
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

		// Shift file-indexed maps (highlights, structure, comments, etc.)
		// so data for subsequent commits stays aligned with their new indices.
		m.shiftFileIndexMapsFrom(endIdx, delta)
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

	// Match persisted comments for the newly loaded files
	if m.commentIndex != nil {
		// Recompute endIdx since file count may have changed
		newEndIdx := len(m.files)
		if commitIdx+1 < len(m.commits) {
			newEndIdx = m.commitFileStarts[commitIdx+1]
		}
		m.matchCommentsForFiles(startIdx, newEndIdx)
	}

	// Invalidate caches
	m.w().rowsCacheValid = false
}

// handleSnapshot handles the S key to create a snapshot.
// Always switches to snapshot view if not already there, then shows the
// incremental diff (lastSnapshot → newSnapshot).
func (m *Model) handleSnapshot() tea.Cmd {
	if !m.autoSnapshots {
		now := time.Now()
		m.statusMessage = "Snapshots not available (no working tree changes)"
		m.statusMessageTime = now
		return m.clearStatusAfter(now)
	}

	if m.git == nil {
		return nil
	}

	// No snapshots yet — just switch to snapshot view and wait for the
	// initial SnapshotCreatedMsg (which triggers buildSnapshotHistoryCmd).
	if len(m.snapshots) == 0 {
		if !m.showSnapshots {
			m.normalViewCommits = make([]sidebyside.CommitSet, len(m.commits))
			copy(m.normalViewCommits, m.commits)
			m.showSnapshots = true
		}
		return nil
	}

	// Switch to snapshot view if not already there: build the timeline
	// separately so the view swaps even if the new snapshot has no changes.
	var viewCmd tea.Cmd
	if !m.showSnapshots {
		m.normalViewCommits = make([]sidebyside.CommitSet, len(m.commits))
		copy(m.normalViewCommits, m.commits)
		m.showSnapshots = true
		m.snapshotViewCommits = nil // invalidate — new snapshot changes the timeline
		viewCmd = m.buildSnapshotHistoryCmd()
	}

	// Capture values for closure
	gitClient := m.git
	allMode := m.allMode
	baseSHA := m.baseSHA
	prevSnapshot := m.snapshots[len(m.snapshots)-1]

	// Format commit message: "dfd: <sha> @ <datetime>"
	baseShort := baseSHA
	if len(baseShort) > 7 {
		baseShort = baseShort[:7]
	}
	dateStr := time.Now().Format("Jan 2 15:04")
	message := fmt.Sprintf("dfd: %s @ %s", baseShort, dateStr)

	snapshotCmd := func() tea.Msg {
		newSnapshot, err := gitClient.CreateSnapshot(allMode, prevSnapshot, message)
		if err != nil {
			return SnapshotDiffReadyMsg{Err: err}
		}

		// Compute the incremental diff
		diffOutput, err := gitClient.DiffSnapshots(prevSnapshot, newSnapshot)
		if err != nil {
			return SnapshotDiffReadyMsg{Err: err}
		}

		if diffOutput == "" {
			return SnapshotDiffReadyMsg{
				Err:         nil,
				SnapshotSHA: newSnapshot,
				CommitSet: sidebyside.CommitSet{
					Info: sidebyside.CommitInfo{
						Subject: "No changes since last snapshot",
					},
				},
			}
		}

		d, err := diff.Parse(diffOutput)
		if err != nil {
			return SnapshotDiffReadyMsg{Err: err}
		}

		files, _ := sidebyside.TransformDiff(d)

		// Create the commit set using the commit message (single source of truth)
		commitSet := sidebyside.CommitSet{
			Info: sidebyside.CommitInfo{
				Subject: message,
				Date:    dateStr,
			},
			Files:          files,
			FoldLevel:      sidebyside.CommitFileHeaders,
			FilesLoaded:    true,
			StatsLoaded:    true,
			IsSnapshot:     true,
			SnapshotOldRef: prevSnapshot,
			SnapshotNewRef: newSnapshot,
		}

		// Calculate stats
		for _, f := range files {
			added, removed := countFileStats(f)
			commitSet.TotalAdded += added
			commitSet.TotalRemoved += removed
		}

		return SnapshotDiffReadyMsg{
			CommitSet:   commitSet,
			SnapshotSHA: newSnapshot,
			Err:         nil,
		}
	}

	if viewCmd != nil {
		return tea.Batch(viewCmd, snapshotCmd)
	}
	return snapshotCmd
}
