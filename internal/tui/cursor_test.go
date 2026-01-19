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
	m := makeTestModel(10) // 17 total lines (header + spacer + 10 pairs + 4 blank + summary)
	m.height = 50          // cursor at line 9
	m.scroll = 0

	// Go to bottom - should position cursor on last line
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Last content line is at index 16 (0-indexed) - the summary row
	// Cursor is at offset 9, so scroll should be 16 - 9 = 7
	// We want cursor (at scroll + cursorOffset) to be on line 16
	// So scroll + 9 = 16, scroll = 7
	assert.Equal(t, 7, model.scroll, "G should put cursor on last line")
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
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs},   // lines 0-20
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs}, // lines 21-42
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

	// At scroll 17, cursor is at line 26 (within second file)
	// First file: header + 20 pairs = 21 lines (indices 0-20)
	// Blank lines: 4 lines after first file (indices 21-24, belong to first file)
	// Second file header: line 25
	// So cursor at line 26 should be in second file
	m.scroll = 17
	info = m.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor at line 26 should be in second file")
}

func TestStatusInfo_CursorOnBlankLine_CountsAsFileAbove(t *testing.T) {
	// When cursor is on a blank line between files, it should count as the file above
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
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
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
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
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Left:  sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							Right: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
			},
			width:  80,
			height: 10, // cursor offset = 1 (20% of 8)
			scroll: -1, // cursor at line 0 (the header)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()

		// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
		// With scroll=-1, content row 0 is blank padding, content row 1 is the header
		// So lines[0]=topBar, lines[1]=divider, lines[2]=blank, lines[3]=header
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
	// The fold icon (◐/○/●) should NOT be highlighted, only the "━━━ " prefix
	withANSIColors(t, func() {
		m := Model{
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Left:  sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							Right: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
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
		// lines[0]=topBar, lines[1]=divider, lines[2]=blank, lines[3]=header
		headerLine := lines[3]

		// The cursor style should end before the space and icon
		// Pattern: [cursor style]━━━━[reset][header style] ◐ filename...
		// Only the gutter (━'s) is highlighted, not the space or icon
		// Gutter width is dynamic (minimum 4), so check for at least ━━━━
		assert.Contains(t, headerLine, ansiCursorStyle+"━━━━", "gutter should be highlighted")
		assert.Contains(t, headerLine, ansiReset, "highlighted section should end with reset")
	})
}

