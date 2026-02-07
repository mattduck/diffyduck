package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
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
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
	})
	m.width = 80
	m.height = 20
	return m
}

func TestUpdate_ScrollDown(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := newM.(Model)

	assert.Equal(t, 1, model.w().scroll)
}

func TestUpdate_ScrollUp(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := newM.(Model)

	assert.Equal(t, 9, model.w().scroll)
}

func TestUpdate_ScrollUp_AtTop(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := newM.(Model)

	// Scroll clamps at 0 (no negative scroll in new model)
	assert.Equal(t, 0, model.w().scroll)
}

func TestUpdate_ScrollDown_AtBottom(t *testing.T) {
	m := makeTestModel(30) // 30 pairs + 1 header = 31 lines
	// height=20, contentHeight=19, cursorOffset=3
	// maxScroll = 31 - 1 - 3 = 27
	m.w().scroll = m.maxScroll()

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := newM.(Model)

	assert.Equal(t, m.maxScroll(), model.w().scroll) // can't exceed max
}

func TestUpdate_PageDown(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown, Alt: false})
	model := newM.(Model)

	// Arrow down should also work
	assert.Equal(t, 1, model.w().scroll)

	// Page down
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model2 := newM2.(Model)

	assert.Equal(t, 21, model2.w().scroll) // 1 + 20
}

func TestUpdate_PageUp(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 40

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model := newM.(Model)

	assert.Equal(t, 20, model.w().scroll) // 40 - 20
}

func TestUpdate_PageDown_Space(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	model := newM.(Model)

	assert.Equal(t, 20, model.w().scroll) // 0 + height (20)
}

func TestUpdate_PageDown_F(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	model := newM.(Model)

	assert.Equal(t, 20, model.w().scroll) // 0 + height (20)
}

func TestUpdate_PageUp_B(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 40

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model := newM.(Model)

	assert.Equal(t, 20, model.w().scroll) // 40 - height (20)
}

func TestUpdate_HalfPageDown(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	// ctrl+d is represented as KeyCtrlD
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	model := newM.(Model)

	assert.Equal(t, 10, model.w().scroll) // height/2 = 20/2 = 10
}

func TestUpdate_GoToTop_gg(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 50

	// First 'g' puts us in pending state
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)

	// Should not have moved yet
	assert.Equal(t, 50, model.w().scroll, "first g should not scroll")
	assert.Equal(t, "g", model.pendingKey, "should be in pending state")

	// Second 'g' completes the sequence
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model2 := newM2.(Model)

	assert.Equal(t, m.minScroll(), model2.w().scroll, "gg should go to top")
	assert.Equal(t, "", model2.pendingKey, "pending state should be cleared")
}

func TestUpdate_PendingKey_CancelledByUnknown(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 50

	// Press 'g' to enter pending state
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	assert.Equal(t, "g", model.pendingKey)

	// Press unknown key 'x' - should cancel pending state without action
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model2 := newM2.(Model)

	assert.Equal(t, 50, model2.w().scroll, "scroll should not change")
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
		{OldPath: "a/first.go", NewPath: "b/first.go", FoldLevel: sidebyside.FoldExpanded, Pairs: makePairs(5)},
		{OldPath: "a/second.go", NewPath: "b/second.go", FoldLevel: sidebyside.FoldExpanded, Pairs: makePairs(5)},
		{OldPath: "a/third.go", NewPath: "b/third.go", FoldLevel: sidebyside.FoldExpanded, Pairs: makePairs(5)},
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
	// Start at top - gg puts cursor on file 0's header (first row in diff view)
	m = sendKeys(m, "g", "g")
	assert.Equal(t, m.minScroll(), m.w().scroll, "should be at top")

	// After gg, cursor is on first file's header (in diff view, there's no top border row)
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "should start at first file")

	// Verify we're on file 0's header
	rows := m.buildRows()
	cursorPos := m.cursorLine()
	assert.True(t, rows[cursorPos].isHeader, "should be on header")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "should be on file 0")

	// gj from file 0's header should move to file 1's header
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
	// Go to top (first file's header in diff view)
	m = sendKeys(m, "g", "g")
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "should be on first file header")

	// Move to second file header
	m = sendKeys(m, "g", "j")
	info = m.StatusInfo()
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

func makeMultiCommitTestModel() Model {
	// Create 2 commits with 2 files each
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

	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123",
				Author:  "Author",
				Subject: "First commit",
			},
			FoldLevel:   sidebyside.CommitNormal, // Expanded to show files
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/file1.go", NewPath: "b/file1.go", Pairs: makePairs(3), FoldLevel: sidebyside.FoldExpanded},
				{OldPath: "a/file2.go", NewPath: "b/file2.go", Pairs: makePairs(3), FoldLevel: sidebyside.FoldExpanded},
			},
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "def456",
				Author:  "Author",
				Subject: "Second commit",
			},
			FoldLevel:   sidebyside.CommitNormal, // Expanded to show files
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/file3.go", NewPath: "b/file3.go", Pairs: makePairs(3), FoldLevel: sidebyside.FoldExpanded},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 60 // tall enough to see all content
	m.initialFoldSet = true
	m.calculateTotalLines()
	return m
}

func TestUpdate_NextHeading_gj_MultiCommit(t *testing.T) {
	m := makeMultiCommitTestModel()

	// Go to top
	m = sendKeys(m, "g", "g")

	// Should start at commit 0 header
	rows := m.buildRows()
	cursorPos := m.cursorLine()
	require.True(t, rows[cursorPos].isCommitHeader, "should start on commit header")
	assert.Equal(t, 0, rows[cursorPos].commitIndex, "should be on commit 0")

	// gj should go to commit info header
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isCommitInfoHeader, "gj from commit header should go to commit info header")
	assert.Equal(t, 0, rows[cursorPos].commitIndex, "should be on commit 0's info header")

	// gj should go to first file header
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isHeader, "gj from commit info header should go to file header")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "should be on file 0")

	// gj should go to second file header
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isHeader, "gj should go to next file header")
	assert.Equal(t, 1, rows[cursorPos].fileIndex, "should be on file 1")

	// gj from last file of commit 0 should go to commit 1 header
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isCommitHeader, "gj from last file should go to next commit header")
	assert.Equal(t, 1, rows[cursorPos].commitIndex, "should be on commit 1")

	// gj should go to commit 1's info header
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isCommitInfoHeader, "gj should go to commit 1's info header")
	assert.Equal(t, 1, rows[cursorPos].commitIndex, "should be on commit 1's info header")

	// gj should go to commit 1's first file
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isHeader, "gj should go to commit 1's file header")
	assert.Equal(t, 2, rows[cursorPos].fileIndex, "should be on file 2 (commit 1's first file)")
}

func TestUpdate_PrevHeading_gk_MultiCommit(t *testing.T) {
	m := makeMultiCommitTestModel()

	// Go to bottom (should be on commit 1's file content)
	m = sendKeys(m, "G")

	// gk should go to commit 1's file header
	m = sendKeys(m, "g", "k")
	rows := m.buildRows()
	cursorPos := m.cursorLine()
	require.True(t, rows[cursorPos].isHeader, "gk should go to file header")
	assert.Equal(t, 2, rows[cursorPos].fileIndex, "should be on file 2")

	// gk from file header should go to commit 1's info header
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isCommitInfoHeader, "gk should go to commit info header")
	assert.Equal(t, 1, rows[cursorPos].commitIndex, "should be on commit 1's info header")

	// gk should go to commit 1 header
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isCommitHeader, "gk should go to commit header")
	assert.Equal(t, 1, rows[cursorPos].commitIndex, "should be on commit 1 header")

	// gk should go to commit 0's last file header
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isHeader, "gk should go to previous file header")
	assert.Equal(t, 1, rows[cursorPos].fileIndex, "should be on file 1 (commit 0's last file)")

	// gk should go to commit 0's first file header
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isHeader, "gk should go to previous file header")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "should be on file 0")

	// gk should go to commit 0's info header
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isCommitInfoHeader, "gk should go to commit info header")
	assert.Equal(t, 0, rows[cursorPos].commitIndex, "should be on commit 0's info header")

	// gk should go to commit 0 header
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	require.True(t, rows[cursorPos].isCommitHeader, "gk should go to commit header")
	assert.Equal(t, 0, rows[cursorPos].commitIndex, "should be on commit 0 header")
}

func TestUpdate_GoToBottom(t *testing.T) {
	m := makeTestModel(100) // 100 pairs + 1 header = 101 lines
	m.w().scroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// G now goes to maxScroll so cursor is at last line
	// maxScroll = 101 - 1 - cursorOffset
	assert.Equal(t, m.maxScroll(), model.w().scroll)
}

func TestUpdate_Quit(t *testing.T) {
	m := makeTestModel(10)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// cmd should be tea.Quit
	assert.NotNil(t, cmd)
}

func TestUpdate_WindowResize(t *testing.T) {
	m := makeTestModel(50)
	m.w().scroll = 40

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
	m.w().scroll = 5

	// Cursor row before resize
	cursorRowBefore := m.cursorLine()

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	model := newM.(Model)

	// Cursor should stay on same row after resize
	assert.Equal(t, cursorRowBefore, model.cursorLine())
}

func TestUpdate_WindowResize_BottomMarginBlanks(t *testing.T) {
	// Each unfolded file should have exactly one blank margin row after its content
	m := makeMultiFileTestModel()

	rows := m.buildRows()
	var marginBlanks []int
	for i, row := range rows {
		if row.isBlank && !row.isHeaderSpacer {
			marginBlanks = append(marginBlanks, i)
		}
	}
	// Count unfolded files (FoldNormal has content)
	unfoldedCount := 0
	for _, fp := range m.files {
		if fp.FoldLevel != sidebyside.FoldFolded {
			unfoldedCount++
		}
	}
	assert.Equal(t, unfoldedCount, len(marginBlanks), "each unfolded file should have one bottom margin blank row")
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
	m.w().hscroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model := newM.(Model)

	assert.Equal(t, 4, model.w().hscroll) // default step is 4
}

func TestUpdate_ScrollLeft(t *testing.T) {
	m := makeTestModel(10)
	m.w().hscroll = 8

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	model := newM.(Model)

	assert.Equal(t, 4, model.w().hscroll) // 8 - 4 = 4
}

func TestUpdate_ScrollLeft_AtZero(t *testing.T) {
	m := makeTestModel(10)
	m.w().hscroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	model := newM.(Model)

	assert.Equal(t, 0, model.w().hscroll) // can't go negative
}

func TestUpdate_ScrollLeft_ClampToZero(t *testing.T) {
	m := makeTestModel(10)
	m.w().hscroll = 2 // less than step (4)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	model := newM.(Model)

	assert.Equal(t, 0, model.w().hscroll) // clamps to 0, not negative
}

func TestUpdate_ScrollRight_ArrowKey(t *testing.T) {
	m := makeTestModel(10)
	m.w().hscroll = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model := newM.(Model)

	assert.Equal(t, 4, model.w().hscroll)
}

func TestUpdate_ScrollLeft_ArrowKey(t *testing.T) {
	m := makeTestModel(10)
	m.w().hscroll = 8

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model := newM.(Model)

	assert.Equal(t, 4, model.w().hscroll)
}

func TestUpdate_MouseWheelDown(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	model := newM.(Model)

	assert.Equal(t, 3, model.w().scroll) // scrolls 3 lines
}

func TestUpdate_MouseWheelUp(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	model := newM.(Model)

	assert.Equal(t, 7, model.w().scroll) // 10 - 3
}

func TestUpdate_MouseWheelUp_AtTop(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	model := newM.(Model)

	// Scroll clamps at 0 (no negative scroll in new model)
	assert.Equal(t, 0, model.w().scroll)
}

func TestUpdate_MouseWheelDown_AtBottom(t *testing.T) {
	m := makeTestModel(30)
	m.w().scroll = m.maxScroll()

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	model := newM.(Model)

	assert.Equal(t, m.maxScroll(), model.w().scroll) // clamped to max
}

func TestUpdate_MouseEvent_SetsFocused(t *testing.T) {
	// When clicking into a tmux pane, tmux may not send a FocusMsg but will
	// pass through the mouse event. Any mouse activity implies focus.
	m := makeTestModel(100)
	m.focused = false

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft})
	model := newM.(Model)

	assert.True(t, model.focused, "mouse event should set focused=true")
}

func TestUpdate_FoldToggle_SingleFile(t *testing.T) {
	m := makeTestModel(10)
	// Initially at FoldExpanded (set by makeTestModel)
	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(0))

	// Position cursor on file header (line 0 in diff view: no top border)
	m.w().scroll = 0

	// Press Tab to cycle to next level: Expanded -> Folded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.fileFoldLevel(0))

	// Press Tab again to cycle to Normal (structural diff)
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)

	assert.Equal(t, sidebyside.FoldNormal, model2.fileFoldLevel(0))

	// Press Tab again to cycle back to Expanded (hunks)
	newM3, _ := model2.Update(tea.KeyMsg{Type: tea.KeyTab})
	model3 := newM3.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, model3.fileFoldLevel(0))
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
	assert.Equal(t, sidebyside.FoldNormal, m.fileFoldLevel(0))
	assert.Equal(t, sidebyside.FoldNormal, m.fileFoldLevel(1))
	assert.Equal(t, sidebyside.CommitNormal, m.commitFoldLevel(0))

	// Press Shift+Tab - should cycle from level 3 to level 1 (all folded)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.fileFoldLevel(0))
	assert.Equal(t, sidebyside.FoldFolded, model.fileFoldLevel(1))
	assert.Equal(t, sidebyside.CommitFolded, model.commitFoldLevel(0))
}

func TestUpdate_FoldToggle_ReturnsCmd_WhenExpanding(t *testing.T) {
	// When expanding from FoldNormal to FoldExpanded, should return a fetch command
	m := makeTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldNormal // start at structural diff
	m.calculateTotalLines()

	// Position cursor on file header (line 0 in diff view)
	m.w().scroll = 0

	assert.Equal(t, sidebyside.FoldNormal, m.fileFoldLevel(0))

	// Press Tab to advance to FoldExpanded
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, model.fileFoldLevel(0))
	// Without a fetcher, cmd should be nil
	assert.Nil(t, cmd, "without fetcher, cmd should be nil")
}

func TestUpdate_FoldToggle_SkipsFetch_WhenContentLoaded(t *testing.T) {
	// When content is already loaded, should not return a fetch command
	m := makeTestModel(10)
	m.files[0].OldContent = []string{"already", "loaded"}
	m.files[0].NewContent = []string{"already", "loaded"}

	// Position cursor on file header (line 0 in diff view)
	m.w().scroll = 0

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
	assert.Equal(t, sidebyside.FoldNormal, m.fileFoldLevel(0))
	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(1))

	// Press Shift+Tab - mixed state resets to level 1 (all folded)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.fileFoldLevel(0))
	assert.Equal(t, sidebyside.FoldFolded, model.fileFoldLevel(1))
	assert.Equal(t, sidebyside.CommitFolded, model.commitFoldLevel(0))
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

func TestNextFoldLevel(t *testing.T) {
	m := Model{}

	// Normal -> Expanded
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevel(sidebyside.FoldNormal))

	// Expanded -> Folded
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevel(sidebyside.FoldExpanded))

	// Folded -> Normal
	assert.Equal(t, sidebyside.FoldNormal, m.nextFoldLevel(sidebyside.FoldFolded))
}

func TestNextFoldLevelForFile_BinaryFile(t *testing.T) {
	m := Model{}

	// Binary file should skip FoldNormal (no structural diff for binary files)
	binaryFile := sidebyside.FilePair{
		OldPath:  "a/image.png",
		NewPath:  "b/image.png",
		IsBinary: true,
	}

	// Expanded -> Folded
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevelForFile(sidebyside.FoldExpanded, binaryFile))

	// Folded -> Expanded (skip Normal)
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevelForFile(sidebyside.FoldFolded, binaryFile))
}

func TestNextFoldLevelForFile_NonBinaryFile(t *testing.T) {
	m := Model{}

	// Non-binary file should go through all levels
	normalFile := sidebyside.FilePair{
		OldPath:  "a/foo.go",
		NewPath:  "b/foo.go",
		IsBinary: false,
	}

	// Normal -> Expanded
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevelForFile(sidebyside.FoldNormal, normalFile))

	// Expanded -> Folded
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevelForFile(sidebyside.FoldExpanded, normalFile))

	// Folded -> Normal
	assert.Equal(t, sidebyside.FoldNormal, m.nextFoldLevelForFile(sidebyside.FoldFolded, normalFile))
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

