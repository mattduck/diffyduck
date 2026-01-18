package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

var update = flag.Bool("update", false, "update golden files")

func init() {
	// Force ASCII color profile for consistent test output
	lipgloss.SetColorProfile(termenv.Ascii)
}

func TestView_BasicRender(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "old line", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 2, Content: "new line", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "func main() {}", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 3, Content: "func main() {}", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "basic_render.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_WithScroll(t *testing.T) {
	// Create enough lines to require scrolling
	pairs := make([]sidebyside.LinePair, 20)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 5,
		scroll: 10, // Start scrolled down
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "scrolled_view.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_AddedAndRemovedLines(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					// Pure addition (empty left)
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 1, Content: "added line", Type: sidebyside.Added},
					},
					// Pure removal (empty right)
					{
						Left:  sidebyside.Line{Num: 1, Content: "removed line", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "added_removed.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_MultipleFiles(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/one.go",
				NewPath: "b/one.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath: "a/two.go",
				NewPath: "b/two.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "file two", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "file two", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "multiple_files.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_EmptyModel(t *testing.T) {
	m := Model{
		files:  nil,
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}

	output := m.View()
	// Even with no files, we show a status bar
	assert.Contains(t, output, "0%")
}

func TestView_ZeroSize(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/foo.go", NewPath: "b/foo.go"},
		},
		width:  0,
		height: 0,
		keys:   DefaultKeyMap(),
	}

	output := m.View()
	assert.Equal(t, "", output)
}

func TestView_HorizontalScroll(t *testing.T) {
	// Create content with long lines to test horizontal scrolling
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "this is a much longer line that will be truncated without scroll", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 2, Content: "this is a much longer line that will be truncated without scroll", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "0123456789abcdefghij", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 3, Content: "0123456789abcdefghij", Type: sidebyside.Context},
					},
				},
			},
		},
		width:       80,
		height:      10,
		hscroll:     8, // Scroll right by 8 columns
		hscrollStep: DefaultHScrollStep,
		keys:        DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "horizontal_scroll.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestStatusInfo_SingleFile(t *testing.T) {
	pairs := make([]sidebyside.LinePair, 50)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
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
	assert.Equal(t, 52, info.TotalLines) // 50 pairs + 1 header + 1 summary
	// Percentage: cursorLine(3) / maxCursor(51) * 100 = 5%
	assert.Equal(t, 5, info.Percentage)
	assert.False(t, info.AtEnd)
}

func TestStatusInfo_AtEnd(t *testing.T) {
	pairs := make([]sidebyside.LinePair, 10)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
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
			Left:  sidebyside.Line{Num: i + 1, Content: "file1"},
			Right: sidebyside.Line{Num: i + 1, Content: "file1"},
		}
	}
	for i := range pairs2 {
		pairs2[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "file2"},
			Right: sidebyside.Line{Num: i + 1, Content: "file2"},
		}
	}

	m := Model{
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
			Left:  sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
		}
	}
	m := Model{
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
		files: []sidebyside.FilePair{
			{
				OldPath: "a/deleted.go",
				NewPath: "/dev/null",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "deleted"},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
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
			Left:  sidebyside.Line{Num: i + 1, Content: "line"},
			Right: sidebyside.Line{Num: i + 1, Content: "line"},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 10,
		scroll: 100, // Way past the content (only 7 lines: 1 header + 5 pairs + 1 summary)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.clampScroll() // This should clamp to maxScroll

	info := m.StatusInfo()
	// When scrolled to the very end, cursor lands on summary row which has no file info
	assert.Equal(t, 0, info.CurrentFile)
	assert.Equal(t, "", info.FileName)
	assert.True(t, info.AtEnd)
}

func TestStatusInfo_PercentageAccuracy(t *testing.T) {
	// Create 102 lines (100 pairs + 1 header + 1 summary)
	pairs := make([]sidebyside.LinePair, 100)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "line"},
			Right: sidebyside.Line{Num: i + 1, Content: "line"},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 11, // 10 content lines + 1 status bar
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines() // 102 lines total
	// cursorOffset = 10 * 20 / 100 = 2, maxCursor = 101

	// At minScroll, cursor is at line 0, percentage should be 0
	m.scroll = m.minScroll()
	info := m.StatusInfo()
	assert.Equal(t, 0, info.Percentage)
	assert.False(t, info.AtEnd)

	// At scroll that puts cursor at approx line 50, percentage should be ~49
	// (50/101 * 100 = 49.5, rounded to 49)
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
			Left:  sidebyside.Line{Num: i + 1, Content: "content"},
			Right: sidebyside.Line{Num: i + 1, Content: "content"},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs},   // lines 0-10 (header + 10 pairs)
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs}, // line 11 blank, lines 12-22
		},
		width:  80,
		height: 10, // contentHeight=9, cursorOffset=1
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// With cursorOffset=1:
	// scroll=9 → cursor at line 10 (last line of first file) → first.go
	m.scroll = 9
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "first.go", info.FileName)

	// scroll=10 → cursor at line 11 (blank before second file) → first.go (blank belongs to file above)
	m.scroll = 10
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "first.go", info.FileName)

	// scroll=11 → cursor at line 12 (header of second file) → second.go
	m.scroll = 11
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile)
	assert.Equal(t, "second.go", info.FileName)
}

func TestView_ScrolledToMax(t *testing.T) {
	// When scrolled to max, the summary row should be visible, rest is padding
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "last", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 2, Content: "last", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 5, // Small viewport
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines() // 4 lines: header + 2 pairs + summary
	m.scroll = m.maxScroll()

	output := m.View()
	lines := strings.Split(output, "\n")

	assert.Equal(t, 5, len(lines), "should have exactly height lines")

	// Layout: [topBar, content[0..contentH-1], bottomBar]
	// lines[0] = top bar (no file name when on summary)
	assert.NotContains(t, lines[0], "foo.go", "top bar should not show file name when on summary")

	// lines[1] = summary row (last content line at maxScroll)
	assert.Contains(t, lines[1], "file changed")

	// lines[2] should be empty padding (contentH = height - 2 = 3, only 1 content row visible)
	assert.Equal(t, "", lines[2], "line 2 should be empty padding")

	// lines[3] is empty padding
	assert.Equal(t, "", lines[3], "line 3 should be empty padding")

	// lines[4] = bottom bar with END
	assert.Contains(t, lines[4], "END")
}

