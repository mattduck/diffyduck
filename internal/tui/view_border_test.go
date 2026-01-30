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

// --- File header border connector tests ---

func TestFileHeader_UnfoldedHasTrailingConnector(t *testing.T) {
	// Unfolded file headers should end with ┏━━━ trailing fill to screen edge
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/hello.go", NewPath: "b/hello.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldNormal},
		{OldPath: "a/world.go", NewPath: "b/world.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldNormal},
	})
	m.width = 80
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	rows := m.buildRows()

	// Find an unfolded header row
	var headerRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindHeader && rows[i].headerMode == HeaderThreeLine {
			headerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, headerRow, "should find an unfolded header row")

	rendered := m.renderHeader(
		headerRow.header, headerRow.foldLevel, headerRow.headerMode,
		headerRow.status, headerRow.added, headerRow.removed,
		headerRow.headerBoxWidth, headerRow.fileIndex, 0, false, headerRow.treePath,
	)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "┏", "unfolded header should contain ┏ trailing connector")
	assert.True(t, strings.Contains(stripped, "┏") && strings.Contains(stripped, "━"),
		"unfolded header should have ┏━━━ trailing fill")
}

func TestFileHeader_FoldedHasNoTrailingConnector(t *testing.T) {
	// Folded file headers should NOT have ┏━━━ trailing fill
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/hello.go", NewPath: "b/hello.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldFolded},
	})
	m.width = 80
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	rows := m.buildRows()

	var headerRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindHeader {
			headerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, headerRow, "should find a header row")
	assert.Equal(t, HeaderSingleLine, headerRow.headerMode, "folded header should be single-line")

	rendered := m.renderHeader(
		headerRow.header, headerRow.foldLevel, headerRow.headerMode,
		headerRow.status, headerRow.added, headerRow.removed,
		headerRow.headerBoxWidth, headerRow.fileIndex, 0, false, headerRow.treePath,
	)
	stripped := stripANSI(rendered)

	assert.NotContains(t, stripped, "┏", "folded header should not contain ┏ trailing connector")
}

func TestFileBottomBorder_HasClosingCorner(t *testing.T) {
	// Unfolded file bottom border should end with ┛
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/one.go", NewPath: "b/one.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldNormal},
		{OldPath: "a/two.go", NewPath: "b/two.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldNormal},
	})
	m.width = 80
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	rows := m.buildRows()

	var spacerRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindHeaderSpacer && rows[i].headerMode == HeaderThreeLine {
			spacerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, spacerRow, "should find an unfolded bottom border row")

	rendered := m.renderHeaderBottomBorder(
		spacerRow.headerBoxWidth, spacerRow.headerMode, spacerRow.status,
		false, spacerRow.treePrefixWidth, spacerRow.treePath,
	)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "┛", "bottom border should end with ┛ closing corner")
	assert.Contains(t, stripped, "┗", "bottom border should contain ┗ left corner")
	// ┛ should be the last non-space character
	trimmed := strings.TrimRight(stripped, " ")
	assert.True(t, strings.HasSuffix(trimmed, "┛"),
		"┛ should be the last character, got: %q", trimmed[max(0, len(trimmed)-10):])
}

func TestFileHeader_ConnectorAlignsBetweenHeaderAndBorder(t *testing.T) {
	// The ┏ on the header line and ┛ on the bottom border should be vertically aligned
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/one.go", NewPath: "b/one.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldNormal},
		{OldPath: "a/two.go", NewPath: "b/two.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldNormal},
	})
	m.width = 100
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	rows := m.buildRows()

	// Find header and bottom border for the same file
	var headerRow, spacerRow *displayRow
	for i := range rows {
		if rows[i].fileIndex == 1 && rows[i].kind == RowKindHeader {
			headerRow = &rows[i]
		}
		if rows[i].fileIndex == 1 && rows[i].kind == RowKindHeaderSpacer {
			spacerRow = &rows[i]
		}
	}
	require.NotNil(t, headerRow, "should find header for file 1")
	require.NotNil(t, spacerRow, "should find bottom border for file 1")

	header := stripANSI(m.renderHeader(
		headerRow.header, headerRow.foldLevel, headerRow.headerMode,
		headerRow.status, headerRow.added, headerRow.removed,
		headerRow.headerBoxWidth, headerRow.fileIndex, 0, false, headerRow.treePath,
	))
	border := stripANSI(m.renderHeaderBottomBorder(
		spacerRow.headerBoxWidth, spacerRow.headerMode, spacerRow.status,
		false, spacerRow.treePrefixWidth, spacerRow.treePath,
	))

	// ┏ position on header should equal ┛ position on border
	headerConnectorPos := findRuneIndex(header, "┏")
	borderConnectorPos := findRuneIndex(border, "┛")

	require.NotEqual(t, -1, headerConnectorPos, "header should contain ┏")
	require.NotEqual(t, -1, borderConnectorPos, "border should contain ┛")
	assert.Equal(t, headerConnectorPos, borderConnectorPos,
		"┏ (col %d) and ┛ (col %d) should be vertically aligned", headerConnectorPos, borderConnectorPos)
}

