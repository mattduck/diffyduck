package tui

import (
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/content"
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

	// Scroll clamps at 0 (no negative scroll in new model)
	assert.Equal(t, 0, model.scroll)
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
	assert.Equal(t, m.minScroll(), m.scroll, "should be at top")

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

func TestUpdate_MouseWheelDown(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	model := newM.(Model)

	assert.Equal(t, 3, model.scroll) // scrolls 3 lines
}

func TestUpdate_MouseWheelUp(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 10

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	model := newM.(Model)

	assert.Equal(t, 7, model.scroll) // 10 - 3
}

func TestUpdate_MouseWheelUp_AtTop(t *testing.T) {
	m := makeTestModel(100)
	m.scroll = 0

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	model := newM.(Model)

	// Scroll clamps at 0 (no negative scroll in new model)
	assert.Equal(t, 0, model.scroll)
}

func TestUpdate_MouseWheelDown_AtBottom(t *testing.T) {
	m := makeTestModel(30)
	m.scroll = m.maxScroll()

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	model := newM.(Model)

	assert.Equal(t, m.maxScroll(), model.scroll) // clamped to max
}

func TestUpdate_FoldToggle_SingleFile(t *testing.T) {
	m := makeTestModel(10)
	// Initially at FoldExpanded (set by makeTestModel)
	assert.Equal(t, sidebyside.FoldExpanded, m.files[0].FoldLevel)

	// Position cursor on file header (line 0 in diff view: no top border)
	m.scroll = 0

	// Press Tab to cycle to next level: Expanded -> Folded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel)

	// Press Tab again to cycle to Normal (structural diff)
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)

	assert.Equal(t, sidebyside.FoldNormal, model2.files[0].FoldLevel)

	// Press Tab again to cycle back to Expanded (hunks)
	newM3, _ := model2.Update(tea.KeyMsg{Type: tea.KeyTab})
	model3 := newM3.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, model3.files[0].FoldLevel)
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
	// When expanding from FoldNormal to FoldExpanded, should return a fetch command
	m := makeTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldNormal // start at structural diff
	m.calculateTotalLines()

	// Position cursor on file header (line 0 in diff view)
	m.scroll = 0

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

	// Position cursor on file header (line 0 in diff view)
	m.scroll = 0

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

func TestUpdate_PagerMode_FoldToggle_SkipsNormal(t *testing.T) {
	m := makeTestModel(10)
	m.pagerMode = true // Enable pager mode

	// Position cursor on file header (line 0 in diff view)
	m.scroll = 0

	// Initially at FoldExpanded (hunks)
	assert.Equal(t, sidebyside.FoldExpanded, m.files[0].FoldLevel)

	// Press Tab - should go to FoldFolded (skipping FoldNormal in pager mode)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel,
		"pager mode should go to FoldFolded")

	// Press Tab again - should skip FoldNormal and go back to FoldExpanded
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)

	assert.Equal(t, sidebyside.FoldExpanded, model2.files[0].FoldLevel,
		"pager mode should skip FoldNormal and cycle back to FoldExpanded")
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
	// Verify normal (non-pager) mode cycles through all levels including FoldNormal
	m := makeTestModel(10)
	// pagerMode is false by default

	// Position cursor on file header (line 0 in diff view)
	m.scroll = 0

	// Initially at FoldExpanded (hunks)
	assert.Equal(t, sidebyside.FoldExpanded, m.files[0].FoldLevel)

	// Press Tab - should go to FoldFolded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel,
		"normal mode should go to FoldFolded")

	// Press Tab again - should go to FoldNormal (structural diff)
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)

	assert.Equal(t, sidebyside.FoldNormal, model2.files[0].FoldLevel,
		"normal mode should include FoldNormal in cycle")
}

func TestNextFoldLevel_PagerMode(t *testing.T) {
	m := Model{pagerMode: true}

	// Expanded -> Folded
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevel(sidebyside.FoldExpanded))

	// Folded -> Expanded (skip Normal, since no structural diff in pager mode)
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevel(sidebyside.FoldFolded))

	// Normal -> Expanded (Normal shouldn't be reached in pager mode, but if it is, proceed normally)
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevel(sidebyside.FoldNormal))
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

	// Binary file should skip FoldNormal (no structural diff for binary files)
	binaryFile := sidebyside.FilePair{
		OldPath:   "a/image.png",
		NewPath:   "b/image.png",
		IsBinary:  true,
		FoldLevel: sidebyside.FoldExpanded,
	}

	// Expanded -> Folded
	assert.Equal(t, sidebyside.FoldFolded, m.nextFoldLevelForFile(binaryFile))

	// Folded -> Expanded (skip Normal)
	binaryFile.FoldLevel = sidebyside.FoldFolded
	assert.Equal(t, sidebyside.FoldExpanded, m.nextFoldLevelForFile(binaryFile))
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
	m.scroll = rowIdx
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
	m.scroll = 1
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
	require.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel)
	require.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel)

	// Position cursor on second commit header
	m.scroll = 1
	m.calculateTotalLines()
	initialScroll := m.scroll

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
	assert.Equal(t, initialScroll, model.scroll,
		"scroll should be unchanged when loading content for folded commit")
}

