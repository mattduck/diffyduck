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

	// Scroll can now go negative to allow cursor to reach first line
	assert.Equal(t, -1, model.scroll)
}

func TestUpdate_ScrollDown_AtBottom(t *testing.T) {
	m := makeTestModel(30) // 30 pairs + 1 header = 31 lines
	// height=20, contentHeight=19, cursorOffset=3
	// maxScroll = 31 - 1 - 3 = 27
	m.scroll = m.maxScroll()

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := newM.(Model)

	assert.Equal(t, m.maxScroll(), model.scroll) // can't exceed max
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

	// g now goes to minScroll so cursor is at first line
	assert.Equal(t, m.minScroll(), model.scroll)
}

func TestUpdate_GoToBottom(t *testing.T) {
	m := makeTestModel(100) // 100 pairs + 1 header = 101 lines
	m.scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// G now goes to maxScroll so cursor is at last line
	// maxScroll = 101 - 1 - cursorOffset
	assert.Equal(t, m.maxScroll(), model.scroll)
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

func TestUpdate_FoldToggle_SingleFile(t *testing.T) {
	m := makeTestModel(10)
	// Initially at FoldNormal (zero value)
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)

	// Press Tab to cycle to next level
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Should advance to FoldExpanded
	assert.Equal(t, sidebyside.FoldExpanded, model.files[0].FoldLevel)

	// Press Tab again to cycle to Folded
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model2.files[0].FoldLevel)

	// Press Tab again to cycle back to Normal
	newM3, _ := model2.Update(tea.KeyMsg{Type: tea.KeyTab})
	model3 := newM3.(Model)

	assert.Equal(t, sidebyside.FoldNormal, model3.files[0].FoldLevel)
}

func TestUpdate_FoldToggleAll_AllSameLevel(t *testing.T) {
	// Create two files
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
	m.height = 20

	// Both at FoldNormal initially
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldNormal, m.files[1].FoldLevel)

	// Press Shift+Tab - both should advance to FoldExpanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, model.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldExpanded, model.files[1].FoldLevel)
}

func TestUpdate_FoldToggle_ReturnsCmd_WhenExpanding(t *testing.T) {
	// When expanding to FoldExpanded and content not loaded, should return a fetch command
	m := makeTestModel(10)
	// Set up a mock fetcher (nil fetcher means no command returned)
	// Since we don't have a fetcher, the command will be nil
	// but the level should still change

	// Initially at FoldNormal
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)

	// Press Tab to advance to FoldExpanded
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, model.files[0].FoldLevel)
	// Without a fetcher, cmd should be nil
	assert.Nil(t, cmd, "without fetcher, cmd should be nil")
}

func TestUpdate_FoldToggle_SkipsFetch_WhenContentLoaded(t *testing.T) {
	// When content is already loaded, should not return a fetch command
	m := makeTestModel(10)
	m.files[0].OldContent = []string{"already", "loaded"}
	m.files[0].NewContent = []string{"already", "loaded"}

	// Press Tab to advance to FoldExpanded
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Content already loaded, so no fetch needed
	assert.Nil(t, cmd)
}

func TestUpdate_FoldToggleAll_DifferentLevels(t *testing.T) {
	// Create two files at different fold levels
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
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs1, FoldLevel: sidebyside.FoldNormal},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs2, FoldLevel: sidebyside.FoldExpanded},
	})
	m.width = 80
	m.height = 20

	// Files at different levels
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldExpanded, m.files[1].FoldLevel)

	// Press Shift+Tab - all should collapse to FoldFolded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldFolded, model.files[1].FoldLevel)
}

func TestUpdate_FileContentLoadedMsg(t *testing.T) {
	m := makeTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldExpanded

	// Simulate receiving content loaded message
	msg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: []string{"old line 1", "old line 2"},
		NewContent: []string{"new line 1", "new line 2", "new line 3"},
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	assert.Equal(t, []string{"old line 1", "old line 2"}, model.files[0].OldContent)
	assert.Equal(t, []string{"new line 1", "new line 2", "new line 3"}, model.files[0].NewContent)
}

func TestUpdate_AllContentLoadedMsg(t *testing.T) {
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "line"},
			Right: sidebyside.Line{Num: i + 1, Content: "line"},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs, FoldLevel: sidebyside.FoldExpanded},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs, FoldLevel: sidebyside.FoldExpanded},
	})
	m.width = 80
	m.height = 20

	// Simulate receiving all content loaded message
	msg := AllContentLoadedMsg{
		Contents: []FileContent{
			{FileIndex: 0, OldContent: []string{"file1 old"}, NewContent: []string{"file1 new"}},
			{FileIndex: 1, OldContent: []string{"file2 old"}, NewContent: []string{"file2 new"}},
		},
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	assert.Equal(t, []string{"file1 old"}, model.files[0].OldContent)
	assert.Equal(t, []string{"file1 new"}, model.files[0].NewContent)
	assert.Equal(t, []string{"file2 old"}, model.files[1].OldContent)
	assert.Equal(t, []string{"file2 new"}, model.files[1].NewContent)
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

	// Total lines: 2 headers + 10 pairs + 1 blank line before second file = 13 lines
	// With cursor-based scrolling:
	// - height=10, contentHeight=9, cursorOffset=1
	// - maxScroll = 13 - 1 - 1 = 11 (cursor at line 12, last content line)

	// Go to bottom
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// maxScroll allows cursor to reach last line
	assert.Equal(t, m.maxScroll(), model.scroll, "should scroll to maxScroll")

	// At this scroll position, the cursor is on the last line (second file's last pair)
	info := model.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "should show second file when cursor is at end")
	assert.Equal(t, "second.go", info.FileName)
}