// --- Commit header border connector tests ---

func TestCommitHeader_UnfoldedHasTrailingConnector(t *testing.T) {
	// Unfolded commit headers should end with ╔═══ trailing fill
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
		commitSpec{sha: "def5678", subject: "Fix bug Y", author: "Bob", date: "2024-01-02", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var commitHeaderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeader && rows[i].commitIndex == 0 && rows[i].headerMode == HeaderThreeLine {
			commitHeaderRow = &rows[i]
			break
		}
	}
	require.NotNil(t, commitHeaderRow, "should find unfolded commit header")

	rendered := m.renderCommitHeaderRow(*commitHeaderRow, false)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "╔", "unfolded commit header should contain ╔ trailing connector")
	assert.Contains(t, stripped, "═", "unfolded commit header should have ═ trailing fill")
}

func TestCommitHeader_FoldedHasNoTrailingConnector(t *testing.T) {
	// Folded commit headers should NOT have ╔═══ trailing fill
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFolded
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var commitHeaderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeader && rows[i].commitIndex == 0 {
			commitHeaderRow = &rows[i]
			break
		}
	}
	require.NotNil(t, commitHeaderRow, "should find folded commit header")
	assert.Equal(t, HeaderSingleLine, commitHeaderRow.headerMode, "folded commit should be single-line")

	rendered := m.renderCommitHeaderRow(*commitHeaderRow, false)
	stripped := stripANSI(rendered)

	assert.NotContains(t, stripped, "╔", "folded commit header should not contain ╔ trailing connector")
}

func TestCommitBottomBorder_HasClosingCorner(t *testing.T) {
	// Unfolded commit bottom border should end with ╝
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var borderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeaderBottomBorder {
			borderRow = &rows[i]
			break
		}
	}
	require.NotNil(t, borderRow, "should find commit bottom border row")

	rendered := m.renderCommitHeaderBottomBorder(*borderRow, false)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "╝", "commit bottom border should end with ╝")
	assert.Contains(t, stripped, "╞", "commit bottom border should start with ╞ connector")
	trimmed := strings.TrimRight(stripped, " ")
	assert.True(t, strings.HasSuffix(trimmed, "╝"),
		"╝ should be the last character, got: %q", trimmed[max(0, len(trimmed)-10):])
}

func TestCommitHeader_ConnectorAlignsBetweenHeaderAndBorder(t *testing.T) {
	// The ╔ on the commit header and ╝ on the bottom border should be vertically aligned
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 2},
	)
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var headerRow, borderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeader && rows[i].commitIndex == 0 {
			headerRow = &rows[i]
		}
		if rows[i].kind == RowKindCommitHeaderBottomBorder {
			borderRow = &rows[i]
		}
	}
	require.NotNil(t, headerRow, "should find commit header")
	require.NotNil(t, borderRow, "should find commit bottom border")

	header := stripANSI(m.renderCommitHeaderRow(*headerRow, false))
	border := stripANSI(m.renderCommitHeaderBottomBorder(*borderRow, false))

	headerPos := findRuneIndex(header, "╔")
	borderPos := findRuneIndex(border, "╝")

	require.NotEqual(t, -1, headerPos, "header should contain ╔")
	require.NotEqual(t, -1, borderPos, "border should contain ╝")
	assert.Equal(t, headerPos, borderPos,
		"╔ (col %d) and ╝ (col %d) should be vertically aligned", headerPos, borderPos)
}

// --- Width calculation tests ---

func TestCommitHeader_TruncatedSubjectAlignsWithBorder(t *testing.T) {
	// When the subject is longer than maxCommitSubjectWidth (cached at 60 by default),
	// the render truncates it with "..." but headerBoxWidth should match the truncated width.
	// This was the bug fixed by capping subjectWidth to maxCommitSubjectWidth in buildRows.
	lipgloss.SetColorProfile(termenv.Ascii)

	longSubject := "feat: add tree prefix to comment boxes and use double-sided borders for all headers"
	require.Greater(t, displayWidth(longSubject), 60, "subject should be longer than default cachedCommitSubjWidth")

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: longSubject, author: "Alice", date: "2024-01-01", fileCount: 3},
	)
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.width = 120
	m.rowsCacheValid = false

	rows := m.buildRows()

	var headerRow, borderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeader && rows[i].commitIndex == 0 {
			headerRow = &rows[i]
		}
		if rows[i].kind == RowKindCommitHeaderBottomBorder {
			borderRow = &rows[i]
		}
	}
	require.NotNil(t, headerRow, "should find commit header")
	require.NotNil(t, borderRow, "should find commit bottom border")

	header := stripANSI(m.renderCommitHeaderRow(*headerRow, false))
	border := stripANSI(m.renderCommitHeaderBottomBorder(*borderRow, false))

	// If subject is truncated, ╔ and ╝ should still align
	headerPos := findRuneIndex(header, "╔")
	borderPos := findRuneIndex(border, "╝")

	if headerPos >= 0 && borderPos >= 0 {
		assert.Equal(t, headerPos, borderPos,
			"╔ (col %d) and ╝ (col %d) should align even with truncated subject", headerPos, borderPos)
	}
}

