package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/syntax"
)

type ExpandLevel int

const (
	HeadersOnly ExpandLevel = iota
	ContextDiff
	FullDiff
)

const DefaultContextLines = 3

type FileWithLines struct {
	FileDiff     parser.FileDiff
	AlignedLines []aligner.AlignedLine
	OldFileType  git.FileType
	NewFileType  git.FileType
}

type NavigableLineRef struct {
	FileIndex int
	LineIndex int
}

type SearchMatch struct {
	FileIndex    int
	LineIndex    int
	IsOldContent bool // true if match is in old content, false if in new content
	StartCol     int
	EndCol       int
}

type Model struct {
	filesWithLines []FileWithLines
	viewport       viewport.Model
	ready          bool
	width          int
	cursorLine     int
	navigableLines []NavigableLineRef
	gPressed       bool
	highlighter    *syntax.Highlighter
	expandLevel    ExpandLevel
	contextLines   int
	// Search-related fields
	searchMode        bool
	searchInput       textinput.Model
	searchQuery       string
	searchMatches     []SearchMatch
	currentMatchIndex int
	searchDirection   bool // true for forward, false for backward
}

func NewModel(filesWithLines []FileWithLines) Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40
	ti.Prompt = ""

	model := Model{
		filesWithLines:    filesWithLines,
		cursorLine:        0,
		highlighter:       syntax.NewHighlighter(),
		expandLevel:       ContextDiff,
		contextLines:      DefaultContextLines,
		searchInput:       ti,
		currentMatchIndex: -1,
		searchDirection:   true,
	}
	model.navigableLines = model.buildNavigableLines()
	return model
}

func (m Model) getEffectiveExpandLevel(fileIndex int) ExpandLevel {
	return m.expandLevel
}

func (m Model) filterAlignedLinesForLevel(lines []aligner.AlignedLine, level ExpandLevel) []aligner.AlignedLine {
	if level == FullDiff {
		return lines
	}
	if level == HeadersOnly {
		return []aligner.AlignedLine{}
	}

	// ContextDiff: show changed lines + N context lines around them
	if len(lines) == 0 {
		return lines
	}

	var result []aligner.AlignedLine
	contextLines := m.contextLines

	// First pass: identify all change blocks (consecutive changed lines)
	changeBlocks := m.identifyChangeBlocks(lines)

	if len(changeBlocks) == 0 {
		// No changes, show limited context from beginning
		endIdx := contextLines
		if endIdx > len(lines) {
			endIdx = len(lines)
		}
		return lines[:endIdx]
	}

	// Second pass: collect lines with context around change blocks
	included := make(map[int]bool)

	for _, block := range changeBlocks {
		// Include the change block itself
		for i := block.start; i <= block.end; i++ {
			included[i] = true
		}

		// Include context before
		contextStart := block.start - contextLines
		if contextStart < 0 {
			contextStart = 0
		}
		for i := contextStart; i < block.start; i++ {
			included[i] = true
		}

		// Include context after
		contextEnd := block.end + contextLines
		if contextEnd >= len(lines) {
			contextEnd = len(lines) - 1
		}
		for i := block.end + 1; i <= contextEnd; i++ {
			included[i] = true
		}
	}

	// Third pass: build result with separators for gaps
	lastIncluded := -1
	for i := 0; i < len(lines); i++ {
		if included[i] {
			// Add separator if there's a gap
			if lastIncluded != -1 && i > lastIncluded+1 {
				result = append(result, m.createContextSeparator())
			}
			result = append(result, lines[i])
			lastIncluded = i
		}
	}

	return result
}

type changeBlock struct {
	start int
	end   int
}

func (m Model) identifyChangeBlocks(lines []aligner.AlignedLine) []changeBlock {
	var blocks []changeBlock
	var currentBlock *changeBlock

	for i, line := range lines {
		isChanged := line.LineType != aligner.Unchanged

		if isChanged {
			if currentBlock == nil {
				currentBlock = &changeBlock{start: i, end: i}
			} else {
				currentBlock.end = i
			}
		} else {
			if currentBlock != nil {
				blocks = append(blocks, *currentBlock)
				currentBlock = nil
			}
		}
	}

	// Don't forget the last block
	if currentBlock != nil {
		blocks = append(blocks, *currentBlock)
	}

	return blocks
}

func (m Model) createContextSeparator() aligner.AlignedLine {
	emptyText := ""
	return aligner.AlignedLine{
		OldLine:    &emptyText,
		NewLine:    &emptyText,
		LineType:   aligner.Unchanged,
		OldLineNum: 0, // Special marker for separator
		NewLineNum: 0, // Special marker for separator
	}
}

func (m *Model) cycleExpandLevel(direction int) {
	levels := []ExpandLevel{HeadersOnly, ContextDiff, FullDiff}
	currentIndex := 0

	// Find current level index
	for i, level := range levels {
		if level == m.expandLevel {
			currentIndex = i
			break
		}
	}

	// Calculate next index with wraparound
	nextIndex := (currentIndex + direction + len(levels)) % len(levels)
	m.expandLevel = levels[nextIndex]
}

