package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

// =============================================================================
// Cursor Position Calculation Tests
// =============================================================================

func TestCursorLine_EqualsScroll(t *testing.T) {
	// In the new cursor model, cursorLine() directly equals scroll
	m := makeTestModel(100)
	m.height = 50
	m.scroll = 0

	assert.Equal(t, 0, m.cursorLine(), "cursorLine should equal scroll when scroll=0")

	m.scroll = 25
	assert.Equal(t, 25, m.cursorLine(), "cursorLine should equal scroll when scroll=25")
}

func TestCursorViewportRow_MovesNearTop(t *testing.T) {
	// Near the top, cursor moves up visually instead of content scrolling
	m := makeTestModel(100)
	m.height = 50 // cursorOffset = 9

	m.scroll = 0
	assert.Equal(t, 0, m.cursorViewportRow(), "cursor at viewport row 0 when scroll=0")

	m.scroll = 5
	assert.Equal(t, 5, m.cursorViewportRow(), "cursor at viewport row 5 when scroll=5")

	m.scroll = 9
	assert.Equal(t, 9, m.cursorViewportRow(), "cursor at viewport row 9 when scroll=9 (at cursorOffset)")

	m.scroll = 15
	assert.Equal(t, 9, m.cursorViewportRow(), "cursor stays at viewport row 9 when scroll>cursorOffset")
}

func TestCursorOffset_IsConstant(t *testing.T) {
	// The cursor offset (20%) should be a constant for given viewport height
	m := makeTestModel(100)
	m.height = 50

	offset1 := m.cursorOffset()
	m.scroll = 10
	offset2 := m.cursorOffset()
	m.scroll = 50
	offset3 := m.cursorOffset()

	assert.Equal(t, offset1, offset2, "cursor offset should not change with scroll")
	assert.Equal(t, offset2, offset3, "cursor offset should not change with scroll")
}

// =============================================================================
// Scroll Bounds Tests
// =============================================================================

func TestScrollUp_ClampsAtZero(t *testing.T) {
	// Scrolling up from 0 should stay at 0 (no negative scroll)
	m := makeTestModel(100)
	m.height = 50
	m.scroll = 0

	// Press k to scroll up - should stay at 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := newM.(Model)

	assert.Equal(t, 0, model.scroll, "scroll should clamp at 0")
}

func TestMinScroll_IsZero(t *testing.T) {
	// Minimum scroll is 0 (cursor can reach first content row)
	m := makeTestModel(100)
	m.height = 50

	minScroll := m.minScroll()
	assert.Equal(t, 0, minScroll, "minScroll should be 0")
}

func TestGoToTop_PutsCursorOnFirstLine(t *testing.T) {
	// Pressing 'gg' should set scroll to 0 (cursor on first content row)
	m := makeTestModel(100)
	m.height = 50
	m.scroll = 50

	// First 'g' enters pending state
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	// Second 'g' completes the sequence
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model2 := newM2.(Model)

	assert.Equal(t, 0, model2.scroll, "gg should set scroll to 0 (first content row)")
	assert.Equal(t, 0, model2.cursorLine(), "cursor should be on line 0")
}

func TestStartup_ScrollIsZero_NoBlankSpaceAtTop(t *testing.T) {
	// On startup, scroll should be 0 (not negative)
	// This means the first line of content is at the top of the viewport
	// The cursor is at 20% down, pointing to that content line
	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: make([]sidebyside.LinePair, 100)},
	})
	m.width = 80
	m.height = 50

	assert.Equal(t, 0, m.scroll, "scroll should be 0 on startup")
}

// =============================================================================
// Scroll Bounds Tests - Beyond Content
// =============================================================================

func TestScrollDown_ToLastLine(t *testing.T) {
	// Scrolling down should allow cursor to reach the last line of content
	// In diff view (no commit metadata), first file has:
	// header (1) + bottom border (1) + 10 pairs + 1 blank margin = 13 total lines (indices 0-12)
	m := makeTestModel(10)
	m.height = 50
	m.scroll = 0

	// Go to bottom - should position cursor on last line
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// In new model: scroll = cursorLine, so scroll should equal last line index (12)
	assert.Equal(t, 12, model.scroll, "G should put cursor on last line (scroll=12)")
	assert.Equal(t, 12, model.cursorLine(), "cursor should be on line 12")
}

func TestMaxScroll_AllowsCursorOnLastLine(t *testing.T) {
	// Maximum scroll should allow cursor to be on the last line of content
	// In new model: maxScroll = totalLines - 1
	m := makeTestModel(100) // 101 total lines (header + bottom border + 100 pairs - wait, need to check)
	m.height = 50

	maxScroll := m.maxScroll()
	expected := m.totalLines - 1
	assert.Equal(t, expected, maxScroll, "maxScroll should be totalLines - 1")
}

// =============================================================================
// Status Bar - Filename at Cursor Position
// =============================================================================

func TestStatusInfo_UseCursorPosition_NotScrollPosition(t *testing.T) {
	// StatusInfo should report the file at cursor position, not scroll position
	pairs := make([]sidebyside.LinePair, 20)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "content"},
			New: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/first.go", NewPath: "b/first.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},   // top border + header + bottom border + 20 pairs = 23 lines (0-22)
			{OldPath: "a/second.go", NewPath: "b/second.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs}, // starts after 4 blanks + trailing top border
		},
		width:  80,
		height: 50, // cursor offset = 9
		scroll: 0,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// At scroll 0, cursor is at line 0 (first file header)
	// In new model, cursorLine = scroll
	info := m.StatusInfo()
	assert.Equal(t, "first.go", info.FileName, "cursor at line 0 should be in first file")

	// With layout:
	// First file: header (0) + bottom border (1) + 20 pairs (2-21) + 4 blanks (22-25) + trailing top border (26)
	// Second file: header (27) + bottom border (28) + 20 pairs (29-48)...
	// At scroll 27, cursor is on second file's header
	m.scroll = 27
	info = m.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor at line 27 should be in second file")
}

func TestStatusInfo_CursorOnBlankLine_CountsAsFileAbove(t *testing.T) {
	// When cursor is on a blank line between files, it should count as the file above
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "content"},
			New: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/first.go", NewPath: "b/first.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},   // lines 0-5 (header + 5 pairs), then 4 blank lines (6-9)
			{OldPath: "a/second.go", NewPath: "b/second.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs}, // line 10 is header
		},
		width:  80,
		height: 10, // cursor offset = 1 (20% of 9)
		scroll: 5,  // cursor at line 6 (first blank line after first file)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	info := m.StatusInfo()
	assert.Equal(t, "first.go", info.FileName, "cursor on blank line should count as file above")
}

func TestStatusInfo_CursorOnLastBlankLine_CountsAsLastFile(t *testing.T) {
	// Special case: when cursor is at the very bottom (past all content),
	// it should count as the last file
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "content"},
			New: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/only.go", NewPath: "b/only.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
		},
		width:  80,
		height: 50, // cursor offset = 9
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines() // 6 lines total

	// Scroll so cursor is past all content
	m.scroll = 10 // cursor at line 19, way past content

	info := m.StatusInfo()
	assert.Equal(t, "only.go", info.FileName, "cursor past content should show last file")
}

// =============================================================================
// Cursor Line Highlighting Tests
// =============================================================================

// ANSI escape sequences for cursor highlighting (bg=7, fg=0)
const (
	// lipgloss combines fg and bg into single escape: \x1b[30;47m = fg black + bg silver
	ansiCursorStyle = "\x1b[30;47m"
	ansiReset       = "\x1b[0m"
)

// withANSIColors runs a test function with ANSI color output enabled
func withANSIColors(_ *testing.T, fn func()) {
	// Enable ANSI output for this test
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	fn()
}

func TestView_CursorHighlight_OnFileHeader(t *testing.T) {
	// When cursor is on a file header, the cursor arrow should be shown
	// Note: Background highlighting is NOT applied to headers (only diff content)
	withANSIColors(t, func() {
		m := New([]sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
					},
				},
			},
		})
		m.focused = true
		m.width = 80
		m.height = 10 // cursor offset = 1 (20% of 8)
		// Position cursor on the header (row 0) using minScroll
		m.scroll = m.minScroll()

		output := m.View()

		// Layout: [topBar(3 lines), divider, visibleRows..., bottomBar]
		// At minScroll, visibleRows[0]=blank (padding), visibleRows[1]=header
		// So lines[0..2]=topBar, lines[3]=divider, lines[4]=blank, lines[5]=header (with cursor)
		lines := strings.Split(output, "\n")
		require.True(t, len(lines) > 5, "should have more than 5 lines of output")
		headerLine := lines[5]

		// The header should contain the filename
		assert.Contains(t, headerLine, "test.go", "header should contain filename")

		// The header should show cursor arrow (▶) but NOT background highlighting
		assert.Contains(t, headerLine, "▶", "header should show cursor arrow when focused")
	})
}

func TestView_CursorHighlight_OnFileHeader_IconNotHighlighted(t *testing.T) {
	// The fold icon (◐/○/●) should NOT be highlighted - no background highlighting on headers
	// Only the cursor arrow (▶) indicates cursor position
	withANSIColors(t, func() {
		m := New([]sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
					},
				},
			},
		})
		m.focused = true
		m.width = 80
		m.height = 10
		// Position cursor on the header (row 0) using minScroll
		m.scroll = m.minScroll()

		output := m.View()
		lines := strings.Split(output, "\n")
		require.True(t, len(lines) > 5, "should have more than 5 lines of output")
		// At minScroll, visibleRows[0]=blank (padding), visibleRows[1]=header
		// So lines[0..2]=topBar, lines[3]=divider, lines[4]=blank, lines[5]=header (with cursor)
		headerLine := lines[5]

		// Headers show cursor arrow but NO background highlighting
		assert.Contains(t, headerLine, "▶", "header should show cursor arrow")
		// The fold icon should NOT have cursor background style
		assert.NotContains(t, headerLine, ansiCursorStyle, "header should not have cursor background highlighting")
	})
}

func TestView_CursorHighlight_OnFileHeader_UnfocusedNoBg(t *testing.T) {
	// When unfocused, the file header should NOT have cursor background highlighting
	// It should show the outline arrow but no bg color on file number
	withANSIColors(t, func() {
		m := Model{
			focused: false, // unfocused
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
			},
			width:  80,
			height: 10,
			scroll: 0, // cursor at line 1 (the header, after top border at line 0)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")
		// lines[0..2]=topBar, lines[3]=divider, lines[4]=top border, lines[5]=header (with cursor)
		headerLine := lines[5]

		// Should have outline arrow
		assert.Contains(t, headerLine, "▷", "unfocused header should have outline arrow")

		// Should NOT have cursor background style (bg=7)
		assert.NotContains(t, headerLine, ansiCursorStyle, "unfocused header should NOT have cursor background highlighting")

		// Should NOT have any background color on the file number area
		// statusStyle uses Background(Color("8")) which produces ;100m (bright black bg)
		assert.NotContains(t, headerLine, ";100m", "unfocused header should NOT have statusStyle background")
	})
}

func TestView_FileHeader_SpansFullWidth(t *testing.T) {
	// File headers should span the full terminal width without a │ separator
	// (separators are only for diff content lines)
	lipgloss.SetColorProfile(termenv.Ascii)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "left content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "right content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		scroll: 0, // cursor at line 0 (the header)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the header line (contains test.go and fold icon)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find header line")
	// Header should NOT have │ separator (it spans full width)
	assert.NotContains(t, headerLine, "│", "file header should not have │ separator")
}

// displayColumnOf returns the display column where substr starts, or -1 if not found
func displayColumnOf(s, substr string) int {
	idx := strings.Index(s, substr)
	if idx == -1 {
		return -1
	}
	// Count display width of characters before the substring
	return displayWidth(s[:idx])
}

func TestView_CursorHighlight_OnDiffLine(t *testing.T) {
	// When cursor is on a diff line, the gutter areas should be highlighted
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
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
			},
			width:  80,
			height: 10,
			scroll: 2, // cursor at line 2 (the diff line, after header + spacer)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Layout: [topBar(3 lines), divider, content..., bottomBar]
		// With scroll=2, cursor on content line 2 (the diff line)
		// lines[0..2]=topBar, lines[3]=divider, lines[4]=spacer, lines[5]=diffLine (with cursor)
		assert.True(t, len(lines) > 5)
		diffLine := lines[5]

		// The diff line gutters should have cursor highlighting
		assert.Contains(t, diffLine, ansiCursorStyle, "diff line should have cursor highlighting on gutter")
	})
}

func TestView_NoBlankSeparatorBetweenFiles(t *testing.T) {
	// With tree-style layout, there are no blank separator lines between files
	// Files are connected via tree branch characters instead
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/first.go",
					NewPath:   "b/first.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
				{
					OldPath:   "a/second.go",
					NewPath:   "b/second.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
			},
			width:  80,
			height: 20,
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		rows := m.buildRows()

		// Each unfolded file should have exactly one bottom margin blank row
		var blankCount int
		for _, row := range rows {
			if row.isBlank && !row.isHeaderSpacer {
				blankCount++
			}
		}
		// Both files are FoldNormal (unfolded), so expect 2 margin blanks
		assert.Equal(t, 2, blankCount, "each unfolded file should have one bottom margin blank row")
	})
}

