package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

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

	// Top bar should contain fold icon and file counter
	assert.Contains(t, topBar, "◐") // fold icon for normal level
	assert.Contains(t, topBar, "1") // file counter (no # prefix)
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

func TestStatusInfo_BreadcrumbsOnChunkSeparator(t *testing.T) {
	// When cursor is on chunk separator (middle) or separator bottom rows,
	// StatusInfo should show breadcrumbs for that chunk's first line.
	// When cursor is on separator top (above the breadcrumb line), no breadcrumb should appear.
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
					// Gap creates hunk separator - next chunk is inside MyFunction (lines 10-50)
					{
						Old: sidebyside.Line{Num: 20, Content: "    code", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 20, Content: "    code", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 30,
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

	// Find separator rows
	rows := m.buildRows()
	var sepTopIdx, sepIdx, sepBottomIdx int
	for i, row := range rows {
		if row.isSeparatorTop {
			sepTopIdx = i
		}
		if row.isSeparator {
			sepIdx = i
		}
		if row.isSeparatorBottom {
			sepBottomIdx = i
		}
	}
	require.NotZero(t, sepIdx, "should find hunk separator row")

	// Test 1: Cursor on separator top - no breadcrumb
	m.scroll = sepTopIdx
	info := m.StatusInfo()
	assert.Empty(t, info.Breadcrumbs, "cursor on separator top should NOT show breadcrumb")

	// Test 2: Cursor on separator (middle/breadcrumb line) - should show breadcrumb
	m.scroll = sepIdx
	info = m.StatusInfo()
	assert.Contains(t, info.Breadcrumbs, "func MyFunction", "cursor on separator should show breadcrumb")

	// Test 3: Cursor on separator bottom - should show breadcrumb
	m.scroll = sepBottomIdx
	info = m.StatusInfo()
	assert.Contains(t, info.Breadcrumbs, "func MyFunction", "cursor on separator bottom should show breadcrumb")
}

func TestTopBar_WithCommitInfo(t *testing.T) {
	// Create model with commit info using NewWithCommits
	files := []sidebyside.FilePair{
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
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890",
			Author:  "Test Author",
			Email:   "test@example.com",
			Date:    "2024-01-15T10:30:00+00:00",
			Subject: "Fix the bug in parser",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	// Move cursor to a file row by adjusting scroll
	// Commit body rows: blank + SHA + Author + Date + blank + Subject + trailing blank = 7 rows
	// Plus commit header = 8 rows before files (row 8 is first file row, 0-indexed)
	// cursorLine = scroll + cursorOffset, where cursorOffset ≈ contentHeight * 0.2
	// For height=20, contentHeight ≈ 17, cursorOffset ≈ 3
	// To get cursor on row 8: scroll = 8 - 3 = 5
	m.scroll = 10 // Set high enough to be past commit body rows

	topBar := m.renderTopBar()
	lines := strings.Split(topBar, "\n")

	// Should have 3 lines: commit line, file line, divider
	require.GreaterOrEqual(t, len(lines), 3, "top bar should have commit line, file line, and divider")

	// First line should contain commit info (SHA + subject only, no author/date)
	commitLine := lines[0]
	assert.Contains(t, commitLine, "abc123d", "commit line should contain short SHA")
	assert.Contains(t, commitLine, "Fix the bug", "commit line should contain subject")
	assert.NotContains(t, commitLine, "Test Author", "commit line should NOT contain author")
	// Stats should be on commit line
	assert.Contains(t, commitLine, "1 file", "commit line should contain file stats")
	assert.Contains(t, commitLine, "+1", "commit line should contain added stats")
	assert.Contains(t, commitLine, "-1", "commit line should contain removed stats")

	// Second line should contain file info but NOT stats
	fileLine := lines[1]
	assert.Contains(t, fileLine, "foo.go", "file line should contain filename")
	assert.NotContains(t, fileLine, "1 file", "file line should NOT contain stats when commit info present")
}

func TestTopBar_WithoutCommitInfo(t *testing.T) {
	// Create model without commit info using New()
	files := []sidebyside.FilePair{
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
	}
	m := New(files)
	m.width = 80
	m.height = 20
	m.focused = true

	topBar := m.renderTopBar()
	lines := strings.Split(topBar, "\n")

	// Should have 2 lines: file line and divider (no commit line)
	require.Equal(t, 2, len(lines), "top bar without commit info should have file line and divider only")

	// First line should contain file info (not commit info)
	fileLine := lines[0]
	assert.Contains(t, fileLine, "foo.go", "should contain filename")
	assert.NotContains(t, fileLine, "abc123", "should NOT contain SHA")
	// Stats should be on file line when no commit info
	assert.Contains(t, fileLine, "1 file", "file line should contain stats when no commit info")
	assert.Contains(t, fileLine, "+1", "file line should contain added stats when no commit info")
	assert.Contains(t, fileLine, "-1", "file line should contain removed stats when no commit info")
}

func TestFormatRelativeDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // what the output should contain
	}{
		{
			name:     "empty string",
			input:    "",
			contains: "",
		},
		{
			name:     "invalid format returns as-is",
			input:    "not a date",
			contains: "not a date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeDate(tt.input)
			if tt.contains == "" {
				assert.Empty(t, result)
			} else {
				assert.Contains(t, result, tt.contains)
			}
		})
	}
}