func TestFileHeaderBoxWidth_MatchesRenderedWidth(t *testing.T) {
	// fileHeaderBoxWidth should accurately predict the rendered header content width
	tests := []struct {
		name    string
		header  string
		added   int
		removed int
	}{
		{"simple file", "hello.go", 10, 5},
		{"no changes", "empty.go", 0, 0},
		{"large stats", "big.go", 9999, 1234},
		{"long filename", "internal/tui/view_commit_border_alignment.go", 42, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := fileHeaderBoxWidth(tt.header, tt.added, tt.removed)
			assert.Greater(t, width, 0, "fileHeaderBoxWidth should be positive")
			// Width should include the header text display width
			assert.GreaterOrEqual(t, width, displayWidth(tt.header),
				"box width should be at least as wide as the header text")
		})
	}
}

func TestFileHeader_UnfoldedUsesPerFileWidth(t *testing.T) {
	// When unfolded, each file's headerBoxWidth should reflect its own content,
	// not the shared max width used for folded headers
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{
			OldPath: "a/short.go", NewPath: "b/short.go",
			Pairs: makePairsN(2), FoldLevel: sidebyside.FoldNormal,
		},
		{
			OldPath: "a/very_long_filename_that_is_much_wider.go",
			NewPath: "b/very_long_filename_that_is_much_wider.go",
			Pairs:   makePairsN(2), FoldLevel: sidebyside.FoldNormal,
		},
	})
	m.width = 120
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	rows := m.buildRows()

	// Find headers for both files
	var header0, header1 *displayRow
	for i := range rows {
		if rows[i].kind == RowKindHeader {
			if rows[i].fileIndex == 0 {
				header0 = &rows[i]
			} else if rows[i].fileIndex == 1 {
				header1 = &rows[i]
			}
		}
	}
	require.NotNil(t, header0, "should find header for file 0")
	require.NotNil(t, header1, "should find header for file 1")

	// When both are unfolded (HeaderThreeLine), they should have different widths
	if header0.headerMode == HeaderThreeLine && header1.headerMode == HeaderThreeLine {
		assert.Less(t, header0.headerBoxWidth, header1.headerBoxWidth,
			"shorter filename should have smaller headerBoxWidth when unfolded")
	}
}

func TestCommitHeader_UnfoldedUsesPerCommitSubjectWidth(t *testing.T) {
	// When unfolded, commit headerBoxWidth should use the per-commit subject width,
	// not the shared max subject width
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Short", author: "Alice", date: "2024-01-01", fileCount: 1},
		commitSpec{sha: "def5678", subject: "A much longer commit subject line here", author: "Bob", date: "2024-01-02", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.commits[1].FoldLevel = sidebyside.CommitNormal
	m.width = 120
	m.rowsCacheValid = false

	rows := m.buildRows()

	var header0, header1 *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeader {
			if rows[i].commitIndex == 0 {
				header0 = &rows[i]
			} else if rows[i].commitIndex == 1 {
				header1 = &rows[i]
			}
		}
	}
	require.NotNil(t, header0, "should find commit header 0")
	require.NotNil(t, header1, "should find commit header 1")

	// Both unfolded: shorter subject should have smaller headerBoxWidth
	assert.Less(t, header0.headerBoxWidth, header1.headerBoxWidth,
		"shorter subject commit should have smaller headerBoxWidth when unfolded")
}

// --- Commit info header border connector tests ---

func TestCommitInfoHeader_ExpandedHasTrailingConnector(t *testing.T) {
	// Expanded commit info header ("details") should have ┏━━━ trailing fill
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitExpanded
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var infoHeaderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitInfoHeader {
			infoHeaderRow = &rows[i]
			break
		}
	}
	require.NotNil(t, infoHeaderRow, "should find commit info header row")
	assert.Equal(t, HeaderThreeLine, infoHeaderRow.headerMode,
		"expanded commit info should have HeaderThreeLine mode")

	rendered := m.renderCommitInfoHeader(*infoHeaderRow, false)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "Jan 1st", "commit info header should contain date text")
	assert.Contains(t, stripped, "●", "expanded commit info should show ● fold icon")
	assert.Contains(t, stripped, "┏", "expanded commit info header should contain ┏ trailing connector")
}

