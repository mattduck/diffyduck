package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestFileStatus(t *testing.T) {
	tests := []struct {
		name       string
		oldPath    string
		newPath    string
		isRename   bool
		isCopy     bool
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
			name:       "renamed file - detected from paths",
			oldPath:    "a/old.go",
			newPath:    "b/new.go",
			wantStatus: FileStatusRenamed,
		},
		{
			name:       "renamed file - explicit metadata",
			oldPath:    "a/old.go",
			newPath:    "b/new.go",
			isRename:   true,
			wantStatus: FileStatusRenamed,
		},
		{
			name:       "copied file",
			oldPath:    "a/original.go",
			newPath:    "b/copy.go",
			isCopy:     true,
			wantStatus: FileStatusCopied,
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
			fp := sidebyside.FilePair{
				OldPath:  tt.oldPath,
				NewPath:  tt.newPath,
				IsRename: tt.isRename,
				IsCopy:   tt.isCopy,
			}
			got := fileStatusFromPair(fp)
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
		{FileStatusRenamed, "→"},
		{FileStatusCopied, "→"},
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
	// Test that file headers show fold icon and filename (no file number or status symbol in main view)
	tests := []struct {
		name      string
		oldPath   string
		newPath   string
		foldLevel sidebyside.FoldLevel
		wantFile  string // expected filename in header
	}{
		{
			name:      "added file - folded",
			oldPath:   "/dev/null",
			newPath:   "b/new.go",
			foldLevel: sidebyside.FoldFolded,
			wantFile:  "new.go",
		},
		{
			name:      "deleted file - folded",
			oldPath:   "a/old.go",
			newPath:   "/dev/null",
			foldLevel: sidebyside.FoldFolded,
			wantFile:  "old.go",
		},
		{
			name:      "modified file - folded",
			oldPath:   "a/file.go",
			newPath:   "b/file.go",
			foldLevel: sidebyside.FoldFolded,
			wantFile:  "file.go",
		},
		{
			name:      "added file - normal",
			oldPath:   "/dev/null",
			newPath:   "b/new.go",
			foldLevel: sidebyside.FoldNormal,
			wantFile:  "new.go",
		},
		{
			name:      "modified file - expanded",
			oldPath:   "a/file.go",
			newPath:   "b/file.go",
			foldLevel: sidebyside.FoldExpanded,
			wantFile:  "file.go",
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
			m.w().scroll = m.minScroll()

			output := m.View()
			lines := strings.Split(output, "\n")

			foldIcon := m.foldLevelIcon(tt.foldLevel)

			// Find the content file header by searching after the divider line
			// Top bar is 3 content lines + divider = 4 lines, so content starts at index 4
			var header string
			for i := 4; i < len(lines); i++ {
				if strings.Contains(lines[i], foldIcon) && strings.Contains(lines[i], tt.wantFile) {
					header = lines[i]
					break
				}
			}

			require.NotEmpty(t, header, "should find header with fold icon %s and filename %s", foldIcon, tt.wantFile)
			// File number and status symbol should NOT be in main view headers
			assert.NotContains(t, header, "#1",
				"main view header should not contain file number")
		})
	}
}

func TestView_CursorArrowOnFileHeader(t *testing.T) {
	// Test that cursor arrow appears on file header in the content area when selected
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
	m.w().scroll = 0

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the content area file header (contains ▌ arrow when cursor is on it)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "▌") && strings.Contains(line, "test.go") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find content file header with arrow and test.go")
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
	m.w().scroll = 0

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

	// The top bar file line (line index 0) shows the file counter (#1)
	topBar := lines[0]
	assert.Contains(t, topBar, "#1", "top bar should contain file number")
	assert.Contains(t, topBar, "test.go", "top bar should contain file name")
}

func TestView_HeaderSpacerWithCursorMatchesContentLineLayout(t *testing.T) {
	// Test that the bottom border with cursor has proper layout:
	// - Single arrow at start
	// - Horizontal line with ┗ corner (heavy, for tree layout)
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

	// In diff view layout: header=0, spacer(bottom border)=1, content=2
	// Position cursor on bottom border (row 1)
	m.w().scroll = 1

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the line with cursor arrow and ┗ corner (bottom border with cursor, heavy for tree layout)
	var borderLine string
	for _, line := range lines {
		if strings.Contains(line, "▌") && strings.Contains(line, "┗") {
			borderLine = line
			break
		}
	}

	require.NotEmpty(t, borderLine, "should find bottom border line with cursor")

	// Bottom border with cursor should have ONE arrow
	arrowCount := strings.Count(borderLine, "▌")
	assert.Equal(t, 1, arrowCount, "bottom border with cursor should have one arrow")

	// Bottom border should have horizontal line (heavy ━)
	assert.Contains(t, borderLine, "━", "bottom border should have heavy horizontal line")

	// Test content line with cursor
	// Position cursor on content line (row 2 in diff view)
	m.w().scroll = 2
	output2 := m.View()
	lines2 := strings.Split(output2, "\n")

	// Find the line with cursor arrow and content (not borders)
	var contentLine string
	for _, line := range lines2 {
		if strings.Count(line, "▌") == 1 && strings.Contains(line, "content") {
			contentLine = line
			break
		}
	}

	require.NotEmpty(t, contentLine, "should find content line with cursor")

	// Content line should have one arrow (in the tree gutter, not in line number columns)
	contentArrowCount := strings.Count(contentLine, "▌")
	assert.Equal(t, 1, contentArrowCount, "content line with cursor should have one arrow in tree gutter")

	// Content line should have a separator (┃ box drawings heavy vertical)
	hasSeparator := strings.Contains(contentLine, "┃")
	assert.True(t, hasSeparator, "content line should have center separator (┃)")
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

	// In diff view, folded file has header + terminator row
	require.Len(t, rows, 2, "folded file should have header + terminator row")
	assert.True(t, rows[0].isHeader, "first row should be header")
	assert.False(t, rows[0].treePath.Current.IsLast, "header should use ├ (terminator follows)")
	assert.True(t, rows[1].isBlank, "second row should be terminator blank")
	assert.True(t, rows[1].treeTerminator, "second row should be a tree terminator")
	require.Greater(t, len(rows[1].treePath.Ancestors), 0, "terminator should have ancestors")
	assert.False(t, rows[1].treePath.Ancestors[0].IsLast, "terminator ancestor IsLast=false so ┴ renders")
}