func (m *Model) rebuildAndRefresh() {
	m.navigableLines = m.buildNavigableLines()

	// Re-run search if there's an active search query
	if m.searchQuery != "" {
		m.performSearch(m.searchQuery)
	}

	m.viewport.SetContent(m.renderContent())

	// Ensure cursor is still within bounds
	if m.cursorLine >= len(m.navigableLines) && len(m.navigableLines) > 0 {
		m.cursorLine = len(m.navigableLines) - 1
	}

	// Refresh cursor position
	if len(m.navigableLines) > 0 {
		m.scrollToCursor()
	}
}

func (m Model) createOriginalToFilteredMapping(originalLines, filteredLines []aligner.AlignedLine) map[int]int {
	mapping := make(map[int]int)

	// Create mapping by comparing line content and numbers
	for filteredIdx, filteredLine := range filteredLines {
		// Skip context separators
		if filteredLine.OldLineNum == 0 && filteredLine.NewLineNum == 0 {
			continue
		}

		// Find matching original line
		for originalIdx, originalLine := range originalLines {
			if m.linesMatch(originalLine, filteredLine) {
				mapping[originalIdx] = filteredIdx
				break
			}
		}
	}

	return mapping
}

func (m Model) linesMatch(line1, line2 aligner.AlignedLine) bool {
	return line1.OldLineNum == line2.OldLineNum &&
		line1.NewLineNum == line2.NewLineNum &&
		line1.LineType == line2.LineType
}

func (m Model) buildNavigableLines() []NavigableLineRef {
	var navigable []NavigableLineRef

	for fileIndex, fileWithLines := range m.filesWithLines {
		// Skip navigation for special file types (binary, deleted, etc.)
		if m.shouldShowSpecialNotice(fileWithLines) {
			continue
		}

		inDeletionBlock := false

		// Always use the original AlignedLines for navigation indexing
		// The filtering is only for display purposes
		for lineIndex, line := range fileWithLines.AlignedLines {
			if line.NewLine != nil && line.LineType != aligner.Unchanged {
				// Line exists in new version and is part of the diff (added or modified)
				navigable = append(navigable, NavigableLineRef{
					FileIndex: fileIndex,
					LineIndex: lineIndex,
				})
				inDeletionBlock = false
			} else if line.LineType == aligner.Deleted && !inDeletionBlock {
				// First line of a deletion block - navigable
				navigable = append(navigable, NavigableLineRef{
					FileIndex: fileIndex,
					LineIndex: lineIndex,
				})
				inDeletionBlock = true
			} else if line.LineType == aligner.Unchanged {
				// Unchanged lines reset deletion block state
				inDeletionBlock = false
			}
			// Skip subsequent lines in deletion block
		}
	}

	return navigable
}

func (m Model) isCursorAt(fileIndex, lineIndex int) bool {
	if m.cursorLine >= len(m.navigableLines) {
		return false
	}
	cursorRef := m.navigableLines[m.cursorLine]
	return cursorRef.FileIndex == fileIndex && cursorRef.LineIndex == lineIndex
}

func (m *Model) scrollToCursor() {
	if m.cursorLine >= len(m.navigableLines) || m.viewport.Height <= 0 {
		return
	}

	// Calculate the rendered line number for the cursor position
	cursorRef := m.navigableLines[m.cursorLine]
	renderedLineNum := m.calculateRenderedLineNumber(cursorRef.FileIndex, cursorRef.LineIndex)

	// Get viewport dimensions
	viewportTop := m.viewport.YOffset
	viewportBottom := viewportTop + m.viewport.Height - 1

	// Add some margin to keep cursor away from edges
	margin := 2

	// Scroll if cursor is outside viewport or too close to edges
	if renderedLineNum < viewportTop+margin {
		newOffset := renderedLineNum - margin
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
	} else if renderedLineNum > viewportBottom-margin {
		newOffset := renderedLineNum - m.viewport.Height + margin + 1
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
	}
}

func (m *Model) scrollToCursorCenter() {
	if m.cursorLine >= len(m.navigableLines) || m.viewport.Height <= 0 {
		return
	}

	// Calculate the rendered line number for the cursor position
	cursorRef := m.navigableLines[m.cursorLine]
	renderedLineNum := m.calculateRenderedLineNumber(cursorRef.FileIndex, cursorRef.LineIndex)

	// Center cursor in viewport
	newOffset := renderedLineNum - m.viewport.Height/2
	if newOffset < 0 {
		newOffset = 0
	}
	m.viewport.SetYOffset(newOffset)
}

func (m Model) calculateRenderedLineNumber(targetFileIndex, targetLineIndex int) int {
	lineCount := 0

	for fileIndex, fileWithLines := range m.filesWithLines {
		if fileIndex > 0 {
			lineCount++ // newline between files
		}

		// File header
		lineCount++
		// Column headers
		lineCount++
		// Separator line
		lineCount++

		// Use filtered lines for counting, just like rendering does
		effectiveLevel := m.getEffectiveExpandLevel(fileIndex)
		linesToRender := m.filterAlignedLinesForLevel(fileWithLines.AlignedLines, effectiveLevel)
		originalToFiltered := m.createOriginalToFilteredMapping(fileWithLines.AlignedLines, linesToRender)

		// Check each filtered line in this file
		for filteredIndex := range linesToRender {
			// Find if this filtered line corresponds to our target original line
			for originalIdx, filteredIdx := range originalToFiltered {
				if filteredIdx == filteredIndex && fileIndex == targetFileIndex && originalIdx == targetLineIndex {
					return lineCount
				}
			}
			lineCount++
		}
	}

	return lineCount
}

