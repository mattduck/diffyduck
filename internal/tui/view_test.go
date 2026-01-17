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
	assert.Equal(t, 1, info.CurrentLine)
	assert.Equal(t, 51, info.TotalLines) // 50 pairs + 1 header
	assert.Equal(t, 0, info.Percentage)
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
		scroll: 0,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

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

	output := m.View()
	lines := strings.Split(output, "\n")
	lastLine := lines[len(lines)-1]

	// Status bar should contain the file name and END (since content fits in viewport)
	assert.Contains(t, lastLine, "foo.go")
	assert.Contains(t, lastLine, "END")
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
		scroll: 100, // Way past the content (only 6 lines)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.clampScroll() // This should clamp to maxScroll

	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "test.go", info.FileName)
	assert.True(t, info.AtEnd)
}

func TestStatusInfo_PercentageAccuracy(t *testing.T) {
	// Create 101 lines (100 pairs + 1 header) for easy percentage math
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
	m.calculateTotalLines() // 101 lines total

	// At scroll 0, percentage should be 0
	m.scroll = 0
	info := m.StatusInfo()
	assert.Equal(t, 0, info.Percentage)
	assert.False(t, info.AtEnd)

	// At scroll 50 (half of maxScroll=100), percentage should be 50
	m.scroll = 50
	info = m.StatusInfo()
	assert.Equal(t, 50, info.Percentage)
	assert.False(t, info.AtEnd)

	// At maxScroll (100), percentage should be 100
	m.scroll = 100
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
			{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs}, // lines 11-21
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Scroll 10 = last line of first file (pair 10)
	m.scroll = 10
	info := m.StatusInfo()
	assert.Equal(t, 1, info.CurrentFile)
	assert.Equal(t, "first.go", info.FileName)

	// Scroll 11 = header of second file
	m.scroll = 11
	info = m.StatusInfo()
	assert.Equal(t, 2, info.CurrentFile)
	assert.Equal(t, "second.go", info.FileName)
}

func TestView_ScrolledToMax(t *testing.T) {
	// When scrolled to max, only the last line should show at top, rest is padding
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
	m.calculateTotalLines() // 3 lines: header + 2 pairs
	m.scroll = 2            // Max scroll = 3 - 1 = 2, so last line at top

	output := m.View()
	lines := strings.Split(output, "\n")

	assert.Equal(t, 5, len(lines), "should have exactly height lines")

	// First line should be the last content line
	assert.Contains(t, lines[0], "last")

	// Lines 1-3 should be empty padding
	for i := 1; i < 4; i++ {
		assert.Equal(t, "", lines[i], "line %d should be empty padding", i)
	}

	// Last line should be status bar
	assert.Contains(t, lines[4], "foo.go")
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

	// Should contain a separator line with box drawing dashes and the cross in the middle
	assert.Contains(t, output, "─┼─")
	// Should have horizontal lines on both sides
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

	// First line should be the file header, not blank
	assert.Contains(t, lines[0], "only.go")
	assert.Contains(t, lines[0], "═══")
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

			// First non-blank line should be the header with the icon
			headerLine := lines[0]
			assert.Contains(t, headerLine, tt.wantIcon, "header should contain %s icon for %s level", tt.wantIcon, tt.level)
			assert.Contains(t, headerLine, "═══ "+tt.wantIcon+" test.go", "header format should be: ═══ <icon> filename")

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

	// Folded view should only show the header and then padding
	// The header should be on line 0
	assert.Contains(t, lines[0], "foo.go", "first line should be the header")
	assert.Contains(t, lines[0], "═══", "header should have the prefix")

	// Header should NOT have trailing "=" characters after the filename
	// The folded header format should be "═══ filename" without trailing "═"
	// Check that the line doesn't end with many "═" (like the normal header does)
	headerContent := strings.TrimRight(lines[0], " ")
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
	// Total should be 11 + 1 = 12
	assert.Equal(t, 12, m.totalLines, "totalLines should account for fold states")
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
		height: 10, // Much taller than content (2 lines: header + 1 pair)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Output should have exactly `height` lines (content + padding + status bar)
	assert.Equal(t, 10, len(lines), "view should fill entire viewport height")

	// Status bar should be the last line
	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "foo.go")
	assert.Contains(t, lastLine, "END")

	// First two lines should be content (header + pair)
	assert.Contains(t, lines[0], "foo.go")
	assert.Contains(t, lines[1], "only line")

	// Lines between content and status bar should be empty/padding
	for i := 2; i < 9; i++ {
		assert.Equal(t, "", strings.TrimSpace(lines[i]),
			"line %d should be empty padding", i)
	}
}
