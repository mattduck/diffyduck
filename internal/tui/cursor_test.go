package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
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
	// Pressing 'g' should set scroll so cursor is on line 0 of content
	m := makeTestModel(100)
	m.height = 50 // cursor offset is 9
	m.scroll = 50

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)

	// scroll should be -9 so that cursor (at offset 9) points to line 0
	assert.Equal(t, -9, model.scroll, "g should set scroll so cursor is on first line")
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
	m := makeTestModel(10) // 11 total lines (header + 10 pairs)
	m.height = 50          // cursor at line 9
	m.scroll = 0

	// Go to bottom - should position cursor on last line
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Last content line is at index 10 (0-indexed)
	// Cursor is at offset 9, so scroll should be 10 - 9 = 1
	// Wait, that's not right. Let me reconsider.
	// We want cursor (at scroll + cursorOffset) to be on line 10
	// So scroll + 9 = 10, scroll = 1
	// But actually with 11 lines and viewport of 50, we'd see everything
	// Let me test with smaller content

	// cursor at index 9, we want it to point to line 10 (last line)
	// scroll + cursorOffset = lineIndex
	// scroll + 9 = 10
	// scroll = 1
	assert.Equal(t, 1, model.scroll, "G should put cursor on last line")
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

	// At scroll 15, cursor is at line 24 (within second file)
	// First file ends at line 20 (header + 20 pairs = 21 lines, indices 0-20)
	// Line 21 is blank before second file, line 22 is second file header
	// So cursor at line 24 should be in second file
	m.scroll = 15
	info = m.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor at line 24 should be in second file")
}

func TestStatusInfo_CursorOnBlankLine_CountsAsFileBelow(t *testing.T) {
	// When cursor is on a blank line between files, it should count as the file below
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs},   // lines 0-5 (header + 5 pairs)
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs}, // line 6 is blank, line 7 is header
		},
		width:  80,
		height: 10, // cursor offset = 1 (20% of 9)
		scroll: 5,  // cursor at line 6 (blank line before second file)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	info := m.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "cursor on blank line should count as file below")
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
	// lipgloss combines fg and bg into single escape: \x1b[30;47m = fg black + bg white
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
			height: 10, // cursor offset = 1 (20% of 9)
			scroll: -1, // cursor at line 0 (the header)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()

		// With scroll=-1, viewport row 0 is blank padding, row 1 is the header
		lines := strings.Split(output, "\n")
		assert.True(t, len(lines) > 1)
		headerLine := lines[1]

		// The header should contain the filename
		assert.Contains(t, headerLine, "test.go", "header should contain filename")

		// The header should have cursor highlighting (fg=0, bg=7 combined)
		assert.Contains(t, headerLine, ansiCursorStyle, "header should have cursor highlighting")
	})
}

func TestView_CursorHighlight_OnFileHeader_IconNotHighlighted(t *testing.T) {
	// The fold icon (◐/○/●) should NOT be highlighted, only the "═══ " prefix
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
		headerLine := lines[1]

		// The cursor style should end before the space and icon
		// Pattern: [cursor style]═══[reset][header style] ◐ filename...
		// Only the "═══" part is highlighted, not the space or icon
		assert.Contains(t, headerLine, ansiCursorStyle+"═══"+ansiReset, "only the ═══ should be highlighted, not the space or icon")
	})
}

func TestView_CursorHighlight_OnFileHeader_SeparatorAligned(t *testing.T) {
	// When cursor is on a Normal/Expanded header, the │ separator should align with diff lines
	// Explicitly disable colors so we can compare character positions directly
	lipgloss.SetColorProfile(termenv.Ascii)
	defer lipgloss.SetColorProfile(termenv.Ascii) // keep disabled after test

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

	// Find the header line and a diff line
	headerLine := lines[1] // header (cursor is here)
	diffLine := lines[2]   // first diff line

	// Find display column of │ in both lines by measuring width up to the separator
	headerSepPos := displayColumnOf(headerLine, "│")
	diffSepPos := displayColumnOf(diffLine, "│")

	assert.NotEqual(t, -1, headerSepPos, "header should have │ separator")
	assert.NotEqual(t, -1, diffSepPos, "diff line should have │ separator")
	assert.Equal(t, diffSepPos, headerSepPos, "│ separator should be at same display column in header and diff line")
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
			scroll: 0,  // cursor at line 1 (the diff line)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Line 1 (index 1) should have the diff content with highlighted gutters
		assert.True(t, len(lines) > 1)
		diffLine := lines[1]

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

		// With scroll=1 and cursorOffset=1, cursor is at content line 2
		// Viewport: [content[1], content[2], content[3], ...]
		// So cursor row in viewport is index 1 (content[2] = blank separator)
		assert.True(t, len(lines) > 1)
		blankLine := lines[1]

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

		// With scroll=1 and cursorOffset=1, cursor is at content line 2
		// Viewport: [content[1], content[2], content[3], ...]
		// So cursor row in viewport is index 1 (content[2] = hunk separator)
		assert.True(t, len(lines) > 1)
		separatorLine := lines[1]

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
			scroll: 0,  // cursor at line 1 (the added line)
			keys:   DefaultKeyMap(),
		}
		m.calculateTotalLines()

		output := m.View()
		lines := strings.Split(output, "\n")

		// Line 1 should have both left (empty) and right gutters highlighted
		assert.True(t, len(lines) > 1)
		addedLine := lines[1]

		// Both left gutter (empty) and right gutter should be highlighted
		assert.Contains(t, addedLine, ansiCursorStyle, "added line should have cursor highlighting on both gutters")
	})
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

	// File layout:
	// alpha.go: lines 0-10 (header + 10 pairs = 11 lines)
	// blank:    line 11 (counts as beta.go - file below)
	// beta.go:  lines 12-22 (header + 10 pairs = 11 lines)
	// blank:    line 23 (counts as gamma.go - file below)
	// gamma.go: lines 24-34 (header + 10 pairs = 11 lines)

	// Scroll through and verify status bar matches cursor position
	for scroll := m.minScroll(); scroll <= m.maxScroll(); scroll++ {
		m.scroll = scroll
		cursorPos := scroll + m.cursorOffset()
		info := m.StatusInfo()

		// Determine expected file based on cursor position
		// Note: blank separator lines count as the file below
		expectedFile := "alpha.go"
		if cursorPos >= 11 && cursorPos < 23 { // Line 11 is blank before beta, counts as beta
			expectedFile = "beta.go"
		} else if cursorPos >= 23 { // Line 23 is blank before gamma, counts as gamma
			expectedFile = "gamma.go"
		}

		if cursorPos >= 0 && cursorPos < m.totalLines {
			assert.Equal(t, expectedFile, info.FileName,
				"at scroll %d, cursor at %d should show %s", scroll, cursorPos, expectedFile)
		}
	}
}
