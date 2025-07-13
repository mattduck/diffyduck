package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"duckdiff/aligner"
	"duckdiff/git"
	"duckdiff/parser"
	"duckdiff/syntax"
)

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

type Model struct {
	filesWithLines []FileWithLines
	viewport       viewport.Model
	ready          bool
	width          int
	cursorLine     int
	navigableLines []NavigableLineRef
	gPressed       bool
	highlighter    *syntax.Highlighter
}

func NewModel(filesWithLines []FileWithLines) Model {
	model := Model{
		filesWithLines: filesWithLines,
		cursorLine:     0,
		highlighter:    syntax.NewHighlighter(),
	}
	model.navigableLines = model.buildNavigableLines()
	return model
}

func (m Model) buildNavigableLines() []NavigableLineRef {
	var navigable []NavigableLineRef

	for fileIndex, fileWithLines := range m.filesWithLines {
		// Skip navigation for special file types (binary, deleted, etc.)
		if m.shouldShowSpecialNotice(fileWithLines) {
			continue
		}

		inDeletionBlock := false

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

		// Check each line in this file
		for lineIndex := range fileWithLines.AlignedLines {
			if fileIndex == targetFileIndex && lineIndex == targetLineIndex {
				return lineCount
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
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-1)
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 1
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
	return m.viewport.View()
}

func (m *Model) Close() {
	if m.highlighter != nil {
		m.highlighter.Close()
		m.highlighter = nil
	}
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
		Foreground(lipgloss.Color("240")) // dim gray

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

		headerText := fmt.Sprintf("%s %s", fileMarker, fileWithLines.FileDiff.NewPath)
		if fileWithLines.FileDiff.NewPath == "/dev/null" {
			headerText = fmt.Sprintf("%s %s", fileMarker, fileWithLines.FileDiff.OldPath)
		}

		fileHeader := fileHeaderStyle.Width(totalWidth).
			Align(lipgloss.Left).
			Render(headerText)
		content.WriteString(fileHeader)
		content.WriteString("\n")

		content.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top,
			strings.Repeat(" ", lineNumWidth+changeMarkerWidth),
			" │ ",
			leftColumnStyle.Render(""),
			" │ ",
			strings.Repeat(" ", lineNumWidth+changeMarkerWidth),
			" │ ",
			rightColumnStyle.Render(""),
		))
		content.WriteString("\n")

		content.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top,
			strings.Repeat("─", lineNumWidth+changeMarkerWidth),
			"─┼─",
			strings.Repeat("─", contentWidth),
			"─┼─",
			strings.Repeat("─", lineNumWidth+changeMarkerWidth),
			"─┼─",
			strings.Repeat("─", contentWidth),
		))
		content.WriteString("\n")

		// Handle special file types (binary, deleted, new)
		if m.shouldShowSpecialNotice(fileWithLines) {
			notice := m.getFileTypeNotice(fileWithLines)
			noticeStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true).
				Align(lipgloss.Center)

			totalContentWidth := 2*contentWidth + 9 + 2*(lineNumWidth+changeMarkerWidth)
			content.WriteString(noticeStyle.Width(totalContentWidth).Render(notice))
			content.WriteString("\n")
			continue
		}

		// Render aligned lines for this file
		for lineIndex, line := range fileWithLines.AlignedLines {
			var leftContent, rightContent string
			var leftLineNumBlock, rightLineNumBlock string

			if line.OldLine != nil {
				content := *line.OldLine
				content = m.highlighter.HighlightLine(content, fileWithLines.FileDiff.OldPath)
				leftContent = " " + content
				if line.LineType == aligner.Deleted {
					leftLineNumBlock = deletedLineNumStyle.Render(fmt.Sprintf("%d ", line.OldLineNum))
				} else if line.LineType == aligner.Modified {
					leftLineNumBlock = modifiedLineNumStyle.Render(fmt.Sprintf("%d ", line.OldLineNum))
				} else {
					leftLineNumBlock = lineNumStyle.Render(fmt.Sprintf("%d", line.OldLineNum)) + " "
				}
			} else {
				leftLineNumBlock = strings.Repeat(" ", lineNumWidth+changeMarkerWidth)
			}

			// Format right side
			if line.NewLine != nil {
				content := *line.NewLine
				content = m.highlighter.HighlightLine(content, fileWithLines.FileDiff.NewPath)
				rightContent = " " + content
				// Check if cursor is on this line
				cursorMarker := " "
				if m.isCursorAt(fileIndex, lineIndex) {
					cursorMarker = "*"
				}

				if line.LineType == aligner.Added {
					rightLineNumBlock = addedLineNumStyle.Render(fmt.Sprintf("%d%s", line.NewLineNum, cursorMarker))
				} else if line.LineType == aligner.Modified {
					rightLineNumBlock = modifiedLineNumStyle.Render(fmt.Sprintf("%d%s", line.NewLineNum, cursorMarker))
				} else {
					rightLineNumBlock = lineNumStyle.Render(fmt.Sprintf("%d", line.NewLineNum)) + cursorMarker
				}
			} else {
				// For deletion blocks, check if cursor is on this line (first line of deletion block)
				cursorMarker := " "
				if m.isCursorAt(fileIndex, lineIndex) {
					cursorMarker = "*"
				}
				rightLineNumBlock = strings.Repeat(" ", lineNumWidth) + cursorMarker
			}

			content.WriteString(lipgloss.JoinHorizontal(
				lipgloss.Top,
				leftLineNumBlock,
				" │ ",
				leftColumnStyle.Render(leftContent),
				" │ ",
				rightLineNumBlock,
				" │ ",
				rightColumnStyle.Render(rightContent),
			))
			content.WriteString("\n")
		}
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
