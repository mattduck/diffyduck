package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"duckdiff/aligner"
	"duckdiff/parser"
)

type FileWithLines struct {
	FileDiff parser.FileDiff
	AlignedLines []aligner.AlignedLine
}

type NavigableLineRef struct {
	FileIndex int
	LineIndex int
}

type Model struct {
	filesWithLines []FileWithLines
	viewport viewport.Model
	ready bool
	width int
	cursorLine int
	navigableLines []NavigableLineRef
}

func NewModel(filesWithLines []FileWithLines) Model {
	model := Model{
		filesWithLines: filesWithLines,
		cursorLine: 0,
	}
	model.navigableLines = model.buildNavigableLines()
	return model
}

func (m Model) buildNavigableLines() []NavigableLineRef {
	var navigable []NavigableLineRef
	
	for fileIndex, fileWithLines := range m.filesWithLines {
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
			}
			// Skip unchanged lines and subsequent lines in deletion block
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
	if renderedLineNum < viewportTop + margin {
		newOffset := renderedLineNum - margin
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
	} else if renderedLineNum > viewportBottom - margin {
		newOffset := renderedLineNum - m.viewport.Height + margin + 1
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
	}
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

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
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
		case "d":
			m.viewport.HalfViewDown()
		case "u":
			m.viewport.HalfViewUp()
		case "g":
			m.viewport.GotoTop()
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
		case "j", "k", "down", "up":
			// Don't pass cursor navigation keys to viewport
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

func (m Model) renderContent() string {
	var content strings.Builder
	
	// Line number column widths
	const lineNumWidth = 5
	const changeMarkerWidth = 1
	
	// Calculate column widths: account for line numbers and separators
	// Layout: [lineNum+marker] | [content] | [lineNum+marker] | [content]
	// Total separators: 3 * " | " = 9 chars, plus 2 * (lineNumWidth + changeMarkerWidth)
	totalSeparators := 9 + 2*(lineNumWidth + changeMarkerWidth)
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
		Foreground(lipgloss.Color("2")) // standard ANSI green
		
	deletedLineNumStyle := lipgloss.NewStyle().
		Width(lineNumWidth + changeMarkerWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("1")) // standard ANSI red
		
	modifiedLineNumStyle := lipgloss.NewStyle().
		Width(lineNumWidth + changeMarkerWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("4")) // standard ANSI blue
	
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
			
		totalWidth := 2*(lineNumWidth + changeMarkerWidth + contentWidth) + 9 // account for separators
		
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
			strings.Repeat(" ", lineNumWidth + changeMarkerWidth),
			" │ ",
			leftColumnStyle.Render(""),
			" │ ",
			strings.Repeat(" ", lineNumWidth + changeMarkerWidth),
			" │ ",
			rightColumnStyle.Render(""),
		))
		content.WriteString("\n")
		
		content.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top,
			strings.Repeat("─", lineNumWidth + changeMarkerWidth),
			"─┼─",
			strings.Repeat("─", contentWidth),
			"─┼─",
			strings.Repeat("─", lineNumWidth + changeMarkerWidth),
			"─┼─",
			strings.Repeat("─", contentWidth),
		))
		content.WriteString("\n")
		
		// Render aligned lines for this file
		for lineIndex, line := range fileWithLines.AlignedLines {
			var leftContent, rightContent string
			var leftLineNumBlock, rightLineNumBlock string
			
			if line.OldLine != nil {
				leftContent = " " + *line.OldLine
				if line.LineType == aligner.Deleted {
					leftLineNumBlock = deletedLineNumStyle.Render(fmt.Sprintf("%d ", line.OldLineNum))
				} else if line.LineType == aligner.Modified {
					leftLineNumBlock = modifiedLineNumStyle.Render(fmt.Sprintf("%d ", line.OldLineNum))
				} else {
					leftLineNumBlock = lineNumStyle.Render(fmt.Sprintf("%d", line.OldLineNum)) + " "
				}
			} else {
				leftLineNumBlock = strings.Repeat(" ", lineNumWidth + changeMarkerWidth)
			}
			
			// Format right side
			if line.NewLine != nil {
				rightContent = " " + *line.NewLine
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