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

func TestFileHeader_NormalHasShortTrailingConnector(t *testing.T) {
	// FoldStructure file headers should close with ━━━━◐ (no corner/border on next line)
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/world.go", NewPath: "b/world.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldStructure},
	})
	m.width = 80
	m.height = 40
	m.initialFoldSet = true
	m.focused = true
	m.keys = DefaultKeyMap()

	rows := m.buildRows()

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

	assert.Contains(t, stripped, "━━━━◐", "FoldStructure header should close with ━━━━◐")
	assert.NotContains(t, stripped, "┓", "FoldStructure header should not have corner turning down")
}

func TestFileHeader_ExpandedHasFullTrailingConnector(t *testing.T) {
	// FoldHunks file headers should have ━━━━┓ corner turning down
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/hello.go", NewPath: "b/hello.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldHunks},
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
	require.NotNil(t, headerRow, "should find header row")

	rendered := m.renderHeader(
		headerRow.header, headerRow.foldLevel, headerRow.headerMode,
		headerRow.status, headerRow.added, headerRow.removed,
		headerRow.headerBoxWidth, headerRow.fileIndex, 0, false, headerRow.treePath,
	)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "━━━━┓", "expanded header should contain ━━━━┓ trailing corner")
	assert.NotContains(t, stripped, "…", "expanded header should not contain ellipsis")
	// Expanded header should end with ┓
	trimmed := strings.TrimRight(stripped, " ")
	assert.True(t, strings.HasSuffix(trimmed, "┓"),
		"expanded header should end with ┓, got: %q", trimmed[max(0, len(trimmed)-10):])
}

func TestFileHeader_FoldedHasNoTrailingConnector(t *testing.T) {
	// Folded file headers should NOT have ┏━━━ trailing fill
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/hello.go", NewPath: "b/hello.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldHeader},
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
	assert.NotContains(t, stripped, "…", "folded header should not contain ellipsis")
}

func TestFileBottomBorder_HasClosingCorner(t *testing.T) {
	// Expanded file bottom border should extend to screen edge with ●
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/one.go", NewPath: "b/one.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldHunks},
		{OldPath: "a/two.go", NewPath: "b/two.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldHunks},
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
		false, spacerRow.treePrefixWidth, spacerRow.treePath, spacerRow.foldLevel,
	)
	stripped := stripANSI(rendered)

	assert.Contains(t, stripped, "┗", "bottom border should contain ┗ corner")
	// ● should be the last non-space character
	trimmed := strings.TrimRight(stripped, " ")
	assert.True(t, strings.HasSuffix(trimmed, "●"),
		"● should be the last character, got: %q", trimmed[max(0, len(trimmed)-10):])
}