// Test: First file unfolded has header as first row (top border is in padding area in diff view)
func TestBuildRows_FirstFileUnfoldedHasHeaderAsFirstRow(t *testing.T) {
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
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// In diff view (no commit metadata), first file has header as first row
	// The top border is rendered in the padding area, not in buildRows
	require.True(t, len(rows) >= 1, "should have at least one row")
	assert.True(t, rows[0].isHeader, "first row should be header (top border is in padding area)")
}

// Test: Non-first file unfolded has no leading top border (comes from file above)
func TestBuildRows_NonFirstFileNoLeadingTopBorder(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldExpanded,
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

	// The row at secondFileStart should be the top border (which now belongs to file 1)
	// The top border visually and semantically belongs to the file it precedes
	assert.True(t, rows[secondFileStart].isHeaderTopBorder, "second file should start with its top border")
}

// Test: Trailing top border visibility - next file unfolded
func TestBuildRows_TrailingTopBorderVisibleWhenNextUnfolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldExpanded,
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
				FoldLevel: sidebyside.FoldExpanded, // Next file is unfolded
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

	// Find the top border of file 1 (the border between file 0 and file 1)
	// This border visually separates file 0's content from file 1's header
	var file1TopBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 && rows[i].isHeaderTopBorder {
			file1TopBorder = &rows[i]
			break
		}
	}
	require.NotNil(t, file1TopBorder, "should find top border of second file")
	assert.Equal(t, HeaderThreeLine, file1TopBorder.headerMode, "top border should be visible when file is unfolded")
}

// Test: Trailing top border visibility - next file folded
func TestBuildRows_TrailingTopBorderHiddenWhenNextFolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldExpanded,
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

	// Find the top border of file 1 (the border between file 0 and file 1)
	var file1TopBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 && rows[i].isHeaderTopBorder {
			file1TopBorder = &rows[i]
			break
		}
	}
	require.NotNil(t, file1TopBorder, "should find top border of second file")
	assert.NotEqual(t, HeaderThreeLine, file1TopBorder.headerMode, "top border should be hidden when file is folded")
}

// Test: Header borderVisible - first file
func TestBuildRows_HeaderBorderVisibleFirstFile(t *testing.T) {
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
	assert.True(t, headerRow.headerMode == HeaderThreeLine, "first file's header should have borderVisible=true")
}

// Test: Header borderVisible - previous file unfolded
func TestBuildRows_HeaderBorderVisibleWhenPrevUnfolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldExpanded, // Previous file unfolded
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
	assert.True(t, headerRow.headerMode == HeaderThreeLine, "header should have borderVisible=true when previous file is unfolded")
}

// Test: Header borderVisible - previous file folded
func TestBuildRows_UnfoldedFileHasBottomBorder(t *testing.T) {
	// In tree layout, an unfolded file has a bottom border regardless of
	// the previous file's fold state
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
				FoldLevel: sidebyside.FoldExpanded, // This file is unfolded
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

	// Find the bottom border row of file 1 (the unfolded file)
	var bottomBorderRow *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 && rows[i].isHeaderSpacer {
			bottomBorderRow = &rows[i]
			break
		}
	}
	require.NotNil(t, bottomBorderRow, "unfolded file should have bottom border row")
	assert.True(t, bottomBorderRow.headerMode == HeaderThreeLine, "unfolded file should have HeaderThreeLine for bottom border")
}

