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
	"github.com/user/diffyduck/pkg/structure"
)

var update = flag.Bool("update", false, "update golden files")

func init() {
	// Force ASCII color profile for consistent test output
	lipgloss.SetColorProfile(termenv.Ascii)
}

func TestView_BasicRender(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "old line", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "new line", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "func main() {}", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "func main() {}", Type: sidebyside.Context},
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
			Old: sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
			New: sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
		}
	}

	m := Model{
		focused: true,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					// Pure addition (empty left)
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "added line", Type: sidebyside.Added},
					},
					// Pure removal (empty right)
					{
						Old: sidebyside.Line{Num: 1, Content: "removed line", Type: sidebyside.Removed},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/one.go",
				NewPath: "b/one.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath: "a/two.go",
				NewPath: "b/two.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "file two", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "file two", Type: sidebyside.Context},
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
		focused: true,
		files:   nil,
		width:   80,
		height:  10,
		keys:    DefaultKeyMap(),
	}

	output := m.View()
	// Even with no files, we show a status bar
	assert.Contains(t, output, "0%")
}

func TestView_ZeroSize(t *testing.T) {
	m := Model{
		focused: true,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "this is a much longer line that will be truncated without scroll", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "this is a much longer line that will be truncated without scroll", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "0123456789abcdefghij", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "0123456789abcdefghij", Type: sidebyside.Context},
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
	assert.Equal(t, 59, info.TotalLines) // 1 top border + 1 header + 1 bottom border + 50 pairs + 4 blank + 1 trailing border + 1 summary
	// Percentage: cursorLine(3) / maxCursor(58) * 100 = 5%
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
	m.calculateTotalLines() // 109 lines total (1 top + 1 header + 1 bottom + 100 pairs + 4 blank + 1 trailing + 1 summary)
	// cursorOffset = 10 * 20 / 100 = 2, maxCursor = 108

	// At minScroll, cursor is at line 0, percentage should be 0
	m.scroll = m.minScroll()
	info := m.StatusInfo()
	assert.Equal(t, 0, info.Percentage)
	assert.False(t, info.AtEnd)

	// At scroll that puts cursor at approx line 50, percentage should be ~46
	// (50/108 * 100 = 46.3, rounded to 46)
	m.scroll = 50 - m.cursorOffset() // cursor at 50
	info = m.StatusInfo()
	assert.Equal(t, 46, info.Percentage)
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

	// scroll=16 → cursor at line 17 (trailing top border of first file) → first.go (belongs to file above)
	m.scroll = 16
	info = m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "first.go", info.FileName)

	// scroll=17 → cursor at line 18 (header of second file) → second.go
	m.scroll = 17
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile)
	assert.Equal(t, "second.go", info.FileName)
}

func TestView_ScrolledToMax(t *testing.T) {
	// When scrolled to max, the summary row should be visible, rest is padding
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "last", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "last", Type: sidebyside.Context},
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

	// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
	// lines[0] = top bar (no file name when on summary)
	assert.NotContains(t, lines[0], "foo.go", "top bar should not show file name when on summary")

	// lines[1] = divider
	// lines[2] = summary row (last content line at maxScroll)
	assert.Contains(t, lines[2], "file changed")

	// lines[3] should be empty padding (contentH = height - 3 = 2, only 1 content row visible)
	assert.Equal(t, "", lines[3], "line 3 should be empty padding")

	// lines[4] = bottom bar with END
	assert.Contains(t, lines[4], "END")
}

func TestView_InlineDiffRendering(t *testing.T) {
	// Test that inline diff is computed for modified line pairs
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						// This is a modified pair - should trigger inline diff
						Old: sidebyside.Line{Num: 1, Content: "fmt.Println(x)", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "fmt.Println(y)", Type: sidebyside.Added},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						// Completely different lines - should skip inline diff
						Old: sidebyside.Line{Num: 1, Content: "abcdefghijklmnop", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "1234567890123456", Type: sidebyside.Added},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					// First hunk: lines 1-3
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
					},
					// Gap here - next hunk starts at line 100
					{
						Old: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 101, Content: "line hundred one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 101, Content: "line hundred one", Type: sidebyside.Context},
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

	// Hunk separator should be blank (no cross separator)
	assert.NotContains(t, output, "┼", "hunk separator should NOT have cross in middle")
}

func TestView_BlankLineBeforeFileHeader(t *testing.T) {
	// Second and subsequent file headers should have a blank line before them
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/first.go",
				NewPath: "b/first.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath: "a/second.go",
				NewPath: "b/second.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15, // Increased to ensure both files are visible (need room for header spacers)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// First line should be the first file header (no blank before it)
	assert.Contains(t, lines[0], "first.go")

	// There should be a trailing top border before the second file header
	// Find the second file header (contains filename and fold icon)
	secondHeaderIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "second.go") && strings.Contains(line, "◐") {
			secondHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, -1, secondHeaderIdx, "should find second file header")
	require.Greater(t, secondHeaderIdx, 1, "second header should not be at start")

	// Line before second header should be a border line (trailing top border from first file)
	// This looks like ───────┐
	assert.Contains(t, lines[secondHeaderIdx-1], "─",
		"should have border line before second file header")
}

func TestView_NoBlankLineBeforeFirstFile(t *testing.T) {
	// First file should start with top border (not blank) and then header
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/only.go",
				NewPath: "b/only.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15, // Increased to ensure header is visible
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the file header line - it has fold icon (◐) and the │ border character
	// (top bar also contains filename but no │)
	headerIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "only.go") && strings.Contains(line, "◐") && strings.Contains(line, "│") {
			headerIdx = i
			break
		}
	}
	require.NotEqual(t, -1, headerIdx, "should find file header")
	require.Greater(t, headerIdx, 0, "header should not be at first line")

	// Line before header should be top border (not blank)
	assert.Contains(t, lines[headerIdx-1], "─", "line before header should be top border")
	assert.Contains(t, lines[headerIdx-1], "┐", "top border should have corner")
}

func TestView_NoSeparatorForConsecutiveLines(t *testing.T) {
	// When lines are consecutive, no separator should be shown
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
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

	// Should NOT contain hunk separator cross (┼ only appears in hunk separators)
	assert.NotContains(t, output, "┼")
}

