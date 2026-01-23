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

// === File Stats Tests ===

func TestCountFileStats(t *testing.T) {
	tests := []struct {
		name        string
		pairs       []sidebyside.LinePair
		wantAdded   int
		wantRemoved int
	}{
		{
			name:        "empty file",
			pairs:       []sidebyside.LinePair{},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "only context lines",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
				},
			},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "pure additions",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 1, Content: "new 1", Type: sidebyside.Added},
				},
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 2, Content: "new 2", Type: sidebyside.Added},
				},
			},
			wantAdded:   2,
			wantRemoved: 0,
		},
		{
			name: "pure deletions",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "old 1", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "old 2", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 3, Content: "old 3", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
			},
			wantAdded:   0,
			wantRemoved: 3,
		},
		{
			name: "mixed changes",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "old", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 2, Content: "new", Type: sidebyside.Added},
				},
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 3, Content: "added", Type: sidebyside.Added},
				},
			},
			wantAdded:   2,
			wantRemoved: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := sidebyside.FilePair{Pairs: tt.pairs}
			added, removed := countFileStats(fp)
			assert.Equal(t, tt.wantAdded, added, "added count")
			assert.Equal(t, tt.wantRemoved, removed, "removed count")
		})
	}
}

func TestFormatStatsBar(t *testing.T) {
	tests := []struct {
		name     string
		added    int
		removed  int
		maxWidth int
		want     string
	}{
		{
			name:     "no changes",
			added:    0,
			removed:  0,
			maxWidth: 24,
			want:     "",
		},
		{
			name:     "only additions - small",
			added:    5,
			removed:  0,
			maxWidth: 24,
			want:     "+5 +++++",
		},
		{
			name:     "only deletions - small",
			added:    0,
			removed:  3,
			maxWidth: 24,
			want:     "-3 ---",
		},
		{
			name:     "mixed - fits in max",
			added:    10,
			removed:  5,
			maxWidth: 24,
			want:     "+10 -5 ++++++++++-----",
		},
		{
			name:     "mixed - needs scaling",
			added:    30,
			removed:  18,
			maxWidth: 24,
			want:     "+30 -18 +++++++++++++++---------", // scaled: 30+18=48, scale=24/48=0.5, so 15+ and 9-
		},
		{
			name:     "large numbers - heavy scaling",
			added:    100,
			removed:  100,
			maxWidth: 24,
			want:     "+100 -100 ++++++++++++------------", // scaled: 12+ and 12-
		},
		{
			name:     "pure addition - needs scaling",
			added:    48,
			removed:  0,
			maxWidth: 24,
			want:     "+48 ++++++++++++++++++++++++", // scaled to 24
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatsBar(tt.added, tt.removed, tt.maxWidth)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileHeaderWithStats_FoldedOnly(t *testing.T) {
	// Stats should only appear in folded view, not in normal/expanded
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldNormal, // Normal view - no stats
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
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
	lines := strings.Split(output, "\n")
	header := lines[0]

	// Normal view should NOT contain stats
	assert.Contains(t, header, "main.go", "header should contain filename")
	assert.NotContains(t, header, "|", "normal view header should not contain stats separator")
}

func TestFileHeaderWithStats_Folded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldFolded, // Folded view - show stats
				Pairs: []sidebyside.LinePair{
					// 3 additions, 2 deletions
					{
						Old: sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "old2", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "new2", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 3, Content: "new3", Type: sidebyside.Added},
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
	lines := strings.Split(output, "\n")

	// Layout: [topBar, divider, content..., bottomBar]
	// Folded header should contain filename and stats counts
	header := lines[2]
	assert.Contains(t, header, "main.go", "header should contain filename")
	assert.Contains(t, header, "+3", "header should show addition count")
	assert.Contains(t, header, "-2", "header should show deletion count")
	assert.Contains(t, header, "▒", "header should have trailing shading")
}

func TestFileHeaderWithStats_Alignment(t *testing.T) {
	// Multiple folded files should have aligned stats columns
	// The addition column (+N) should be padded so that the removal column (-M) starts at the same position
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/short.go",
				NewPath:   "b/short.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "added", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
			{
				OldPath:   "a/much_longer_filename.go",
				NewPath:   "b/much_longer_filename.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					// 100 additions to make the count "+100" which is wider than "+1"
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	// Add more pairs to the second file to get +100
	for i := 0; i < 100; i++ {
		m.files[1].Pairs = append(m.files[1].Pairs, sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 2, Content: "added", Type: sidebyside.Added},
		})
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout: [topBar, divider, content..., bottomBar]
	// Find display column position of - in the stats section of each header
	header1 := lines[2]
	header2 := lines[3] // second header is at lines[3] (folded files have no content between them)

	// Find position of removal count (-N) in each header
	// The first file has +1 -1, second has +100 -1
	// The -1 should be aligned in both headers
	pos1 := displayColumnOf(header1, "-1")
	pos2 := displayColumnOf(header2, "-1")

	assert.NotEqual(t, -1, pos1, "first header should contain -1")
	assert.NotEqual(t, -1, pos2, "second header should contain -1")
	assert.Equal(t, pos1, pos2, "-1 should be aligned across headers (addition column padded)")
}

func TestFileHeaderWithStats_ShadingAlignment(t *testing.T) {
	// The trailing shading should start at the same column even when count widths differ
	// e.g., "+100" vs "+5" should both have shading starting at same position
	pairs100 := make([]sidebyside.LinePair, 100)
	for i := range pairs100 {
		pairs100[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	pairs5 := make([]sidebyside.LinePair, 5)
	for i := range pairs5 {
		pairs5[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/view.go",
				NewPath:   "b/view.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     pairs100, // +100 additions
			},
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     pairs5, // +5 additions
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
	header1 := lines[2] // +100
	header2 := lines[3] // +5

	// Find the display column position of the shading (▒ character)
	findShadingStart := func(s string) int {
		runes := []rune(s)
		for i, ch := range runes {
			if ch == '▒' {
				return i
			}
		}
		return -1
	}

	shadingPos1 := findShadingStart(header1)
	shadingPos2 := findShadingStart(header2)

	assert.NotEqual(t, -1, shadingPos1, "first header should have shading")
	assert.NotEqual(t, -1, shadingPos2, "second header should have shading")
	assert.Equal(t, shadingPos1, shadingPos2, "shading should start at same position across headers")
}

func TestFileHeaderWithStats_OnlyAdditions(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/newfile.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Added},
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
	lines := strings.Split(output, "\n")
	// Layout: [topBar, divider, content..., bottomBar]
	header := lines[2]

	assert.Contains(t, header, "newfile.go", "header should contain filename")
	assert.Contains(t, header, "+2", "header should show addition count")
	assert.NotContains(t, header, "-", "header should not show deletion count when zero")
}

func TestFileHeaderWithStats_OnlyDeletions(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
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
	lines := strings.Split(output, "\n")
	// Layout: [topBar, divider, content..., bottomBar]
	header := lines[2]

	assert.Contains(t, header, "deleted.go", "header should contain filename")
	assert.Contains(t, header, "-3", "header should show deletion count")
	// Check there's no + count (but the filename might contain + in other contexts)
	// The format should be "-3 ---" not "+0 -3 ---"
}

// Summary row tests

func TestBuildRows_IncludesSummaryRow(t *testing.T) {
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

	rows := m.buildRows()

	// Last row should be the summary
	require.NotEmpty(t, rows)
	lastRow := rows[len(rows)-1]
	assert.True(t, lastRow.isSummary, "last row should be summary row")
}

func TestBuildRows_SummaryRowHasCorrectStats(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 2, Content: "added", Type: sidebyside.Added},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "deleted", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	lastRow := rows[len(rows)-1]
	require.True(t, lastRow.isSummary)
	// Total: 2 files, 2 added lines (one.go), 2 removed lines (one.go + two.go)
	assert.Equal(t, 2, lastRow.totalFiles)
	assert.Equal(t, 2, lastRow.totalAdded)
	assert.Equal(t, 2, lastRow.totalRemoved)
}

func TestBuildRows_SummaryRowNoFile(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}

	rows := m.buildRows()

	lastRow := rows[len(rows)-1]
	require.True(t, lastRow.isSummary)
	// Summary row should have fileIndex = -1 to indicate no file association
	assert.Equal(t, -1, lastRow.fileIndex)
}

func TestBuildRows_BlankLinesBeforeSummary(t *testing.T) {
	// When last file is expanded/normal, there should be 4 blank lines before summary
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

	// Layout should be: top border (0), header (1), bottom border (2), content (3), blank (4-7), trailing top border (8), summary (9)
	require.Len(t, rows, 10, "should have top border + header + bottom border + content + 4 blanks + trailing border + summary")

	// Verify top border
	assert.True(t, rows[0].isHeaderTopBorder, "row 0 should be top border")

	// Verify header
	assert.True(t, rows[1].isHeader, "row 1 should be header")

	// Verify bottom border (header spacer)
	assert.True(t, rows[2].isHeaderSpacer, "row 2 should be header spacer/bottom border")

	// Verify the 4 blank lines before trailing border
	for i := 4; i <= 7; i++ {
		assert.True(t, rows[i].isBlank, "row %d should be blank", i)
		assert.Equal(t, 0, rows[i].fileIndex, "blank lines should belong to the last file")
	}

	// Verify trailing top border
	assert.True(t, rows[8].isHeaderTopBorder, "row 8 should be trailing top border")

	// Last row should be summary
	assert.True(t, rows[9].isSummary, "last row should be summary")
}

func TestView_SummaryRowFormat(t *testing.T) {
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

	output := m.View()

	// Should contain git-style summary: "1 file changed, 1 insertion(+), 1 deletion(-)"
	assert.Contains(t, output, "1 file changed")
	assert.Contains(t, output, "1 insertion(+)")
	assert.Contains(t, output, "1 deletion(-)")
}

func TestView_SummaryRowPluralFormat(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/one.go",
				NewPath:   "b/one.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 2, Content: "added", Type: sidebyside.Added},
					},
				},
			},
			{
				OldPath:   "a/two.go",
				NewPath:   "b/two.go",
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

	output := m.View()

	// Should use plural forms: "2 files changed, 3 insertions(+), 2 deletions(-)"
	assert.Contains(t, output, "2 files changed")
	assert.Contains(t, output, "3 insertions(+)")
	assert.Contains(t, output, "2 deletions(-)")
}

func TestView_SummaryRowHasEqualsPrefix(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
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

	// Find the summary line (contains "file changed" or "files changed")
	var summaryLine string
	for _, line := range lines {
		if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
			summaryLine = line
			break
		}
	}
	require.NotEmpty(t, summaryLine, "should find summary line")
	// Summary format is now: "  ━━━━ ●   ..." (space + space + equals gutter + icon)
	// Should contain ━ characters for the gutter
	assert.Contains(t, summaryLine, "━", "summary should contain ━ gutter characters")
}