// Test: Bottom border visibility is based on file's own fold state, not previous file
func TestBuildRows_BottomBorderBasedOnOwnFoldState(t *testing.T) {
	tests := []struct {
		name          string
		prevFoldLevel sidebyside.FoldLevel
		thisFoldLevel sidebyside.FoldLevel
		expectBottom  bool // expect bottom border for file 1
	}{
		{
			name:          "both_unfolded",
			thisFoldLevel: sidebyside.FoldExpanded,
			expectBottom:  true,
		},
		{
			name:          "prev_folded_this_unfolded",
			thisFoldLevel: sidebyside.FoldExpanded,
			expectBottom:  true, // unfolded file has bottom border regardless of prev
		},
		{
			name:          "both_folded",
			thisFoldLevel: sidebyside.FoldFolded,
			expectBottom:  false, // folded file has no bottom border
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
						FoldLevel: tt.thisFoldLevel,
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

			// Find bottom border for file 1
			var bottomBorderRow *displayRow
			for i := range rows {
				if rows[i].fileIndex == 1 && rows[i].isHeaderSpacer {
					bottomBorderRow = &rows[i]
					break
				}
			}

			if tt.expectBottom {
				require.NotNil(t, bottomBorderRow, "unfolded file should have bottom border row")
				assert.True(t, bottomBorderRow.headerMode == HeaderThreeLine, "unfolded file should have HeaderThreeLine")
			} else {
				assert.Nil(t, bottomBorderRow, "folded file should not have bottom border row")
			}
		})
	}
}

// Test: renderHeaderTopBorder now renders tree continuation (not box border)
func TestRenderHeaderTopBorder_TreeContinuation(t *testing.T) {
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
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// With no tree path, top border renders empty (for layout spacing)
	output := m.renderHeaderTopBorder(30, HeaderThreeLine, FileStatusModified, false, treeWidth(0, true), TreePath{})
	assert.Equal(t, "", output, "top border with no tree path should be empty")

	// With tree path, top border renders tree continuation
	treePath := TreePath{
		Ancestors: []TreeLevel{{IsLast: false, Style: lipgloss.NewStyle()}},
		Current:   nil,
	}
	treeOutput := m.renderHeaderTopBorder(30, HeaderThreeLine, FileStatusModified, false, treeWidth(1, true), treePath)
	assert.Contains(t, treeOutput, "│", "top border with tree path should show tree continuation")
}

// Test: renderHeaderTopBorder cursor highlighting
func TestRenderHeaderTopBorder_CursorRow(t *testing.T) {
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
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// Cursor row should have arrow indicator
	cursorOutput := m.renderHeaderTopBorder(30, HeaderThreeLine, FileStatusModified, true, treeWidth(0, true), TreePath{})
	assert.Contains(t, cursorOutput, "▌", "cursor row should have arrow indicator")

	// Non-cursor row should not have arrow
	normalOutput := m.renderHeaderTopBorder(30, HeaderThreeLine, FileStatusModified, false, treeWidth(0, true), TreePath{})
	assert.NotContains(t, normalOutput, "▌", "non-cursor row should not have arrow")

	// Cursor row WITH tree path should have both arrow and tree continuation
	treePath := TreePath{
		Ancestors: []TreeLevel{{IsLast: false, Style: lipgloss.NewStyle()}},
		Current:   nil,
	}
	cursorTreeOutput := m.renderHeaderTopBorder(30, HeaderThreeLine, FileStatusModified, true, treeWidth(1, true), treePath)
	assert.Contains(t, cursorTreeOutput, "▌", "cursor+tree should have arrow")
	assert.Contains(t, cursorTreeOutput, "│", "cursor+tree should preserve tree continuation")
	assert.NotContains(t, cursorTreeOutput, "░", "cursor+tree should not have shading")
}

// Test: renderHeaderBottomBorder uses correct corner character
func TestRenderHeaderBottomBorder_CorrectCorner(t *testing.T) {
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
		height: 20,
		keys:   DefaultKeyMap(),
	}

	output := m.renderHeaderBottomBorder(30, HeaderThreeLine, FileStatusModified, false, treeWidth(0, true), TreePath{}, sidebyside.FoldNormal)
	// Uses heavy box-drawing: ┗ corner (not ┘)
	assert.Contains(t, output, "┗", "bottom border should use ┗ corner")
	assert.Contains(t, output, "━", "bottom border should use heavy horizontal line")
}

// Test: renderHeaderBottomBorder border visibility styling
func TestRenderHeaderBottomBorder_BorderVisibility(t *testing.T) {
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
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// Both visible and hidden borders should render (just with different colors)
	visibleOutput := m.renderHeaderBottomBorder(30, HeaderThreeLine, FileStatusModified, false, treeWidth(0, true)+1, TreePath{}, sidebyside.FoldNormal)
	hiddenOutput := m.renderHeaderBottomBorder(30, HeaderSingleLine, FileStatusModified, false, treeWidth(0, true)+1, TreePath{}, sidebyside.FoldNormal)

	// Uses heavy box-drawing characters: ┗ corner and ━ horizontal
	assert.Contains(t, visibleOutput, "━", "visible border should contain heavy horizontal line")
	assert.Contains(t, hiddenOutput, "━", "hidden border should contain heavy horizontal line")
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
	// When all files are folded, there are no top borders or bottom borders
	// Top borders are only added after unfolded content (for the next file)
	assert.Equal(t, 0, topBorderCount, "should have 0 top borders when all files are folded")
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
				FoldLevel: sidebyside.FoldExpanded,
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
				FoldLevel: sidebyside.FoldExpanded,
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
		height: 50,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Check that headers and their bottom borders have correct visibility
	for _, row := range rows {
		if row.isHeader && row.fileIndex >= 0 {
			// All headers should have borderVisible=true since all files are unfolded
			assert.True(t, row.headerMode == HeaderThreeLine, "header for file %d should have borderVisible=true", row.fileIndex)
		}
		if row.isHeaderSpacer && row.fileIndex >= 0 {
			// All bottom borders should have borderVisible=true
			assert.True(t, row.headerMode == HeaderThreeLine, "bottom border for file %d should have borderVisible=true", row.fileIndex)
		}
	}

	// Check top borders between files
	// The top border for each file (except the first) is the visual separator from the previous file
	// File 1's top border should be visible (file 1 is unfolded)
	// File 2's top border should be visible (file 2 is unfolded)
	// File 2 has no "trailing" border after it since there's no file 3
	topBorders := make(map[int]*displayRow)
	for i := range rows {
		if rows[i].isHeaderTopBorder {
			topBorders[rows[i].fileIndex] = &rows[i]
		}
	}

	if tb, ok := topBorders[1]; ok {
		assert.True(t, tb.headerMode == HeaderThreeLine, "file 1's top border should be visible (file 1 is unfolded)")
	}
	if tb, ok := topBorders[2]; ok {
		assert.True(t, tb.headerMode == HeaderThreeLine, "file 2's top border should be visible (file 2 is unfolded)")
	}
	// There's no border after file 2 since there's no file 3
}