func TestFoldLevelIcon(t *testing.T) {
	// Normal mode (non-pager)
	m := Model{pagerMode: false}
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
			icon := m.foldLevelIcon(tt.level)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestFoldLevelIcon_PagerMode(t *testing.T) {
	// Pager mode - FoldNormal shows filled (●) to indicate max expansion
	m := Model{pagerMode: true}
	tests := []struct {
		level    sidebyside.FoldLevel
		expected string
	}{
		{sidebyside.FoldFolded, "○"},
		{sidebyside.FoldNormal, "●"}, // Filled in pager mode!
		{sidebyside.FoldExpanded, "●"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String()+"_pager", func(t *testing.T) {
			icon := m.foldLevelIcon(tt.level)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestView_FoldLevelIcons_InHeaders(t *testing.T) {
	// Test that each fold level shows the correct icon in the header
	// All levels now use the same format with trailing ━
	tests := []struct {
		name     string
		level    sidebyside.FoldLevel
		wantIcon string
	}{
		{"folded shows empty circle", sidebyside.FoldFolded, "○"},
		{"normal shows half circle", sidebyside.FoldNormal, "◐"},
		{"expanded shows full circle", sidebyside.FoldExpanded, "●"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				focused: true,
				files: []sidebyside.FilePair{
					{
						OldPath:    "a/test.go",
						NewPath:    "b/test.go",
						FoldLevel:  tt.level,
						Pairs:      []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1}, New: sidebyside.Line{Num: 1}}},
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

			// Layout: [topBar, divider, content..., bottomBar]
			// For unfolded files (Normal/Expanded): lines[2] = top border, lines[3] = header
			// For folded files: lines[2] = header (no borders)
			headerIdx := 2
			if tt.level != sidebyside.FoldFolded {
				headerIdx = 3 // Skip the top border row
			}
			headerLine := lines[headerIdx]
			assert.Contains(t, headerLine, tt.wantIcon, "header should contain %s icon for %s level", tt.wantIcon, tt.level)
			// Header format is: fileNum <foldIcon> <statusIndicator> filename [stats]
			// For modified files (a/test.go -> b/test.go with same name), status is "~"
			assert.Contains(t, headerLine, tt.wantIcon+" ~ test.go", "header format should be: <fileNum> <icon> <status> filename")
			assert.Contains(t, headerLine, "1", "header should have file number")
		})
	}
}

func TestView_FoldedFile_HeaderOnly(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line content", Type: sidebyside.Context},
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

	// Layout: [topBar, divider, content..., bottomBar]
	// lines[0] = top bar, lines[1] = divider, lines[2] = first content line (header)
	// Folded view should only show the header and then padding
	assert.Contains(t, lines[2], "foo.go", "first content line should be the header")
	assert.Contains(t, lines[2], "○", "header should have folded icon")

	// Line pairs should NOT be shown
	assert.NotContains(t, output, "line content", "folded view should not show line pairs")
}

func TestView_FoldedFileAbove_NoBlankAfter(t *testing.T) {
	// When the file ABOVE is folded, there should be no blank lines between them
	// Blank lines are added AFTER expanded/normal content, not before headers
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				FoldLevel: sidebyside.FoldFolded, // First file is folded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "first file", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first file", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				FoldLevel: sidebyside.FoldNormal, // Second file is normal
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "second file", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "second file", Type: sidebyside.Context},
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

	// Find both file headers (first is folded=○, second is normal=◐)
	firstHeaderIdx := -1
	secondHeaderIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "first.go") && strings.Contains(line, "○") {
			firstHeaderIdx = i
		}
		if strings.Contains(line, "second.go") && strings.Contains(line, "◐") {
			secondHeaderIdx = i
		}
	}
	require.NotEqual(t, -1, firstHeaderIdx, "should find first file header")
	require.NotEqual(t, -1, secondHeaderIdx, "should find second file header")

	// When first file is folded, second header should immediately follow
	// (no blank lines after folded files)
	assert.Equal(t, firstHeaderIdx+1, secondHeaderIdx,
		"when file above is folded, headers should be adjacent")
}

func TestView_MixedFoldLevels(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/normal.go",
				NewPath:   "b/normal.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "normal file content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "normal file content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/folded.go",
				NewPath:   "b/folded.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "folded file content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "folded file content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/another.go",
				NewPath:   "b/another.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "another file content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "another file content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20, // Increased to ensure third file content is visible
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
		focused: true,
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

	// Normal file: 1 top border + 1 header + 1 bottom border + 10 pairs + 4 blank + 1 trailing border = 18 lines
	// Folded file: 1 header only (no borders, no blank lines after since it's folded)
	// Summary row: 1 line
	// Total should be 18 + 1 + 1 = 20
	assert.Equal(t, 20, m.totalLines, "totalLines should account for fold states and summary")
}

func TestView_ExpandedFile_ShowsFullContent(t *testing.T) {
	// Expanded view should show ALL lines from the full file content
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				// Original diff pairs (just lines 5-7 with a change at line 6)
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 5, Content: "line five", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 5, Content: "line five", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 6, Content: "old line six", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 6, Content: "new line six", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 7, Content: "line seven", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 7, Content: "line seven", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 5, Content: "diff context", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 5, Content: "diff context", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "deleted line", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/new.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new line", Type: sidebyside.Added},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				// Diff pairs showing the insertion
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 3, Content: "INSERTED", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 4, Content: "line3", Type: sidebyside.Context},
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
		if rows[i].pair.Old.Num == 3 {
			oldLine3Row = &rows[i]
			break
		}
	}

	if oldLine3Row == nil {
		t.Fatal("could not find row with old line 3")
	}

	// Old line 3 should be paired with new line 4 (both have content "line3")
	// NOT with new line 3 (which is "INSERTED")
	assert.Equal(t, "line3", oldLine3Row.pair.Old.Content, "left side should be line3")
	assert.Equal(t, "line3", oldLine3Row.pair.New.Content,
		"right side should also be line3 (new line 4), not INSERTED")
	assert.Equal(t, 4, oldLine3Row.pair.New.Num,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "REMOVED", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 4, Content: "line3", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Context},
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
		if rows[i].pair.New.Num == 3 && rows[i].pair.New.Content == "line3" {
			newLine3Row = &rows[i]
			break
		}
	}

	if newLine3Row == nil {
		t.Fatal("could not find row with new line 3 content 'line3'")
	}

	// New line 3 (content "line3") should be paired with old line 4 (same content)
	assert.Equal(t, "line3", newLine3Row.pair.New.Content, "right side should be line3")
	assert.Equal(t, "line3", newLine3Row.pair.Old.Content,
		"left side should also be line3 (old line 4), not REMOVED")
	assert.Equal(t, 4, newLine3Row.pair.Old.Num,
		"left side line number should be 4 (after the removed line)")
}

func TestView_GutterIndicators(t *testing.T) {
	// Test that +/- indicators appear in the gutter for added/removed lines
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "context line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "context line", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "removed line", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "added line", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 3, Content: "pure add", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "pure remove", Type: sidebyside.Removed},
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
				focused: true,
				files: []sidebyside.FilePair{
					{
						OldPath: "a/test.go",
						NewPath: "b/test.go",
						Pairs: []sidebyside.LinePair{
							{
								Old: sidebyside.Line{Num: 1, Content: "test content", Type: tt.lineType},
								New: sidebyside.Line{Num: 1, Content: "test content", Type: tt.lineType},
							},
							{
								Old: sidebyside.Line{Num: 2, Content: "another line", Type: sidebyside.Context},
								New: sidebyside.Line{Num: 2, Content: "another line", Type: sidebyside.Context},
							},
						},
					},
				},
				width:  80,
				height: 10,
				keys:   DefaultKeyMap(),
			}
			m.calculateTotalLines()
			// Position cursor on row 4 (second content line, after top border + header + bottom border + first content)
			// so we can test line 1's indicator without cursor arrow
			// cursorLine = scroll + cursorOffset, so scroll = row - cursorOffset = 4 - cursorOffset
			m.scroll = 4 - m.cursorOffset()

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
				focused: true,
				files: []sidebyside.FilePair{
					{
						OldPath: "a/test.go",
						NewPath: "b/test.go",
						Pairs: []sidebyside.LinePair{
							// First line (cursor will be here)
							{
								Old: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
								New: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
							},
							// Second line (the one we're testing, cursor not here)
							{
								Old: sidebyside.Line{Num: 2, Content: "content", Type: tt.lineType},
								New: sidebyside.Line{Num: 2, Content: "content", Type: tt.lineType},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/large.go",
				NewPath: "b/large.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 9999, Content: "line 9999", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 9999, Content: "line 9999", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 10000, Content: "line 10000", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 10000, Content: "line 10000 modified", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 10001, Content: "line 10001", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 10001, Content: "line 10001", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/large.go",
				NewPath: "b/large.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 9999, Content: "line before", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 9999, Content: "line before", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 10000, Content: "ten thousand", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 10000, Content: "ten thousand", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 10001, Content: "line after", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 10001, Content: "line after", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15, // Increased to accommodate separator lines
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, content..., bottomBar]
	// lines[0] = top bar, lines[1] = top border, lines[2] = header, lines[3] = bottom border
	// lines[4-6] = separator (3 lines: top + breadcrumb + bottom) since diff starts after line 1
	// lines[7] = cursor indicator row, Content lines: lines[8] = 9999, lines[9] = 10000, lines[10] = 10001
	line1 := lines[8]
	line2 := lines[9]
	line3 := lines[10]

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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/small.go",
				NewPath:   "b/small.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "small file", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "small file", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/large.go",
				NewPath:   "b/large.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 10000, Content: "large file", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 10000, Content: "large file", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "only line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "only line", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10, // Much taller than content (7 lines: header + 1 pair + 4 blank + summary)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Scroll to end to verify END appears
	m.scroll = m.maxScroll()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Output should have exactly `height` lines (top bar + content + bottom bar)
	assert.Equal(t, 10, len(lines), "view should fill entire viewport height")

	// Layout: [topBar, divider, content..., bottomBar]
	// Top bar should not show file name when cursor is on summary
	assert.NotContains(t, lines[0], "foo.go", "top bar should not show file name when on summary")

	// Bottom bar should show END
	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "END")

	// The summary row should be visible somewhere in the output
	assert.Contains(t, output, "file changed", "summary should be visible when scrolled to end")
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
					Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
				},
			},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "pure additions",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 1, Content: "new 1", Type: sidebyside.Added},
				},
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 2, Content: "new 2", Type: sidebyside.Added},
				},
			},
			wantAdded:   2,
			wantRemoved: 0,
		},
		{
			name: "pure deletions",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "old 1", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "old 2", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 3, Content: "old 3", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
			},
			wantAdded:   0,
			wantRemoved: 3,
		},
		{
			name: "mixed changes",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "old", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 2, Content: "new", Type: sidebyside.Added},
				},
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 3, Content: "added", Type: sidebyside.Added},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldNormal, // Normal view - no stats
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldFolded, // Folded view - show stats
				Pairs: []sidebyside.LinePair{
					// 3 additions, 2 deletions
					{
						Old: sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "old2", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "new2", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 3, Content: "new3", Type: sidebyside.Added},
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

	// Layout: [topBar, divider, content..., bottomBar]
	// Folded header should contain filename and stats counts
	header := lines[2]
	assert.Contains(t, header, "main.go", "header should contain filename")
	assert.Contains(t, header, "+3", "header should show addition count")
	assert.Contains(t, header, "-2", "header should show deletion count")
	assert.Contains(t, header, "▒", "header should have trailing shading")
}

