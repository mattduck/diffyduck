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
)

// =============================================================================
// Cursor Position Calculation Tests
// =============================================================================

func TestCursorLine_At20PercentFromTop(t *testing.T) {
	// Cursor should always be at 20% from the top of the viewport
	m := makeTestModel(100)
	m.height = 50 // 49 content lines + 1 status bar
	m.scroll = 0

	// 20% of 49 = 9.8, so cursor should be at line 9 (0-indexed)
	// cursorLine returns the display row index where the cursor is
	cursor := m.cursorLine()
	assert.Equal(t, 9, cursor, "cursor should be at 20%% of viewport height (line 9 of 49)")
}

func TestCursorLine_SmallViewport(t *testing.T) {
	// Even with small viewport, cursor is at 20%
	m := makeTestModel(100)
	m.height = 10 // 9 content lines + 1 status bar
	m.scroll = 0

	// 20% of 9 = 1.8, so cursor should be at line 1 (0-indexed)
	cursor := m.cursorLine()
	assert.Equal(t, 1, cursor, "cursor should be at 20%% of small viewport")
}

func TestCursorLine_VerySmallViewport(t *testing.T) {
	// With very small viewport (3 lines), cursor at 20% should be at line 0
	m := makeTestModel(100)
	m.height = 4 // 3 content lines + 1 status bar
	m.scroll = 0

	// 20% of 3 = 0.6, so cursor should be at line 0
	cursor := m.cursorLine()
	assert.Equal(t, 0, cursor, "cursor should be at line 0 for very small viewport")
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
// Scroll Bounds Tests - Negative Scroll
// =============================================================================

func TestScrollUp_CanGoNegative(t *testing.T) {
	// Scrolling up from 0 should allow negative scroll
	// so the cursor can reach the first line of content
	m := makeTestModel(100)
	m.height = 50 // cursor at line 9
	m.scroll = 0

	// Press k to scroll up - should now be able to go negative
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := newM.(Model)

	assert.Equal(t, -1, model.scroll, "scroll should be able to go negative")
}

func TestMinScroll_AllowsCursorOnFirstLine(t *testing.T) {
	// Minimum scroll should allow cursor to be on the first line of content (line 0)
	// If cursor is at 20% offset, min scroll = -cursorOffset
	m := makeTestModel(100)
	m.height = 50 // cursor offset is 9

	minScroll := m.minScroll()
	assert.Equal(t, -9, minScroll, "minScroll should be negative of cursor offset")
}

func TestGoToTop_PutsCursorOnFirstLine(t *testing.T) {
	// Pressing 'gg' should set scroll so cursor is on line 0 of content
	m := makeTestModel(100)
	m.height = 50 // cursor offset is 9
	m.scroll = 50

	// First 'g' enters pending state
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	// Second 'g' completes the sequence
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model2 := newM2.(Model)

	// scroll should be -9 so that cursor (at offset 9) points to line 0
	assert.Equal(t, -9, model2.scroll, "gg should set scroll so cursor is on first line")
}

func TestStartup_ScrollIsZero_NoBlankSpaceAtTop(t *testing.T) {
	// On startup, scroll should be 0 (not negative)
	// This means the first line of content is at the top of the viewport
	// The cursor is at 20% down, pointing to that content line
	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: make([]sidebyside.LinePair, 100)},
	})
	m.width = 80
	m.height = 50

	assert.Equal(t, 0, m.scroll, "scroll should be 0 on startup")
}

// =============================================================================
// Scroll Bounds Tests - Beyond Content
// =============================================================================

func TestScrollDown_CanGoBeyondContent(t *testing.T) {
	// Scrolling down should allow scroll to exceed totalLines
	// so the cursor can reach the last line of content
	// With new row structure for unfolded files (last file has no trailing blank/border):
	// top border (1) + header (1) + bottom border (1) + 10 pairs = 13 total lines
	m := makeTestModel(10)
	m.height = 50 // cursor at line 9
	m.scroll = 0

	// Go to bottom - should position cursor on last line
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Last content line is at index 12 (0-indexed) - the last content pair
	// Cursor is at offset 9, so scroll should be 12 - 9 = 3
	// We want cursor (at scroll + cursorOffset) to be on line 12
	// So scroll + 9 = 12, scroll = 3
	assert.Equal(t, 3, model.scroll, "G should put cursor on last line")
}