func TestView_FileHeader_SpansFullWidth(t *testing.T) {
	// File headers should span the full terminal width without a │ separator
	// (separators are only for diff content lines)
	lipgloss.SetColorProfile(termenv.Ascii)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "left content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "right content", Type: sidebyside.Context},
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

	// Find the header line (contains test.go and ━ or ─)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && (strings.Contains(line, "━") || strings.Contains(line, "─")) {
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
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
							Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
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
			files: []sidebyside.FilePair{
				{
					OldPath: "a/first.go",
					NewPath: "b/first.go",
					Pairs: []sidebyside.LinePair{
						{
							Left:  sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							Right: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						},
					},
				},
				{
					OldPath: "a/second.go",
					NewPath: "b/second.go",
					Pairs: []sidebyside.LinePair{
						{
							Left:  sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
							Right: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
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
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Left:  sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
							Right: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						},
						// Gap - next line is 100, so there will be a hunk separator
						{
							Left:  sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
							Right: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
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
			files: []sidebyside.FilePair{
				{
					OldPath: "a/test.go",
					NewPath: "b/test.go",
					Pairs: []sidebyside.LinePair{
						{
							Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
							Right: sidebyside.Line{Num: 1, Content: "added", Type: sidebyside.Added},
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
	// Setup: cursor on file header (line 0)
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}},
					{Left: sidebyside.Line{Num: 2, Content: "line2"}, Right: sidebyside.Line{Num: 2, Content: "line2"}},
				},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3 (20% of 19)
		scroll: -3, // cursor at line 0 (the header)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Verify cursor is on header initially
	assert.Equal(t, 0, m.cursorLine(), "cursor should start on header (line 0)")

	// Toggle fold: Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Cursor should still be on header (which is still line 0)
	assert.Equal(t, 0, model.cursorLine(), "after Normal->Expanded, cursor should still be on header")

	// Toggle fold again: Expanded -> Folded
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(Model)

	// Cursor should still be on header (line 0, now the only line)
	assert.Equal(t, 0, model.cursorLine(), "after Expanded->Folded, cursor should still be on header")

	// Toggle fold again: Folded -> Normal
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(Model)

	// Cursor should still be on header
	assert.Equal(t, 0, model.cursorLine(), "after Folded->Normal, cursor should still be on header")
}

// Test: When cursor is on a diff line that remains visible, cursor stays on it
func TestFoldToggle_CursorOnDiffLine_StaysOnDiffLine(t *testing.T) {
	// Setup: cursor on diff line (line 2 = first diff line after header + spacer)
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}},
					{Left: sidebyside.Line{Num: 2, Content: "line2"}, Right: sidebyside.Line{Num: 2, Content: "line2"}},
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
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}},
					{Left: sidebyside.Line{Num: 2, Content: "line2"}, Right: sidebyside.Line{Num: 2, Content: "line2"}},
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
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Layout: line 0 = first header, line 1 = header spacer, line 2 = first diff, lines 3-6 = blank, line 7 = second header
	// Put cursor on blank line (line 3)
	m.scroll = 0 // cursor at line 3 (first inter-file blank line)

	assert.Equal(t, 3, m.cursorLine(), "cursor should start on blank line")

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
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldExpanded, // Will toggle to Folded via Shift+Tab
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
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
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}},
					// Gap - next line number is 100, creating a hunk separator
					{Left: sidebyside.Line{Num: 100, Content: "line100"}, Right: sidebyside.Line{Num: 100, Content: "line100"}},
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
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20, // cursor offset = 3
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Layout: 0=first header, 1=first diff, 2-5=blank (4 lines), 6=second header, 7=second diff
	// Put cursor on second file's diff line (line 7)
	m.scroll = 4 // cursor at line 7

	assert.Equal(t, 7, m.cursorLine(), "cursor should start on second file's diff line")

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
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldNormal,
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Put cursor on second file's header (line 7 with spacer + 4 blank lines after first file)
	// Layout: line 0=first header, line 1=spacer, line 2=first content, lines 3-6=blank, line 7=second header
	m.scroll = 4 // cursor offset = 3, so cursor at line 7

	assert.Equal(t, 7, m.cursorLine(), "cursor should start on second file header")
	rows := m.buildRows()
	assert.True(t, rows[7].isHeader, "line 7 should be a header")

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
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldFolded,
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
				FoldLevel: sidebyside.FoldFolded,
			},
			{
				OldPath:   "a/third.go",
				NewPath:   "b/third.go",
				Pairs:     []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1, Content: "line1"}, Right: sidebyside.Line{Num: 1, Content: "line1"}}},
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
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{Left: sidebyside.Line{Num: 10, Content: "line10"}, Right: sidebyside.Line{Num: 10, Content: "line10"}},
					{Left: sidebyside.Line{Num: 11, Content: "line11"}, Right: sidebyside.Line{Num: 11, Content: "line11"}},
					{Left: sidebyside.Line{Num: 12, Content: "line12"}, Right: sidebyside.Line{Num: 12, Content: "line12"}},
					{Left: sidebyside.Line{Num: 13, Content: "line13"}, Right: sidebyside.Line{Num: 13, Content: "line13"}},
					{Left: sidebyside.Line{Num: 14, Content: "line14"}, Right: sidebyside.Line{Num: 14, Content: "line14"}},
					{Left: sidebyside.Line{Num: 15, Content: "line15"}, Right: sidebyside.Line{Num: 15, Content: "line15"}},
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

	// Layout in Normal view:
	// Line 0: header
	// Line 1: header spacer
	// Line 2: diff line (file line 10)
	// Line 3: diff line (file line 11)
	// Line 4: diff line (file line 12) <- cursor here
	// Line 5: diff line (file line 13)
	// ...

	// Position cursor on line 4 (file line 12)
	m.scroll = 1 // cursor offset is 3, so cursor at line 4
	assert.Equal(t, 4, m.cursorLine(), "cursor should be on line 4")

	// Verify we're on the line with content "line12"
	rows := m.buildRows()
	assert.Equal(t, 12, rows[4].pair.Left.Num, "cursor should be on file line 12")

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
		assert.Equal(t, 12, row.pair.Left.Num,
			"after content loads, cursor should still be on file line 12 (got line %d at cursor pos %d)",
			row.pair.Left.Num, cursorPos)
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
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
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

	// File layout (header spacer + 4 blank lines after each file):
	// alpha.go: lines 0-11 (header + spacer + 10 pairs = 12 lines)
	// blank:    lines 12-15 (4 lines, count as alpha.go - file above)
	// beta.go:  lines 16-27 (header + spacer + 10 pairs = 12 lines)
	// blank:    lines 28-31 (4 lines, count as beta.go - file above)
	// gamma.go: lines 32-43 (header + spacer + 10 pairs = 12 lines)
	// blank:    lines 44-47 (4 lines, count as gamma.go - file above)
	// summary:  line 48

	// Scroll through and verify status bar matches cursor position
	for scroll := m.minScroll(); scroll <= m.maxScroll(); scroll++ {
		m.scroll = scroll
		cursorPos := scroll + m.cursorOffset()
		info := m.StatusInfo()

		// Determine expected file based on cursor position
		// Note: blank lines count as the file above
		// Summary row (last line) has no file info
		expectedFile := "alpha.go"
		if cursorPos >= 16 && cursorPos < 32 { // Line 16 is beta header
			expectedFile = "beta.go"
		} else if cursorPos >= 32 && cursorPos < m.totalLines-1 { // Line 32 is gamma header
			expectedFile = "gamma.go"
		} else if cursorPos == m.totalLines-1 { // Summary row
			expectedFile = ""
		}

		if cursorPos >= 0 && cursorPos < m.totalLines {
			assert.Equal(t, expectedFile, info.FileName,
				"at scroll %d, cursor at %d should show %s", scroll, cursorPos, expectedFile)
		}
	}
}