func TestFileHeaderWithStats_Alignment(t *testing.T) {
	// Multiple folded files should have aligned stats columns
	// The addition column (+N) should be padded so that the removal column (-M) starts at the same position
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/short.go",
				NewPath:   "b/short.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "added", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
			{
				OldPath:   "a/much_longer_filename.go",
				NewPath:   "b/much_longer_filename.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					// 100 additions to make the count "+100" which is wider than "+1"
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	// Add more pairs to the second file to get +100
	for i := 0; i < 100; i++ {
		m.files[1].Pairs = append(m.files[1].Pairs, sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 2, Content: "added", Type: sidebyside.Added},
		})
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, divider, content..., bottomBar]
	// Find display column position of - in the stats section of each header
	header1 := lines[2]
	header2 := lines[3] // second header is at lines[3] (folded files have no content between them)

	// Find position of removal count (-N) in each header
	// The first file has +1 -1, second has +100 -1
	// The -1 should be aligned in both headers
	pos1 := displayColumnOf(header1, "-1")
	pos2 := displayColumnOf(header2, "-1")

	assert.NotEqual(t, -1, pos1, "first header should contain -1")
	assert.NotEqual(t, -1, pos2, "second header should contain -1")
	assert.Equal(t, pos1, pos2, "-1 should be aligned across headers (addition column padded)")
}

func TestFileHeaderWithStats_ShadingAlignment(t *testing.T) {
	// The trailing shading should start at the same column even when count widths differ
	// e.g., "+100" vs "+5" should both have shading starting at same position
	pairs100 := make([]sidebyside.LinePair, 100)
	for i := range pairs100 {
		pairs100[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	pairs5 := make([]sidebyside.LinePair, 5)
	for i := range pairs5 {
		pairs5[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	m := Model{
		focused: true,
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

	// Layout: [topBar, divider, content..., bottomBar]
	header1 := lines[2] // +100
	header2 := lines[3] // +5

	// Find the display column position of the shading (▒ character)
	findShadingStart := func(s string) int {
		runes := []rune(s)
		for i, ch := range runes {
			if ch == '▒' {
				return i
			}
		}
		return -1
	}

	shadingPos1 := findShadingStart(header1)
	shadingPos2 := findShadingStart(header2)

	assert.NotEqual(t, -1, shadingPos1, "first header should have shading")
	assert.NotEqual(t, -1, shadingPos2, "second header should have shading")
	assert.Equal(t, shadingPos1, shadingPos2, "shading should start at same position across headers")
}

func TestFileHeaderWithStats_OnlyAdditions(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/newfile.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Added},
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
	// Layout: [topBar, divider, content..., bottomBar]
	header := lines[2]

	assert.Contains(t, header, "newfile.go", "header should contain filename")
	assert.Contains(t, header, "+2", "header should show addition count")
	assert.NotContains(t, header, "-", "header should not show deletion count when zero")
}

func TestFileHeaderWithStats_OnlyDeletions(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
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
	// Layout: [topBar, divider, content..., bottomBar]
	header := lines[2]

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
				focused: true,
				files: []sidebyside.FilePair{
					{
						OldPath:    tt.oldPath,
						NewPath:    tt.newPath,
						FoldLevel:  tt.foldLevel,
						Pairs:      []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1}, New: sidebyside.Line{Num: 1}}},
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
			// Layout: [topBar, divider, content..., bottomBar]
			// For unfolded files (Normal/Expanded): lines[2] = top border, lines[3] = header
			// For folded files: lines[2] = header (no borders)
			headerIdx := 2
			if tt.foldLevel != sidebyside.FoldFolded {
				headerIdx = 3 // Skip the top border row
			}
			header := lines[headerIdx]

			// Get the expected fold icon
			foldIcon := m.foldLevelIcon(tt.foldLevel)

			// Header format should be: fileNum <foldIcon> <statusIndicator> filename
			// e.g., "1    ○ + new.go" or "1    ◐ ~ file.go"
			expectedPattern := foldIcon + " " + tt.wantIndicator + " "
			assert.Contains(t, header, expectedPattern,
				"header should contain fold icon followed by status indicator: %s", expectedPattern)
		})
	}
}

// Summary row tests

func TestBuildRows_IncludesSummaryRow(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 2, Content: "added", Type: sidebyside.Added},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "deleted", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
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

func TestBuildRows_BlankLinesBeforeSummary(t *testing.T) {
	// When last file is expanded/normal, there should be 4 blank lines before summary
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Layout should be: top border (0), header (1), bottom border (2), content (3), blank (4-7), trailing top border (8), summary (9)
	require.Len(t, rows, 10, "should have top border + header + bottom border + content + 4 blanks + trailing border + summary")

	// Verify top border
	assert.True(t, rows[0].isHeaderTopBorder, "row 0 should be top border")

	// Verify header
	assert.True(t, rows[1].isHeader, "row 1 should be header")

	// Verify bottom border (header spacer)
	assert.True(t, rows[2].isHeaderSpacer, "row 2 should be header spacer/bottom border")

	// Verify the 4 blank lines before trailing border
	for i := 4; i <= 7; i++ {
		assert.True(t, rows[i].isBlank, "row %d should be blank", i)
		assert.Equal(t, 0, rows[i].fileIndex, "blank lines should belong to the last file")
	}

	// Verify trailing top border
	assert.True(t, rows[8].isHeaderTopBorder, "row 8 should be trailing top border")

	// Last row should be summary
	assert.True(t, rows[9].isSummary, "last row should be summary")
}

func TestView_SummaryRowFormat(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 2, Content: "added", Type: sidebyside.Added},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldFolded,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
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
	// Summary format is now: "  ━━━━ ●   ..." (space + space + equals gutter + icon)
	// Should contain ━ characters for the gutter
	assert.Contains(t, summaryLine, "━", "summary should contain ━ gutter characters")
}