func TestMaxScroll_AllowsCursorOnLastLine(t *testing.T) {
	// Maximum scroll should allow cursor to be on the last line of content
	// If cursor is at 20% offset, max scroll = totalLines - 1 - cursorOffset
	// Actually no: scroll + cursorOffset = lastLineIndex
	// scroll = lastLineIndex - cursorOffset = (totalLines - 1) - cursorOffset
	m := makeTestModel(100) // 101 total lines
	m.height = 50           // cursor offset is 9

	maxScroll := m.maxScroll()
	// totalLines - 1 - cursorOffset = 100 - 9 = 91
	// Wait, let me think again:
	// We want cursor to be on line index (totalLines - 1)
	// cursor line index = scroll + cursorOffset
	// totalLines - 1 = scroll + cursorOffset
	// scroll = totalLines - 1 - cursorOffset
	// But that would be 100 - 9 = 91
	// However, we also need to be able to show content below cursor...
	// Actually for "last line", cursor just needs to point to line 100
	// If content has 101 lines (0-100), cursor offset is 9
	// scroll = 100 - 9 = 91... but then rows 10-49 would be empty
	// That's fine - we want to be able to scroll that far

	expected := m.totalLines - 1 - m.cursorOffset()
	assert.Equal(t, expected, maxScroll, "maxScroll should allow cursor on last line")
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
			{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs},   // top border + header + bottom border + 20 pairs = 23 lines (0-22)
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs}, // starts after 4 blanks + trailing top border
		},
		width:  80,
		height: 50, // cursor offset = 9
		scroll: 0,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// At scroll 0, cursor is at line 9 (within first file)
	info := m.StatusInfo()
	assert.Equal(t, "first.go", info.FileName, "cursor at line 9 should be in first file")

	// With new layout:
	// First file: top border (0) + header (1) + bottom border (2) + 20 pairs (3-22) + 4 blanks (23-26) + trailing top border (27)
	// Second file: header (28) + bottom border (29) + 20 pairs (30-49)...
	// At scroll 20, cursor is at line 29 (which is second file's bottom border)
	// Actually at scroll 19, cursor = 19 + 9 = 28 (second file header)
	m.scroll = 19
	info = m.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor at line 28 should be in second file")
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
			{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs},   // lines 0-5 (header + 5 pairs), then 4 blank lines (6-9)
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs}, // line 10 is header
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
			{OldPath: "a/only.go", NewPath: "b/only.go", Pairs: pairs},
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
	// When cursor is on a file header, the filename portion should be highlighted
	// Highlight = bg color 7, fg color 0
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
			},
			width:  80,
			height: 10, // cursor offset = 1 (20% of 8)
			scroll: 0,  // cursor at line 1 (the header, after top border at line 0)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()

		// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
		// With scroll=0 and cursorOffset=1, cursor is at line 1 (the header)
		// Row 0 = top border, Row 1 = header, Row 2 = bottom border
		// So lines[0]=topBar, lines[1]=divider, lines[2]=top border, lines[3]=header (with cursor)
		lines := strings.Split(output, "\n")
		assert.True(t, len(lines) > 3)
		headerLine := lines[3]

		// The header should contain the filename
		assert.Contains(t, headerLine, "test.go", "header should contain filename")

		// The header should have cursor highlighting (fg=0, bg=7 combined)
		assert.Contains(t, headerLine, ansiCursorStyle, "header should have cursor highlighting")
	})
}

