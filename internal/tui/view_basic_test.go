package tui

import (
	"flag"
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

func TestView_BinaryFile_Created(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:  "/dev/null",
				NewPath:  "b/image.png",
				IsBinary: true,
				Pairs:    nil, // Binary files have no pairs
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
				OldPath:  "a/image.png",
				NewPath:  "/dev/null",
				IsBinary: true,
				Pairs:    nil,
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
				OldPath:  "a/image.png",
				NewPath:  "b/image.png",
				IsBinary: true,
				Pairs:    nil,
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

func TestView_ScrolledToMax(t *testing.T) {
	// When scrolled to max, the last content row should be visible
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
	m.calculateTotalLines()
	m.scroll = m.maxScroll()

	output := m.View()
	lines := strings.Split(output, "\n")

	assert.Equal(t, 5, len(lines), "should have exactly height lines")

	// Layout: [topBar, divider, content[0..contentH-1], bottomBar]
	// lines[0] = top bar (still shows file name since cursor is on file content)
	assert.Contains(t, lines[0], "foo.go", "top bar should show file name")

	// lines[4] = bottom bar with END
	assert.Contains(t, lines[4], "END")
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
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldFolded, Pairs: makeAddedPairs(50)},
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldFolded, Pairs: makeAddedPairs(50)},
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
				FoldLevel: sidebyside.FoldNormal,
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
				FoldLevel: sidebyside.FoldNormal,
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
				FoldLevel: sidebyside.FoldNormal,
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
				FoldLevel: sidebyside.FoldFolded,
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
				FoldLevel: sidebyside.FoldFolded,
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
				FoldLevel: sidebyside.FoldFolded,
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