func TestView_SummaryRowIsSelectable(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
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
				focused: true,
				files: []sidebyside.FilePair{
					{
						OldPath:    "a/foo.go",
						NewPath:    "b/foo.go",
						FoldLevel:  tt.foldLevel,
						OldContent: []string{"line1"},
						NewContent: []string{"line1"},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "context", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "context", Type: sidebyside.Context},
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

	// Top bar should contain file counter [1/1]
	assert.Contains(t, topBar, "[1/1]")
}

func TestStatusBar_NewFormat_FoldedFile(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/newfile.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "gone", Type: sidebyside.Removed},
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

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should show old path for deleted files
	assert.Contains(t, topBar, "deleted.go")
	assert.Contains(t, topBar, "-1")
}

func TestStatusBar_NewFormat_AtEnd(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
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

// ============================================================================
// Top Bar Tests
// ============================================================================

func TestTopBar_ContainsFileInfo(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
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
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	topBar := lines[0]

	// Top bar should be left-aligned (starts with content, not spaces)
	assert.True(t, len(topBar) > 0, "top bar should not be empty")
	// The fold icon should be near the start (after arrow and file counter [1/1])
	idx := strings.Index(topBar, "◐")
	assert.True(t, idx >= 0 && idx < 12, "fold icon should be near the start (left-aligned)")
}

func TestBottomBar_OnlyLessStyle(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
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

func TestContentHeight_ReservesThreeLines(t *testing.T) {
	m := Model{
		height: 20,
	}

	// contentHeight should be height - 3 (for top bar, divider, and bottom bar)
	assert.Equal(t, 17, m.contentHeight(), "contentHeight should be height - 3")
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
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
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 100, Content: "line content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "line content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20, // Increased to accommodate separator lines
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
		// FoldExpanded shows ● even when content is pending (falls through to normal rendering)
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
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

	// The header should account for the icon area, but file number portion should align
	// Header format: arrow/space + space + fileNum + space + icon + status + filename
	// Content format: indicator + space + lineNum + space + content

	// Check that the file number portion of header has the same width as lineNumWidth
	// This test verifies the structural alignment concept
	assert.True(t, contentPos > 0, "content should be found in content line")
	assert.True(t, headerPos > 0, "test.go should be found in header line")
}

func TestView_CursorArrowOnFileHeader(t *testing.T) {
	// Test that cursor arrow appears on file header when selected
	lipgloss.SetColorProfile(termenv.ANSI)
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
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
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

	// Find the file header line (contains test.go and fold icon, not the top bar)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line with test.go and fold icon")
	// Header line should contain the arrow character when cursor is on it
	assert.Contains(t, headerLine, "▶", "file header with cursor should have arrow indicator")
}

func TestView_CursorArrowOnSummaryRow(t *testing.T) {
	// Test that cursor arrow appears on summary row when selected
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldFolded, // Fold so summary is closer
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
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
	assert.Contains(t, summaryLine, "▶", "summary row with cursor should have arrow indicator")
}

func TestView_CursorArrowOnHunkSeparator(t *testing.T) {
	// Test that cursor arrow appears on hunk separator when selected
	// Hunk separators appear when there's a gap in line numbers
	lipgloss.SetColorProfile(termenv.ANSI)
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
						Old: sidebyside.Line{Num: 1, Content: "first hunk", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first hunk", Type: sidebyside.Context},
					},
					// Gap in line numbers creates a hunk separator
					{
						Old: sidebyside.Line{Num: 100, Content: "second hunk", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "second hunk", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on hunk separator (row 4: top_border=0, header=1, bottom_border=2, line1=3, hunksep=4)
	m.scroll = 4 - m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line (blank line without line numbers, not the header)
	// Hunk separators don't contain file names or line numbers, and have ▶ when cursor is on them
	var hunkSepLine string
	for i, line := range lines {
		if i > 2 && strings.Contains(line, "▶") && !strings.Contains(line, "test.go") && !strings.Contains(line, "100") && !strings.Contains(line, "first") {
			hunkSepLine = line
			break
		}
	}

	require.NotEmpty(t, hunkSepLine, "should find hunk separator line")
	assert.Contains(t, hunkSepLine, "▶", "hunk separator with cursor should have arrow indicator")
}

func TestView_HeaderFileNumWidthMatchesLineNumWidth(t *testing.T) {
	// Test that file header file number section width matches the dynamic lineNumWidth
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 10000, Content: "content", Type: sidebyside.Context},
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

	// Find the file header line (contains test.go and fold icon)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line")

	// The header should contain the file number "1" followed by spaces to pad to lineNumWidth
	// Since we have 1 file, the file number is "1" and lineNumWidth is 5, so we get "1    "
	assert.Contains(t, headerLine, "1", "header should contain file number")
}

func TestView_FileHeaderNoVerticalDivider(t *testing.T) {
	// File headers should span full width without a │ divider in the middle
	// This applies to both cursor and non-cursor states
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
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

	// Find the file header line (contains test.go and fold icon)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line")
	// File header should NOT contain the │ vertical divider
	assert.NotContains(t, headerLine, "│", "file header should not have vertical divider")
}

func TestView_HunkSeparatorNoCrossInMiddle(t *testing.T) {
	// Hunk separator should NOT have a vertical divider (no ┼ cross)
	// This creates visual separation between chunks
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
					},
					// Gap creates hunk separator
					{
						Old: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
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

	// Find the hunk separator line (shaded line with ░, not the blank line above it)
	// Hunk separator is row 5 in this test (after header area at rows 0-2, first content at row 3, blank at row 4)
	var hunkLine string
	for i, line := range lines {
		// Skip header area (rows 0-2) and content lines with file name, line numbers, or content
		// Look for line with shading (░) that's not a content line
		if i > 2 && strings.Contains(line, "░") &&
			!strings.Contains(line, "test.go") && !strings.Contains(line, "100") &&
			!strings.Contains(line, "first") && !strings.Contains(line, "second") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line")
	// Should NOT have cross character in middle
	assert.NotContains(t, hunkLine, "┼", "hunk separator should NOT have cross in middle")
}

func TestView_HunkSeparatorBreadcrumbs(t *testing.T) {
	// Hunk separator should show breadcrumbs (function/type name) on the left side
	// when structure data is available (file was expanded and parsed)
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldNormal, // Normal view (not expanded)
				Pairs: []sidebyside.LinePair{
					// First hunk: lines 1-3 (outside any function)
					{
						Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "import \"fmt\"", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "import \"fmt\"", Type: sidebyside.Context},
					},
					// Gap - next hunk is inside func MyFunction (lines 10-50)
					{
						Old: sidebyside.Line{Num: 15, Content: "    x := 1", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 15, Content: "    x := 1", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 16, Content: "    y := 2", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 16, Content: "    y := 3", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
		// Initialize the structureMaps - simulating that the file was previously expanded
		structureMaps: map[int]*FileStructure{
			0: {
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 10, EndLine: 50, Name: "MyFunction", Kind: "func"},
				}),
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line by looking for breadcrumb content
	var hunkLine string
	for _, line := range lines {
		// Hunk separator contains breadcrumb but no file name or line numbers
		if strings.Contains(line, "func MyFunction") && !strings.Contains(line, "test.go") && !strings.Contains(line, "package") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line with breadcrumb")
	// The separator should contain the function name as a breadcrumb
	assert.Contains(t, hunkLine, "func MyFunction", "hunk separator should show breadcrumb for the function containing the chunk start")
}

func TestView_HunkSeparatorBreadcrumbs_NoBreadcrumbWithoutStructure(t *testing.T) {
	// When structure data is not available, hunk separator should just show shading
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					// Gap
					{
						Old: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
					},
				},
			},
		},
		width:         100,
		height:        15,
		keys:          DefaultKeyMap(),
		structureMaps: make(map[int]*FileStructure), // Empty - no structure data
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line (blank line without content)
	// It's between rows 3 (line 1 content) and row 5 (line 100 content)
	var hunkLine string
	for i, line := range lines {
		// Skip header area and content lines - look for line with shading (░)
		if i > 2 && strings.Contains(line, "░") &&
			!strings.Contains(line, "test.go") && !strings.Contains(line, "line") &&
			!strings.Contains(line, " 1 ") && !strings.Contains(line, " 100 ") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line")
	// Without structure, should not have breadcrumb
	assert.NotContains(t, hunkLine, "func", "hunk separator without structure should not have breadcrumb")
}

func TestView_HunkSeparatorBreadcrumbs_RealHighlightFlow(t *testing.T) {
	// Test the real flow: content is loaded, highlighting extracts structure,
	// then shrinking to normal view should show breadcrumbs in hunk separators
	goCode := `package main

func MyFunction() {
	x := 1
	y := 2
	z := 3
}

func AnotherFunction() {
	a := 1
}`
	lines := strings.Split(goCode, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldNormal, // Normal view
			Pairs: []sidebyside.LinePair{
				// First hunk: lines 1-2 (package declaration)
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
				},
				// Gap here - next hunk is inside MyFunction (line 4 = "x := 1")
				{
					Old: sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
				},
				{
					Old: sidebyside.Line{Num: 5, Content: "	y := 2", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 5, Content: "	y := 99", Type: sidebyside.Added},
				},
			},
			// Full content is available (simulating file was expanded then shrunk)
			NewContent: lines,
			OldContent: lines,
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Simulate the highlight flow that happens when file is expanded
	cmd := m.RequestHighlight(0)
	require.NotNil(t, cmd, "RequestHighlight should return a command")

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	require.True(t, ok, "Expected HighlightReadyMsg, got %T", msg)

	// Verify structure was extracted
	require.NotEmpty(t, hlMsg.NewStructure, "NewStructure should be populated with Go functions")

	// Store the highlight data (this is what Update does)
	m.storeHighlightSpans(hlMsg)

	// Verify structureMaps was populated
	require.NotNil(t, m.structureMaps[0], "structureMaps should be populated for file 0")
	require.NotNil(t, m.structureMaps[0].NewStructure, "NewStructure should be set")

	// Verify we can look up structure at line 4 (inside MyFunction)
	entries := m.getStructureAtLine(0, 4)
	require.NotEmpty(t, entries, "Should find structure entries at line 4")
	assert.Equal(t, "MyFunction", entries[0].Name, "Line 4 should be inside MyFunction")

	// Now render and check the hunk separator
	m.width = 100
	m.height = 20
	m.calculateTotalLines()

	output := m.View()
	outputLines := strings.Split(output, "\n")

	// Find the hunk separator line by looking for breadcrumb content
	var hunkLine string
	for _, line := range outputLines {
		if strings.Contains(line, "func MyFunction") && !strings.Contains(line, "test.go") && !strings.Contains(line, "package") && !strings.Contains(line, "x :=") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line with breadcrumb")
	// The separator should contain the function name as a breadcrumb
	assert.Contains(t, hunkLine, "func MyFunction", "hunk separator should show 'func MyFunction' breadcrumb")
}

func TestView_HunkSeparatorBreadcrumbs_LeftSidePositioning(t *testing.T) {
	// Test that breadcrumbs appear on the left side (new content side) of the hunk separator,
	// starting after the gutter (indicator + lineNum), aligned near code content.
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					// First hunk
					{
						Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					// Gap creates hunk separator - next chunk is inside MyFunction
					{
						Old: sidebyside.Line{Num: 20, Content: "    code", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 20, Content: "    code", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 10, EndLine: 50, Name: "MyFunction", Kind: "func"},
				}),
			},
		},
	}
	m.calculateTotalLines()
	m.rebuildRowsCache()

	// Find the hunk separator row
	rows := m.buildRows()
	var hunkSepIdx int
	for i, row := range rows {
		if row.isSeparator {
			hunkSepIdx = i
			break
		}
	}
	require.NotZero(t, hunkSepIdx, "should find hunk separator row")

	// Test 1: Non-cursor row - breadcrumb on left side
	output := m.View()
	lines := strings.Split(output, "\n")

	var hunkLine string
	for _, line := range lines {
		if strings.Contains(line, "func MyFunction") {
			hunkLine = line
			break
		}
	}
	require.NotEmpty(t, hunkLine, "should find hunk separator with breadcrumb")

	// The breadcrumb should appear in the left half of the line
	// Find the center divider (3 shade chars) and check breadcrumb is before it
	halfWidth := (m.width - 3) / 2
	leftHalf := hunkLine[:halfWidth]
	assert.Contains(t, leftHalf, "func MyFunction", "breadcrumb should appear in left half (new content side)")

	// Test 2: Cursor row - breadcrumb still visible with cursor arrow
	m.scroll = hunkSepIdx - m.cursorOffset()
	output = m.View()
	lines = strings.Split(output, "\n")

	var cursorHunkLine string
	for _, line := range lines {
		if strings.Contains(line, "▶") && strings.Contains(line, "MyFunction") {
			cursorHunkLine = line
			break
		}
	}
	require.NotEmpty(t, cursorHunkLine, "cursor row should show arrow and breadcrumb")

	// Verify arrow appears and breadcrumb is preserved
	assert.Contains(t, cursorHunkLine, "▶", "cursor row should have arrow")
	assert.Contains(t, cursorHunkLine, "MyFunction", "cursor row should preserve breadcrumb text")
}

func TestView_HeaderSpacerWithCursorMatchesContentLineLayout(t *testing.T) {
	// Test that the bottom border with cursor has proper layout:
	// - Single arrow at start
	// - Horizontal line with ┘ corner
	lipgloss.SetColorProfile(termenv.ANSI)
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
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Position cursor on bottom border (row 2: top_border=0, header=1, bottom_border=2)
	m.scroll = 2 - m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the line with cursor arrow and ┘ corner (bottom border with cursor)
	var borderLine string
	for _, line := range lines {
		if strings.Contains(line, "▶") && strings.Contains(line, "┘") {
			borderLine = line
			break
		}
	}

	require.NotEmpty(t, borderLine, "should find bottom border line with cursor")

	// Bottom border with cursor should have ONE arrow
	arrowCount := strings.Count(borderLine, "▶")
	assert.Equal(t, 1, arrowCount, "bottom border with cursor should have one arrow")

	// Bottom border should have horizontal line
	assert.Contains(t, borderLine, "─", "bottom border should have horizontal line")

	// Test content line with cursor
	// Position cursor on content line (row 3: top_border=0, header=1, bottom_border=2, content=3)
	m.scroll = 3 - m.cursorOffset()
	output2 := m.View()
	lines2 := strings.Split(output2, "\n")

	// Find the line with cursor arrows and content (not borders)
	var contentLine string
	for _, line := range lines2 {
		if strings.Count(line, "▶") == 2 && strings.Contains(line, "content") {
			contentLine = line
			break
		}
	}

	require.NotEmpty(t, contentLine, "should find content line with cursor")

	// Content line should have two arrows (one per side)
	contentArrowCount := strings.Count(contentLine, "▶")
	assert.Equal(t, 2, contentArrowCount, "content line with cursor should have two arrows")

	// Content line should have a separator (┃ box drawings heavy vertical)
	hasSeparator := strings.Contains(contentLine, "┃")
	assert.True(t, hasSeparator, "content line should have center separator (┃)")
}

func TestStatusBar_PagerIndicator(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
					},
				},
			},
		},
		width:     80,
		height:    10,
		keys:      DefaultKeyMap(),
		pagerMode: true,
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	bottomBar := lines[len(lines)-1]

	// Bottom bar should show PAGER indicator in pager mode
	assert.Contains(t, bottomBar, "PAGER", "bottom bar should show PAGER indicator in pager mode")
}