func TestContentHeight_WithCommitInfo(t *testing.T) {
	// Without commit info
	m1 := New([]sidebyside.FilePair{{OldPath: "a/foo.go", NewPath: "b/foo.go"}})
	m1.height = 20

	// With commit info (unfolded - shows both commit and file lines)
	commit := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc123", Author: "Test"},
		Files:       []sidebyside.FilePair{{OldPath: "a/foo.go", NewPath: "b/foo.go"}},
		FoldLevel:   sidebyside.CommitNormal, // Important: unfolded state
		FilesLoaded: true,
	}
	m2 := NewWithCommits([]sidebyside.CommitSet{commit})
	m2.height = 20

	// Content height should be 1 less when commit info is present (extra line for commit info)
	assert.Equal(t, m1.contentHeight()-1, m2.contentHeight(),
		"content height should be 1 less when commit info is present")
}

func TestContentHeight_CommitFolded(t *testing.T) {
	// With commit info but folded - top bar still shows
	commit := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc123", Author: "Test"},
		Files:       []sidebyside.FilePair{{OldPath: "a/foo.go", NewPath: "b/foo.go"}},
		FoldLevel:   sidebyside.CommitFolded,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.height = 20

	// Top bar always shows: commit line + file line + divider + bottom bar = 4 reserved
	// Content height = 20 - 4 = 16
	assert.Equal(t, 16, m.contentHeight(),
		"folded commit should still reserve 4 lines (commit + file + divider + bottom)")
}

func TestCommitHeaderRow(t *testing.T) {
	// Create model with commit info
	files := []sidebyside.FilePair{
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
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890",
			Author:  "Test Author",
			Subject: "Fix the bug",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	// Build rows and check first row is commit header
	rows := m.buildRows()
	require.NotEmpty(t, rows, "should have rows")
	assert.True(t, rows[0].isCommitHeader, "first row should be commit header")
	assert.Equal(t, RowKindCommitHeader, rows[0].kind, "first row kind should be RowKindCommitHeader")

	// Check that file rows come after commit header
	foundFileHeader := false
	for _, row := range rows[1:] {
		if row.isHeader && !row.isCommitHeader {
			foundFileHeader = true
			break
		}
	}
	assert.True(t, foundFileHeader, "should have file header after commit header")
}

func TestCommitFolding(t *testing.T) {
	// Create model with commit info
	files := []sidebyside.FilePair{
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
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890",
			Author:  "Test Author",
			Subject: "Fix the bug",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal, // Start unfolded
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	// Normal (unfolded) state should have multiple rows
	rowsUnfolded := m.buildRows()
	assert.Greater(t, len(rowsUnfolded), 1, "unfolded commit should have more than 1 row")

	// Fold the commit
	m.commits[0].FoldLevel = sidebyside.CommitFolded
	m.calculateTotalLines()

	// Folded state should have only 1 row (the commit header)
	rowsFolded := m.buildRows()
	assert.Equal(t, 1, len(rowsFolded), "folded commit should have exactly 1 row")
	assert.True(t, rowsFolded[0].isCommitHeader, "the only row should be commit header")
}

func TestCommitFoldCycle(t *testing.T) {
	// Create model with commit info and multiple files
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/foo.go",
			NewPath:   "b/foo.go",
			FoldLevel: sidebyside.FoldFolded,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
			},
		},
		{
			OldPath:   "a/bar.go",
			NewPath:   "b/bar.go",
			FoldLevel: sidebyside.FoldFolded,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
			},
		},
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890",
			Author:  "Test Author",
			Subject: "Fix the bug",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitFolded, // Start at Level 1
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	// Start at Level 1 (folded)
	assert.Equal(t, 1, m.commitVisibilityLevel(), "should start at level 1")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel)

	// Cycle to Level 2 (file headings only)
	m.handleCommitFoldCycle()
	assert.Equal(t, 2, m.commitVisibilityLevel(), "should be at level 2 after first cycle")
	assert.Equal(t, sidebyside.CommitNormal, m.commits[0].FoldLevel)
	for _, f := range m.files {
		assert.Equal(t, sidebyside.FoldFolded, f.FoldLevel, "all files should be FoldFolded at level 2")
	}

	// Cycle to Level 3 (file hunks visible)
	m.handleCommitFoldCycle()
	assert.Equal(t, 3, m.commitVisibilityLevel(), "should be at level 3 after second cycle")
	for _, f := range m.files {
		assert.Equal(t, sidebyside.FoldNormal, f.FoldLevel, "all files should be FoldNormal at level 3")
	}

	// Cycle back to Level 1 (folded)
	m.handleCommitFoldCycle()
	assert.Equal(t, 1, m.commitVisibilityLevel(), "should be back at level 1 after third cycle")
	assert.Equal(t, sidebyside.CommitFolded, m.commits[0].FoldLevel)
}

