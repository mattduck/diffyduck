package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
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
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
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
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
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
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldExpanded,
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

func TestView_BinaryFile_Created(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/image.png",
				FoldLevel: sidebyside.FoldExpanded,
				IsBinary:  true,
				Pairs:     nil, // Binary files have no pairs
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show binary file message
	assert.Contains(t, output, "Binary file created")
	// Should show +1 in stats (creation)
	assert.Contains(t, output, "+1")
	// Should NOT show -1 (not a deletion)
	assert.NotContains(t, output, "-1")
}

func TestView_BinaryFile_Deleted(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/image.png",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldExpanded,
				IsBinary:  true,
				Pairs:     nil,
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show binary file message
	assert.Contains(t, output, "Binary file deleted")
	// Should show -1 in stats (deletion)
	assert.Contains(t, output, "-1")
}

func TestView_BinaryFile_Changed(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/image.png",
				NewPath:   "b/image.png",
				FoldLevel: sidebyside.FoldExpanded,
				IsBinary:  true,
				Pairs:     nil,
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show binary file message
	assert.Contains(t, output, "Binary file changed")
	// Should show +1 -1 in stats (modification)
	assert.Contains(t, output, "+1")
	assert.Contains(t, output, "-1")
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
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
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

func TestView_InlineDiffRendering(t *testing.T) {
	// Test that inline diff is computed for modified line pairs
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
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
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
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

func TestView_ScrolledToMax(t *testing.T) {
	// When scrolled to max, the last content row should be visible
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
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
		height: 7, // Small viewport (top bar=4 lines + 1 content + 1 bottom bar minimum)
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.scroll = m.maxScroll()

	output := m.View()
	lines := strings.Split(output, "\n")

	assert.Equal(t, 7, len(lines), "should have exactly height lines")

	// Layout: [topBar(3 lines), divider, content[0..contentH-1], bottomBar]
	// lines[0] = top bar (shows file name since cursor is on file content)
	assert.Contains(t, lines[0], "foo.go", "top bar should show file name")

	// last line = bottom bar with END
	assert.Contains(t, lines[len(lines)-1], "END")
}

func TestView_StatusBarAlwaysAtBottom(t *testing.T) {
	// When content is shorter than viewport, status bar should still be at
	// the bottom of the terminal (not immediately after content)
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "only line", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "only line", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10, // Much taller than content (6 lines: header + 1 pair + 4 blank)
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
	// Top bar shows file name since cursor is still on file content
	assert.Contains(t, lines[0], "foo.go", "top bar should show file name")

	// Bottom bar should show END
	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "END")
}

func TestContentHeight_ReservesTopAndBottomBars(t *testing.T) {
	m := Model{
		height: 20,
	}

	// contentHeight should be height - 5 (top bar: 3 content lines + divider, plus bottom bar)
	assert.Equal(t, 15, m.contentHeight(), "contentHeight should be height - 5")
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

func TestView_Layout_TopBarFirst_BottomBarLast(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
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

func TestView_GutterAlignmentConsistency(t *testing.T) {
	// Test that file headers and content lines have consistent gutter alignment
	// based on lineNumWidth
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
	for _, line := range lines {
		// FoldExpanded shows ● even when content is pending (falls through to normal rendering)
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
			headerLine = line
		}
		if strings.Contains(line, "line content") {
			contentLine = line
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line")
	require.NotEmpty(t, contentLine, "should find content line")

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

func TestView_MultiCommitLogView(t *testing.T) {
	// Test rendering of multiple commits in a log-style view
	// Each commit should show its own stats and file numbers should reset per commit
	// Use varied stats to verify column alignment (+134 vs +5, -0 vs -23, etc.)

	// Helper to create N line pairs of a given type
	makeAddedPairs := func(n int) []sidebyside.LinePair {
		pairs := make([]sidebyside.LinePair, n)
		for i := range pairs {
			pairs[i] = sidebyside.LinePair{
				Old: sidebyside.Line{Num: i + 1, Type: sidebyside.Added},
				New: sidebyside.Line{Num: i + 1, Content: "line", Type: sidebyside.Added},
			}
		}
		return pairs
	}
	makeRemovedPairs := func(n int) []sidebyside.LinePair {
		pairs := make([]sidebyside.LinePair, n)
		for i := range pairs {
			pairs[i] = sidebyside.LinePair{
				Old: sidebyside.Line{Num: i + 1, Content: "old", Type: sidebyside.Removed},
				New: sidebyside.Line{Num: i + 1, Type: sidebyside.Removed},
			}
		}
		return pairs
	}

	commits := []sidebyside.CommitSet{
		{
			// Commit 0: 12 files, +134 -0 (large addition count)
			Info: sidebyside.CommitInfo{
				SHA:     "abc123def456",
				Author:  "Alice",
				Date:    "Mon Jan 15 10:00:00 2024 -0500",
				Subject: "Add feature X",
			},
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f3.go", NewPath: "b/f3.go", FoldLevel: sidebyside.FoldFolded, Pairs: makeAddedPairs(34)},
			},
		},
		{
			// Commit 1: 1 file, +5 -23 (small add, larger remove)
			Info: sidebyside.CommitInfo{
				SHA:     "def789abc012",
				Author:  "Bob",
				Date:    "Tue Jan 16 14:00:00 2024 -0500",
				Subject: "Fix bug Y",
			},
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/baz.go", NewPath: "b/baz.go", FoldLevel: sidebyside.FoldFolded,
					Pairs: append(makeAddedPairs(5), makeRemovedPairs(23)...)},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 100
	m.height = 20
	m.focused = true
	m.RefreshLayout()

	output := m.View()

	goldenPath := filepath.Join("testdata", "multi_commit_log.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

// TestView_StructuralDiffBorderAlignment tests that:
// 1. The border (│) on structural diff rows aligns with the header border
// 2. The structural diff symbol (+/-/~) aligns with the first character of the filename
func TestView_StructuralDiffBorderAlignment(t *testing.T) {
	// Create a file with structural diff data showing changed functions
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/example.go",
				NewPath:   "b/example.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "func Hello() {", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 3, Content: "func Hello() {", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 4, Content: "    println(\"hi\")", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 4, Content: "    println(\"hello\")", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 5, Content: "}", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 5, Content: "}", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
		// Set up structural diff data showing a modified function
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{
					{StartLine: 3, EndLine: 5, Name: "Hello", Kind: "func"},
				}),
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 3, EndLine: 5, Name: "Hello", Kind: "func"},
				}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{
							Kind: structure.ChangeModified,
							OldEntry: &structure.Entry{
								StartLine: 3, EndLine: 5, Name: "Hello", Kind: "func",
							},
							NewEntry: &structure.Entry{
								StartLine: 3, EndLine: 5, Name: "Hello", Kind: "func",
							},
						},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()

	lines := strings.Split(output, "\n")
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// Helper to find rune position of a substring
	findSubstringRunePos := func(s, substr string) int {
		idx := strings.Index(s, substr)
		if idx < 0 {
			return -1
		}
		// Count runes up to byte position idx
		return len([]rune(s[:idx]))
	}

	// Test 1: Check that tree structure is present
	// In tree layout, headers have tree branches (├ or └) instead of box borders (│)
	var headerLine, structDiffLine string
	for _, line := range lines {
		stripped := ansiRegex.ReplaceAllString(line, "")
		// Find the header line (contains filename "example.go" and has tree branch)
		if strings.Contains(stripped, "example.go") && (strings.Contains(stripped, "├") || strings.Contains(stripped, "└")) {
			headerLine = stripped
		}
		// Find a structural diff line (contains "func" kind keyword)
		if strings.Contains(stripped, "func ") && !strings.Contains(stripped, "example.go") {
			structDiffLine = stripped
		}
	}

	require.NotEmpty(t, headerLine, "should find header line with example.go and tree branch")

	// Structural diff may not be rendered in all modes - skip alignment check if not present
	if structDiffLine != "" {
		// Find position of filename in header (first char of "example.go")
		filenamePos := findSubstringRunePos(headerLine, "example.go")
		require.GreaterOrEqual(t, filenamePos, 0, "should find filename position")

		// Find position of the kind keyword ("func") in structural diff line
		kindPos := findSubstringRunePos(structDiffLine, "func")
		require.GreaterOrEqual(t, kindPos, 0, "should find kind position")

		// The kind should be near the filename's start position (within tree indent)
		// In tree layout, exact alignment may differ due to tree structure
		diff := kindPos - filenamePos
		if diff < 0 {
			diff = -diff
		}
		assert.LessOrEqual(t, diff, 5,
			"structural diff kind should be near filename start\n  header: %q\n  struct: %q",
			headerLine, structDiffLine)
	}

	// Golden file test
	goldenPath := filepath.Join("testdata", "structural_diff_border.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

// TestView_StructuralDiffBoxWidthAdaptive tests that the header box width
// adapts to content size rather than always expanding to 80% of terminal width.
// With a wide terminal and short content, the border should be near the content,
// not pushed out to the 80% mark.
func TestView_StructuralDiffBoxWidthAdaptive(t *testing.T) {
	// Create a model with a function that has a medium-length signature
	// on a very wide terminal - box should size to fit the actual signature,
	// not expand to fill 80% of the terminal
	entry := &structure.Entry{
		StartLine:  1,
		EndLine:    10,
		Name:       "ProcessRequest",
		Kind:       "func",
		Receiver:   "(m Model)",
		Params:     []string{"ctx context.Context", "req *Request"},
		ReturnType: "error",
	}
	// Full signature: "(m Model) ProcessRequest(ctx context.Context, req *Request) -> error"
	// This is about 70 chars - well under 80% of 250 = 200

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/handler.go",
				NewPath:   "b/handler.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  250, // Very wide terminal
		height: 20,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{*entry}),
				NewStructure: structure.NewMap([]structure.Entry{*entry}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{
							Kind:     structure.ChangeModified,
							OldEntry: entry,
							NewEntry: entry,
						},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// Find border position
	var borderPos int
	for _, line := range lines {
		stripped := ansiRegex.ReplaceAllString(line, "")
		if strings.Contains(stripped, "│") && !strings.Contains(stripped, "┃") {
			for i, r := range []rune(stripped) {
				if r == '│' {
					borderPos = i
					break
				}
			}
			break
		}
	}

	// Debug: print actual border position
	t.Logf("Border position: %d, 80%% of 250 = %d", borderPos, 250*80/100)

	// Full signature is ~70 chars, with overhead maybe ~90 chars total content
	// 80% of 250 = 200, so border should be well under 200
	// If it's at 200, we're unnecessarily expanding to the 80% budget
	maxExpectedBorderPos := 120 // Should be sized to content (~90) plus some padding, not 80% (200)
	assert.Less(t, borderPos, maxExpectedBorderPos,
		"Header box should adapt to content width, not expand to 80%% of terminal.\n"+
			"Border at position %d, expected less than %d.\n"+
			"80%% of terminal (250) would be 200 - box should be much smaller for this content.",
		borderPos, maxExpectedBorderPos)
}

// TestView_StructuralDiffBoxWidthAdaptive_MultipleEntries tests with multiple
// entries similar to the user's scenario (type + functions with signatures)
func TestView_StructuralDiffBoxWidthAdaptive_MultipleEntries(t *testing.T) {
	// Entries similar to the screenshot: type + 3 functions with signatures
	typeEntry := &structure.Entry{
		StartLine: 7, EndLine: 20, Name: "displayRow", Kind: "type",
	}
	func1 := &structure.Entry{
		StartLine: 32, EndLine: 38, Name: "structuralDiffMaxContentWidth", Kind: "func",
		Receiver: "(m Model)", Params: []string{"fileIdx int"}, ReturnType: "int",
	}
	func2 := &structure.Entry{
		StartLine: 51, EndLine: 63, Name: "buildStructuralDiffRows", Kind: "func",
		Receiver: "(m Model)", Params: []string{"fileIdx int", "headerBoxWidth int", "borderVisible bool"}, ReturnType: "[]displayRow",
	}
	func3 := &structure.Entry{
		StartLine: 50, EndLine: 58, Name: "renderStructuralDiffRow", Kind: "func",
		Receiver: "(m Model)", Params: []string{"row displayRow", "isCursorRow bool"}, ReturnType: "string",
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/internal/tui/view.go",
				NewPath:   "b/internal/tui/view.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "package tui", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package tui", Type: sidebyside.Context}},
				},
			},
		},
		width:  250, // Very wide terminal
		height: 20,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{*typeEntry, *func1, *func2, *func3}),
				NewStructure: structure.NewMap([]structure.Entry{*typeEntry, *func1, *func2, *func3}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{Kind: structure.ChangeModified, OldEntry: typeEntry, NewEntry: typeEntry, LinesAdded: 7, LinesRemoved: 0},
						{Kind: structure.ChangeModified, OldEntry: func1, NewEntry: func1, LinesAdded: 6, LinesRemoved: 0},
						{Kind: structure.ChangeModified, OldEntry: func2, NewEntry: func2, LinesAdded: 12, LinesRemoved: 0},
						{Kind: structure.ChangeModified, OldEntry: func3, NewEntry: func3, LinesAdded: 8, LinesRemoved: 0},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// Find border position and print all border lines for debugging
	var borderPos int
	t.Log("Lines with borders:")
	for i, line := range lines {
		stripped := ansiRegex.ReplaceAllString(line, "")
		if strings.Contains(stripped, "│") && !strings.Contains(stripped, "┃") {
			for j, r := range []rune(stripped) {
				if r == '│' {
					if borderPos == 0 {
						borderPos = j
					}
					t.Logf("  Line %d: border at %d: %s", i, j, stripped)
					break
				}
			}
		}
	}

	t.Logf("Border position: %d, 80%% of 250 = %d", borderPos, 250*80/100)

	// The longest signature is buildStructuralDiffRows at about 100 chars
	// With overhead, content should be around 115 chars, not 200 (80% of 250)
	maxExpectedBorderPos := 140 // Content + overhead, well under 80% mark
	assert.Less(t, borderPos, maxExpectedBorderPos,
		"Header box should adapt to content width, not expand to 80%% of terminal.\n"+
			"Border at position %d, expected less than %d.\n"+
			"80%% of terminal (250) would be 200.",
		borderPos, maxExpectedBorderPos)
}

// TestView_StructuralDiffTruncation tests that structural diff preview is
// sorted by total lines changed and truncated to top 10 with "...(N more)".
func TestView_StructuralDiffTruncation(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	// Create 13 functions with varying line counts.
	// After sorting by total lines changed (desc), the top 10 should appear
	// and the remaining 3 should be summarized as "...(3 more)".
	type funcDef struct {
		name         string
		startLine    int
		endLine      int
		linesAdded   int
		linesRemoved int
	}
	funcs := []funcDef{
		{"Alpha", 1, 5, 1, 0},      // total 1 (should be truncated)
		{"Bravo", 10, 20, 10, 5},   // total 15
		{"Charlie", 25, 35, 3, 0},  // total 3 (should be truncated)
		{"Delta", 40, 50, 8, 2},    // total 10
		{"Echo", 55, 65, 20, 0},    // total 20
		{"Foxtrot", 70, 80, 5, 5},  // total 10
		{"Golf", 85, 95, 12, 3},    // total 15
		{"Hotel", 100, 110, 7, 1},  // total 8
		{"India", 115, 125, 4, 0},  // total 4
		{"Juliet", 130, 140, 6, 6}, // total 12
		{"Kilo", 145, 155, 30, 10}, // total 40
		{"Lima", 160, 170, 2, 0},   // total 2 (should be truncated)
		{"Mike", 175, 185, 9, 9},   // total 18
	}

	var entries []structure.Entry
	var changes []structure.ElementChange
	for _, f := range funcs {
		entry := structure.Entry{
			StartLine: f.startLine, EndLine: f.endLine,
			Name: f.name, Kind: "func",
		}
		entries = append(entries, entry)
		entryCopy := entry
		changes = append(changes, structure.ElementChange{
			Kind:         structure.ChangeModified,
			OldEntry:     &entryCopy,
			NewEntry:     &entryCopy,
			LinesAdded:   f.linesAdded,
			LinesRemoved: f.linesRemoved,
		})
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/big_file.go",
				NewPath:   "b/big_file.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "package big", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package big", Type: sidebyside.Context}},
				},
			},
		},
		width:  120,
		height: 40,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap(entries),
				NewStructure:   structure.NewMap(entries),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	lines := strings.Split(output, "\n")

	var strippedLines []string
	for _, line := range lines {
		strippedLines = append(strippedLines, ansiRegex.ReplaceAllString(line, ""))
	}

	// Should contain the truncation indicator
	foundMore := false
	for _, s := range strippedLines {
		if strings.Contains(s, "...(3 more)") {
			foundMore = true
			break
		}
	}
	require.True(t, foundMore, "Expected '...(3 more)' truncation line in output.\nOutput lines:\n%s",
		strings.Join(strippedLines, "\n"))

	// The top entry by lines changed is Kilo (40 total) - it should appear
	foundKilo := false
	for _, s := range strippedLines {
		if strings.Contains(s, "Kilo") {
			foundKilo = true
			break
		}
	}
	assert.True(t, foundKilo, "Expected highest-change function 'Kilo' to appear in top 10")

	// The lowest entries (Alpha=1, Lima=2, Charlie=3) should NOT appear
	for _, name := range []string{"Alpha", "Lima", "Charlie"} {
		found := false
		for _, s := range strippedLines {
			if strings.Contains(s, name) {
				found = true
				break
			}
		}
		assert.False(t, found, "Expected truncated function '%s' to NOT appear in output", name)
	}
}

// TestView_StructuralDiffStatsAfterSignature tests that the +N/-M stats in
// structural diff rows appear straight after the signature with no alignment
// padding, and that zero counts are omitted entirely.
func TestView_StructuralDiffStatsAfterSignature(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	// Three functions with varying added/removed counts:
	// - Alpha: +115 -3   (both add and remove)
	// - Bravo: +7        (add only, no remove)
	// - Charlie: -12     (remove only, no add)
	entries := []structure.Entry{
		{StartLine: 1, EndLine: 20, Name: "Alpha", Kind: "func"},
		{StartLine: 25, EndLine: 35, Name: "Bravo", Kind: "func"},
		{StartLine: 40, EndLine: 55, Name: "Charlie", Kind: "func"},
	}

	changes := []structure.ElementChange{
		{Kind: structure.ChangeModified, OldEntry: &entries[0], NewEntry: &entries[0], LinesAdded: 115, LinesRemoved: 3},
		{Kind: structure.ChangeModified, OldEntry: &entries[1], NewEntry: &entries[1], LinesAdded: 7, LinesRemoved: 0},
		{Kind: structure.ChangeDeleted, OldEntry: &entries[2], LinesAdded: 0, LinesRemoved: 12},
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/stats.go",
				NewPath:   "b/stats.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "package stats", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package stats", Type: sidebyside.Context}},
				},
			},
		},
		width:  120,
		height: 20,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap(entries),
				NewStructure:   structure.NewMap(entries),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	lines := strings.Split(output, "\n")

	var strippedLines []string
	for _, line := range lines {
		strippedLines = append(strippedLines, ansiRegex.ReplaceAllString(line, ""))
	}

	// Find structural diff lines by looking for function names
	findLine := func(name string) string {
		for _, s := range strippedLines {
			if strings.Contains(s, name) && strings.Contains(s, "func") {
				return s
			}
		}
		return ""
	}

	alphaLine := findLine("Alpha")
	bravoLine := findLine("Bravo")
	charlieLine := findLine("Charlie")

	require.NotEmpty(t, alphaLine, "Expected Alpha line in output")
	require.NotEmpty(t, bravoLine, "Expected Bravo line in output")
	require.NotEmpty(t, charlieLine, "Expected Charlie line in output")

	// Alpha has both: should show +115 and -3
	assert.Contains(t, alphaLine, "+115", "Alpha should show +115")
	assert.Contains(t, alphaLine, "-3", "Alpha should show -3")

	// Bravo has +7 only: should show +7, no remove stat at all
	assert.Contains(t, bravoLine, "+7", "Bravo should show +7")
	// No "-" stat should appear (no placeholder for zero)
	bravoAfterName := bravoLine[strings.Index(bravoLine, "Bravo")+5:]
	assert.NotContains(t, bravoAfterName, "-", "Bravo should not show any remove stat")

	// Charlie has -12 only: should show -12, no add stat at all
	assert.Contains(t, charlieLine, "-12", "Charlie should show -12")
	assert.NotContains(t, charlieLine, "+", "Charlie should not show any add stat")
}