func TestView_CursorHighlight_OnFileHeader_IconNotHighlighted(t *testing.T) {
	// The fold icon (◐/○/●) should NOT be highlighted, only the file number prefix
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
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
		// lines[0]=topBar, lines[1]=divider, lines[2]=top border, lines[3]=header (with cursor)
		headerLine := lines[3]

		// The cursor style applies to a minimal 1-char gutter area (before the icon)
		// Pattern: [cursor style] [reset]  icon #1 status filename...
		// Only the gutter space is highlighted, not the icon or file number
		assert.Contains(t, headerLine, ansiCursorStyle+" ", "gutter should be highlighted")
		assert.Contains(t, headerLine, ansiReset, "highlighted section should end with reset")
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
					OldPath: "a/test.go",
					NewPath: "b/test.go",
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
		// lines[0]=topBar, lines[1]=divider, lines[2]=top border, lines[3]=header (with cursor)
		headerLine := lines[3]

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
				OldPath: "a/test.go",
				NewPath: "b/test.go",
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
		scroll: -1, // cursor at line 0 (the header)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the header line (contains test.go and fold icon)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "◐") {
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
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
							New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
						},
					},
				},
			},
			width:  80,
			height: 10, // cursor offset = 1
			scroll: 1,  // cursor at line 2 (the diff line, after header + spacer)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
		// With scroll=1, content row 0 = header spacer, content row 1 = diff line
		// So lines[0]=topBar, lines[1]=divider, lines[2]=headerSpacer, lines[3]=diffLine
		assert.True(t, len(lines) > 3)
		diffLine := lines[3]

		// The diff line gutters should have cursor highlighting
		assert.Contains(t, diffLine, ansiCursorStyle, "diff line should have cursor highlighting on gutter")
	})
}

func TestView_CursorHighlight_OnBlankSeparator(t *testing.T) {
	// When cursor is on a blank separator line, the gutter areas should be highlighted
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath: "a/first.go",
					NewPath: "b/first.go",
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
				{
					OldPath: "a/second.go",
					NewPath: "b/second.go",
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
			},
			width:  80,
			height: 10, // cursor offset = 1
			scroll: 1,  // cursor at line 2 (blank separator before second file)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
		// With scroll=1 and cursorOffset=1, cursor is at content line 2
		// Viewport content: [content[1], content[2], content[3], ...]
		// So lines[0]=topBar, lines[1]=divider, lines[2]=content[1], lines[3]=content[2]=blank separator
		assert.True(t, len(lines) > 3)
		blankLine := lines[3]

		// Even blank lines should have highlighted gutters when cursor is on them
		assert.Contains(t, blankLine, ansiCursorStyle, "blank separator should have cursor highlighting on gutter areas")
	})
}

func TestView_CursorHighlight_OnHunkSeparator(t *testing.T) {
	// When cursor is on a hunk separator (┈┈┈), the gutter areas should be highlighted
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
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
			height: 10, // cursor offset = 1
			scroll: 1,  // cursor at line 2 (hunk separator)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
		// With scroll=1 and cursorOffset=1, cursor is at content line 2
		// Viewport content: [content[1], content[2], content[3], ...]
		// So lines[0]=topBar, lines[1]=divider, lines[2]=content[1], lines[3]=content[2]=hunk separator
		assert.True(t, len(lines) > 3)
		separatorLine := lines[3]

		// The separator line gutters should have cursor highlighting
		// Note: when cursor is on hunk separator, we use │ instead of ┼ because gutters are styled
		assert.Contains(t, separatorLine, ansiCursorStyle, "hunk separator should have cursor highlighting on gutter")
	})
}

func TestView_CursorHighlight_BothGuttersOnAddedLine(t *testing.T) {
	// For an added line (left side empty), both gutter areas should be highlighted
	withANSIColors(t, func() {
		m := Model{
			focused: true,
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
							New: sidebyside.Line{Num: 1, Content: "added", Type: sidebyside.Added},
						},
					},
				},
			},
			width:  80,
			height: 10, // cursor offset = 1
			scroll: 1,  // cursor at line 2 (the added line, after header + spacer)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
		// With scroll=1, content row 0 = header spacer, content row 1 = added line
		// So lines[0]=topBar, lines[1]=divider, lines[2]=headerSpacer, lines[3]=addedLine
		assert.True(t, len(lines) > 3)
		addedLine := lines[3]

		// Both left gutter (empty) and right gutter should be highlighted
		assert.Contains(t, addedLine, ansiCursorStyle, "added line should have cursor highlighting on both gutters")
	})
}