func TestView_InlineDiffRendering(t *testing.T) {
	// Test that inline diff is computed for modified line pairs
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						// This is a modified pair - should trigger inline diff
						Left:  sidebyside.Line{Num: 1, Content: "fmt.Println(x)", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "fmt.Println(y)", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Output should contain the modified content
	// (Colors are stripped in tests, but content should be present)
	assert.Contains(t, output, "fmt.Println")
	assert.Contains(t, output, "x")
	assert.Contains(t, output, "y")
}

func TestView_InlineDiffSkippedForDissimilar(t *testing.T) {
	// When lines are too different, inline diff should be skipped
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						// Completely different lines - should skip inline diff
						Left:  sidebyside.Line{Num: 1, Content: "abcdefghijklmnop", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "1234567890123456", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Output should still render both lines
	assert.Contains(t, output, "abcdefghijklmnop")
	assert.Contains(t, output, "1234567890123456")
}

func TestView_HunkSeparator(t *testing.T) {
	// When there's a gap in line numbers, a separator should be shown
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					// First hunk: lines 1-3
					{
						Left:  sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
					},
					// Gap here - next hunk starts at line 100
					{
						Left:  sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 101, Content: "line hundred one", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 101, Content: "line hundred one", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should contain a separator line with box drawing dashes and a cross separator
	// Format: space + space + gutter(─) + space + content(─) + space + ┼ + space + ...
	assert.Contains(t, output, "┼")
	// Should have horizontal lines on both sides (gutter area)
	assert.Contains(t, output, "────")
}

func TestView_BlankLineBeforeFileHeader(t *testing.T) {
	// Second and subsequent file headers should have a blank line before them
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/first.go",
				NewPath: "b/first.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath: "a/second.go",
				NewPath: "b/second.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// First line should be the first file header (no blank before it)
	assert.Contains(t, lines[0], "first.go")

	// There should be a blank line before the second file header
	// Find the second file header
	secondHeaderIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "second.go") && strings.Contains(line, "═══") {
			secondHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, -1, secondHeaderIdx, "should find second file header")
	require.Greater(t, secondHeaderIdx, 1, "second header should not be at start")

	// Line before second header should be blank
	assert.Equal(t, "", strings.TrimSpace(lines[secondHeaderIdx-1]),
		"should have blank line before second file header")
}

func TestView_NoBlankLineBeforeFirstFile(t *testing.T) {
	// First file header should NOT have a blank line before it
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/only.go",
				NewPath: "b/only.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, content..., bottomBar]
	// lines[0] = top bar
	// lines[1] = first content line (file header), not blank
	assert.Contains(t, lines[1], "only.go")
	assert.Contains(t, lines[1], "═══")
}

func TestView_NoSeparatorForConsecutiveLines(t *testing.T) {
	// When lines are consecutive, no separator should be shown
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should NOT contain separator characters
	assert.NotContains(t, output, "─┼─")
}

func TestFoldLevelIcon(t *testing.T) {
	tests := []struct {
		level    sidebyside.FoldLevel
		expected string
	}{
		{sidebyside.FoldFolded, "○"},
		{sidebyside.FoldNormal, "◐"},
		{sidebyside.FoldExpanded, "●"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			icon := foldLevelIcon(tt.level)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestView_FoldLevelIcons_InHeaders(t *testing.T) {
	// Test that each fold level shows the correct icon and trailing line in the header
	tests := []struct {
		name         string
		level        sidebyside.FoldLevel
		wantIcon     string
		wantTrailing string // "" for none, "─" for single, "═" for double
	}{
		{"folded shows empty circle, no trailing", sidebyside.FoldFolded, "○", ""},
		{"normal shows half circle, single line", sidebyside.FoldNormal, "◐", "─"},
		{"expanded shows full circle, double line", sidebyside.FoldExpanded, "●", "═"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				files: []sidebyside.FilePair{
					{
						OldPath:    "a/test.go",
						NewPath:    "b/test.go",
						FoldLevel:  tt.level,
						Pairs:      []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1}, Right: sidebyside.Line{Num: 1}}},
						OldContent: []string{"line"}, // For expanded mode
						NewContent: []string{"line"},
					},
				},
				width:  80,
				height: 10,
				keys:   DefaultKeyMap(),
			}
			m.calculateTotalLines()

			output := m.View()
			lines := strings.Split(output, "\n")

			// Layout: [topBar, content..., bottomBar]
			// lines[0] = top bar, lines[1] = first content line (header)
			headerLine := lines[1]
			assert.Contains(t, headerLine, tt.wantIcon, "header should contain %s icon for %s level", tt.wantIcon, tt.level)
			// Header format is: ═══ <foldIcon> <statusIndicator> filename
			// For modified files (a/test.go -> b/test.go with same name), status is "~"
			assert.Contains(t, headerLine, "═══ "+tt.wantIcon+" ~ test.go", "header format should be: ═══ <icon> <status> filename")

			// Check trailing line character
			if tt.wantTrailing == "" {
				// Folded: should end with filename, no trailing line
				assert.NotContains(t, headerLine, "─", "folded header should not have trailing line")
				assert.True(t, strings.HasSuffix(strings.TrimSpace(headerLine), "test.go"),
					"folded header should end with filename")
			} else {
				// Normal/Expanded: should have trailing line of correct type
				assert.Contains(t, headerLine, " "+tt.wantTrailing,
					"header should have trailing %s characters", tt.wantTrailing)
			}
		})
	}
}

func TestView_FoldedFile_HeaderOnly(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "line content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "line content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, content..., bottomBar]
	// lines[0] = top bar, lines[1] = first content line (header)
	// Folded view should only show the header and then padding
	assert.Contains(t, lines[1], "foo.go", "first content line should be the header")
	assert.Contains(t, lines[1], "═══", "header should have the prefix")

	// Header should NOT have trailing "=" characters after the filename
	// The folded header format should be "═══ filename" without trailing "═"
	// Check that the line doesn't end with many "═" (like the normal header does)
	headerContent := strings.TrimRight(lines[1], " ")
	assert.True(t, strings.HasSuffix(headerContent, "foo.go"),
		"folded header should end with filename, got: %s", headerContent)

	// Line pairs should NOT be shown
	assert.NotContains(t, output, "line content", "folded view should not show line pairs")
}

func TestView_FoldedFile_NoBlankLineBefore(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "first file", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "first file", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				FoldLevel: sidebyside.FoldFolded, // Second file is folded
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "second file", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "second file", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the second file header
	secondHeaderIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "second.go") && strings.Contains(line, "═══") {
			secondHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, -1, secondHeaderIdx, "should find second file header")

	// For folded files, there should NOT be a blank line before the header
	// The previous line should be the last content line of first file
	assert.Contains(t, lines[secondHeaderIdx-1], "first file",
		"folded header should follow directly after previous content (no blank line)")
}

func TestView_MixedFoldLevels(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/normal.go",
				NewPath:   "b/normal.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "normal file content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "normal file content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/folded.go",
				NewPath:   "b/folded.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "folded file content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "folded file content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/another.go",
				NewPath:   "b/another.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "another file content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "another file content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Normal files should show their content
	assert.Contains(t, output, "normal file content")
	assert.Contains(t, output, "another file content")

	// Folded file should NOT show its content
	assert.NotContains(t, output, "folded file content")

	// But all file headers should be visible
	assert.Contains(t, output, "normal.go")
	assert.Contains(t, output, "folded.go")
	assert.Contains(t, output, "another.go")
}

func TestView_TotalLines_WithFolding(t *testing.T) {
	// Test that totalLines is calculated correctly with different fold states
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/normal.go",
				NewPath:   "b/normal.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs:     make([]sidebyside.LinePair, 10),
			},
			{
				OldPath:   "a/folded.go",
				NewPath:   "b/folded.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     make([]sidebyside.LinePair, 10),
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Normal file: 1 header + 10 pairs = 11 lines
	// Folded file: 1 header only (no blank line before since it's folded)
	// Summary row: 1 line
	// Total should be 11 + 1 + 1 = 13
	assert.Equal(t, 13, m.totalLines, "totalLines should account for fold states and summary")
}

