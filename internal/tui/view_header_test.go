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
	// Test that file status indicators appear in headers for all fold levels
	tests := []struct {
		name          string
		oldPath       string
		newPath       string
		foldLevel     sidebyside.FoldLevel
		wantIndicator string
	}{
		{
			name:          "added file - folded",
			oldPath:       "/dev/null",
			newPath:       "b/new.go",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: "+",
		},
		{
			name:          "deleted file - folded",
			oldPath:       "a/old.go",
			newPath:       "/dev/null",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: "-",
		},
		{
			name:          "renamed file - folded",
			oldPath:       "a/old.go",
			newPath:       "b/new.go",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: "→",
		},
		{
			name:          "modified file - folded",
			oldPath:       "a/file.go",
			newPath:       "b/file.go",
			foldLevel:     sidebyside.FoldFolded,
			wantIndicator: "~",
		},
		{
			name:          "added file - normal",
			oldPath:       "/dev/null",
			newPath:       "b/new.go",
			foldLevel:     sidebyside.FoldNormal,
			wantIndicator: "+",
		},
		{
			name:          "modified file - expanded",
			oldPath:       "a/file.go",
			newPath:       "b/file.go",
			foldLevel:     sidebyside.FoldExpanded,
			wantIndicator: "~",
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

			output := m.View()
			lines := strings.Split(output, "\n")
			// Layout: [topBar, divider, content..., bottomBar]
			// lines[2] = first file border slot (blank when folded, border when unfolded)
			// lines[3] = header (always at this position)
			header := lines[3]

			// Get the expected fold icon
			foldIcon := m.foldLevelIcon(tt.foldLevel)

			// Header format should be: <foldIcon> <#fileNum> <statusIndicator> filename
			// e.g., "○ #1 + new.go" or "◐ #1 ~ file.go"
			expectedPattern := foldIcon + " #1 " + tt.wantIndicator + " "
			assert.Contains(t, header, expectedPattern,
				"header should contain fold icon, file number, then status indicator: %s", expectedPattern)
		})
	}
}

func TestView_CursorArrowOnFileHeader(t *testing.T) {
	// Test that cursor arrow appears on file header when selected
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
	// cursorLine = scroll + cursorOffset, so scroll = 0 - cursorOffset
	m.scroll = -m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the file header line (contains test.go and fold icon, not the top bar)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line with test.go and fold icon")
	// Header line should contain the arrow character when cursor is on it
	assert.Contains(t, headerLine, "▶", "file header with cursor should have arrow indicator")
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
	m.scroll = -m.cursorOffset()

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

	// Find the file header line (contains test.go and fold icon)
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "test.go") && strings.Contains(line, "●") {
			headerLine = line
			break
		}
	}

	require.NotEmpty(t, headerLine, "should find file header line")

	// The header should contain the file number "1" followed by spaces to pad to lineNumWidth
	// Since we have 1 file, the file number is "1" and lineNumWidth is 5, so we get "1    "
	assert.Contains(t, headerLine, "1", "header should contain file number")
}

