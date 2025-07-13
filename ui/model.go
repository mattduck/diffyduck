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

type Model struct {
	filesWithLines []FileWithLines
	viewport viewport.Model
	ready bool
	width int
}

func NewModel(filesWithLines []FileWithLines) Model {
	return Model{
		filesWithLines: filesWithLines,
	}
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
			m.viewport.LineDown(1)
		case "k", "up":
			m.viewport.LineUp(1)
		case "d":
			m.viewport.HalfViewDown()
		case "u":
			m.viewport.HalfViewUp()
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

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
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
			leftColumnStyle.Render("OLD"),
			" │ ",
			strings.Repeat(" ", lineNumWidth + changeMarkerWidth),
			" │ ",
			rightColumnStyle.Render("NEW"),
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
		for _, line := range fileWithLines.AlignedLines {
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
				if line.LineType == aligner.Added {
					rightLineNumBlock = addedLineNumStyle.Render(fmt.Sprintf("%d ", line.NewLineNum))
				} else if line.LineType == aligner.Modified {
					rightLineNumBlock = modifiedLineNumStyle.Render(fmt.Sprintf("%d ", line.NewLineNum))
				} else {
					rightLineNumBlock = lineNumStyle.Render(fmt.Sprintf("%d", line.NewLineNum)) + " "
				}
			} else {
				rightLineNumBlock = strings.Repeat(" ", lineNumWidth + changeMarkerWidth)
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