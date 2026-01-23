package tui

import (
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func makeTestModel(numLines int) Model {
	// Create a single file with numLines pairs
	pairs := make([]sidebyside.LinePair, numLines)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "left"},
			New: sidebyside.Line{Num: i + 1, Content: "right"},
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

func TestUpdate_GoToTop_gg(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 50

	// First 'g' puts us in pending state
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)

	// Should not have moved yet
	assert.Equal(t, 50, model.scroll, "first g should not scroll")
	assert.Equal(t, "g", model.pendingKey, "should be in pending state")

	// Second 'g' completes the sequence
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model2 := newM2.(Model)

	assert.Equal(t, m.minScroll(), model2.scroll, "gg should go to top")
	assert.Equal(t, "", model2.pendingKey, "pending state should be cleared")
}

func TestUpdate_PendingKey_CancelledByUnknown(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 50

	// Press 'g' to enter pending state
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	assert.Equal(t, "g", model.pendingKey)

	// Press unknown key 'x' - should cancel pending state without action
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model2 := newM2.(Model)

	assert.Equal(t, 50, model2.scroll, "scroll should not change")
	assert.Equal(t, "", model2.pendingKey, "pending state should be cleared")
}

func makeMultiFileTestModel() Model {
	// Create 3 files with 5 lines each
	makePairs := func(n int) []sidebyside.LinePair {
		pairs := make([]sidebyside.LinePair, n)
		for i := range pairs {
			pairs[i] = sidebyside.LinePair{
				Old: sidebyside.Line{Num: i + 1, Content: "content"},
				New: sidebyside.Line{Num: i + 1, Content: "content"},
			}
		}
		return pairs
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: makePairs(5)},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: makePairs(5)},
		{OldPath: "a/third.go", NewPath: "b/third.go", Pairs: makePairs(5)},
	})
	m.width = 80
	m.height = 40           // tall enough to see all content
	m.initialFoldSet = true // prevent WindowSizeMsg from changing fold levels
	return m
}

func sendKeys(m Model, keys ...string) Model {
	for _, k := range keys {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		m = newM.(Model)
	}
	return m
}

func TestUpdate_NextHeading_gj(t *testing.T) {
	m := makeMultiFileTestModel()
	// Start at top (first file header)
	m = sendKeys(m, "g", "g")
	assert.Equal(t, m.minScroll(), m.scroll, "should be at top")

	// Get cursor position before - should be on first file header
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "should start at first file")

	// gj should move to next file header
	m = sendKeys(m, "g", "j")
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "gj should move to second file")

	// gj again should move to third file
	m = sendKeys(m, "g", "j")
	info = m.StatusInfo()
	assert.Equal(t, 3, info.CurrentFile, "gj should move to third file")

	// gj at last file should stay (no more files to go to)
	m = sendKeys(m, "g", "j")
	info = m.StatusInfo()
	assert.Equal(t, 3, info.CurrentFile, "gj at last file should stay")
}

func TestUpdate_PrevHeading_gk(t *testing.T) {
	m := makeMultiFileTestModel()
	// Go to bottom (last file)
	m = sendKeys(m, "G")
	info := m.StatusInfo()
	assert.Equal(t, 3, info.CurrentFile, "G should go to last file")

	// gk from last file content should go to last file's header
	m = sendKeys(m, "g", "k")
	info = m.StatusInfo()
	assert.Equal(t, 3, info.CurrentFile, "gk should stay on third file (go to header)")

	// gk from header should move to previous file header
	m = sendKeys(m, "g", "k")
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "gk should move to second file")

	// gk again should move to first file
	m = sendKeys(m, "g", "k")
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "gk should move to first file")

	// gk at first file should stay there
	m = sendKeys(m, "g", "k")
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "gk at first file should stay")
}

func TestUpdate_PrevHeading_gk_FromMiddleOfFile(t *testing.T) {
	m := makeMultiFileTestModel()
	// Go to top (first file header)
	m = sendKeys(m, "g", "g")

	// Move to second file header
	m = sendKeys(m, "g", "j")
	info := m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "should be on second file")

	// Move down a few lines into the middle of the file content
	m = sendKeys(m, "j", "j", "j")
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "should still be in second file")

	// Verify we're NOT on the header
	rows := m.buildRows()
	cursorPos := m.cursorLine()
	assert.False(t, rows[cursorPos].isHeader, "should not be on header after moving down")

	// gk should first jump to the CURRENT file's header (not the previous file)
	m = sendKeys(m, "g", "k")
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "gk from middle of file should stay in same file")

	// Verify we're now on the header
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isHeader, "gk from middle should jump to current file's header")

	// Now gk again should go to the previous file
	m = sendKeys(m, "g", "k")
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "gk from header should go to previous file")
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
	m.scroll = 40

	// Cursor row before resize
	cursorRowBefore := m.cursorLine()

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := newM.(Model)

	assert.Equal(t, 120, model.width)
	assert.Equal(t, 40, model.height)
	// Cursor should stay on same row after resize
	assert.Equal(t, cursorRowBefore, model.cursorLine())
}