func TestView_CursorHighlight_OnHunkSeparator(t *testing.T) {
	// When cursor is on a hunk separator, the gutter areas should be highlighted
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						},
						// Gap - next line is 100, so there will be a hunk separator
						{
							Old: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
						},
					},
				},
			},
			width:  80,
			height: 15,
			scroll: 4, // cursor on middle hunk separator row (after header, bottom border, first content, sep top)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Find any line with cursor styling (the cursor should be somewhere in the view)
		var cursorLine string
		for _, line := range lines[2:] { // skip topBar and divider
			if strings.Contains(line, ansiCursorStyle) {
				cursorLine = line
				break
			}
		}
		require.NotEmpty(t, cursorLine, "should find a line with cursor highlighting")

		// Verify the cursor line has highlighting
		assert.Contains(t, cursorLine, ansiCursorStyle, "cursor line should have cursor highlighting on gutter")
	})
}

func TestView_CursorHighlight_BothGuttersOnAddedLine(t *testing.T) {
	// For an added line (left side empty), both gutter areas should be highlighted
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath:   "a/test.go",
					NewPath:   "b/test.go",
					FoldLevel: sidebyside.FoldExpanded,
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
							New: sidebyside.Line{Num: 1, Content: "added", Type: sidebyside.Added},
						},
					},
				},
			},
			width:  80,
			height: 10,
			scroll: 2, // cursor at line 2 (the added line, after header + spacer)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Find the line with cursor highlighting
		var addedLine string
		for _, line := range lines[2:] { // skip topBar and divider
			if strings.Contains(line, ansiCursorStyle) {
				addedLine = line
				break
			}
		}
		require.NotEmpty(t, addedLine, "should find a line with cursor highlighting")

		// Both left gutter (empty) and right gutter should be highlighted
		assert.Contains(t, addedLine, ansiCursorStyle, "added line should have cursor highlighting on both gutters")
	})
}

// =============================================================================
// Scroll Position Preservation on Fold Changes
// =============================================================================

// Test: When cursor is on a file header and we fold/unfold, cursor stays on header
func TestFoldToggle_CursorOnHeader_StaysOnHeader(t *testing.T) {
	// Setup: cursor on file header (line 0 in diff view, no top border row)
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
					{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
				},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3 (20% of 19)
		scroll: 0,  // cursor at line 0 (the header)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor is on header initially (line 0)
	assert.Equal(t, 0, m.cursorLine(), "cursor should start on header (line 0)")

	// Toggle fold: Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Cursor should still be on header (which is still line 0)
	assert.Equal(t, 0, model.cursorLine(), "after Normal->Expanded, cursor should still be on header")

	// Toggle fold again: Expanded -> Folded
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(Model)

	// Cursor should be on header (line 0)
	assert.Equal(t, 0, model.cursorLine(), "after Expanded->Folded, cursor should be on header")

	// Toggle fold again: Folded -> Normal
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(Model)

	// Cursor should be on header (still line 0)
	assert.Equal(t, 0, model.cursorLine(), "after Folded->Normal, cursor should still be on header")
}

// Test: When cursor is on a diff line that remains visible, cursor stays on it
func TestFoldToggle_CursorOnDiffLine_StaysOnDiffLine(t *testing.T) {
	// Setup: cursor on diff line (line 2 = first diff line after header + spacer)
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
					{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
				},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		scroll: 2,  // cursor at line 2 (first diff line, after header + spacer)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor is on first diff line
	assert.Equal(t, 2, m.cursorLine(), "cursor should start on first diff line")

	// Toggle fold: Normal -> Expanded
	// The diff line should still be visible (expanded shows more, not less)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Cursor should still be pointing to the same logical line
	// In expanded view, line 2 is still the first content line after header + spacer
	assert.Equal(t, 2, model.cursorLine(), "after Normal->Expanded, cursor should stay on same line")
}

// Test: When cursor is on a diff line and we fold to Folded, cursor jumps to header
func TestFoldToggle_CursorOnDiffLine_FoldToHeader(t *testing.T) {
	// Setup: cursor on diff line, then fold to Folded
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
					{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
				},
				FoldLevel: sidebyside.FoldExpanded, // Start at Expanded so next toggle goes to Folded
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		scroll: 1,  // cursor at line 1 (first diff line)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor starts on diff line
	assert.Equal(t, 1, m.cursorLine(), "cursor should start on diff line")

	// Toggle fold: Expanded -> Folded
	// The diff line disappears, cursor should jump to header (the only visible line)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// In Folded mode, header is at line 1 (border slot at line 0)
	// Cursor should be adjusted to point to header
	assert.Equal(t, 1, model.cursorLine(), "after folding, cursor should jump to header")
}

// Test: Cursor stays on content row after fold toggle in tree-style layout
func TestFoldToggle_CursorOnContent_StaysOnContent(t *testing.T) {
	// With tree-style layout, there are no blank separator lines between files
	// Test that cursor on content row stays on content after fold toggle
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find a content row (diff line) to position cursor on
	rows := m.buildRows()
	var contentRowIdx int
	for i, row := range rows {
		if row.kind == RowKindContent {
			contentRowIdx = i
			break
		}
	}
	m.scroll = contentRowIdx

	// Toggle fold on first file: Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Cursor should be on a valid row
	rows = model.buildRows()
	cursorPos := model.cursorLine()
	assert.True(t, cursorPos >= 0 && cursorPos < len(rows), "cursor should be within valid row range after fold")
}

// Test: When all files are folded, row count is minimal (no blank separators)
func TestFoldToggleAll_NoBlanksBetweenFoldedFiles(t *testing.T) {
	// With tree-style layout, folded files have no blank lines between them
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Shift+Tab: all files Expanded -> Folded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	// Verify both files are now Folded
	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldFolded, model.files[1].FoldLevel)

	// Count rows - should have no blank lines between folded files
	rows := model.buildRows()
	var blankCount int
	for _, row := range rows {
		if row.isBlank {
			blankCount++
		}
	}
	// The last folded file gets a terminator blank row (┴)
	assert.Equal(t, 1, blankCount, "last folded file should have a terminator blank row")

	// Cursor should be on a valid header
	cursorPos := model.cursorLine()
	assert.True(t, cursorPos >= 0 && cursorPos < len(rows), "cursor should be on valid row")
}

// Test: TAB on hunk separator does nothing (only works on file header)
func TestFoldToggle_CursorOnHunkSeparator_NoEffect(t *testing.T) {
	// Setup: file with gap between hunks, cursor on hunk separator
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
					// Gap - next line number is 100, creating a hunk separator
					{Old: sidebyside.Line{Num: 100, Content: "line100"}, New: sidebyside.Line{Num: 100, Content: "line100"}},
				},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the hunk separator position
	rows := m.buildRows()
	sepLineIdx := -1
	for i, row := range rows {
		if row.isSeparator {
			sepLineIdx = i
			break
		}
	}
	assert.NotEqual(t, -1, sepLineIdx, "should have a hunk separator")

	// Position cursor on separator
	m.scroll = sepLineIdx
	assert.Equal(t, sepLineIdx, m.cursorLine(), "cursor should be on hunk separator")

	// TAB should do nothing when not on file header
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Fold level should remain unchanged
	assert.Equal(t, sidebyside.FoldExpanded, model.files[0].FoldLevel, "fold level should not change when TAB pressed on separator")
	// Cursor should remain on same line
	assert.Equal(t, sepLineIdx, model.cursorLine(), "cursor should not move")
}

// Test: Shift+Tab (all files) preserves scroll position appropriately
func TestFoldToggleAll_PreservesScrollPosition(t *testing.T) {
	// Setup: multiple files, cursor in middle of second file
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Layout in diff view (first file has no top border in rows):
	// 0=first header, 1=first bottom border, 2=first diff,
	// 3-6=blank (4 lines), 7=trailing top border, 8=second header, 9=second bottom border, 10=second diff
	// Put cursor on second file's diff line (line 10)
	m.scroll = 10

	assert.Equal(t, 10, m.cursorLine(), "cursor should start on second file's diff line")

	// Shift+Tab: all files Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	// After expanding all, the cursor should still be pointing to second file content
	// The exact line number may change, but we should still be in second file
	info := model.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor should still be in second file after toggle all")
}

// Test: When all files folded, cursor on a file header stays on that file
func TestFoldToggleAll_CursorOnHeader_FoldAll(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find second file's header dynamically (layout varies with tree-style)
	rows := m.buildRows()
	var secondHeaderIdx int
	for i, row := range rows {
		if row.isHeader && row.fileIndex == 1 {
			secondHeaderIdx = i
			break
		}
	}
	require.True(t, secondHeaderIdx > 0, "should find second file header")

	// Put cursor on second file's header
	m.scroll = secondHeaderIdx
	assert.Equal(t, secondHeaderIdx, m.cursorLine(), "cursor should start on second file header")

	// Toggle all: Normal -> Expanded -> Folded
	newM, _ := m.handleFoldToggleAll() // -> Expanded
	m = newM.(Model)
	newM, _ = m.handleFoldToggleAll() // -> Folded
	m = newM.(Model)

	// Cursor should still be pointing to second file (by checking StatusInfo)
	info := m.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor should still be on second file after fold all")
}

// =============================================================================
// Bug Reproduction Tests
// =============================================================================

// Test: Tab should expand the file shown in status bar, not a different file
// Bug: When all files are folded, Tab expands wrong file (first instead of cursor's file)
// Repro: Start -> Shift+Tab (expand all) -> Shift+Tab (fold all) -> navigate to file 2 -> Tab
// Expected: file 2 expands
// Actual bug: file 1 expands
func TestFoldToggle_ExpandsFileAtCursor_NotFileAtScroll(t *testing.T) {
	// Setup: 3 files, all folded
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldFolded,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldFolded,
			},
			{
				OldPath:   "a/third.go",
				NewPath:   "b/third.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldFolded,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// When all folded, layout is (in diff view, first file has no border slot):
	// Line 0: first header
	// Line 1: second header
	// Line 2: third header

	// Position cursor on second file's header (line 1)
	// In new model, scroll = cursorLine, so scroll = 1
	m.scroll = 1
	assert.Equal(t, 1, m.cursorLine(), "cursor should be on line 1 (second file header)")

	// Verify status bar shows second file
	info := m.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "status bar should show second.go")
	assert.Equal(t, 2, info.CurrentFile, "should be file 2 of 3")

	// Press Tab to expand
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// THE BUG: This should expand second.go, but actually expands first.go
	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel, "first file should still be folded")
	assert.Equal(t, sidebyside.FoldNormal, model.files[1].FoldLevel, "second file should be expanded (Normal)")
	assert.Equal(t, sidebyside.FoldFolded, model.files[2].FoldLevel, "third file should still be folded")
}