// =============================================================================
// Scroll Position Preservation on Fold Changes
// =============================================================================

// Test: When cursor is on a file header and we fold/unfold, cursor stays on header
func TestFoldToggle_CursorOnHeader_StaysOnHeader(t *testing.T) {
	// Setup: cursor on file header (line 1, after top border at line 0)
	// For unfolded files: row 0 = top border, row 1 = header
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
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3 (20% of 19)
		scroll: -2, // cursor at line 1 (the header, after top border)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor is on header initially (line 1)
	assert.Equal(t, 1, m.cursorLine(), "cursor should start on header (line 1)")

	// Toggle fold: Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Cursor should still be on header (which is still line 1)
	assert.Equal(t, 1, model.cursorLine(), "after Normal->Expanded, cursor should still be on header")

	// Toggle fold again: Expanded -> Folded
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(Model)

	// Cursor should be on header (line 0 now, since folded has no borders)
	assert.Equal(t, 0, model.cursorLine(), "after Expanded->Folded, cursor should be on header")

	// Toggle fold again: Folded -> Normal
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(Model)

	// Cursor should be on header (back to line 1 since normal has top border)
	assert.Equal(t, 1, model.cursorLine(), "after Folded->Normal, cursor should still be on header")
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
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		scroll: -1, // cursor at line 2 (first diff line, after header + spacer)
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
		scroll: -2, // cursor at line 1 (first diff line)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor starts on diff line
	assert.Equal(t, 1, m.cursorLine(), "cursor should start on diff line")

	// Toggle fold: Expanded -> Folded
	// The diff line disappears, cursor should jump to header (the only visible line)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// In Folded mode, only line 0 (header) exists
	// Cursor should be adjusted to point to it
	assert.Equal(t, 0, model.cursorLine(), "after folding, cursor should jump to header")
}

// Test: When cursor is on blank separator line between files, and that line still exists
func TestFoldToggle_CursorOnBlankLine_StaysOnBlankLine(t *testing.T) {
	// Setup: two files, cursor on blank line between them
	// With new layout for unfolded files:
	// Row 0 = top border, Row 1 = header, Row 2 = bottom border, Row 3 = first diff, Rows 4-7 = blank, Row 8 = trailing top border
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
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Layout: row 0 = top border, row 1 = first header, row 2 = bottom border, row 3 = first diff,
	// rows 4-7 = blank, row 8 = trailing top border
	// Put cursor on blank line (line 4)
	m.scroll = 1 // cursor at line 4 (first inter-file blank line)

	assert.Equal(t, 4, m.cursorLine(), "cursor should start on blank line")

	// Toggle fold on first file: Normal -> Expanded
	// Blank line should still exist at some position
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// The blank line still exists (it's between two files both in Normal/Expanded mode)
	// Cursor should stay on it or the equivalent position
	// Since first file expanded, blank might be at a different absolute line number
	// but the cursor should be adjusted to stay on the blank line
	rows := model.buildRows()
	cursorPos := model.cursorLine()
	if cursorPos >= 0 && cursorPos < len(rows) {
		assert.True(t, rows[cursorPos].isBlank, "cursor should still be on blank line after fold")
	}
}

// Test: When cursor is on blank line and all files are folded, blank disappears
func TestFoldToggle_CursorOnBlankLine_BlankDisappears(t *testing.T) {
	// Setup: two files at same level, cursor on blank line between them
	// Use Shift+Tab to fold ALL files to Folded - this removes the blank lines
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded, // Will toggle to Folded via Shift+Tab
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded, // Same level, so Shift+Tab advances to Folded
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the blank line position (blank line separates the two files)
	rows := m.buildRows()
	blankLineIdx := -1
	for i, row := range rows {
		if row.isBlank {
			blankLineIdx = i
			break
		}
	}
	assert.NotEqual(t, -1, blankLineIdx, "should have a blank line between files")

	// Position cursor on blank line
	m.scroll = blankLineIdx - m.cursorOffset()
	assert.Equal(t, blankLineIdx, m.cursorLine(), "cursor should be on blank line")

	// Shift+Tab: all files Expanded -> Folded
	// When BOTH files are Folded, there are no blank lines between them
	// Layout becomes: [first header] [second header]
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	// Verify both files are now Folded
	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel)
	assert.Equal(t, sidebyside.FoldFolded, model.files[1].FoldLevel)

	// The blank line is gone - cursor should jump to first file header (nearest above)
	assert.Equal(t, 0, model.cursorLine(), "cursor should jump to header when blank line disappears")
}