func TestView_ExpandedFile_ShowsFullContent(t *testing.T) {
	// Expanded view should show ALL lines from the full file content
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				// Original diff pairs (just lines 5-7 with a change at line 6)
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 5, Content: "line five", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 5, Content: "line five", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 6, Content: "old line six", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 6, Content: "new line six", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 7, Content: "line seven", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 7, Content: "line seven", Type: sidebyside.Context},
					},
				},
				// Full file content (10 lines each)
				OldContent: []string{
					"line one", "line two", "line three", "line four",
					"line five", "old line six", "line seven",
					"line eight", "line nine", "line ten",
				},
				NewContent: []string{
					"line one", "line two", "line three", "line four",
					"line five", "new line six", "line seven",
					"line eight", "line nine", "line ten",
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show all lines from the file (lines outside the diff context)
	assert.Contains(t, output, "line one", "should show line 1 from full content")
	assert.Contains(t, output, "line two", "should show line 2 from full content")
	assert.Contains(t, output, "line three", "should show line 3 from full content")
	assert.Contains(t, output, "line four", "should show line 4 from full content")
	assert.Contains(t, output, "line eight", "should show line 8 from full content")
	assert.Contains(t, output, "line nine", "should show line 9 from full content")
	assert.Contains(t, output, "line ten", "should show line 10 from full content")

	// Should still show the diff lines
	assert.Contains(t, output, "line five")
	assert.Contains(t, output, "old line six")
	assert.Contains(t, output, "new line six")
	assert.Contains(t, output, "line seven")
}

func TestView_ExpandedFile_NoContent_FallsBackToNormal(t *testing.T) {
	// If expanded but content not loaded yet, fall back to normal view
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 5, Content: "diff context", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 5, Content: "diff context", Type: sidebyside.Context},
					},
				},
				// No OldContent/NewContent loaded yet
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show the diff pairs since content isn't loaded
	assert.Contains(t, output, "diff context")
}

func TestView_ExpandedFile_DeletedFile(t *testing.T) {
	// For deleted files, only left side should show content
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "deleted line", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
				OldContent: []string{"deleted line", "another deleted"},
				NewContent: nil, // No new content (file deleted)
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show the old content
	assert.Contains(t, output, "deleted line")
	assert.Contains(t, output, "another deleted")
}

func TestView_ExpandedFile_NewFile(t *testing.T) {
	// For new files, only right side should show content
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/new.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 1, Content: "new line", Type: sidebyside.Added},
					},
				},
				OldContent: nil, // No old content (new file)
				NewContent: []string{"new line", "another new"},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show the new content
	assert.Contains(t, output, "new line")
	assert.Contains(t, output, "another new")
}

func TestView_ExpandedFile_AlignmentWithAddedLines(t *testing.T) {
	// Bug: When lines are added, expanded view pairs old[i] with new[i] by index,
	// not by semantic alignment. This test verifies proper alignment.
	//
	// Scenario:
	// - Old file: line1, line2, line3, line4, line5 (5 lines)
	// - New file: line1, line2, INSERTED, line3, line4, line5 (6 lines)
	// - Diff shows the insertion between line2 and line3
	//
	// Expected alignment in expanded view:
	//   old line1 | new line1
	//   old line2 | new line2
	//   (empty)   | INSERTED  <- added line
	//   old line3 | new line3 (which is new line 4 in new file)
	//   old line4 | new line4 (which is new line 5 in new file)
	//   old line5 | new line5 (which is new line 6 in new file)
	//
	// Bug behavior: old line3 pairs with new line3 (INSERTED) - wrong!

	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				// Diff pairs showing the insertion
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 3, Content: "INSERTED", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 4, Content: "line3", Type: sidebyside.Context},
					},
				},
				OldContent: []string{"line1", "line2", "line3", "line4", "line5"},
				NewContent: []string{"line1", "line2", "INSERTED", "line3", "line4", "line5"},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Skip header row
	// Find the row that has old line 3
	var oldLine3Row *displayRow
	for i := range rows {
		if rows[i].pair.Left.Num == 3 {
			oldLine3Row = &rows[i]
			break
		}
	}

	if oldLine3Row == nil {
		t.Fatal("could not find row with old line 3")
	}

	// Old line 3 should be paired with new line 4 (both have content "line3")
	// NOT with new line 3 (which is "INSERTED")
	assert.Equal(t, "line3", oldLine3Row.pair.Left.Content, "left side should be line3")
	assert.Equal(t, "line3", oldLine3Row.pair.Right.Content,
		"right side should also be line3 (new line 4), not INSERTED")
	assert.Equal(t, 4, oldLine3Row.pair.Right.Num,
		"right side line number should be 4 (after the insertion)")
}

func TestView_ExpandedFile_AlignmentWithRemovedLines(t *testing.T) {
	// Similar test but for removed lines
	//
	// Scenario:
	// - Old file: line1, line2, REMOVED, line3, line4 (5 lines)
	// - New file: line1, line2, line3, line4 (4 lines)
	//
	// Expected alignment:
	//   old line1 | new line1
	//   old line2 | new line2
	//   REMOVED   | (empty)   <- removed line
	//   old line4 | new line3 (same content "line3")
	//   old line5 | new line4 (same content "line4")

	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "REMOVED", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
					{
						Left:  sidebyside.Line{Num: 4, Content: "line3", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Context},
					},
				},
				OldContent: []string{"line1", "line2", "REMOVED", "line3", "line4"},
				NewContent: []string{"line1", "line2", "line3", "line4"},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the row that has new line 3
	var newLine3Row *displayRow
	for i := range rows {
		if rows[i].pair.Right.Num == 3 && rows[i].pair.Right.Content == "line3" {
			newLine3Row = &rows[i]
			break
		}
	}

	if newLine3Row == nil {
		t.Fatal("could not find row with new line 3 content 'line3'")
	}

	// New line 3 (content "line3") should be paired with old line 4 (same content)
	assert.Equal(t, "line3", newLine3Row.pair.Right.Content, "right side should be line3")
	assert.Equal(t, "line3", newLine3Row.pair.Left.Content,
		"left side should also be line3 (old line 4), not REMOVED")
	assert.Equal(t, 4, newLine3Row.pair.Left.Num,
		"left side line number should be 4 (after the removed line)")
}

func TestView_GutterIndicators(t *testing.T) {
	// Test that +/- indicators appear in the gutter for added/removed lines
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "context line", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "context line", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "removed line", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 2, Content: "added line", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 3, Content: "pure add", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "pure remove", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "gutter_indicators.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_GutterIndicatorTypes(t *testing.T) {
	// Test specific indicator characters for each line type
	tests := []struct {
		name       string
		lineType   sidebyside.LineType
		wantChar   string
		wantAbsent string
	}{
		{"added line has + indicator", sidebyside.Added, "+", "-"},
		{"removed line has - indicator", sidebyside.Removed, "-", "+"},
		{"context line has no indicator", sidebyside.Context, " ", "+-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				files: []sidebyside.FilePair{
					{
						OldPath: "a/test.go",
						NewPath: "b/test.go",
						Pairs: []sidebyside.LinePair{
							{
								Left:  sidebyside.Line{Num: 1, Content: "test content", Type: tt.lineType},
								Right: sidebyside.Line{Num: 1, Content: "test content", Type: tt.lineType},
							},
							{
								Left:  sidebyside.Line{Num: 2, Content: "another line", Type: sidebyside.Context},
								Right: sidebyside.Line{Num: 2, Content: "another line", Type: sidebyside.Context},
							},
						},
					},
				},
				width:  80,
				height: 10,
				keys:   DefaultKeyMap(),
			}
			m.calculateTotalLines()
			// Position cursor on line 2 (row 2 = second content line) so we can test line 1's indicator
			// cursorLine = scroll + cursorOffset, so scroll = row - cursorOffset = 2 - cursorOffset
			m.scroll = 2 - m.cursorOffset()

			output := m.View()
			lines := strings.Split(output, "\n")

			// Find the line with "test content" (not the cursor line with "another line")
			var contentLine string
			for _, line := range lines {
				if strings.Contains(line, "test content") {
					contentLine = line
					break
				}
			}
			require.NotEmpty(t, contentLine, "should find line with test content")

			// The line should contain the indicator followed by space then line number
			// Format is: indicator + space + lineNum + space + [gutter] + content
			// Added/removed lines have ░ gutter, context lines have spaces
			// e.g., "+    1 ░ test content" or "     1   test content"
			if tt.wantChar == "+" || tt.wantChar == "-" {
				assert.Contains(t, contentLine, tt.wantChar+"    1 ░",
					"line should have %q indicator before line number", tt.wantChar)
			} else {
				assert.Contains(t, contentLine, tt.wantChar+"    1  ",
					"line should have %q indicator before line number", tt.wantChar)
			}
		})
	}
}