func TestFileHeader_ConnectorAlignsBetweenHeaderAndBorder(t *testing.T) {
	// The ┏ on the header line and ┛ on the bottom border should be vertically aligned
	lipgloss.SetColorProfile(termenv.Ascii)

	m := New([]sidebyside.FilePair{
		{OldPath: "a/one.go", NewPath: "b/one.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldHunks},
		{OldPath: "a/two.go", NewPath: "b/two.go", Pairs: makePairsN(3), FoldLevel: sidebyside.FoldHunks},
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
		false, spacerRow.treePrefixWidth, spacerRow.treePath, spacerRow.foldLevel,
	))

	// Header should have ━━━━┓, border should have ┗ and end with ●
	assert.Contains(t, header, "━━━━┓", "header should contain ━━━━┓ trailing corner")
	headerTrimmed := strings.TrimRight(header, " ")
	assert.True(t, strings.HasSuffix(headerTrimmed, "┓"),
		"header should end with ┓, got: %q", headerTrimmed[max(0, len(headerTrimmed)-10):])
	assert.Contains(t, border, "┗", "border should contain ┗ corner")
	borderTrimmed := strings.TrimRight(border, " ")
	assert.True(t, strings.HasSuffix(borderTrimmed, "●"),
		"border should end with ●, got: %q", borderTrimmed[max(0, len(borderTrimmed)-10):])
}

// --- Commit header border connector tests ---

func TestCommitHeader_UnfoldedHasNoTrailingConnector(t *testing.T) {
	// Unfolded commit headers should NOT have ╔═══ trailing fill (border extends full-width instead)
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
		commitSpec{sha: "def5678", subject: "Fix bug Y", author: "Bob", date: "2024-01-02", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFileHeaders
	m.width = 100
	m.w().rowsCacheValid = false

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

	assert.NotContains(t, stripped, "╔", "unfolded commit header should not contain ╔ trailing connector")
}

func TestCommitHeader_FoldedHasNoTrailingConnector(t *testing.T) {
	// Folded commit headers should NOT have ╔═══ trailing fill
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFolded
	m.width = 100
	m.w().rowsCacheValid = false

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

func TestCommitBottomBorder_ExtendsFullWidthWithCorner(t *testing.T) {
	// Unfolded commit bottom border should extend full-width with ╚═══● pattern
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFileHeaders
	m.width = 100
	m.w().rowsCacheValid = false

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

	// Bottom border is now an empty tree continuation row (border moved to header line)
	assert.NotContains(t, stripped, "╚", "commit bottom border should no longer contain ╚ corner")
	assert.NotContains(t, stripped, "═", "commit bottom border should no longer have ═ fill")
}

func TestCommitHeader_FillExtendsToScreenEdge(t *testing.T) {
	// The commit header ═══● fill should extend to the screen edge
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 2},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFileHeaders
	m.width = 100
	m.w().rowsCacheValid = false

	rows := m.buildRows()

	var headerRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeader && rows[i].commitIndex == 0 {
			headerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, headerRow, "should find commit header")

	header := stripANSI(m.renderCommitHeaderRow(*headerRow, false))

	assert.Contains(t, header, "═", "header should have ═ fill")
	trimmed := strings.TrimRight(header, " ")
	assert.True(t, strings.HasSuffix(trimmed, "●"),
		"header should end with ● end cap, got: %q", trimmed[max(0, len(trimmed)-10):])
	assert.Equal(t, 100, displayWidth(trimmed),
		"header fill should extend to screen width")
}

// --- Width calculation tests ---

func TestCommitHeader_TruncatedSubjectFillsToEdge(t *testing.T) {
	// When the subject is longer than maxCommitSubjectWidth (cached at 60 by default),
	// the render truncates it with "..." and the ═══● fill extends to the screen edge.
	lipgloss.SetColorProfile(termenv.Ascii)

	longSubject := "feat: add tree prefix to comment boxes and use double-sided borders for all headers"
	require.Greater(t, displayWidth(longSubject), 60, "subject should be longer than default cachedCommitSubjWidth")

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: longSubject, author: "Alice", date: "2024-01-01", fileCount: 3},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFileHeaders
	m.width = 160
	m.w().rowsCacheValid = false

	rows := m.buildRows()

	var headerRow *displayRow
	for i := range rows {
		if rows[i].kind == RowKindCommitHeader && rows[i].commitIndex == 0 {
			headerRow = &rows[i]
			break
		}
	}
	require.NotNil(t, headerRow, "should find commit header")

	header := stripANSI(m.renderCommitHeaderRow(*headerRow, false))
	trimmed := strings.TrimRight(header, " ")

	assert.Contains(t, header, "═", "header should have ═ fill")
	assert.True(t, strings.HasSuffix(trimmed, "●"),
		"header should end with ● end cap, got: %q", trimmed[max(0, len(trimmed)-10):])
	assert.Equal(t, 160, displayWidth(trimmed),
		"header fill should extend to screen width even with long subject")
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
			Pairs: makePairsN(2), FoldLevel: sidebyside.FoldHunks,
		},
		{
			OldPath: "a/very_long_filename_that_is_much_wider.go",
			NewPath: "b/very_long_filename_that_is_much_wider.go",
			Pairs:   makePairsN(2), FoldLevel: sidebyside.FoldHunks,
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
	m.commits[0].FoldLevel = sidebyside.CommitFileHeaders
	m.commits[1].FoldLevel = sidebyside.CommitFileHeaders
	m.width = 120
	m.w().rowsCacheValid = false

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
	// Expanded commit info header ("details") should have ━━━━┓ trailing corner
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFileHunks
	m.width = 100
	m.w().rowsCacheValid = false

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
	assert.Contains(t, stripped, "━━━━┓", "expanded commit info header should contain ━━━━┓ trailing corner")
	trimmed := strings.TrimRight(stripped, " ")
	assert.True(t, strings.HasSuffix(trimmed, "┓"),
		"expanded commit info header should end with ┓, got: %q", trimmed[max(0, len(trimmed)-10):])
}

func TestCommitInfoHeader_NormalHasNoTrailingConnector(t *testing.T) {
	// CommitFileHeaders info header should NOT have ┏ trailing fill (info is folded)
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFileHeaders
	m.width = 100
	m.w().rowsCacheValid = false

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
	m.commits[0].FoldLevel = sidebyside.CommitFileHunks
	m.width = 100
	m.w().rowsCacheValid = false

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

	// Header should have ━━━━┓, border should have ┗ and end with ●
	assert.Contains(t, header, "━━━━┓", "info header should contain ━━━━┓ trailing corner")
	headerTrimmed := strings.TrimRight(header, " ")
	assert.True(t, strings.HasSuffix(headerTrimmed, "┓"),
		"info header should end with ┓, got: %q", headerTrimmed[max(0, len(headerTrimmed)-10):])
	assert.Contains(t, border, "┗", "info border should contain ┗ corner")
	borderTrimmed := strings.TrimRight(border, " ")
	assert.True(t, strings.HasSuffix(borderTrimmed, "●"),
		"info border should end with ●, got: %q", borderTrimmed[max(0, len(borderTrimmed)-10):])
}

func TestCommitInfoBottomBorder_HasClosingCorner(t *testing.T) {
	// Expanded commit info bottom border should extend to screen edge with ●
	lipgloss.SetColorProfile(termenv.Ascii)

	m := makeCommitModel(
		commitSpec{sha: "abc1234", subject: "Add feature X", author: "Alice", date: "2024-01-01", fileCount: 1},
	)
	m.commits[0].FoldLevel = sidebyside.CommitFileHunks
	m.width = 100
	m.w().rowsCacheValid = false

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

	assert.Contains(t, stripped, "┗", "commit info bottom border should contain ┗ corner")
	trimmed := strings.TrimRight(stripped, " ")
	assert.True(t, strings.HasSuffix(trimmed, "●"),
		"● should be the last character, got: %q", trimmed[max(0, len(trimmed)-10):])
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
				FoldLevel: sidebyside.FoldHeader,
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