func TestCommitFoldCycleWithMixedFiles(t *testing.T) {
	// Test that if any file is expanded, we're at level 3
	files := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldFolded},
		{OldPath: "a/bar.go", NewPath: "b/bar.go", FoldLevel: sidebyside.FoldNormal}, // One expanded
	}
	commit := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc123"},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 40
	m.calculateTotalLines()

	// Should be at level 3 because one file is expanded
	assert.Equal(t, 3, m.commitVisibilityLevel(), "mixed files should be level 3")

	// Cycling should go back to level 1
	m.handleCommitFoldCycle()
	assert.Equal(t, 1, m.commitVisibilityLevel(), "should collapse to level 1")
}

func TestCommitBodyRows_WhenNotFolded(t *testing.T) {
	// Test that commit body rows appear when commit is not folded (level 2+)
	files := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldNormal},
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890fedcba9876543210",
			Author:  "Test Author",
			Email:   "test@example.com",
			Date:    "Mon Jan 15 10:30:00 2024 -0500",
			Subject: "Add new feature",
			Body:    "This is the commit body.\nIt has multiple lines.",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal, // Normal state (level 2)
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	// Build rows and check for commit body rows
	rows := m.buildRows()

	// Should have commit header row first
	require.Greater(t, len(rows), 1, "should have more than one row")
	assert.True(t, rows[0].isCommitHeader, "first row should be commit header")

	// Find commit body rows
	var bodyRows []displayRow
	for _, row := range rows {
		if row.isCommitBody {
			bodyRows = append(bodyRows, row)
		}
	}

	// Should have body rows when expanded
	require.Greater(t, len(bodyRows), 0, "should have commit body rows when expanded")

	// Check content of body rows
	var foundSHA, foundAuthor, foundDate, foundSubject, foundBody bool
	for _, row := range bodyRows {
		if strings.Contains(row.commitBodyLine, "commit abc123def4567890fedcba9876543210") {
			foundSHA = true
		}
		if strings.Contains(row.commitBodyLine, "Author: Test Author") {
			foundAuthor = true
		}
		if strings.Contains(row.commitBodyLine, "Date:") {
			foundDate = true
		}
		if strings.Contains(row.commitBodyLine, "Add new feature") {
			foundSubject = true
		}
		if strings.Contains(row.commitBodyLine, "commit body") {
			foundBody = true
		}
	}

	assert.True(t, foundSHA, "should have full SHA in body rows")
	assert.True(t, foundAuthor, "should have author in body rows")
	assert.True(t, foundDate, "should have date in body rows")
	assert.True(t, foundSubject, "should have subject in body rows")
	assert.True(t, foundBody, "should have body text in body rows")
}