// Test: Integration - mixed fold states
func TestBuildRows_MixedFoldStates(t *testing.T) {
	// In tree layout, bottom border visibility depends on the file's own fold state,
	// not the previous file's fold state
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded, // File 0: folded - no bottom border
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
				FoldLevel: sidebyside.FoldExpanded, // File 1: unfolded - has bottom border
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
				FoldLevel: sidebyside.FoldFolded, // File 2: folded - no bottom border
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

	// File 0: folded, should have header only (no bottom border)
	file0Rows := filterRowsByFileIndex(rows, 0)
	assert.Equal(t, 1, len(file0Rows), "folded file 0 should have 1 row (header only)")
	assert.True(t, file0Rows[0].isHeader, "file 0's row should be header")

	// File 1: unfolded, should have header + bottom border + content
	var file1Header, file1BottomBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 {
			if rows[i].isHeader {
				file1Header = &rows[i]
			}
			if rows[i].isHeaderSpacer {
				file1BottomBorder = &rows[i]
			}
		}
	}

	require.NotNil(t, file1Header, "unfolded file 1 should have header")
	require.NotNil(t, file1BottomBorder, "unfolded file 1 should have bottom border")
	assert.True(t, file1BottomBorder.headerMode == HeaderThreeLine,
		"unfolded file 1's bottom border should have HeaderThreeLine")

	// File 2: folded, should have header only (no bottom border)
	var file2BottomBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 2 && rows[i].isHeaderSpacer {
			file2BottomBorder = &rows[i]
			break
		}
	}
	assert.Nil(t, file2BottomBorder, "folded file 2 should not have bottom border")
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

// Test: Bottom border width scales with headerBoxWidth
func TestRenderHeaderBottomBorder_WidthScaling(t *testing.T) {
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
		height: 20,
		keys:   DefaultKeyMap(),
	}

	// Different header box widths should produce different border widths
	widths := []int{20, 30, 40, 50}
	var prevWidth int

	for _, w := range widths {
		bottomBorder := m.renderHeaderBottomBorder(w, HeaderThreeLine, FileStatusModified, false, treeWidth(0, true)+1, TreePath{}, sidebyside.FoldNormal)
		borderWidth := displayWidth(bottomBorder)

		// Wider header box should produce wider border
		assert.Greater(t, borderWidth, prevWidth, "border width should increase with headerBoxWidth=%d", w)
		prevWidth = borderWidth
	}
}

// === Commit Header Border Tests ===

// Test: First commit should NOT have a top border slot in content rows.
// The top border for the first commit uses the divider line above content, not a new row.
// This ensures cursor starts on the commit header, not an empty line.
func TestCommitBorder_FirstCommit_NoTopBorderSlotInContent(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "abc123",
					Subject: "Test commit",
					Author:  "Alice",
				},
				FoldLevel: sidebyside.CommitFolded,
			},
		},
		commitFileStarts: []int{0},
		files:            []sidebyside.FilePair{},
		width:            100,
		height:           20,
		keys:             DefaultKeyMap(),
	}

	rows := m.buildRows()

	// First content row should be the commit header, NOT a border slot
	require.Greater(t, len(rows), 0, "should have at least one row")
	assert.True(t, rows[0].isCommitHeader, "first row should be commit header, not a border slot")
	assert.Equal(t, 0, rows[0].commitIndex, "first row should belong to commit 0")

	// There should be NO top border slot rows for the first commit
	for i, row := range rows {
		if row.isCommitHeaderTopBorder && row.commitIndex == 0 {
			t.Errorf("row %d is a top border slot for first commit - should not exist in content rows", i)
		}
	}
}

// Test: First commit unfolded - still no top border slot, border renders in divider area
func TestCommitBorder_FirstCommit_Unfolded_NoTopBorderSlot(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "abc123",
					Subject: "Test commit",
					Author:  "Alice",
					Date:    "2024-01-01",
				},
				FoldLevel: sidebyside.CommitNormal, // UNFOLDED
			},
		},
		commitFileStarts: []int{0},
		files:            []sidebyside.FilePair{},
		width:            100,
		height:           20,
		keys:             DefaultKeyMap(),
	}

	rows := m.buildRows()

	// First content row should still be the commit header
	require.Greater(t, len(rows), 0, "should have at least one row")
	assert.True(t, rows[0].isCommitHeader, "first row should be commit header even when unfolded")

	// Second row should be bottom border (when unfolded)
	require.Greater(t, len(rows), 1, "should have at least two rows when unfolded")
	assert.True(t, rows[1].isCommitHeaderBottomBorder, "second row should be bottom border")
}

