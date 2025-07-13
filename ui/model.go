package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"duckdiff/aligner"
	"duckdiff/parser"
)

type Model struct {
	fileDiffs []parser.FileDiff
	alignedLines []aligner.AlignedLine
	viewport viewport.Model
	ready bool
}

func NewModel(fileDiffs []parser.FileDiff, alignedLines []aligner.AlignedLine) Model {
	return Model{
		fileDiffs: fileDiffs,
		alignedLines: alignedLines,
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
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-1)
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 1
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
	
	leftColumnStyle := lipgloss.NewStyle().
		Width(60).
		Align(lipgloss.Left)
		
	rightColumnStyle := lipgloss.NewStyle().
		Width(60).
		Align(lipgloss.Left)
	
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	deletedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	
	for _, fileDiff := range m.fileDiffs {
		content.WriteString(lipgloss.NewStyle().Bold(true).Render("=== " + fileDiff.NewPath + " ==="))
		content.WriteString("\n")
		
		content.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftColumnStyle.Render("OLD"),
			" │ ",
			rightColumnStyle.Render("NEW"),
		))
		content.WriteString("\n")
		
		content.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top,
			strings.Repeat("─", 60),
			"─┼─",
			strings.Repeat("─", 60),
		))
		content.WriteString("\n")
		break
	}
	
	for _, line := range m.alignedLines {
		var leftContent, rightContent string
		
		if line.OldLine != nil {
			leftContent = *line.OldLine
			if line.LineType == aligner.Deleted {
				leftContent = deletedStyle.Render("-" + leftContent)
			} else {
				leftContent = " " + leftContent
			}
		}
		
		if line.NewLine != nil {
			rightContent = *line.NewLine
			if line.LineType == aligner.Added {
				rightContent = addedStyle.Render("+" + rightContent)
			} else {
				rightContent = " " + rightContent
			}
		}
		
		content.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftColumnStyle.Render(leftContent),
			" │ ",
			rightColumnStyle.Render(rightContent),
		))
		content.WriteString("\n")
	}
	
	return content.String()
}