func TestView_HeaderSpacerWithCursorMatchesContentLineLayout(t *testing.T) {
	// Test that the bottom border with cursor has proper layout:
	// - Single arrow at start
	// - Horizontal line with ┘ corner
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

	// Position cursor on bottom border (row 2: top_border=0, header=1, bottom_border=2)
	m.scroll = 2 - m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the line with cursor arrow and ┘ corner (bottom border with cursor)
	var borderLine string
	for _, line := range lines {
		if strings.Contains(line, "▶") && strings.Contains(line, "┘") {
			borderLine = line
			break
		}
	}

	require.NotEmpty(t, borderLine, "should find bottom border line with cursor")

	// Bottom border with cursor should have ONE arrow
	arrowCount := strings.Count(borderLine, "▶")
	assert.Equal(t, 1, arrowCount, "bottom border with cursor should have one arrow")

	// Bottom border should have horizontal line
	assert.Contains(t, borderLine, "─", "bottom border should have horizontal line")

	// Test content line with cursor
	// Position cursor on content line (row 3: top_border=0, header=1, bottom_border=2, content=3)
	m.scroll = 3 - m.cursorOffset()
	output2 := m.View()
	lines2 := strings.Split(output2, "\n")

	// Find the line with cursor arrows and content (not borders)
	var contentLine string
	for _, line := range lines2 {
		if strings.Count(line, "▶") == 2 && strings.Contains(line, "content") {
			contentLine = line
			break
		}
	}

	require.NotEmpty(t, contentLine, "should find content line with cursor")

	// Content line should have two arrows (one per side)
	contentArrowCount := strings.Count(contentLine, "▶")
	assert.Equal(t, 2, contentArrowCount, "content line with cursor should have two arrows")

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

	// Should have: border slot + header = 2 rows
	// Border slot renders as blank when folded
	require.Len(t, rows, 2, "folded file should have border slot + header")
	assert.True(t, rows[0].isHeaderTopBorder, "first row should be border slot")
	assert.False(t, rows[0].borderVisible, "border slot should not be visible when folded")
	assert.True(t, rows[1].isHeader, "second row should be header")
}

// Test: First file unfolded has leading top border with borderVisible=true
func TestBuildRows_FirstFileUnfoldedHasTopBorder(t *testing.T) {
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
		height: 20,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// First row should be isHeaderTopBorder with borderVisible=true
	require.True(t, len(rows) >= 1, "should have at least one row")
	assert.True(t, rows[0].isHeaderTopBorder, "first row should be top border")
	assert.True(t, rows[0].borderVisible, "first file's top border should be visible")
}

// Test: Non-first file unfolded has no leading top border (comes from file above)
func TestBuildRows_NonFirstFileNoLeadingTopBorder(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal,
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
				FoldLevel: sidebyside.FoldNormal,
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
				FoldLevel: sidebyside.FoldNormal, // Next file is unfolded
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
	assert.True(t, file1TopBorder.borderVisible, "top border should be visible when file is unfolded")
}

// Test: Trailing top border visibility - next file folded
func TestBuildRows_TrailingTopBorderHiddenWhenNextFolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal,
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
	assert.False(t, file1TopBorder.borderVisible, "top border should be hidden when file is folded")
}

// Test: Header borderVisible - first file
func TestBuildRows_HeaderBorderVisibleFirstFile(t *testing.T) {
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
	assert.True(t, headerRow.borderVisible, "first file's header should have borderVisible=true")
}

// Test: Header borderVisible - previous file unfolded
func TestBuildRows_HeaderBorderVisibleWhenPrevUnfolded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldNormal, // Previous file unfolded
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
	assert.True(t, headerRow.borderVisible, "header should have borderVisible=true when previous file is unfolded")
}

// Test: Header borderVisible - previous file folded
func TestBuildRows_HeaderBorderHiddenWhenPrevFolded(t *testing.T) {
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
	assert.False(t, headerRow.borderVisible, "header should have borderVisible=false when previous file is folded")
}

// Test: Bottom border borderVisible matches header
func TestBuildRows_BottomBorderMatchesHeaderVisibility(t *testing.T) {
	tests := []struct {
		name          string
		prevFoldLevel sidebyside.FoldLevel
		expectVisible bool
	}{
		{
			name:          "previous_unfolded",
			prevFoldLevel: sidebyside.FoldNormal,
			expectVisible: true,
		},
		{
			name:          "previous_folded",
			prevFoldLevel: sidebyside.FoldFolded,
			expectVisible: false,
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
				height: 20,
				keys:   DefaultKeyMap(),
			}

			rows := m.buildRows()

			// Find header and bottom border for file 1
			var headerRow, bottomBorderRow *displayRow
			for i := range rows {
				if rows[i].fileIndex == 1 {
					if rows[i].isHeader {
						headerRow = &rows[i]
					}
					if rows[i].isHeaderSpacer {
						bottomBorderRow = &rows[i]
					}
				}
			}
			require.NotNil(t, headerRow, "should find header row")
			require.NotNil(t, bottomBorderRow, "should find bottom border row")

			assert.Equal(t, tt.expectVisible, headerRow.borderVisible, "header borderVisible should match expected")
			assert.Equal(t, tt.expectVisible, bottomBorderRow.borderVisible, "bottom border borderVisible should match header")
		})
	}
}