// Test: First commit's top border renders in the file line (View output)
// The file line becomes the top border when first commit is unfolded and cursor is on commit section
func TestCommitBorder_FirstCommit_TopBorderInFileLine(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	// Test with first commit FOLDED - should have ▔ divider, no ━ border
	mFolded := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "abc123",
					Subject: "Test commit",
					Author:  "Alice",
				},
				FoldLevel: sidebyside.CommitFolded,
			},
		},
		commitFileStarts: []int{0},
		files:            []sidebyside.FilePair{},
		width:            50,
		height:           10,
		focused:          true,
		keys:             DefaultKeyMap(),
	}
	mFolded.calculateTotalLines()

	outputFolded := mFolded.View()
	linesFolded := strings.Split(outputFolded, "\n")

	// Find the divider line (should contain ▔ when folded)
	var hasDivider bool
	for _, line := range linesFolded {
		if strings.Contains(line, "▔") {
			hasDivider = true
			break
		}
	}
	assert.True(t, hasDivider, "folded commit should have divider with ▔")

	// Should NOT have ━ border when folded
	var hasBorderWhenFolded bool
	for _, line := range linesFolded {
		if strings.Contains(line, "━") {
			hasBorderWhenFolded = true
			break
		}
	}
	assert.False(t, hasBorderWhenFolded, "folded commit should not have ━ border")

	// Test with first commit UNFOLDED - should have ▔ divider but NO ─ border
	// (commit header bottom border is now an empty line)
	mUnfolded := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "abc123",
					Subject: "Test commit",
					Author:  "Alice",
					Date:    "2024-01-01",
				},
				FoldLevel: sidebyside.CommitNormal, // UNFOLDED
			},
		},
		commitFileStarts: []int{0},
		files:            []sidebyside.FilePair{},
		width:            50,
		height:           10,
		focused:          true,
		keys:             DefaultKeyMap(),
	}
	mUnfolded.calculateTotalLines()

	outputUnfolded := mUnfolded.View()
	linesUnfolded := strings.Split(outputUnfolded, "\n")

	// Should NOT have ─ bottom border (it's now an empty line)
	var hasBorder bool
	for _, line := range linesUnfolded {
		if strings.Contains(line, "─") {
			hasBorder = true
			break
		}
	}
	assert.False(t, hasBorder, "unfolded commit should NOT have ─ bottom border (now empty line)")

	// Should still have ▔ divider (unchanged)
	var hasDividerUnfolded bool
	for _, line := range linesUnfolded {
		if strings.Contains(line, "▔") {
			hasDividerUnfolded = true
			break
		}
	}
	assert.True(t, hasDividerUnfolded, "unfolded commit should still have ▔ divider")
}

// Test: Second commit unfolded but first folded - NO shift (no extra top border row)
func TestCommitBorder_SecondCommit_Unfolded_FirstFolded_NoShift(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "aaa111",
					Subject: "First commit",
					Author:  "Alice",
				},
				FoldLevel: sidebyside.CommitFolded, // First commit FOLDED
			},
			{
				Info: sidebyside.CommitInfo{
					SHA:     "bbb222",
					Subject: "Second commit",
					Author:  "Bob",
					Date:    "2024-01-02",
				},
				FoldLevel: sidebyside.CommitNormal, // Second commit UNFOLDED
			},
		},
		commitFileStarts: []int{0, 0},
		files:            []sidebyside.FilePair{},
		width:            100,
		height:           20,
		keys:             DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Count row types
	var topBorderCount, commitHeaderCount, bottomBorderCount int
	for _, row := range rows {
		if row.isCommitHeaderTopBorder {
			topBorderCount++
		}
		if row.isCommitHeader {
			commitHeaderCount++
		}
		if row.isCommitHeaderBottomBorder {
			bottomBorderCount++
		}
	}

	// Should have 0 top borders (first commit doesn't have top border slot in content rows)
	assert.Equal(t, 0, topBorderCount, "should have no top borders (first commit has no slot, second has none because first is folded)")

	// Should have 2 commit headers
	assert.Equal(t, 2, commitHeaderCount, "should have 2 commit headers")

	// Should have 1 bottom border (for second commit which is unfolded)
	assert.Equal(t, 1, bottomBorderCount, "should have 1 bottom border (second commit is unfolded)")

	// Find the second commit's header - there should be NO top border row immediately before it
	// because the first commit is folded (no trailing blank to convert)
	var secondCommitHeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, 0, secondCommitHeaderIdx, "should find second commit header")

	// The row before second commit header should NOT be a top border
	// (since first commit is folded, there's no trailing blank to convert)
	prevRow := rows[secondCommitHeaderIdx-1]
	assert.False(t, prevRow.isCommitHeaderTopBorder, "row before second commit header should NOT be a top border when first commit is folded")
}