func TestUpdate_WindowResize_SmallContent(t *testing.T) {
	m := makeTestModel(10) // 11 lines total
	m.scroll = 5

	// Cursor row before resize
	cursorRowBefore := m.cursorLine()

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	model := newM.(Model)

	// Cursor should stay on same row after resize
	assert.Equal(t, cursorRowBefore, model.cursorLine())
}

func TestUpdate_WindowResize_PreservesCursorOnInterFileBlank(t *testing.T) {
	m := makeMultiFileTestModel()

	// Find inter-file blank rows (between files)
	rows := m.buildRows()
	var blankIndices []int
	for i, row := range rows {
		if row.isBlank && !row.isHeaderSpacer {
			blankIndices = append(blankIndices, i)
		}
	}
	require.NotEmpty(t, blankIndices, "should have inter-file blank rows")

	// Test with the last blank row (not the first one)
	lastBlankIdx := blankIndices[len(blankIndices)-1]
	m.adjustScrollToRow(lastBlankIdx)
	require.Equal(t, lastBlankIdx, m.cursorLine(), "cursor should be on blank row")

	// Resize terminal
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := newM.(Model)

	// Cursor should stay on the same blank row after resize
	assert.Equal(t, lastBlankIdx, model.cursorLine(), "cursor should stay on same blank row after resize")
}

func TestUpdate_WindowResize_PreservesCursorOnHeaderSpacer(t *testing.T) {
	m := makeMultiFileTestModel()

	// Find header spacer rows (blank line after header, before content)
	rows := m.buildRows()
	var spacerIndices []int
	for i, row := range rows {
		if row.isHeaderSpacer {
			spacerIndices = append(spacerIndices, i)
		}
	}
	require.NotEmpty(t, spacerIndices, "should have header spacer rows")

	// Test with the second file's header spacer (not the first one)
	require.True(t, len(spacerIndices) >= 2, "should have at least 2 header spacers")
	spacerIdx := spacerIndices[1]
	m.adjustScrollToRow(spacerIdx)
	require.Equal(t, spacerIdx, m.cursorLine(), "cursor should be on header spacer")

	// Resize terminal
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := newM.(Model)

	// Cursor should stay on the same header spacer after resize
	assert.Equal(t, spacerIdx, model.cursorLine(), "cursor should stay on same header spacer after resize")
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
			Old: sidebyside.Line{Num: i + 1, Content: "file1"},
			New: sidebyside.Line{Num: i + 1, Content: "file1"},
		}
	}
	for i := range pairs2 {
		pairs2[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "file2"},
			New: sidebyside.Line{Num: i + 1, Content: "file2"},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs1},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs2},
	})
	m.width = 80
	m.height = 20

	// Both files at FoldNormal, commit at CommitNormal = level 3
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldNormal, m.files[1].FoldLevel)
	assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel)

	// Press Shift+Tab - should cycle from level 3 to level 1 (all folded)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldFolded, model.files[1].FoldLevel)
	assert.Equal(t, sidebyside.CommitFolded, model.commits[0].FoldLevel)
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
			Old: sidebyside.Line{Num: i + 1, Content: "file1"},
			New: sidebyside.Line{Num: i + 1, Content: "file1"},
		}
	}
	for i := range pairs2 {
		pairs2[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "file2"},
			New: sidebyside.Line{Num: i + 1, Content: "file2"},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs1, FoldLevel: sidebyside.FoldNormal},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs2, FoldLevel: sidebyside.FoldExpanded},
	})
	m.width = 80
	m.height = 20

	// Files at different levels = mixed state
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldExpanded, m.files[1].FoldLevel)

	// Press Shift+Tab - mixed state resets to level 1 (all folded)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldFolded, model.files[1].FoldLevel)
	assert.Equal(t, sidebyside.CommitFolded, model.commits[0].FoldLevel)
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

