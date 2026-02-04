package tui

import (
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

func TestView_GutterIndicators(t *testing.T) {
	// Test that +/- indicators appear in the gutter for added/removed lines
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
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
						OldPath:   "a/test.go",
						NewPath:   "b/test.go",
						FoldLevel: sidebyside.FoldExpanded,
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
			// Position cursor on row 3 (second content line in diff view layout: header=0, spacer=1, first content=2, second content=3)
			// so we can test line 1's indicator without cursor arrow
			// cursorLine = scroll + cursorOffset, so scroll = row - cursorOffset = 3 - cursorOffset
			m.w().scroll = 3

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
						OldPath:   "a/test.go",
						NewPath:   "b/test.go",
						FoldLevel: sidebyside.FoldExpanded,
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
				OldPath:   "a/large.go",
				NewPath:   "b/large.go",
				FoldLevel: sidebyside.FoldExpanded,
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
				OldPath:   "a/large.go",
				NewPath:   "b/large.go",
				FoldLevel: sidebyside.FoldExpanded,
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
	// Position scroll so content lines are visible
	// In diff view layout: header=0, spacer=1, separator (3 lines)=2-4, content=5+
	// Position cursor on content row
	m.w().scroll = 5

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find lines containing the test content
	var line1, line2, line3 string
	for _, line := range lines {
		if strings.Contains(line, "line before") {
			line1 = line
		} else if strings.Contains(line, "ten thousand") {
			line2 = line
		} else if strings.Contains(line, "line after") {
			line3 = line
		}
	}

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
				FoldLevel: sidebyside.FoldExpanded,
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
				FoldLevel: sidebyside.FoldExpanded,
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