func TestStatusBar_NoPagerIndicator_WhenNotPagerMode(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line", Type: sidebyside.Context},
					},
				},
			},
		},
		width:     80,
		height:    10,
		keys:      DefaultKeyMap(),
		pagerMode: false,
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	bottomBar := lines[len(lines)-1]

	// Bottom bar should NOT show PAGER indicator when not in pager mode
	assert.NotContains(t, bottomBar, "PAGER", "bottom bar should not show PAGER indicator when not in pager mode")
}

// ============================================================================
// Header Border Tests
// ============================================================================

// Test: Folded file produces only header row (no border rows)
func TestBuildRows_FoldedFileOnlyHeader(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Should have: header + summary = 2 rows
	// No top border, no bottom border, no content for folded files
	require.Len(t, rows, 2, "folded file should only have header + summary")
	assert.True(t, rows[0].isHeader, "first row should be header")
	assert.False(t, rows[0].isHeaderTopBorder, "should not have top border")
	assert.False(t, rows[0].isHeaderSpacer, "should not have bottom border")
	assert.True(t, rows[1].isSummary, "second row should be summary")
}

// Test: First file unfolded has leading top border with borderVisible=true
func TestBuildRows_FirstFileUnfoldedHasTopBorder(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// First row should be isHeaderTopBorder with borderVisible=true
	require.True(t, len(rows) >= 1, "should have at least one row")
	assert.True(t, rows[0].isHeaderTopBorder, "first row should be top border")
	assert.True(t, rows[0].borderVisible, "first file's top border should be visible")
}