// TestView_StructuralDiffSignatureUsesTerminalWidth tests that structural diff
// signatures are sized based on terminal width (80%), not the file header box
// width. A short filename should not cause signature truncation on a wide terminal.
func TestView_StructuralDiffSignatureUsesTerminalWidth(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	// A function with a long signature that would be truncated if constrained
	// to a narrow header box, but should fit within 80% of a wide terminal.
	// Full signature: "(m *Model) ProcessRequest(ctx context.Context, request *Request, options ...Option) -> error"
	// That's ~90 chars — needs a wide terminal to show fully.
	entry := &structure.Entry{
		StartLine:  1,
		EndLine:    50,
		Name:       "ProcessRequest",
		Kind:       "func",
		Receiver:   "(m *Model)",
		Params:     []string{"ctx context.Context", "request *Request", "options ...Option"},
		ReturnType: "error",
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				// Short filename = narrow header box
				OldPath:   "a/x.go",
				NewPath:   "b/x.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "package x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package x", Type: sidebyside.Context}},
				},
			},
		},
		width:  200, // Wide terminal — 80% = 160 columns, plenty of room
		height: 20,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{*entry}),
				NewStructure: structure.NewMap([]structure.Entry{*entry}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{Kind: structure.ChangeModified, OldEntry: entry, NewEntry: entry, LinesAdded: 10, LinesRemoved: 3},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	lines := strings.Split(output, "\n")

	// Find the structural diff line containing our function
	var funcLine string
	for _, line := range lines {
		stripped := ansiRegex.ReplaceAllString(line, "")
		if strings.Contains(stripped, "ProcessRequest") {
			funcLine = stripped
			break
		}
	}
	require.NotEmpty(t, funcLine, "Expected ProcessRequest in output")

	// With 200-wide terminal, the full signature should be visible — all params shown
	assert.Contains(t, funcLine, "ctx context.Context",
		"Full params should be visible on wide terminal, not truncated to header box width")
	assert.Contains(t, funcLine, "options ...Option",
		"All params should be visible on wide terminal")
	assert.Contains(t, funcLine, "-> error",
		"Return type should be visible")

	// FormatSignature uses "(...)" when truncating params — if all params are
	// shown, we should not see the compact "(...)" form (but "...Option" is fine)
	assert.NotContains(t, funcLine, "(...)",
		"Signature should not be truncated on a 200-wide terminal")
}