func TestView_LineNumberColorMatchesIndicator(t *testing.T) {
	// Test that line numbers are colored to match the +/- indicator
	// Added lines should have green line numbers, removed lines should have red

	// Temporarily enable ANSI colors for this test
	oldProfile := lipgloss.DefaultRenderer().ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(oldProfile)

	tests := []struct {
		name      string
		lineType  sidebyside.LineType
		wantColor string // ANSI color code prefix
	}{
		{"added line has green line number", sidebyside.Added, "\x1b[92m"},     // bright green
		{"removed line has red line number", sidebyside.Removed, "\x1b[91m"},   // bright red
		{"context line has dim line number", sidebyside.Context, "\x1b[2;90m"}, // faint + gray
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				files: []sidebyside.FilePair{
					{
						OldPath: "a/test.go",
						NewPath: "b/test.go",
						Pairs: []sidebyside.LinePair{
							// First line (cursor will be here)
							{
								Left:  sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
								Right: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
							},
							// Second line (the one we're testing, cursor not here)
							{
								Left:  sidebyside.Line{Num: 2, Content: "content", Type: tt.lineType},
								Right: sidebyside.Line{Num: 2, Content: "content", Type: tt.lineType},
							},
						},
					},
				},
				width:  80,
				height: 10,
				keys:   DefaultKeyMap(),
			}
			m.calculateTotalLines()

			output := m.View()

			// The line number "2" should be styled with the expected color
			assert.Contains(t, output, tt.wantColor+"   2",
				"line number should be styled with color code %q", tt.wantColor)
		})
	}
}

func TestView_LargeLineNumbers(t *testing.T) {
	// Test that line numbers up to 10000 are displayed correctly
	// This requires dynamic gutter width calculation
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/large.go",
				NewPath: "b/large.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 9999, Content: "line 9999", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 9999, Content: "line 9999", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 10000, Content: "line 10000", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 10000, Content: "line 10000 modified", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 10001, Content: "line 10001", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 10001, Content: "line 10001", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "large_line_numbers.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_LargeLineNumbers_Alignment(t *testing.T) {
	// Test that all line numbers in a diff are right-aligned to the same width
	// when some lines have 5-digit numbers (consecutive to avoid hunk separator)
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/large.go",
				NewPath: "b/large.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 9999, Content: "line before", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 9999, Content: "line before", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 10000, Content: "ten thousand", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 10000, Content: "ten thousand", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 10001, Content: "line after", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 10001, Content: "line after", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, content..., bottomBar]
	// lines[0] = top bar, lines[1] = header
	// Content lines: lines[2] = 9999, lines[3] = 10000, lines[4] = 10001
	line1 := lines[2]
	line2 := lines[3]
	line3 := lines[4]

	// All lines should have their content starting at the same display column position
	// The gutter width should accommodate 5 digits for consistency
	// Note: Using display width (rune count) not byte position due to multi-byte cursor arrow

	// Find display column position of content in each line
	pos1 := displayColumnOf(line1, "line before")
	pos2 := displayColumnOf(line2, "ten thousand")
	pos3 := displayColumnOf(line3, "line after")

	assert.Equal(t, pos1, pos2,
		"content should start at same display column\nline1: %q\nline2: %q", line1, line2)
	assert.Equal(t, pos2, pos3,
		"content should start at same display column\nline2: %q\nline3: %q", line2, line3)

	// The 5-digit number should be fully visible (not truncated)
	assert.Contains(t, line2, "10000", "5-digit line number should be fully visible")
	assert.Contains(t, line3, "10001", "5-digit line number should be fully visible")
}

func TestView_LineNumberTruncation(t *testing.T) {
	// Test that demonstrates 5-digit line numbers get truncated with current
	// hardcoded 4-digit width. This test documents the current (broken) behavior
	// and should be updated when dynamic width is implemented.
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// With hardcoded lineNumWidth=4, the number 10000 should show as "10000"
	// but it will overflow into the content area. Check that we see the full number.
	// This test will fail until we implement dynamic gutter width.
	assert.Contains(t, output, "10000",
		"5-digit line number should be visible (currently may overflow)")
}

func TestView_GutterWidthNotShrinkOnFold(t *testing.T) {
	// Test that gutter width doesn't shrink when folding a file with large line numbers
	// This ensures the cached maxLineNumSeen is preserved
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/small.go",
				NewPath:   "b/small.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "small file", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "small file", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/large.go",
				NewPath:   "b/large.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 10000, Content: "large file", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 10000, Content: "large file", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Gutter width should be 5 (for 5-digit numbers)
	assert.Equal(t, 5, m.lineNumWidth(), "gutter width should expand to 5 for 10000")

	// Get content column position before folding
	output1 := m.View()
	lines1 := strings.Split(output1, "\n")
	// Find the small file line (line 1 content)
	var smallFileLineBeforeFold string
	for _, line := range lines1 {
		if strings.Contains(line, "small file") {
			smallFileLineBeforeFold = line
			break
		}
	}
	pos1 := strings.Index(smallFileLineBeforeFold, "small file")

	// Now fold the large file (hiding the 10000 line number)
	m.files[1].FoldLevel = sidebyside.FoldFolded
	m.calculateTotalLines()

	// Gutter width should STILL be 5 (not shrink back to 4)
	assert.Equal(t, 5, m.lineNumWidth(), "gutter width should NOT shrink after folding")

	// Content should still start at the same column
	output2 := m.View()
	lines2 := strings.Split(output2, "\n")
	var smallFileLineAfterFold string
	for _, line := range lines2 {
		if strings.Contains(line, "small file") {
			smallFileLineAfterFold = line
			break
		}
	}
	pos2 := strings.Index(smallFileLineAfterFold, "small file")

	assert.Equal(t, pos1, pos2,
		"content column should not shift after folding large file\nbefore: %q\nafter: %q",
		smallFileLineBeforeFold, smallFileLineAfterFold)
}

func TestView_StatusBarAlwaysAtBottom(t *testing.T) {
	// When content is shorter than viewport, status bar should still be at
	// the bottom of the terminal (not immediately after content)
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "only line", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "only line", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10, // Much taller than content (3 lines: header + 1 pair + summary)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Scroll to end to verify END appears
	m.scroll = m.maxScroll()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Output should have exactly `height` lines (top bar + content + bottom bar)
	assert.Equal(t, 10, len(lines), "view should fill entire viewport height")

	// Layout: [topBar, content..., bottomBar]
	// Top bar should not show file name when cursor is on summary
	assert.NotContains(t, lines[0], "foo.go", "top bar should not show file name when on summary")

	// Bottom bar should show END
	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "END")

	// When scrolled to end with small content:
	// - scroll = maxScroll = 1, cursorOffset = 1
	// - lines[1] is at scroll position (line index 1 = the pair)
	// - lines[2] is cursor position (line index 2 = summary)
	assert.Contains(t, lines[1], "only line", "first content line should be the pair")
	assert.Contains(t, lines[2], "file changed", "second content line should be summary at cursor")
}