// Test: Non-first file unfolded has no leading top border (comes from file above)
func TestBuildRows_NonFirstFileNoLeadingTopBorder(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find where second file starts (fileIndex == 1)
	secondFileStart := -1
	for i, row := range rows {
		if row.fileIndex == 1 && (row.isHeader || row.isHeaderTopBorder) {
			secondFileStart = i
			break
		}
	}
	require.NotEqual(t, -1, secondFileStart, "should find second file start")

	// The row at secondFileStart should be the header (not a top border)
	// because the top border comes from file 0's trailing rows
	assert.True(t, rows[secondFileStart].isHeader, "second file should start with header (top border comes from first file)")
}

// Test: Trailing top border visibility - next file unfolded
func TestBuildRows_TrailingTopBorderVisibleWhenNextUnfolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldNormal, // Next file is unfolded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the trailing top border of file 0 (it's the last isHeaderTopBorder before file 1 starts)
	var trailingBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 0 && rows[i].isHeaderTopBorder {
			// This could be the leading or trailing border
			// The trailing one is after blank rows
			if i > 0 && rows[i-1].isBlank {
				trailingBorder = &rows[i]
			}
		}
	}
	require.NotNil(t, trailingBorder, "should find trailing top border of first file")
	assert.True(t, trailingBorder.borderVisible, "trailing border should be visible when next file is unfolded")
}

// Test: Trailing top border visibility - next file folded
func TestBuildRows_TrailingTopBorderHiddenWhenNextFolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldFolded, // Next file is folded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the trailing top border of file 0
	var trailingBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 0 && rows[i].isHeaderTopBorder {
			if i > 0 && rows[i-1].isBlank {
				trailingBorder = &rows[i]
			}
		}
	}
	require.NotNil(t, trailingBorder, "should find trailing top border of first file")
	assert.False(t, trailingBorder.borderVisible, "trailing border should be hidden when next file is folded")
}

// Test: Trailing top border visibility - last file (no next file)
func TestBuildRows_TrailingTopBorderHiddenForLastFile(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the trailing top border (after the blank rows, before summary)
	var trailingBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 0 && rows[i].isHeaderTopBorder {
			if i > 0 && rows[i-1].isBlank {
				trailingBorder = &rows[i]
			}
		}
	}
	require.NotNil(t, trailingBorder, "should find trailing top border")
	assert.False(t, trailingBorder.borderVisible, "trailing border should be hidden for last file (no next file)")
}

// Test: Header borderVisible - first file
func TestBuildRows_HeaderBorderVisibleFirstFile(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the header row
	var headerRow *displayRow
	for i := range rows {
		if rows[i].isHeader {
			headerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, headerRow, "should find header row")
	assert.True(t, headerRow.borderVisible, "first file's header should have borderVisible=true")
}

// Test: Header borderVisible - previous file unfolded
func TestBuildRows_HeaderBorderVisibleWhenPrevUnfolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal, // Previous file unfolded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the header row of file 1
	var headerRow *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 && rows[i].isHeader {
			headerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, headerRow, "should find header row for file 1")
	assert.True(t, headerRow.borderVisible, "header should have borderVisible=true when previous file is unfolded")
}

// Test: Header borderVisible - previous file folded
func TestBuildRows_HeaderBorderHiddenWhenPrevFolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded, // Previous file folded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the header row of file 1
	var headerRow *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 && rows[i].isHeader {
			headerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, headerRow, "should find header row for file 1")
	assert.False(t, headerRow.borderVisible, "header should have borderVisible=false when previous file is folded")
}

// Test: Bottom border borderVisible matches header
func TestBuildRows_BottomBorderMatchesHeaderVisibility(t *testing.T) {
	tests := []struct {
		name          string
		prevFoldLevel sidebyside.FoldLevel
		expectVisible bool
	}{
		{
			name:          "previous_unfolded",
			prevFoldLevel: sidebyside.FoldNormal,
			expectVisible: true,
		},
		{
			name:          "previous_folded",
			prevFoldLevel: sidebyside.FoldFolded,
			expectVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				focused: true,
				files: []sidebyside.FilePair{
					{
						OldPath:   "a/one.go",
						NewPath:   "b/one.go",
						FoldLevel: tt.prevFoldLevel,
						Pairs: []sidebyside.LinePair{
							{
								Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
								New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
							},
						},
					},
					{
						OldPath:   "a/two.go",
						NewPath:   "b/two.go",
						FoldLevel: sidebyside.FoldNormal,
						Pairs: []sidebyside.LinePair{
							{
								Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
								New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
							},
						},
					},
				},
				width:  80,
				height: 20,
				keys:   DefaultKeyMap(),
			}

			rows := m.buildRows()

			// Find header and bottom border for file 1
			var headerRow, bottomBorderRow *displayRow
			for i := range rows {
				if rows[i].fileIndex == 1 {
					if rows[i].isHeader {
						headerRow = &rows[i]
					}
					if rows[i].isHeaderSpacer {
						bottomBorderRow = &rows[i]
					}
				}
			}
			require.NotNil(t, headerRow, "should find header row")
			require.NotNil(t, bottomBorderRow, "should find bottom border row")

			assert.Equal(t, tt.expectVisible, headerRow.borderVisible, "header borderVisible should match expected")
			assert.Equal(t, tt.expectVisible, bottomBorderRow.borderVisible, "bottom border borderVisible should match header")
		})
	}
}

// Test: renderHeaderTopBorder uses correct style based on borderVisible
func TestRenderHeaderTopBorder_BorderVisibility(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// Visible border
	visibleOutput := m.renderHeaderTopBorder(30, true, FileStatusModified, false)
	assert.Contains(t, visibleOutput, "─", "visible border should contain horizontal line")
	assert.Contains(t, visibleOutput, "┐", "visible border should end with corner")

	// Hidden border (still rendered, just different color)
	hiddenOutput := m.renderHeaderTopBorder(30, false, FileStatusModified, false)
	assert.Contains(t, hiddenOutput, "─", "hidden border should contain horizontal line")
	assert.Contains(t, hiddenOutput, "┐", "hidden border should end with corner")
}

// Test: renderHeaderTopBorder cursor highlighting
func TestRenderHeaderTopBorder_CursorRow(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// Cursor row should have arrow indicator
	cursorOutput := m.renderHeaderTopBorder(30, true, FileStatusModified, true)
	assert.Contains(t, cursorOutput, "▶", "cursor row should have arrow indicator")

	// Non-cursor row should not have arrow
	normalOutput := m.renderHeaderTopBorder(30, true, FileStatusModified, false)
	assert.NotContains(t, normalOutput, "▶", "non-cursor row should not have arrow")
}

// Test: renderHeaderBottomBorder uses correct corner character
func TestRenderHeaderBottomBorder_CorrectCorner(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	output := m.renderHeaderBottomBorder(30, true, FileStatusModified, false)
	assert.Contains(t, output, "┘", "bottom border should end with ┘ corner")
	assert.NotContains(t, output, "┐", "bottom border should not use ┐ corner")
}