// Test: Both commits unfolded - separator becomes top border for second commit
func TestCommitBorder_BothUnfolded_SeparatorBecomesTopBorder(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "aaa111",
					Subject: "First commit",
					Author:  "Alice",
					Date:    "2024-01-01",
				},
				FoldLevel: sidebyside.CommitNormal, // UNFOLDED
			},
			{
				Info: sidebyside.CommitInfo{
					SHA:     "bbb222",
					Subject: "Second commit",
					Author:  "Bob",
					Date:    "2024-01-02",
				},
				FoldLevel: sidebyside.CommitNormal, // UNFOLDED
			},
		},
		commitFileStarts: []int{0, 0},
		files:            []sidebyside.FilePair{},
		width:            100,
		height:           20,
		keys:             DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the second commit's header
	var secondCommitHeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, 0, secondCommitHeaderIdx, "should find second commit header")

	// The row immediately before second commit header SHOULD be a top border
	// (the separator row is converted to top border when both commits are unfolded)
	prevRow := rows[secondCommitHeaderIdx-1]
	assert.True(t, prevRow.isCommitHeaderTopBorder, "row before second commit header should be a top border when both commits are unfolded")
	assert.Equal(t, HeaderThreeLine, prevRow.headerMode, "top border should be visible when both commits are unfolded")
	assert.Equal(t, 1, prevRow.commitIndex, "top border should belong to second commit")
}

// Test: Row count stability when toggling second commit (first folded)
func TestCommitBorder_RowCountStability_FirstFolded(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "aaa111",
					Subject: "First commit",
					Author:  "Alice",
				},
				FoldLevel: sidebyside.CommitFolded, // FOLDED
			},
			{
				Info: sidebyside.CommitInfo{
					SHA:     "bbb222",
					Subject: "Second commit",
					Author:  "Bob",
					Date:    "2024-01-02",
				},
				FoldLevel: sidebyside.CommitFolded, // Start FOLDED
			},
		},
		commitFileStarts: []int{0, 0},
		files:            []sidebyside.FilePair{},
		width:            100,
		height:           20,
		keys:             DefaultKeyMap(),
	}

	// Count rows when second commit is folded
	rowsFolded := m.buildRows()
	foldedCount := len(rowsFolded)

	// Unfold second commit to CommitNormal
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	rowsUnfolded := m.buildRows()
	unfoldedCount := len(rowsUnfolded)

	// With CommitNormal, the commit info node is shown under the commit header.
	// In tree layout, the difference includes:
	// - 1 for commit info header
	// - 1 for commit info bottom border
	// - 1 for additional tree structure row
	t.Logf("Folded: %d rows, Unfolded: %d rows", foldedCount, unfoldedCount)

	// Verify the difference is consistent (actual behavior shows 3 rows added)
	expectedDiff := 3
	actualDiff := unfoldedCount - foldedCount
	assert.Equal(t, expectedDiff, actualDiff, "row count difference should be consistent for commit unfold")
}

// Test: With files - separator row converts to top border when both unfolded
func TestCommitBorder_WithFiles_SeparatorConversion(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "aaa111",
					Subject: "First commit",
					Author:  "Alice",
					Date:    "2024-01-01",
				},
				FoldLevel: sidebyside.CommitNormal, // UNFOLDED
			},
			{
				Info: sidebyside.CommitInfo{
					SHA:     "bbb222",
					Subject: "Second commit",
					Author:  "Bob",
					Date:    "2024-01-02",
				},
				FoldLevel: sidebyside.CommitNormal, // UNFOLDED
			},
		},
		commitFileStarts: []int{0, 1},
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/file1.go",
				NewPath:   "b/file1.go",
				FoldLevel: sidebyside.FoldFolded,
			},
			{
				OldPath:   "a/file2.go",
				NewPath:   "b/file2.go",
				FoldLevel: sidebyside.FoldFolded,
			},
		},
		width:  100,
		height: 30,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the second commit's header
	var secondCommitHeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, 0, secondCommitHeaderIdx, "should find second commit header")

	// The row immediately before second commit's header should be the top border
	prevRow := rows[secondCommitHeaderIdx-1]
	assert.True(t, prevRow.isCommitHeaderTopBorder, "separator should be converted to top border when both commits unfolded")
	assert.Equal(t, HeaderThreeLine, prevRow.headerMode, "top border should be visible")

	// Count blank separators vs top borders between commits
	var separatorCount, topBorderCount int
	for _, row := range rows {
		if row.isCommitBody && row.commitBodyIsBlank {
			separatorCount++
		}
		if row.isCommitHeaderTopBorder {
			topBorderCount++
		}
	}

	// Should have 1 top border total:
	// - One converted from separator (between commits)
	// First commit has no top border slot in content rows
	assert.Equal(t, 1, topBorderCount, "should have 1 top border (separator conversion only)")
}