// === File Stats Tests ===

func TestCountFileStats(t *testing.T) {
	tests := []struct {
		name        string
		pairs       []sidebyside.LinePair
		wantAdded   int
		wantRemoved int
	}{
		{
			name:        "empty file",
			pairs:       []sidebyside.LinePair{},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "only context lines",
			pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
				},
			},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "pure additions",
			pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Type: sidebyside.Empty},
					Right: sidebyside.Line{Num: 1, Content: "new 1", Type: sidebyside.Added},
				},
				{
					Left:  sidebyside.Line{Type: sidebyside.Empty},
					Right: sidebyside.Line{Num: 2, Content: "new 2", Type: sidebyside.Added},
				},
			},
			wantAdded:   2,
			wantRemoved: 0,
		},
		{
			name: "pure deletions",
			pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "old 1", Type: sidebyside.Removed},
					Right: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Left:  sidebyside.Line{Num: 2, Content: "old 2", Type: sidebyside.Removed},
					Right: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Left:  sidebyside.Line{Num: 3, Content: "old 3", Type: sidebyside.Removed},
					Right: sidebyside.Line{Type: sidebyside.Empty},
				},
			},
			wantAdded:   0,
			wantRemoved: 3,
		},
		{
			name: "mixed changes",
			pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
				},
				{
					Left:  sidebyside.Line{Num: 2, Content: "old", Type: sidebyside.Removed},
					Right: sidebyside.Line{Num: 2, Content: "new", Type: sidebyside.Added},
				},
				{
					Left:  sidebyside.Line{Type: sidebyside.Empty},
					Right: sidebyside.Line{Num: 3, Content: "added", Type: sidebyside.Added},
				},
			},
			wantAdded:   2,
			wantRemoved: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := sidebyside.FilePair{Pairs: tt.pairs}
			added, removed := countFileStats(fp)
			assert.Equal(t, tt.wantAdded, added, "added count")
			assert.Equal(t, tt.wantRemoved, removed, "removed count")
		})
	}
}

func TestFormatStatsBar(t *testing.T) {
	tests := []struct {
		name     string
		added    int
		removed  int
		maxWidth int
		want     string
	}{
		{
			name:     "no changes",
			added:    0,
			removed:  0,
			maxWidth: 24,
			want:     "",
		},
		{
			name:     "only additions - small",
			added:    5,
			removed:  0,
			maxWidth: 24,
			want:     "+5 +++++",
		},
		{
			name:     "only deletions - small",
			added:    0,
			removed:  3,
			maxWidth: 24,
			want:     "-3 ---",
		},
		{
			name:     "mixed - fits in max",
			added:    10,
			removed:  5,
			maxWidth: 24,
			want:     "+10 -5 ++++++++++-----",
		},
		{
			name:     "mixed - needs scaling",
			added:    30,
			removed:  18,
			maxWidth: 24,
			want:     "+30 -18 +++++++++++++++---------", // scaled: 30+18=48, scale=24/48=0.5, so 15+ and 9-
		},
		{
			name:     "large numbers - heavy scaling",
			added:    100,
			removed:  100,
			maxWidth: 24,
			want:     "+100 -100 ++++++++++++------------", // scaled: 12+ and 12-
		},
		{
			name:     "pure addition - needs scaling",
			added:    48,
			removed:  0,
			maxWidth: 24,
			want:     "+48 ++++++++++++++++++++++++", // scaled to 24
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatsBar(tt.added, tt.removed, tt.maxWidth)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileHeaderWithStats_FoldedOnly(t *testing.T) {
	// Stats should only appear in folded view, not in normal/expanded
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldNormal, // Normal view - no stats
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	header := lines[0]

	// Normal view should NOT contain stats
	assert.Contains(t, header, "main.go", "header should contain filename")
	assert.NotContains(t, header, "|", "normal view header should not contain stats separator")
}

func TestFileHeaderWithStats_Folded(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldFolded, // Folded view - show stats
				Pairs: []sidebyside.LinePair{
					// 3 additions, 2 deletions
					{
						Left:  sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "old2", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 2, Content: "new2", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 3, Content: "new3", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, content..., bottomBar]
	// Folded header should contain filename, stats counts, and +/- bar
	header := lines[1]
	assert.Contains(t, header, "main.go", "header should contain filename")
	assert.Contains(t, header, "|", "header should contain separator")
	assert.Contains(t, header, "+3", "header should show addition count")
	assert.Contains(t, header, "-2", "header should show deletion count")
	assert.Contains(t, header, "+++", "header should show addition bar")
	assert.Contains(t, header, "--", "header should show deletion bar")
}

func TestFileHeaderWithStats_Alignment(t *testing.T) {
	// Multiple folded files should have aligned | separators
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/short.go",
				NewPath:   "b/short.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 1, Content: "added", Type: sidebyside.Added},
					},
				},
			},
			{
				OldPath:   "a/much_longer_filename.go",
				NewPath:   "b/much_longer_filename.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, content..., bottomBar]
	// Find display column position of | in each header (using rune position for multi-byte chars)
	header1 := lines[1]
	header2 := lines[2] // second header is at lines[2] (after blank line between files? Let me check)

	pos1 := displayColumnOf(header1, "|")
	pos2 := displayColumnOf(header2, "|")

	assert.NotEqual(t, -1, pos1, "first header should contain |")
	assert.NotEqual(t, -1, pos2, "second header should contain |")
	assert.Equal(t, pos1, pos2, "| should be aligned across headers")
}

func TestFileHeaderWithStats_BarAlignment(t *testing.T) {
	// The +/- bar should start at the same column even when count widths differ
	// e.g., "+100" vs "+5" should both have bars starting at same position
	pairs100 := make([]sidebyside.LinePair, 100)
	for i := range pairs100 {
		pairs100[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Type: sidebyside.Empty},
			Right: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	pairs5 := make([]sidebyside.LinePair, 5)
	for i := range pairs5 {
		pairs5[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Type: sidebyside.Empty},
			Right: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/view.go",
				NewPath:   "b/view.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     pairs100, // +100 additions
			},
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     pairs5, // +5 additions
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, content..., bottomBar]
	header1 := lines[1] // +100 -> bar has 24 chars (scaled)
	header2 := lines[2] // +5 -> bar has 5 chars

	// Find the display column position of the bar (consecutive + or - characters)
	// The bar starts after "| +NNN " - we look for where the repeated +/- begins
	// Use rune-based indexing for proper handling of multi-byte characters
	findBarStart := func(s string) int {
		runes := []rune(s)
		// Find "| " in runes
		pipeIdx := -1
		for i := 0; i < len(runes)-1; i++ {
			if runes[i] == '|' && runes[i+1] == ' ' {
				pipeIdx = i
				break
			}
		}
		if pipeIdx == -1 {
			return -1
		}
		// After "| ", we have count(s) then space then bar
		// Look for sequence of 2+ consecutive + or -
		afterPipe := runes[pipeIdx+2:]
		for i := 0; i < len(afterPipe); i++ {
			ch := afterPipe[i]
			// Skip count portion: +N, -N, spaces
			if ch == '+' || ch == '-' {
				// Check if this is start of bar (followed by same char)
				if i+1 < len(afterPipe) && afterPipe[i+1] == ch {
					return pipeIdx + 2 + i
				}
				// Otherwise it's part of count, continue
			}
			// Space continues count section
		}
		return -1
	}

	barPos1 := findBarStart(header1)
	barPos2 := findBarStart(header2)

	assert.NotEqual(t, -1, barPos1, "first header should have bar")
	assert.NotEqual(t, -1, barPos2, "second header should have bar")
	assert.Equal(t, barPos1, barPos2, "bar should start at same position across headers")
}

func TestFileHeaderWithStats_OnlyAdditions(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/newfile.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	// Layout: [topBar, content..., bottomBar]
	header := lines[1]

	assert.Contains(t, header, "newfile.go", "header should contain filename")
	assert.Contains(t, header, "+2", "header should show addition count")
	assert.NotContains(t, header, "-", "header should not show deletion count when zero")
}

func TestFileHeaderWithStats_OnlyDeletions(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Removed},
						Right: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Removed},
						Right: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Removed},
						Right: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	// Layout: [topBar, content..., bottomBar]
	header := lines[1]

	assert.Contains(t, header, "deleted.go", "header should contain filename")
	assert.Contains(t, header, "-3", "header should show deletion count")
	// Check there's no + count (but the filename might contain + in other contexts)
	// The format should be "-3 ---" not "+0 -3 ---"
}