func TestCommitInfoHeader_NormalHasNoTrailingConnector(t *testing.T) {
	// CommitNormal info header should NOT have ┏ trailing fill (info is folded)
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitNormal
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var infoHeaderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitInfoHeader {
			infoHeaderRow = &rows[i]
			break
		}
	}
	require.NotNil(t, infoHeaderRow, "should find commit info header row")
	assert.Equal(t, HeaderSingleLine, infoHeaderRow.headerMode,
		"normal commit info should have HeaderSingleLine mode")

	rendered := m.renderCommitInfoHeader(*infoHeaderRow, false)
	stripped := stripANSI(rendered)

	assert.NotContains(t, stripped, "┏", "normal commit info header should not contain ┏")
}

func TestCommitInfoHeader_ConnectorAlignsBetweenHeaderAndBorder(t *testing.T) {
	// The ┏ on the info header and ┛ on its bottom border should be vertically aligned
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitExpanded
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var infoHeaderRow, infoBorderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitInfoHeader {
			infoHeaderRow = &rows[i]
		}
		if rows[i].kind == RowKindCommitInfoBottomBorder {
			infoBorderRow = &rows[i]
		}
	}
	require.NotNil(t, infoHeaderRow, "should find commit info header")
	require.NotNil(t, infoBorderRow, "should find commit info bottom border")

	header := stripANSI(m.renderCommitInfoHeader(*infoHeaderRow, false))
	border := stripANSI(m.renderCommitInfoBottomBorder(*infoBorderRow, false))

	headerPos := findRuneIndex(header, "┏")
	borderPos := findRuneIndex(border, "┛")

	require.NotEqual(t, -1, headerPos, "info header should contain ┏")
	require.NotEqual(t, -1, borderPos, "info border should contain ┛")
	assert.Equal(t, headerPos, borderPos,
		"┏ (col %d) and ┛ (col %d) should be vertically aligned", headerPos, borderPos)
}

func TestCommitInfoBottomBorder_HasClosingCorner(t *testing.T) {
	// Expanded commit info bottom border should end with ┛
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitExpanded
	m.width = 100
	m.rowsCacheValid = false

	rows := m.buildRows()

	var infoBorderRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitInfoBottomBorder {
			infoBorderRow = &rows[i]
			break
		}
	}
	require.NotNil(t, infoBorderRow, "should find commit info bottom border row")

	rendered := m.renderCommitInfoBottomBorder(*infoBorderRow, false)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "┛", "commit info bottom border should end with ┛")
	trimmed := strings.TrimRight(stripped, " ")
	assert.True(t, strings.HasSuffix(trimmed, "┛"),
		"┛ should be the last character, got: %q", trimmed[max(0, len(trimmed)-10):])
}

// --- Helpers ---

// makePairsN creates n line pairs for test files
func makePairsN(n int) []sidebyside.LinePair {
	pairs := make([]sidebyside.LinePair, n)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
			New: sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
		}
	}
	return pairs
}

type commitSpec struct {
	sha       string
	subject   string
	author    string
	date      string
	fileCount int
}

// makeCommitModel creates a Model with the given commit specs, each with dummy files.
// Sets StatsLoaded with non-zero stats so width calculations are consistent.
func makeCommitModel(specs ...commitSpec) Model {
	var commits []sidebyside.CommitSet
	var allFiles []sidebyside.FilePair
	fileStarts := make([]int, len(specs))

	for i, spec := range specs {
		fileStarts[i] = len(allFiles)
		commits = append(commits, sidebyside.CommitSet{
			Info: sidebyside.CommitInfo{
				SHA:     spec.sha,
				Subject: spec.subject,
				Author:  spec.author,
				Date:    spec.date,
			},
			FoldLevel:    sidebyside.CommitFolded,
			StatsLoaded:  true,
			TotalAdded:   10 * (spec.fileCount + 1),
			TotalRemoved: 5 * (spec.fileCount + 1),
		})
		for j := 0; j < spec.fileCount; j++ {
			allFiles = append(allFiles, sidebyside.FilePair{
				OldPath:   "a/file.go",
				NewPath:   "b/file.go",
				Pairs:     makePairsN(2),
				FoldLevel: sidebyside.FoldFolded,
			})
		}
	}

	m := Model{
		commits:          commits,
		commitFileStarts: fileStarts,
		files:            allFiles,
		width:            100,
		height:           40,
		keys:             DefaultKeyMap(),
		initialFoldSet:   true,
		focused:          true,
	}
	m.calculateTotalLines()
	return m
}