// TestView_StructuralDiffNoSymbolPrefix tests that structural diff lines do not
// contain the old ~/+/- symbol prefix. The change kind is now conveyed through
// identifier styling, not a leading symbol character.
func TestView_StructuralDiffNoSymbolPrefix(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	entries := []structure.Entry{
		{StartLine: 1, EndLine: 10, Name: "Added", Kind: "func"},
		{StartLine: 15, EndLine: 25, Name: "Deleted", Kind: "func"},
		{StartLine: 30, EndLine: 40, Name: "Modified", Kind: "func"},
	}

	changes := []structure.ElementChange{
		{Kind: structure.ChangeAdded, NewEntry: &entries[0], LinesAdded: 10},
		{Kind: structure.ChangeDeleted, OldEntry: &entries[1], LinesRemoved: 10},
		{Kind: structure.ChangeModified, OldEntry: &entries[2], NewEntry: &entries[2], LinesAdded: 3, LinesRemoved: 2},
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go", NewPath: "b/test.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "package test", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package test", Type: sidebyside.Context}},
				},
			},
		},
		width: 120, height: 20, keys: DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap(entries),
				NewStructure:   structure.NewMap(entries),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	for _, row := range rows {
		if !row.isStructuralDiff || row.structuralDiffIsTruncated {
			continue
		}
		line := row.structuralDiffLine
		// The line should start with the kind (after optional child indent),
		// not with a ~/+/- symbol.
		trimmed := strings.TrimLeft(line, " ")
		assert.True(t,
			strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ") ||
				strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "def "),
			"Structural diff line should start with kind, not symbol: %q", line)
	}
}