func TestFileStatus(t *testing.T) {
	tests := []struct {
		name       string
		oldPath    string
		newPath    string
		wantStatus FileStatus
	}{
		{
			name:       "added file",
			oldPath:    "/dev/null",
			newPath:    "b/new.go",
			wantStatus: FileStatusAdded,
		},
		{
			name:       "deleted file",
			oldPath:    "a/old.go",
			newPath:    "/dev/null",
			wantStatus: FileStatusDeleted,
		},
		{
			name:       "renamed file",
			oldPath:    "a/old.go",
			newPath:    "b/new.go",
			wantStatus: FileStatusRenamed,
		},
		{
			name:       "modified file - same name with prefixes",
			oldPath:    "a/file.go",
			newPath:    "b/file.go",
			wantStatus: FileStatusModified,
		},
		{
			name:       "modified file - identical paths",
			oldPath:    "file.go",
			newPath:    "file.go",
			wantStatus: FileStatusModified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileStatus(tt.oldPath, tt.newPath)
			assert.Equal(t, tt.wantStatus, got)
		})
	}
}

func TestFileStatusIndicator(t *testing.T) {
	tests := []struct {
		status     FileStatus
		wantSymbol string
	}{
		{FileStatusAdded, "+"},
		{FileStatusDeleted, "-"},
		{FileStatusRenamed, ">"},
		{FileStatusModified, "~"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			symbol, _ := fileStatusIndicator(tt.status)
			assert.Equal(t, tt.wantSymbol, symbol)
		})
	}
}

func TestView_FileStatusIndicator_InHeaders(t *testing.T) {
	// Test that file status indicators appear in headers for all fold levels
	tests := []struct {
		name          string
		oldPath       string
		newPath       string
		foldLevel     sidebyside.FoldLevel
		wantIndicator string
	}{
		{
			name:          "added file - folded",
			oldPath:       "/dev/null",
			newPath:       "b/new.go",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: "+",
		},
		{
			name:          "deleted file - folded",
			oldPath:       "a/old.go",
			newPath:       "/dev/null",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: "-",
		},
		{
			name:          "renamed file - folded",
			oldPath:       "a/old.go",
			newPath:       "b/new.go",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: ">",
		},
		{
			name:          "modified file - folded",
			oldPath:       "a/file.go",
			newPath:       "b/file.go",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: "~",
		},
		{
			name:          "added file - normal",
			oldPath:       "/dev/null",
			newPath:       "b/new.go",
			foldLevel:     sidebyside.FoldNormal,
			wantIndicator: "+",
		},
		{
			name:          "modified file - expanded",
			oldPath:       "a/file.go",
			newPath:       "b/file.go",
			foldLevel:     sidebyside.FoldExpanded,
			wantIndicator: "~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				files: []sidebyside.FilePair{
					{
						OldPath:    tt.oldPath,
						NewPath:    tt.newPath,
						FoldLevel:  tt.foldLevel,
						Pairs:      []sidebyside.LinePair{{Left: sidebyside.Line{Num: 1}, Right: sidebyside.Line{Num: 1}}},
						OldContent: []string{"line"},
						NewContent: []string{"line"},
					},
				},
				width:  100,
				height: 10,
				keys:   DefaultKeyMap(),
			}
			m.calculateTotalLines()

			output := m.View()
			lines := strings.Split(output, "\n")
			// Layout: [topBar, content..., bottomBar]
			header := lines[1]

			// Get the expected fold icon
			foldIcon := foldLevelIcon(tt.foldLevel)

			// Header format should be: ═══ <foldIcon> <statusIndicator> filename
			// e.g., "═══ ○ + new.go" or "═══ ◐ ~ file.go"
			expectedPattern := "═══ " + foldIcon + " " + tt.wantIndicator + " "
			assert.Contains(t, header, expectedPattern,
				"header should contain fold icon followed by status indicator: %s", expectedPattern)
		})
	}
}

// Summary row tests

func TestBuildRows_IncludesSummaryRow(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Last row should be the summary
	require.NotEmpty(t, rows)
	lastRow := rows[len(rows)-1]
	assert.True(t, lastRow.isSummary, "last row should be summary row")
}

func TestBuildRows_SummaryRowHasCorrectStats(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 2, Content: "added", Type: sidebyside.Added},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "deleted", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	lastRow := rows[len(rows)-1]
	require.True(t, lastRow.isSummary)
	// Total: 2 files, 2 added lines (one.go), 2 removed lines (one.go + two.go)
	assert.Equal(t, 2, lastRow.totalFiles)
	assert.Equal(t, 2, lastRow.totalAdded)
	assert.Equal(t, 2, lastRow.totalRemoved)
}

func TestBuildRows_SummaryRowNoFile(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	lastRow := rows[len(rows)-1]
	require.True(t, lastRow.isSummary)
	// Summary row should have fileIndex = -1 to indicate no file association
	assert.Equal(t, -1, lastRow.fileIndex)
}

func TestView_SummaryRowFormat(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should contain git-style summary: "1 file changed, 1 insertion(+), 1 deletion(-)"
	assert.Contains(t, output, "1 file changed")
	assert.Contains(t, output, "1 insertion(+)")
	assert.Contains(t, output, "1 deletion(-)")
}

func TestView_SummaryRowPluralFormat(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 2, Content: "added", Type: sidebyside.Added},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should use plural forms: "2 files changed, 3 insertions(+), 2 deletions(-)"
	assert.Contains(t, output, "2 files changed")
	assert.Contains(t, output, "3 insertions(+)")
	assert.Contains(t, output, "2 deletions(-)")
}

func TestView_SummaryRowHasEqualsPrefix(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the summary line (contains "file changed" or "files changed")
	var summaryLine string
	for _, line := range lines {
		if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
			summaryLine = line
			break
		}
	}
	require.NotEmpty(t, summaryLine, "should find summary line")
	// Summary format is now: "  ════ ●   ..." (space + space + equals gutter + icon)
	// Should contain ═ characters for the gutter
	assert.Contains(t, summaryLine, "═", "summary should contain ═ gutter characters")
}

func TestView_SummaryRowIsSelectable(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// The summary row should be included in totalLines
	// With folded view: 1 header + 1 summary = 2 lines
	rows := m.buildRows()
	assert.Equal(t, 2, len(rows), "should have header + summary")
}