func TestCommitBodyRow_Rendering(t *testing.T) {
	// Test the actual rendering of commit body rows
	files := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldNormal},
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890fedcba9876543210",
			Author:  "Test Author",
			Email:   "test@example.com",
			Date:    "Mon Jan 15 10:30:00 2024 -0500",
			Subject: "Add new feature",
			Body:    "Details here.",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal, // Shows at level 2
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the SHA row and render it
	for _, row := range rows {
		if row.isCommitBody && strings.HasPrefix(row.commitBodyLine, "commit ") {
			rendered := m.renderCommitBodyRow(row, false)
			// Should have 2-space indent after the 2-char prefix (4 spaces total)
			assert.True(t, strings.HasPrefix(rendered, "    commit "), "SHA row should have proper indent")
			assert.Contains(t, rendered, "abc123def4567890fedcba9876543210", "should contain full SHA")
			break
		}
	}

	// Find author row
	for _, row := range rows {
		if row.isCommitBody && strings.HasPrefix(row.commitBodyLine, "Author:") {
			rendered := m.renderCommitBodyRow(row, false)
			assert.Contains(t, rendered, "Author: Test Author <test@example.com>", "should have author with email")
			break
		}
	}

	// Find subject row (indented message)
	for _, row := range rows {
		if row.isCommitBody && strings.Contains(row.commitBodyLine, "Add new feature") {
			rendered := m.renderCommitBodyRow(row, false)
			// Subject should have additional indent (4 spaces) plus the 3-space base indent
			assert.Contains(t, rendered, "    Add new feature", "subject should be indented")
			break
		}
	}
}

func TestCommitBodyRows_NotShownWhenFolded(t *testing.T) {
	// Test that commit body rows do NOT appear when commit is folded (level 1)
	files := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldNormal},
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890",
			Author:  "Test Author",
			Subject: "Add new feature",
			Body:    "This is the commit body.",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitFolded, // Folded state (level 1)
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 100
	m.height = 40
	m.calculateTotalLines()

	rows := m.buildRows()

	// Should NOT have commit body rows when folded
	for _, row := range rows {
		assert.False(t, row.isCommitBody, "should not have commit body rows when folded")
	}
}

func TestCommitBodyRows_BlankLineBetweenSubjectAndBody(t *testing.T) {
	// Test that there's a blank line between the subject and body (traditional git log format)
	files := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldNormal},
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890fedcba9876543210",
			Author:  "Test Author",
			Email:   "test@example.com",
			Date:    "Mon Jan 15 10:30:00 2024 -0500",
			Subject: "Add new feature",
			Body:    "This is the first line of the body.\nThis is the second line.",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find body rows and check for blank line between subject and body
	var bodyRows []displayRow
	for _, row := range rows {
		if row.isCommitBody {
			bodyRows = append(bodyRows, row)
		}
	}

	// Find the subject row index
	subjectIdx := -1
	for i, row := range bodyRows {
		if strings.Contains(row.commitBodyLine, "Add new feature") {
			subjectIdx = i
			break
		}
	}
	require.NotEqual(t, -1, subjectIdx, "should find subject row")

	// The row after the subject should be a blank line
	require.Greater(t, len(bodyRows), subjectIdx+1, "should have rows after subject")
	blankRow := bodyRows[subjectIdx+1]
	assert.True(t, blankRow.commitBodyIsBlank, "row after subject should be blank")
	assert.Empty(t, blankRow.commitBodyLine, "blank row should have empty content")

	// The row after the blank should be the first body line
	require.Greater(t, len(bodyRows), subjectIdx+2, "should have body rows after blank line")
	firstBodyRow := bodyRows[subjectIdx+2]
	assert.Contains(t, firstBodyRow.commitBodyLine, "first line of the body", "should have body content after blank line")
}

func TestCommitSeparatorRow_BetweenCommits(t *testing.T) {
	// Test that there's a blank separator row between commits that belongs to the first commit
	// This ensures proper visual separation and cursor association
	files1 := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldFolded},
	}
	files2 := []sidebyside.FilePair{
		{OldPath: "a/bar.go", NewPath: "b/bar.go", FoldLevel: sidebyside.FoldFolded},
	}
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123",
				Author:  "Author One",
				Subject: "First commit",
			},
			Files:       files1,
			FoldLevel:   sidebyside.CommitNormal, // Level 2 - file headers shown
			FilesLoaded: true,
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "def456",
				Author:  "Author Two",
				Subject: "Second commit",
			},
			Files:       files2,
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
		},
	}
	m := NewWithCommits(commits)
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the second commit header row
	secondCommitHeaderIdx := -1
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, -1, secondCommitHeaderIdx, "should find second commit header")
	require.Greater(t, secondCommitHeaderIdx, 1, "second commit header should not be at start")

	// The row before the second commit header should be a top border (when both commits unfolded)
	// When both commits are unfolded, the separator row becomes a top border for the second commit
	separatorRow := rows[secondCommitHeaderIdx-1]
	assert.True(t, separatorRow.isCommitHeaderTopBorder, "separator should be a top border when both commits unfolded")
	assert.True(t, separatorRow.commitBorderVisible, "border should be visible when both commits unfolded")
	assert.Equal(t, 1, separatorRow.commitIndex, "border should belong to second commit (index 1)")
}