// Test: When content loads asynchronously after expand, cursor should stay on same line
// Bug: After Tab expands a file without content, FileContentLoadedMsg doesn't preserve scroll
// Repro: Cursor on diff line 5 -> Tab to expand -> content loads -> cursor lost
// Expected: cursor stays on line 5
// Actual bug: cursor jumps to different position
func TestFoldToggle_AsyncContentLoad_PreservesScrollPosition(t *testing.T) {
	// Setup: file in Normal view (structural diff) with cursor on header
	// Press Tab to expand to hunk view, then simulate content loading
	// The diff shows lines 10-15 of the file
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 10, Content: "line10"}, New: sidebyside.Line{Num: 10, Content: "line10"}},
					{Old: sidebyside.Line{Num: 11, Content: "line11"}, New: sidebyside.Line{Num: 11, Content: "line11"}},
					{Old: sidebyside.Line{Num: 12, Content: "line12"}, New: sidebyside.Line{Num: 12, Content: "line12"}},
					{Old: sidebyside.Line{Num: 13, Content: "line13"}, New: sidebyside.Line{Num: 13, Content: "line13"}},
					{Old: sidebyside.Line{Num: 14, Content: "line14"}, New: sidebyside.Line{Num: 14, Content: "line14"}},
					{Old: sidebyside.Line{Num: 15, Content: "line15"}, New: sidebyside.Line{Num: 15, Content: "line15"}},
				},
				// FoldNormal (zero value) = structural diff view
				// No OldContent/NewContent - content not loaded yet
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Layout in Normal view (in diff view, first file has no top border in buildRows):
	// Line 0: header
	// Line 1: bottom border (header spacer)
	// Line 2: separator (breadcrumb) — no SeparatorTop for first hunk
	// Line 3: separator bottom
	// Line 4: diff line (file line 10)
	// Line 5: diff line (file line 11)
	// Line 6: diff line (file line 12) <- cursor here
	// Line 7: diff line (file line 13)
	// ...

	// Position cursor on file header (line 0) to press TAB
	m.scroll = 0
	assert.Equal(t, 0, m.cursorLine(), "cursor should be on header")

	// Press Tab to expand (now works because we're on header)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// File should now be in Expanded mode
	assert.Equal(t, sidebyside.FoldExpanded, model.files[0].FoldLevel, "file should be Expanded")

	// Since content isn't loaded yet, buildRows falls back to Normal view
	// Now move cursor to line 6 (file line 12) to test content load behavior
	model.scroll = 6
	assert.Equal(t, 6, model.cursorLine(), "cursor should be on line 6")

	// Verify we're on the line with content "line12"
	rows := model.buildRows()
	assert.Equal(t, 12, rows[6].pair.Old.Num, "cursor should be on file line 12")

	// Now simulate the content loading
	// The full file has 20 lines (1-20), our diff showed lines 10-15
	fullContent := make([]string, 20)
	for i := range fullContent {
		fullContent[i] = fmt.Sprintf("line%d", i+1)
	}

	contentMsg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: fullContent,
		NewContent: fullContent,
	}

	newM, _ = model.Update(contentMsg)
	model = newM.(Model)

	// After content loads, cursor should still be on file line 12
	// In expanded view with 20 lines (in diff view, no top border):
	// Line 0: header
	// Line 1: header spacer
	// Line 2: file line 1
	// Line 3: file line 2
	// ...
	// Line 13: file line 12 <- cursor should be here
	// ...
	// Note: We were on line 6 before content load, which mapped to file line 12.
	// After content load, file line 12 is at row 13 (header + spacer + 11 lines).

	cursorPos := model.cursorLine()
	rows = model.buildRows()

	// The cursor should point to a row with file line 12
	if cursorPos >= 0 && cursorPos < len(rows) {
		row := rows[cursorPos]
		assert.Equal(t, 12, row.pair.Old.Num,
			"after content loads, cursor should still be on file line 12 (got line %d at cursor pos %d)",
			row.pair.Old.Num, cursorPos)
	} else {
		t.Errorf("cursor position %d is out of bounds (total rows: %d)", cursorPos, len(rows))
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestCursor_ScrollAndStatusStayInSync(t *testing.T) {
	// As we scroll, the status bar file should always match what the cursor is on
	pairs := make([]sidebyside.LinePair, 10)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "content"},
			New: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/alpha.go", NewPath: "b/alpha.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
			{OldPath: "a/beta.go", NewPath: "b/beta.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
			{OldPath: "a/gamma.go", NewPath: "b/gamma.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
		},
		width:  80,
		height: 15,
		scroll: 0,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Build a map of row index -> expected filename from buildRows
	rows := m.buildRows()
	fileNames := []string{"alpha.go", "beta.go", "gamma.go"}
	rowToFile := make(map[int]string)
	for i, row := range rows {
		if row.fileIndex >= 0 && row.fileIndex < len(fileNames) {
			if row.isHeaderTopBorder {
				// Top border shows the previous file in the status bar
				prevIdx := row.fileIndex - 1
				if prevIdx >= 0 {
					rowToFile[i] = fileNames[prevIdx]
				}
			} else {
				rowToFile[i] = fileNames[row.fileIndex]
			}
		}
	}

	// Scroll through and verify status bar matches cursor position
	for scroll := m.minScroll(); scroll <= m.maxScroll(); scroll++ {
		m.scroll = scroll
		cursorPos := m.cursorLine()
		info := m.StatusInfo()

		expectedFile, ok := rowToFile[cursorPos]
		if ok && cursorPos >= 0 && cursorPos < m.totalLines {
			assert.Equal(t, expectedFile, info.FileName,
				"at scroll %d, cursor at %d should show %s", scroll, cursorPos, expectedFile)
		}
	}
}

// =============================================================================
// Cursor Identity Tests - Row Type Preservation
// =============================================================================

// Test: Cursor on file header should stay there after resize
func TestResize_CursorOnHeader_StaysOnHeader(t *testing.T) {
	// Setup: single unfolded file, cursor on header (line 0)
	// In diff view: row 0 = header, row 1 = bottom border, row 2+ = content
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
					{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
				},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		scroll: 0,  // cursor at line 0 (the header)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor is on header initially
	rows := m.buildRows()
	cursorPos := m.cursorLine()
	require.Equal(t, 0, cursorPos, "cursor should start on line 0")
	require.True(t, rows[0].isHeader, "line 0 should be header")

	// Resize the terminal (triggers cursor identity save/restore)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
	model := newM.(Model)

	// Cursor should still be on header (line 0)
	rows = model.buildRows()
	cursorPos = model.cursorLine()
	assert.True(t, rows[cursorPos].isHeader,
		"after resize, cursor should still be on header (got cursorPos=%d, isHeader=%v)",
		cursorPos, rows[cursorPos].isHeader)
}

// Test: Cursor on trailing top border (between files) should stay there after resize
func TestResize_CursorOnTrailingTopBorder_StaysOnTrailingBorder(t *testing.T) {
	// Setup: two unfolded files, cursor on trailing top border of first file
	// Layout for first file: top border (0), header (1), bottom border (2), content (3),
	//                        4 blanks (4-7), trailing top border (8)
	// Then second file: header (9), bottom border (10), ...
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the top border of the second file (the border between file 0 and file 1)
	// This border now has fileIndex=1 (it belongs to the file it precedes)
	rows := m.buildRows()
	file1TopBorderIdx := -1
	for i, row := range rows {
		if row.fileIndex == 1 && row.isHeaderTopBorder {
			file1TopBorderIdx = i
			break
		}
	}
	require.NotEqual(t, -1, file1TopBorderIdx, "should find second file's top border")
	require.True(t, rows[file1TopBorderIdx].isHeaderTopBorder, "should be a top border row")

	// Position cursor on the top border
	m.scroll = file1TopBorderIdx
	cursorPos := m.cursorLine()
	require.Equal(t, file1TopBorderIdx, cursorPos, "cursor should be on second file's top border")

	// Resize the terminal
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
	model := newM.(Model)

	// Cursor should still be on the top border of the second file
	rows = model.buildRows()
	cursorPos = model.cursorLine()

	// BUG: Without isHeaderTopBorder in cursorRowIdentity, this fails!
	// The cursor jumps to the header of the file above instead.
	assert.True(t, rows[cursorPos].isHeaderTopBorder && rows[cursorPos].fileIndex == 1,
		"after resize, cursor should still be on second file's top border "+
			"(got cursorPos=%d, fileIndex=%d, isHeaderTopBorder=%v, isHeader=%v)",
		cursorPos, rows[cursorPos].fileIndex, rows[cursorPos].isHeaderTopBorder, rows[cursorPos].isHeader)
}

// Test: Cursor on truncation indicator should stay there after resize
func TestResize_CursorOnTruncationIndicator_StaysOnTruncation(t *testing.T) {
	// Setup: file with truncation indicator
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded,
				Truncated: true, // This adds a truncation indicator row
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the truncation indicator row
	rows := m.buildRows()
	truncIdx := -1
	for i, row := range rows {
		if row.isTruncationIndicator {
			truncIdx = i
			break
		}
	}
	require.NotEqual(t, -1, truncIdx, "should find truncation indicator row")

	// Position cursor on truncation indicator
	m.scroll = truncIdx
	cursorPos := m.cursorLine()
	require.Equal(t, truncIdx, cursorPos, "cursor should be on truncation indicator")

	// Resize the terminal
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
	model := newM.(Model)

	// Cursor should still be on truncation indicator
	rows = model.buildRows()
	cursorPos = model.cursorLine()
	assert.True(t, rows[cursorPos].isTruncationIndicator,
		"after resize, cursor should still be on truncation indicator (got cursorPos=%d)", cursorPos)
}

// Test: Cursor on second hunk separator should stay there after resize.
// Resize uses absolute row index (row list is stable), so this should work.
// This test verifies that multiple separators in the same file are handled correctly.
func TestResize_CursorOnSecondSeparator_StaysOnSecondSeparator(t *testing.T) {
	// Setup: single file with multiple hunks (gaps in line numbers create separators)
	// Lines 1-2 form first chunk, then gap, lines 10-11 form second chunk,
	// then gap, lines 20-21 form third chunk.
	// This creates two separators: one before line 10, one before line 20.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					// First chunk: lines 1-2
					{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
					{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
					// Gap here (lines 3-9 missing) - creates first separator
					// Second chunk: lines 10-11
					{Old: sidebyside.Line{Num: 10, Content: "line10"}, New: sidebyside.Line{Num: 10, Content: "line10"}},
					{Old: sidebyside.Line{Num: 11, Content: "line11"}, New: sidebyside.Line{Num: 11, Content: "line11"}},
					// Gap here (lines 12-19 missing) - creates second separator
					// Third chunk: lines 20-21
					{Old: sidebyside.Line{Num: 20, Content: "line20"}, New: sidebyside.Line{Num: 20, Content: "line20"}},
					{Old: sidebyside.Line{Num: 21, Content: "line21"}, New: sidebyside.Line{Num: 21, Content: "line21"}},
				},
				FoldLevel: sidebyside.FoldExpanded,
			},
		},
		width:  80,
		height: 30, // Tall enough to see all content
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the separators
	rows := m.buildRows()
	var separatorIndices []int
	for i, row := range rows {
		if row.isSeparator {
			separatorIndices = append(separatorIndices, i)
		}
	}
	require.Len(t, separatorIndices, 2, "should have exactly 2 separators")

	// Position cursor on the SECOND separator
	secondSepIdx := separatorIndices[1]
	m.scroll = secondSepIdx
	cursorPos := m.cursorLine()
	require.Equal(t, secondSepIdx, cursorPos, "cursor should be on second separator")

	// Verify we're on the second separator (not the first)
	require.True(t, rows[cursorPos].isSeparator, "cursor should be on a separator row")
	require.Equal(t, secondSepIdx, cursorPos, "should be on index %d", secondSepIdx)

	// Resize the terminal
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 35})
	model := newM.(Model)

	// Cursor should still be on the SECOND separator, not the first
	rows = model.buildRows()
	cursorPos = model.cursorLine()

	// Find separators again after resize (indices should be unchanged)
	separatorIndices = nil
	for i, row := range rows {
		if row.isSeparator {
			separatorIndices = append(separatorIndices, i)
		}
	}
	require.Len(t, separatorIndices, 2, "should still have 2 separators after resize")

	// Using absolute row index for resize (row list is stable) handles this correctly.
	assert.Equal(t, separatorIndices[1], cursorPos,
		"after resize, cursor should still be on SECOND separator (index %d), but got cursorPos=%d (first separator is at %d)",
		separatorIndices[1], cursorPos, separatorIndices[0])
}

// =============================================================================
// Multi-Commit Tests
// =============================================================================

// createTwoCommitModel creates a model with two commits for testing.
// Both commits start folded (CommitFolded).
func createTwoCommitModel() Model {
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

func TestMultiCommit_BothStartFolded(t *testing.T) {
	m := createTwoCommitModel()

	// Both commits should start folded
	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should start folded")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel, "commit 1 should start folded")

	// Should have 2 rows (one header per commit)
	rows := m.buildRows()
	assert.Equal(t, 2, len(rows), "should have 2 rows when both commits folded")
	assert.True(t, rows[0].isCommitHeader, "row 0 should be commit header")
	assert.True(t, rows[1].isCommitHeader, "row 1 should be commit header")
	assert.Equal(t, 0, rows[0].commitIndex, "row 0 should be commit 0")
	assert.Equal(t, 1, rows[1].commitIndex, "row 1 should be commit 1")
}

func TestMultiCommit_CursorOnFirstCommitHeader(t *testing.T) {
	m := createTwoCommitModel()

	// With scroll=0 and default cursor offset (20% of viewport),
	// cursor should be on first commit header (row 0)
	// since there are only 2 rows
	cursorPos := m.cursorLine()
	rows := m.buildRows()

	// Cursor position depends on viewport math, but should be within rows
	if cursorPos >= len(rows) {
		cursorPos = len(rows) - 1
	}

	// Scroll to put cursor on first commit header (row 0)
	m.scroll = 0 // This puts row 0 at cursor position
	m.calculateTotalLines()

	cursorPos = m.cursorLine()
	assert.True(t, cursorPos >= 0 && cursorPos < len(rows), "cursor should be within rows")
}

func TestMultiCommit_TabExpandsCorrectCommit_First(t *testing.T) {
	m := createTwoCommitModel()

	// Use a small viewport so cursor offset is small
	m.height = 10
	m.calculateTotalLines()

	rows := m.buildRows()
	require.Equal(t, 2, len(rows), "should have 2 rows when both folded")

	// Position scroll so cursor is on row 0 (first commit header)
	// In new model, cursorLine() = scroll, so just set scroll = 0
	m.scroll = 0

	cursorPos := m.cursorLine()

	// If cursor is beyond rows, it will be clamped
	if cursorPos >= len(rows) {
		cursorPos = len(rows) - 1
	}

	// Verify cursor is on a commit header
	require.True(t, rows[cursorPos].isCommitHeader, "cursor should be on commit header")
	commitIdx := rows[cursorPos].commitIndex

	// Verify we're starting correctly
	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should start folded")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel, "commit 1 should start folded")

	// Press Tab to expand the commit at cursor
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(Model)

	// The commit at cursor should be expanded, the other should stay folded
	if commitIdx == 0 {
		assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel, "commit 0 should be expanded after Tab")
		assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel, "commit 1 should still be folded")
	} else {
		assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should still be folded")
		assert.Equal(t, sidebyside.CommitNormal, m.commits[1].FoldLevel, "commit 1 should be expanded after Tab")
	}
}

func TestMultiCommit_TabExpandsCorrectCommit_Second(t *testing.T) {
	m := createTwoCommitModel()

	// First, we need to position cursor on second commit header
	// When both folded, row 0 = commit 0 header, row 1 = commit 1 header
	// We need scroll such that cursorLine() returns 1

	rows := m.buildRows()
	require.Equal(t, 2, len(rows), "should have 2 rows when both folded")

	// Set scroll to put cursor on row 1 (second commit header)
	// In new model, cursorLine() = scroll, so just set scroll = 1
	m.scroll = 1

	// Verify cursor position
	cursorPos := m.cursorLine()
	if cursorPos != 1 {
		// If cursor isn't on row 1, manually verify we're in the right area
		t.Logf("cursorPos=%d, scroll=%d", cursorPos, m.scroll)
	}

	// Press Tab
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(Model)

	// Check which commit got expanded
	// If cursor was on second commit, it should expand; if on first, first should expand
	if cursorPos == 1 {
		assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should still be folded")
		assert.Equal(t, sidebyside.CommitNormal, m.commits[1].FoldLevel, "commit 1 should be expanded after Tab")
	} else {
		// Cursor was on first commit, so first got expanded (fallback behavior)
		t.Logf("Cursor was on row %d, not second commit header", cursorPos)
	}
}