func TestFileContentLoaded_SkipsScrollPreservation_WhenFileNotExpanded(t *testing.T) {
	m := createTwoCommitModelForIdentityTests()

	// Expand first commit but keep file folded
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.files[0].FoldLevel = sidebyside.FoldFolded // file is folded, not expanded
	m.calculateTotalLines()

	// Position cursor somewhere
	m.scroll = 0
	initialScroll := m.scroll

	// Simulate file content loading
	msg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: []string{"line1", "line2", "line3"},
		NewContent: []string{"line1", "line2", "line3"},
	}

	newM, _ := m.Update(msg)
	model := newM.(Model)

	// Scroll should be unchanged because file is not in FoldExpanded mode
	assert.Equal(t, initialScroll, model.scroll,
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

	rows := m.cachedRows

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
	m.scroll = hunk1ContentIdx
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
	assert.Equal(t, sidebyside.FoldExpanded, m.files[0].FoldLevel, "FoldLevel should remain FoldExpanded")

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

	assert.Equal(t, sidebyside.FoldExpanded, m.files[0].FoldLevel, "should auto-expand to FoldExpanded")
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
	m.scroll = sepTopIdx

	// Toggle full-file view
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	// Cursor should be on new-side line 3 (last content line above separator)
	newRows := m.buildRows()
	cursorRow := newRows[m.scroll]
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

	m.scroll = sepBottomIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	newRows := m.buildRows()
	cursorRow := newRows[m.scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should be on a content row")
	assert.Equal(t, 10, cursorRow.pair.New.Num, "cursor should be on line 10 (first line below separator)")
}

func TestFullFileToggle_FirstSeparatorTop_GoesToLineBelow(t *testing.T) {
	// First hunk starts at line 5 — the separator at the top has no content above it.
	// SeparatorTop should fall through to line below (like middle separator without breadcrumb).
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 5, Content: "e", Type: sidebyside.Context}, New: sidebyside.Line{Num: 5, Content: "e", Type: sidebyside.Context}},
					{Old: sidebyside.Line{Num: 6, Content: "f", Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Content: "f", Type: sidebyside.Context}},
				},
				OldContent: []string{"a", "b", "c", "d", "e", "f"},
				NewContent: []string{"a", "b", "c", "d", "e", "f"},
			},
		},
		fetcher: content.NewFetcher(nil, content.ModeShow, "abc", ""),
		width:   80,
		height:  40,
		keys:    DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// First hunk starts at line 5, so there's a separator at the top with no content above
	rows := m.buildRows()
	sepTopIdx := -1
	for i, row := range rows {
		if row.kind == RowKindSeparatorTop && row.fileIndex == 0 {
			sepTopIdx = i
			break
		}
	}
	require.True(t, sepTopIdx >= 0, "should have a SeparatorTop row")

	m.scroll = sepTopIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	newRows := m.buildRows()
	cursorRow := newRows[m.scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should be on a content row")
	assert.Equal(t, 5, cursorRow.pair.New.Num, "cursor should land on line 5 (first content line below, since no content above)")
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

	m.scroll = sepIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	newRows := m.buildRows()
	cursorRow := newRows[m.scroll]
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

	m.scroll = sepIdx

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	newRows := m.buildRows()
	cursorRow := newRows[m.scroll]
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
			m.scroll = i
			break
		}
	}

	// Toggle OFF full-file view
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	m = result.(Model)

	assert.False(t, m.files[0].ShowFullFile)
	newRows := m.buildRows()
	cursorRow := newRows[m.scroll]
	assert.Equal(t, RowKindContent, cursorRow.kind, "cursor should still be on a content row")
	assert.Equal(t, 3, cursorRow.pair.New.Num, "cursor should stay on line 3 after toggling off")
}