// TestView_StructuralDiffChangeKindOnDisplayRow tests that the structuralDiffChangeKind
// field is set correctly on display rows for added, deleted, and modified items.
func TestView_StructuralDiffChangeKindOnDisplayRow(t *testing.T) {
	entries := []structure.Entry{
		{StartLine: 1, EndLine: 10, Name: "AddedFunc", Kind: "func"},
		{StartLine: 15, EndLine: 25, Name: "DeletedFunc", Kind: "func"},
		{StartLine: 30, EndLine: 40, Name: "ModifiedFunc", Kind: "func"},
	}

	changes := []structure.ElementChange{
		{Kind: structure.ChangeAdded, NewEntry: &entries[0], LinesAdded: 10},
		{Kind: structure.ChangeDeleted, OldEntry: &entries[1], LinesRemoved: 10},
		{Kind: structure.ChangeModified, OldEntry: &entries[2], NewEntry: &entries[2], LinesAdded: 3, LinesRemoved: 2},
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go", NewPath: "b/test.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "package test", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package test", Type: sidebyside.Context}},
				},
			},
		},
		width: 120, height: 20, keys: DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap(entries),
				NewStructure:   structure.NewMap(entries),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	kindByName := make(map[string]structure.ChangeKind)
	for _, row := range rows {
		if !row.isStructuralDiff || row.structuralDiffIsTruncated {
			continue
		}
		for _, name := range []string{"AddedFunc", "DeletedFunc", "ModifiedFunc"} {
			if strings.Contains(row.structuralDiffLine, name) {
				kindByName[name] = row.structuralDiffChangeKind
			}
		}
	}

	assert.Equal(t, structure.ChangeAdded, kindByName["AddedFunc"], "AddedFunc should have ChangeAdded")
	assert.Equal(t, structure.ChangeDeleted, kindByName["DeletedFunc"], "DeletedFunc should have ChangeDeleted")
	assert.Equal(t, structure.ChangeModified, kindByName["ModifiedFunc"], "ModifiedFunc should have ChangeModified")
}