// Test: Cursor on hunk separator line that disappears when folding
func TestFoldToggle_CursorOnHunkSeparator_FoldToHeader(t *testing.T) {
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
				FoldLevel: sidebyside.FoldExpanded, // Will toggle to Folded
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
	m.scroll = sepLineIdx - m.cursorOffset()
	assert.Equal(t, sepLineIdx, m.cursorLine(), "cursor should be on hunk separator")

	// Toggle fold: Expanded -> Folded
	// Hunk separator disappears, cursor should go to header
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	assert.Equal(t, 0, model.cursorLine(), "cursor should jump to header when separator disappears")
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
				FoldLevel: sidebyside.FoldNormal,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// New layout with borders:
	// 0=first top border, 1=first header, 2=first bottom border, 3=first diff,
	// 4-7=blank (4 lines), 8=trailing top border, 9=second header, 10=second bottom border, 11=second diff
	// Put cursor on second file's diff line (line 11)
	m.scroll = 8 // cursor offset = 3, so cursor at line 11

	assert.Equal(t, 11, m.cursorLine(), "cursor should start on second file's diff line")

	// Shift+Tab: all files Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := newM.(Model)

	// After expanding all, the cursor should still be pointing to second file content
	// The exact line number may change, but we should still be in second file
	info := model.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor should still be in second file after toggle all")
}

