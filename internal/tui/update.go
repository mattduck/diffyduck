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
			m.files[msg.FileIndex].OldContent = msg.OldContent
			m.files[msg.FileIndex].NewContent = msg.NewContent
			m.calculateTotalLines()
			m.refreshSearch()
		}
		return m, nil

	case AllContentLoadedMsg:
		for _, fc := range msg.Contents {
			if fc.FileIndex >= 0 && fc.FileIndex < len(m.files) {
				m.files[fc.FileIndex].OldContent = fc.OldContent
				m.files[fc.FileIndex].NewContent = fc.NewContent
			}
		}
		m.calculateTotalLines()
		m.refreshSearch()
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
		m.scroll = 0

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

// handleFoldToggle cycles the fold level of the current file.
func (m Model) handleFoldToggle() (tea.Model, tea.Cmd) {
	fileIdx := m.currentFileIndex()
	if fileIdx < 0 || fileIdx >= len(m.files) {
		return m, nil
	}

	newLevel := m.files[fileIdx].FoldLevel.NextLevel()
	m.files[fileIdx].FoldLevel = newLevel
	m.calculateTotalLines()
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

// currentFileIndex returns the index of the file at the current scroll position.
func (m Model) currentFileIndex() int {
	if len(m.files) == 0 {
		return -1
	}

	linesSoFar := 0
	for i, fp := range m.files {
		fileLines := m.fileLinesCount(fp, i)
		if m.scroll < linesSoFar+fileLines {
			return i
		}
		linesSoFar += fileLines
	}

	// Past all content - return last file
	return len(m.files) - 1
}

// fileLinesCount returns the number of display lines for a file.
func (m Model) fileLinesCount(fp sidebyside.FilePair, fileIdx int) int {
	switch fp.FoldLevel {
	case sidebyside.FoldFolded:
		// Just the header, no blank line before
		return 1
	case sidebyside.FoldExpanded:
		// Header + all content lines (+ blank line before if not first file)
		lines := 1 + len(fp.Pairs) // For now, use Pairs; will change when expanded view is implemented
		if fileIdx > 0 {
			lines++ // blank line before
		}
		return lines
	default: // FoldNormal
		// Header + pairs (+ blank line before if not first file)
		lines := 1 + len(fp.Pairs)
		if fileIdx > 0 {
			lines++ // blank line before
		}
		return lines
	}
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
