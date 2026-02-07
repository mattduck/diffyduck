package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestVisualMode_EnterWithV(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10

	// Initially not in visual mode
	assert.False(t, m.w().visualSelection.Active)

	// Press V to enter visual mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	assert.True(t, model.w().visualSelection.Active, "should be in visual mode after pressing V")
	assert.Equal(t, 10, model.w().visualSelection.AnchorRow, "anchor should be set to scroll position")
}

func TestVisualMode_ExitWithEsc(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10
	m.w().visualSelection.Active = true
	m.w().visualSelection.AnchorRow = 10

	// Press ESC to exit visual mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := newM.(Model)

	assert.False(t, model.w().visualSelection.Active, "should exit visual mode on ESC")
}

func TestVisualMode_ExitWithCtrlG(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10
	m.w().visualSelection.Active = true
	m.w().visualSelection.AnchorRow = 10

	// Press C-g to exit visual mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	model := newM.(Model)

	assert.False(t, model.w().visualSelection.Active, "should exit visual mode on C-g")
}

func TestVisualMode_MovementExtendsSelection(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10

	// Enter visual mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	assert.True(t, model.w().visualSelection.Active)
	assert.Equal(t, 10, model.w().visualSelection.AnchorRow)

	// Move down with j
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newM.(Model)

	// Should still be in visual mode, anchor unchanged, scroll moved
	assert.True(t, model.w().visualSelection.Active, "should remain in visual mode after movement")
	assert.Equal(t, 10, model.w().visualSelection.AnchorRow, "anchor should not change on movement")
	assert.Equal(t, 11, model.w().scroll, "scroll should move down")
}

func TestVisualMode_MovementUp(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 20

	// Enter visual mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	// Move up with k
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = newM.(Model)

	assert.True(t, model.w().visualSelection.Active)
	assert.Equal(t, 20, model.w().visualSelection.AnchorRow, "anchor unchanged")
	assert.Equal(t, 19, model.w().scroll, "scroll moved up")
}

func TestVisualMode_PageMovement(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10

	// Enter visual mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	// Page down
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = newM.(Model)

	assert.True(t, model.w().visualSelection.Active)
	assert.Equal(t, 10, model.w().visualSelection.AnchorRow, "anchor unchanged")
	assert.Equal(t, 30, model.w().scroll, "scroll moved by page")
}

func TestVisualMode_QuitWorksInVisualMode(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10
	m.w().visualSelection.Active = true
	m.w().visualSelection.AnchorRow = 10

	// Press q to quit - should work even in visual mode
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// Quit command should be issued (visual mode state doesn't matter since we're exiting)
	assert.NotNil(t, cmd, "should return quit command")
}

func TestVisualMode_StatusBarIndicator(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line"}, New: sidebyside.Line{Num: 1, Content: "line"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40
	m.focused = true

	// Not in visual mode - view should not contain <VISUAL>
	view := m.View()
	assert.NotContains(t, view, "<VISUAL>", "should not show <VISUAL> indicator when not in visual mode")

	// Enter visual mode
	m.w().visualSelection.Active = true
	m.w().visualSelection.AnchorRow = 0

	// In visual mode - view should contain <VISUAL>
	view = m.View()
	assert.Contains(t, view, "<VISUAL>", "should show <VISUAL> indicator when in visual mode")
}

func TestVisualMode_PerWindowIndependence(t *testing.T) {
	m := makeTestModel(100)
	m.width = 160 // wide enough for split
	m.height = 40

	// Create a second window
	newM, _ := m.windowSplitVertical()
	m = newM.(Model)
	assert.Equal(t, 2, len(m.windows), "should have 2 windows")

	// Enter visual mode in first window
	m.activeWindowIdx = 0
	m.w().scroll = 10
	m.w().visualSelection.Active = true
	m.w().visualSelection.AnchorRow = 10

	// Second window should not be in visual mode
	assert.True(t, m.windows[0].visualSelection.Active, "first window should be in visual mode")
	assert.False(t, m.windows[1].visualSelection.Active, "second window should not be in visual mode")

	// Enter visual mode in second window with different anchor
	m.activeWindowIdx = 1
	m.w().scroll = 20
	m.w().visualSelection.Active = true
	m.w().visualSelection.AnchorRow = 20

	// Both windows have independent visual selection
	assert.True(t, m.windows[0].visualSelection.Active)
	assert.Equal(t, 10, m.windows[0].visualSelection.AnchorRow)
	assert.True(t, m.windows[1].visualSelection.Active)
	assert.Equal(t, 20, m.windows[1].visualSelection.AnchorRow)
}