// findTopBorderRowForFile finds the top border row that visually belongs to the given file.
// This is the row immediately BEFORE the file's header row.
// Returns the row index or -1 if not found.
func findTopBorderRowForFile(rows []displayRow, fileIndex int) int {
	// Find the file's header row
	for i, row := range rows {
		if row.isHeader && row.fileIndex == fileIndex {
			// The top border is the row immediately before the header
			if i > 0 && rows[i-1].isHeaderTopBorder {
				return i - 1
			}
		}
	}
	return -1
}

// moveCursorToRow adjusts scroll so that the cursor lands on the given row index.
func moveCursorToRow(m Model, rowIdx int) Model {
	// cursor is at scroll + cursorOffset(), so scroll = rowIdx - cursorOffset()
	m.w().scroll = rowIdx
	return m
}

func TestUpdate_gk_FromTopBorder_GoesToNodeAbove(t *testing.T) {
	// Test: gk on top border line goes up to the node (file/commit) above
	m := makeMultiFileTestModel()

	// Build rows to find the top border of file 1 (second file)
	rows := m.buildRows()
	file1TopBorder := findTopBorderRowForFile(rows, 1)
	require.NotEqual(t, -1, file1TopBorder, "should find top border for file 1")

	// Position cursor on file 1's top border
	m = moveCursorToRow(m, file1TopBorder)
	cursorPos := m.cursorLine()
	assert.Equal(t, file1TopBorder, cursorPos, "cursor should be on file 1's top border")
	assert.True(t, rows[cursorPos].isHeaderTopBorder, "cursor row should be a top border")

	// gk from top border should go to the node above (file 0's header)
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isHeader, "gk from top border should go to a header")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "gk from file 1's top border should go to file 0's header")
}

func TestUpdate_gj_FromTopBorder_GoesToFileHeaderBelow(t *testing.T) {
	// Test: gj on top border line goes down to the filename line (visually below)
	m := makeMultiFileTestModel()

	// Build rows to find the top border of file 1 (second file)
	rows := m.buildRows()
	file1TopBorder := findTopBorderRowForFile(rows, 1)
	require.NotEqual(t, -1, file1TopBorder, "should find top border for file 1")

	// Position cursor on file 1's top border
	m = moveCursorToRow(m, file1TopBorder)
	cursorPos := m.cursorLine()
	assert.Equal(t, file1TopBorder, cursorPos, "cursor should be on file 1's top border")

	// gj from top border should go to the file header (visually one line below)
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isHeader, "gj from top border should go to a header")
	assert.Equal(t, 1, rows[cursorPos].fileIndex, "gj from file 1's top border should go to file 1's header")
}

func TestUpdate_TopBarShowsPreviousFile_WhenOnTopBorder(t *testing.T) {
	// Test: when on top border line, the top-bar should show the previous file
	// (not the next file whose border it is)
	m := makeMultiFileTestModel()

	// Build rows to find the top border of file 1 (second file)
	rows := m.buildRows()
	file1TopBorder := findTopBorderRowForFile(rows, 1)
	require.NotEqual(t, -1, file1TopBorder, "should find top border for file 1")

	// Position cursor on file 1's top border
	m = moveCursorToRow(m, file1TopBorder)

	// StatusInfo should show file 0 (the previous file)
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "top-bar should show previous file when cursor is on top border")
	assert.Contains(t, info.FileName, "first", "should show the first file's name")
}

func TestUpdate_gk_FromFirstHeader_StaysOnFirstFile(t *testing.T) {
	// Test: gk from the very first file's header (file 0) should stay there
	// In diff view, file 0's header is the first row (no top border in buildRows)
	m := makeMultiFileTestModel()

	// Go to top - puts cursor on file 0's header in diff view
	m = sendKeys(m, "g", "g")
	cursorPos := m.cursorLine()
	rows := m.buildRows()
	assert.True(t, rows[cursorPos].isHeader, "cursor should be on file 0's header")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "should be file 0")

	// gk should stay on file 0's area (no previous file to go to)
	m = sendKeys(m, "g", "k")
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "gk from first file's header should stay on first file")
}

// =============================================================================
// Hunk Separator Navigation Tests (gj/gk)
// =============================================================================

// makeMultiHunkTestModel creates a model with 2 files where the first file has
// 3 hunks (two hunk boundaries) and a trailing separator, and the second file
// has a single hunk. This exercises gj/gk stopping on hunk separators.
func makeMultiHunkTestModel() Model {
	// File 0: 3 hunks with gaps in line numbers to create hunk boundaries.
	//   Hunk 1: lines 1-3
	//   Hunk 2: lines 10-12 (gap from 3→10)
	//   Hunk 3: lines 20-22 (gap from 12→20)
	// NewContent has 30 lines so there's a trailing separator after hunk 3.
	file0Pairs := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "b"}},
		{Old: sidebyside.Line{Num: 2, Content: "a"}, New: sidebyside.Line{Num: 2, Content: "b"}},
		{Old: sidebyside.Line{Num: 3, Content: "a"}, New: sidebyside.Line{Num: 3, Content: "b"}},
		// gap: 3→10
		{Old: sidebyside.Line{Num: 10, Content: "a"}, New: sidebyside.Line{Num: 10, Content: "b"}},
		{Old: sidebyside.Line{Num: 11, Content: "a"}, New: sidebyside.Line{Num: 11, Content: "b"}},
		{Old: sidebyside.Line{Num: 12, Content: "a"}, New: sidebyside.Line{Num: 12, Content: "b"}},
		// gap: 12→20
		{Old: sidebyside.Line{Num: 20, Content: "a"}, New: sidebyside.Line{Num: 20, Content: "b"}},
		{Old: sidebyside.Line{Num: 21, Content: "a"}, New: sidebyside.Line{Num: 21, Content: "b"}},
		{Old: sidebyside.Line{Num: 22, Content: "a"}, New: sidebyside.Line{Num: 22, Content: "b"}},
	}
	newContent := make([]string, 30)
	for i := range newContent {
		newContent[i] = "line"
	}

	// File 1: single contiguous hunk, no separators.
	file1Pairs := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 1, Content: "x"}, New: sidebyside.Line{Num: 1, Content: "y"}},
		{Old: sidebyside.Line{Num: 2, Content: "x"}, New: sidebyside.Line{Num: 2, Content: "y"}},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/multi.go", NewPath: "b/multi.go", FoldLevel: sidebyside.FoldExpanded, Pairs: file0Pairs, NewContent: newContent},
		{OldPath: "a/single.go", NewPath: "b/single.go", FoldLevel: sidebyside.FoldExpanded, Pairs: file1Pairs},
	})
	m.width = 80
	m.height = 80 // tall enough to see all content
	m.initialFoldSet = true
	return m
}

// findNthSeparatorForFile returns the row index of the nth (0-based) RowKindSeparator
// row belonging to the given fileIndex, or -1 if not found.
func findNthSeparatorForFile(rows []displayRow, fileIndex, n int) int {
	count := 0
	for i, row := range rows {
		if row.isSeparator && row.fileIndex == fileIndex {
			if count == n {
				return i
			}
			count++
		}
	}
	return -1
}

// findTrailingSeparatorTopForFile returns the row index of the trailing
// RowKindSeparatorTop (the one NOT followed by a RowKindSeparator) for the
// given fileIndex, or -1 if not found.
func findTrailingSeparatorTopForFile(rows []displayRow, fileIndex int) int {
	for i, row := range rows {
		if row.isSeparatorTop && row.fileIndex == fileIndex {
			if i+1 >= len(rows) || !rows[i+1].isSeparator {
				return i
			}
		}
	}
	return -1
}

func TestUpdate_gj_StopsOnHunkSeparators(t *testing.T) {
	m := makeMultiHunkTestModel()
	rows := m.buildRows()

	// Verify setup: file 0 should have hunk separators
	sep0 := findNthSeparatorForFile(rows, 0, 0)
	sep1 := findNthSeparatorForFile(rows, 0, 1)
	trailing := findTrailingSeparatorTopForFile(rows, 0)
	require.NotEqual(t, -1, sep0, "file 0 should have a first hunk separator")
	require.NotEqual(t, -1, sep1, "file 0 should have a second hunk separator")
	require.NotEqual(t, -1, trailing, "file 0 should have a trailing separator")

	// Go to top (file 0's header)
	m = sendKeys(m, "g", "g")
	cursorPos := m.cursorLine()
	require.True(t, rows[cursorPos].isHeader, "should start on file 0 header")
	require.Equal(t, 0, rows[cursorPos].fileIndex)

	// gj → first hunk separator (between hunk 1 and 2)
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isSeparator, "gj from header should land on first hunk separator")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "should still be in file 0")

	// gj → second hunk separator (between hunk 2 and 3)
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isSeparator, "gj should land on second hunk separator")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "should still be in file 0")

	// gj → trailing separator top (end of file 0)
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isSeparatorTop, "gj should land on trailing separator")
	assert.Equal(t, 0, rows[cursorPos].fileIndex, "should still be in file 0")

	// gj → file 1's header (next file)
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isHeader, "gj should land on file 1 header")
	assert.Equal(t, 1, rows[cursorPos].fileIndex, "should be on file 1")
}

func TestUpdate_gk_StopsOnHunkSeparators(t *testing.T) {
	m := makeMultiHunkTestModel()
	rows := m.buildRows()

	// Find file 1's header and position cursor there
	file1Header := -1
	for i, row := range rows {
		if row.isHeader && row.fileIndex == 1 {
			file1Header = i
			break
		}
	}
	require.NotEqual(t, -1, file1Header)
	m = moveCursorToRow(m, file1Header)

	// gk from file 1 header → trailing separator of file 0
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos := m.cursorLine()
	assert.True(t, rows[cursorPos].isSeparatorTop, "gk from file 1 header should land on file 0's trailing separator")
	assert.Equal(t, 0, rows[cursorPos].fileIndex)

	// gk → second hunk separator of file 0
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isSeparator, "gk should land on second hunk separator")
	assert.Equal(t, 0, rows[cursorPos].fileIndex)

	// gk → first hunk separator of file 0
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isSeparator, "gk should land on first hunk separator")
	assert.Equal(t, 0, rows[cursorPos].fileIndex)

	// gk → file 0's header
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isHeader, "gk should land on file 0 header")
	assert.Equal(t, 0, rows[cursorPos].fileIndex)
}

func TestUpdate_gk_FromContentBetweenHunks_GoesToNearestSeparator(t *testing.T) {
	m := makeMultiHunkTestModel()
	rows := m.buildRows()

	// Find the second hunk separator in file 0
	sep1 := findNthSeparatorForFile(rows, 0, 1)
	require.NotEqual(t, -1, sep1)

	// Position cursor a few lines after the second separator (in hunk 3 content)
	contentRow := sep1 + 3 // skip past SeparatorBottom + into content
	m = moveCursorToRow(m, contentRow)
	cursorPos := m.cursorLine()
	rows = m.buildRows()
	require.False(t, rows[cursorPos].isSeparator, "should be on content, not separator")

	// gk should go to the second hunk separator (nearest target above)
	m = sendKeys(m, "g", "k")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isSeparator, "gk from content should go to nearest separator above")
	assert.Equal(t, sep1, cursorPos, "should land on the second separator")
}

func TestUpdate_gj_SkipsHunksInSingleHunkFile(t *testing.T) {
	// File with no hunk boundaries should have gj go straight from header to next file
	makePairs := func(n int) []sidebyside.LinePair {
		pairs := make([]sidebyside.LinePair, n)
		for i := range pairs {
			pairs[i] = sidebyside.LinePair{
				Old: sidebyside.Line{Num: i + 1, Content: "a"},
				New: sidebyside.Line{Num: i + 1, Content: "b"},
			}
		}
		return pairs
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/one.go", NewPath: "b/one.go", FoldLevel: sidebyside.FoldExpanded, Pairs: makePairs(5)},
		{OldPath: "a/two.go", NewPath: "b/two.go", FoldLevel: sidebyside.FoldExpanded, Pairs: makePairs(5)},
	})
	m.width = 80
	m.height = 60
	m.initialFoldSet = true

	// Go to top
	m = sendKeys(m, "g", "g")
	rows := m.buildRows()
	cursorPos := m.cursorLine()
	require.True(t, rows[cursorPos].isHeader)
	require.Equal(t, 0, rows[cursorPos].fileIndex)

	// gj should go straight to file 1's header (no separators to stop on)
	m = sendKeys(m, "g", "j")
	rows = m.buildRows()
	cursorPos = m.cursorLine()
	assert.True(t, rows[cursorPos].isHeader, "gj should go to next file header")
	assert.Equal(t, 1, rows[cursorPos].fileIndex, "should be on file 1")
}

// =============================================================================
// Cursor Identity Tests - Multi-Commit Support
// =============================================================================

// createTwoCommitModelForIdentityTests creates a model with two commits for testing
// cursor identity preservation across content loading.
func createTwoCommitModelForIdentityTests() Model {
	commit1 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "aaa1111",
			Author:  "Author One",
			Subject: "First commit subject",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/file1.go",
				NewPath:   "b/file1.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "old1"}, New: sidebyside.Line{Num: 1, Content: "new1"}},
				},
			},
		},
		FoldLevel:   sidebyside.CommitFolded,
		FilesLoaded: true,
	}
	commit2 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "bbb2222",
			Author:  "Author Two",
			Subject: "Second commit subject",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/file2.go",
				NewPath:   "b/file2.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "old2"}, New: sidebyside.Line{Num: 1, Content: "new2"}},
				},
			},
		},
		FoldLevel:   sidebyside.CommitFolded,
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit1, commit2})
	m.width = 80
	m.height = 40
	m.focused = true
	m.calculateTotalLines()
	return m
}

func TestCursorIdentity_CommitIndex_CapturedCorrectly(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()

	// Both commits folded -> 2 rows (commit headers)
	rows := m.buildRows()
	require.Equal(t, 2, len(rows), "should have 2 rows when both commits folded")
	require.True(t, rows[0].isCommitHeader, "row 0 should be commit header")
	require.True(t, rows[1].isCommitHeader, "row 1 should be commit header")

	// Position cursor on second commit header (row 1)
	m.w().scroll = 1
	m.calculateTotalLines()

	identity := m.getCursorRowIdentity()
	assert.Equal(t, RowKindCommitHeader, identity.kind, "identity kind should be CommitHeader")
	assert.Equal(t, 1, identity.commitIndex, "identity should capture commit index 1")
	assert.Equal(t, -1, identity.fileIndex, "commit rows have fileIndex -1")
}

func TestRowMatchesIdentity_CommitHeader_RequiresMatchingCommitIndex(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()
	rows := m.buildRows()

	// Create identity for commit 1's header
	identity := cursorRowIdentity{
		kind:        RowKindCommitHeader,
		fileIndex:   -1,
		commitIndex: 1,
	}

	// Row 0 is commit 0's header - should NOT match
	assert.False(t, m.rowMatchesIdentity(rows[0], identity, 0, 0, 0, 0),
		"commit 0's header should not match identity for commit 1")

	// Row 1 is commit 1's header - SHOULD match
	assert.True(t, m.rowMatchesIdentity(rows[1], identity, 0, 0, 0, 0),
		"commit 1's header should match identity for commit 1")
}

func TestFindRowOrNearestAbove_CommitRow_FindsCorrectCommit(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()

	// Identity for second commit header
	identity := cursorRowIdentity{
		kind:        RowKindCommitHeader,
		fileIndex:   -1,
		commitIndex: 1,
	}

	// Should find row 1 (second commit header), not row 0
	rowIdx := m.findRowOrNearestAbove(identity)
	assert.Equal(t, 1, rowIdx, "should find row 1 for commit 1's header")

	rows := m.buildRows()
	assert.Equal(t, 1, rows[rowIdx].commitIndex, "found row should be commit 1")
}

func TestFindRowOrNearestAbove_CommitRow_FallbackToCommitHeader(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()

	// Expand second commit to get more row types
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	// Create identity for a row type that might not exist after rebuild
	// (e.g., CommitHeaderBottomBorder for commit 1)
	identity := cursorRowIdentity{
		kind:        RowKindCommitHeaderBottomBorder,
		fileIndex:   -1,
		commitIndex: 1,
	}

	rowIdx := m.findRowOrNearestAbove(identity)
	rows := m.buildRows()

	// Should find either the exact row or fall back to commit 1's header
	assert.True(t, rows[rowIdx].commitIndex == 1 || rowIdx == 0,
		"should find a row in commit 1 or fall back gracefully")
}