func TestUpdate_FileContentLoadedMsg_OldTruncatedOnly(t *testing.T) {
	m := makeTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldExpanded

	// Simulate receiving content where only old side was truncated
	msg := FileContentLoadedMsg{
		FileIndex:    0,
		OldContent:   []string{"old line 1"},
		NewContent:   []string{"new line 1"},
		OldTruncated: true,
		NewTruncated: false,
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	assert.True(t, model.files[0].OldContentTruncated, "OldContentTruncated should be true")
	assert.False(t, model.files[0].NewContentTruncated, "NewContentTruncated should be false")
}

func TestUpdate_FileContentLoadedMsg_NewTruncatedOnly(t *testing.T) {
	m := makeTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldExpanded

	// Simulate receiving content where only new side was truncated
	msg := FileContentLoadedMsg{
		FileIndex:    0,
		OldContent:   []string{"old line 1"},
		NewContent:   []string{"new line 1"},
		OldTruncated: false,
		NewTruncated: true,
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	assert.False(t, model.files[0].OldContentTruncated, "OldContentTruncated should be false")
	assert.True(t, model.files[0].NewContentTruncated, "NewContentTruncated should be true")
}

func TestUpdate_FileContentLoadedMsg_BothTruncated(t *testing.T) {
	m := makeTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldExpanded

	// Simulate receiving content where both sides were truncated
	msg := FileContentLoadedMsg{
		FileIndex:    0,
		OldContent:   []string{"old line 1"},
		NewContent:   []string{"new line 1"},
		OldTruncated: true,
		NewTruncated: true,
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	assert.True(t, model.files[0].OldContentTruncated, "OldContentTruncated should be true")
	assert.True(t, model.files[0].NewContentTruncated, "NewContentTruncated should be true")
}

func TestUpdate_FileContentLoadedMsg_NeitherTruncated(t *testing.T) {
	m := makeTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldExpanded

	// Simulate receiving content where neither side was truncated
	msg := FileContentLoadedMsg{
		FileIndex:    0,
		OldContent:   []string{"old line 1"},
		NewContent:   []string{"new line 1"},
		OldTruncated: false,
		NewTruncated: false,
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	assert.False(t, model.files[0].OldContentTruncated, "OldContentTruncated should be false")
	assert.False(t, model.files[0].NewContentTruncated, "NewContentTruncated should be false")
}

func TestUpdate_AllContentLoadedMsg(t *testing.T) {
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "line"},
			New: sidebyside.Line{Num: i + 1, Content: "line"},
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

func TestUpdate_AllContentLoadedMsg_PerSideTruncation(t *testing.T) {
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "line"},
			New: sidebyside.Line{Num: i + 1, Content: "line"},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs, FoldLevel: sidebyside.FoldExpanded},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs, FoldLevel: sidebyside.FoldExpanded},
	})
	m.width = 80
	m.height = 20

	// Simulate: first file has only old truncated, second file has only new truncated
	msg := AllContentLoadedMsg{
		Contents: []FileContent{
			{FileIndex: 0, OldContent: []string{"file1 old"}, NewContent: []string{"file1 new"}, OldTruncated: true, NewTruncated: false},
			{FileIndex: 1, OldContent: []string{"file2 old"}, NewContent: []string{"file2 new"}, OldTruncated: false, NewTruncated: true},
		},
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	// First file: old truncated only
	assert.True(t, model.files[0].OldContentTruncated, "file 0 OldContentTruncated should be true")
	assert.False(t, model.files[0].NewContentTruncated, "file 0 NewContentTruncated should be false")

	// Second file: new truncated only
	assert.False(t, model.files[1].OldContentTruncated, "file 1 OldContentTruncated should be false")
	assert.True(t, model.files[1].NewContentTruncated, "file 1 NewContentTruncated should be true")
}

// Pager mode tests - fold toggle should skip FoldExpanded

func TestUpdate_PagerMode_FoldToggle_SkipsExpanded(t *testing.T) {
	m := makeTestModel(10)
	m.pagerMode = true // Enable pager mode

	// Initially at FoldNormal
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)

	// Press Tab - should skip FoldExpanded and go to FoldFolded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel,
		"pager mode should skip FoldExpanded")

	// Press Tab again - should go back to FoldNormal
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)

	assert.Equal(t, sidebyside.FoldNormal, model2.files[0].FoldLevel,
		"pager mode should cycle back to FoldNormal")
}

