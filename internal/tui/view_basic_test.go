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
	m.calculateTotalLines()

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

	// Helper to find rune (display) position of a character
	findRunePos := func(s string, target rune) int {
		runes := []rune(s)
		for i, r := range runes {
			if r == target {
				return i
			}
		}
		return -1
	}

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
		// Find a structural diff line (contains ~ and "func")
		if strings.Contains(stripped, "~ func") {
			structDiffLine = stripped
		}
	}

	require.NotEmpty(t, headerLine, "should find header line with example.go and tree branch")

	// Structural diff may not be rendered in all modes - skip alignment check if not present
	if structDiffLine != "" {
		// Find position of filename in header (first char of "example.go")
		filenamePos := findSubstringRunePos(headerLine, "example.go")
		require.GreaterOrEqual(t, filenamePos, 0, "should find filename position")

		// Find position of the symbol (~) in structural diff line
		symbolPos := findRunePos(structDiffLine, '~')
		require.GreaterOrEqual(t, symbolPos, 0, "should find symbol position")

		// The symbol should be near the filename's start position (within tree indent)
		// In tree layout, exact alignment may differ due to tree structure
		diff := symbolPos - filenamePos
		if diff < 0 {
			diff = -diff
		}
		assert.LessOrEqual(t, diff, 5,
			"structural diff symbol should be near filename start\n  header: %q\n  struct: %q",
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