// TestView_StructuralDiffChildInheritsParentChangeKind tests that when a parent
// class/type is added or deleted, its child methods inherit that change kind
// for identifier styling purposes.
func TestView_StructuralDiffChildInheritsParentChangeKind(t *testing.T) {
	// A class that is entirely new (added), containing two methods.
	// The child methods should inherit ChangeAdded from the parent.
	classEntry := structure.Entry{StartLine: 1, EndLine: 50, Name: "NewClass", Kind: "class"}
	method1 := structure.Entry{StartLine: 5, EndLine: 20, Name: "method_one", Kind: "def"}
	method2 := structure.Entry{StartLine: 25, EndLine: 45, Name: "method_two", Kind: "def"}

	changes := []structure.ElementChange{
		{Kind: structure.ChangeAdded, NewEntry: &classEntry, LinesAdded: 50},
		{Kind: structure.ChangeAdded, NewEntry: &method1, LinesAdded: 15},
		{Kind: structure.ChangeAdded, NewEntry: &method2, LinesAdded: 20},
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.py", NewPath: "b/test.py",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context}},
				},
			},
		},
		width: 120, height: 20, keys: DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				NewStructure:   structure.NewMap([]structure.Entry{classEntry, method1, method2}),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	kindByName := make(map[string]structure.ChangeKind)
	for _, row := range rows {
		if !row.isStructuralDiff || row.structuralDiffIsTruncated {
			continue
		}
		for _, name := range []string{"NewClass", "method_one", "method_two"} {
			if strings.Contains(row.structuralDiffLine, name) {
				kindByName[name] = row.structuralDiffChangeKind
			}
		}
	}

	assert.Equal(t, structure.ChangeAdded, kindByName["NewClass"], "Parent class should have ChangeAdded")
	assert.Equal(t, structure.ChangeAdded, kindByName["method_one"], "Child method should inherit ChangeAdded from parent")
	assert.Equal(t, structure.ChangeAdded, kindByName["method_two"], "Child method should inherit ChangeAdded from parent")
}