func TestFileContentLoaded_SkipsScrollPreservation_WhenCommitFolded(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()

	// Both commits are folded
	require.Equal(t, sidebyside.CommitFolded, m.commitFoldLevel(0))
	require.Equal(t, sidebyside.CommitFolded, m.commitFoldLevel(1))

	// Position cursor on second commit header
	m.w().scroll = 1
	m.calculateTotalLines()
	initialScroll := m.w().scroll

	// Simulate file content loading for file in first (folded) commit
	msg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: []string{"line1", "line2"},
		NewContent: []string{"line1", "line2"},
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	// Scroll should be unchanged because commit was folded
	// (no scroll preservation needed, content doesn't affect visible rows)
	assert.Equal(t, initialScroll, model.w().scroll,
		"scroll should be unchanged when loading content for folded commit")
}

func TestFileContentLoaded_SkipsScrollPreservation_WhenFileNotExpanded(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()

	// Expand first commit but keep file folded
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.files[0].FoldLevel = sidebyside.FoldFolded // file is folded, not expanded
	m.calculateTotalLines()

	// Position cursor somewhere
	m.w().scroll = 0
	initialScroll := m.w().scroll

	// Simulate file content loading
	msg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: []string{"line1", "line2", "line3"},
		NewContent: []string{"line1", "line2", "line3"},
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	// Scroll should be unchanged because file is not in FoldExpanded mode
	assert.Equal(t, initialScroll, model.w().scroll,
		"scroll should be unchanged when loading content for non-expanded file")
}

func TestFileContentLoaded_PreservesScroll_WhenFileExpanded(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()

	// Expand first commit and its file to FoldExpanded
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.files[0].FoldLevel = sidebyside.FoldExpanded
	m.calculateTotalLines()

	// Get initial cursor identity
	initialIdentity := m.getCursorRowIdentity()

	// Simulate file content loading - this SHOULD trigger scroll preservation
	msg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: []string{"line1", "line2", "line3", "line4", "line5"},
		NewContent: []string{"line1", "line2", "line3", "line4", "line5"},
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	// After content loads, cursor should still point to same logical row
	newIdentity := model.getCursorRowIdentity()
	assert.Equal(t, initialIdentity.kind, newIdentity.kind,
		"cursor should be on same kind of row after content load")
}

// makeHunkedTestModel creates a file with 3 hunks at specific source line numbers:
// - Hunk 1: lines 10-15 (6 lines)
// - Hunk 2: lines 25-30 (6 lines) - 10 lines gap from hunk 1, within threshold
// - Hunk 3: lines 100-105 (6 lines) - 70 lines gap from hunk 2, outside threshold
func makeHunkedTestModel() Model {
	// Build pairs with gaps to create hunks
	var pairs []sidebyside.LinePair

	// Hunk 1: lines 10-15
	for i := 10; i <= 15; i++ {
		pairs = append(pairs, sidebyside.LinePair{
			Old: sidebyside.Line{Num: i, Content: "hunk1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: i, Content: "hunk1", Type: sidebyside.Context},
		})
	}

	// Hunk 2: lines 25-30 (10 lines gap = within 15 line threshold)
	for i := 25; i <= 30; i++ {
		pairs = append(pairs, sidebyside.LinePair{
			Old: sidebyside.Line{Num: i, Content: "hunk2", Type: sidebyside.Context},
			New: sidebyside.Line{Num: i, Content: "hunk2", Type: sidebyside.Context},
		})
	}

	// Hunk 3: lines 100-105 (70 lines gap = outside 15 line threshold)
	for i := 100; i <= 105; i++ {
		pairs = append(pairs, sidebyside.LinePair{
			Old: sidebyside.Line{Num: i, Content: "hunk3", Type: sidebyside.Context},
			New: sidebyside.Line{Num: i, Content: "hunk3", Type: sidebyside.Context},
		})
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
	})
	m.width = 100
	m.height = 40
	m.focusColour = true
	return m
}

func TestFocusProximity_NearbyHunksIncluded(t *testing.T) {
	m := makeHunkedTestModel()
	m.rebuildRowsCache()

	rows := m.w().cachedRows

	// Find a content row in hunk 1 (lines 10-15)
	var hunk1ContentIdx int
	for i, row := range rows {
		if row.kind == RowKindContent && row.pair.New.Num >= 10 && row.pair.New.Num <= 15 {
			hunk1ContentIdx = i
			break
		}
	}
	require.NotZero(t, hunk1ContentIdx, "should find hunk 1 content")

	// Position cursor on hunk 1
	m.w().scroll = hunk1ContentIdx
	m.clampScroll()

	predicate := m.getFocusPredicate()
	require.NotNil(t, predicate, "focus predicate should be active when on content")

	// Count which hunks are in focus
	var hunk1InFocus, hunk2InFocus, hunk3InFocus bool
	for i, row := range rows {
		if row.kind != RowKindContent {
			continue
		}
		inFocus := predicate(i, row)
		lineNum := row.pair.New.Num
		if lineNum >= 10 && lineNum <= 15 {
			if inFocus {
				hunk1InFocus = true
			}
		} else if lineNum >= 25 && lineNum <= 30 {
			if inFocus {
				hunk2InFocus = true
			}
		} else if lineNum >= 100 && lineNum <= 105 {
			if inFocus {
				hunk3InFocus = true
			}
		}
	}

	assert.True(t, hunk1InFocus, "hunk 1 (cursor hunk) should be in focus")
	assert.True(t, hunk2InFocus, "hunk 2 (10 lines away, within threshold) should be in focus")
	assert.False(t, hunk3InFocus, "hunk 3 (70 lines away, outside threshold) should NOT be in focus")
}

func TestFullFileToggle_Basic(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
					},
				},
				OldContent: []string{"line1", "line2"},
				NewContent: []string{"line1", "line2"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  20,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	assert.False(t, m.files[0].ShowFullFile)

	// Simulate Shift+F
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	assert.True(t, m.files[0].ShowFullFile, "ShowFullFile should be true after toggle")
	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(0), "FoldLevel should remain FoldExpanded")

	// Toggle off
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)
	assert.False(t, m.files[0].ShowFullFile, "ShowFullFile should be false after second toggle")
}

func TestFullFileToggle_ExpandsFromFolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
					},
				},
				OldContent: []string{"line1"},
				NewContent: []string{"line1"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  20,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(0), "should auto-expand to FoldExpanded")
	assert.True(t, m.files[0].ShowFullFile, "ShowFullFile should be true")
}

func TestFullFileToggle_TabClearsShowFullFile(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				ShowFullFile: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Tab cycles away from FoldExpanded to FoldFolded
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.False(t, m.files[0].ShowFullFile, "Tab should clear ShowFullFile when cycling away from FoldExpanded")
}

func TestFullFileToggle_SeparatorTop_GoesToLineAbove(t *testing.T) {
	// File with two hunks: lines 1-3 and lines 10-12 (gap creates separator)
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 3, Content: "c", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "c", Type: sidebyside.Context}},
					// Gap: lines 4-9 are missing → hunk separator here
					{Old: sidebyside.Line{Num: 10, Content: "j", Type: sidebyside.Context}, New: sidebyside.Line{Num: 10, Content: "j", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 11, Content: "k", Type: sidebyside.Context}, New: sidebyside.Line{Num: 11, Content: "k", Type: sidebyside.Context}},
				},
				OldContent: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
				NewContent: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the SeparatorTop row
	rows := m.buildRows()
	sepTopIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			sepTopIdx = i
			break
		}
	}
	require.True(t, sepTopIdx >= 0, "should have a SeparatorTop row")

	// Position cursor on separator top
	m.w().scroll = sepTopIdx

	// Toggle full-file view
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	// Cursor should be on new-side line 3 (last content line above separator)
	newRows := m.buildRows()
	cursorRow := newRows[m.w().scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should be on a content row")
	assert.Equal(t, 3, cursorRow.pair.New.Num, "cursor should be on line 3 (last line above separator)")
}

func TestFullFileToggle_SeparatorBottom_GoesToLineBelow(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}},
					// Gap: lines 3-9 missing → separator
					{Old: sidebyside.Line{Num: 10, Content: "j", Type: sidebyside.Context}, New: sidebyside.Line{Num: 10, Content: "j", Type: sidebyside.Context}},
				},
				OldContent: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				NewContent: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the SeparatorBottom row
	rows := m.buildRows()
	sepBottomIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorBottom && row.fileIndex == 0 {
			sepBottomIdx = i
			break
		}
	}
	require.True(t, sepBottomIdx >= 0, "should have a SeparatorBottom row")

	m.w().scroll = sepBottomIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	newRows := m.buildRows()
	cursorRow := newRows[m.w().scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should be on a content row")
	assert.Equal(t, 10, cursorRow.pair.New.Num, "cursor should be on line 10 (first line below separator)")
}

func TestFullFileToggle_SeparatorMiddle_WithBreadcrumb(t *testing.T) {
	// Middle separator with a breadcrumb should navigate to the innermost
	// structure entry's StartLine.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}},
					// Gap → separator. Next chunk starts at line 20, which is inside a func starting at line 15.
					{Old: sidebyside.Line{Num: 20, Content: "t", Type: sidebyside.Context}, New: sidebyside.Line{Num: 20, Content: "t", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 21, Content: "u", Type: sidebyside.Context}, New: sidebyside.Line{Num: 21, Content: "u", Type: sidebyside.Context}},
				},
				OldContent: make([]string, 25),
				NewContent: make([]string, 25),
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  40,
		keys:    DefaultKeyMap(),
		// Provide structure so the breadcrumb resolves
		structureMaps: map[int]*FileStructure{
			0: {
				NewStructure: &structure.Map{
					Entries: []structure.Entry{
						{StartLine: 15, EndLine: 25, Name: "myFunc", Kind: "func"},
					},
				},
			},
		},
	}
	// Fill content so HasContent() is true
	for i := range m.files[0].OldContent {
		m.files[0].OldContent[i] = string(rune('a' + i%26))
	}
	for i := range m.files[0].NewContent {
		m.files[0].NewContent[i] = string(rune('a' + i%26))
	}
	m.calculateTotalLines()

	// Find the middle Separator row
	rows := m.buildRows()
	sepIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparator && row.fileIndex == 0 {
			sepIdx = i
			break
		}
	}
	require.True(t, sepIdx >= 0, "should have a Separator row")

	m.w().scroll = sepIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	newRows := m.buildRows()
	cursorRow := newRows[m.w().scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should be on a content row")
	assert.Equal(t, 15, cursorRow.pair.New.Num, "cursor should land on line 15 (function start from breadcrumb)")
}

func TestFullFileToggle_SeparatorMiddle_NoBreadcrumb(t *testing.T) {
	// Middle separator without structure data should go to the first content
	// line below the separator.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Context}},
					// Gap → separator
					{Old: sidebyside.Line{Num: 10, Content: "j", Type: sidebyside.Context}, New: sidebyside.Line{Num: 10, Content: "j", Type: sidebyside.Context}},
				},
				OldContent: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				NewContent: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the middle Separator row
	rows := m.buildRows()
	sepIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparator && row.fileIndex == 0 {
			sepIdx = i
			break
		}
	}
	require.True(t, sepIdx >= 0, "should have a Separator row")

	m.w().scroll = sepIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	newRows := m.buildRows()
	cursorRow := newRows[m.w().scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should be on a content row")
	assert.Equal(t, 10, cursorRow.pair.New.Num, "cursor should land on line 10 (first content below, no breadcrumb)")
}

func TestFullFileToggle_OffFromFullFile_PreservesPosition(t *testing.T) {
	// Toggling full-file view OFF should use identity-based scroll and land
	// back on the same content line in hunk view.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				ShowFullFile: true,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "a", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "b", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 2, Content: "B", Type: sidebyside.Added}},
					{Old: sidebyside.Line{Num: 3, Content: "c", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "c", Type: sidebyside.Context}},
				},
				OldContent: []string{"a", "b", "c", "d", "e"},
				NewContent: []string{"a", "B", "c", "d", "e"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Position cursor on line 3 (a content row present in both views)
	rows := m.buildRows()
	for i, row := range rows {
		if row.kind == RowKindContent && row.fileIndex == 0 && row.pair.New.Num == 3 {
			m.w().scroll = i
			break
		}
	}

	// Toggle OFF full-file view
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	assert.False(t, m.files[0].ShowFullFile)
	newRows := m.buildRows()
	cursorRow := newRows[m.w().scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should still be on a content row")
	assert.Equal(t, 3, cursorRow.pair.New.Num, "cursor should stay on line 3 after toggling off")
}

func TestFullFileToggle_NarrowsWhenNotNarrowed(t *testing.T) {
	// With multiple files, pressing F should also narrow to the current file
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context}},
				},
				OldContent: []string{"line1", "line2"},
				NewContent: []string{"line1", "line2"},
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "other1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "other1", Type: sidebyside.Context}},
				},
				OldContent: []string{"other1"},
				NewContent: []string{"other1"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   120,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify we start without narrow
	assert.False(t, m.w().narrow.Active)

	// Both files should be visible before F
	fullRows := m.buildRows()
	hasSecondFile := false
	for _, row := range fullRows {
		if row.fileIndex == 1 {
			hasSecondFile = true
			break
		}
	}
	assert.True(t, hasSecondFile, "second file should be visible before F")

	// Press F
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	assert.True(t, m.files[0].ShowFullFile, "ShowFullFile should be true")
	assert.True(t, m.w().narrow.Active, "narrow mode should be active after F")
	assert.Equal(t, 0, m.w().narrow.FileIdx, "should be narrowed to file 0")

	// Only file 0 rows should be visible
	narrowRows := m.buildRows()
	for i, row := range narrowRows {
		if row.fileIndex >= 0 {
			assert.Equal(t, 0, row.fileIndex, "row %d should belong to file 0 in narrowed view", i)
		}
	}
}

func TestFullFileToggle_DoesNotRenarrowWhenAlreadyNarrowed(t *testing.T) {
	// If already narrowed to a file, F should not change the narrow scope
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context}},
				},
				OldContent: []string{"line1"},
				NewContent: []string{"line1"},
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "other1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "other1", Type: sidebyside.Context}},
				},
				OldContent: []string{"other1"},
				NewContent: []string{"other1"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   120,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Pre-set narrow to file 1 (second file), then move cursor there
	m.w().narrow = NarrowScope{
		Active:    true,
		CommitIdx: -1,
		FileIdx:   1,
		HunkIdx:   -1,
	}
	m.rebuildRowsCache()

	// Press F — should toggle full-file for file 1 without changing narrow scope
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	assert.True(t, m.files[1].ShowFullFile, "ShowFullFile should be true for file 1")
	// Narrow scope should be unchanged
	assert.True(t, m.w().narrow.Active, "narrow should still be active")
	assert.Equal(t, 1, m.w().narrow.FileIdx, "narrow file scope should be unchanged")
}

func TestFullFileToggle_OffDoesNotClearNarrow(t *testing.T) {
	// Toggling F off should leave narrow mode active
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context}},
				},
				OldContent: []string{"line1", "line2"},
				NewContent: []string{"line1", "line2"},
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "other1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "other1", Type: sidebyside.Context}},
				},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   120,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Press F to enable (should also narrow)
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)
	assert.True(t, m.w().narrow.Active, "should be narrowed after F on")

	// Press F again to disable full-file view
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	assert.False(t, m.files[0].ShowFullFile, "ShowFullFile should be off")
	assert.True(t, m.w().narrow.Active, "narrow should persist after F off")
	assert.Equal(t, 0, m.w().narrow.FileIdx, "narrow file scope should persist")
}

// --- Enter on hunk separator: context expansion tests ---

func makeEnterTestModel() Model {
	// Two hunks with a large gap: lines 1-3 and lines 25-27, in a 30-line file.
	return Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
					// gap: lines 4-24
					{Old: sidebyside.Line{Num: 25, Content: "25", Type: sidebyside.Context}, New: sidebyside.Line{Num: 25, Content: "25", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 26, Content: "26", Type: sidebyside.Context}, New: sidebyside.Line{Num: 26, Content: "26", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 27, Content: "27", Type: sidebyside.Context}, New: sidebyside.Line{Num: 27, Content: "27", Type: sidebyside.Context}},
				},
				OldContent: makeTestContent(30),
				NewContent: makeTestContent(30),
			},
		},
		width:  80,
		height: 40,
		keys:   DefaultKeyMap(),
	}
}

func makeTestContent(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%d", i+1)
	}
	return lines
}