func (m *Model) gotoNextBlock() {
	if len(m.navigableLines) == 0 || m.cursorLine >= len(m.navigableLines)-1 {
		return
	}

	// Get current line's type and file
	currentType := m.getLineTypeAtCursor(m.cursorLine)
	currentFileIndex := m.navigableLines[m.cursorLine].FileIndex
	currentLineIndex := m.navigableLines[m.cursorLine].LineIndex

	// Skip forward while in same block (same type + same file + consecutive lines)
	nextPos := m.cursorLine + 1
	for nextPos < len(m.navigableLines) {
		nextType := m.getLineTypeAtCursor(nextPos)
		nextFileIndex := m.navigableLines[nextPos].FileIndex
		nextLineIndex := m.navigableLines[nextPos].LineIndex

		// Found start of next block?
		// New block if: different type OR different file OR non-consecutive lines (context in between)
		if nextType != currentType ||
			nextFileIndex != currentFileIndex ||
			nextLineIndex != currentLineIndex+1 {
			// Use exact same pattern as j/k but center for big jumps
			m.cursorLine = nextPos
			m.viewport.SetContent(m.renderContent())
			m.scrollToCursorCenter()
			return
		}

		// Update for next iteration
		currentLineIndex = nextLineIndex
		nextPos++
	}
	// No next block found - do nothing (like j/k at end)
}

func (m *Model) gotoPrevBlock() {
	if len(m.navigableLines) == 0 || m.cursorLine <= 0 {
		return
	}

	// Get current line's type and file
	currentType := m.getLineTypeAtCursor(m.cursorLine)
	currentFileIndex := m.navigableLines[m.cursorLine].FileIndex
	currentLineIndex := m.navigableLines[m.cursorLine].LineIndex

	// Skip backward while in same block (same type + same file + consecutive lines)
	prevPos := m.cursorLine - 1
	for prevPos >= 0 {
		prevType := m.getLineTypeAtCursor(prevPos)
		prevFileIndex := m.navigableLines[prevPos].FileIndex
		prevLineIndex := m.navigableLines[prevPos].LineIndex

		// Found start of previous block?
		// New block if: different type OR different file OR non-consecutive lines (context in between)
		if prevType != currentType ||
			prevFileIndex != currentFileIndex ||
			prevLineIndex != currentLineIndex-1 {

			// Phase 2: Found a line in previous block, now find the START of that block
			blockType := prevType
			blockFileIndex := prevFileIndex
			blockStartPos := prevPos

			// Scan backward to find start of this block
			for blockStartPos > 0 {
				checkPos := blockStartPos - 1
				checkType := m.getLineTypeAtCursor(checkPos)
				checkFileIndex := m.navigableLines[checkPos].FileIndex
				checkLineIndex := m.navigableLines[checkPos].LineIndex

				// Stop if we hit a different block
				if checkType != blockType ||
					checkFileIndex != blockFileIndex ||
					checkLineIndex != m.navigableLines[blockStartPos].LineIndex-1 {
					break
				}
				blockStartPos = checkPos
			}

			// Go to start of the previous block
			m.cursorLine = blockStartPos
			m.viewport.SetContent(m.renderContent())
			m.scrollToCursorCenter()
			return
		}

		// Update for next iteration
		currentLineIndex = prevLineIndex
		prevPos--
	}
	// No previous block found - do nothing (like k at beginning)
}

func (m Model) getLineTypeAtCursor(cursorIndex int) aligner.LineType {
	if cursorIndex >= len(m.navigableLines) {
		return aligner.Unchanged
	}

	ref := m.navigableLines[cursorIndex]
	line := m.filesWithLines[ref.FileIndex].AlignedLines[ref.LineIndex]
	return line.LineType
}

func (m *Model) gotoTop() {
	// Go to first navigable line and scroll to top
	if len(m.navigableLines) > 0 {
		m.cursorLine = 0
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
	}
}

type gTimeoutMsg struct{}