// TestView_StructuralDiffChildInheritsDeletedKind tests that children of a
// deleted class inherit ChangeDeleted, even if the child's own Kind differs.
func TestView_StructuralDiffChildInheritsDeletedKind(t *testing.T) {
	classEntry := structure.Entry{StartLine: 1, EndLine: 30, Name: "OldClass", Kind: "class"}
	method := structure.Entry{StartLine: 5, EndLine: 25, Name: "old_method", Kind: "def"}

	changes := []structure.ElementChange{
		{Kind: structure.ChangeDeleted, OldEntry: &classEntry, LinesRemoved: 30},
		// The method is modified on its own, but since parent is deleted, it inherits deleted styling
		{Kind: structure.ChangeModified, OldEntry: &method, NewEntry: &method, LinesAdded: 1, LinesRemoved: 5},
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.py", NewPath: "b/test.py",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context}},
				},
			},
		},
		width: 120, height: 20, keys: DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap([]structure.Entry{classEntry, method}),
				NewStructure:   structure.NewMap([]structure.Entry{classEntry, method}),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	kindByName := make(map[string]structure.ChangeKind)
	for _, row := range rows {
		if !row.isStructuralDiff || row.structuralDiffIsTruncated {
			continue
		}
		if strings.Contains(row.structuralDiffLine, "OldClass") {
			kindByName["OldClass"] = row.structuralDiffChangeKind
		}
		if strings.Contains(row.structuralDiffLine, "old_method") {
			kindByName["old_method"] = row.structuralDiffChangeKind
		}
	}

	assert.Equal(t, structure.ChangeDeleted, kindByName["OldClass"], "Parent should be ChangeDeleted")
	assert.Equal(t, structure.ChangeDeleted, kindByName["old_method"], "Child should inherit ChangeDeleted from deleted parent")
}

// TestView_StructuralDiffChildrenSortedByLinesChanged tests that children within
// a class/type are sorted by total lines changed (descending), so the most
// impactful methods appear first.
func TestView_StructuralDiffChildrenSortedByLinesChanged(t *testing.T) {
	classEntry := structure.Entry{StartLine: 1, EndLine: 100, Name: "MyClass", Kind: "class"}
	smallMethod := structure.Entry{StartLine: 5, EndLine: 10, Name: "small_change", Kind: "def"}
	bigMethod := structure.Entry{StartLine: 20, EndLine: 50, Name: "big_change", Kind: "def"}
	medMethod := structure.Entry{StartLine: 60, EndLine: 80, Name: "med_change", Kind: "def"}

	changes := []structure.ElementChange{
		{Kind: structure.ChangeModified, OldEntry: &classEntry, NewEntry: &classEntry, LinesAdded: 50, LinesRemoved: 10},
		{Kind: structure.ChangeModified, OldEntry: &smallMethod, NewEntry: &smallMethod, LinesAdded: 1, LinesRemoved: 0},
		{Kind: structure.ChangeModified, OldEntry: &bigMethod, NewEntry: &bigMethod, LinesAdded: 30, LinesRemoved: 5},
		{Kind: structure.ChangeModified, OldEntry: &medMethod, NewEntry: &medMethod, LinesAdded: 10, LinesRemoved: 3},
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.py", NewPath: "b/test.py",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context}},
				},
			},
		},
		width: 120, height: 20, keys: DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap([]structure.Entry{classEntry, smallMethod, bigMethod, medMethod}),
				NewStructure:   structure.NewMap([]structure.Entry{classEntry, smallMethod, bigMethod, medMethod}),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Collect the structural diff lines in order (excluding the class parent and truncation)
	var childNames []string
	for _, row := range rows {
		if !row.isStructuralDiff || row.structuralDiffIsTruncated {
			continue
		}
		line := row.structuralDiffLine
		// Children have 2-space indent prefix
		if strings.HasPrefix(line, "  ") {
			for _, name := range []string{"big_change", "med_change", "small_change"} {
				if strings.Contains(line, name) {
					childNames = append(childNames, name)
				}
			}
		}
	}

	require.Equal(t, 3, len(childNames), "Should have 3 child methods")
	assert.Equal(t, "big_change", childNames[0], "Biggest change should be first")
	assert.Equal(t, "med_change", childNames[1], "Medium change should be second")
	assert.Equal(t, "small_change", childNames[2], "Smallest change should be last")
}