func TestTab_SeparatorTop_ExpandsDown(t *testing.T) {
	m := makeEnterTestModel()
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	// Find SeparatorTop
	rows := m.buildRows()
	sepTopIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			sepTopIdx = i
			break
		}
	}
	require.True(t, sepTopIdx >= 0)
	m.w().scroll = sepTopIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.Greater(t, len(m.files[0].Pairs), originalPairsLen, "Pairs should grow after expansion")
	// Should have added 15 context lines after hunk 0 (lines 4-18)
	assert.Equal(t, originalPairsLen+15, len(m.files[0].Pairs))
	// Verify inserted line numbers
	assert.Equal(t, 4, m.files[0].Pairs[3].New.Num, "first inserted should be line 4")
	assert.Equal(t, 18, m.files[0].Pairs[17].New.Num, "last inserted should be line 18")
	// Cursor should land on first inserted line (line 4, just below where we clicked)
	cursorRow := m.getRows()[m.cursorLine()]
	assert.Equal(t, 4, cursorRow.pair.New.Num, "cursor should be on first inserted line")
}

func TestTab_SeparatorBottom_ExpandsUp(t *testing.T) {
	m := makeEnterTestModel()
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	// Find SeparatorBottom
	rows := m.buildRows()
	sepBotIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorBottom && row.fileIndex == 0 {
			sepBotIdx = i
			break
		}
	}
	require.True(t, sepBotIdx >= 0)
	m.w().scroll = sepBotIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.Equal(t, originalPairsLen+15, len(m.files[0].Pairs))
	// Should have prepended 15 lines before hunk 1 (lines 10-24)
	// Hunk 1 originally started at Pairs index 3. Now context starts at index 3.
	assert.Equal(t, 10, m.files[0].Pairs[3].New.Num, "first inserted should be line 10")
	assert.Equal(t, 24, m.files[0].Pairs[17].New.Num, "last inserted should be line 24")
	// Cursor should land on last inserted line (line 24, just above hunk at 25)
	cursorRow := m.getRows()[m.cursorLine()]
	assert.Equal(t, 24, cursorRow.pair.New.Num, "cursor should be on last inserted line")
}

func TestTab_SeparatorMiddle_WithBreadcrumb(t *testing.T) {
	m := makeEnterTestModel()
	// Add structure: a function starting at line 20 that contains the hunk at line 25
	m.structureMaps = map[int]*FileStructure{
		0: {
			NewStructure: &structure.Map{
				Entries: []structure.Entry{
					{StartLine: 20, EndLine: 30, Name: "myFunc", Kind: "func"},
				},
			},
		},
	}
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	// Find Separator (middle)
	rows := m.buildRows()
	sepIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparator && row.fileIndex == 0 {
			sepIdx = i
			break
		}
	}
	require.True(t, sepIdx >= 0)
	m.w().scroll = sepIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	// Should expand to signature start (20) minus 2 = line 18, through line 24
	expectedInserted := 24 - 18 + 1 // lines 18-24 = 7 lines
	assert.Equal(t, originalPairsLen+expectedInserted, len(m.files[0].Pairs))
	assert.Equal(t, 18, m.files[0].Pairs[3].New.Num, "first inserted should be line 18 (sig-2)")
	// Cursor should land on last inserted line (line 24, just above hunk at 25)
	cursorRow := m.getRows()[m.cursorLine()]
	assert.Equal(t, 24, cursorRow.pair.New.Num, "cursor should be on last inserted line")
}

func TestTab_SeparatorMiddle_NoBreadcrumb_Noop(t *testing.T) {
	m := makeEnterTestModel()
	// No structureMaps → no breadcrumb
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	rows := m.buildRows()
	sepIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparator && row.fileIndex == 0 {
			sepIdx = i
			break
		}
	}
	require.True(t, sepIdx >= 0)
	m.w().scroll = sepIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.Equal(t, originalPairsLen, len(m.files[0].Pairs), "should not change Pairs without breadcrumb")
}

func TestTab_RepeatedExpansion_MergesHunks(t *testing.T) {
	// Small gap: lines 1-3 and 10-12 (gap of 6 lines). Two Enter presses on
	// SeparatorTop should fill the gap and merge the hunks.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
					// gap: 4-9
					{Old: sidebyside.Line{Num: 10, Content: "10", Type: sidebyside.Context}, New: sidebyside.Line{Num: 10, Content: "10", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 11, Content: "11", Type: sidebyside.Context}, New: sidebyside.Line{Num: 11, Content: "11", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 12, Content: "12", Type: sidebyside.Context}, New: sidebyside.Line{Num: 12, Content: "12", Type: sidebyside.Context}},
				},
				OldContent: makeTestContent(15),
				NewContent: makeTestContent(15),
			},
		},
		width:  80,
		height: 40,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify there's a separator
	rows := m.buildRows()
	hasSep := false
	sepTopIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			hasSep = true
			sepTopIdx = i
			break
		}
	}
	require.True(t, hasSep, "should have separator before expansion")

	// First Enter: expand down 6 lines (gap is only 6, clamped)
	m.w().scroll = sepTopIdx
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	// Gap was 6 lines (4-9), all should be filled, hunks merged
	assert.Equal(t, 12, len(m.files[0].Pairs), "should have 12 pairs (gap filled)")

	// Verify no separator remains
	m.calculateTotalLines()
	rows = m.buildRows()
	for _, row := range rows {
		assert.NotEqual(t, RowKindSeparator, row.kind, "separator should be gone after merge")
	}
}

func TestTab_NonSeparatorRow_Noop(t *testing.T) {
	m := makeEnterTestModel()
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	// Position on a content row
	m.w().scroll = 0

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.Equal(t, originalPairsLen, len(m.files[0].Pairs), "should not change on content row")
}

func TestTab_NoContent_Noop(t *testing.T) {
	m := makeEnterTestModel()
	m.files[0].OldContent = nil
	m.files[0].NewContent = nil
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	rows := m.buildRows()
	sepIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			sepIdx = i
			break
		}
	}
	require.True(t, sepIdx >= 0)
	m.w().scroll = sepIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.Equal(t, originalPairsLen, len(m.files[0].Pairs), "should not change without content")
}

func TestTab_CursorAfterMerge(t *testing.T) {
	// Small gap: lines 1-3 and 10-12 (gap of 6 lines).
	// Expanding from SeparatorTop fills the gap and merges hunks.
	// Cursor should land on line 4 (first inserted line).
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
					// gap: 4-9
					{Old: sidebyside.Line{Num: 10, Content: "10", Type: sidebyside.Context}, New: sidebyside.Line{Num: 10, Content: "10", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 11, Content: "11", Type: sidebyside.Context}, New: sidebyside.Line{Num: 11, Content: "11", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 12, Content: "12", Type: sidebyside.Context}, New: sidebyside.Line{Num: 12, Content: "12", Type: sidebyside.Context}},
				},
				OldContent: makeTestContent(15),
				NewContent: makeTestContent(15),
			},
		},
		width:  80,
		height: 40,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find SeparatorTop and expand (fills entire 6-line gap)
	rows := m.buildRows()
	sepTopIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			sepTopIdx = i
			break
		}
	}
	require.True(t, sepTopIdx >= 0)
	m.w().scroll = sepTopIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.Equal(t, 12, len(m.files[0].Pairs), "gap should be fully filled")
	cursorRow := m.getRows()[m.cursorLine()]
	assert.Equal(t, 4, cursorRow.pair.New.Num, "cursor should be on first inserted line after merge")
}

func TestTab_SmallGap_CursorPositioning(t *testing.T) {
	// Gap of only 3 lines (4-6) between hunks at 1-3 and 7-9.
	// SeparatorTop: inserts 3 lines (clamped), cursor on line 4.
	// SeparatorBottom: inserts 3 lines (clamped), cursor on line 6.
	t.Run("SeparatorTop", func(t *testing.T) {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
						// gap: 4-6
						{Old: sidebyside.Line{Num: 7, Content: "7", Type: sidebyside.Context}, New: sidebyside.Line{Num: 7, Content: "7", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 8, Content: "8", Type: sidebyside.Context}, New: sidebyside.Line{Num: 8, Content: "8", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 9, Content: "9", Type: sidebyside.Context}, New: sidebyside.Line{Num: 9, Content: "9", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(10),
					NewContent: makeTestContent(10),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		sepTopIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				sepTopIdx = i
				break
			}
		}
		require.True(t, sepTopIdx >= 0)
		m.w().scroll = sepTopIdx

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		assert.Equal(t, 9, len(m.files[0].Pairs), "3-line gap should be filled")
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 4, cursorRow.pair.New.Num, "cursor should be on first inserted line")
	})

	t.Run("SeparatorBottom", func(t *testing.T) {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
						// gap: 4-6
						{Old: sidebyside.Line{Num: 7, Content: "7", Type: sidebyside.Context}, New: sidebyside.Line{Num: 7, Content: "7", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 8, Content: "8", Type: sidebyside.Context}, New: sidebyside.Line{Num: 8, Content: "8", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 9, Content: "9", Type: sidebyside.Context}, New: sidebyside.Line{Num: 9, Content: "9", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(10),
					NewContent: makeTestContent(10),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		sepBotIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorBottom && row.fileIndex == 0 {
				sepBotIdx = i
				break
			}
		}
		require.True(t, sepBotIdx >= 0)
		m.w().scroll = sepBotIdx

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		assert.Equal(t, 9, len(m.files[0].Pairs), "3-line gap should be filled")
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 6, cursorRow.pair.New.Num, "cursor should be on last inserted line")
	})
}

func TestTab_ExpandNearFileBoundary(t *testing.T) {
	t.Run("SeparatorTop_ClampedByFileEnd", func(t *testing.T) {
		// Hunk at lines 1-3, gap to lines 15-17 in a 17-line file.
		// Expanding down from hunk 0: 15 lines requested but only 11 available (4-14).
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
						// gap: 4-14
						{Old: sidebyside.Line{Num: 15, Content: "15", Type: sidebyside.Context}, New: sidebyside.Line{Num: 15, Content: "15", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 16, Content: "16", Type: sidebyside.Context}, New: sidebyside.Line{Num: 16, Content: "16", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 17, Content: "17", Type: sidebyside.Context}, New: sidebyside.Line{Num: 17, Content: "17", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(17),
					NewContent: makeTestContent(17),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		sepTopIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				sepTopIdx = i
				break
			}
		}
		require.True(t, sepTopIdx >= 0)
		m.w().scroll = sepTopIdx

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		// Clamped to next hunk: inserts lines 4-14 (11 lines, not 15)
		assert.Equal(t, 17, len(m.files[0].Pairs), "gap should be fully filled (clamped to next hunk)")
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 4, cursorRow.pair.New.Num, "cursor should be on first inserted line")
	})

	t.Run("SeparatorBottom_ClampedByFileStart", func(t *testing.T) {
		// Hunks at lines 3-5 and 18-20 in a 20-line file.
		// Expanding up on the separator between them: gap is 6-17 (12 lines),
		// only 15 requested so all 12 fit → clamped to prev hunk.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 4, Content: "4", Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Content: "4", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 5, Content: "5", Type: sidebyside.Context}, New: sidebyside.Line{Num: 5, Content: "5", Type: sidebyside.Context}},
						// gap: 6-17
						{Old: sidebyside.Line{Num: 18, Content: "18", Type: sidebyside.Context}, New: sidebyside.Line{Num: 18, Content: "18", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 19, Content: "19", Type: sidebyside.Context}, New: sidebyside.Line{Num: 19, Content: "19", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 20, Content: "20", Type: sidebyside.Context}, New: sidebyside.Line{Num: 20, Content: "20", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(20),
					NewContent: makeTestContent(20),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		// Find last SeparatorBottom (the one between the two hunks, not before hunk 0)
		sepBotIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorBottom && row.fileIndex == 0 {
				sepBotIdx = i
			}
		}
		require.True(t, sepBotIdx >= 0)
		m.w().scroll = sepBotIdx

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		// Expands 15 lines up from hunk at 18: clamped to prev hunk ending at 5,
		// so inserts lines 6-17 (12 lines).
		assert.Equal(t, 18, len(m.files[0].Pairs), "should insert 12 lines (6-17)")
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 17, cursorRow.pair.New.Num, "cursor should be on last inserted line")
	})
}

func TestTab_MultipleSeparators_LastBottom(t *testing.T) {
	// Three hunks: 1-2, 20-21, 40-42 in a 50-line file.
	// Expand SeparatorBottom of the last separator (between hunks 2 and 3).
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
					// gap: 3-19
					{Old: sidebyside.Line{Num: 20, Content: "20", Type: sidebyside.Context}, New: sidebyside.Line{Num: 20, Content: "20", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 21, Content: "21", Type: sidebyside.Context}, New: sidebyside.Line{Num: 21, Content: "21", Type: sidebyside.Context}},
					// gap: 22-39
					{Old: sidebyside.Line{Num: 40, Content: "40", Type: sidebyside.Context}, New: sidebyside.Line{Num: 40, Content: "40", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 41, Content: "41", Type: sidebyside.Context}, New: sidebyside.Line{Num: 41, Content: "41", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 42, Content: "42", Type: sidebyside.Context}, New: sidebyside.Line{Num: 42, Content: "42", Type: sidebyside.Context}},
				},
				OldContent: makeTestContent(50),
				NewContent: makeTestContent(50),
			},
		},
		width:  80,
		height: 40,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the LAST SeparatorBottom (the one before hunk at line 40)
	rows := m.buildRows()
	sepBotIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorBottom && row.fileIndex == 0 {
			sepBotIdx = i // keep overwriting to get the last one
		}
	}
	require.True(t, sepBotIdx >= 0)
	m.w().scroll = sepBotIdx

	originalPairsLen := len(m.files[0].Pairs)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)

	assert.Greater(t, len(m.files[0].Pairs), originalPairsLen, "should have expanded")
	// Expands 15 lines up from hunk at 40: lines 25-39, clamped to prev hunk ending at 21
	assert.Equal(t, originalPairsLen+15, len(m.files[0].Pairs))
	cursorRow := m.getRows()[m.cursorLine()]
	assert.Equal(t, 39, cursorRow.pair.New.Num, "cursor should be on last inserted line (39)")
}

func TestFoldToggle_ResetsMultipleExpansions(t *testing.T) {
	// Three hunks: 1-2, 20-21, 40-42 in a 50-line file.
	// Expand two different separators, then fold cycle — both should reset.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
					// gap: 3-19
					{Old: sidebyside.Line{Num: 20, Content: "20", Type: sidebyside.Context}, New: sidebyside.Line{Num: 20, Content: "20", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 21, Content: "21", Type: sidebyside.Context}, New: sidebyside.Line{Num: 21, Content: "21", Type: sidebyside.Context}},
					// gap: 22-39
					{Old: sidebyside.Line{Num: 40, Content: "40", Type: sidebyside.Context}, New: sidebyside.Line{Num: 40, Content: "40", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 41, Content: "41", Type: sidebyside.Context}, New: sidebyside.Line{Num: 41, Content: "41", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 42, Content: "42", Type: sidebyside.Context}, New: sidebyside.Line{Num: 42, Content: "42", Type: sidebyside.Context}},
				},
				OldContent: makeTestContent(50),
				NewContent: makeTestContent(50),
			},
		},
		width:  80,
		height: 40,
		keys:   DefaultKeyMap(),
	}
	m.files[0].SaveOriginalPairs()
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	// Expand first separator (SeparatorTop between hunks 0 and 1)
	rows := m.buildRows()
	sepTopIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			sepTopIdx = i
			break
		}
	}
	require.True(t, sepTopIdx >= 0)
	m.w().scroll = sepTopIdx
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	assert.Greater(t, len(m.files[0].Pairs), originalPairsLen)

	// Expand second separator (SeparatorBottom of last separator)
	m.calculateTotalLines()
	rows = m.buildRows()
	sepBotIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorBottom && row.fileIndex == 0 {
			sepBotIdx = i
		}
	}
	require.True(t, sepBotIdx >= 0)
	m.w().scroll = sepBotIdx
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	assert.Greater(t, len(m.files[0].Pairs), originalPairsLen+15, "both expansions should be present")

	// Fold cycle: move to header, fold, unfold back
	rows = m.buildRows()
	headerIdx := -1
	for i, row := range rows {
		if row.kind == RowKindHeader && row.fileIndex == 0 {
			headerIdx = i
			break
		}
	}
	require.True(t, headerIdx >= 0)
	m.w().scroll = headerIdx

	// FoldExpanded → FoldFolded → FoldNormal → FoldExpanded
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(0))
	assert.Equal(t, originalPairsLen, len(m.files[0].Pairs), "both expansions should be reset after fold cycle")
}