func gTimeout() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return gTimeoutMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case gTimeoutMsg:
		// Timeout for g key - just reset the state
		m.gPressed = false
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Handle search mode
		if m.searchMode {
			switch key {
			case "esc", "escape":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				// Submit search
				query := m.searchInput.Value()
				m.performSearch(query)
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			default:
				// Pass to text input
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		}

		// Handle g + j/k/g sequences
		if m.gPressed {
			m.gPressed = false
			switch key {
			case "j":
				m.gotoNextBlock()
				return m, nil
			case "k":
				m.gotoPrevBlock()
				return m, nil
			case "g":
				m.gotoTop()
				return m, nil
			default:
				// Invalid sequence, reset state
				return m, nil
			}
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			// Enter forward search mode
			m.searchMode = true
			m.searchDirection = true
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			return m, nil
		case "?":
			// Enter backward search mode
			m.searchMode = true
			m.searchDirection = false
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			return m, nil
		case "n":
			// Next match
			if m.searchDirection {
				m.nextMatch()
			} else {
				m.prevMatch()
			}
			return m, nil
		case "N":
			// Previous match (reverse of current direction)
			if m.searchDirection {
				m.prevMatch()
			} else {
				m.nextMatch()
			}
			return m, nil
		case "tab":
			// Cycle through expand levels: HeadersOnly -> ContextDiff -> FullDiff -> HeadersOnly
			m.cycleExpandLevel(1)
			m.rebuildAndRefresh()
			return m, nil
		case "shift+tab":
			// Reverse cycle through expand levels
			m.cycleExpandLevel(-1)
			m.rebuildAndRefresh()
			return m, nil
		// TODO: Re-enable cursor navigation - j/k for line-by-line movement through navigable lines
		// This feature allows precise navigation through diff changes only (skipping unchanged lines)
		// To restore: uncomment the cases below and remove "j", "k" from viewport key blocking (line ~361)
		// Related functions: scrollToCursor(), buildNavigableLines(), navigableLines field
		/*
			case "j", "down":
				if m.cursorLine < len(m.navigableLines)-1 {
					m.cursorLine++
					m.viewport.SetContent(m.renderContent())
					m.scrollToCursor()
				}
			case "k", "up":
				if m.cursorLine > 0 {
					m.cursorLine--
					m.viewport.SetContent(m.renderContent())
					m.scrollToCursor()
				}
		*/
		case "down":
			if m.cursorLine < len(m.navigableLines)-1 {
				m.cursorLine++
				m.viewport.SetContent(m.renderContent())
				m.scrollToCursor()
			}
		case "up":
			if m.cursorLine > 0 {
				m.cursorLine--
				m.viewport.SetContent(m.renderContent())
				m.scrollToCursor()
			}
		case "d":
			m.viewport.HalfViewDown()
		case "u":
			m.viewport.HalfViewUp()
		case "g":
			m.gPressed = true
			return m, gTimeout()
		case "G":
			m.viewport.GotoBottom()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width

		// Calculate height: reserve space for status line and search input (if active)
		reservedHeight := 2 // status line
		if m.searchMode {
			reservedHeight += 1 // search input line
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-reservedHeight)
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - reservedHeight
			m.viewport.SetContent(m.renderContent())
		}
	}

	// Only pass non-cursor navigation messages to viewport
	var cmd tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "down", "up":
			// Don't pass cursor navigation keys to viewport
			// TODO: Add "j", "k" back to this list when re-enabling cursor navigation
		default:
			m.viewport, cmd = m.viewport.Update(msg)
		}
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Create status line
	statusLine := m.createStatusLine()

	// Combine viewport content with status line
	content := m.viewport.View()
	if statusLine != "" {
		content += "\n" + statusLine
	}

	// Add search input if in search mode
	if m.searchMode {
		searchPrompt := "/"
		if !m.searchDirection {
			searchPrompt = "?"
		}
		searchLine := searchPrompt + m.searchInput.View()
		content += "\n" + searchLine
	}

	return content
}

func (m Model) createStatusLine() string {
	if m.width == 0 {
		return ""
	}

	// Get expand level name
	var levelName string
	switch m.expandLevel {
	case HeadersOnly:
		levelName = "Headers"
	case ContextDiff:
		levelName = fmt.Sprintf("Context(%d)", m.contextLines)
	case FullDiff:
		levelName = "Full"
	}

	// Create status parts
	leftStatus := fmt.Sprintf("[%s]", levelName)

	// Add search info if there are matches
	if len(m.searchMatches) > 0 && m.searchQuery != "" {
		leftStatus += fmt.Sprintf(" Search: %d/%d", m.currentMatchIndex+1, len(m.searchMatches))
	}

	rightStatus := "/:search ?:reverse n/N:next/prev Tab:cycle"

	// Calculate padding
	totalUsed := len(leftStatus) + len(rightStatus)
	padding := ""
	if totalUsed < m.width {
		padding = strings.Repeat(" ", m.width-totalUsed)
	}

	// Style the status line
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("7"))

	return statusStyle.Width(m.width).Render(leftStatus + padding + rightStatus)
}

func (m *Model) Close() {
	if m.highlighter != nil {
		m.highlighter.Close()
		m.highlighter = nil
	}
}