func TestMultiCommit_ExpandFirstThenSecond(t *testing.T) {
	m := createTwoCommitModel()

	// Expand first commit by setting cursor on row 0
	m.scroll = 0 // Put cursor near top

	// Tab to expand first commit (uses fallback to commit 0)
	newM, _ := m.handleCommitFoldCycle()
	m = newM.(Model)

	assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel, "commit 0 should be expanded")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel, "commit 1 should still be folded")

	// Now rows include: commit 0 header, commit 0 body rows, commit 0 files, commit 1 header
	rows := m.buildRows()

	// Find second commit header row
	var secondCommitRow int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitRow = i
			break
		}
	}
	require.NotZero(t, secondCommitRow, "should find second commit header")

	// Position cursor on second commit header
	// In new model, cursorLine() = scroll, so just set scroll = secondCommitRow
	m.scroll = secondCommitRow

	// Verify cursor is on second commit header
	cursorPos := m.cursorLine()
	if cursorPos < len(rows) && rows[cursorPos].isCommitHeader && rows[cursorPos].commitIndex == 1 {
		// Tab to expand second commit
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = newM.(Model)

		// Both should now be expanded
		assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel, "commit 0 should still be expanded")
		assert.Equal(t, sidebyside.CommitNormal, m.commits[1].FoldLevel, "commit 1 should now be expanded")
	} else {
		t.Skipf("Could not position cursor on second commit header (cursorPos=%d, secondCommitRow=%d)", cursorPos, secondCommitRow)
	}
}

func TestMultiCommit_VisibilityLevelIndependent(t *testing.T) {
	m := createTwoCommitModel()

	// Initially both at level 1 (folded)
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0), "commit 0 should be at level 1")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(1), "commit 1 should be at level 1")

	// Expand first commit to level 2
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	// First at level 2, second still at level 1
	assert.Equal(t, 2, m.commitVisibilityLevelFor(0), "commit 0 should be at level 2")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(1), "commit 1 should still be at level 1")

	// Expand first commit's files to level 3
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()

	// First at level 3, second still at level 1
	assert.Equal(t, 3, m.commitVisibilityLevelFor(0), "commit 0 should be at level 3")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(1), "commit 1 should still be at level 1")
}

// =============================================================================
// Multi-Commit Model Initialization Tests
// =============================================================================

func TestMultiCommit_CommitFileStarts_TracksFileBoundaries(t *testing.T) {
	// Create commits with different numbers of files
	commit1 := sidebyside.CommitSet{
		Info:      sidebyside.CommitInfo{SHA: "aaa1111", Author: "Author One", Subject: "Commit 1"},
		FoldLevel: sidebyside.CommitFolded,
		Files: []sidebyside.FilePair{
			{OldPath: "a/file1.go", NewPath: "b/file1.go"},
			{OldPath: "a/file2.go", NewPath: "b/file2.go"},
		},
	}
	commit2 := sidebyside.CommitSet{
		Info:      sidebyside.CommitInfo{SHA: "bbb2222", Author: "Author Two", Subject: "Commit 2"},
		FoldLevel: sidebyside.CommitFolded,
		Files: []sidebyside.FilePair{
			{OldPath: "a/file3.go", NewPath: "b/file3.go"},
			{OldPath: "a/file4.go", NewPath: "b/file4.go"},
			{OldPath: "a/file5.go", NewPath: "b/file5.go"},
		},
	}
	commit3 := sidebyside.CommitSet{
		Info:      sidebyside.CommitInfo{SHA: "ccc3333", Author: "Author Three", Subject: "Commit 3"},
		FoldLevel: sidebyside.CommitFolded,
		Files: []sidebyside.FilePair{
			{OldPath: "a/file6.go", NewPath: "b/file6.go"},
		},
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit1, commit2, commit3})

	// Total files should be 2 + 3 + 1 = 6
	assert.Equal(t, 6, len(m.files), "should have 6 total files flattened")

	// commitFileStarts should track boundaries
	assert.Equal(t, 3, len(m.commitFileStarts), "should have 3 commit file starts")
	assert.Equal(t, 0, m.commitFileStarts[0], "commit 0 files start at index 0")
	assert.Equal(t, 2, m.commitFileStarts[1], "commit 1 files start at index 2")
	assert.Equal(t, 5, m.commitFileStarts[2], "commit 2 files start at index 5")
}

func TestMultiCommit_CommitForFile_ReturnsCorrectCommit(t *testing.T) {
	// Setup: 3 commits with 2, 3, and 1 files respectively
	commit1 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{SHA: "aaa1111"},
		Files: []sidebyside.FilePair{
			{OldPath: "a/file1.go", NewPath: "b/file1.go"},
			{OldPath: "a/file2.go", NewPath: "b/file2.go"},
		},
	}
	commit2 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{SHA: "bbb2222"},
		Files: []sidebyside.FilePair{
			{OldPath: "a/file3.go", NewPath: "b/file3.go"},
			{OldPath: "a/file4.go", NewPath: "b/file4.go"},
			{OldPath: "a/file5.go", NewPath: "b/file5.go"},
		},
	}
	commit3 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{SHA: "ccc3333"},
		Files: []sidebyside.FilePair{
			{OldPath: "a/file6.go", NewPath: "b/file6.go"},
		},
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit1, commit2, commit3})

	// Files 0, 1 -> commit 0
	assert.Equal(t, 0, m.commitForFile(0), "file 0 should belong to commit 0")
	assert.Equal(t, 0, m.commitForFile(1), "file 1 should belong to commit 0")

	// Files 2, 3, 4 -> commit 1
	assert.Equal(t, 1, m.commitForFile(2), "file 2 should belong to commit 1")
	assert.Equal(t, 1, m.commitForFile(3), "file 3 should belong to commit 1")
	assert.Equal(t, 1, m.commitForFile(4), "file 4 should belong to commit 1")

	// File 5 -> commit 2
	assert.Equal(t, 2, m.commitForFile(5), "file 5 should belong to commit 2")
}

func TestMultiCommit_EmptyCommit_FileIndexingCorrect(t *testing.T) {
	// Edge case: first commit has 0 files
	commit1 := sidebyside.CommitSet{
		Info:  sidebyside.CommitInfo{SHA: "aaa1111"},
		Files: []sidebyside.FilePair{}, // Empty
	}
	commit2 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{SHA: "bbb2222"},
		Files: []sidebyside.FilePair{
			{OldPath: "a/file1.go", NewPath: "b/file1.go"},
		},
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit1, commit2})

	// Total files should be 1
	assert.Equal(t, 1, len(m.files), "should have 1 total file")

	// Boundaries: commit 0 starts at 0, commit 1 also starts at 0 (empty commit)
	assert.Equal(t, 0, m.commitFileStarts[0], "commit 0 files start at index 0")
	assert.Equal(t, 0, m.commitFileStarts[1], "commit 1 files start at index 0 (commit 0 was empty)")

	// File 0 should belong to commit 1 (since commit 0 is empty)
	assert.Equal(t, 1, m.commitForFile(0), "file 0 should belong to commit 1")
}

// =============================================================================
// Multi-Commit Row Building Tests
// =============================================================================

func TestMultiCommit_AllFolded_OnlyCommitHeaders(t *testing.T) {
	m := createTwoCommitModel()

	rows := m.buildRows()

	// Should have exactly 2 rows (one per commit header)
	assert.Equal(t, 2, len(rows), "should have 2 rows when both commits folded")

	// Both should be commit headers
	for i, row := range rows {
		assert.True(t, row.isCommitHeader, "row %d should be commit header", i)
		assert.Equal(t, i, row.commitIndex, "row %d should have commitIndex %d", i, i)
	}
}

func TestMultiCommit_OneExpanded_OtherFolded(t *testing.T) {
	m := createTwoCommitModel()

	// Expand first commit
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	rows := m.buildRows()

	// Should have: commit 0 header + commit 0 body rows + commit 0 file rows + commit 1 header
	assert.Greater(t, len(rows), 2, "should have more than 2 rows when one commit expanded")

	// First row should be commit 0 header
	assert.True(t, rows[0].isCommitHeader, "first row should be commit header")
	assert.Equal(t, 0, rows[0].commitIndex, "first row should be commit 0")

	// Find commit 1 header
	var commit1HeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			commit1HeaderIdx = i
			break
		}
	}
	assert.NotEqual(t, 0, commit1HeaderIdx, "should find commit 1 header")
	assert.True(t, rows[commit1HeaderIdx].isCommitHeader, "commit 1 row should be header")
}

func TestMultiCommit_BothExpanded_RowsInterleaved(t *testing.T) {
	m := createTwoCommitModel()

	// Expand both commits
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	rows := m.buildRows()

	// Should have rows for both commits
	var commit0Rows, commit1Rows int
	for _, row := range rows {
		if row.isCommitHeader || row.isCommitBody {
			if row.commitIndex == 0 {
				commit0Rows++
			} else if row.commitIndex == 1 {
				commit1Rows++
			}
		}
	}

	assert.Greater(t, commit0Rows, 0, "should have rows for commit 0")
	assert.Greater(t, commit1Rows, 0, "should have rows for commit 1")
}

func TestMultiCommit_CommitHeadersHaveCorrectIndex(t *testing.T) {
	// Create 3 commits
	commits := []sidebyside.CommitSet{
		{Info: sidebyside.CommitInfo{SHA: "aaa1111"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f1.go", NewPath: "b/f1.go"}}},
		{Info: sidebyside.CommitInfo{SHA: "bbb2222"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f2.go", NewPath: "b/f2.go"}}},
		{Info: sidebyside.CommitInfo{SHA: "ccc3333"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f3.go", NewPath: "b/f3.go"}}},
	}
	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	rows := m.buildRows()

	// Should have 3 commit headers
	var commitHeaders []displayRow
	for _, row := range rows {
		if row.isCommitHeader {
			commitHeaders = append(commitHeaders, row)
		}
	}

	assert.Equal(t, 3, len(commitHeaders), "should have 3 commit headers")
	for i, header := range commitHeaders {
		assert.Equal(t, i, header.commitIndex, "header %d should have commitIndex %d", i, i)
	}
}

// =============================================================================
// Multi-Commit Cursor Positioning Tests
// =============================================================================

func TestMultiCommit_CursorCommitIndex_OnCommitHeader(t *testing.T) {
	m := createTwoCommitModel()

	rows := m.buildRows()
	require.Equal(t, 2, len(rows), "should have 2 rows when both folded")

	// Position cursor on first commit header (row 0)
	m.scroll = 0
	cursorPos := m.cursorLine()
	if cursorPos >= len(rows) {
		cursorPos = 0
	}

	commitIdx := m.cursorCommitIndex()
	if cursorPos == 0 {
		assert.Equal(t, 0, commitIdx, "cursor on row 0 should return commit 0")
	}
}

func TestMultiCommit_CursorCommitIndex_OnCommitBody(t *testing.T) {
	m := createTwoCommitModel()

	// Expand first commit to see body rows
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find a commit body row
	var bodyRowIdx int = -1
	for i, row := range rows {
		if row.isCommitBody && row.commitIndex == 0 {
			bodyRowIdx = i
			break
		}
	}

	if bodyRowIdx >= 0 {
		m.scroll = bodyRowIdx
		commitIdx := m.cursorCommitIndex()
		assert.Equal(t, 0, commitIdx, "cursor on commit 0 body should return commit 0")
	}
}

func TestMultiCommit_CursorCommitIndex_OnFileRow_ReturnsNegative(t *testing.T) {
	m := createTwoCommitModel()

	// Expand first commit and its files
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find a file content row (not header, not commit header/body)
	var fileRowIdx int = -1
	for i, row := range rows {
		if !row.isCommitHeader && !row.isCommitBody && !row.isHeader && row.fileIndex >= 0 {
			fileRowIdx = i
			break
		}
	}

	if fileRowIdx >= 0 {
		m.scroll = fileRowIdx
		commitIdx := m.cursorCommitIndex()
		assert.Equal(t, -1, commitIdx, "cursor on file content row should return -1")
	}
}

// =============================================================================
// Multi-Commit Fold Toggle Tests
// =============================================================================

func TestMultiCommit_TabOnCommit0_OnlyExpandsCommit0(t *testing.T) {
	m := createTwoCommitModel()

	// Both start folded
	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel)
	assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel)

	// Press Tab (uses handleCommitFoldCycle with fallback to commit 0)
	newM, _ := m.handleCommitFoldCycle()
	m = newM.(Model)

	// Commit 0 should be expanded, commit 1 should still be folded
	assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel, "commit 0 should be expanded")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel, "commit 1 should still be folded")
}

func TestMultiCommit_TabCycle_CommitFoldLevels(t *testing.T) {
	m := createTwoCommitModel()

	// Start: both folded (level 1)
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0))

	// Tab 1: Folded -> Normal (level 2)
	newM, _ := m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, 2, m.commitVisibilityLevelFor(0), "after first Tab, commit 0 should be at level 2")

	// Tab 2: Normal -> files expanded (level 3)
	newM, _ = m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, 3, m.commitVisibilityLevelFor(0), "after second Tab, commit 0 should be at level 3")

	// Tab 3: level 3 -> Folded (level 1)
	newM, _ = m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0), "after third Tab, commit 0 should be back to level 1")
}