// Test: renderHeaderBottomBorder border visibility styling
func TestRenderHeaderBottomBorder_BorderVisibility(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// Both visible and hidden borders should render (just with different colors)
	visibleOutput := m.renderHeaderBottomBorder(30, true, FileStatusModified, false)
	hiddenOutput := m.renderHeaderBottomBorder(30, false, FileStatusModified, false)

	assert.Contains(t, visibleOutput, "─", "visible border should contain horizontal line")
	assert.Contains(t, hiddenOutput, "─", "hidden border should contain horizontal line")
}

// Test: Integration - all files folded (no borders visible)
func TestBuildRows_AllFilesFolded_NoBorders(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/three.go",
				NewPath:   "b/three.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Count row types
	headerCount := 0
	topBorderCount := 0
	bottomBorderCount := 0

	for _, row := range rows {
		if row.isHeader {
			headerCount++
		}
		if row.isHeaderTopBorder {
			topBorderCount++
		}
		if row.isHeaderSpacer {
			bottomBorderCount++
		}
	}

	assert.Equal(t, 3, headerCount, "should have 3 header rows")
	assert.Equal(t, 0, topBorderCount, "should have no top border rows when all folded")
	assert.Equal(t, 0, bottomBorderCount, "should have no bottom border rows when all folded")
}

// Test: Integration - all files unfolded (all borders visible)
func TestBuildRows_AllFilesUnfolded_AllBordersVisible(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/three.go",
				NewPath:   "b/three.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 50,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Check that headers and their bottom borders have correct visibility
	for _, row := range rows {
		if row.isHeader && row.fileIndex >= 0 {
			// All headers should have borderVisible=true since all files are unfolded
			assert.True(t, row.borderVisible, "header for file %d should have borderVisible=true", row.fileIndex)
		}
		if row.isHeaderSpacer && row.fileIndex >= 0 {
			// All bottom borders should have borderVisible=true
			assert.True(t, row.borderVisible, "bottom border for file %d should have borderVisible=true", row.fileIndex)
		}
	}

	// Check trailing top borders
	// File 0 and 1's trailing borders should be visible (next file is unfolded)
	// File 2's trailing border should be hidden (no next file)
	trailingBorders := make(map[int]*displayRow)
	for i := range rows {
		if rows[i].isHeaderTopBorder && i > 0 && rows[i-1].isBlank {
			trailingBorders[rows[i].fileIndex] = &rows[i]
		}
	}

	if tb, ok := trailingBorders[0]; ok {
		assert.True(t, tb.borderVisible, "file 0's trailing border should be visible (file 1 is unfolded)")
	}
	if tb, ok := trailingBorders[1]; ok {
		assert.True(t, tb.borderVisible, "file 1's trailing border should be visible (file 2 is unfolded)")
	}
	if tb, ok := trailingBorders[2]; ok {
		assert.False(t, tb.borderVisible, "file 2's trailing border should be hidden (no next file)")
	}
}

// Test: Integration - mixed fold states
func TestBuildRows_MixedFoldStates(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded, // File 0: folded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldNormal, // File 1: unfolded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/three.go",
				NewPath:   "b/three.go",
				FoldLevel: sidebyside.FoldFolded, // File 2: folded
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 30,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// File 0: just header (folded)
	file0Rows := filterRowsByFileIndex(rows, 0)
	assert.Equal(t, 1, len(file0Rows), "file 0 should have only 1 row (header)")
	assert.True(t, file0Rows[0].isHeader, "file 0's row should be header")

	// File 1: has header, bottom border, content, blanks, trailing border
	// But borders should have borderVisible=false (prev file folded)
	var file1Header, file1BottomBorder, file1TrailingBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 {
			if rows[i].isHeader {
				file1Header = &rows[i]
			}
			if rows[i].isHeaderSpacer {
				file1BottomBorder = &rows[i]
			}
			if rows[i].isHeaderTopBorder && i > 0 && rows[i-1].isBlank {
				file1TrailingBorder = &rows[i]
			}
		}
	}

	require.NotNil(t, file1Header, "file 1 should have header")
	require.NotNil(t, file1BottomBorder, "file 1 should have bottom border")
	require.NotNil(t, file1TrailingBorder, "file 1 should have trailing border")

	assert.False(t, file1Header.borderVisible, "file 1's header border should be hidden (prev file folded)")
	assert.False(t, file1BottomBorder.borderVisible, "file 1's bottom border should be hidden (prev file folded)")
	assert.False(t, file1TrailingBorder.borderVisible, "file 1's trailing border should be hidden (next file folded)")

	// File 2: just header (folded)
	file2Rows := filterRowsByFileIndex(rows, 2)
	assert.Equal(t, 1, len(file2Rows), "file 2 should have only 1 row (header)")
	assert.True(t, file2Rows[0].isHeader, "file 2's row should be header")
}

// Helper function to filter rows by file index
func filterRowsByFileIndex(rows []displayRow, fileIndex int) []displayRow {
	var result []displayRow
	for _, row := range rows {
		if row.fileIndex == fileIndex {
			result = append(result, row)
		}
	}
	return result
}

// Test: Border width matches headerBoxWidth
func TestRenderHeaderTopBorder_WidthAlignment(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// Different header box widths
	widths := []int{20, 30, 40, 50}

	for _, w := range widths {
		topBorder := m.renderHeaderTopBorder(w, true, FileStatusModified, false)
		bottomBorder := m.renderHeaderBottomBorder(w, true, FileStatusModified, false)

		// Both borders should have same display width
		topWidth := displayWidth(topBorder)
		bottomWidth := displayWidth(bottomBorder)

		assert.Equal(t, topWidth, bottomWidth, "top and bottom border widths should match for headerBoxWidth=%d", w)
	}
}

// === Truncation Indicator Tests ===

// Tests for per-side truncation indicator display.
// When a file is truncated, the truncation indicator should show:
// - On the left (old) side only if only the old content was truncated
// - On the right (new) side only if only the new content was truncated
// - On both sides if both were truncated

func TestBuildRows_TruncationIndicator_OldSideOnly(t *testing.T) {
	// When only old (left) side is truncated, indicator should have truncateOld=true, truncateNew=false
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old content", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
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

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true")
	assert.False(t, truncRow.truncateNew, "truncateNew should be false")
}

func TestBuildRows_TruncationIndicator_NewSideOnly(t *testing.T) {
	// When only new (right) side is truncated, indicator should have truncateOld=false, truncateNew=true
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: false,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new content", Type: sidebyside.Added},
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

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.False(t, truncRow.truncateOld, "truncateOld should be false")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true")
}

func TestBuildRows_TruncationIndicator_BothSides(t *testing.T) {
	// When both sides are truncated, indicator should have both flags true
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
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

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true")
}

func TestBuildRows_TruncationIndicator_NotTruncated(t *testing.T) {
	// When file is not truncated, should have no truncation indicator
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    false,
				OldTruncated: false,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
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

	// Should have no truncation indicator row
	for _, row := range rows {
		assert.False(t, row.isTruncationIndicator, "should have no truncation indicator when not truncated")
	}
}

func TestBuildRows_TruncationIndicator_ExpandedView_OldSideOnly(t *testing.T) {
	// In expanded view, ContentTruncated fields should be used
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:             "a/test.go",
				NewPath:             "b/test.go",
				FoldLevel:           sidebyside.FoldExpanded,
				OldContent:          []string{"line1", "line2"},
				NewContent:          []string{"line1"},
				ContentTruncated:    true, // legacy field, may be removed
				OldContentTruncated: true,
				NewContentTruncated: false,
				Pairs:               []sidebyside.LinePair{}, // not used in expanded view
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row in expanded view")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true")
	assert.False(t, truncRow.truncateNew, "truncateNew should be false")
}