func (m *Model) performSearch(query string) {
	if query == "" {
		m.searchMatches = nil
		m.currentMatchIndex = -1
		return
	}

	// Determine if search should be case-sensitive
	caseSensitive := false
	for _, r := range query {
		if r >= 'A' && r <= 'Z' {
			caseSensitive = true
			break
		}
	}

	searchQuery := query
	if !caseSensitive {
		searchQuery = strings.ToLower(query)
	}

	var matches []SearchMatch

	// Search through all files and lines
	for fileIndex, fileWithLines := range m.filesWithLines {
		// Skip special files (binary, etc.)
		if m.shouldShowSpecialNotice(fileWithLines) {
			continue
		}

		// Get effective expand level and filter lines accordingly
		effectiveLevel := m.getEffectiveExpandLevel(fileIndex)
		visibleLines := m.filterAlignedLinesForLevel(fileWithLines.AlignedLines, effectiveLevel)

		// Create mapping from original to filtered indices
		originalToFiltered := m.createOriginalToFilteredMapping(fileWithLines.AlignedLines, visibleLines)

		// Only search through visible lines
		for originalLineIndex, line := range fileWithLines.AlignedLines {
			// Skip if this line is not visible in current expand level
			if _, isVisible := originalToFiltered[originalLineIndex]; !isVisible {
				continue
			}
			// Search in old content
			if line.OldLine != nil {
				content := *line.OldLine
				searchContent := content
				if !caseSensitive {
					searchContent = strings.ToLower(content)
				}

				// Find all occurrences in the line
				startPos := 0
				for {
					idx := strings.Index(searchContent[startPos:], searchQuery)
					if idx == -1 {
						break
					}
					matches = append(matches, SearchMatch{
						FileIndex:    fileIndex,
						LineIndex:    originalLineIndex,
						IsOldContent: true,
						StartCol:     startPos + idx,
						EndCol:       startPos + idx + len(searchQuery),
					})
					startPos += idx + 1
				}
			}

			// Search in new content
			if line.NewLine != nil {
				content := *line.NewLine
				searchContent := content
				if !caseSensitive {
					searchContent = strings.ToLower(content)
				}

				// Find all occurrences in the line
				startPos := 0
				for {
					idx := strings.Index(searchContent[startPos:], searchQuery)
					if idx == -1 {
						break
					}
					matches = append(matches, SearchMatch{
						FileIndex:    fileIndex,
						LineIndex:    originalLineIndex,
						IsOldContent: false,
						StartCol:     startPos + idx,
						EndCol:       startPos + idx + len(searchQuery),
					})
					startPos += idx + 1
				}
			}
		}
	}

	m.searchMatches = matches
	m.searchQuery = query

	// Reset current match index
	if len(matches) > 0 {
		if m.searchDirection {
			m.currentMatchIndex = 0
		} else {
			m.currentMatchIndex = len(matches) - 1
		}
		// Scroll to first match
		m.scrollToMatch(m.currentMatchIndex)
	} else {
		m.currentMatchIndex = -1
	}
}

func (m *Model) scrollToMatch(matchIndex int) {
	if matchIndex < 0 || matchIndex >= len(m.searchMatches) {
		return
	}

	match := m.searchMatches[matchIndex]

	// Calculate the rendered line number for this match
	renderedLineNum := m.calculateRenderedLineNumber(match.FileIndex, match.LineIndex)

	// Put the match at the top of the viewport
	m.viewport.SetYOffset(renderedLineNum)
	m.viewport.SetContent(m.renderContent())
}

func (m *Model) nextMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatchIndex = (m.currentMatchIndex + 1) % len(m.searchMatches)
	m.scrollToMatch(m.currentMatchIndex)
}

func (m *Model) prevMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatchIndex = (m.currentMatchIndex - 1 + len(m.searchMatches)) % len(m.searchMatches)
	m.scrollToMatch(m.currentMatchIndex)
}

func (m *Model) lineHasSearchMatches(fileIndex int, lineIndex int, isOldContent bool) bool {
	if m.searchQuery == "" || len(m.searchMatches) == 0 {
		return false
	}

	for _, match := range m.searchMatches {
		if match.FileIndex == fileIndex && match.LineIndex == lineIndex && match.IsOldContent == isOldContent {
			return true
		}
	}
	return false
}

func (m *Model) highlightSearchMatches(content string, fileIndex int, lineIndex int, isOldContent bool) string {
	if m.searchQuery == "" || len(m.searchMatches) == 0 {
		return content
	}

	// Find matches for this line
	var matches []SearchMatch
	for _, match := range m.searchMatches {
		if match.FileIndex == fileIndex && match.LineIndex == lineIndex && match.IsOldContent == isOldContent {
			matches = append(matches, match)
		}
	}

	if len(matches) == 0 {
		return content
	}

	// Sort matches by start position (descending) to process from right to left
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].StartCol < matches[j].StartCol {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// Apply highlighting from right to left to avoid position shifts
	result := content
	for _, match := range matches {
		matchIndex := -1
		for j, globalMatch := range m.searchMatches {
			if globalMatch.FileIndex == match.FileIndex &&
				globalMatch.LineIndex == match.LineIndex &&
				globalMatch.IsOldContent == match.IsOldContent &&
				globalMatch.StartCol == match.StartCol {
				matchIndex = j
				break
			}
		}

		// Choose highlight style based on whether this is the current match
		var highlightStyle lipgloss.Style
		if matchIndex == m.currentMatchIndex {
			// Current match - red background
			highlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("1")). // red
				Foreground(lipgloss.Color("15")) // white text
		} else {
			// Other matches - yellow background
			highlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("11")). // bright yellow
				Foreground(lipgloss.Color("0"))   // black text
		}

		// Extract the match text and apply highlighting
		if match.StartCol >= 0 && match.EndCol <= len(result) {
			before := result[:match.StartCol]
			matchText := result[match.StartCol:match.EndCol]
			after := result[match.EndCol:]

			highlightedMatch := highlightStyle.Render(matchText)
			result = before + highlightedMatch + after
		}
	}

	return result
}

