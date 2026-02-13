package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestFoldLevelIcon(t *testing.T) {
	m := Model{}
	tests := []struct {
		level    sidebyside.FoldLevel
		expected string
	}{
		{sidebyside.FoldHeader, "○"},
		{sidebyside.FoldStructure, "◐"},
		{sidebyside.FoldHunks, "●"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
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
		{"folded shows empty circle", sidebyside.FoldHeader, "○"},
		{"normal shows half circle", sidebyside.FoldStructure, "◐"},
		{"expanded shows full circle", sidebyside.FoldHunks, "●"},
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
			m.w().scroll = m.minScroll() // Position cursor at top so header is visible

			output := m.View()
			lines := strings.Split(output, "\n")

			// Layout at minScroll: [topBar(3 lines), divider, padding, header, ...]
			// lines[5] = header (at cursorOffset position)
			headerLine := lines[5]
			assert.Contains(t, headerLine, tt.wantIcon, "header should contain %s icon for %s level", tt.wantIcon, tt.level)
			// Header format is: <foldIcon> filename (no file number or status symbol in main view)
			assert.Contains(t, headerLine, tt.wantIcon+" test.go", "header format should be: <icon> filename")
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
				FoldLevel: sidebyside.FoldHeader,
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
	m.w().scroll = m.minScroll() // Position cursor at top so header is visible

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout at minScroll: [topBar(3 lines), divider, padding, header, ...]
	// lines[5] = header (at cursorOffset position)
	// Folded view should only show the header and then padding
	assert.Contains(t, lines[5], "foo.go", "first content line should be the header")
	assert.Contains(t, lines[5], "○", "header should have folded icon")

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
				FoldLevel: sidebyside.FoldHeader, // First file is folded
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
				FoldLevel: sidebyside.FoldHunks, // Second file is normal
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
		if strings.Contains(line, "second.go") && strings.Contains(line, "●") {
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
				FoldLevel: sidebyside.FoldHunks,
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
				FoldLevel: sidebyside.FoldHeader,
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
				FoldLevel: sidebyside.FoldHunks,
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
				FoldLevel: sidebyside.FoldHunks,
				Pairs:     make([]sidebyside.LinePair, 10),
			},
			{
				OldPath:   "a/folded.go",
				NewPath:   "b/folded.go",
				FoldLevel: sidebyside.FoldHeader,
				Pairs:     make([]sidebyside.LinePair, 10),
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// In tree layout:
	// Normal file (first): 1 header + 1 bottom border + 10 pairs + 1 blank margin + 1 top border (next file) = 14 lines
	// Folded file (second): 1 header + 1 terminator row
	// Total: 14 + 2 = 16 lines
	assert.Equal(t, 16, m.w().totalLines, "totalLines should account for fold states")
}

func TestView_ExpandedFile_ShowsFullContent(t *testing.T) {
	// Full-file view (ShowFullFile) should show ALL lines from the full file content
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/foo.go",
				NewPath:      "b/foo.go",
				FoldLevel:    sidebyside.FoldHunks,
				ShowFullFile: true,
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

	assert.Contains(t, output, "line one", "should show line 1 from full content")
	assert.Contains(t, output, "line two", "should show line 2 from full content")
	assert.Contains(t, output, "line three", "should show line 3 from full content")
	assert.Contains(t, output, "line four", "should show line 4 from full content")
	assert.Contains(t, output, "line eight", "should show line 8 from full content")
	assert.Contains(t, output, "line nine", "should show line 9 from full content")
	assert.Contains(t, output, "line ten", "should show line 10 from full content")

	assert.Contains(t, output, "line five")
	assert.Contains(t, output, "old line six")
	assert.Contains(t, output, "new line six")
	assert.Contains(t, output, "line seven")
}

func TestView_ExpandedFile_DeletedFile(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/deleted.go",
				NewPath:      "/dev/null",
				FoldLevel:    sidebyside.FoldHunks,
				ShowFullFile: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "deleted line", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
				OldContent: []string{"deleted line", "another deleted"},
				NewContent: nil,
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	assert.Contains(t, output, "deleted line")
	assert.Contains(t, output, "another deleted")
}

func TestView_ExpandedFile_NewFile(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "/dev/null",
				NewPath:      "b/new.go",
				FoldLevel:    sidebyside.FoldHunks,
				ShowFullFile: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new line", Type: sidebyside.Added},
					},
				},
				OldContent: nil,
				NewContent: []string{"new line", "another new"},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	assert.Contains(t, output, "new line")
	assert.Contains(t, output, "another new")
}

func TestView_ExpandedFile_NoContent_FallsBackToNormal(t *testing.T) {
	// If expanded but content not loaded yet, fall back to normal view
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldHunks,
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
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should show the diff pairs since content isn't loaded
	assert.Contains(t, output, "diff context")
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
				FoldLevel: sidebyside.FoldHunks,
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
				FoldLevel: sidebyside.FoldHunks,
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

func TestCommitHeader_ExpandedShowsFullFillIcon(t *testing.T) {
	// When a commit is expanded past CommitFolded, the header icon becomes ●.
	// This test cycles through all 4 commit fold levels and checks the icon.
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc1234",
			Author:  "Test Author",
			Subject: "Test commit subject",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldHeader,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
				},
			},
		},
		FoldLevel:   sidebyside.CommitFolded, // Start folded
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	// Tab 1: CommitFolded → CommitFileHeaders
	newM, _ := m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, sidebyside.CommitFileHeaders, m.commitFoldLevel(0))

	// Tab 2: CommitFileHeaders → CommitFileStructure
	newM, _ = m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, sidebyside.CommitFileStructure, m.commitFoldLevel(0))

	// Tab 3: CommitFileStructure → CommitFileHunks
	newM, _ = m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, sidebyside.CommitFileHunks, m.commitFoldLevel(0))

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the commit header line in the content area (skip top bar lines).
	// Content area commit headers have fold icons; the top bar does not.
	var commitHeaderLine string
	for i, line := range lines {
		// Skip top bar area (first few lines before content)
		if i < 4 {
			continue
		}
		if strings.Contains(line, "abc1234") {
			commitHeaderLine = line
			break
		}
	}

	require.NotEmpty(t, commitHeaderLine, "should find commit header with SHA in content area")

	// Any non-folded commit shows ════╗ border
	assert.Contains(t, commitHeaderLine, "════╗",
		"commit header at CommitFileHunks should show ════╗ border")
}