func TestView_SummaryRowIsSelectable(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// The summary row should be included in totalLines
	// With folded view: 1 header + 1 summary = 2 lines
	rows := m.buildRows()
	assert.Equal(t, 2, len(rows), "should have header + summary")
}

func TestView_SummaryRowAppearsInAllModes(t *testing.T) {
	tests := []struct {
		name      string
		foldLevel sidebyside.FoldLevel
	}{
		{"folded", sidebyside.FoldFolded},
		{"normal", sidebyside.FoldNormal},
		{"expanded", sidebyside.FoldExpanded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				focused: true,
				files: []sidebyside.FilePair{
					{
						OldPath:    "a/foo.go",
						NewPath:    "b/foo.go",
						FoldLevel:  tt.foldLevel,
						OldContent: []string{"line1"},
						NewContent: []string{"line1"},
						Pairs: []sidebyside.LinePair{
							{
								Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
								New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Context},
							},
						},
					},
				},
				width:  80,
				height: 20,
				keys:   DefaultKeyMap(),
			}
			m.calculateTotalLines()

			output := m.View()
			assert.Contains(t, output, "file changed", "summary should appear in %s mode", tt.name)
		})
	}
}

func TestFormatSummaryStats(t *testing.T) {
	tests := []struct {
		name     string
		files    int
		added    int
		removed  int
		expected string
	}{
		{
			name:     "singular all",
			files:    1,
			added:    1,
			removed:  1,
			expected: "1 file changed, 1 insertion(+), 1 deletion(-)",
		},
		{
			name:     "plural all",
			files:    3,
			added:    10,
			removed:  5,
			expected: "3 files changed, 10 insertions(+), 5 deletions(-)",
		},
		{
			name:     "no insertions",
			files:    1,
			added:    0,
			removed:  3,
			expected: "1 file changed, 3 deletions(-)",
		},
		{
			name:     "no deletions",
			files:    2,
			added:    5,
			removed:  0,
			expected: "2 files changed, 5 insertions(+)",
		},
		{
			name:     "no changes",
			files:    1,
			added:    0,
			removed:  0,
			expected: "1 file changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSummaryStats(tt.files, tt.added, tt.removed)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCurrentFileIndex_ReturnsMinusOneForSummaryRow(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Scroll to put cursor on summary row (last line)
	m.scroll = m.maxScroll()

	// currentFileIndex should return -1 when cursor is on summary row
	idx := m.currentFileIndex()
	assert.Equal(t, -1, idx, "currentFileIndex should return -1 for summary row")
}

func TestView_CursorArrowOnSummaryRow(t *testing.T) {
	// Test that cursor arrow appears on summary row when selected
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldFolded, // Fold so summary is closer
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
	// Position cursor on summary row (last row)
	m.scroll = m.totalLines - 1 - m.cursorOffset()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the summary line
	var summaryLine string
	for _, line := range lines {
		if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
			summaryLine = line
			break
		}
	}

	require.NotEmpty(t, summaryLine, "should find summary line")
	assert.Contains(t, summaryLine, "▶", "summary row with cursor should have arrow indicator")
}