func TestFoldToggle_ResetsPairsAfterExpansion(t *testing.T) {
	m := makeEnterTestModel()
	// Save original pairs (simulating what highlight.go does after semantic expansion)
	m.files[0].SaveOriginalPairs()
	m.calculateTotalLines()

	originalPairsLen := len(m.files[0].Pairs)

	// Expand context via Enter on SeparatorTop
	rows := m.buildRows()
	sepTopIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			sepTopIdx = i
			break
		}
	}
	require.True(t, sepTopIdx >= 0)
	m.w().scroll = sepTopIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	assert.Greater(t, len(m.files[0].Pairs), originalPairsLen, "Pairs should grow after expansion")

	// Now fold the file (cycle: FoldExpanded → FoldFolded)
	// Position cursor on file header first
	rows = m.buildRows()
	fileHeaderIdx := -1
	for i, row := range rows {
		if row.kind == RowKindHeader && row.fileIndex == 0 {
			fileHeaderIdx = i
			break
		}
	}
	require.True(t, fileHeaderIdx >= 0)
	m.w().scroll = fileHeaderIdx

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	assert.Equal(t, sidebyside.FoldFolded, m.fileFoldLevel(0))

	// Unfold back (FoldFolded → FoldNormal → FoldExpanded requires two presses)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(Model)
	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(0))

	// Pairs should be back to original length
	assert.Equal(t, originalPairsLen, len(m.files[0].Pairs), "Pairs should reset to original after fold cycle")
}

func TestTab_ExpandWithMixedDiffTypes(t *testing.T) {
	// Hunk 0 ends with an addition (Old.Num=0), hunk 1 starts with a deletion (New.Num=0).
	// This tests that expansion works correctly when boundary lines have zero line numbers.

	t.Run("SeparatorTop_HunkEndsWithAddition", func(t *testing.T) {
		// Hunk 0: context line 1, then addition at new line 2 (no old-side).
		// Hunk 1: context lines 20-21.
		// Gap is new-side 3-19.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "ctx", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty}, New: sidebyside.Line{Num: 2, Content: "added", Type: sidebyside.Added}},
						// gap: new 3-19
						{Old: sidebyside.Line{Num: 19, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 20, Content: "ctx", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 20, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 21, Content: "ctx", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(25),
					NewContent: makeTestContent(25),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		sepTopIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				sepTopIdx = i
				break
			}
		}
		require.True(t, sepTopIdx >= 0)
		m.w().scroll = sepTopIdx

		originalLen := len(m.files[0].Pairs)
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		assert.Greater(t, len(m.files[0].Pairs), originalLen, "should expand")
		// Cursor should land on new line 3 (first inserted, after added line at new 2)
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 3, cursorRow.pair.New.Num, "cursor should be on first inserted line")
	})

	t.Run("SeparatorBottom_HunkStartsWithDeletion", func(t *testing.T) {
		// Hunk 0: context lines 1-2.
		// Hunk 1: deletion at old line 19 (no new-side), then context at new line 20.
		// Gap is new-side 3-19.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "ctx", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "ctx", Type: sidebyside.Context}},
						// gap: new 3-19
						{Old: sidebyside.Line{Num: 19, Content: "deleted", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty}},
						{Old: sidebyside.Line{Num: 20, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 20, Content: "ctx", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(25),
					NewContent: makeTestContent(25),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		sepBotIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorBottom && row.fileIndex == 0 {
				sepBotIdx = i
				break
			}
		}
		require.True(t, sepBotIdx >= 0)
		m.w().scroll = sepBotIdx

		originalLen := len(m.files[0].Pairs)
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		assert.Greater(t, len(m.files[0].Pairs), originalLen, "should expand")
		// Cursor should land on new line 19 (last inserted, just above hunk below's first new line 20)
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 19, cursorRow.pair.New.Num, "cursor should be on last inserted line")
	})

	t.Run("SeparatorTop_HunkEndsWithDeletion", func(t *testing.T) {
		// Hunk 0: context at new line 1, then deletion at old line 2 (no new-side).
		// Hunk 1: context lines 20-21.
		// Last new-side line in hunk 0 is 1, so expansion starts at new line 2.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "ctx", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "deleted", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty}},
						// gap: new 2-19
						{Old: sidebyside.Line{Num: 20, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 20, Content: "ctx", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 21, Content: "ctx", Type: sidebyside.Context}, New: sidebyside.Line{Num: 21, Content: "ctx", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(25),
					NewContent: makeTestContent(25),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		sepTopIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				sepTopIdx = i
				break
			}
		}
		require.True(t, sepTopIdx >= 0)
		m.w().scroll = sepTopIdx

		originalLen := len(m.files[0].Pairs)
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		assert.Greater(t, len(m.files[0].Pairs), originalLen, "should expand")
		// Last new line in hunk 0 is 1, so first inserted is new line 2
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 2, cursorRow.pair.New.Num, "cursor should be on first inserted line")
	})
}

func TestTab_TrailingSeparator(t *testing.T) {
	t.Run("appears when more content below last hunk", func(t *testing.T) {
		// Hunk at lines 1-3 in a 20-line file. Trailing separator should appear.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(20),
					NewContent: makeTestContent(20),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		// Find trailing SeparatorTop (after content, not between hunks)
		lastSepTopIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				lastSepTopIdx = i
			}
		}
		require.True(t, lastSepTopIdx >= 0, "trailing separator should exist")

		// It should be after the last content row
		lastContentIdx := -1
		for i, row := range rows {
			if row.kind == RowKindContent && row.fileIndex == 0 {
				lastContentIdx = i
			}
		}
		assert.Greater(t, lastSepTopIdx, lastContentIdx, "trailing separator should be after last content row")
	})

	t.Run("absent when last hunk reaches end of file", func(t *testing.T) {
		// Hunk at lines 1-3 in a 3-line file. No trailing separator.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(3),
					NewContent: makeTestContent(3),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		for _, row := range rows {
			assert.NotEqual(t, RowKindSeparatorTop, row.kind, "no trailing separator when file ends at last hunk")
		}
	})

	t.Run("absent when content not loaded", func(t *testing.T) {
		// Hunk at lines 5-7 with no NewContent loaded. No trailing separator.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 5, Content: "5", Type: sidebyside.Context}, New: sidebyside.Line{Num: 5, Content: "5", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 6, Content: "6", Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Content: "6", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 7, Content: "7", Type: sidebyside.Context}, New: sidebyside.Line{Num: 7, Content: "7", Type: sidebyside.Context}},
					},
					// No OldContent/NewContent
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		for _, row := range rows {
			assert.NotEqual(t, RowKindSeparatorTop, row.kind, "no trailing separator without content")
		}
	})

	t.Run("Tab expands down from last hunk", func(t *testing.T) {
		// Hunk at lines 1-3 in a 20-line file. Tab on trailing separator expands down.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(20),
					NewContent: makeTestContent(20),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()
		lastSepTopIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				lastSepTopIdx = i
			}
		}
		require.True(t, lastSepTopIdx >= 0)
		m.w().scroll = lastSepTopIdx

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		// Should insert 15 lines (4-18), total pairs = 3 + 15 = 18
		assert.Equal(t, 18, len(m.files[0].Pairs), "should expand 15 lines down")
		cursorRow := m.getRows()[m.cursorLine()]
		assert.Equal(t, 4, cursorRow.pair.New.Num, "cursor should be on first inserted line")
	})

	t.Run("repeated Tab expands to end of file", func(t *testing.T) {
		// Hunk at lines 1-3 in a 10-line file. First Tab adds 7 lines (all remaining).
		// Second Tab is a no-op and trailing separator disappears.
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "1", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "2", Type: sidebyside.Context}},
						{Old: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "3", Type: sidebyside.Context}},
					},
					OldContent: makeTestContent(10),
					NewContent: makeTestContent(10),
				},
			},
			width:  80,
			height: 40,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		// First Tab: expand all 7 remaining lines
		rows := m.buildRows()
		lastSepTopIdx := -1
		for i, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				lastSepTopIdx = i
			}
		}
		require.True(t, lastSepTopIdx >= 0)
		m.w().scroll = lastSepTopIdx

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)

		assert.Equal(t, 10, len(m.files[0].Pairs), "should expand to full file (10 lines)")

		// After expanding to end, trailing separator should be gone
		rows = m.buildRows()
		for _, row := range rows {
			if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
				t.Error("trailing separator should disappear after expanding to end of file")
			}
		}
	})
}

// =============================================================================
// Window Management Tests
// =============================================================================

func TestWindowSplit_CreatesSecondWindow(t *testing.T) {
	m := makeTestModel(20)
	m.w().scroll = 5
	m.setFileFoldLevel(0, sidebyside.FoldExpanded)

	// Should start with 1 window
	assert.Equal(t, 1, len(m.windows))
	assert.Equal(t, 0, m.activeWindowIdx)

	// Ctrl+W % creates a split
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+w"), Alt: false})
	m = newM.(Model)
	assert.Equal(t, "ctrl+w", m.pendingKey)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("%")})
	m = newM.(Model)

	// Should now have 2 windows
	assert.Equal(t, 2, len(m.windows))
	// New window should be active
	assert.Equal(t, 1, m.activeWindowIdx)
	// New window should have same scroll position
	assert.Equal(t, 5, m.windows[1].scroll)
	// New window should have same fold state
	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(0))
}

func TestWindowSplit_MaxTwoWindows(t *testing.T) {
	m := makeTestModel(20)

	// Create first split
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows))

	// Try to create another split - should fail
	newM, _ = m.windowSplitVertical()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows), "should not create more than 2 windows")
	assert.Equal(t, "Maximum 2 windows", m.statusMessage)
}

func TestWindowClose_ClosesCurrentWindow(t *testing.T) {
	m := makeTestModel(20)

	// Create a split
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows))
	assert.Equal(t, 1, m.activeWindowIdx)

	// Close current window
	newM, _ = m.windowClose()
	m = newM.(Model)
	assert.Equal(t, 1, len(m.windows))
	assert.Equal(t, 0, m.activeWindowIdx)
}

func TestWindowClose_CannotCloseLastWindow(t *testing.T) {
	m := makeTestModel(20)
	assert.Equal(t, 1, len(m.windows))

	// Try to close the last window - should fail
	newM, _ := m.windowClose()
	m = newM.(Model)
	assert.Equal(t, 1, len(m.windows), "should not close last window")
	assert.Equal(t, "Cannot close last window", m.statusMessage)
}

func TestQuitClosesWindowWhenMultiple(t *testing.T) {
	m := makeTestModel(20)
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows))

	// Press q - should close window, not quit
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newM, cmd := m.Update(msg)
	m = newM.(Model)

	assert.Equal(t, 1, len(m.windows), "q should close window when multiple exist")
	assert.Nil(t, cmd, "should not return quit command")
}

func TestQuitQuitsWhenSingleWindow(t *testing.T) {
	m := makeTestModel(20)
	assert.Equal(t, 1, len(m.windows))

	// Press q - should quit
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	assert.NotNil(t, cmd, "should return quit command")
}

func TestWindowFocusLeft(t *testing.T) {
	m := makeTestModel(20)
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx)

	// Focus left
	newM, _ = m.windowFocusLeft()
	m = newM.(Model)
	assert.Equal(t, 0, m.activeWindowIdx)

	// Try to go further left - should stay at 0
	newM, _ = m.windowFocusLeft()
	m = newM.(Model)
	assert.Equal(t, 0, m.activeWindowIdx)
}

func TestWindowFocusRight(t *testing.T) {
	m := makeTestModel(20)
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	m.activeWindowIdx = 0 // Start at left window

	// Focus right
	newM, _ = m.windowFocusRight()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx)

	// Try to go further right - should stay at 1
	newM, _ = m.windowFocusRight()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx)
}

func TestWindowIndependentScroll(t *testing.T) {
	m := makeTestModel(100)
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Set different scroll positions
	m.windows[0].scroll = 10
	m.windows[1].scroll = 50

	// Scrolling in one window shouldn't affect the other
	m.activeWindowIdx = 0
	m.w().scroll = 15 // Scroll window 0

	assert.Equal(t, 15, m.windows[0].scroll)
	assert.Equal(t, 50, m.windows[1].scroll, "window 1 scroll should be unchanged")
}

func TestWindowIndependentFoldState(t *testing.T) {
	m := makeTestModel(20)
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Set different fold states in each window
	m.activeWindowIdx = 0
	m.setFileFoldLevel(0, sidebyside.FoldFolded)

	m.activeWindowIdx = 1
	m.setFileFoldLevel(0, sidebyside.FoldExpanded)

	// Verify fold states are independent
	m.activeWindowIdx = 0
	assert.Equal(t, sidebyside.FoldFolded, m.fileFoldLevel(0))

	m.activeWindowIdx = 1
	assert.Equal(t, sidebyside.FoldExpanded, m.fileFoldLevel(0))
}

func TestWindowCommentSync(t *testing.T) {
	// When a comment is added in one window, it should appear in both windows
	m := makeTestModel(20)
	m.setFileFoldLevel(0, sidebyside.FoldExpanded)
	m.calculateTotalLines()

	// Split into two windows
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Window 0 is active, add a comment
	m.activeWindowIdx = 0
	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = "Test comment from window 0"

	// Rebuild all caches (simulating what submitComment does)
	m.rebuildAllRowCachesPreservingCursor()

	// Both windows should have the comment in their cached rows
	// Check window 0
	m.activeWindowIdx = 0
	rows0 := m.getRows()
	hasComment0 := false
	for _, row := range rows0 {
		if row.kind == RowKindComment && row.commentLineNum == 5 {
			hasComment0 = true
			break
		}
	}
	assert.True(t, hasComment0, "window 0 should have the comment row")

	// Check window 1
	m.activeWindowIdx = 1
	rows1 := m.getRows()
	hasComment1 := false
	for _, row := range rows1 {
		if row.kind == RowKindComment && row.commentLineNum == 5 {
			hasComment1 = true
			break
		}
	}
	assert.True(t, hasComment1, "window 1 should also have the comment row")
}

func TestWindowCursorPreservedOnCommentAdd(t *testing.T) {
	// When a comment is added, both windows should preserve cursor on same logical row
	m := makeTestModel(20)
	m.setFileFoldLevel(0, sidebyside.FoldExpanded)
	m.rebuildRowsCache()

	// Split into two windows
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Rebuild both windows' caches
	m.activeWindowIdx = 0
	m.rebuildRowsCache()
	m.activeWindowIdx = 1
	m.rebuildRowsCache()

	// Find content rows to position cursors on (skip header rows)
	// Position window 0 on line 5, window 1 on line 15
	m.activeWindowIdx = 0
	rows0 := m.getRows()
	var scroll0, scroll1 int
	for i, row := range rows0 {
		if row.kind == RowKindContent && row.pair.New.Num == 5 {
			scroll0 = i
		}
		if row.kind == RowKindContent && row.pair.New.Num == 15 {
			scroll1 = i
		}
	}

	m.activeWindowIdx = 0
	m.w().scroll = scroll0

	m.activeWindowIdx = 1
	m.w().scroll = scroll1

	// Verify initial positions
	m.activeWindowIdx = 0
	identity0 := m.getCursorRowIdentity()
	require.Equal(t, 5, identity0.newNum, "window 0 should start on line 5")

	m.activeWindowIdx = 1
	identity1 := m.getCursorRowIdentity()
	require.Equal(t, 15, identity1.newNum, "window 1 should start on line 15")

	// Add a comment near the top (should shift rows below it)
	key := commentKey{fileIndex: 0, newLineNum: 2}
	m.comments[key] = "Comment that shifts rows"
	m.rebuildAllRowCachesPreservingCursor()

	// Both windows should still be on their original logical rows (same line numbers)
	m.activeWindowIdx = 0
	newIdentity0 := m.getCursorRowIdentity()
	assert.Equal(t, 5, newIdentity0.newNum, "window 0 should stay on line 5")

	m.activeWindowIdx = 1
	newIdentity1 := m.getCursorRowIdentity()
	assert.Equal(t, 15, newIdentity1.newNum, "window 1 should stay on line 15")

	// Scroll positions should have increased to account for the comment rows
	assert.Greater(t, m.windows[0].scroll, scroll0, "window 0 scroll should increase due to comment")
	assert.Greater(t, m.windows[1].scroll, scroll1, "window 1 scroll should increase due to comment")
}