func TestCurrentCommit_UpdatesWithCursorPosition(t *testing.T) {
	// This test verifies that currentCommit() returns the commit the cursor is currently on,
	// including when cursor is on commit body rows (not just the header).
	files1 := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldFolded},
	}
	files2 := []sidebyside.FilePair{
		{OldPath: "a/bar.go", NewPath: "b/bar.go", FoldLevel: sidebyside.FoldFolded},
	}
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "first111",
				Author:  "Author One",
				Subject: "First commit",
			},
			Files:       files1,
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "second22",
				Author:  "Author Two",
				Subject: "Second commit",
			},
			Files:       files2,
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
		},
	}
	m := NewWithCommits(commits)
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find various rows for first and second commit
	var firstCommitHeaderIdx, firstCommitBodyIdx, secondCommitHeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 0 && firstCommitHeaderIdx == 0 {
			firstCommitHeaderIdx = i
		}
		if row.isCommitBody && row.commitIndex == 0 && firstCommitBodyIdx == 0 {
			firstCommitBodyIdx = i
		}
		if row.isCommitHeader && row.commitIndex == 1 && secondCommitHeaderIdx == 0 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, 0, firstCommitBodyIdx, "should find a body row belonging to first commit")
	require.NotEqual(t, 0, secondCommitHeaderIdx, "should find a row belonging to second commit")

	// Position cursor on first commit header
	m.scroll = firstCommitHeaderIdx
	commit := m.currentCommit()
	require.NotNil(t, commit, "should return a commit")
	assert.Equal(t, "first111", commit.Info.SHA, "cursor on first commit header should return first commit")

	// Position cursor on first commit body - should still return first commit
	m.scroll = firstCommitBodyIdx
	commit = m.currentCommit()
	require.NotNil(t, commit, "should return a commit")
	assert.Equal(t, "first111", commit.Info.SHA, "cursor on first commit body should return first commit")

	// Position cursor on second commit header
	m.scroll = secondCommitHeaderIdx
	commit = m.currentCommit()
	require.NotNil(t, commit, "should return a commit")
	assert.Equal(t, "second22", commit.Info.SHA, "cursor on second commit header should return second commit")

	// Find second commit body row
	var secondCommitBodyIdx int
	for i, row := range rows {
		if row.isCommitBody && row.commitIndex == 1 {
			secondCommitBodyIdx = i
			break
		}
	}
	require.NotEqual(t, 0, secondCommitBodyIdx, "should find a body row belonging to second commit")

	// Position cursor on second commit body - should return second commit
	m.scroll = secondCommitBodyIdx
	commit = m.currentCommit()
	require.NotNil(t, commit, "should return a commit")
	assert.Equal(t, "second22", commit.Info.SHA, "cursor on second commit body should return second commit")

	// Find second commit's file row (the file header for bar.go)
	var secondCommitFileIdx int
	for i, row := range rows {
		if row.isHeader && row.fileIndex >= 0 {
			// Check if this file belongs to the second commit
			commitIdx := m.commitForFile(row.fileIndex)
			if commitIdx == 1 {
				secondCommitFileIdx = i
				break
			}
		}
	}
	require.NotEqual(t, 0, secondCommitFileIdx, "should find a file row belonging to second commit")

	// Position cursor on second commit's file - should return second commit
	m.scroll = secondCommitFileIdx
	commit = m.currentCommit()
	require.NotNil(t, commit, "should return a commit")
	assert.Equal(t, "second22", commit.Info.SHA, "cursor on second commit's file should return second commit")
}