// Test: First commit unfolded, second folded - separator stays as blank
func TestCommitBorder_TreeLayoutConnectsCommitsDirectly(t *testing.T) {
	// In tree layout, commits are connected via tree branches, not blank separators
	lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		commits: []sidebyside.CommitSet{
			{
				Info: sidebyside.CommitInfo{
					SHA:     "aaa111",
					Subject: "First commit",
					Author:  "Alice",
					Date:    "2024-01-01",
				},
				FoldLevel: sidebyside.CommitNormal, // UNFOLDED
			},
			{
				Info: sidebyside.CommitInfo{
					SHA:     "bbb222",
					Subject: "Second commit",
					Author:  "Bob",
				},
				FoldLevel: sidebyside.CommitFolded, // FOLDED
			},
		},
		commitFileStarts: []int{0, 1},
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/file1.go",
				NewPath:   "b/file1.go",
				FoldLevel: sidebyside.FoldFolded,
			},
			{
				OldPath:   "a/file2.go",
				NewPath:   "b/file2.go",
				FoldLevel: sidebyside.FoldFolded,
			},
		},
		width:  100,
		height: 30,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Find the second commit's header
	var secondCommitHeaderIdx int
	for i, row := range rows {
		if row.isCommitHeader && row.commitIndex == 1 {
			secondCommitHeaderIdx = i
			break
		}
	}
	require.NotEqual(t, 0, secondCommitHeaderIdx, "should find second commit header")

	// Verify tree layout - no stray blank separator rows between commits
	// Tree terminator blanks (┴) are expected; other blanks are not
	var strayBlanks bool
	for i := 0; i < secondCommitHeaderIdx; i++ {
		row := rows[i]
		if row.isBlank && !row.isHeaderSpacer && !row.isCommitBody && !row.treeTerminator {
			strayBlanks = true
			break
		}
	}
	assert.False(t, strayBlanks, "tree layout should not have stray blank rows between commits (tree terminators are OK)")
}

func TestBorderAlignmentWithCursor(t *testing.T) {
	// Test: In tree layout, bottom border with cursor has same width as without cursor
	// Tree layout structure:
	//   ├━━━ ◐ #1 ~ first.go     <- header with tree branch
	//   │    ┗━━━━━━━━━━━━        <- bottom border with ┗ corner
	//   │      1   content        <- content with tree continuation

	lipgloss.SetColorProfile(termenv.TrueColor)

	makePairs := func(n int) []sidebyside.LinePair {
		pairs := make([]sidebyside.LinePair, n)
		for i := range pairs {
			pairs[i] = sidebyside.LinePair{
				Old: sidebyside.Line{Num: i + 1, Content: "left content"},
				New: sidebyside.Line{Num: i + 1, Content: "right content"},
			}
		}
		return pairs
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: makePairs(5), FoldLevel: sidebyside.FoldExpanded},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: makePairs(5), FoldLevel: sidebyside.FoldExpanded},
	})
	m.width = 80
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	rows := m.buildRows()

	// Find the bottom border (header spacer) for file 1
	var spacerIdx int
	for i, row := range rows {
		if row.isHeaderSpacer && row.fileIndex == 1 {
			spacerIdx = i
			break
		}
	}
	require.NotZero(t, spacerIdx, "should find header spacer for file 1")

	spacerRow := rows[spacerIdx]
	noCursorSpacer := m.renderHeaderBottomBorder(spacerRow.headerBoxWidth, spacerRow.headerMode, spacerRow.status, false, spacerRow.treePrefixWidth, spacerRow.treePath, spacerRow.foldLevel)
	cursorSpacer := m.renderHeaderBottomBorder(spacerRow.headerBoxWidth, spacerRow.headerMode, spacerRow.status, true, spacerRow.treePrefixWidth, spacerRow.treePath, spacerRow.foldLevel)

	noCursorSpacerLen := utf8.RuneCountInString(stripANSI(noCursorSpacer))
	cursorSpacerLen := utf8.RuneCountInString(stripANSI(cursorSpacer))

	// Bottom border should have same width with or without cursor
	assert.Equal(t, noCursorSpacerLen, cursorSpacerLen,
		"bottom border with cursor (%d) should have same width as border without cursor (%d)",
		cursorSpacerLen, noCursorSpacerLen)

	// Bottom border with cursor should start with arrow
	strippedCursor := stripANSI(cursorSpacer)
	assert.True(t, strings.HasPrefix(strippedCursor, "▌"),
		"bottom border with cursor should start with arrow, got: %q", strippedCursor[:min(10, len(strippedCursor))])

	// Bottom border should contain ┗ corner (heavy, for tree layout)
	assert.Contains(t, strippedCursor, "┗",
		"bottom border should contain ┗ corner for tree layout")
}

func TestDiffView_CursorStartsOnFileHeader(t *testing.T) {
	// In diff view (no commits), the cursor should start on the file header line,
	// not on a top border. The file header should be the first row of content,
	// meaning you can't scroll up to see a border above it.
	//
	// This contrasts with log/show view where the first commit's top border
	// is rendered in the fixed top bar (non-scrollable).

	lipgloss.SetColorProfile(termenv.Ascii)

	// Create a simple diff view with one file
	m := New([]sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "old line", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 1, Content: "new line", Type: sidebyside.Added},
				},
			},
		},
	})
	m.width = 80
	m.height = 20

	rows := m.buildRows()
	require.NotEmpty(t, rows, "should have at least one row")

	// The first row should be the file header, not a top border
	assert.Equal(t, RowKindHeader, rows[0].kind,
		"first row should be file header (RowKindHeader), not top border")

	// At minScroll, the cursor should be on row 0 (the file header)
	m.w().scroll = m.minScroll()
	cursorPos := m.cursorLine()
	assert.Equal(t, 0, cursorPos,
		"at minScroll, cursor should be on row 0 (file header)")

	// Verify that row 0 is indeed the file header
	assert.Equal(t, RowKindHeader, rows[cursorPos].kind,
		"cursor at minScroll should be on file header row")

	// Verify there's no top border row we could scroll to
	for i, row := range rows {
		if row.kind == RowKindHeaderTopBorder && row.fileIndex == 0 {
			t.Errorf("found top border for first file at row %d; first file should not have a scrollable top border", i)
		}
	}
}