func TestWindowSearchNavigationIndependent(t *testing.T) {
	// Search navigation (n/N) should operate independently per window
	m := makeTestModel(20)
	m.setFileFoldLevel(0, sidebyside.FoldExpanded)
	m.calculateTotalLines()

	// Set up a search query (shared state)
	m.searchQuery = "right" // matches every line

	// Split into two windows
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Navigate search in window 0
	m.activeWindowIdx = 0
	m.w().searchMatchIdx = 3
	m.w().searchMatchSide = 1

	// Navigate search in window 1 to different position
	m.activeWindowIdx = 1
	m.w().searchMatchIdx = 7
	m.w().searchMatchSide = 1

	// Verify they are independent
	assert.Equal(t, 3, m.windows[0].searchMatchIdx, "window 0 search index should be 3")
	assert.Equal(t, 7, m.windows[1].searchMatchIdx, "window 1 search index should be 7")
}

func TestWindowCloseWhileInCommentMode(t *testing.T) {
	// Closing a window while in comment mode should work (comment is lost)
	m := makeTestModel(20)
	m.setFileFoldLevel(0, sidebyside.FoldExpanded)
	m.calculateTotalLines()

	// Split into two windows
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Enter comment mode in window 0
	m.activeWindowIdx = 0
	m.w().commentMode = true
	m.w().commentInput = "Unsaved comment"
	m.w().commentKey = commentKey{fileIndex: 0, newLineNum: 5}

	// Close window 0
	newM, _ = m.windowClose()
	m = newM.(Model)

	// Should now have only 1 window
	assert.Equal(t, 1, len(m.windows), "should have 1 window after close")

	// The remaining window should not be in comment mode
	assert.False(t, m.w().commentMode, "remaining window should not be in comment mode")

	// The unsaved comment should not be in the comments map
	key := commentKey{fileIndex: 0, newLineNum: 5}
	_, exists := m.comments[key]
	assert.False(t, exists, "unsaved comment should not be saved")
}

func TestWindowSplitWhileInCommentMode(t *testing.T) {
	// Splitting while in comment mode: original stays in comment mode, new window doesn't
	m := makeTestModel(20)
	m.setFileFoldLevel(0, sidebyside.FoldExpanded)
	m.calculateTotalLines()

	// Enter comment mode
	m.w().commentMode = true
	m.w().commentInput = "Editing a comment"
	m.w().commentKey = commentKey{fileIndex: 0, newLineNum: 5}

	// Split
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Original window (index 0) should still be in comment mode
	assert.True(t, m.windows[0].commentMode, "original window should stay in comment mode")
	assert.Equal(t, "Editing a comment", m.windows[0].commentInput)

	// New window (index 1) should NOT be in comment mode
	assert.False(t, m.windows[1].commentMode, "new window should not be in comment mode")
	assert.Equal(t, "", m.windows[1].commentInput)
}

func TestInvalidateAllRowCaches(t *testing.T) {
	m := makeTestModel(20)
	m.calculateTotalLines()

	// Split into two windows and ensure both have valid caches
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	m.activeWindowIdx = 0
	m.rebuildRowsCache()
	assert.True(t, m.windows[0].rowsCacheValid)

	m.activeWindowIdx = 1
	m.rebuildRowsCache()
	assert.True(t, m.windows[1].rowsCacheValid)

	// Invalidate all
	m.invalidateAllRowCaches()

	// Both should be invalid
	assert.False(t, m.windows[0].rowsCacheValid, "window 0 cache should be invalid")
	assert.False(t, m.windows[1].rowsCacheValid, "window 1 cache should be invalid")
}

func TestWindowResizeLeft(t *testing.T) {
	m := makeTestModel(20)
	m.width = 100 // 100 char terminal

	// Split into two windows
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Initial ratio should be 0.5
	assert.Equal(t, 0.5, m.windowSplitRatio)

	// Resize left (shrink left window)
	newM, _ = m.windowResizeLeft()
	m = newM.(Model)

	// Ratio should decrease by 8/100 = 0.08
	assert.InDelta(t, 0.42, m.windowSplitRatio, 0.001, "ratio should decrease")

	// Multiple resizes
	for i := 0; i < 10; i++ {
		newM, _ = m.windowResizeLeft()
		m = newM.(Model)
	}

	// Should be clamped at minimum 0.2
	assert.Equal(t, 0.2, m.windowSplitRatio, "ratio should clamp at 0.2")
}

func TestWindowResizeRight(t *testing.T) {
	m := makeTestModel(20)
	m.width = 100 // 100 char terminal

	// Split into two windows
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)

	// Initial ratio should be 0.5
	assert.Equal(t, 0.5, m.windowSplitRatio)

	// Resize right (grow left window)
	newM, _ = m.windowResizeRight()
	m = newM.(Model)

	// Ratio should increase by 8/100 = 0.08
	assert.InDelta(t, 0.58, m.windowSplitRatio, 0.001, "ratio should increase")

	// Multiple resizes
	for i := 0; i < 10; i++ {
		newM, _ = m.windowResizeRight()
		m = newM.(Model)
	}

	// Should be clamped at maximum 0.8
	assert.Equal(t, 0.8, m.windowSplitRatio, "ratio should clamp at 0.8")
}

func TestWindowResizeNoopWithSingleWindow(t *testing.T) {
	m := makeTestModel(20)
	m.width = 100
	m.windowSplitRatio = 0.5

	// Without split, resize should be a no-op
	newM, _ := m.windowResizeLeft()
	m = newM.(Model)
	assert.Equal(t, 0.5, m.windowSplitRatio, "ratio should not change without split")

	newM, _ = m.windowResizeRight()
	m = newM.(Model)
	assert.Equal(t, 0.5, m.windowSplitRatio, "ratio should not change without split")
}

func TestWindowSplitHorizontal_CreatesSecondWindow(t *testing.T) {
	m := makeTestModel(20)
	m.height = 40
	assert.Equal(t, 1, len(m.windows))

	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows))
	assert.Equal(t, 1, m.activeWindowIdx, "new window should be active")
	assert.False(t, m.windowSplitV, "split should be horizontal")
}

func TestWindowSplitHorizontal_MaxTwoWindows(t *testing.T) {
	m := makeTestModel(20)
	m.height = 40

	// Create first split
	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows))

	// Try to create another split - should fail
	newM, _ = m.windowSplitHorizontal()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows), "should not create more than 2 windows")
	assert.Equal(t, "Maximum 2 windows", m.statusMessage)
}

func TestWindowFocusDown(t *testing.T) {
	m := makeTestModel(20)
	m.height = 40
	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	m.activeWindowIdx = 0 // Start at top window

	// Focus down
	newM, _ = m.windowFocusDown()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx)

	// Try to go further down - should stay at 1
	newM, _ = m.windowFocusDown()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx)
}

func TestWindowFocusUp(t *testing.T) {
	m := makeTestModel(20)
	m.height = 40
	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx)

	// Focus up
	newM, _ = m.windowFocusUp()
	m = newM.(Model)
	assert.Equal(t, 0, m.activeWindowIdx)

	// Try to go further up - should stay at 0
	newM, _ = m.windowFocusUp()
	m = newM.(Model)
	assert.Equal(t, 0, m.activeWindowIdx)
}

func TestWindowFocusUpDown_NoopForVerticalSplit(t *testing.T) {
	m := makeTestModel(20)
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	assert.True(t, m.windowSplitV, "should be vertical split")
	m.activeWindowIdx = 0

	// Focus down should be noop for vertical split
	newM, _ = m.windowFocusDown()
	m = newM.(Model)
	assert.Equal(t, 0, m.activeWindowIdx, "j should not change focus in vertical split")

	// Focus up should be noop for vertical split
	m.activeWindowIdx = 1
	newM, _ = m.windowFocusUp()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx, "k should not change focus in vertical split")
}

func TestWindowFocusLeftRight_NoopForHorizontalSplit(t *testing.T) {
	m := makeTestModel(20)
	m.height = 40
	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	assert.False(t, m.windowSplitV, "should be horizontal split")
	m.activeWindowIdx = 0

	// Focus right should be noop for horizontal split
	newM, _ = m.windowFocusRight()
	m = newM.(Model)
	assert.Equal(t, 0, m.activeWindowIdx, "l should not change focus in horizontal split")

	// Focus left should be noop for horizontal split
	m.activeWindowIdx = 1
	newM, _ = m.windowFocusLeft()
	m = newM.(Model)
	assert.Equal(t, 1, m.activeWindowIdx, "h should not change focus in horizontal split")
}

func TestWindowResizeDown(t *testing.T) {
	m := makeTestModel(20)
	m.height = 100
	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	m.windowSplitRatioH = 0.5

	// Resize down (give top window more space)
	newM, _ = m.windowResizeDown()
	m = newM.(Model)
	assert.Greater(t, m.windowSplitRatioH, 0.5, "ratio should increase")

	// Keep resizing until we hit the max
	for i := 0; i < 20; i++ {
		newM, _ = m.windowResizeDown()
		m = newM.(Model)
	}

	// Should be clamped at maximum 0.8
	assert.Equal(t, 0.8, m.windowSplitRatioH, "ratio should clamp at 0.8")
}

func TestWindowResizeUp(t *testing.T) {
	m := makeTestModel(20)
	m.height = 100
	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	m.windowSplitRatioH = 0.5

	// Resize up (give bottom window more space)
	newM, _ = m.windowResizeUp()
	m = newM.(Model)
	assert.Less(t, m.windowSplitRatioH, 0.5, "ratio should decrease")

	// Keep resizing until we hit the min
	for i := 0; i < 20; i++ {
		newM, _ = m.windowResizeUp()
		m = newM.(Model)
	}

	// Should be clamped at minimum 0.2
	assert.Equal(t, 0.2, m.windowSplitRatioH, "ratio should clamp at 0.2")
}

func TestWindowResizeUpDown_NoopForVerticalSplit(t *testing.T) {
	m := makeTestModel(20)
	m.height = 100
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	m.windowSplitRatioH = 0.5

	// Resize down should be noop for vertical split
	newM, _ = m.windowResizeDown()
	m = newM.(Model)
	assert.Equal(t, 0.5, m.windowSplitRatioH, "ratio should not change in vertical split")

	// Resize up should be noop for vertical split
	newM, _ = m.windowResizeUp()
	m = newM.(Model)
	assert.Equal(t, 0.5, m.windowSplitRatioH, "ratio should not change in vertical split")
}

func TestWindowResizeLeftRight_NoopForHorizontalSplit(t *testing.T) {
	m := makeTestModel(20)
	m.height = 100
	newM, _ := m.windowSplitHorizontal()
	m = newM.(Model)
	m.windowSplitRatio = 0.5

	// Resize left should be noop for horizontal split
	newM, _ = m.windowResizeLeft()
	m = newM.(Model)
	assert.Equal(t, 0.5, m.windowSplitRatio, "ratio should not change in horizontal split")

	// Resize right should be noop for horizontal split
	newM, _ = m.windowResizeRight()
	m = newM.(Model)
	assert.Equal(t, 0.5, m.windowSplitRatio, "ratio should not change in horizontal split")
}

// =============================================================================
// shiftFileIndexMaps tests
// =============================================================================

func TestShiftFileIndexMaps_ShiftsHighlightSpans(t *testing.T) {
	m := Model{
		highlightSpans: map[int]*FileHighlight{
			0: {OldSpans: []highlight.Span{{Start: 0, End: 5}}},
			1: {OldSpans: []highlight.Span{{Start: 10, End: 15}}},
		},
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
	}

	m.shiftFileIndexMaps(3)

	// Old indices 0, 1 should now be at 3, 4
	assert.Nil(t, m.highlightSpans[0], "index 0 should be empty after shift")
	assert.Nil(t, m.highlightSpans[1], "index 1 should be empty after shift")
	assert.NotNil(t, m.highlightSpans[3], "index 3 should have old index 0's data")
	assert.NotNil(t, m.highlightSpans[4], "index 4 should have old index 1's data")
	assert.Equal(t, 0, m.highlightSpans[3].OldSpans[0].Start)
	assert.Equal(t, 10, m.highlightSpans[4].OldSpans[0].Start)
}

func TestShiftFileIndexMaps_ShiftsPairsHighlightSpans(t *testing.T) {
	m := Model{
		highlightSpans: make(map[int]*FileHighlight),
		pairsHighlightSpans: map[int]*PairsFileHighlight{
			0: {OldSpans: []highlight.Span{{Start: 0, End: 5}}},
			2: {OldSpans: []highlight.Span{{Start: 20, End: 25}}},
		},
		structureMaps:      make(map[int]*FileStructure),
		pairsStructureMaps: make(map[int]*FileStructure),
		loadingFiles:       make(map[int]time.Time),
		inlineDiffCache:    make(map[inlineDiffKey]inlineDiffResult),
	}

	m.shiftFileIndexMaps(2)

	assert.Nil(t, m.pairsHighlightSpans[0])
	assert.NotNil(t, m.pairsHighlightSpans[2], "index 2 should have old index 0's data")
	assert.NotNil(t, m.pairsHighlightSpans[4], "index 4 should have old index 2's data")
}

func TestShiftFileIndexMaps_ShiftsStructureMaps(t *testing.T) {
	m := Model{
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps: map[int]*FileStructure{
			0: {NewStructure: &structure.Map{}},
		},
		pairsStructureMaps: map[int]*FileStructure{
			1: {NewStructure: &structure.Map{}},
		},
		loadingFiles:    make(map[int]time.Time),
		inlineDiffCache: make(map[inlineDiffKey]inlineDiffResult),
	}

	m.shiftFileIndexMaps(5)

	assert.Nil(t, m.structureMaps[0])
	assert.NotNil(t, m.structureMaps[5])
	assert.Nil(t, m.pairsStructureMaps[1])
	assert.NotNil(t, m.pairsStructureMaps[6])
}

func TestShiftFileIndexMaps_ShiftsLoadingFiles(t *testing.T) {
	now := time.Now()
	m := Model{
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles: map[int]time.Time{
			0: now,
			3: now.Add(time.Second),
		},
		inlineDiffCache: make(map[inlineDiffKey]inlineDiffResult),
	}

	m.shiftFileIndexMaps(2)

	assert.True(t, m.loadingFiles[0].IsZero(), "index 0 should be empty")
	assert.False(t, m.loadingFiles[2].IsZero(), "index 2 should have old index 0's time")
	assert.False(t, m.loadingFiles[5].IsZero(), "index 5 should have old index 3's time")
}

func TestShiftFileIndexMaps_ClearsInlineDiffCache(t *testing.T) {
	m := Model{
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache: map[inlineDiffKey]inlineDiffResult{
			{fileIndex: 0, oldNum: 1, newNum: 1}: {},
			{fileIndex: 1, oldNum: 2, newNum: 2}: {},
		},
	}

	m.shiftFileIndexMaps(1)

	assert.Empty(t, m.inlineDiffCache, "inlineDiffCache should be cleared")
}

func TestShiftFileIndexMaps_EmptyMaps(t *testing.T) {
	m := Model{
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
	}

	// Should not panic with empty maps
	m.shiftFileIndexMaps(5)

	assert.Empty(t, m.highlightSpans)
	assert.Empty(t, m.pairsHighlightSpans)
}

// =============================================================================
// insertSnapshotCommit tests
// =============================================================================

func TestInsertSnapshotCommit_PrependsCommit(t *testing.T) {
	m := Model{
		commits: []sidebyside.CommitSet{
			{Info: sidebyside.CommitInfo{Subject: "Original"}},
		},
		files: []sidebyside.FilePair{
			{NewPath: "original.go"},
		},
		commitFileStarts:    []int{0},
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
	}

	newCommit := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{Subject: "Snapshot"},
		Files: []sidebyside.FilePair{{NewPath: "snapshot.go"}},
	}

	m.insertSnapshotCommit(newCommit)

	require.Len(t, m.commits, 2)
	assert.Equal(t, "Snapshot", m.commits[0].Info.Subject)
	assert.Equal(t, "Original", m.commits[1].Info.Subject)
}