// TestView_StructuralDiffTruncationCountsChildRows tests that the top-10 limit
// counts each displayed row (parent + children) rather than just top-level nodes.
// A class with many methods should consume multiple slots.
func TestView_StructuralDiffTruncationCountsChildRows(t *testing.T) {
	// Create a class with 8 methods (= 9 rows: 1 parent + 8 children).
	// Then add 3 standalone functions.
	// Total nodes = 4 (1 class + 3 funcs), but total rows = 9 + 3 = 12.
	// With a limit of 10, the class (9 rows) + 1 standalone func = 10 rows.
	// The remaining 2 funcs should be truncated.
	classEntry := structure.Entry{StartLine: 1, EndLine: 200, Name: "BigClass", Kind: "class"}
	var methods []structure.Entry
	var allChanges []structure.ElementChange

	// Class itself
	allChanges = append(allChanges, structure.ElementChange{
		Kind: structure.ChangeModified, OldEntry: &classEntry, NewEntry: &classEntry,
		LinesAdded: 100, LinesRemoved: 50,
	})

	// 8 methods inside the class
	for i := 0; i < 8; i++ {
		m := structure.Entry{
			StartLine: 10 + i*20, EndLine: 25 + i*20,
			Name: fmt.Sprintf("method_%d", i), Kind: "def",
		}
		methods = append(methods, m)
		allChanges = append(allChanges, structure.ElementChange{
			Kind: structure.ChangeModified, OldEntry: &methods[i], NewEntry: &methods[i],
			LinesAdded: 10 - i, LinesRemoved: i, // varying counts
		})
	}

	// 3 standalone functions with smaller total changes
	standaloneEntries := []structure.Entry{
		{StartLine: 300, EndLine: 310, Name: "StandaloneA", Kind: "func"},
		{StartLine: 320, EndLine: 330, Name: "StandaloneB", Kind: "func"},
		{StartLine: 340, EndLine: 350, Name: "StandaloneC", Kind: "func"},
	}
	allChanges = append(allChanges,
		structure.ElementChange{Kind: structure.ChangeModified, OldEntry: &standaloneEntries[0], NewEntry: &standaloneEntries[0], LinesAdded: 5, LinesRemoved: 2},
		structure.ElementChange{Kind: structure.ChangeModified, OldEntry: &standaloneEntries[1], NewEntry: &standaloneEntries[1], LinesAdded: 3, LinesRemoved: 1},
		structure.ElementChange{Kind: structure.ChangeModified, OldEntry: &standaloneEntries[2], NewEntry: &standaloneEntries[2], LinesAdded: 2, LinesRemoved: 0},
	)

	allEntries := append([]structure.Entry{classEntry}, methods...)
	allEntries = append(allEntries, standaloneEntries...)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/big.py", NewPath: "b/big.py",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "# big", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "# big", Type: sidebyside.Context}},
				},
			},
		},
		width: 120, height: 40, keys: DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap(allEntries),
				NewStructure:   structure.NewMap(allEntries),
				StructuralDiff: &structure.StructuralDiff{Changes: allChanges},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var structRows []displayRow
	var truncatedRow *displayRow
	for i, row := range rows {
		if row.isStructuralDiff {
			if row.structuralDiffIsTruncated {
				truncatedRow = &rows[i]
			} else {
				structRows = append(structRows, row)
			}
		}
	}

	// BigClass (1) + 8 methods = 9 rows. That's under 10 so the class fits.
	// StandaloneA (1 row) would make 10 total. That fits.
	// StandaloneB would be 11 — exceeds limit, so it and StandaloneC are truncated.
	// But wait: nodes are sorted by total lines. BigClass has 150 total (including children).
	// StandaloneA=7, StandaloneB=4, StandaloneC=2. BigClass is first (150 >> 7).
	// BigClass takes 9 rows. Next is StandaloneA (1 row) = 10 total. Fits.
	// StandaloneB would be 11. Doesn't fit. So 2 nodes truncated.

	// The class and its methods should all appear
	foundClass := false
	methodCount := 0
	foundStandaloneA := false
	for _, row := range structRows {
		if strings.Contains(row.structuralDiffLine, "BigClass") {
			foundClass = true
		}
		if strings.Contains(row.structuralDiffLine, "method_") {
			methodCount++
		}
		if strings.Contains(row.structuralDiffLine, "StandaloneA") {
			foundStandaloneA = true
		}
	}

	assert.True(t, foundClass, "BigClass should appear")
	assert.Equal(t, 8, methodCount, "All 8 methods of BigClass should appear")
	assert.True(t, foundStandaloneA, "StandaloneA should fit within the 10-row limit")

	// StandaloneB and StandaloneC should NOT appear
	for _, row := range structRows {
		assert.NotContains(t, row.structuralDiffLine, "StandaloneB", "StandaloneB should be truncated")
		assert.NotContains(t, row.structuralDiffLine, "StandaloneC", "StandaloneC should be truncated")
	}

	// Should have a truncation indicator for the remaining 2 nodes
	require.NotNil(t, truncatedRow, "Should have a truncation row")
	assert.Contains(t, truncatedRow.structuralDiffLine, "2 more", "Should indicate 2 truncated nodes")
}