func TestTreeLayoutAlignment(t *testing.T) {
	// Test that tree layout components are properly aligned.
	// In tree layout:
	//   ├━━━ ◐ #1 ~ tmp.txt     <- header with tree branch
	//   │    ┗━━━━━━━━━━━━       <- bottom border with ┗ corner
	//   │      1   content       <- content with tree continuation
	//
	// Key alignments:
	// 1. treePrefixWidth is consistent between header and bottom border rows
	// 2. headerBoxWidth is consistent across related rows
	// 3. Bottom border ┗ is properly indented under header

	lipgloss.SetColorProfile(termenv.TrueColor)

	tests := []struct {
		name    string
		oldPath string
		newPath string
		added   int
		removed int
	}{
		{"added file with one line", "/dev/null", "b/tmp.txt", 1, 0},
		{"additions only", "a/tmp.txt", "b/tmp.txt", 1, 0},
		{"removals only", "a/tmp.txt", "b/tmp.txt", 0, 1},
		{"both additions and removals", "a/tmp.txt", "b/tmp.txt", 1, 1},
		{"larger additions only", "a/tmp.txt", "b/tmp.txt", 42, 0},
		{"no changes", "a/tmp.txt", "b/tmp.txt", 0, 0},
		{"deleted file", "a/tmp.txt", "/dev/null", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pairs []sidebyside.LinePair
			for i := range tt.added {
				pairs = append(pairs, sidebyside.LinePair{
					New: sidebyside.Line{Num: i + 1, Content: "added line", Type: sidebyside.Added},
					Old: sidebyside.Line{Type: sidebyside.Empty},
				})
			}
			for i := range tt.removed {
				pairs = append(pairs, sidebyside.LinePair{
					Old: sidebyside.Line{Num: i + 1, Content: "removed line", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				})
			}
			if len(pairs) == 0 {
				pairs = append(pairs, sidebyside.LinePair{
					Old: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
				})
			}

			m := New([]sidebyside.FilePair{
				{
					OldPath:   "a/dummy.txt",
					NewPath:   "b/dummy.txt",
					Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context}}},
					FoldLevel: sidebyside.FoldExpanded,
				},
				{
					OldPath:   tt.oldPath,
					NewPath:   tt.newPath,
					Pairs:     pairs,
					FoldLevel: sidebyside.FoldExpanded,
				},
			})
			m.width = 100
			m.height = 40
			m.initialFoldSet = true
			m.focused = true
			m.keys = DefaultKeyMap()

			rows := m.buildRows()

			// Find header and bottom border rows for file 1
			var headerRow, bottomBorderRow *displayRow
			for i := range rows {
				if rows[i].fileIndex == 1 {
					if rows[i].isHeader {
						headerRow = &rows[i]
					} else if rows[i].isHeaderSpacer {
						bottomBorderRow = &rows[i]
					}
				}
			}

			require.NotNil(t, headerRow, "should find header row for file 1")
			require.NotNil(t, bottomBorderRow, "should find bottom border row for file 1")

			// Verify headerBoxWidth is consistent
			assert.Equal(t, headerRow.headerBoxWidth, bottomBorderRow.headerBoxWidth,
				"bottom border should have same headerBoxWidth as header")

			// Render the bottom border and verify it has proper tree structure
			bottomBorder := m.renderHeaderBottomBorder(
				bottomBorderRow.headerBoxWidth,
				bottomBorderRow.headerMode,
				bottomBorderRow.status,
				false,
				bottomBorderRow.treePrefixWidth,
				bottomBorderRow.treePath,
				bottomBorderRow.foldLevel,
			)
			bottomStripped := stripANSI(bottomBorder)

			// Bottom border should contain ┗ (heavy corner for tree layout)
			assert.Contains(t, bottomStripped, "┗",
				"bottom border should contain ┗ corner: %q", bottomStripped)

			// Bottom border should contain heavy horizontal line ━
			assert.Contains(t, bottomStripped, "━",
				"bottom border should contain heavy horizontal line: %q", bottomStripped)

			// Render the header and verify it has tree branch
			header := m.renderHeader(
				headerRow.header,
				headerRow.foldLevel,
				headerRow.headerMode,
				headerRow.status,
				headerRow.added,
				headerRow.removed,
				headerRow.headerBoxWidth,
				headerRow.fileIndex,
				0,
				false,
				headerRow.treePath,
			)
			headerStripped := stripANSI(header)

			// Header should have tree branch (├ for non-last, └ for last file)
			hasTreeBranch := strings.Contains(headerStripped, "├") || strings.Contains(headerStripped, "└")
			assert.True(t, hasTreeBranch,
				"header should contain tree branch (├ or └): %q", headerStripped)
		})
	}
}
