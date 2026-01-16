package tui

import (
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func makeTestModel(numLines int) Model {
	// Create a single file with numLines pairs
	pairs := make([]sidebyside.LinePair, numLines)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "left"},
			Right: sidebyside.Line{Num: i + 1, Content: "right"},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
	})
	m.width = 80
	m.height = 20
	return m
}

func TestUpdate_ScrollDown(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := newM.(Model)

	assert.Equal(t, 1, model.scroll)
}

func TestUpdate_ScrollUp(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 10

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := newM.(Model)

	assert.Equal(t, 9, model.scroll)
}

func TestUpdate_ScrollUp_AtTop(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := newM.(Model)

	assert.Equal(t, 0, model.scroll) // can't go negative
}

func TestUpdate_ScrollDown_AtBottom(t *testing.T) {
	m := makeTestModel(30) // 30 pairs + 1 header = 31 lines
	m.scroll = 30          // 31 - 1 = 30 is max (allows last line at top of viewport)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := newM.(Model)

	assert.Equal(t, 30, model.scroll) // can't exceed max
}

func TestUpdate_PageDown(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown, Alt: false})
	model := newM.(Model)

	// Arrow down should also work
	assert.Equal(t, 1, model.scroll)

	// Page down
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model2 := newM2.(Model)

	assert.Equal(t, 21, model2.scroll) // 1 + 20
}

func TestUpdate_PageUp(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 40

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model := newM.(Model)

	assert.Equal(t, 20, model.scroll) // 40 - 20
}

func TestUpdate_PageDown_Space(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	model := newM.(Model)

	assert.Equal(t, 20, model.scroll) // 0 + height (20)
}

func TestUpdate_PageDown_F(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	model := newM.(Model)

	assert.Equal(t, 20, model.scroll) // 0 + height (20)
}

func TestUpdate_PageUp_B(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 40

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model := newM.(Model)

	assert.Equal(t, 20, model.scroll) // 40 - height (20)
}

func TestUpdate_HalfPageDown(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	// ctrl+d is represented as KeyCtrlD
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	model := newM.(Model)

	assert.Equal(t, 10, model.scroll) // height/2 = 20/2 = 10
}

func TestUpdate_GoToTop(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 50

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)

	assert.Equal(t, 0, model.scroll)
}

func TestUpdate_GoToBottom(t *testing.T) {
	m := makeTestModel(100) // 100 pairs + 1 header = 101 lines
	m.scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// max scroll = 101 - 1 = 100 (allows last line at top of viewport)
	assert.Equal(t, 100, model.scroll)
}

func TestUpdate_Quit(t *testing.T) {
	m := makeTestModel(10)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// cmd should be tea.Quit
	assert.NotNil(t, cmd)
}

func TestUpdate_WindowResize(t *testing.T) {
	m := makeTestModel(50)
	m.scroll = 40 // still valid after resize (max is 50)

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := newM.(Model)

	assert.Equal(t, 120, model.width)
	assert.Equal(t, 40, model.height)
	// scroll stays at 40 since max = 51 - 1 = 50
	assert.Equal(t, 40, model.scroll)
}

func TestUpdate_WindowResize_SmallContent(t *testing.T) {
	m := makeTestModel(10) // 11 lines total
	m.scroll = 5

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	model := newM.(Model)

	// scroll stays at 5, max = 11 - 1 = 10 (allows scrolling past content)
	assert.Equal(t, 5, model.scroll)
}

func TestUpdate_ScrollRight(t *testing.T) {
	m := makeTestModel(10)
	m.hscroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model := newM.(Model)

	assert.Equal(t, 4, model.hscroll) // default step is 4
}

func TestUpdate_ScrollLeft(t *testing.T) {
	m := makeTestModel(10)
	m.hscroll = 8

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	model := newM.(Model)

	assert.Equal(t, 4, model.hscroll) // 8 - 4 = 4
}

func TestUpdate_ScrollLeft_AtZero(t *testing.T) {
	m := makeTestModel(10)
	m.hscroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	model := newM.(Model)

	assert.Equal(t, 0, model.hscroll) // can't go negative
}

func TestUpdate_ScrollLeft_ClampToZero(t *testing.T) {
	m := makeTestModel(10)
	m.hscroll = 2 // less than step (4)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	model := newM.(Model)

	assert.Equal(t, 0, model.hscroll) // clamps to 0, not negative
}

func TestUpdate_ScrollRight_ArrowKey(t *testing.T) {
	m := makeTestModel(10)
	m.hscroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model := newM.(Model)

	assert.Equal(t, 4, model.hscroll)
}

func TestUpdate_ScrollLeft_ArrowKey(t *testing.T) {
	m := makeTestModel(10)
	m.hscroll = 8

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model := newM.(Model)

	assert.Equal(t, 4, model.hscroll)
}

func TestUpdate_ScrollPastEnd_ToShowLastFile(t *testing.T) {
	// Create two files with different amounts of content
	pairs1 := make([]sidebyside.LinePair, 5)
	pairs2 := make([]sidebyside.LinePair, 5)
	for i := range pairs1 {
		pairs1[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "file1"},
			Right: sidebyside.Line{Num: i + 1, Content: "file1"},
		}
	}
	for i := range pairs2 {
		pairs2[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "file2"},
			Right: sidebyside.Line{Num: i + 1, Content: "file2"},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs1},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs2},
	})
	m.width = 80
	m.height = 10 // viewport height

	// Total lines: 2 headers + 10 pairs = 12 lines
	// We should be able to scroll until the LAST line is at the TOP
	// That means max scroll should be totalLines - 1 = 11
	// (so line 12 is at the top of a 10-line viewport, with 9 empty lines below)

	// Go to bottom
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// With scrolling past end, max scroll should be totalLines - 1
	// This allows the last line to be at the top of the viewport
	assert.Equal(t, 11, model.scroll, "should scroll until last line is at top of viewport")

	// At this scroll position, the current file should be the second file
	info := model.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "should show second file when scrolled to its header")
	assert.Equal(t, "second.go", info.FileName)
}