func TestCommitHeader_ExpandingFileKeepsCommitFoldLevel(t *testing.T) {
	// Expanding a file individually via Tab doesn't change the commit fold level.
	// The commit stays at CommitFileHeaders even when a file is at FoldHunks.
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "def5678",
			Author:  "Test Author",
			Subject: "Test commit",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldStructure,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
				},
			},
		},
		FoldLevel:   sidebyside.CommitFileHeaders,
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	assert.Equal(t, sidebyside.CommitFileHeaders, m.commitFoldLevel(0))
	assert.Equal(t, sidebyside.FoldStructure, m.fileFoldLevel(0))

	// Navigate to the file header
	m.w().scroll = m.minScroll()
	rows := m.buildRows()
	for i, row := range rows {
		if row.isHeader && row.fileIndex == 0 {
			m.w().scroll = i
			break
		}
	}

	// Expand the file using handleFoldToggle (Tab on a file)
	newM, _ := m.handleFoldToggle()
	m = newM.(Model)

	// File should now be at FoldHunks
	assert.Equal(t, sidebyside.FoldHunks, m.fileFoldLevel(0),
		"file should be FoldHunks after toggle")

	// Commit fold level stays at CommitFileHeaders
	assert.Equal(t, sidebyside.CommitFileHeaders, m.commitFoldLevel(0),
		"commit fold level should stay CommitFileHeaders when a file is expanded individually")
}

func TestCommitBorder_CursorRendersArrowOnBorderLine(t *testing.T) {
	// When cursor is on a commit bottom border line, it should render
	// the arrow followed by the border characters.
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc1234",
			Author:  "Test Author",
			Subject: "Test commit",
		},
		Files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldHeader,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
				},
			},
		},
		FoldLevel:   sidebyside.CommitFileHeaders, // Unfolded so borders are visible
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	// Find the bottom border row and position cursor on it
	rows := m.buildRows()
	var bottomBorderRowIdx int = -1
	for i, row := range rows {
		if row.isCommitHeaderBottomBorder {
			bottomBorderRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, bottomBorderRowIdx, "should find commit bottom border row")

	// Position cursor on the border row
	m.w().scroll = bottomBorderRowIdx

	// Render and check the output
	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the line with the cursor arrow in the content area
	var cursorLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "▌") {
			cursorLine = line
			break
		}
	}

	require.NotEmpty(t, cursorLine, "should find content line with cursor arrow")
	// Arrow should be present at the start of the border line
	assert.True(t, strings.HasPrefix(cursorLine, "▌"),
		"commit bottom border with cursor should start with arrow, got: %s", cursorLine)
}