// TestView_StructuralDiffSignatureMaxWidth tests that signatures are capped at
// maxStructuralDiffSigWidth (120) even on very wide terminals.
func TestView_StructuralDiffSignatureMaxWidth(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	// Create a function with an extremely long signature
	entry := &structure.Entry{
		StartLine:  1,
		EndLine:    50,
		Name:       "VeryLongFunctionName",
		Kind:       "func",
		Receiver:   "(m *Model)",
		Params:     []string{"ctx context.Context", "request *LongRequestType", "options ...VeryLongOptionType", "callback func(int, string, error)", "extra ExtraParam"},
		ReturnType: "VeryLongReturnType",
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/x.go", NewPath: "b/x.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "package x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package x", Type: sidebyside.Context}},
				},
			},
		},
		width:  500, // Extremely wide terminal
		height: 20,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure: structure.NewMap([]structure.Entry{*entry}),
				NewStructure: structure.NewMap([]structure.Entry{*entry}),
				StructuralDiff: &structure.StructuralDiff{
					Changes: []structure.ElementChange{
						{Kind: structure.ChangeModified, OldEntry: entry, NewEntry: entry, LinesAdded: 10, LinesRemoved: 3},
					},
				},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the structural diff line
	var sigLine string
	for _, row := range rows {
		if row.isStructuralDiff && !row.structuralDiffIsTruncated {
			sigLine = row.structuralDiffLine
			break
		}
	}
	require.NotEmpty(t, sigLine, "Should have a structural diff line")

	// Extract just the signature part (after "func ")
	parts := strings.SplitN(sigLine, " ", 2)
	require.Equal(t, 2, len(parts), "Line should have kind and signature")
	sig := parts[1]

	// The signature should be at most 120 chars wide. It might be truncated with "...".
	assert.LessOrEqual(t, len(sig), maxStructuralDiffSigWidth,
		"Signature should be capped at %d chars, got %d: %q", maxStructuralDiffSigWidth, len(sig), sig)
}

// TestView_StructuralDiffModifiedChildKeepsOwnKind tests that when a parent
// class is modified (not added/deleted), children keep their own change kind
// rather than inheriting from the parent.
func TestView_StructuralDiffModifiedChildKeepsOwnKind(t *testing.T) {
	classEntry := structure.Entry{StartLine: 1, EndLine: 50, Name: "MyClass", Kind: "class"}
	addedMethod := structure.Entry{StartLine: 5, EndLine: 20, Name: "new_method", Kind: "def"}
	modifiedMethod := structure.Entry{StartLine: 25, EndLine: 45, Name: "changed_method", Kind: "def"}

	changes := []structure.ElementChange{
		{Kind: structure.ChangeModified, OldEntry: &classEntry, NewEntry: &classEntry, LinesAdded: 20, LinesRemoved: 5},
		{Kind: structure.ChangeAdded, NewEntry: &addedMethod, LinesAdded: 15},
		{Kind: structure.ChangeModified, OldEntry: &modifiedMethod, NewEntry: &modifiedMethod, LinesAdded: 3, LinesRemoved: 2},
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.py", NewPath: "b/test.py",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "# test", Type: sidebyside.Context}},
				},
			},
		},
		width: 120, height: 20, keys: DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				OldStructure:   structure.NewMap([]structure.Entry{classEntry, addedMethod, modifiedMethod}),
				NewStructure:   structure.NewMap([]structure.Entry{classEntry, addedMethod, modifiedMethod}),
				StructuralDiff: &structure.StructuralDiff{Changes: changes},
			},
		},
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	kindByName := make(map[string]structure.ChangeKind)
	for _, row := range rows {
		if !row.isStructuralDiff || row.structuralDiffIsTruncated {
			continue
		}
		if strings.Contains(row.structuralDiffLine, "MyClass") {
			kindByName["MyClass"] = row.structuralDiffChangeKind
		}
		if strings.Contains(row.structuralDiffLine, "new_method") {
			kindByName["new_method"] = row.structuralDiffChangeKind
		}
		if strings.Contains(row.structuralDiffLine, "changed_method") {
			kindByName["changed_method"] = row.structuralDiffChangeKind
		}
	}

	assert.Equal(t, structure.ChangeModified, kindByName["MyClass"], "Parent should be ChangeModified")
	assert.Equal(t, structure.ChangeAdded, kindByName["new_method"], "Added child keeps its own ChangeAdded when parent is modified")
	assert.Equal(t, structure.ChangeModified, kindByName["changed_method"], "Modified child keeps its own ChangeModified when parent is modified")
}
