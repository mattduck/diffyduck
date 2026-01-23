package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

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
			// Header format is: <foldIcon> <fileNum> <statusIndicator> filename [stats]
			// For modified files (a/test.go -> b/test.go with same name), status is "~"
			assert.Contains(t, headerLine, tt.wantIcon+" #1 ~ test.go", "header format should be: <icon> <#fileNum> <status> filename")
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
	// Total should be 18 + 1 = 19
	assert.Equal(t, 19, m.totalLines, "totalLines should account for fold states")
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