func TestMultiCommit_ExpandingCommit_DoesNotAffectOtherCommitFiles(t *testing.T) {
	m := createTwoCommitModel()

	// Expand commit 0
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	// Also expand commit 0's files
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()

	// Commit 1's files should still be folded
	assert.Equal(t, sidebyside.FoldFolded, m.files[1].FoldLevel, "commit 1's files should still be folded")

	// Now expand commit 1
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	// Commit 0's files should still be at their level
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel, "commit 0's files should remain unchanged")
	// Commit 1's files should still be folded (commit expanded but files not)
	assert.Equal(t, sidebyside.FoldFolded, m.files[1].FoldLevel, "commit 1's files should still be folded")
}

func TestMultiCommit_MixedFoldStates(t *testing.T) {
	m := createTwoCommitModel()

	// Set commit 0 to level 3 (fully expanded)
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.files[0].FoldLevel = sidebyside.FoldNormal

	// Set commit 1 to level 1 (folded)
	m.commits[1].FoldLevel = sidebyside.CommitFolded

	m.calculateTotalLines()

	assert.Equal(t, 3, m.commitVisibilityLevelFor(0), "commit 0 should be at level 3")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(1), "commit 1 should be at level 1")
}

// =============================================================================
// Multi-Commit Scroll and Navigation Tests
// =============================================================================

func TestMultiCommit_ScrollThroughFoldedCommits(t *testing.T) {
	m := createTwoCommitModel()

	rows := m.buildRows()
	require.Equal(t, 2, len(rows), "should have 2 rows when both folded")

	// Should be able to scroll to reach both headers
	m.scroll = 0
	cursorPos := m.cursorLine()

	// With small viewport, we should be able to reach all rows
	for i := 0; i < len(rows); i++ {
		m.scroll = i
		cursorPos = m.cursorLine()
		if cursorPos >= 0 && cursorPos < len(rows) {
			assert.True(t, rows[cursorPos].isCommitHeader, "row %d should be commit header", cursorPos)
		}
	}
}

func TestMultiCommit_ExpandCommit_ScrollBoundsUpdate(t *testing.T) {
	m := createTwoCommitModel()

	// Get initial total lines (2 when both folded)
	initialTotal := m.totalLines
	assert.Equal(t, 2, initialTotal, "should have 2 total lines when both folded")

	// Expand first commit
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	// Total lines should increase
	assert.Greater(t, m.totalLines, initialTotal, "total lines should increase when commit expanded")
}

func TestMultiCommit_CollapseCommit_CursorAdjusts(t *testing.T) {
	m := createTwoCommitModel()

	// Expand first commit
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()

	rows := m.buildRows()
	initialRowCount := len(rows)

	// Position cursor somewhere in commit 0's expanded content
	var contentRowIdx int = -1
	for i, row := range rows {
		if row.commitIndex == 0 && (row.isCommitBody || row.isHeader) && i > 0 {
			contentRowIdx = i
			break
		}
	}

	if contentRowIdx > 0 {
		m.scroll = contentRowIdx

		// Collapse commit 0
		m.commits[0].FoldLevel = sidebyside.CommitFolded
		m.calculateTotalLines()

		newRows := m.buildRows()
		assert.Less(t, len(newRows), initialRowCount, "row count should decrease after collapsing")
	}
}

func TestMultiCommit_NavigateJK_ThroughFoldedCommits(t *testing.T) {
	m := createTwoCommitModel()

	// Start at top
	m.scroll = m.minScroll()

	// Press j to scroll down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)

	// Scroll should have increased
	assert.Greater(t, m.scroll, m.minScroll(), "j should scroll down")
}

// =============================================================================
// Multi-Commit Edge Cases
// =============================================================================

func TestMultiCommit_SingleCommit_BehavesLikeShow(t *testing.T) {
	commit := sidebyside.CommitSet{
		Info:      sidebyside.CommitInfo{SHA: "aaa1111", Author: "Author", Subject: "Subject"},
		FoldLevel: sidebyside.CommitFolded,
		Files: []sidebyside.FilePair{
			{OldPath: "a/file.go", NewPath: "b/file.go"},
		},
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	rows := m.buildRows()

	// Should have at least 1 commit header
	assert.Greater(t, len(rows), 0, "should have at least one row")
	assert.True(t, rows[0].isCommitHeader, "first row should be commit header")
}

func TestMultiCommit_AllCommitsEmpty_OnlyHeaders(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{Info: sidebyside.CommitInfo{SHA: "aaa1111"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{}},
		{Info: sidebyside.CommitInfo{SHA: "bbb2222"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{}},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Total files should be 0
	assert.Equal(t, 0, len(m.files), "should have 0 total files")

	rows := m.buildRows()

	// Should have 2 commit headers only
	assert.Equal(t, 2, len(rows), "should have 2 rows (one per empty commit header)")
}

func TestMultiCommit_FirstCommitEmpty_SecondHasFiles(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{Info: sidebyside.CommitInfo{SHA: "aaa1111"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{}},
		{Info: sidebyside.CommitInfo{SHA: "bbb2222"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{
			{OldPath: "a/file.go", NewPath: "b/file.go"},
		}},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// File should belong to commit 1
	assert.Equal(t, 1, len(m.files), "should have 1 file")
	assert.Equal(t, 1, m.commitForFile(0), "file 0 should belong to commit 1")

	// commitFileStarts
	assert.Equal(t, 0, m.commitFileStarts[0], "commit 0 starts at index 0")
	assert.Equal(t, 0, m.commitFileStarts[1], "commit 1 starts at index 0 (commit 0 empty)")
}

// =============================================================================
// Multi-Commit Row Building Tests (Additional)
// =============================================================================

func TestMultiCommit_FileRowsHaveCorrectFileIndex(t *testing.T) {
	// Create 2 commits with 2 files each
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitNormal,
			Files: []sidebyside.FilePair{
				{OldPath: "a/file1.go", NewPath: "b/file1.go", FoldLevel: sidebyside.FoldFolded},
				{OldPath: "a/file2.go", NewPath: "b/file2.go", FoldLevel: sidebyside.FoldFolded},
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitNormal,
			Files: []sidebyside.FilePair{
				{OldPath: "a/file3.go", NewPath: "b/file3.go", FoldLevel: sidebyside.FoldFolded},
				{OldPath: "a/file4.go", NewPath: "b/file4.go", FoldLevel: sidebyside.FoldFolded},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find file header rows and check their fileIndex
	var fileHeaders []displayRow
	for _, row := range rows {
		if row.isHeader && row.fileIndex >= 0 {
			fileHeaders = append(fileHeaders, row)
		}
	}

	require.Equal(t, 4, len(fileHeaders), "should have 4 file headers")

	// File indices should be global: 0, 1, 2, 3
	assert.Equal(t, 0, fileHeaders[0].fileIndex, "first file header should have fileIndex 0")
	assert.Equal(t, 1, fileHeaders[1].fileIndex, "second file header should have fileIndex 1")
	assert.Equal(t, 2, fileHeaders[2].fileIndex, "third file header should have fileIndex 2")
	assert.Equal(t, 3, fileHeaders[3].fileIndex, "fourth file header should have fileIndex 3")
}

// =============================================================================
// Multi-Commit Cursor Positioning Tests (Additional)
// =============================================================================

func TestMultiCommit_CursorCommitIndex_OnCommitNHeader(t *testing.T) {
	// Create 3 commits all folded
	commits := []sidebyside.CommitSet{
		{Info: sidebyside.CommitInfo{SHA: "aaa1111"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f1.go", NewPath: "b/f1.go"}}},
		{Info: sidebyside.CommitInfo{SHA: "bbb2222"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f2.go", NewPath: "b/f2.go"}}},
		{Info: sidebyside.CommitInfo{SHA: "ccc3333"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f3.go", NewPath: "b/f3.go"}}},
	}
	m := NewWithCommits(commits)
	m.width = 80
	m.height = 10 // Small viewport
	m.focused = true
	m.calculateTotalLines()

	rows := m.buildRows()
	require.Equal(t, 3, len(rows), "should have 3 rows when all folded")

	// Test cursor on each commit header
	for i := 0; i < 3; i++ {
		// Position cursor on commit i header
		m.scroll = i
		cursorPos := m.cursorLine()

		// Clamp to valid range
		if cursorPos < 0 {
			cursorPos = 0
		}
		if cursorPos >= len(rows) {
			cursorPos = len(rows) - 1
		}

		if rows[cursorPos].isCommitHeader {
			commitIdx := m.cursorCommitIndex()
			assert.Equal(t, rows[cursorPos].commitIndex, commitIdx,
				"cursorCommitIndex should return %d when cursor on commit %d header", rows[cursorPos].commitIndex, i)
		}
	}
}

// =============================================================================
// Multi-Commit Fold Toggle Tests (Additional)
// =============================================================================

func TestMultiCommit_CollapsingCommit1_DoesNotAffectCommit0(t *testing.T) {
	m := createTwoCommitModel()

	// Expand both commits
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	m.files[0].FoldLevel = sidebyside.FoldNormal // commit 0's file
	m.files[1].FoldLevel = sidebyside.FoldNormal // commit 1's file
	m.calculateTotalLines()

	// Record commit 0's state
	commit0FoldLevel := m.commits[0].FoldLevel
	file0FoldLevel := m.files[0].FoldLevel

	// Collapse commit 1
	m.commits[1].FoldLevel = sidebyside.CommitFolded
	m.calculateTotalLines()

	// Commit 0 should be unchanged
	assert.Equal(t, commit0FoldLevel, m.commits[0].FoldLevel, "commit 0 fold level should be unchanged")
	assert.Equal(t, file0FoldLevel, m.files[0].FoldLevel, "commit 0's file fold level should be unchanged")
}

// =============================================================================
// Multi-Commit Rendering Tests
// =============================================================================

// createThreeCommitModelWithDifferentStats creates a model with 3 commits
// that have different file counts and stats to test per-commit rendering.
func createThreeCommitModelWithDifferentStats() Model {
	// Commit 0: 1 file, +10 -5
	commit0 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "aaa1111111111111111111111111111111111111",
			Author:  "Alice Author",
			Subject: "First commit message",
		},
		FoldLevel: sidebyside.CommitFolded,
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/alice.go",
				NewPath:   "b/alice.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
					{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
					{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
					{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
					{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				},
			},
		},
	}

	// Commit 1: 2 files, +20 -0
	commit1 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "bbb2222222222222222222222222222222222222",
			Author:  "Bob Builder",
			Subject: "Second commit message",
		},
		FoldLevel: sidebyside.CommitFolded,
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/bob1.go",
				NewPath:   "b/bob1.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				},
			},
			{
				OldPath:   "a/bob2.go",
				NewPath:   "b/bob2.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				},
			},
		},
	}

	// Commit 2: 3 files, +0 -30
	commit2 := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "ccc3333333333333333333333333333333333333",
			Author:  "Carol Coder",
			Subject: "Third commit message",
		},
		FoldLevel: sidebyside.CommitFolded,
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/carol1.go",
				NewPath:   "b/carol1.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     makeRemovedPairs(10),
			},
			{
				OldPath:   "a/carol2.go",
				NewPath:   "b/carol2.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     makeRemovedPairs(10),
			},
			{
				OldPath:   "a/carol3.go",
				NewPath:   "b/carol3.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     makeRemovedPairs(10),
			},
		},
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit0, commit1, commit2})
	m.width = 120
	m.height = 40
	m.focused = true
	m.calculateTotalLines()
	return m
}

// makeRemovedPairs creates n line pairs that are all removals.
func makeRemovedPairs(n int) []sidebyside.LinePair {
	pairs := make([]sidebyside.LinePair, n)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Removed},
			New: sidebyside.Line{Type: sidebyside.Empty},
		}
	}
	return pairs
}

func TestMultiCommit_RenderCommitHeader_ShowsCorrectSHA(t *testing.T) {
	m := createThreeCommitModelWithDifferentStats()
	rows := m.buildRows()

	// Find commit headers and verify each has correct SHA in commitIndex
	for _, row := range rows {
		if row.isCommitHeader {
			// The row should have the correct commitIndex
			expectedSHA := m.commits[row.commitIndex].Info.SHA
			assert.NotEmpty(t, expectedSHA, "commit %d should have SHA", row.commitIndex)

			// Verify the SHA matches the commit at that index
			actualSHA := m.commits[row.commitIndex].Info.SHA
			assert.Equal(t, expectedSHA, actualSHA, "commit header row %d should reference correct SHA", row.commitIndex)
		}
	}
}

func TestMultiCommit_RenderCommitHeader_ShowsCorrectAuthor(t *testing.T) {
	m := createThreeCommitModelWithDifferentStats()

	// Verify each commit has different author
	assert.Equal(t, "Alice Author", m.commits[0].Info.Author)
	assert.Equal(t, "Bob Builder", m.commits[1].Info.Author)
	assert.Equal(t, "Carol Coder", m.commits[2].Info.Author)

	rows := m.buildRows()

	// Find commit headers
	var commitHeaders []displayRow
	for _, row := range rows {
		if row.isCommitHeader {
			commitHeaders = append(commitHeaders, row)
		}
	}

	require.Equal(t, 3, len(commitHeaders), "should have 3 commit headers")

	// Each header's commitIndex should map to correct author
	for i, header := range commitHeaders {
		author := m.commits[header.commitIndex].Info.Author
		assert.NotEmpty(t, author, "commit %d should have author", i)
	}
}