func (m Model) renderContent() string {
	var content strings.Builder

	// Line number column widths
	const lineNumWidth = 5
	const changeMarkerWidth = 1

	// Calculate column widths: account for line numbers and separators
	// Layout: [lineNum+marker] | [content] | [lineNum+marker] | [content]
	// Total separators: 3 * " | " = 9 chars, plus 2 * (lineNumWidth + changeMarkerWidth)
	totalSeparators := 9 + 2*(lineNumWidth+changeMarkerWidth)
	contentWidth := (m.width - totalSeparators) / 2
	if contentWidth < 20 {
		contentWidth = 20 // minimum width
	}

	leftColumnStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left)

	rightColumnStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left)

	lineNumStyle := lipgloss.NewStyle().
		Width(lineNumWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("0")) // dim gray

	addedLineNumStyle := lipgloss.NewStyle().
		Width(lineNumWidth + changeMarkerWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("2")). // standard ANSI green
		Background(lipgloss.Color("16")) // dark background for non-context lines

	deletedLineNumStyle := lipgloss.NewStyle().
		Width(lineNumWidth + changeMarkerWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("1")). // standard ANSI red
		Background(lipgloss.Color("16")) // dark background for non-context lines

	modifiedLineNumStyle := lipgloss.NewStyle().
		Width(lineNumWidth + changeMarkerWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("4")). // standard ANSI blue
		Background(lipgloss.Color("16")) // dark background for non-context lines

	emptyLineNumStyle := lipgloss.NewStyle().
		Width(lineNumWidth + changeMarkerWidth).
		Align(lipgloss.Right).
		Background(lipgloss.Color("16")) // dark background to match changed lines

	for fileIndex, fileWithLines := range m.filesWithLines {
		if fileIndex > 0 {
			content.WriteString("\n")
		}

		// Determine file status marker and color
		var fileMarker string
		var color string
		if fileWithLines.FileDiff.OldPath == "/dev/null" {
			// New file
			fileMarker = "+"
			color = "2" // green
		} else if fileWithLines.FileDiff.NewPath == "/dev/null" {
			// Deleted file
			fileMarker = "-"
			color = "1" // red
		} else {
			// Modified file
			fileMarker = "~"
			color = "4" // blue
		}

		// File header styling
		fileHeaderStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(color))

		totalWidth := 2*(lineNumWidth+changeMarkerWidth+contentWidth) + 9 // account for separators

		// Add top horizontal separator before file header
		if fileIndex > 0 || true { // Always show top separator
			content.WriteString(lipgloss.JoinHorizontal(
				lipgloss.Top,
				strings.Repeat("─", totalWidth),
			))
			content.WriteString("\n")
		}

		// Generate base filename text
		baseFilename := fileWithLines.FileDiff.NewPath
		if fileWithLines.FileDiff.NewPath == "/dev/null" {
			baseFilename = fileWithLines.FileDiff.OldPath
		}

		// Create stat info to append to filename
		statInfo := ""
		totalChanges := fileWithLines.FileDiff.Additions + fileWithLines.FileDiff.Deletions
		if totalChanges > 0 {
			// Create visual representation with + and - characters
			maxStatChars := 20
			additionChars := 0
			deletionChars := 0

			if totalChanges <= maxStatChars {
				additionChars = fileWithLines.FileDiff.Additions
				deletionChars = fileWithLines.FileDiff.Deletions
			} else {
				// Scale down proportionally
				ratio := float64(maxStatChars) / float64(totalChanges)
				additionChars = int(float64(fileWithLines.FileDiff.Additions) * ratio)
				deletionChars = int(float64(fileWithLines.FileDiff.Deletions) * ratio)
				// Ensure at least 1 char if there are changes
				if fileWithLines.FileDiff.Additions > 0 && additionChars == 0 {
					additionChars = 1
				}
				if fileWithLines.FileDiff.Deletions > 0 && deletionChars == 0 {
					deletionChars = 1
				}
			}

			// Build stat text with proper colors
			statText := fmt.Sprintf(" %d ", totalChanges)
			if additionChars > 0 {
				statText += lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(strings.Repeat("+", additionChars))
			}
			if deletionChars > 0 {
				statText += lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(strings.Repeat("-", deletionChars))
			}
			statInfo = statText
		}

		// Create filename part with file marker (colored)
		filenameText := fmt.Sprintf("%s %s", fileMarker, baseFilename)
		filenameStyled := fileHeaderStyle.Render(filenameText)

		// Combine filename with separator and stat info
		headerContent := filenameStyled
		if statInfo != "" {
			headerContent += " | " + statInfo
		}

		// Render the complete header line
		finalHeader := lipgloss.NewStyle().Width(totalWidth).Align(lipgloss.Left).Render(headerContent)
		content.WriteString(finalHeader)
		content.WriteString("\n")

		// Removed vertical separator lines - they don't add value

		content.WriteString(strings.Repeat("─", totalWidth))
		content.WriteString("\n")

		// Handle special file types (binary, deleted, new)
		if m.shouldShowSpecialNotice(fileWithLines) {
			notice := m.getFileTypeNotice(fileWithLines)
			noticeStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Italic(true).
				Align(lipgloss.Center)

			totalContentWidth := 2*contentWidth + 9 + 2*(lineNumWidth+changeMarkerWidth)
			content.WriteString(noticeStyle.Width(totalContentWidth).Render(notice))
			content.WriteString("\n")
			continue
		}

		// Get effective expand level for this file and filter lines accordingly
		effectiveLevel := m.getEffectiveExpandLevel(fileIndex)
		linesToRender := m.filterAlignedLinesForLevel(fileWithLines.AlignedLines, effectiveLevel)

		// Create mapping from original indices to filtered indices for cursor display
		originalToFiltered := m.createOriginalToFilteredMapping(fileWithLines.AlignedLines, linesToRender)

		// Render aligned lines for this file
		for filteredIndex, line := range linesToRender {
			var leftContent, rightContent string
			var leftLineNumBlock, rightLineNumBlock string

			if line.OldLine != nil {
				var content string
				// Handle context separator with full-width dash line
				if line.OldLineNum == 0 {
					// content = strings.Repeat("-", contentWidth-1) // -1 for the leading space
					content = strings.Repeat("-", contentWidth)
					leftContent = content
				} else if line.LineType == aligner.Modified && line.WordDiff != nil {
					// Find original line index for this filtered line
					originalLineIndex := filteredIndex
					for origIdx, filtIdx := range originalToFiltered {
						if filtIdx == filteredIndex {
							originalLineIndex = origIdx
							break
						}
					}

					originalText := *line.OldLine

					// Check if this line has search matches
					if m.lineHasSearchMatches(fileIndex, originalLineIndex, true) {
						// Skip syntax highlighting and just apply search highlighting
						content = m.highlightSearchMatches(originalText, fileIndex, originalLineIndex, true)
					} else {
						// Apply normal syntax highlighting with word diff
						content = m.highlighter.HighlightLineWithWordDiff(originalText, fileWithLines.FileDiff.OldPath, line.WordDiff.OldSegments)
					}
					leftContent = " " + content
				} else {
					// Find original line index for this filtered line
					originalLineIndex := filteredIndex
					for origIdx, filtIdx := range originalToFiltered {
						if filtIdx == filteredIndex {
							originalLineIndex = origIdx
							break
						}
					}

					originalText := *line.OldLine

					// Check if this line has search matches
					if m.lineHasSearchMatches(fileIndex, originalLineIndex, true) {
						// Skip syntax highlighting and just apply search highlighting
						content = m.highlightSearchMatches(originalText, fileIndex, originalLineIndex, true)
					} else {
						// Apply normal syntax highlighting
						content = m.highlighter.HighlightLine(originalText, fileWithLines.FileDiff.OldPath)
					}
					leftContent = " " + content
				}

				// Handle context separator (line numbers are 0)
				if line.OldLineNum == 0 {
					leftLineNumBlock = strings.Repeat("-", lineNumWidth+changeMarkerWidth)
				} else if line.LineType == aligner.Deleted {
					leftLineNumBlock = deletedLineNumStyle.Render(fmt.Sprintf("%d ", line.OldLineNum))
				} else if line.LineType == aligner.Modified {
					leftLineNumBlock = modifiedLineNumStyle.Render(fmt.Sprintf("%d ", line.OldLineNum))
				} else {
					leftLineNumBlock = lineNumStyle.Render(fmt.Sprintf("%d", line.OldLineNum)) + " "
				}
			} else {
				// Empty left side - apply background to match changed lines
				leftLineNumBlock = emptyLineNumStyle.Render(strings.Repeat(" ", lineNumWidth+changeMarkerWidth))
			}

			// Format right side
			if line.NewLine != nil {
				var content string
				// Handle context separator with full-width dash line
				if line.NewLineNum == 0 {
					// content = strings.Repeat("-", contentWidth-1) // -1 for the leading space
					content = strings.Repeat("-", contentWidth)
					rightContent = content
				} else if line.LineType == aligner.Modified && line.WordDiff != nil {
					// Find original line index for this filtered line
					originalLineIndex := filteredIndex
					for origIdx, filtIdx := range originalToFiltered {
						if filtIdx == filteredIndex {
							originalLineIndex = origIdx
							break
						}
					}

					originalText := *line.NewLine

					// Check if this line has search matches
					if m.lineHasSearchMatches(fileIndex, originalLineIndex, false) {
						// Skip syntax highlighting and just apply search highlighting
						content = m.highlightSearchMatches(originalText, fileIndex, originalLineIndex, false)
					} else {
						// Apply normal syntax highlighting with word diff
						content = m.highlighter.HighlightLineWithWordDiff(originalText, fileWithLines.FileDiff.NewPath, line.WordDiff.NewSegments)
					}
					rightContent = " " + content
				} else {
					// Find original line index for this filtered line
					originalLineIndex := filteredIndex
					for origIdx, filtIdx := range originalToFiltered {
						if filtIdx == filteredIndex {
							originalLineIndex = origIdx
							break
						}
					}

					originalText := *line.NewLine

					// Check if this line has search matches
					if m.lineHasSearchMatches(fileIndex, originalLineIndex, false) {
						// Skip syntax highlighting and just apply search highlighting
						content = m.highlightSearchMatches(originalText, fileIndex, originalLineIndex, false)
					} else {
						// Apply normal syntax highlighting
						content = m.highlighter.HighlightLine(originalText, fileWithLines.FileDiff.NewPath)
					}
					rightContent = " " + content
				}
				// Check if cursor is on this line (need to map from original to filtered index)
				cursorMarker := " "
				// Find if any original line that maps to this filtered line has cursor
				for originalIdx, filteredIdx := range originalToFiltered {
					if filteredIdx == filteredIndex && m.isCursorAt(fileIndex, originalIdx) {
						cursorMarker = "*"
						break
					}
				}

				// Handle context separator (line numbers are 0)
				if line.NewLineNum == 0 {
					// rightLineNumBlock = strings.Repeat("-", lineNumWidth) + cursorMarker
					rightLineNumBlock = strings.Repeat("-", lineNumWidth+1)
				} else if line.LineType == aligner.Added {
					rightLineNumBlock = addedLineNumStyle.Render(fmt.Sprintf("%d%s", line.NewLineNum, cursorMarker))
				} else if line.LineType == aligner.Modified {
					rightLineNumBlock = modifiedLineNumStyle.Render(fmt.Sprintf("%d%s", line.NewLineNum, cursorMarker))
				} else {
					rightLineNumBlock = lineNumStyle.Render(fmt.Sprintf("%d", line.NewLineNum)) + cursorMarker
				}
			} else {
				// For deletion blocks, check if cursor is on this line (first line of deletion block)
				cursorMarker := " "
				// Find if any original line that maps to this filtered line has cursor
				for originalIdx, filteredIdx := range originalToFiltered {
					if filteredIdx == filteredIndex && m.isCursorAt(fileIndex, originalIdx) {
						cursorMarker = "*"
						break
					}
				}
				// Empty right side - apply background to match changed lines
				// Note: emptyLineNumStyle already has the full width, so we adjust for cursor
				emptyContent := strings.Repeat(" ", lineNumWidth) + cursorMarker
				rightLineNumBlock = emptyLineNumStyle.Render(emptyContent)
			}

			content.WriteString(lipgloss.JoinHorizontal(
				lipgloss.Top,
				leftLineNumBlock,
				// TODO: let's make inclusion of these a config variable.
				// "│ ",
				leftColumnStyle.Render(leftContent),
				"│",
				rightLineNumBlock,
				// "│ ",
				rightColumnStyle.Render(rightContent),
			))
			content.WriteString("\n")
		}
	}

	// Add summary line at the bottom
	totalFiles := len(m.filesWithLines)
	totalAdditions := 0
	totalDeletions := 0

	for _, fileWithLines := range m.filesWithLines {
		totalAdditions += fileWithLines.FileDiff.Additions
		totalDeletions += fileWithLines.FileDiff.Deletions
	}

	if totalFiles > 0 {
		content.WriteString("\n")

		// Format summary text
		filesText := "file"
		if totalFiles > 1 {
			filesText = "files"
		}

		summaryText := fmt.Sprintf(" %d %s changed", totalFiles, filesText)

		if totalAdditions > 0 {
			additionsText := "insertion"
			if totalAdditions > 1 {
				additionsText = "insertions"
			}
			summaryText += fmt.Sprintf(", %d %s(+)", totalAdditions, additionsText)
		}

		if totalDeletions > 0 {
			deletionsText := "deletion"
			if totalDeletions > 1 {
				deletionsText = "deletions"
			}
			summaryText += fmt.Sprintf(", %d %s(-)", totalDeletions, deletionsText)
		}

		// Style the summary line
		summaryStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

		content.WriteString(summaryStyle.Render(summaryText))
		content.WriteString("\n")
	}

	return content.String()
}