func TestUpdate_PagerMode_FoldToggleAll_SkipsExpanded(t *testing.T) {
	pairs1 := make([]sidebyside.LinePair, 5)
	pairs2 := make([]sidebyside.LinePair, 5)
	for i := range pairs1 {
		pairs1[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "file1"},
			New: sidebyside.Line{Num: i + 1, Content: "file1"},
		}
	}
	for i := range pairs2 {
		pairs2[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "file2"},
			New: sidebyside.Line{Num: i + 1, Content: "file2"},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs1},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs2},
	}, WithPagerMode())
	m.width = 80
	m.height = 20

	// Both at FoldNormal initially
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldNormal, m.files[1].FoldLevel)

	// Press Shift+Tab - should skip FoldExpanded and go to FoldFolded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel,
		"pager mode should skip FoldExpanded for all files")
	assert.Equal(t, sidebyside.FoldFolded, model.files[1].FoldLevel,
		"pager mode should skip FoldExpanded for all files")
}

func TestUpdate_PagerMode_NormalMode_DoesNotSkipExpanded(t *testing.T) {
	// Verify normal (non-pager) mode still goes through FoldExpanded
	m := makeTestModel(10)
	// pagerMode is false by default

	// Initially at FoldNormal
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel)

	// Press Tab - should go to FoldExpanded in normal mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, model.files[0].FoldLevel,
		"normal mode should go to FoldExpanded")
}

func TestNextFoldLevel_PagerMode(t *testing.T) {
	m := Model{pagerMode: true}

	// Normal -> Folded (skip Expanded)
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevel(sidebyside.FoldNormal))

	// Folded -> Normal
	assert.Equal(t, sidebyside.FoldNormal, m.nextFoldLevel(sidebyside.FoldFolded))

	// Expanded -> Folded (same as normal, this level shouldn't be reached in pager mode)
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevel(sidebyside.FoldExpanded))
}

func TestNextFoldLevel_NormalMode(t *testing.T) {
	m := Model{pagerMode: false}

	// Normal -> Expanded
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevel(sidebyside.FoldNormal))

	// Expanded -> Folded
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevel(sidebyside.FoldExpanded))

	// Folded -> Normal
	assert.Equal(t, sidebyside.FoldNormal, m.nextFoldLevel(sidebyside.FoldFolded))
}

func TestNextFoldLevelForFile_BinaryFile(t *testing.T) {
	m := Model{pagerMode: false}

	// Binary file should skip FoldExpanded, same as pager mode
	binaryFile := sidebyside.FilePair{
		OldPath:   "a/image.png",
		NewPath:   "b/image.png",
		IsBinary:  true,
		FoldLevel: sidebyside.FoldNormal,
	}

	// Normal -> Folded (skip Expanded)
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevelForFile(binaryFile))

	// Folded -> Normal
	binaryFile.FoldLevel = sidebyside.FoldFolded
	assert.Equal(t, sidebyside.FoldNormal, m.nextFoldLevelForFile(binaryFile))
}

func TestNextFoldLevelForFile_NonBinaryFile(t *testing.T) {
	m := Model{pagerMode: false}

	// Non-binary file should go through all levels
	normalFile := sidebyside.FilePair{
		OldPath:   "a/foo.go",
		NewPath:   "b/foo.go",
		IsBinary:  false,
		FoldLevel: sidebyside.FoldNormal,
	}

	// Normal -> Expanded
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevelForFile(normalFile))

	// Expanded -> Folded
	normalFile.FoldLevel = sidebyside.FoldExpanded
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevelForFile(normalFile))

	// Folded -> Normal
	normalFile.FoldLevel = sidebyside.FoldFolded
	assert.Equal(t, sidebyside.FoldNormal, m.nextFoldLevelForFile(normalFile))
}

func TestFetchFileContent_SkipsBinaryFiles(t *testing.T) {
	// Use a real fetcher (with nil git, which won't be called)
	fetcher := &content.Fetcher{}
	m := Model{
		fetcher: fetcher,
		files: []sidebyside.FilePair{
			{
				OldPath:  "/dev/null",
				NewPath:  "b/image.png",
				IsBinary: true,
			},
		},
	}

	// Should return nil for binary files (no fetch attempt)
	cmd := m.FetchFileContent(0)
	assert.Nil(t, cmd, "FetchFileContent should return nil for binary files")
}