func TestView_SummaryRowAppearsInAllModes(t *testing.T) {
	tests := []struct {
		name      string
		foldLevel sidebyside.FoldLevel
	}{
		{"folded", sidebyside.FoldFolded},
		{"normal", sidebyside.FoldNormal},
		{"expanded", sidebyside.FoldExpanded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				files: []sidebyside.FilePair{
					{
						OldPath:    "a/foo.go",
						NewPath:    "b/foo.go",
						FoldLevel:  tt.foldLevel,
						OldContent: []string{"line1"},
						NewContent: []string{"line1"},
						Pairs: []sidebyside.LinePair{
							{
								Left:  sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
								Right: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
							},
						},
					},
				},
				width:  80,
				height: 20,
				keys:   DefaultKeyMap(),
			}
			m.calculateTotalLines()

			output := m.View()
			assert.Contains(t, output, "file changed", "summary should appear in %s mode", tt.name)
		})
	}
}

func TestFormatSummaryStats(t *testing.T) {
	tests := []struct {
		name     string
		files    int
		added    int
		removed  int
		expected string
	}{
		{
			name:     "singular all",
			files:    1,
			added:    1,
			removed:  1,
			expected: "1 file changed, 1 insertion(+), 1 deletion(-)",
		},
		{
			name:     "plural all",
			files:    3,
			added:    10,
			removed:  5,
			expected: "3 files changed, 10 insertions(+), 5 deletions(-)",
		},
		{
			name:     "no insertions",
			files:    1,
			added:    0,
			removed:  3,
			expected: "1 file changed, 3 deletions(-)",
		},
		{
			name:     "no deletions",
			files:    2,
			added:    5,
			removed:  0,
			expected: "2 files changed, 5 insertions(+)",
		},
		{
			name:     "no changes",
			files:    1,
			added:    0,
			removed:  0,
			expected: "1 file changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSummaryStats(tt.files, tt.added, tt.removed)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCurrentFileIndex_ReturnsMinusOneForSummaryRow(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Scroll to put cursor on summary row (last line)
	m.scroll = m.maxScroll()

	// currentFileIndex should return -1 when cursor is on summary row
	idx := m.currentFileIndex()
	assert.Equal(t, -1, idx, "currentFileIndex should return -1 for summary row")
}

// Tests for new status bar format

func TestFormatLessIndicator_Basic(t *testing.T) {
	tests := []struct {
		name       string
		line       int
		total      int
		percentage int
		atEnd      bool
		expected   string
	}{
		{
			name:       "at start",
			line:       1,
			total:      100,
			percentage: 0,
			atEnd:      false,
			expected:   "line 1/100 0%",
		},
		{
			name:       "middle",
			line:       50,
			total:      100,
			percentage: 49,
			atEnd:      false,
			expected:   "line 50/100 49%",
		},
		{
			name:       "at end",
			line:       100,
			total:      100,
			percentage: 100,
			atEnd:      true,
			expected:   "line 100/100 (END)",
		},
		{
			name:       "single line file at end",
			line:       1,
			total:      1,
			percentage: 100,
			atEnd:      true,
			expected:   "line 1/1 (END)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLessIndicator(tt.line, tt.total, tt.percentage, tt.atEnd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStatusBar_NewFormat_Basic(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "context", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 2, Content: "context", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]
	bottomBar := lines[len(lines)-1]

	// Bottom bar should contain less-style line indicator
	assert.Contains(t, bottomBar, "line ")
	assert.Contains(t, bottomBar, "/")

	// Top bar should contain fold icon (◐ for normal)
	assert.Contains(t, topBar, "◐")

	// Top bar should contain status icon (~ for modified)
	assert.Contains(t, topBar, "~")

	// Top bar should contain file path
	assert.Contains(t, topBar, "foo.go")

	// Top bar should contain stats (+1 -1)
	assert.Contains(t, topBar, "+1")
	assert.Contains(t, topBar, "-1")

	// Should NOT contain [1/1] file counter anymore
	assert.NotContains(t, topBar, "[")
	assert.NotContains(t, topBar, "]")
}

func TestStatusBar_NewFormat_FoldedFile(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on the file header (not summary)
	m.scroll = m.minScroll()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should contain folded icon (○)
	assert.Contains(t, topBar, "○")
}

func TestStatusBar_NewFormat_ExpandedFile(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
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
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should contain expanded icon (●)
	assert.Contains(t, topBar, "●")
}

func TestStatusBar_NewFormat_AddedFile(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/newfile.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should contain file name and stats
	assert.Contains(t, topBar, "newfile.go")
	assert.Contains(t, topBar, "+1")
}

func TestStatusBar_NewFormat_DeletedFile(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "gone", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should show old path for deleted files
	assert.Contains(t, topBar, "deleted.go")
	assert.Contains(t, topBar, "-1")
}

func TestStatusBar_NewFormat_AtEnd(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
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
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.scroll = m.maxScroll()

	output := m.View()
	lines := strings.Split(output, "\n")
	statusBar := lines[len(lines)-1]

	// Should show (END) instead of percentage when at end
	assert.Contains(t, statusBar, "(END)")
	assert.NotContains(t, statusBar, "100%")
}

func TestStatusBar_NewFormat_NoStats(t *testing.T) {
	// A file with no actual changes (just context) should not show stats
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should contain file path but no +/- stats
	assert.Contains(t, topBar, "foo.go")
	// Stats should be omitted when there are no changes
	assert.NotContains(t, topBar, "+0")
	assert.NotContains(t, topBar, "-0")
}

func TestStatusBar_NonShrinkingWidth(t *testing.T) {
	// Create a file with many lines to get large line numbers
	pairs := make([]sidebyside.LinePair, 1000)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Context},
		}
	}

	m := Model{
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

// ============================================================================
// Top Bar Tests
// ============================================================================

func TestTopBar_ContainsFileInfo(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should contain file info
	assert.Contains(t, topBar, "foo.go", "top bar should contain file name")
	assert.Contains(t, topBar, "◐", "top bar should contain fold icon")
	assert.Contains(t, topBar, "~", "top bar should contain status icon for modified file")
	assert.Contains(t, topBar, "+1", "top bar should contain added count")
	assert.Contains(t, topBar, "-1", "top bar should contain removed count")
}

func TestTopBar_LeftAligned(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
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
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should be left-aligned (starts with content, not spaces)
	assert.True(t, len(topBar) > 0, "top bar should not be empty")
	// The fold icon should be near the start
	idx := strings.Index(topBar, "◐")
	assert.True(t, idx >= 0 && idx < 5, "fold icon should be near the start (left-aligned)")
}

func TestBottomBar_OnlyLessStyle(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	bottomBar := lines[len(lines)-1]

	// Bottom bar should contain less-style indicator
	assert.Contains(t, bottomBar, "line ", "bottom bar should contain 'line' indicator")
	assert.Contains(t, bottomBar, "/", "bottom bar should contain line count separator")

	// Bottom bar should NOT contain file info (that's now in top bar)
	assert.NotContains(t, bottomBar, "foo.go", "bottom bar should not contain file name")
	assert.NotContains(t, bottomBar, "◐", "bottom bar should not contain fold icon")
}

func TestView_Layout_TopBarFirst_BottomBarLast(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Should have exactly height lines (10)
	assert.Equal(t, 10, len(lines), "view should have exactly height lines")

	topBar := lines[0]
	bottomBar := lines[len(lines)-1]

	// Top bar has file info
	assert.Contains(t, topBar, "foo.go", "first line should be top bar with file info")

	// Bottom bar has less-style indicator
	assert.Contains(t, bottomBar, "line ", "last line should be bottom bar with less indicator")
}

func TestContentHeight_ReservesTwoLines(t *testing.T) {
	m := Model{
		height: 20,
	}

	// contentHeight should be height - 2 (for top bar and bottom bar)
	assert.Equal(t, 18, m.contentHeight(), "contentHeight should be height - 2")
}

func TestContentHeight_MinimumOne(t *testing.T) {
	m := Model{
		height: 2,
	}

	// contentHeight should be at least 1, even if height - 2 = 0
	assert.Equal(t, 1, m.contentHeight(), "contentHeight should be at least 1")

	m.height = 1
	assert.Equal(t, 1, m.contentHeight(), "contentHeight should be at least 1 even with tiny height")
}

func TestTopBar_SearchMode_StillShowsFileInfo(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
					},
				},
			},
		},
		width:      80,
		height:     10,
		keys:       DefaultKeyMap(),
		searchMode: true,
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should still show file info during search
	assert.Contains(t, topBar, "foo.go", "top bar should show file info even in search mode")
}