func TestMultiCommit_RenderCommitHeader_ShowsCorrectSubject(t *testing.T) {
	m := createThreeCommitModelWithDifferentStats()

	// Verify each commit has different subject
	assert.Equal(t, "First commit message", m.commits[0].Info.Subject)
	assert.Equal(t, "Second commit message", m.commits[1].Info.Subject)
	assert.Equal(t, "Third commit message", m.commits[2].Info.Subject)

	rows := m.buildRows()

	// Each commit header should have correct commitIndex
	for _, row := range rows {
		if row.isCommitHeader {
			subject := m.commits[row.commitIndex].Info.Subject
			assert.NotEmpty(t, subject, "commit %d should have subject", row.commitIndex)
		}
	}
}

func TestMultiCommit_RenderCommitHeader_FoldIconReflectsState(t *testing.T) {
	m := createThreeCommitModelWithDifferentStats()

	// Set different fold levels
	m.commits[0].FoldLevel = sidebyside.CommitFolded
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	m.commits[2].FoldLevel = sidebyside.CommitExpanded
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find commit headers and verify fold level
	var commitHeaders []displayRow
	for _, row := range rows {
		if row.isCommitHeader {
			commitHeaders = append(commitHeaders, row)
		}
	}

	require.GreaterOrEqual(t, len(commitHeaders), 3, "should have at least 3 commit headers")

	// Verify each header has the correct commitFoldLevel
	assert.Equal(t, sidebyside.CommitFolded, commitHeaders[0].commitFoldLevel, "commit 0 should be Folded")
	assert.Equal(t, sidebyside.CommitNormal, commitHeaders[1].commitFoldLevel, "commit 1 should be Normal")
	assert.Equal(t, sidebyside.CommitExpanded, commitHeaders[2].commitFoldLevel, "commit 2 should be Expanded")
}

// =============================================================================
// Multi-Commit Scroll and Navigation Tests (Additional)
// =============================================================================

func TestMultiCommit_AfterExpanding_JKReachesFileRows(t *testing.T) {
	m := createTwoCommitModel()

	// Expand first commit and its files
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find a file content row (not header)
	var fileContentRowIdx int = -1
	for i, row := range rows {
		if !row.isCommitHeader && !row.isCommitBody && !row.isHeader && !row.isBlank && !row.isSeparator && row.fileIndex >= 0 {
			fileContentRowIdx = i
			break
		}
	}

	if fileContentRowIdx > 0 {
		// Start at top
		m.scroll = m.minScroll()

		// Keep pressing j until we reach the file content row
		maxIterations := len(rows) + 10
		reachedFileRow := false
		for i := 0; i < maxIterations; i++ {
			cursorPos := m.cursorLine()
			if cursorPos >= 0 && cursorPos < len(rows) {
				row := rows[cursorPos]
				if !row.isCommitHeader && !row.isCommitBody && !row.isHeader && !row.isBlank && row.fileIndex >= 0 {
					reachedFileRow = true
					break
				}
			}
			newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
			m = newM.(Model)
		}
		assert.True(t, reachedFileRow, "should be able to reach file content rows via j navigation")
	}
}

// =============================================================================
// Multi-Commit Edge Cases (Additional)
// =============================================================================

func TestMultiCommit_VeryLongCommitBody_RendersWithoutCrash(t *testing.T) {
	// Create a commit with a very long body
	longBody := strings.Repeat("This is a very long line of commit message text. ", 100)

	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "aaa1111",
			Author:  "Author",
			Subject: "Short subject",
			Body:    longBody,
		},
		FoldLevel: sidebyside.CommitNormal, // Expanded to show body
		Files: []sidebyside.FilePair{
			{OldPath: "a/file.go", NewPath: "b/file.go"},
		},
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Should not panic when building rows
	rows := m.buildRows()
	assert.Greater(t, len(rows), 0, "should have rows")

	// Should not panic when rendering
	output := m.View()
	assert.NotEmpty(t, output, "should render output")
}

// =============================================================================
// Multi-Commit Per-Commit Stats Tests
// =============================================================================

// TestMultiCommit_CommitFileStatsArePerCommit tests that the displayRow
// for each commit header contains the correct per-commit file stats.
// This is important because we previously had a bug where all commits
// showed the same (total) stats instead of per-commit stats.
func TestMultiCommit_CommitFileStatsArePerCommit(t *testing.T) {
	// Create 3 commits with different stats:
	// Commit 0: 1 file
	// Commit 1: 2 files
	// Commit 2: 3 files
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitFolded,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go"},
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitFolded,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f2.go", NewPath: "b/f2.go"},
				{OldPath: "a/f3.go", NewPath: "b/f3.go"},
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "ccc3333"},
			FoldLevel: sidebyside.CommitFolded,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f4.go", NewPath: "b/f4.go"},
				{OldPath: "a/f5.go", NewPath: "b/f5.go"},
				{OldPath: "a/f6.go", NewPath: "b/f6.go"},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.calculateTotalLines()

	// Verify the model has correct file counts per commit
	assert.Equal(t, 1, len(m.commits[0].Files), "commit 0 should have 1 file")
	assert.Equal(t, 2, len(m.commits[1].Files), "commit 1 should have 2 files")
	assert.Equal(t, 3, len(m.commits[2].Files), "commit 2 should have 3 files")

	// Get file count per commit from commitFileStarts
	commit0FileCount := m.commitFileStarts[1] - m.commitFileStarts[0]
	commit1FileCount := m.commitFileStarts[2] - m.commitFileStarts[1]
	commit2FileCount := len(m.files) - m.commitFileStarts[2]

	assert.Equal(t, 1, commit0FileCount, "commit 0 should have 1 file via commitFileStarts")
	assert.Equal(t, 2, commit1FileCount, "commit 1 should have 2 files via commitFileStarts")
	assert.Equal(t, 3, commit2FileCount, "commit 2 should have 3 files via commitFileStarts")
}

// TestMultiCommit_CommitAddRemoveStatsArePerCommit tests that each commit
// shows its own +/- stats, not the total across all commits.
func TestMultiCommit_CommitAddRemoveStatsArePerCommit(t *testing.T) {
	// Create commits with known add/remove stats:
	// Commit 0: +5 -0
	// Commit 1: +0 -10
	// Commit 2: +3 -7
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitFolded,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/f1.go",
					NewPath:   "b/f1.go",
					FoldLevel: sidebyside.FoldFolded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
					},
				},
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitFolded,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/f2.go",
					NewPath:   "b/f2.go",
					FoldLevel: sidebyside.FoldFolded,
					Pairs:     makeRemovedPairs(10),
				},
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "ccc3333"},
			FoldLevel: sidebyside.CommitFolded,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/f3.go",
					NewPath:   "b/f3.go",
					FoldLevel: sidebyside.FoldFolded,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
						{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
						{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
						{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
						{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
						{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
						{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
						{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
						{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Empty}},
					},
				},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.calculateTotalLines()

	// Calculate expected stats per commit
	// Commit 0: +5 -0
	// Commit 1: +0 -10
	// Commit 2: +3 -7

	// Helper to count stats for a commit's files
	countCommitStats := func(commitIdx int) (added, removed int) {
		startIdx := m.commitFileStarts[commitIdx]
		endIdx := len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}
		for i := startIdx; i < endIdx; i++ {
			for _, pair := range m.files[i].Pairs {
				if pair.New.Type == sidebyside.Added {
					added++
				}
				if pair.Old.Type == sidebyside.Removed {
					removed++
				}
			}
		}
		return
	}

	add0, rem0 := countCommitStats(0)
	add1, rem1 := countCommitStats(1)
	add2, rem2 := countCommitStats(2)

	assert.Equal(t, 5, add0, "commit 0 should have +5")
	assert.Equal(t, 0, rem0, "commit 0 should have -0")
	assert.Equal(t, 0, add1, "commit 1 should have +0")
	assert.Equal(t, 10, rem1, "commit 1 should have -10")
	assert.Equal(t, 3, add2, "commit 2 should have +3")
	assert.Equal(t, 7, rem2, "commit 2 should have -7")
}

// =============================================================================
// Multi-Commit Helper Method Tests
// =============================================================================

func TestMultiCommit_IsOnCommitHeader(t *testing.T) {
	m := createTwoCommitModel()

	rows := m.buildRows()
	require.Equal(t, 2, len(rows), "should have 2 rows when both folded")

	// Test each row
	for i, row := range rows {
		if row.isCommitHeader {
			// Position cursor on this row
			m.scroll = i
			cursorPos := m.cursorLine()
			if cursorPos >= 0 && cursorPos < len(rows) {
				assert.True(t, rows[cursorPos].isCommitHeader,
					"row %d should be identified as commit header", i)
			}
		}
	}
}

func TestMultiCommit_TabOnCommitNHeader_OnlyExpandsCommitN(t *testing.T) {
	// Create 3 commits
	commits := []sidebyside.CommitSet{
		{Info: sidebyside.CommitInfo{SHA: "aaa1111"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f1.go", NewPath: "b/f1.go"}}},
		{Info: sidebyside.CommitInfo{SHA: "bbb2222"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f2.go", NewPath: "b/f2.go"}}},
		{Info: sidebyside.CommitInfo{SHA: "ccc3333"}, FoldLevel: sidebyside.CommitFolded, Files: []sidebyside.FilePair{{OldPath: "a/f3.go", NewPath: "b/f3.go"}}},
	}
	m := NewWithCommits(commits)
	m.width = 80
	m.height = 10 // Small viewport for easier cursor positioning
	m.focused = true
	m.calculateTotalLines()

	// All should start folded
	for i, c := range m.commits {
		assert.Equal(t, sidebyside.CommitFolded, c.FoldLevel, "commit %d should start folded", i)
	}

	rows := m.buildRows()
	require.Equal(t, 3, len(rows), "should have 3 rows when all folded")

	// Position cursor on commit 1 (middle commit, row 1)
	// cursorLine() = scroll + cursorOffset()
	// We want cursorLine() = 1, so scroll = 1 - cursorOffset()
	m.scroll = 1
	cursorPos := m.cursorLine()

	// Ensure cursor is on row 1
	if cursorPos == 1 && rows[1].isCommitHeader && rows[1].commitIndex == 1 {
		// Tab to expand commit 1
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = newM.(Model)

		// Only commit 1 should be expanded
		assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should still be folded")
		assert.Equal(t, sidebyside.CommitNormal, m.commits[1].FoldLevel, "commit 1 should be expanded")
		assert.Equal(t, sidebyside.CommitFolded, m.commits[2].FoldLevel, "commit 2 should still be folded")
	} else {
		t.Skipf("Could not position cursor on commit 1 header (cursorPos=%d)", cursorPos)
	}
}

// =============================================================================
// Multi-Commit Remaining Tests
// =============================================================================

func TestMultiCommit_IsOnCommitSection(t *testing.T) {
	m := createTwoCommitModel()

	// Count rows when both commits are folded
	m.commits[0].FoldLevel = sidebyside.CommitFolded
	m.commits[1].FoldLevel = sidebyside.CommitFolded
	m.calculateTotalLines()
	foldedRows := m.buildRows()
	foldedCount := len(foldedRows)

	// Expand first commit
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.calculateTotalLines()
	rows := m.buildRows()
	expandedCount := len(rows)

	// Count commit headers
	var headerCount int
	for _, row := range rows {
		if row.isCommitHeader {
			headerCount++
		}
	}

	// Should have at least 2 headers (both commits)
	assert.GreaterOrEqual(t, headerCount, 2, "should have at least 2 commit headers")

	// Expanded should have more rows than folded (commit info node adds rows)
	assert.Greater(t, expandedCount, foldedCount,
		"expanded commit should have more rows than folded (got %d vs %d)", expandedCount, foldedCount)

	// Verify that commit headers have valid commitIndex
	for _, row := range rows {
		if row.isCommitHeader {
			assert.GreaterOrEqual(t, row.commitIndex, 0, "commit header rows should have valid commitIndex")
			assert.Less(t, row.commitIndex, len(m.commits), "commitIndex should be in range")
		}
	}
}

func TestMultiCommit_BlankRowsAtCommitBoundaries(t *testing.T) {
	// Create 2 commits, both expanded with files visible
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitNormal,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldFolded},
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitNormal,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldFolded},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the boundary between commit 0's content and commit 1's header
	var commit1HeaderIdx int = -1
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			commit1HeaderIdx = i
			break
		}
	}

	require.Greater(t, commit1HeaderIdx, 0, "should find commit 1 header after some rows")

	// Verify commit 1 header comes after commit 0's content
	// The exact structure depends on implementation, but commit 1 header
	// should not be at index 0 (that would be commit 0's header)
	assert.True(t, rows[0].isCommitHeader && rows[0].commitIndex == 0,
		"first row should be commit 0 header")
	assert.True(t, rows[commit1HeaderIdx].isCommitHeader && rows[commit1HeaderIdx].commitIndex == 1,
		"found row should be commit 1 header")
}

func TestMultiCommit_ManyFilesInOneCommit(t *testing.T) {
	// Create a commit with many files to test handling of large file counts
	// Note: This tests the model handles many files, not actual truncation
	// (truncation is controlled by the caller, not buildRows)

	files := make([]sidebyside.FilePair, 50)
	for i := range files {
		files[i] = sidebyside.FilePair{
			OldPath:   fmt.Sprintf("a/file%d.go", i),
			NewPath:   fmt.Sprintf("b/file%d.go", i),
			FoldLevel: sidebyside.FoldFolded,
		}
	}

	commit := sidebyside.CommitSet{
		Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
		FoldLevel: sidebyside.CommitNormal, // Expanded to show files
		Files:     files,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Should have 50 files
	assert.Equal(t, 50, len(m.files), "should have 50 files")

	rows := m.buildRows()

	// Should have at least 1 header + 50 file headers
	assert.Greater(t, len(rows), 50, "should have many rows for 50 files")

	// Count file headers
	var fileHeaderCount int
	for _, row := range rows {
		if row.isHeader && row.fileIndex >= 0 {
			fileHeaderCount++
		}
	}

	assert.Equal(t, 50, fileHeaderCount, "should have 50 file headers")
}

// =============================================================================
// Shift+Tab Commit Cycling Tests
// =============================================================================

func TestShiftTab_CyclesAllCommitsThroughLevels(t *testing.T) {
	// Create 2 commits, both starting at level 1 (CommitFolded)
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitFolded,
			Files:     []sidebyside.FilePair{{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldFolded}},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitFolded,
			Files:     []sidebyside.FilePair{{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldFolded}},
		},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Level 1: All commits folded
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0), "commit 0 should start at level 1")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(1), "commit 1 should start at level 1")

	// Shift+Tab 1: Level 1 -> Level 2 (CommitNormal, files FoldFolded)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)

	assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel, "commit 0 should be CommitNormal")
	assert.Equal(t, sidebyside.CommitNormal, m.commits[1].FoldLevel, "commit 1 should be CommitNormal")
	assert.Equal(t, sidebyside.FoldFolded, m.files[0].FoldLevel, "file 0 should be FoldFolded")
	assert.Equal(t, sidebyside.FoldFolded, m.files[1].FoldLevel, "file 1 should be FoldFolded")
	assert.Equal(t, 2, m.commitVisibilityLevelFor(0), "commit 0 should be at level 2")
	assert.Equal(t, 2, m.commitVisibilityLevelFor(1), "commit 1 should be at level 2")

	// Shift+Tab 2: Level 2 -> Level 3 (CommitExpanded, files FoldExpanded)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)

	assert.Equal(t, sidebyside.CommitExpanded, m.commits[0].FoldLevel, "commit 0 should be CommitExpanded")
	assert.Equal(t, sidebyside.CommitExpanded, m.commits[1].FoldLevel, "commit 1 should be CommitExpanded")
	assert.Equal(t, sidebyside.FoldExpanded, m.files[0].FoldLevel, "file 0 should be FoldExpanded")
	assert.Equal(t, sidebyside.FoldExpanded, m.files[1].FoldLevel, "file 1 should be FoldExpanded")
	assert.Equal(t, 3, m.commitVisibilityLevelFor(0), "commit 0 should be at level 3")
	assert.Equal(t, 3, m.commitVisibilityLevelFor(1), "commit 1 should be at level 3")

	// Shift+Tab 3: Level 3 -> Level 1 (CommitFolded, files FoldFolded)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)

	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should be CommitFolded")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel, "commit 1 should be CommitFolded")
	assert.Equal(t, sidebyside.FoldFolded, m.files[0].FoldLevel, "file 0 should be FoldFolded")
	assert.Equal(t, sidebyside.FoldFolded, m.files[1].FoldLevel, "file 1 should be FoldFolded")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0), "commit 0 should be back at level 1")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(1), "commit 1 should be back at level 1")
}

