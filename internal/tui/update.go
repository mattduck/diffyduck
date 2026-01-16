package tui

import (
	"github.com/charmbracelet/bubbletea"
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
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keys := m.keys

	switch {
	case matchesKey(msg, keys.Quit):
		return m, tea.Quit

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
	}

	return m, nil
}
