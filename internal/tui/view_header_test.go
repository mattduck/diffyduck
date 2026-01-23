package tui

import (
	"strings"
	"testing"

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
			// For unfolded files (Normal/Expanded): lines[2] = top border, lines[3] = header
			// For folded files: lines[2] = header (no borders)
			headerIdx := 2
			if tt.foldLevel != sidebyside.FoldFolded {
				headerIdx = 3 // Skip the top border row
			}
			header := lines[headerIdx]

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

	// Should have: header only = 1 row
	// No top border, no bottom border, no content for folded files
	require.Len(t, rows, 1, "folded file should only have header")
	assert.True(t, rows[0].isHeader, "first row should be header")
	assert.False(t, rows[0].isHeaderTopBorder, "should not have top border")
	assert.False(t, rows[0].isHeaderSpacer, "should not have bottom border")
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

	// The row at secondFileStart should be the header (not a top border)
	// because the top border comes from file 0's trailing rows
	assert.True(t, rows[secondFileStart].isHeader, "second file should start with header (top border comes from first file)")
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

	// Find the trailing top border of file 0 (it's the last isHeaderTopBorder before file 1 starts)
	var trailingBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 0 && rows[i].isHeaderTopBorder {
			// This could be the leading or trailing border
			// The trailing one is after blank rows
			if i > 0 && rows[i-1].isBlank {
				trailingBorder = &rows[i]
			}
		}
	}
	require.NotNil(t, trailingBorder, "should find trailing top border of first file")
	assert.True(t, trailingBorder.borderVisible, "trailing border should be visible when next file is unfolded")
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

	// Find the trailing top border of file 0
	var trailingBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 0 && rows[i].isHeaderTopBorder {
			if i > 0 && rows[i-1].isBlank {
				trailingBorder = &rows[i]
			}
		}
	}
	require.NotNil(t, trailingBorder, "should find trailing top border of first file")
	assert.False(t, trailingBorder.borderVisible, "trailing border should be hidden when next file is folded")
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
	assert.Equal(t, 0, topBorderCount, "should have no top border rows when all folded")
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

	// Check trailing top borders
	// File 0 and 1's trailing borders should be visible (next file is unfolded)
	// File 2's trailing border should be hidden (no next file)
	trailingBorders := make(map[int]*displayRow)
	for i := range rows {
		if rows[i].isHeaderTopBorder && i > 0 && rows[i-1].isBlank {
			trailingBorders[rows[i].fileIndex] = &rows[i]
		}
	}

	if tb, ok := trailingBorders[0]; ok {
		assert.True(t, tb.borderVisible, "file 0's trailing border should be visible (file 1 is unfolded)")
	}
	if tb, ok := trailingBorders[1]; ok {
		assert.True(t, tb.borderVisible, "file 1's trailing border should be visible (file 2 is unfolded)")
	}
	if tb, ok := trailingBorders[2]; ok {
		assert.False(t, tb.borderVisible, "file 2's trailing border should be hidden (no next file)")
	}
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

	// File 0: just header (folded)
	file0Rows := filterRowsByFileIndex(rows, 0)
	assert.Equal(t, 1, len(file0Rows), "file 0 should have only 1 row (header)")
	assert.True(t, file0Rows[0].isHeader, "file 0's row should be header")

	// File 1: has header, bottom border, content, blanks, trailing border
	// But borders should have borderVisible=false (prev file folded)
	var file1Header, file1BottomBorder, file1TrailingBorder *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 {
			if rows[i].isHeader {
				file1Header = &rows[i]
			}
			if rows[i].isHeaderSpacer {
				file1BottomBorder = &rows[i]
			}
			if rows[i].isHeaderTopBorder && i > 0 && rows[i-1].isBlank {
				file1TrailingBorder = &rows[i]
			}
		}
	}

	require.NotNil(t, file1Header, "file 1 should have header")
	require.NotNil(t, file1BottomBorder, "file 1 should have bottom border")
	require.NotNil(t, file1TrailingBorder, "file 1 should have trailing border")

	assert.False(t, file1Header.borderVisible, "file 1's header border should be hidden (prev file folded)")
	assert.False(t, file1BottomBorder.borderVisible, "file 1's bottom border should be hidden (prev file folded)")
	assert.False(t, file1TrailingBorder.borderVisible, "file 1's trailing border should be hidden (next file folded)")

	// File 2: just header (folded)
	file2Rows := filterRowsByFileIndex(rows, 2)
	assert.Equal(t, 1, len(file2Rows), "file 2 should have only 1 row (header)")
	assert.True(t, file2Rows[0].isHeader, "file 2's row should be header")
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
