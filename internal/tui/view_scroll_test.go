package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
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
	// With new cursor model: cursorLine = scroll = 0
	// CurrentLine = cursorLine + 1 = 1
	assert.Equal(t, 1, info.CurrentLine)
	assert.Equal(t, 53, info.TotalLines) // In diff view: 1 header + 1 bottom border + 50 pairs + 1 blank margin (no top border for first file)
	// Percentage: cursorLine(0) / maxCursor(51) * 100 = 0%
	assert.Equal(t, 0, info.Percentage)
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
			{OldPath: "a/small.go", NewPath: "b/small.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
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
			{OldPath: "a/first.go", NewPath: "b/first.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs1},
			{OldPath: "a/second.go", NewPath: "b/second.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs2},
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

	// Scroll to file 2's header
	// File 1: header (0) + bottom border (1) + 20 pairs (2-21) + 4 blanks (22-25) + top border (26)
	// File 2: header (27) + ...
	m.scroll = 27
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
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs:     pairs,
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
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldExpanded,
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
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
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
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
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
	m.scroll = 50 // cursor at 50
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
			{OldPath: "a/first.go", NewPath: "b/first.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
			{OldPath: "a/second.go", NewPath: "b/second.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Build rows to find actual file boundaries dynamically
	rows := m.buildRows()
	fileNames := []string{"first.go", "second.go"}

	// Find the header row of the second file (skip top border)
	var secondFileHeaderRow int
	for i, row := range rows {
		if row.fileIndex == 1 && row.isHeader {
			secondFileHeaderRow = i
			break
		}
	}
	require.NotZero(t, secondFileHeaderRow, "should find second file header")

	// Test: two rows before second file's header (blank line) should show first.go
	m.scroll = secondFileHeaderRow - 2
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "blank line before second file should show file 1")
	assert.Equal(t, fileNames[0], info.FileName)

	// Test: top border row should show previous file (first.go)
	m.scroll = secondFileHeaderRow - 1
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile, "top border should show file above")
	assert.Equal(t, fileNames[0], info.FileName)

	// Test: header row of second file should show second.go
	m.scroll = secondFileHeaderRow
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile, "header of second file should show file 2")
	assert.Equal(t, fileNames[1], info.FileName)
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
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs:     pairs,
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