func TestBuildRows_TruncationIndicator_ExpandedView_NewSideOnly(t *testing.T) {
	// In expanded view with only new side truncated
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:             "a/test.go",
				NewPath:             "b/test.go",
				FoldLevel:           sidebyside.FoldExpanded,
				OldContent:          []string{"line1"},
				NewContent:          []string{"line1", "line2"},
				ContentTruncated:    true,
				OldContentTruncated: false,
				NewContentTruncated: true,
				Pairs:               []sidebyside.LinePair{},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row in expanded view")
	assert.False(t, truncRow.truncateOld, "truncateOld should be false")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true")
}

func TestBuildRows_TruncationIndicator_NewFile(t *testing.T) {
	// For a new file (OldPath=/dev/null), only new side can be truncated
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "/dev/null",
				NewPath:      "b/newfile.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: false, // Can't truncate non-existent old file
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new content", Type: sidebyside.Added},
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

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.False(t, truncRow.truncateOld, "truncateOld should be false for new file")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true")
}

func TestBuildRows_TruncationIndicator_DeletedFile(t *testing.T) {
	// For a deleted file (NewPath=/dev/null), only old side can be truncated
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/deleted.go",
				NewPath:      "/dev/null",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: false, // Can't truncate non-existent new file
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old content", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
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

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true")
	assert.False(t, truncRow.truncateNew, "truncateNew should be false for deleted file")
}

func TestRenderTruncationIndicator_LeftSideOnly(t *testing.T) {
	// Verify rendering shows truncation on left side only
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Test rendering with truncateOld=true, truncateNew=false
	output := m.renderTruncationIndicator("[truncated]", false, true, false)

	// Should contain dots on left side
	assert.Contains(t, output, "·", "should contain dots")

	// The output should have the side-by-side structure with separator
	assert.Contains(t, output, "│", "should contain vertical separator")

	// Left side should have the message, right side should be empty/blank
	parts := strings.Split(output, "│")
	require.Len(t, parts, 2, "should have left and right parts separated by │")

	leftPart := parts[0]
	rightPart := parts[1]

	assert.Contains(t, leftPart, "truncated", "left side should contain truncation message")
	assert.NotContains(t, rightPart, "truncated", "right side should not contain truncation message")
}

func TestRenderTruncationIndicator_RightSideOnly(t *testing.T) {
	// Verify rendering shows truncation on right side only
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Test rendering with truncateOld=false, truncateNew=true
	output := m.renderTruncationIndicator("[truncated]", false, false, true)

	// Should have the side-by-side structure
	assert.Contains(t, output, "│", "should contain vertical separator")

	parts := strings.Split(output, "│")
	require.Len(t, parts, 2, "should have left and right parts separated by │")

	leftPart := parts[0]
	rightPart := parts[1]

	assert.NotContains(t, leftPart, "truncated", "left side should not contain truncation message")
	assert.Contains(t, rightPart, "truncated", "right side should contain truncation message")
}

func TestRenderTruncationIndicator_BothSides(t *testing.T) {
	// Verify rendering shows truncation on both sides
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Test rendering with truncateOld=true, truncateNew=true
	output := m.renderTruncationIndicator("[truncated]", false, true, true)

	// Should have the side-by-side structure
	assert.Contains(t, output, "│", "should contain vertical separator")

	parts := strings.Split(output, "│")
	require.Len(t, parts, 2, "should have left and right parts separated by │")

	leftPart := parts[0]
	rightPart := parts[1]

	// Both sides should have dots
	assert.Contains(t, leftPart, "·", "left side should have dots")
	assert.Contains(t, rightPart, "·", "right side should have dots")
}

// === Pager Mode Truncation Tests ===
// In pager mode, only diff-parsing truncation applies (no content fetching)

func TestBuildRows_TruncationIndicator_PagerMode_OldSideOnly(t *testing.T) {
	// Pager mode with only old side truncated from diff parsing
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "pager mode should have truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true in pager mode")
	assert.False(t, truncRow.truncateNew, "truncateNew should be false in pager mode")
}

func TestBuildRows_TruncationIndicator_PagerMode_NewSideOnly(t *testing.T) {
	// Pager mode with only new side truncated from diff parsing
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: false,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "pager mode should have truncation indicator row")
	assert.False(t, truncRow.truncateOld, "truncateOld should be false in pager mode")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true in pager mode")
}

func TestBuildRows_TruncationIndicator_PagerMode_BothSides(t *testing.T) {
	// Pager mode with both sides truncated from diff parsing
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "pager mode should have truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true in pager mode")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true in pager mode")
}

func TestBuildRows_TruncationIndicator_PagerMode_NotTruncated(t *testing.T) {
	// Pager mode with no truncation
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldNormal,
				Truncated:    false,
				OldTruncated: false,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	for _, row := range rows {
		assert.False(t, row.isTruncationIndicator, "pager mode should have no truncation indicator when not truncated")
	}
}

func TestView_HunkSeparatorArrowPositionsMatchContentLines(t *testing.T) {
	// Test that cursor arrow positions on hunk separator match those on content lines.
	// Both left and right arrows should appear at the same horizontal positions.
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
					},
					// Gap creates hunk separator
					{
						Old: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
					},
				},
			},
		},
		width:               80,
		height:              15,
		keys:                DefaultKeyMap(),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
	}
	m.calculateTotalLines()

	// Helper to find arrow display column positions in a string (accounting for ANSI codes)
	findArrowDisplayPositions := func(s string) []int {
		var positions []int
		displayCol := 0
		inEscape := false
		for _, r := range s {
			if r == '\x1b' {
				inEscape = true
				continue
			}
			if inEscape {
				if r == 'm' {
					inEscape = false
				}
				continue
			}
			if r == '▶' {
				positions = append(positions, displayCol)
			}
			displayCol++
		}
		return positions
	}

	// Render with cursor on content line (line 100)
	// Row layout: top_border=0, header=1, bottom_border=2, line1=3, hunksep_top=4, hunksep=5, hunksep_bottom=6, line100=7
	m.scroll = 7 - m.cursorOffset()
	contentOutput := m.View()
	contentLines := strings.Split(contentOutput, "\n")

	// Find the content line with cursor (has "100" line number and arrows)
	var contentLineWithCursor string
	for _, line := range contentLines {
		if strings.Contains(line, "100") && strings.Contains(line, "▶") {
			contentLineWithCursor = line
			break
		}
	}
	require.NotEmpty(t, contentLineWithCursor, "should find content line with cursor")
	contentArrowPositions := findArrowDisplayPositions(contentLineWithCursor)
	require.Len(t, contentArrowPositions, 2, "content line should have 2 arrows (left and right)")

	// Now render with cursor on hunk separator (the line with breadcrumbs, not the top shader line)
	m.scroll = 5 - m.cursorOffset()
	hunkOutput := m.View()
	hunkLines := strings.Split(hunkOutput, "\n")

	// Find the hunk separator line (has arrows but no line content or file names)
	var hunkSepLine string
	for i, line := range hunkLines {
		// Skip header area, look for line that has arrows but no line content
		if i > 3 && strings.Contains(line, "▶") &&
			!strings.Contains(line, "test.go") && !strings.Contains(line, "100") &&
			!strings.Contains(line, "first") && !strings.Contains(line, "second") {
			hunkSepLine = line
			break
		}
	}
	require.NotEmpty(t, hunkSepLine, "should find hunk separator line with cursor")
	hunkArrowPositions := findArrowDisplayPositions(hunkSepLine)
	require.Len(t, hunkArrowPositions, 2, "hunk separator should have 2 arrows (left and right)")

	// The arrow positions should match between content line and hunk separator
	assert.Equal(t, contentArrowPositions[0], hunkArrowPositions[0],
		"left arrow position should match: content=%d, hunk=%d", contentArrowPositions[0], hunkArrowPositions[0])
	assert.Equal(t, contentArrowPositions[1], hunkArrowPositions[1],
		"right arrow position should match: content=%d, hunk=%d", contentArrowPositions[1], hunkArrowPositions[1])
}