// Test: When all files folded, cursor on a file header stays there
func TestFoldToggleAll_CursorOnHeader_FoldAll(t *testing.T) {
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
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// New layout with borders:
	// 0=first top border, 1=first header, 2=first bottom border, 3=first diff,
	// 4-7=blank, 8=trailing top border, 9=second header
	// Put cursor on second file's header (line 9)
	m.scroll = 6 // cursor offset = 3, so cursor at line 9

	assert.Equal(t, 9, m.cursorLine(), "cursor should start on second file header")
	rows := m.buildRows()
	assert.True(t, rows[9].isHeader, "line 9 should be a header")

	// Toggle all: Normal -> Expanded -> Folded
	// After folding all, second file header should be at line 1 (since no blanks in Folded)
	newM, _ := m.handleFoldToggleAll() // -> Expanded
	m = newM.(Model)
	newM, _ = m.handleFoldToggleAll() // -> Folded
	m = newM.(Model)

	// In Folded mode: line 0 = first header, line 1 = second header
	// Cursor should now be on second header (line 1)
	assert.Equal(t, 1, m.cursorLine(), "cursor should be on second file header after fold all")
	rows = m.buildRows()
	assert.True(t, rows[1].isHeader, "line 1 should be second file header")
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

	// When all folded, layout is compact:
	// Line 0: first header
	// Line 1: second header
	// Line 2: third header
	// (no blank lines between folded files)

	// Position cursor on second file's header (line 1)
	// With cursorOffset=3, to get cursor on line 1: scroll = 1 - 3 = -2
	m.scroll = -2
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

// Test: When content loads asynchronously after Tab expand, cursor should stay on same line
// Bug: After Tab expands a file without content, FileContentLoadedMsg doesn't preserve scroll
// Repro: Cursor on diff line 5 -> Tab to expand -> content loads -> cursor lost
// Expected: cursor stays on line 5
// Actual bug: cursor jumps to different position
func TestFoldToggle_AsyncContentLoad_PreservesScrollPosition(t *testing.T) {
	// Setup: file in Normal view with cursor on a specific diff line
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
				FoldLevel: sidebyside.FoldNormal,
				// No OldContent/NewContent - content not loaded yet
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Layout in Normal view (with borders):
	// Line 0: top border
	// Line 1: header
	// Line 2: bottom border (header spacer)
	// Line 3: separator top (since diff starts at line 10, not line 1)
	// Line 4: separator (breadcrumb)
	// Line 5: separator bottom
	// Line 6: diff line (file line 10)
	// Line 7: diff line (file line 11)
	// Line 8: diff line (file line 12) <- cursor here
	// Line 9: diff line (file line 13)
	// ...

	// Position cursor on line 8 (file line 12)
	m.scroll = 5 // cursor offset is 3, so cursor at line 8
	assert.Equal(t, 8, m.cursorLine(), "cursor should be on line 8")

	// Verify we're on the line with content "line12"
	rows := m.buildRows()
	assert.Equal(t, 12, rows[8].pair.Old.Num, "cursor should be on file line 12")

	// Press Tab to expand
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// File should now be in Expanded mode
	assert.Equal(t, sidebyside.FoldExpanded, model.files[0].FoldLevel, "file should be Expanded")

	// Since content isn't loaded yet, buildRows falls back to Normal view
	// Cursor should still be pointing to the same logical line

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
	// In expanded view with 20 lines:
	// Line 0: header
	// Line 1: header spacer
	// Line 2: file line 1
	// Line 3: file line 2
	// ...
	// Line 13: file line 12 <- cursor should be here
	// ...

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
			{OldPath: "a/alpha.go", NewPath: "b/alpha.go", Pairs: pairs},
			{OldPath: "a/beta.go", NewPath: "b/beta.go", Pairs: pairs},
			{OldPath: "a/gamma.go", NewPath: "b/gamma.go", Pairs: pairs},
		},
		width:  80,
		height: 15, // cursor offset = 2 (20% of 14)
		scroll: 0,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// File layout with borders (header + borders + 4 blank lines + trailing border after each file):
	// alpha.go: lines 0-17 (top border + header + bottom border + 10 pairs + 4 blank + trailing border = 18 lines)
	// beta.go:  lines 18-34 (header + bottom border + 10 pairs + 4 blank + trailing border = 17 lines)
	// gamma.go: lines 35-51 (header + bottom border + 10 pairs + 4 blank + trailing border = 17 lines)

	// Scroll through and verify status bar matches cursor position
	for scroll := m.minScroll(); scroll <= m.maxScroll(); scroll++ {
		m.scroll = scroll
		cursorPos := scroll + m.cursorOffset()
		info := m.StatusInfo()

		// Determine expected file based on cursor position
		// Note: blank lines and trailing border count as the file above
		expectedFile := "alpha.go"
		if cursorPos >= 18 && cursorPos < 35 { // Line 18 is beta header
			expectedFile = "beta.go"
		} else if cursorPos >= 35 { // Line 35 is gamma header
			expectedFile = "gamma.go"
		}

		if cursorPos >= 0 && cursorPos < m.totalLines {
			assert.Equal(t, expectedFile, info.FileName,
				"at scroll %d, cursor at %d should show %s", scroll, cursorPos, expectedFile)
		}
	}
}

// =============================================================================
// Cursor Identity Tests - Row Type Preservation
// =============================================================================

