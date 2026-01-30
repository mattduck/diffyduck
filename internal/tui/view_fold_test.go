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
			m.scroll = m.minScroll() // Position cursor at top so header is visible

			output := m.View()
			lines := strings.Split(output, "\n")

			// Layout at minScroll: [topBar, divider, padding, header, ...]
			// lines[3] = header (at cursorOffset position)
			headerLine := lines[3]
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
	m.scroll = m.minScroll() // Position cursor at top so header is visible

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout at minScroll: [topBar, divider, padding, header, ...]
	// lines[3] = header (at cursorOffset position)
	// Folded view should only show the header and then padding
	assert.Contains(t, lines[3], "foo.go", "first content line should be the header")
	assert.Contains(t, lines[3], "○", "header should have folded icon")

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

	// In tree layout:
	// Normal file (first): 1 header + 1 bottom border + 10 pairs + 1 blank margin + 1 top border (next file) = 14 lines
	// Folded file (second): 1 header only
	// Total: 14 + 1 = 15 lines
	assert.Equal(t, 15, m.totalLines, "totalLines should account for fold states")
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

func TestCommitHeader_ExpandedShowsFullFillIcon(t *testing.T) {
	// When a commit is at visibility level 3 (after pressing tab twice),
	// its header should show the full-fill icon ● instead of half-fill ◐
	//
	// This test simulates pressing Tab twice on a commit header to get to level 3,
	// then verifies the fold icon is correctly updated to the full-fill style.
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
				FoldLevel: sidebyside.FoldFolded,
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

	// Tab 1: Level 1 (Folded) -> Level 2 (Normal, files folded)
	newM, _ := m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, 2, m.commitVisibilityLevelFor(0), "after first Tab, should be at level 2")

	// Tab 2: Level 2 -> Level 3 (files expanded)
	newM, _ = m.handleCommitFoldCycle()
	m = newM.(Model)
	assert.Equal(t, 3, m.commitVisibilityLevelFor(0), "after second Tab, should be at level 3")

	// At level 3, the commit's FoldLevel should be CommitExpanded
	assert.Equal(t, sidebyside.CommitExpanded, m.commits[0].FoldLevel,
		"at level 3, commit.FoldLevel should be CommitExpanded")

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the commit header line (contains the SHA)
	var commitHeaderLine string
	for _, line := range lines {
		if strings.Contains(line, "abc1234") {
			commitHeaderLine = line
			break
		}
	}

	require.NotEmpty(t, commitHeaderLine, "should find commit header with SHA")

	// At level 3, should show full-fill icon ●, not half-fill ◐
	assert.Contains(t, commitHeaderLine, "●",
		"commit header at level 3 should show full-fill icon ●")
	assert.NotContains(t, commitHeaderLine, "◐",
		"commit header at level 3 should NOT show half-fill icon ◐")
}

func TestCommitHeader_ExpandingFileUpdatesCommitToLevel3(t *testing.T) {
	// When a file is expanded beyond just its header (FoldFolded),
	// the visibility level becomes 3, but the commit stays at CommitNormal.
	// This allows file expansion without automatically expanding commit info.
	// Level 2 means "file headings only" - if any file shows content,
	// the visibility level is 3 but commit fold level stays CommitNormal.
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
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
				},
			},
		},
		FoldLevel:   sidebyside.CommitNormal, // Start at level 2 (file headers visible)
		FilesLoaded: true,
	}

	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	// Verify we're at level 2
	assert.Equal(t, 2, m.commitVisibilityLevelFor(0), "should start at level 2")
	assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel, "commit should be CommitNormal")
	assert.Equal(t, sidebyside.FoldFolded, m.files[0].FoldLevel, "file should be FoldFolded")

	// Navigate to the file and expand it
	// First, move cursor to be on the file (not the commit header)
	m.scroll = m.minScroll()
	rows := m.buildRows()
	for i, row := range rows {
		if row.isHeader && row.fileIndex == 0 {
			// Position cursor on this file header
			m.scroll = i
			break
		}
	}

	// Expand the file using handleFoldToggle (simulates pressing Tab on a file)
	newM, _ := m.handleFoldToggle()
	m = newM.(Model)

	// File should now be expanded
	assert.Equal(t, sidebyside.FoldNormal, m.files[0].FoldLevel,
		"file should be FoldNormal after toggle")

	// The commit should now be at level 3 but still CommitNormal
	// (file expansion doesn't force commit info to expand)
	assert.Equal(t, 3, m.commitVisibilityLevelFor(0),
		"commit visibility should be level 3 after file expansion")
	assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel,
		"commit.FoldLevel should stay CommitNormal when a file is expanded")

	// Verify the commit header shows the half-fill icon (◐) since commit is CommitNormal
	output := m.View()
	lines := strings.Split(output, "\n")

	var commitHeaderLine string
	for _, line := range lines {
		if strings.Contains(line, "def5678") {
			commitHeaderLine = line
			break
		}
	}

	require.NotEmpty(t, commitHeaderLine, "should find commit header with SHA")
	assert.Contains(t, commitHeaderLine, "◐",
		"commit header should show half-fill icon ◐ when commit is CommitNormal")
}

func TestCommitBorder_CursorRendersArrowOnEmptyLine(t *testing.T) {
	// When cursor is on a commit bottom border line, it should render:
	// - Arrow (▶)
	// - Space
	// - Highlighted gutter space (cursor indicator)
	//
	// The border line itself is empty (no border characters).
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
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
				},
			},
		},
		FoldLevel:   sidebyside.CommitNormal, // Unfolded so borders are visible
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
	m.scroll = bottomBorderRowIdx

	// Render and check the output
	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the line with the cursor arrow (should be the empty border line)
	var cursorLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "▶") {
			cursorLine = line
			break
		}
	}

	require.NotEmpty(t, cursorLine, "should find line with cursor arrow")

	// Should start with arrow followed by space (no border characters)
	assert.True(t, strings.HasPrefix(cursorLine, "▶ "),
		"commit bottom border with cursor should be: arrow + space, got: %s", cursorLine)
}
