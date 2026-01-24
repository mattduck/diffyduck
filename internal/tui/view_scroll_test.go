package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestStatusInfo_SingleFile(t *testing.T) {
	pairs := make([]sidebyside.LinePair, 50)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "content"},
			New: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 20,
		scroll: 0,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	info := m.StatusInfo()

	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, 1, info.TotalFiles)
	assert.Equal(t, "test.go", info.FileName)
	// With cursor-based positioning: height=20, contentHeight=19, cursorOffset=3
	// cursorLine = scroll(0) + cursorOffset(3) = 3
	// CurrentLine = cursorLine + 1 = 4
	assert.Equal(t, 4, info.CurrentLine)
	assert.Equal(t, 53, info.TotalLines) // 1 top border + 1 header + 1 bottom border + 50 pairs (no trailing blank/border for last file)
	// Percentage: cursorLine(3) / maxCursor(52) * 100 = 5%
	assert.Equal(t, 5, info.Percentage)
	assert.False(t, info.AtEnd)
}

func TestStatusInfo_AtEnd(t *testing.T) {
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
			{OldPath: "a/small.go", NewPath: "b/small.go", Pairs: pairs},
		},
		width:  80,
		height: 20, // bigger than content (11 lines)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Set scroll to maxScroll so cursor is at the end
	m.scroll = m.maxScroll()

	info := m.StatusInfo()

	assert.Equal(t, 100, info.Percentage)
	assert.True(t, info.AtEnd)
}

func TestStatusInfo_MultipleFiles(t *testing.T) {
	pairs1 := make([]sidebyside.LinePair, 20)
	pairs2 := make([]sidebyside.LinePair, 20)
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

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs1},
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs2},
		},
		width:  80,
		height: 20,
		scroll: 0,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// At start - should be in file 1
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, 2, info.TotalFiles)
	assert.Equal(t, "first.go", info.FileName)

	// Scroll into file 2 (file 1 has 21 lines: 1 header + 20 pairs)
	m.scroll = 25
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile)
	assert.Equal(t, "second.go", info.FileName)
}

func TestView_StatusBarContent(t *testing.T) {
	// Create enough content that scrolling to end keeps cursor on file content, not summary
	pairs := make([]sidebyside.LinePair, 20)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
			New: sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
		}
	}
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs:   pairs,
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on last file content line (not summary)
	// totalLines = 1 header + 20 pairs + 1 summary = 22
	// cursorOffset = (10-2)*20/100 = 1
	// To put cursor on line 20 (last pair), scroll = 20 - cursorOffset = 19
	m.scroll = 19

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should contain the file name (file info moved from bottom to top bar)
	assert.Contains(t, topBar, "foo.go")
}

func TestStatusInfo_DeletedFile(t *testing.T) {
	// When a file is deleted, newPath is /dev/null, so we should show oldPath
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/deleted.go",
				NewPath: "/dev/null",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "deleted"},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	info := m.StatusInfo()
	// Should show the old path (without a/ prefix) since file was deleted
	assert.Equal(t, "deleted.go", info.FileName)
}

func TestStatusInfo_ScrollPastAllContent(t *testing.T) {
	// When scrolled past all content, should still show the last file
	pairs := make([]sidebyside.LinePair, 5)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "line"},
			New: sidebyside.Line{Num: i + 1, Content: "line"},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 10,
		scroll: 100, // Way past the content (only 6 lines: 1 header + 5 pairs)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.clampScroll() // This should clamp to maxScroll

	info := m.StatusInfo()
	// When scrolled to the very end, cursor lands on the file content (no summary row)
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "test.go", info.FileName)
	assert.True(t, info.AtEnd)
}

func TestStatusInfo_PercentageAccuracy(t *testing.T) {
	// Create 102 lines (100 pairs + 1 header + 1 summary)
	pairs := make([]sidebyside.LinePair, 100)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "line"},
			New: sidebyside.Line{Num: i + 1, Content: "line"},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 11, // 10 content lines + 1 status bar
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines() // 103 lines total (1 top + 1 header + 1 bottom + 100 pairs, no trailing blank/border for last file)
	// cursorOffset = 10 * 20 / 100 = 2, maxCursor = 102

	// At minScroll, cursor is at line 0, percentage should be 0
	m.scroll = m.minScroll()
	info := m.StatusInfo()
	assert.Equal(t, 0, info.Percentage)
	assert.False(t, info.AtEnd)

	// At scroll that puts cursor at approx line 50, percentage should be ~49
	// (50/102 * 100 = 49.0, rounded to 49)
	m.scroll = 50 - m.cursorOffset() // cursor at 50
	info = m.StatusInfo()
	assert.Equal(t, 49, info.Percentage)
	assert.False(t, info.AtEnd)

	// At maxScroll, cursor is at last line, percentage should be 100
	m.scroll = m.maxScroll()
	info = m.StatusInfo()
	assert.Equal(t, 100, info.Percentage)
	assert.True(t, info.AtEnd)
}

func TestStatusInfo_FileBoundary(t *testing.T) {
	// Test that current file updates correctly at exact boundaries
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
			{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs},   // lines 0-17 (top border + header + bottom + 10 pairs + 4 blank + trailing)
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs}, // line 18 is header
		},
		width:  80,
		height: 10, // contentHeight=8, cursorOffset=1
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// With cursorOffset=1:
	// scroll=11 → cursor at line 12 (last pair of first file) → first.go
	m.scroll = 11
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "first.go", info.FileName)

	// scroll=12 → cursor at line 13 (first blank after first file) → first.go (blank belongs to file above)
	m.scroll = 12
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "first.go", info.FileName)

	// scroll=16 → cursor at line 17 (top border of second file) → second.go
	// The top border now belongs to the file it precedes (file 1), not the file above
	m.scroll = 16
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile)
	assert.Equal(t, "second.go", info.FileName)

	// scroll=17 → cursor at line 18 (header of second file) → second.go
	m.scroll = 17
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile)
	assert.Equal(t, "second.go", info.FileName)
}

func TestStatusBar_NonShrinkingWidth(t *testing.T) {
	// Create a file with many lines to get large line numbers
	pairs := make([]sidebyside.LinePair, 1000)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
			New: sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs:   pairs,
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Navigate to a high line number to establish max width
	m.scroll = 500
	bar1 := m.renderStatusBar()

	// Navigate back to start
	m.scroll = m.minScroll()
	bar2 := m.renderStatusBar()

	// The less indicator part should have the same width in both cases
	// Extract the "line X/Y Z%" portion and compare widths
	// The max width should be maintained (padded with trailing spaces)
	assert.Equal(t, len(bar1), len(bar2), "status bar should maintain consistent width")
}