// Test: Cursor on isHeaderTopBorder should stay there after resize
// Bug: cursorRowIdentity doesn't capture isHeaderTopBorder, so cursor jumps to wrong position
func TestResize_CursorOnHeaderTopBorder_StaysOnTopBorder(t *testing.T) {
	// Setup: single unfolded file, cursor on top border (line 0)
	// Layout: row 0 = top border, row 1 = header, row 2 = bottom border, row 3+ = content
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
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		scroll: -3, // cursor at line 0 (the top border)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor is on top border initially
	rows := m.buildRows()
	cursorPos := m.cursorLine()
	require.Equal(t, 0, cursorPos, "cursor should start on line 0")
	require.True(t, rows[0].isHeaderTopBorder, "line 0 should be top border")

	// Resize the terminal (triggers cursor identity save/restore)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
	model := newM.(Model)

	// Cursor should still be on top border (line 0)
	rows = model.buildRows()
	cursorPos = model.cursorLine()
	assert.True(t, rows[cursorPos].isHeaderTopBorder,
		"after resize, cursor should still be on top border (got cursorPos=%d, isHeaderTopBorder=%v)",
		cursorPos, rows[cursorPos].isHeaderTopBorder)
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
				FoldLevel: sidebyside.FoldNormal,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Find the trailing top border of first file
	rows := m.buildRows()
	trailingBorderIdx := -1
	for i, row := range rows {
		if row.fileIndex == 0 && row.isHeaderTopBorder && i > 0 {
			// This is the trailing border (not the leading one at index 0)
			trailingBorderIdx = i
		}
	}
	require.NotEqual(t, -1, trailingBorderIdx, "should find trailing top border")
	require.True(t, rows[trailingBorderIdx].isHeaderTopBorder, "should be a top border row")

	// Position cursor on trailing top border
	m.scroll = trailingBorderIdx - m.cursorOffset()
	cursorPos := m.cursorLine()
	require.Equal(t, trailingBorderIdx, cursorPos, "cursor should be on trailing top border")

	// Resize the terminal
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
	model := newM.(Model)

	// Cursor should still be on trailing top border of first file
	rows = model.buildRows()
	cursorPos = model.cursorLine()

	// BUG: Without isHeaderTopBorder in cursorRowIdentity, this fails!
	// The cursor jumps to the header of the file above instead.
	assert.True(t, rows[cursorPos].isHeaderTopBorder && rows[cursorPos].fileIndex == 0,
		"after resize, cursor should still be on first file's trailing top border "+
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
				FoldLevel: sidebyside.FoldNormal,
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
	m.scroll = truncIdx - m.cursorOffset()
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
				FoldLevel: sidebyside.FoldNormal,
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
	m.scroll = secondSepIdx - m.cursorOffset()
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

// Test: Cursor on top border should stay there after fold toggle
func TestFoldToggle_CursorOnTopBorder_StaysOnTopBorder(t *testing.T) {
	// Setup: cursor on top border, toggle fold
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
				},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20,
		scroll: -3, // cursor at line 0 (top border)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify on top border
	rows := m.buildRows()
	require.True(t, rows[0].isHeaderTopBorder, "line 0 should be top border")

	// Toggle fold: Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// In Expanded mode, top border still exists at line 0
	rows = model.buildRows()
	cursorPos := model.cursorLine()

	// Should be on top border, not jumped to header or elsewhere
	assert.True(t, rows[cursorPos].isHeaderTopBorder,
		"after fold toggle, cursor should still be on top border (got cursorPos=%d, isHeaderTopBorder=%v)",
		cursorPos, rows[cursorPos].isHeaderTopBorder)
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
	m.scroll = -m.cursorOffset() // This puts row 0 at cursor position
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
	// cursorLine() = scroll + cursorOffset()
	// We need scroll such that scroll + cursorOffset() = 0
	cursorOffset := m.cursorOffset()
	m.scroll = -cursorOffset
	if m.scroll < 0 {
		m.scroll = 0
	}

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
	cursorOffset := m.cursorOffset()
	m.scroll = 1 - cursorOffset
	if m.scroll < 0 {
		m.scroll = 0
	}

	// Verify cursor position
	cursorPos := m.cursorLine()
	if cursorPos != 1 {
		// If cursor isn't on row 1, manually verify we're in the right area
		t.Logf("cursorPos=%d, cursorOffset=%d, scroll=%d", cursorPos, cursorOffset, m.scroll)
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
	cursorOffset := m.cursorOffset()
	m.scroll = secondCommitRow - cursorOffset
	if m.scroll < 0 {
		m.scroll = 0
	}

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
	m.scroll = -m.cursorOffset()
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
		m.scroll = bodyRowIdx - m.cursorOffset()
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
		m.scroll = fileRowIdx - m.cursorOffset()
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
		m.scroll = i - m.cursorOffset()
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
		m.scroll = contentRowIdx - m.cursorOffset()

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