func (m Model) shouldShowSpecialNotice(fileWithLines FileWithLines) bool {
	// Show notice for binary files
	if fileWithLines.OldFileType == git.BinaryFile || fileWithLines.NewFileType == git.BinaryFile {
		return true
	}

	// Show notice for completely deleted files (no aligned lines to show)
	// Only if NewPath is explicitly /dev/null (not just an empty/zero file type)
	if fileWithLines.FileDiff.NewPath == "/dev/null" && len(fileWithLines.AlignedLines) == 0 {
		return true
	}

	// Show notice for completely new files (no aligned lines to show)
	// Only if OldPath is explicitly /dev/null (not just an empty/zero file type)
	if fileWithLines.FileDiff.OldPath == "/dev/null" && len(fileWithLines.AlignedLines) == 0 {
		return true
	}

	return false
}

func (m Model) getFileTypeNotice(fileWithLines FileWithLines) string {
	// Handle binary files
	if fileWithLines.OldFileType == git.BinaryFile || fileWithLines.NewFileType == git.BinaryFile {
		if fileWithLines.FileDiff.OldPath == "/dev/null" {
			return "Binary file added"
		} else if fileWithLines.FileDiff.NewPath == "/dev/null" {
			return "Binary file deleted"
		} else {
			return "Binary file changed"
		}
	}

	// Handle completely deleted files
	if fileWithLines.FileDiff.NewPath == "/dev/null" || fileWithLines.NewFileType == git.DeletedFile {
		return "File deleted"
	}

	// Handle completely new files without content
	if fileWithLines.FileDiff.OldPath == "/dev/null" || fileWithLines.OldFileType == git.NewFile {
		return "New file added"
	}

	return "No content to display"
}