// Test: renderHeaderTopBorder uses correct style based on borderVisible
func TestRenderHeaderTopBorder_BorderVisibility(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
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

	// Visible border
	visibleOutput := m.renderHeaderTopBorder(30, true, FileStatusModified, false)
	assert.Contains(t, visibleOutput, "─", "visible border should contain horizontal line")
	assert.Contains(t, visibleOutput, "┐", "visible border should end with corner")

	// Hidden border (still rendered, just different color)
	hiddenOutput := m.renderHeaderTopBorder(30, false, FileStatusModified, false)
	assert.Contains(t, hiddenOutput, "─", "hidden border should contain horizontal line")
	assert.Contains(t, hiddenOutput, "┐", "hidden border should end with corner")
}

// Test: renderHeaderTopBorder cursor highlighting
func TestRenderHeaderTopBorder_CursorRow(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
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
	cursorOutput := m.renderHeaderTopBorder(30, true, FileStatusModified, true)
	assert.Contains(t, cursorOutput, "▶", "cursor row should have arrow indicator")

	// Non-cursor row should not have arrow
	normalOutput := m.renderHeaderTopBorder(30, true, FileStatusModified, false)
	assert.NotContains(t, normalOutput, "▶", "non-cursor row should not have arrow")
}

// Test: renderHeaderBottomBorder uses correct corner character
func TestRenderHeaderBottomBorder_CorrectCorner(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
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

	output := m.renderHeaderBottomBorder(30, true, FileStatusModified, false)
	assert.Contains(t, output, "┘", "bottom border should end with ┘ corner")
	assert.NotContains(t, output, "┐", "bottom border should not use ┐ corner")
}