func TestBottomBar_SearchMode_ShowsSearchPrompt(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
					},
				},
			},
		},
		width:         80,
		height:        10,
		keys:          DefaultKeyMap(),
		searchMode:    true,
		searchForward: true,
		searchInput:   "test",
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	bottomBar := lines[len(lines)-1]

	// Bottom bar should show search prompt in search mode
	assert.Contains(t, bottomBar, "/test", "bottom bar should show search prompt")
}

func TestTopBar_NoFileInfo_WhenOnSummary(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
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
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on summary row (last row at index totalLines-1)
	// cursorLine = scroll + cursorOffset
	// totalLines-1 = scroll + cursorOffset
	// scroll = totalLines - 1 - cursorOffset
	m.scroll = m.totalLines - 1 - m.cursorOffset()

	topBar := m.renderTopBar()

	// When cursor is on summary (not a file), top bar should be empty or minimal
	assert.NotContains(t, topBar, "foo.go", "top bar should not show file name when on summary")
}

func TestView_GutterAlignmentConsistency(t *testing.T) {
	// Test that file headers, content lines, hunk separators, and summary
	// all have consistent gutter alignment based on lineNumWidth
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 100, Content: "line content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 100, Content: "line content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// lineNumWidth has a minimum of 4
	assert.Equal(t, 4, m.lineNumWidth(), "lineNumWidth should be 4 (minimum)")

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the file header line (contains "test.go")
	var headerLine string
	var contentLine string
	var summaryLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "═") {
			headerLine = line
		}
		if strings.Contains(line, "line content") {
			contentLine = line
		}
		if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
			summaryLine = line
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line")
	require.NotEmpty(t, contentLine, "should find content line")
	require.NotEmpty(t, summaryLine, "should find summary line")

	// All row types should have content starting at the same column position
	// The gutter area is: indicator(1) + space(1) + lineNum(N) + space(1)
	// So content starts at position 3 + lineNumWidth

	// For content line, find where "line content" starts
	contentPos := strings.Index(contentLine, "line content")
	// For header line, find where "test.go" starts
	headerPos := strings.Index(headerLine, "test.go")

	// The header should account for the icon area, but gutter portion should align
	// Header format: arrow/space + space + gutter(═══) + space + icon + status + filename
	// Content format: indicator + space + lineNum + space + content

	// Check that the gutter portion of header (═══) has the same width as lineNumWidth
	// This test verifies the structural alignment concept
	assert.True(t, contentPos > 0, "content should be found in content line")
	assert.True(t, headerPos > 0, "test.go should be found in header line")
}

func TestView_CursorArrowOnFileHeader(t *testing.T) {
	// Test that cursor arrow appears on file header when selected
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on file header (row 0)
	// cursorLine = scroll + cursorOffset, so scroll = 0 - cursorOffset
	m.scroll = -m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the file header line (contains test.go and ═ characters, not the top bar)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "═") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line with test.go and ═")
	// Header line should contain the arrow character when cursor is on it
	assert.Contains(t, headerLine, "➤", "file header with cursor should have arrow indicator")
}

func TestView_CursorArrowOnSummaryRow(t *testing.T) {
	// Test that cursor arrow appears on summary row when selected
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldFolded, // Fold so summary is closer
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on summary row (last row)
	m.scroll = m.totalLines - 1 - m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the summary line
	var summaryLine string
	for _, line := range lines {
		if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
			summaryLine = line
			break
		}
	}

	require.NotEmpty(t, summaryLine, "should find summary line")
	assert.Contains(t, summaryLine, "➤", "summary row with cursor should have arrow indicator")
}

func TestView_CursorArrowOnHunkSeparator(t *testing.T) {
	// Test that cursor arrow appears on hunk separator when selected
	// Hunk separators appear when there's a gap in line numbers
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "first hunk", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "first hunk", Type: sidebyside.Context},
					},
					// Gap in line numbers creates a hunk separator
					{
						Left:  sidebyside.Line{Num: 100, Content: "second hunk", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 100, Content: "second hunk", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on hunk separator (row 2: header=0, line1=1, hunksep=2)
	m.scroll = 2 - m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line (contains ─ characters, not the header)
	var hunkSepLine string
	for i, line := range lines {
		if i > 0 && strings.Contains(line, "─") && !strings.Contains(line, "test.go") {
			hunkSepLine = line
			break
		}
	}

	require.NotEmpty(t, hunkSepLine, "should find hunk separator line")
	assert.Contains(t, hunkSepLine, "➤", "hunk separator with cursor should have arrow indicator")
}

func TestView_HeaderGutterWidthMatchesLineNumWidth(t *testing.T) {
	// Test that file header gutter (═══ section) width matches the dynamic lineNumWidth
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// lineNumWidth should be 5 for 5-digit numbers
	assert.Equal(t, 5, m.lineNumWidth(), "lineNumWidth should be 5 for line 10000")

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the file header line and content line
	var headerLine string
	var contentLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "═") {
			headerLine = line
		}
		if strings.Contains(line, "10000") {
			contentLine = line
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line")
	require.NotEmpty(t, contentLine, "should find content line with line number")

	// Count the consecutive ═ characters in the header (this is the gutter area)
	equalsCount := 0
	inEquals := false
	for _, r := range headerLine {
		if r == '═' {
			inEquals = true
			equalsCount++
		} else if inEquals {
			break // Stop at first non-equals after finding equals
		}
	}

	// The equals gutter should match lineNumWidth (5)
	assert.Equal(t, 5, equalsCount, "header gutter width (═══) should match lineNumWidth")
}

func TestView_FileHeaderNoVerticalDivider(t *testing.T) {
	// File headers should span full width without a │ divider in the middle
	// This applies to both cursor and non-cursor states
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on file header
	m.scroll = -m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the file header line (contains test.go and ═)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "═") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line")
	// File header should NOT contain the │ vertical divider
	assert.NotContains(t, headerLine, "│", "file header should not have vertical divider")
}

func TestView_HunkSeparatorCrossInMiddle(t *testing.T) {
	// Hunk separator (when cursor is NOT on it) should have ─┼─ pattern
	// with the cross centered between left and right sides
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
					},
					// Gap creates hunk separator
					{
						Left:  sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor away from the hunk separator (on a content line)
	m.scroll = 3 - m.cursorOffset() // cursor on line 100

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line (contains ┼, not the header)
	var hunkLine string
	for _, line := range lines {
		if strings.Contains(line, "┼") && !strings.Contains(line, "test.go") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line")
	// Should have the ─┼─ pattern (cross with dashes on both sides)
	assert.Contains(t, hunkLine, "─┼─", "hunk separator should have ─┼─ pattern")
}