func TestVisualMode_SelectionRangeCalculation(t *testing.T) {
	tests := []struct {
		name          string
		anchorRow     int
		scrollPos     int
		expectedStart int
		expectedEnd   int
	}{
		{
			name:          "anchor before scroll",
			anchorRow:     5,
			scrollPos:     15,
			expectedStart: 5,
			expectedEnd:   15,
		},
		{
			name:          "anchor after scroll",
			anchorRow:     20,
			scrollPos:     10,
			expectedStart: 10,
			expectedEnd:   20,
		},
		{
			name:          "anchor equals scroll",
			anchorRow:     10,
			scrollPos:     10,
			expectedStart: 10,
			expectedEnd:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(100)
			m.w().visualSelection.Active = true
			m.w().visualSelection.AnchorRow = tt.anchorRow
			m.w().scroll = tt.scrollPos

			// Calculate range as done in view.go
			anchor := m.w().visualSelection.AnchorRow
			current := m.w().scroll
			var start, end int
			if anchor <= current {
				start, end = anchor, current
			} else {
				start, end = current, anchor
			}

			assert.Equal(t, tt.expectedStart, start, "selection start")
			assert.Equal(t, tt.expectedEnd, end, "selection end")
		})
	}
}

func TestVisualYank_CopiesSelectedLines(t *testing.T) {
	m := makeTestModel(20)
	clip := &MemoryClipboard{}
	m.clipboard = clip

	// Enter visual mode at row 5
	m.w().scroll = 5
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	// Move down 2 rows
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newM.(Model)

	assert.True(t, model.w().visualSelection.Active)
	assert.Equal(t, 5, model.w().visualSelection.AnchorRow)
	assert.Equal(t, 7, model.w().scroll)

	// Press y to yank
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = newM.(Model)

	// Visual mode should be exited
	assert.False(t, model.w().visualSelection.Active, "should exit visual mode after yank")

	// Clipboard should have content (3 lines for rows 5-7)
	assert.NotEmpty(t, clip.Content, "clipboard should have content")
	lines := strings.Split(clip.Content, "\n")
	assert.Equal(t, 3, len(lines), "should have 3 lines (rows 5-7)")

	// Status message should indicate copied lines
	assert.Contains(t, model.statusMessage, "3 lines")
}

func TestVisualYank_SingleLine(t *testing.T) {
	m := makeTestModel(20)
	clip := &MemoryClipboard{}
	m.clipboard = clip

	// Enter visual mode without moving (single line)
	m.w().scroll = 5
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	// Yank immediately (no movement)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = newM.(Model)

	assert.False(t, model.w().visualSelection.Active, "should exit visual mode")
	assert.NotEmpty(t, clip.Content, "clipboard should have content")
	// Single line: no newline splits
	lines := strings.Split(clip.Content, "\n")
	assert.Equal(t, 1, len(lines), "should have 1 line")
	assert.Contains(t, model.statusMessage, "1 lines")
}

func TestVisualYank_ReverseSelection(t *testing.T) {
	m := makeTestModel(20)
	clip := &MemoryClipboard{}
	m.clipboard = clip

	// Enter visual mode at row 10
	m.w().scroll = 10
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	// Move UP 3 rows (reverse selection: anchor=10, current=7)
	for i := 0; i < 3; i++ {
		newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
		model = newM.(Model)
	}

	assert.Equal(t, 10, model.w().visualSelection.AnchorRow)
	assert.Equal(t, 7, model.w().scroll)

	// Yank
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = newM.(Model)

	assert.False(t, model.w().visualSelection.Active, "should exit visual mode")
	lines := strings.Split(clip.Content, "\n")
	assert.Equal(t, 4, len(lines), "should have 4 lines (rows 7-10)")
	assert.Contains(t, model.statusMessage, "4 lines")
}

func TestVisualMode_DoesNotInterfereWithSearchMode(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10

	// Enter search mode first
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model := newM.(Model)

	assert.True(t, model.searchMode, "should be in search mode")

	// V in search mode should type V, not enter visual mode
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model = newM.(Model)

	assert.True(t, model.searchMode, "should still be in search mode")
	assert.False(t, model.w().visualSelection.Active, "should not enter visual mode while in search mode")
	assert.Equal(t, "V", model.searchInput, "V should be typed into search input")
}

func TestVisualMode_DoesNotInterfereWithCommentMode(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
				},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Move to content row and enter comment mode
	m.w().scroll = 2 // should be on a content row
	m.startComment()

	if m.w().commentMode {
		// V in comment mode should type V, not enter visual mode
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
		model := newM.(Model)

		assert.True(t, model.w().commentMode, "should still be in comment mode")
		assert.False(t, model.w().visualSelection.Active, "should not enter visual mode while in comment mode")
	}
}

func TestVisualMode_VWhileAlreadyInVisualMode(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 10
	m.w().visualSelection.Active = true
	m.w().visualSelection.AnchorRow = 5

	// Press V again while in visual mode - should reset anchor to current position
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	model := newM.(Model)

	assert.True(t, model.w().visualSelection.Active, "should remain in visual mode")
	assert.Equal(t, 10, model.w().visualSelection.AnchorRow, "anchor should be reset to current scroll")
}