func TestInsertSnapshotCommit_PrependsFiles(t *testing.T) {
	m := Model{
		commits: []sidebyside.CommitSet{
			{Info: sidebyside.CommitInfo{Subject: "Original"}},
		},
		files: []sidebyside.FilePair{
			{NewPath: "original1.go"},
			{NewPath: "original2.go"},
		},
		commitFileStarts:    []int{0},
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
	}

	newCommit := sidebyside.CommitSet{
		Files: []sidebyside.FilePair{
			{NewPath: "snapshot1.go"},
			{NewPath: "snapshot2.go"},
			{NewPath: "snapshot3.go"},
		},
	}

	m.insertSnapshotCommit(newCommit)

	require.Len(t, m.files, 5)
	assert.Equal(t, "snapshot1.go", m.files[0].NewPath)
	assert.Equal(t, "snapshot2.go", m.files[1].NewPath)
	assert.Equal(t, "snapshot3.go", m.files[2].NewPath)
	assert.Equal(t, "original1.go", m.files[3].NewPath)
	assert.Equal(t, "original2.go", m.files[4].NewPath)
}

func TestInsertSnapshotCommit_UpdatesCommitFileStarts(t *testing.T) {
	m := Model{
		commits: []sidebyside.CommitSet{
			{Info: sidebyside.CommitInfo{Subject: "Commit1"}},
			{Info: sidebyside.CommitInfo{Subject: "Commit2"}},
		},
		files: []sidebyside.FilePair{
			{NewPath: "c1f1.go"},
			{NewPath: "c1f2.go"},
			{NewPath: "c2f1.go"},
		},
		commitFileStarts:    []int{0, 2}, // Commit1 starts at 0, Commit2 starts at 2
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
	}

	newCommit := sidebyside.CommitSet{
		Files: []sidebyside.FilePair{
			{NewPath: "snap1.go"},
			{NewPath: "snap2.go"},
		},
	}

	m.insertSnapshotCommit(newCommit)

	require.Len(t, m.commitFileStarts, 3)
	assert.Equal(t, 0, m.commitFileStarts[0], "new commit starts at 0")
	assert.Equal(t, 2, m.commitFileStarts[1], "old commit1 now starts at 2")
	assert.Equal(t, 4, m.commitFileStarts[2], "old commit2 now starts at 4")
}

func TestInsertSnapshotCommit_ShiftsHighlightMaps(t *testing.T) {
	m := Model{
		commits: []sidebyside.CommitSet{
			{Info: sidebyside.CommitInfo{Subject: "Original"}},
		},
		files: []sidebyside.FilePair{
			{NewPath: "original.go"},
		},
		commitFileStarts: []int{0},
		highlightSpans: map[int]*FileHighlight{
			0: {OldSpans: []highlight.Span{{Start: 100, End: 200}}},
		},
		pairsHighlightSpans: map[int]*PairsFileHighlight{
			0: {OldSpans: []highlight.Span{{Start: 50, End: 75}}},
		},
		structureMaps:      make(map[int]*FileStructure),
		pairsStructureMaps: make(map[int]*FileStructure),
		loadingFiles:       make(map[int]time.Time),
		inlineDiffCache:    make(map[inlineDiffKey]inlineDiffResult),
	}

	newCommit := sidebyside.CommitSet{
		Files: []sidebyside.FilePair{
			{NewPath: "snap1.go"},
			{NewPath: "snap2.go"},
		},
	}

	m.insertSnapshotCommit(newCommit)

	// Original file was at index 0, now at index 2 (after 2 new files)
	assert.Nil(t, m.highlightSpans[0], "old index 0 should be empty")
	assert.NotNil(t, m.highlightSpans[2], "shifted index 2 should have the data")
	assert.Equal(t, 100, m.highlightSpans[2].OldSpans[0].Start)

	assert.Nil(t, m.pairsHighlightSpans[0])
	assert.NotNil(t, m.pairsHighlightSpans[2])
	assert.Equal(t, 50, m.pairsHighlightSpans[2].OldSpans[0].Start)
}

func TestInsertSnapshotCommit_ScrollsToTop(t *testing.T) {
	m := Model{
		commits: []sidebyside.CommitSet{
			{Info: sidebyside.CommitInfo{Subject: "Original"}},
		},
		files:               []sidebyside.FilePair{{NewPath: "original.go"}},
		commitFileStarts:    []int{0},
		windows:             []*Window{{scroll: 50}},
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
	}

	m.insertSnapshotCommit(sidebyside.CommitSet{
		Files: []sidebyside.FilePair{{NewPath: "snap.go"}},
	})

	assert.Equal(t, 0, m.w().scroll, "scroll should reset to 0")
}

// =============================================================================
// SnapshotDiffReadyMsg handling tests
// =============================================================================

func TestSnapshotDiffReadyMsg_RequestsHighlighting(t *testing.T) {
	m := Model{
		commits: []sidebyside.CommitSet{
			{Info: sidebyside.CommitInfo{Subject: "Original"}},
		},
		files:               []sidebyside.FilePair{{NewPath: "original.go"}},
		commitFileStarts:    []int{0},
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		keys:                DefaultKeyMap(),
	}

	msg := SnapshotDiffReadyMsg{
		CommitSet: sidebyside.CommitSet{
			Info: sidebyside.CommitInfo{Subject: "Diff 1"},
			Files: []sidebyside.FilePair{
				{NewPath: "file1.go", Pairs: []sidebyside.LinePair{{}}},
				{NewPath: "file2.go", Pairs: []sidebyside.LinePair{{}}},
			},
		},
		SnapshotSHA: "abc123",
	}

	newModel, cmd := m.Update(msg)
	resultModel := newModel.(Model)

	// Should have prepended the new commit
	require.Len(t, resultModel.commits, 2)
	assert.Equal(t, "Diff 1", resultModel.commits[0].Info.Subject)

	// Should have prepended the files
	require.Len(t, resultModel.files, 3)
	assert.Equal(t, "file1.go", resultModel.files[0].NewPath)
	assert.Equal(t, "file2.go", resultModel.files[1].NewPath)

	// Should return a command for highlighting (non-nil)
	assert.NotNil(t, cmd, "should return highlight command for new files")
}

func TestSnapshotDiffReadyMsg_NoFilesNoHighlightCommand(t *testing.T) {
	m := Model{
		commits: []sidebyside.CommitSet{
			{Info: sidebyside.CommitInfo{Subject: "Original"}},
		},
		files:               []sidebyside.FilePair{{NewPath: "original.go"}},
		commitFileStarts:    []int{0},
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		keys:                DefaultKeyMap(),
	}

	// Empty files = no changes
	msg := SnapshotDiffReadyMsg{
		CommitSet: sidebyside.CommitSet{
			Info:  sidebyside.CommitInfo{Subject: "No changes"},
			Files: []sidebyside.FilePair{}, // empty
		},
		SnapshotSHA: "abc123",
	}

	newModel, cmd := m.Update(msg)
	resultModel := newModel.(Model)

	// Should not add the empty commit
	assert.Len(t, resultModel.commits, 1)
	assert.Equal(t, "Original", resultModel.commits[0].Info.Subject)

	// Command should be for clearing status message, not highlighting
	// (since no files were added)
	assert.NotNil(t, cmd, "should return status clear command")
}

func TestSnapshotDiffReadyMsg_Error(t *testing.T) {
	m := Model{
		commits:             []sidebyside.CommitSet{},
		files:               []sidebyside.FilePair{},
		commitFileStarts:    []int{},
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		keys:                DefaultKeyMap(),
	}

	msg := SnapshotDiffReadyMsg{
		Err: fmt.Errorf("snapshot failed"),
	}

	newModel, cmd := m.Update(msg)
	resultModel := newModel.(Model)

	// Should set error status message
	assert.Equal(t, "Snapshot diff failed", resultModel.statusMessage)
	// Should return command (for clearing status)
	assert.NotNil(t, cmd)
}

func TestSnapshotDiffReadyMsg_StoresSnapshotRefs(t *testing.T) {
	m := Model{
		commits:             []sidebyside.CommitSet{},
		files:               []sidebyside.FilePair{},
		commitFileStarts:    []int{},
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		loadingFiles:        make(map[int]time.Time),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		keys:                DefaultKeyMap(),
	}

	msg := SnapshotDiffReadyMsg{
		CommitSet: sidebyside.CommitSet{
			Info:           sidebyside.CommitInfo{Subject: "Diff 1"},
			Files:          []sidebyside.FilePair{{NewPath: "file.go", Pairs: []sidebyside.LinePair{{}}}},
			IsSnapshot:     true,
			SnapshotOldRef: "abc123",
			SnapshotNewRef: "def456",
		},
		SnapshotSHA: "def456",
	}

	newModel, _ := m.Update(msg)
	resultModel := newModel.(Model)

	require.Len(t, resultModel.commits, 1)
	assert.True(t, resultModel.commits[0].IsSnapshot)
	assert.Equal(t, "abc123", resultModel.commits[0].SnapshotOldRef)
	assert.Equal(t, "def456", resultModel.commits[0].SnapshotNewRef)
}

// =============================================================================
// SnapshotCreatedMsg handler tests
// =============================================================================

func TestSnapshotCreatedMsg_Success(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{
		{
			Info:       sidebyside.CommitInfo{Subject: "initial diff"},
			IsSnapshot: true,
			// SHA empty — simulates the initial snapshot before background completes
		},
	}, WithAutoSnapshots(true), WithShowSnapshots(true), WithBaseSHA("baseabc1234567"), WithGit(&git.MockGit{}))

	msg := SnapshotCreatedMsg{
		SHA:     "deadbeef1234567890abcdef1234567890abcdef",
		Subject: "dfd: baseabc @ Feb 5 09:15",
		Date:    "Feb 5 09:15",
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	// Should append SHA to snapshots list
	require.Len(t, result.snapshots, 1)
	assert.Equal(t, "deadbeef1234567890abcdef1234567890abcdef", result.snapshots[0])

	// Should update the first commit's info (SHA truncated to 7 chars)
	assert.Equal(t, "deadbee", result.commits[0].Info.SHA)
	assert.Equal(t, "dfd: baseabc @ Feb 5 09:15", result.commits[0].Info.Subject)
	assert.Equal(t, "Feb 5 09:15", result.commits[0].Info.Date)
}

func TestSnapshotCreatedMsg_Error(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{
		{Info: sidebyside.CommitInfo{Subject: "diff"}, IsSnapshot: true},
	}, WithAutoSnapshots(true), WithShowSnapshots(true))

	msg := SnapshotCreatedMsg{
		Err: fmt.Errorf("git error"),
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	// Should disable snapshots
	assert.False(t, result.autoSnapshots)
	// Should not append anything to snapshots list
	assert.Empty(t, result.snapshots)
}

func TestSnapshotCreatedMsg_ShortSHA(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{
		{Info: sidebyside.CommitInfo{}, IsSnapshot: true},
	}, WithAutoSnapshots(true), WithShowSnapshots(true), WithGit(&git.MockGit{}))

	msg := SnapshotCreatedMsg{
		SHA:     "abc12", // shorter than 7 chars
		Subject: "short",
		Date:    "Feb 5 09:15",
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	// SHA should be used as-is (no panic on short string)
	assert.Equal(t, "abc12", result.commits[0].Info.SHA)
}

func TestSnapshotCreatedMsg_SkipsUpdateWhenSHAAlreadySet(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{
		{
			Info:       sidebyside.CommitInfo{SHA: "existing", Subject: "original subject"},
			IsSnapshot: true,
		},
	}, WithAutoSnapshots(true), WithShowSnapshots(true), WithGit(&git.MockGit{}))

	msg := SnapshotCreatedMsg{
		SHA:     "newsha1234567890",
		Subject: "new subject",
		Date:    "Feb 6 10:00",
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	// SHA was already set, so the commit info should not be updated
	assert.Equal(t, "existing", result.commits[0].Info.SHA)
	assert.Equal(t, "original subject", result.commits[0].Info.Subject)
	// But the snapshot list should still grow
	require.Len(t, result.snapshots, 1)
}

// =============================================================================
// handleSnapshot (R key) tests
// =============================================================================

func TestHandleSnapshot_DisabledShowsStatus(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = false
	m.keys = DefaultKeyMap()

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	result := newModel.(Model)

	assert.Equal(t, "Snapshots not available (no working tree changes)", result.statusMessage)
	assert.NotNil(t, cmd, "should return clear-status command")
}

func TestHandleSnapshot_NoInitialSnapshotShowsStatus(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = true
	m.snapshots = nil // no initial snapshot yet
	m.keys = DefaultKeyMap()

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	result := newModel.(Model)

	assert.Equal(t, "Initial snapshot not ready yet", result.statusMessage)
	assert.NotNil(t, cmd, "should return clear-status command")
}

func TestLoadCommitDiff_ShiftsHighlightMaps(t *testing.T) {
	// When loadCommitDiff produces a different file count than the skeleton,
	// file indices shift for subsequent commits. Highlight and structure maps
	// keyed by file index must be shifted to match.

	// C0: skeleton with 1 file, FilesLoaded false (will be loaded with 2 files → delta +1)
	c0 := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc1111"},
		Files:       []sidebyside.FilePair{sidebyside.SkeletonFilePairNoStats("alpha.go")},
		FoldLevel:   sidebyside.CommitFolded,
		FilesLoaded: false,
	}

	// C1: already loaded, 1 file with content and highlight data
	c1 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{SHA: "def2222"},
		Files: []sidebyside.FilePair{
			{
				OldPath: "a/beta.go", NewPath: "b/beta.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "package beta", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package beta", Type: sidebyside.Context},
					},
				},
				OldContent: []string{"package beta"},
				NewContent: []string{"package beta", "func Hello() {}"},
				FoldLevel:  sidebyside.FoldExpanded,
			},
		},
		FoldLevel:   sidebyside.CommitNormal,
		FilesLoaded: true,
	}

	// MockGit returns a diff with 2 files (skeleton had 1 → delta +1)
	diffOutput := "diff --git a/alpha.go b/alpha.go\n--- a/alpha.go\n+++ b/alpha.go\n@@ -1,3 +1,3 @@\n-old line\n+new line\ndiff --git a/gamma.go b/gamma.go\nnew file mode 100644\n--- /dev/null\n+++ b/gamma.go\n@@ -0,0 +1 @@\n+package gamma\n"
	mock := &git.MockGit{ShowOutput: diffOutput}

	m := NewWithCommits([]sidebyside.CommitSet{c0, c1}, WithGit(mock))
	m.width = 80
	m.height = 40

	// Verify initial layout: C0 files at [0], C1 files at [1]
	require.Equal(t, 2, len(m.files))
	require.Equal(t, 0, m.commitFileStarts[0])
	require.Equal(t, 1, m.commitFileStarts[1])

	// Pre-store highlight data for C1's file at index 1
	sentinelSpan := highlight.Span{Start: 0, End: 12, Category: highlight.CategoryKeyword}
	m.highlightSpans[1] = &FileHighlight{
		OldSpans: []highlight.Span{sentinelSpan},
		NewSpans: []highlight.Span{sentinelSpan},
	}
	m.pairsHighlightSpans[1] = &PairsFileHighlight{
		OldSpans:      []highlight.Span{sentinelSpan},
		OldLineStarts: map[int]int{1: 0},
		OldLineLens:   map[int]int{1: 12},
	}
	m.structureMaps[1] = &FileStructure{
		NewStructure: structure.NewMap([]structure.Entry{{StartLine: 1, EndLine: 2, Name: "Hello", Kind: "function"}}),
	}
	now := time.Now()
	m.loadingFiles[1] = now

	// Also store a comment for C1's file
	m.comments[commentKey{fileIndex: 1, newLineNum: 1}] = "test comment"

	// --- Act: load C0's diff (2 files replace 1 skeleton → delta +1) ---
	m.loadCommitDiff(0)

	// --- Assert: file layout shifted ---
	require.Equal(t, 3, len(m.files), "should have 3 files total (2 for C0, 1 for C1)")
	require.Equal(t, 2, m.commitFileStarts[1], "C1 file start should shift from 1 to 2")

	// C1's file is now at index 2. Highlight data must follow.
	assert.NotNil(t, m.highlightSpans[2], "highlightSpans for C1 should shift to index 2")
	assert.Nil(t, m.highlightSpans[1], "index 1 is now C0's second file, should have no highlight data")

	assert.NotNil(t, m.pairsHighlightSpans[2], "pairsHighlightSpans for C1 should shift to index 2")
	assert.Nil(t, m.pairsHighlightSpans[1], "pairsHighlightSpans at index 1 should be cleared")

	assert.NotNil(t, m.structureMaps[2], "structureMaps for C1 should shift to index 2")
	assert.Nil(t, m.structureMaps[1], "structureMaps at index 1 should be cleared")

	assert.Equal(t, now, m.loadingFiles[2], "loadingFiles for C1 should shift to index 2")
	_, hasOldLoading := m.loadingFiles[1]
	assert.False(t, hasOldLoading, "loadingFiles at index 1 should be cleared")

	// Comment should also shift
	assert.Equal(t, "test comment", m.comments[commentKey{fileIndex: 2, newLineNum: 1}],
		"comment for C1 should shift to fileIndex 2")
	_, hasOldComment := m.comments[commentKey{fileIndex: 1, newLineNum: 1}]
	assert.False(t, hasOldComment, "comment at old fileIndex 1 should be cleared")
}