// Test: renderHeaderBottomBorder border visibility styling
func TestRenderHeaderBottomBorder_BorderVisibility(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
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
	visibleOutput := m.renderHeaderBottomBorder(30, true, FileStatusModified, false)
	hiddenOutput := m.renderHeaderBottomBorder(30, false, FileStatusModified, false)

	assert.Contains(t, visibleOutput, "─", "visible border should contain horizontal line")
	assert.Contains(t, hiddenOutput, "─", "hidden border should contain horizontal line")
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
	// First file always has a border slot (but renders as blank when folded)
	assert.Equal(t, 1, topBorderCount, "should have 1 border slot (for first file, renders blank when folded)")
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
				FoldLevel: sidebyside.FoldNormal,
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
				FoldLevel: sidebyside.FoldNormal,
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
		height: 50,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	// Check that headers and their bottom borders have correct visibility
	for _, row := range rows {
		if row.isHeader && row.fileIndex >= 0 {
			// All headers should have borderVisible=true since all files are unfolded
			assert.True(t, row.borderVisible, "header for file %d should have borderVisible=true", row.fileIndex)
		}
		if row.isHeaderSpacer && row.fileIndex >= 0 {
			// All bottom borders should have borderVisible=true
			assert.True(t, row.borderVisible, "bottom border for file %d should have borderVisible=true", row.fileIndex)
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
		assert.True(t, tb.borderVisible, "file 1's top border should be visible (file 1 is unfolded)")
	}
	if tb, ok := topBorders[2]; ok {
		assert.True(t, tb.borderVisible, "file 2's top border should be visible (file 2 is unfolded)")
	}
	// There's no border after file 2 since there's no file 3
}

// Test: Integration - mixed fold states
func TestBuildRows_MixedFoldStates(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded, // File 0: folded
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
				FoldLevel: sidebyside.FoldNormal, // File 1: unfolded
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
				FoldLevel: sidebyside.FoldFolded, // File 2: folded
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

	// File 0: border slot + header (folded)
	file0Rows := filterRowsByFileIndex(rows, 0)
	assert.Equal(t, 2, len(file0Rows), "file 0 should have 2 rows (border slot + header)")
	assert.True(t, file0Rows[0].isHeaderTopBorder, "file 0's first row should be border slot")
	assert.True(t, file0Rows[1].isHeader, "file 0's second row should be header")

	// File 1: has header, bottom border, content, blanks
	// NOTE: File 1 does NOT have a top border because file 0 is folded
	// (folded files don't create trailing borders)
	// The border between file 1 and file 2 belongs to file 2 (fileIndex=2)
	var file1Header, file1BottomBorder, file2TopBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 {
			if rows[i].isHeader {
				file1Header = &rows[i]
			}
			if rows[i].isHeaderSpacer {
				file1BottomBorder = &rows[i]
			}
		}
		// The border after file 1's content belongs to file 2
		if rows[i].fileIndex == 2 && rows[i].isHeaderTopBorder {
			file2TopBorder = &rows[i]
		}
	}

	require.NotNil(t, file1Header, "file 1 should have header")
	require.NotNil(t, file1BottomBorder, "file 1 should have bottom border")
	require.NotNil(t, file2TopBorder, "file 2 should have top border (the border after file 1's content)")

	// Header/spacer borders are hidden when previous file is folded
	assert.False(t, file1Header.borderVisible, "file 1's header border should be hidden (prev file folded)")
	assert.False(t, file1BottomBorder.borderVisible, "file 1's bottom border should be hidden (prev file folded)")
	assert.False(t, file2TopBorder.borderVisible, "file 2's top border should be hidden (file 2 is folded)")

	// File 2: top border + header (folded)
	// The top border belongs to file 2 (the file it precedes)
	file2Rows := filterRowsByFileIndex(rows, 2)
	assert.Equal(t, 2, len(file2Rows), "file 2 should have 2 rows (top border + header)")
	assert.True(t, file2Rows[0].isHeaderTopBorder, "file 2's first row should be top border")
	assert.True(t, file2Rows[1].isHeader, "file 2's second row should be header")
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

// Test: Border width matches headerBoxWidth
func TestRenderHeaderTopBorder_WidthAlignment(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
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

	// Different header box widths
	widths := []int{20, 30, 40, 50}

	for _, w := range widths {
		topBorder := m.renderHeaderTopBorder(w, true, FileStatusModified, false)
		bottomBorder := m.renderHeaderBottomBorder(w, true, FileStatusModified, false)

		// Both borders should have same display width
		topWidth := displayWidth(topBorder)
		bottomWidth := displayWidth(bottomBorder)

		assert.Equal(t, topWidth, bottomWidth, "top and bottom border widths should match for headerBoxWidth=%d", w)
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

	// Test with first commit UNFOLDED - should have both ▔ divider AND ━ border
	// The ━ border appears in the file line (which replaces what would be empty space)
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

	// Should have ━ border (in file line position) when unfolded
	var hasBorder bool
	for _, line := range linesUnfolded {
		if strings.Contains(line, "━") {
			hasBorder = true
			break
		}
	}
	assert.True(t, hasBorder, "unfolded commit should have ━ border in file line")

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
	assert.True(t, prevRow.commitBorderVisible, "top border should be visible when both commits are unfolded")
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

	// Unfold second commit
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	rowsUnfolded := m.buildRows()
	unfoldedCount := len(rowsUnfolded)

	// The second commit body adds rows, but NO extra row should be added for
	// top border since first commit is folded (no separator to convert)
	// The bottom border replaces the first body blank, so net effect is:
	// - Body rows are added (this is expected behavior)
	// - No extra top border row added
	t.Logf("Folded: %d rows, Unfolded: %d rows", foldedCount, unfoldedCount)

	// Count how many body rows the second commit would add
	bodyRows := m.buildCommitBodyRowsSkipFirstBlank(&m.commits[1], 1)
	expectedBodyRowCount := len(bodyRows) + 1 // +1 for bottom border (replaces first blank)

	// The difference should be exactly the body rows (no extra top border)
	actualDiff := unfoldedCount - foldedCount
	assert.Equal(t, expectedBodyRowCount, actualDiff, "row count difference should be exactly body rows + bottom border")
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
	assert.True(t, prevRow.commitBorderVisible, "top border should be visible")

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
func TestCommitBorder_FirstUnfolded_SecondFolded_SeparatorStaysBlank(t *testing.T) {
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

	// The row immediately before second commit's header should be a blank (not top border)
	// because second commit is folded
	prevRow := rows[secondCommitHeaderIdx-1]
	assert.False(t, prevRow.isCommitHeaderTopBorder, "separator should remain blank when second commit is folded")
	assert.True(t, prevRow.isCommitBody && prevRow.commitBodyIsBlank, "separator should be a blank commit body row")
}

func TestBorderAlignmentWithCursor(t *testing.T) {
	// Test: when cursor is on top or bottom border line for a file, the border is still aligned
	// (same width as when cursor is not on the border)

	lipgloss.SetColorProfile(termenv.TrueColor)

	// Create a model with multiple files
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
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: makePairs(5), FoldLevel: sidebyside.FoldNormal},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: makePairs(5), FoldLevel: sidebyside.FoldNormal},
	})
	m.width = 80
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	// Build rows
	rows := m.buildRows()

	// Find the top border for file 1 (second file) by looking for the row before file 1's header
	var topBorderIdx int
	for i, row := range rows {
		if row.isHeader && row.fileIndex == 1 {
			// The top border is the row immediately before the header
			if i > 0 && rows[i-1].isHeaderTopBorder {
				topBorderIdx = i - 1
			}
			break
		}
	}
	require.NotZero(t, topBorderIdx, "should find top border for file 1")

	// Get the headerBoxWidth for this border
	borderRow := rows[topBorderIdx]
	headerBoxWidth := borderRow.headerBoxWidth

	// Render the border without cursor
	noCursorBorder := m.renderHeaderTopBorder(headerBoxWidth, borderRow.borderVisible, borderRow.status, false)

	// Render the border with cursor
	cursorBorder := m.renderHeaderTopBorder(headerBoxWidth, borderRow.borderVisible, borderRow.status, true)

	// Strip ANSI codes and compare visual character width (rune count, not byte count)
	noCursorLen := utf8.RuneCountInString(stripANSI(noCursorBorder))
	cursorLen := utf8.RuneCountInString(stripANSI(cursorBorder))

	// Both should have the same visual width
	assert.Equal(t, noCursorLen, cursorLen,
		"border with cursor (%d) should have same width as border without cursor (%d)",
		cursorLen, noCursorLen)

	// Verify the cursor border has the correct structure: arrow + space + styled gutter + rest
	// The stripped content should start with "▶─" (arrow followed by a dash as space)
	strippedCursor := stripANSI(cursorBorder)
	assert.True(t, strings.HasPrefix(strippedCursor, "▶─"),
		"cursor border should start with arrow followed by dash (space), got: %q", strippedCursor[:min(10, len(strippedCursor))])

	// Also test the bottom border (header spacer)
	var spacerIdx int
	for i, row := range rows {
		if row.isHeaderSpacer && row.fileIndex == 1 {
			spacerIdx = i
			break
		}
	}
	require.NotZero(t, spacerIdx, "should find header spacer for file 1")

	spacerRow := rows[spacerIdx]
	noCursorSpacer := m.renderHeaderBottomBorder(spacerRow.headerBoxWidth, spacerRow.borderVisible, spacerRow.status, false)
	cursorSpacer := m.renderHeaderBottomBorder(spacerRow.headerBoxWidth, spacerRow.borderVisible, spacerRow.status, true)

	noCursorSpacerLen := utf8.RuneCountInString(stripANSI(noCursorSpacer))
	cursorSpacerLen := utf8.RuneCountInString(stripANSI(cursorSpacer))

	assert.Equal(t, noCursorSpacerLen, cursorSpacerLen,
		"bottom border with cursor (%d) should have same width as border without cursor (%d)",
		cursorSpacerLen, noCursorSpacerLen)
}