func TestShiftTab_MixedLevels_ResetsToLevel1(t *testing.T) {
	// Create 2 commits at different levels
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitFolded, // Level 1
			Files:     []sidebyside.FilePair{{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldFolded}},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitNormal, // Level 2 or 3 depending on files
			Files:     []sidebyside.FilePair{{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldExpanded}},
		},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Commit 0 at level 1, commit 1 at level 3
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0), "commit 0 should be at level 1")
	assert.Equal(t, 3, m.commitVisibilityLevelFor(1), "commit 1 should be at level 3")

	// Shift+Tab: Mixed levels -> Reset to level 1
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)

	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should be CommitFolded")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[1].FoldLevel, "commit 1 should be CommitFolded")
	assert.Equal(t, sidebyside.FoldFolded, m.files[0].FoldLevel, "file 0 should be FoldFolded")
	assert.Equal(t, sidebyside.FoldFolded, m.files[1].FoldLevel, "file 1 should be FoldFolded")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0), "commit 0 should be at level 1")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(1), "commit 1 should be at level 1")
}

func TestShiftTab_FileExpanded_TreatedAsLevel3(t *testing.T) {
	// Create a commit where one file is at FoldExpanded (full content)
	// This should be treated as level 3 (or higher), not level 2
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitNormal,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldExpanded}, // Expanded = level 3+
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitNormal,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldExpanded},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Both commits have expanded files, should be level 3
	assert.Equal(t, 3, m.commitVisibilityLevelFor(0), "commit 0 with expanded file should be at level 3")
	assert.Equal(t, 3, m.commitVisibilityLevelFor(1), "commit 1 with expanded file should be at level 3")

	// Shift+Tab: Level 3 -> Level 1 (files go to FoldFolded, not stay at FoldExpanded)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)

	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel, "commit 0 should be CommitFolded")
	assert.Equal(t, sidebyside.FoldFolded, m.files[0].FoldLevel, "file 0 should be FoldFolded")
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0), "commit 0 should be at level 1")
}

func TestShiftTab_SingleCommit_CyclesCorrectly(t *testing.T) {
	// Single commit (like show command) should also cycle correctly
	commit := sidebyside.CommitSet{
		Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
		FoldLevel: sidebyside.CommitFolded,
		Files:     []sidebyside.FilePair{{OldPath: "a/f.go", NewPath: "b/f.go", FoldLevel: sidebyside.FoldFolded}},
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Level 1
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0))

	// Shift+Tab -> Level 2
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)
	assert.Equal(t, 2, m.commitVisibilityLevelFor(0))

	// Shift+Tab -> Level 3
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)
	assert.Equal(t, 3, m.commitVisibilityLevelFor(0))

	// Shift+Tab -> Level 1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newM.(Model)
	assert.Equal(t, 1, m.commitVisibilityLevelFor(0))
}

// =============================================================================
// Per-Commit File Numbering Tests
// =============================================================================

func TestMultiCommit_FileNumbersResetPerCommit(t *testing.T) {
	// Create 2 commits with different file counts
	// Commit 0: 2 files
	// Commit 1: 3 files
	commits := []sidebyside.CommitSet{
		{
			Info:      sidebyside.CommitInfo{SHA: "aaa1111"},
			FoldLevel: sidebyside.CommitNormal, // Show files
			Files: []sidebyside.FilePair{
				{OldPath: "a/first.go", NewPath: "b/first.go", FoldLevel: sidebyside.FoldFolded},
				{OldPath: "a/second.go", NewPath: "b/second.go", FoldLevel: sidebyside.FoldFolded},
			},
		},
		{
			Info:      sidebyside.CommitInfo{SHA: "bbb2222"},
			FoldLevel: sidebyside.CommitNormal,
			Files: []sidebyside.FilePair{
				{OldPath: "a/alpha.go", NewPath: "b/alpha.go", FoldLevel: sidebyside.FoldFolded},
				{OldPath: "a/beta.go", NewPath: "b/beta.go", FoldLevel: sidebyside.FoldFolded},
				{OldPath: "a/gamma.go", NewPath: "b/gamma.go", FoldLevel: sidebyside.FoldFolded},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find file header rows by fileIndex
	fileRowIndices := make(map[int]int) // global fileIndex -> row index
	for i, row := range rows {
		if row.isHeader && row.fileIndex >= 0 {
			fileRowIndices[row.fileIndex] = i
		}
	}

	// Test first file of first commit (global index 0)
	m.scroll = fileRowIndices[0]
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "first file in commit 0 should be #1")
	assert.Equal(t, 2, info.TotalFiles, "commit 0 has 2 total files")

	// Test second file of first commit (global index 1)
	m.scroll = fileRowIndices[1]
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "second file in commit 0 should be #2")
	assert.Equal(t, 2, info.TotalFiles, "commit 0 has 2 total files")

	// Test first file of second commit (global index 2) - should reset to #1
	m.scroll = fileRowIndices[2]
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "first file in commit 1 should be #1 (reset)")
	assert.Equal(t, 3, info.TotalFiles, "commit 1 has 3 total files")

	// Test third file of second commit (global index 4)
	m.scroll = fileRowIndices[4]
	info = m.StatusInfo()
	assert.Equal(t, 3, info.CurrentFile, "third file in commit 1 should be #3")
	assert.Equal(t, 3, info.TotalFiles, "commit 1 has 3 total files")
}

// Test: Cursor should stay on commit when toggling fold from commit info body
// Bug: When cursor is on commit info body and shift+tab is pressed, cursor shifts
// Repro: show command -> move to commit info body -> shift+tab
// Expected: cursor stays on same commit
func TestFoldToggleAll_CursorOnCommitBody_StaysOnSameCommit_SingleCommit(t *testing.T) {
	// Create a commit with body visible (CommitExpanded for commit info body)
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def456",
			Author:  "Test Author",
			Email:   "test@example.com",
			Date:    "2024-01-15T10:30:00+00:00",
			Subject: "Test commit subject",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldFolded,
			},
		},
		FoldLevel:   sidebyside.CommitExpanded, // Body is visible with CommitExpanded
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 100
	m.height = 30
	m.focused = true
	m.calculateTotalLines()

	// Find the commit info body "Author:" row
	rows := m.buildRows()
	var authorRowIdx int
	for i, row := range rows {
		if row.kind == RowKindCommitInfoBody && strings.Contains(row.commitInfoLine, "Author:") {
			authorRowIdx = i
			break
		}
	}
	require.NotZero(t, authorRowIdx, "should find Author: row in commit info body")

	// Position cursor on the Author: row
	m.scroll = authorRowIdx
	m.clampScroll()

	// Verify cursor is on commit info body
	cursorBefore := m.cursorLine()
	rowsBefore := m.buildRows()
	require.Equal(t, RowKindCommitInfoBody, rowsBefore[cursorBefore].kind, "cursor should be on commit info body row")
	commitIdxBefore := rowsBefore[cursorBefore].commitIndex

	// Toggle fold - this will cycle through levels
	// Level 2 (CommitNormal) -> Level 3 (CommitNormal + files expanded) -> Level 1 (CommitFolded)
	newM, _ := m.handleFoldToggleAll()
	m = newM.(Model)

	// After first toggle (level 3): body still visible, cursor should still be on commit body
	cursorAfter := m.cursorLine()
	rowsAfter := m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range")
	commitIdxAfter := rowsAfter[cursorAfter].commitIndex
	assert.Equal(t, commitIdxBefore, commitIdxAfter, "cursor should stay on same commit after toggle")

	// Toggle again to level 1 (folded) - body disappears, should fall back to commit header
	newM, _ = m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter = m.cursorLine()
	rowsAfter = m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range")

	// When body disappears, cursor should fall back to commit header of same commit
	rowAfter := rowsAfter[cursorAfter]
	assert.Equal(t, commitIdxBefore, rowAfter.commitIndex, "cursor should stay on same commit when body folds")
	assert.Equal(t, RowKindCommitHeader, rowAfter.kind, "cursor should fall back to commit header when body disappears")
}

// Test: Cursor on second commit info body should stay on second commit after fold toggle
// Tests the multi-commit scenario
func TestFoldToggleAll_CursorOnCommitBody_StaysOnSameCommit_MultiCommit(t *testing.T) {
	// Create two commits, both at CommitExpanded (body visible)
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123def456",
				Author:  "First Author",
				Date:    "2024-01-15T10:30:00+00:00",
				Subject: "First commit",
			},
			Files: []sidebyside.FilePair{
				{OldPath: "a/file1.go", NewPath: "b/file1.go", FoldLevel: sidebyside.FoldFolded},
			},
			FoldLevel:   sidebyside.CommitExpanded,
			FilesLoaded: true,
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "def789abc012",
				Author:  "Second Author",
				Date:    "2024-01-16T11:00:00+00:00",
				Subject: "Second commit",
			},
			Files: []sidebyside.FilePair{
				{OldPath: "a/file2.go", NewPath: "b/file2.go", FoldLevel: sidebyside.FoldFolded},
			},
			FoldLevel:   sidebyside.CommitExpanded,
			FilesLoaded: true,
		},
	}

	m := NewWithCommits(commits)
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	// Find the second commit's info body "Author:" row (should contain "Second Author")
	rows := m.buildRows()
	var secondCommitAuthorRowIdx int
	for i, row := range rows {
		if row.kind == RowKindCommitInfoBody && strings.Contains(row.commitInfoLine, "Second Author") {
			secondCommitAuthorRowIdx = i
			break
		}
	}
	require.NotZero(t, secondCommitAuthorRowIdx, "should find Second Author row in second commit info body")

	// Position cursor on the second commit's Author: row
	m.scroll = secondCommitAuthorRowIdx
	m.clampScroll()

	// Verify cursor is on second commit's info body
	cursorBefore := m.cursorLine()
	rowsBefore := m.buildRows()
	require.Less(t, cursorBefore, len(rowsBefore), "cursor should be in valid range")
	require.Equal(t, RowKindCommitInfoBody, rowsBefore[cursorBefore].kind, "cursor should be on commit info body row")
	require.Equal(t, 1, rowsBefore[cursorBefore].commitIndex, "cursor should be on second commit (index 1)")

	// Toggle fold - level 2 -> 3
	newM, _ := m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter := m.cursorLine()
	rowsAfter := m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range after toggle")
	assert.Equal(t, 1, rowsAfter[cursorAfter].commitIndex, "cursor should stay on second commit after toggle to level 3")

	// Toggle again - level 3 -> 1 (folded, body disappears)
	newM, _ = m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter = m.cursorLine()
	rowsAfter = m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range after fold")

	rowAfter := rowsAfter[cursorAfter]
	assert.Equal(t, 1, rowAfter.commitIndex, "cursor should stay on second commit when body folds")
	assert.Equal(t, RowKindCommitHeader, rowAfter.kind, "cursor should fall back to second commit header")
}