func TestTopBar_ShowsCorrectCommit_WhenCursorMoves(t *testing.T) {
	// Verifies the top bar shows the commit that the cursor is currently on
	files1 := []sidebyside.FilePair{
		{OldPath: "a/foo.go", NewPath: "b/foo.go", FoldLevel: sidebyside.FoldFolded},
	}
	files2 := []sidebyside.FilePair{
		{OldPath: "a/bar.go", NewPath: "b/bar.go", FoldLevel: sidebyside.FoldFolded},
	}
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "aaa11111",
				Author:  "Author One",
				Subject: "First commit message",
			},
			Files:       files1,
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "bbb22222",
				Author:  "Author Two",
				Subject: "Second commit message",
			},
			Files:       files2,
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
		},
	}
	m := NewWithCommits(commits)
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the second commit's header row
	var secondCommitHeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, 0, secondCommitHeaderIdx, "should find second commit header")

	// Position cursor on second commit header
	m.scroll = secondCommitHeaderIdx

	topBar := m.renderTopBar()

	// Top bar should show the second commit's info, not the first
	assert.Contains(t, topBar, "bbb2222", "top bar should show second commit SHA")
	assert.Contains(t, topBar, "Second commit", "top bar should show second commit subject")
	assert.NotContains(t, topBar, "aaa1111", "top bar should NOT show first commit SHA")
	assert.NotContains(t, topBar, "First commit", "top bar should NOT show first commit subject")
}

func TestTopBar_ShowsCorrectStats_WhenCursorMoves(t *testing.T) {
	// Verifies the top bar shows the correct file stats for the commit under the cursor
	// First commit: 1 file with +5 -3
	// Second commit: 2 files with +10 -2 total
	files1 := []sidebyside.FilePair{
		{
			OldPath:   "a/foo.go",
			NewPath:   "b/foo.go",
			FoldLevel: sidebyside.FoldFolded,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
			},
		},
	}
	files2 := []sidebyside.FilePair{
		{
			OldPath:   "a/bar.go",
			NewPath:   "b/bar.go",
			FoldLevel: sidebyside.FoldFolded,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
			},
		},
		{
			OldPath:   "a/baz.go",
			NewPath:   "b/baz.go",
			FoldLevel: sidebyside.FoldFolded,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Type: sidebyside.Removed}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
				{Old: sidebyside.Line{Type: sidebyside.Empty}, New: sidebyside.Line{Type: sidebyside.Added}},
			},
		},
	}
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "aaa11111",
				Author:  "Author One",
				Subject: "First commit",
			},
			Files:       files1,
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "bbb22222",
				Author:  "Author Two",
				Subject: "Second commit",
			},
			Files:       files2,
			FoldLevel:   sidebyside.CommitNormal,
			FilesLoaded: true,
		},
	}
	m := NewWithCommits(commits)
	m.width = 100
	m.height = 40
	m.focused = true
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the first and second commit's header rows
	var firstCommitHeaderIdx, secondCommitHeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 0 && firstCommitHeaderIdx == 0 {
			firstCommitHeaderIdx = i
		}
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, 0, secondCommitHeaderIdx, "should find second commit header")

	// Position cursor on first commit - should show "1 file" and "+5 -3"
	m.scroll = firstCommitHeaderIdx
	topBar := m.renderTopBar()
	assert.Contains(t, topBar, "1 file", "first commit should show 1 file")
	assert.Contains(t, topBar, "+5", "first commit should show +5")
	assert.Contains(t, topBar, "-3", "first commit should show -3")

	// Position cursor on second commit - should show "2 files" and "+10 -2"
	m.scroll = secondCommitHeaderIdx
	topBar = m.renderTopBar()
	assert.Contains(t, topBar, "2 files", "second commit should show 2 files")
	assert.Contains(t, topBar, "+10", "second commit should show +10")
	assert.Contains(t, topBar, "-2", "second commit should show -2")
}

func TestIsOnCommitHeader(t *testing.T) {
	// Create model with commit info
	files := []sidebyside.FilePair{
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
	}
	commit := sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     "abc123def4567890",
			Author:  "Test Author",
			Subject: "Fix the bug",
		},
		Files:       files,
		FoldLevel:   sidebyside.CommitNormal,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 80
	m.height = 20
	m.focused = true
	m.calculateTotalLines()

	// Position cursor on commit header (row 0)
	m.scroll = 0 // minScroll, so cursor is at row 0
	assert.True(t, m.isOnCommitHeader(), "cursor at row 0 should be on commit header")

	// Move cursor down past commit header
	m.scroll = 0 // cursor is now at cursorOffset, which should be after commit header
	// For small heights this might still be on commit header, so let's be more explicit
	m.scroll = 1 // Move scroll so cursor is further down
	// This might or might not be on commit header depending on total lines
	// Let's just verify isOnCommitHeader returns the correct value
	rows := m.getRows()
	cursorPos := m.cursorLine()
	if cursorPos >= 0 && cursorPos < len(rows) {
		assert.Equal(t, rows[cursorPos].isCommitHeader, m.isOnCommitHeader())
	}
}