func TestLoadCommitDiff_NegativeDelta_ShiftsCorrectly(t *testing.T) {
	// Skeleton has 3 files, actual diff has 1 → delta -2.
	// This commonly happens with merge commits where --name-only lists more
	// files than the combined diff actually shows.

	c0 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{SHA: "merge111"},
		Files: []sidebyside.FilePair{
			sidebyside.SkeletonFilePairNoStats("a.go"),
			sidebyside.SkeletonFilePairNoStats("b.go"),
			sidebyside.SkeletonFilePairNoStats("c.go"),
		},
		FoldLevel:   sidebyside.CommitFolded,
		FilesLoaded: false,
	}

	c1 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{SHA: "next222"},
		Files: []sidebyside.FilePair{
			{
				OldPath: "a/delta.go", NewPath: "b/delta.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "package delta", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package delta", Type: sidebyside.Context},
					},
				},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		FoldLevel:   sidebyside.CommitNormal,
		FilesLoaded: true,
	}

	// Diff has only 1 file (delta = 1 - 3 = -2)
	diffOutput := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n"
	mock := &git.MockGit{ShowOutput: diffOutput}

	m := NewWithCommits([]sidebyside.CommitSet{c0, c1}, WithGit(mock))
	m.width = 80
	m.height = 40

	// C1's file is at index 3
	require.Equal(t, 3, m.commitFileStarts[1])

	// Store highlight data for C1's file at index 3
	m.highlightSpans[3] = &FileHighlight{
		OldSpans: []highlight.Span{{Start: 0, End: 5, Category: highlight.CategoryKeyword}},
	}

	m.loadCommitDiff(0)

	// C1's file should now be at index 1 (3 + delta(-2) = 1)
	require.Equal(t, 1, m.commitFileStarts[1])
	require.Equal(t, 2, len(m.files))

	assert.NotNil(t, m.highlightSpans[1], "highlightSpans should shift from index 3 to 1")
	_, hasOld := m.highlightSpans[3]
	assert.False(t, hasOld, "old index 3 should no longer have highlight data")
}

func TestHandleSnapshot_NilGitReturnsNil(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = true
	m.snapshots = []string{"abc123"} // has initial snapshot
	m.git = nil
	m.keys = DefaultKeyMap()

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	result := newModel.(Model)

	// No status message, no command — just a no-op
	assert.Empty(t, result.statusMessage)
	assert.Nil(t, cmd)
}

// =============================================================================
// SnapshotToggle (S key) tests
// =============================================================================

func TestSnapshotToggle_DisabledWhenNoAutoSnapshots(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = false
	m.keys = DefaultKeyMap()

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result := newModel.(Model)

	assert.Equal(t, "Snapshots not enabled", result.statusMessage)
	assert.NotNil(t, cmd, "should return clear-status command")
	assert.False(t, result.showSnapshots)
}

func TestSnapshotToggle_BuildsHistoryAsync(t *testing.T) {
	// First S press (no cached snapshot view) should trigger an async history
	// build via buildSnapshotHistoryCmd. The normal view is cached so we can
	// restore it when toggling back.
	baseCommit := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{Subject: "base diff"},
		Files: []sidebyside.FilePair{{NewPath: "base.go"}},
	}

	m := NewWithCommits([]sidebyside.CommitSet{baseCommit},
		WithAutoSnapshots(true),
	)
	m.keys = DefaultKeyMap()

	assert.False(t, m.showSnapshots)

	// Press S to toggle on
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result := newModel.(Model)

	assert.True(t, result.showSnapshots)
	// Normal view should be cached for later restoration
	require.Len(t, result.normalViewCommits, 1)
	assert.Equal(t, "base diff", result.normalViewCommits[0].Info.Subject)
	// Original commits are unchanged (async SnapshotHistoryReadyMsg will swap them)
	assert.False(t, result.commits[0].IsSnapshot)
}

func TestSnapshotToggle_SwitchesToCachedSnapshotView(t *testing.T) {
	// When snapshotViewCommits is cached, S should swap to it immediately
	baseCommit := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{Subject: "base diff"},
		Files: []sidebyside.FilePair{{NewPath: "base.go"}},
	}
	snapshotCommit := sidebyside.CommitSet{
		Info:       sidebyside.CommitInfo{Subject: "snapshot diff"},
		Files:      []sidebyside.FilePair{{NewPath: "snap.go"}},
		IsSnapshot: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{baseCommit},
		WithAutoSnapshots(true),
		WithSnapshotViewCommits([]sidebyside.CommitSet{snapshotCommit}),
	)
	m.keys = DefaultKeyMap()

	assert.False(t, m.showSnapshots)
	assert.Equal(t, "base.go", m.files[0].NewPath)

	// Press S to toggle to snapshot view
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result := newModel.(Model)

	assert.True(t, result.showSnapshots)
	require.Len(t, result.commits, 1)
	assert.Equal(t, "snapshot diff", result.commits[0].Info.Subject)
	assert.Equal(t, "snap.go", result.files[0].NewPath)
}

func TestSnapshotToggle_SwitchesBack(t *testing.T) {
	baseCommit := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{Subject: "base diff"},
		Files: []sidebyside.FilePair{{NewPath: "base.go"}},
	}
	snapshotCommit := sidebyside.CommitSet{
		Info:       sidebyside.CommitInfo{Subject: "snapshot diff"},
		Files:      []sidebyside.FilePair{{NewPath: "snap.go"}},
		IsSnapshot: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{baseCommit},
		WithAutoSnapshots(true),
		WithSnapshotViewCommits([]sidebyside.CommitSet{snapshotCommit}),
	)
	m.keys = DefaultKeyMap()

	// Toggle on
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result := newModel.(Model)
	assert.True(t, result.showSnapshots)

	// Toggle off
	newModel2, _ := result.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result2 := newModel2.(Model)
	assert.False(t, result2.showSnapshots)
	require.Len(t, result2.commits, 1)
	assert.Equal(t, "base diff", result2.commits[0].Info.Subject)
	assert.Equal(t, "base.go", result2.files[0].NewPath)
}

func TestSnapshotToggle_StartInSnapshotView(t *testing.T) {
	// When the model starts with showSnapshots=true (e.g. from config or --snapshots),
	// toggling OFF should swap to the base→WT view, not stay on snapshot data.
	baseCommit := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{Subject: "base diff"},
		Files: []sidebyside.FilePair{{NewPath: "base.go"}},
	}
	snapshotCommit := sidebyside.CommitSet{
		Info:       sidebyside.CommitInfo{Subject: "snapshot diff"},
		Files:      []sidebyside.FilePair{{NewPath: "snap.go"}},
		IsSnapshot: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{baseCommit},
		WithAutoSnapshots(true),
		WithShowSnapshots(true),
		WithSnapshotViewCommits([]sidebyside.CommitSet{snapshotCommit}),
	)
	m.keys = DefaultKeyMap()
	// Init should have swapped to snapshot view and cached normal view
	m.Init()

	assert.True(t, m.showSnapshots)
	assert.Equal(t, "snap.go", m.files[0].NewPath, "should start in snapshot view")
	require.NotNil(t, m.normalViewCommits, "normal view should be cached at init")
	assert.Equal(t, "base diff", m.normalViewCommits[0].Info.Subject)

	// First S press: toggle OFF → should restore base→WT view
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result := newModel.(Model)
	assert.False(t, result.showSnapshots)
	assert.Equal(t, "base.go", result.files[0].NewPath, "should swap to normal view")

	// Second S press: toggle ON → should restore snapshot view
	newModel2, _ := result.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result2 := newModel2.(Model)
	assert.True(t, result2.showSnapshots)
	assert.Equal(t, "snap.go", result2.files[0].NewPath, "should swap back to snapshot view")
}

// =============================================================================
// End-to-end snapshot toggle test (S key → async build → message → view swap)
// =============================================================================

func TestSnapshotToggle_EndToEnd_BuildsAndSwapsView(t *testing.T) {
	// Simulates the real user flow: user runs dfd, snapshots are taken,
	// user presses S, async command builds snapshot timeline, message arrives,
	// view swaps to show snapshot commits with different diff content.
	simpleDiff := "diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n line1\n-old\n+new\n line3\n"

	baseCommit := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{Subject: "base diff"},
		Files: []sidebyside.FilePair{{NewPath: "base.go"}},
	}

	m := NewWithCommits([]sidebyside.CommitSet{baseCommit},
		WithAutoSnapshots(true),
		WithBaseSHA("aaaa"),
		WithBranch("main"),
		WithPersistedSnapshots([]string{"bbbb"}),
	)
	m.keys = DefaultKeyMap()
	m.git = &git.MockGit{DiffOutput: simpleDiff}

	// Verify initial state
	assert.False(t, m.showSnapshots)
	assert.Equal(t, "base.go", m.files[0].NewPath)

	// Step 1: Press S to toggle on snapshot view
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	result := newModel.(Model)

	assert.True(t, result.showSnapshots, "showSnapshots should be set immediately")
	require.NotNil(t, cmd, "should return async command from buildSnapshotHistoryCmd")

	// Normal view should be cached
	require.Len(t, result.normalViewCommits, 1)
	assert.Equal(t, "base diff", result.normalViewCommits[0].Info.Subject)

	// Step 2: Execute the async command (simulates Bubble Tea runtime)
	msg := cmd()
	readyMsg, ok := msg.(SnapshotHistoryReadyMsg)
	require.True(t, ok, "command should return SnapshotHistoryReadyMsg, got %T", msg)
	assert.NoError(t, readyMsg.Err)
	assert.NotEmpty(t, readyMsg.Commits, "should have snapshot commits")

	// Step 3: Feed the message back through Update
	newModel2, _ := result.Update(readyMsg)
	result2 := newModel2.(Model)

	assert.True(t, result2.showSnapshots, "should still be in snapshot view")
	assert.NotNil(t, result2.snapshotViewCommits, "should cache snapshot view")

	// The commits should be snapshot commits, not the original base commit
	for _, c := range result2.commits {
		assert.True(t, c.IsSnapshot, "all commits in snapshot view should be IsSnapshot")
	}

	// Files should have changed from the base view
	assert.NotEqual(t, "base.go", result2.files[0].NewPath,
		"files should reflect snapshot diff content, not the original base diff")
}

// =============================================================================
// SnapshotCreatedSilentMsg tests
// =============================================================================

func TestSnapshotCreatedSilentMsg_Success(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = true
	m.baseSHA = "base123"
	m.git = &git.MockGit{}

	msg := SnapshotCreatedSilentMsg{
		SHA:     "newsnap1234567890",
		Subject: "dfd: base12 @ Feb 5 09:15",
		Date:    "Feb 5 09:15",
	}

	newModel, cmd := m.Update(msg)
	result := newModel.(Model)

	require.Len(t, result.snapshots, 1)
	assert.Equal(t, "newsnap1234567890", result.snapshots[0])
	assert.Equal(t, "Snapshot taken", result.statusMessage)
	assert.NotNil(t, cmd)
	// Snapshot view cache should be invalidated
	assert.Nil(t, result.snapshotViewCommits)
}

func TestSnapshotCreatedSilentMsg_Error(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = true

	msg := SnapshotCreatedSilentMsg{
		Err: fmt.Errorf("snapshot failed"),
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	assert.False(t, result.autoSnapshots, "should disable snapshots on error")
	assert.Empty(t, result.snapshots)
}

// =============================================================================
// swapToView tests
// =============================================================================

func TestSwapToView_RebuildsState(t *testing.T) {
	m := makeTestModel(10)
	m.highlightSpans[0] = &FileHighlight{}
	m.highlightSpans[1] = &FileHighlight{}

	newCommits := []sidebyside.CommitSet{
		{
			Info:  sidebyside.CommitInfo{Subject: "new view"},
			Files: []sidebyside.FilePair{{NewPath: "new1.go"}, {NewPath: "new2.go"}},
		},
	}

	m.swapToView(newCommits)

	require.Len(t, m.commits, 1)
	assert.Equal(t, "new view", m.commits[0].Info.Subject)
	require.Len(t, m.files, 2)
	assert.Equal(t, "new1.go", m.files[0].NewPath)
	assert.Equal(t, "new2.go", m.files[1].NewPath)
	require.Len(t, m.commitFileStarts, 1)
	assert.Equal(t, 0, m.commitFileStarts[0])

	// Old highlight cache should be cleared
	assert.Empty(t, m.highlightSpans)
	assert.Empty(t, m.inlineDiffCache)
}

// =============================================================================
// buildSnapshotHistoryCmd fallback tests
// =============================================================================

func TestBuildSnapshotHistoryCmd_FallsBackToInMemorySnapshots(t *testing.T) {
	// MockGit.ListSnapshotRefs returns nil by default, simulating
	// a ref that was never persisted. With in-memory snapshots, the
	// function should fall back to those SHAs.
	simpleDiff := "diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n line1\n-old\n+new\n line3\n"

	m := makeTestModel(10)
	m.autoSnapshots = true
	m.baseSHA = "aaaa"
	m.branch = "main"
	m.snapshots = []string{"bbbb"}
	m.git = &git.MockGit{DiffOutput: simpleDiff}

	cmd := m.buildSnapshotHistoryCmd()
	require.NotNil(t, cmd)

	msg := cmd()
	readyMsg, ok := msg.(SnapshotHistoryReadyMsg)
	require.True(t, ok)
	assert.NoError(t, readyMsg.Err)
	assert.NotEmpty(t, readyMsg.Commits, "should build history from in-memory snapshots")
}

func TestBuildSnapshotHistoryCmd_EmptyWhenNoSnapshots(t *testing.T) {
	// Neither ListSnapshotRefs nor in-memory snapshots have entries.
	m := makeTestModel(10)
	m.autoSnapshots = true
	m.baseSHA = "aaaa"
	m.branch = "main"
	m.snapshots = nil
	m.git = &git.MockGit{}

	cmd := m.buildSnapshotHistoryCmd()
	require.NotNil(t, cmd)

	msg := cmd()
	readyMsg, ok := msg.(SnapshotHistoryReadyMsg)
	require.True(t, ok)
	assert.Empty(t, readyMsg.Commits, "should return empty when no snapshots exist anywhere")
}

// =============================================================================
// SnapshotHistoryReadyMsg handler tests
// =============================================================================

func TestSnapshotHistoryReadyMsg_SwapsView(t *testing.T) {
	baseCommit := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{Subject: "base diff"},
		Files: []sidebyside.FilePair{{NewPath: "base.go"}},
	}
	m := NewWithCommits([]sidebyside.CommitSet{baseCommit},
		WithAutoSnapshots(true),
	)
	m.showSnapshots = true // already toggled on, waiting for history

	snapshotCommits := []sidebyside.CommitSet{
		{
			Info:       sidebyside.CommitInfo{Subject: "Working tree changes"},
			Files:      []sidebyside.FilePair{{NewPath: "wt.go"}},
			IsSnapshot: true,
		},
	}
	newModel, _ := m.Update(SnapshotHistoryReadyMsg{Commits: snapshotCommits})
	result := newModel.(Model)

	assert.True(t, result.showSnapshots)
	require.Len(t, result.commits, 1)
	assert.Equal(t, "Working tree changes", result.commits[0].Info.Subject)
	assert.Equal(t, "wt.go", result.files[0].NewPath)
	assert.NotNil(t, result.snapshotViewCommits)
}

func TestSnapshotHistoryReadyMsg_EmptyResetsToggle(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = true
	m.showSnapshots = true // was toggled on

	newModel, _ := m.Update(SnapshotHistoryReadyMsg{})
	result := newModel.(Model)

	assert.False(t, result.showSnapshots, "should reset toggle on empty history")
	assert.Equal(t, "No snapshot history", result.statusMessage)
}

func TestSnapshotHistoryReadyMsg_ErrorResetsToggle(t *testing.T) {
	m := makeTestModel(10)
	m.autoSnapshots = true
	m.showSnapshots = true

	newModel, _ := m.Update(SnapshotHistoryReadyMsg{Err: fmt.Errorf("git error")})
	result := newModel.(Model)

	assert.False(t, result.showSnapshots, "should reset toggle on error")
	assert.Equal(t, "Failed to load snapshot history", result.statusMessage)
}