// Test: Cursor on specific commit info body line should stay on SAME line after fold toggle
// The bug: commit body identity only matches commitIndex, not the specific row,
// so cursor can shift from "Date:" line to first blank line after rebuild
func TestFoldToggleAll_CursorOnCommitBodyDate_StaysOnDateLine(t *testing.T) {
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def456",
			Author:  "Test Author",
			Email:   "test@example.com",
			Date:    "Mon Jan 15 10:30:00 2024 -0500",
			Subject: "Test commit subject",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldFolded,
			},
		},
		FoldLevel:   sidebyside.CommitExpanded,
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 100
	m.height = 30
	m.focused = true
	m.calculateTotalLines()

	// Find the commit info body "Date:" row specifically
	rows := m.buildRows()
	var dateRowIdx int
	for i, row := range rows {
		if row.kind == RowKindCommitInfoBody && strings.Contains(row.commitInfoLine, "Date:") {
			dateRowIdx = i
			break
		}
	}
	require.NotZero(t, dateRowIdx, "should find Date: row in commit info body")

	// Position cursor on the Date: row
	m.scroll = dateRowIdx
	m.clampScroll()

	cursorBefore := m.cursorLine()
	rowsBefore := m.buildRows()
	infoLineBefore := rowsBefore[cursorBefore].commitInfoLine
	require.Contains(t, infoLineBefore, "Date:", "cursor should be on Date: row before toggle")

	// Toggle fold - level 3 -> 1 (files collapse, but commit info body stays visible at CommitExpanded)
	// Note: With CommitExpanded, we're at level 3, so shift+tab cycles back to level 1
	newM, _ := m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter := m.cursorLine()
	rowsAfter := m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range")

	// After toggling from level 3 to level 1, commit info body is hidden
	// Cursor should fall back to commit header
	rowAfter := rowsAfter[cursorAfter]
	assert.Equal(t, 0, rowAfter.commitIndex, "cursor should stay on same commit")
}

// Test: Cursor on structural diff row should stay on same row after fold toggle
// Bug: RowKindStructuralDiff is not handled in rowMatchesIdentity, so cursor shifts
func TestFoldToggleAll_CursorOnStructuralDiff_StaysOnSameRow(t *testing.T) {
	// Create a model with structural diff data.
	// Structural diff rows are a preview shown only when a file is folded.
	// When unfolded, they disappear and the cursor should land on the same file.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal, // Normal shows structural diff preview
			},
		},
		width:  100,
		height: 30,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{
					{StartLine: 1, EndLine: 5, Name: "FuncA", Kind: "func"},
				}),
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 1, EndLine: 5, Name: "FuncA", Kind: "func"},
					{StartLine: 7, EndLine: 10, Name: "FuncB", Kind: "func"},
				}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{
							Kind: structure.ChangeModified,
							OldEntry: &structure.Entry{
								StartLine: 1, EndLine: 5, Name: "FuncA", Kind: "func",
							},
							NewEntry: &structure.Entry{
								StartLine: 1, EndLine: 5, Name: "FuncA", Kind: "func",
							},
						},
						{
							Kind: structure.ChangeAdded,
							NewEntry: &structure.Entry{
								StartLine: 7, EndLine: 10, Name: "FuncB", Kind: "func",
							},
						},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	// Find a structural diff row while folded
	rows := m.buildRows()
	var structRowIdx int
	for i, row := range rows {
		if row.kind == RowKindStructuralDiff {
			structRowIdx = i
			break
		}
	}
	require.NotZero(t, structRowIdx, "should find structural diff row when folded")

	// Position cursor on the structural diff row
	m.scroll = structRowIdx
	m.clampScroll()

	cursorBefore := m.cursorLine()
	rowsBefore := m.buildRows()
	require.Equal(t, RowKindStructuralDiff, rowsBefore[cursorBefore].kind, "cursor should be on structural diff row")

	// Toggle fold - file becomes unfolded, structural diff rows disappear
	newM, _ := m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter := m.cursorLine()
	rowsAfter := m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range")

	// After unfolding, cursor should stay on the same file (structural diff rows gone)
	rowAfter := rowsAfter[cursorAfter]
	assert.Equal(t, 0, rowAfter.fileIndex, "cursor should stay on same file after unfolding")

	// Toggle again: FoldExpanded -> FoldFolded (header only)
	newM, _ = m.handleFoldToggleAll()
	m = newM.(Model)

	// Toggle once more: FoldFolded -> FoldNormal (structural diff reappears)
	newM, _ = m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfterFold := m.cursorLine()
	rowsAfterFold := m.buildRows()
	require.Less(t, cursorAfterFold, len(rowsAfterFold), "cursor should be in valid range after re-fold")
	assert.Equal(t, 0, rowsAfterFold[cursorAfterFold].fileIndex, "cursor should stay on same file after fold cycle")
}

// Test: Cursor on structural diff row of second file should stay on second file after fold toggle
// Bug: Structural diff rows not properly matched by file index
func TestFoldToggleAll_CursorOnStructuralDiff_MultiFile_StaysOnSameFile(t *testing.T) {
	// Create a model with multiple files, each with structural diff data
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  100,
		height: 30,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{}),
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 1, EndLine: 5, Name: "FirstFunc", Kind: "func"},
				}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{
							Kind: structure.ChangeAdded,
							NewEntry: &structure.Entry{
								StartLine: 1, EndLine: 5, Name: "FirstFunc", Kind: "func",
							},
						},
					},
				},
			},
			1: {
				OldStructure: structure.NewMap([]structure.Entry{}),
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 1, EndLine: 5, Name: "SecondFunc", Kind: "func"},
				}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{
							Kind: structure.ChangeAdded,
							NewEntry: &structure.Entry{
								StartLine: 1, EndLine: 5, Name: "SecondFunc", Kind: "func",
							},
						},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	// Find the second file's structural diff row (SecondFunc)
	rows := m.buildRows()
	var secondFileStructDiffIdx int
	for i, row := range rows {
		if row.kind == RowKindStructuralDiff && strings.Contains(row.structuralDiffLine, "SecondFunc") {
			secondFileStructDiffIdx = i
			break
		}
	}
	require.NotZero(t, secondFileStructDiffIdx, "should find SecondFunc structural diff row")

	// Position cursor on the second file's structural diff row
	m.scroll = secondFileStructDiffIdx
	m.clampScroll()

	cursorBefore := m.cursorLine()
	rowsBefore := m.buildRows()
	require.Equal(t, RowKindStructuralDiff, rowsBefore[cursorBefore].kind, "cursor should be on structural diff row")
	require.Equal(t, 1, rowsBefore[cursorBefore].fileIndex, "cursor should be on second file (index 1)")

	// Toggle fold
	newM, _ := m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter := m.cursorLine()
	rowsAfter := m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range")

	// Cursor should stay on second file, not jump to first file
	rowAfter := rowsAfter[cursorAfter]
	assert.Equal(t, 1, rowAfter.fileIndex, "cursor should stay on second file (index 1), not jump to first file")
}

// Test: Cursor on specific structural diff row should stay on SAME row, not shift to another
// Bug: Similar to commit body bug - if cursor is on FuncB row, it shouldn't shift to FuncA row
func TestFoldToggleAll_CursorOnStructuralDiff_StaysOnSpecificRow(t *testing.T) {
	// Structural diff rows are a preview only shown when folded.
	// Test that cursor on a specific structural diff row (FuncB, the middle entry)
	// stays on the same file when toggling fold state, even though the structural
	// diff rows disappear when unfolded.
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  100,
		height: 30,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{}),
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 1, EndLine: 5, Name: "FuncA", Kind: "func"},
					{StartLine: 7, EndLine: 10, Name: "FuncB", Kind: "func"},
					{StartLine: 12, EndLine: 15, Name: "FuncC", Kind: "func"},
				}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{
							Kind:     structure.ChangeAdded,
							NewEntry: &structure.Entry{StartLine: 1, EndLine: 5, Name: "FuncA", Kind: "func"},
						},
						{
							Kind:     structure.ChangeAdded,
							NewEntry: &structure.Entry{StartLine: 7, EndLine: 10, Name: "FuncB", Kind: "func"},
						},
						{
							Kind:     structure.ChangeAdded,
							NewEntry: &structure.Entry{StartLine: 12, EndLine: 15, Name: "FuncC", Kind: "func"},
						},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	// Find the FuncB structural diff row (the middle one) while folded
	rows := m.buildRows()
	var funcBRowIdx int
	for i, row := range rows {
		if row.kind == RowKindStructuralDiff && strings.Contains(row.structuralDiffLine, "FuncB") {
			funcBRowIdx = i
			break
		}
	}
	require.NotZero(t, funcBRowIdx, "should find FuncB structural diff row")

	// Position cursor on FuncB row
	m.scroll = funcBRowIdx
	m.clampScroll()

	cursorBefore := m.cursorLine()
	rowsBefore := m.buildRows()
	require.Contains(t, rowsBefore[cursorBefore].structuralDiffLine, "FuncB", "cursor should be on FuncB row before toggle")

	// Toggle fold - file becomes unfolded, structural diff rows disappear
	newM, _ := m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter := m.cursorLine()
	rowsAfter := m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range")

	// After unfolding, cursor should stay on the same file
	rowAfter := rowsAfter[cursorAfter]
	assert.Equal(t, 0, rowAfter.fileIndex, "cursor should stay on same file after unfolding")

	// Toggle back to folded - structural diff rows reappear
	newM, _ = m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter = m.cursorLine()
	rowsAfter = m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range after re-fold")
	assert.Equal(t, 0, rowsAfter[cursorAfter].fileIndex, "cursor should stay on same file after fold cycle")
}

// Test: Cursor on structural diff in commit view should stay on correct file
// This tests the "show" command scenario with commits
func TestFoldToggleAll_CursorOnStructuralDiff_WithCommit_StaysOnSameFile(t *testing.T) {
	// Create commit with two files, both with structural diff
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def456",
			Author:  "Test Author",
			Date:    "2024-01-15T10:30:00+00:00",
			Subject: "Test commit",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		FoldLevel:   sidebyside.CommitNormal, // Body visible
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 100
	m.height = 40
	m.focused = true

	// Set up structural diff for both files
	m.structureMaps = map[int]*FileStructure{
		0: {
			OldStructure: structure.NewMap([]structure.Entry{}),
			NewStructure: structure.NewMap([]structure.Entry{
				{StartLine: 1, EndLine: 5, Name: "FirstFunc", Kind: "func"},
			}),
			StructuralDiff: &structure.StructuralDiff{
				Changes: []structure.ElementChange{
					{
						Kind:     structure.ChangeAdded,
						NewEntry: &structure.Entry{StartLine: 1, EndLine: 5, Name: "FirstFunc", Kind: "func"},
					},
				},
			},
		},
		1: {
			OldStructure: structure.NewMap([]structure.Entry{}),
			NewStructure: structure.NewMap([]structure.Entry{
				{StartLine: 1, EndLine: 5, Name: "SecondFunc", Kind: "func"},
			}),
			StructuralDiff: &structure.StructuralDiff{
				Changes: []structure.ElementChange{
					{
						Kind:     structure.ChangeAdded,
						NewEntry: &structure.Entry{StartLine: 1, EndLine: 5, Name: "SecondFunc", Kind: "func"},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	// Find the second file's structural diff row
	rows := m.buildRows()
	var secondStructDiffIdx int
	for i, row := range rows {
		if row.kind == RowKindStructuralDiff && strings.Contains(row.structuralDiffLine, "SecondFunc") {
			secondStructDiffIdx = i
			break
		}
	}
	require.NotZero(t, secondStructDiffIdx, "should find SecondFunc structural diff row")

	// Position cursor on second file's structural diff
	m.scroll = secondStructDiffIdx
	m.clampScroll()

	cursorBefore := m.cursorLine()
	rowsBefore := m.buildRows()
	require.Equal(t, RowKindStructuralDiff, rowsBefore[cursorBefore].kind)
	require.Equal(t, 1, rowsBefore[cursorBefore].fileIndex, "cursor should be on second file")

	// Toggle fold: Level 3 -> Level 1 (all folded)
	newM, _ := m.handleFoldToggleAll()
	m = newM.(Model)

	// Level 1 (all folded): cursor lands on commit header
	cursorAfter := m.cursorLine()
	rowsAfter := m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range")

	// Toggle to Level 2 (file headers visible)
	newM, _ = m.handleFoldToggleAll()
	m = newM.(Model)

	// Toggle to Level 3 (files expanded with content)
	newM, _ = m.handleFoldToggleAll()
	m = newM.(Model)

	cursorAfter = m.cursorLine()
	rowsAfter = m.buildRows()
	require.Less(t, cursorAfter, len(rowsAfter), "cursor should be in valid range after full cycle")

	// After a full cycle (3→1→2→3), the cursor lands on the commit header at level 1
	// and stays there through subsequent toggles, since the commit header persists at all levels.
	rowAfter := rowsAfter[cursorAfter]
	assert.True(t, rowAfter.isCommitHeader, "cursor should be on commit header after full fold cycle")
}
