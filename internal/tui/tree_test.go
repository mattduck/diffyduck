package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestRenderTreeContinuation(t *testing.T) {
	// Create a simple style that renders text as-is (no ANSI codes)
	plainStyle := lipgloss.NewStyle()

	tests := []struct {
		name      string
		ancestors []TreeLevel
		want      string
	}{
		{
			name:      "empty ancestors",
			ancestors: nil,
			want:      "",
		},
		{
			name: "single non-last",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0},
			},
			want: "│    ", // │ + 4 spaces = 5 chars
		},
		{
			name: "single last",
			ancestors: []TreeLevel{
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 0},
			},
			want: "     ", // 5 spaces (no continuation for last)
		},
		{
			name: "single folded",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: true, Style: plainStyle, Depth: 0},
			},
			want: "     ", // 5 spaces (no continuation for folded)
		},
		{
			name: "two levels, neither last",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0},
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			want: "│    │    ", // 10 chars total
		},
		{
			name: "two levels, first non-last, second last",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0},
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			want: "│         ", // │ + 4 spaces + 5 spaces
		},
		{
			name: "two levels, first last, second non-last",
			ancestors: []TreeLevel{
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 0},
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			want: "     │    ", // 5 spaces + │ + 4 spaces
		},
		{
			name: "three levels for hunk support",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1}, // file
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 2},  // hunk (last)
			},
			want: "│    │         ", // 15 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTreeContinuation(tt.ancestors)
			if got != tt.want {
				t.Errorf("renderTreeContinuation() = %q (len=%d), want %q (len=%d)",
					got, len(got), tt.want, len(tt.want))
			}
		})
	}
}

func TestRenderTreeBranch(t *testing.T) {
	plainStyle := lipgloss.NewStyle()

	tests := []struct {
		name  string
		level TreeLevel
		want  string
	}{
		{
			name:  "non-last branch",
			level: TreeLevel{IsLast: false, Style: plainStyle},
			want:  "├━", // uses heavy horizontal line
		},
		{
			name:  "last branch",
			level: TreeLevel{IsLast: true, Style: plainStyle},
			want:  "└━", // uses heavy horizontal line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTreeBranch(tt.level)
			if got != tt.want {
				t.Errorf("renderTreeBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTreePrefix(t *testing.T) {
	plainStyle := lipgloss.NewStyle()

	tests := []struct {
		name     string
		path     TreePath
		isHeader bool
		want     string
	}{
		{
			name:     "empty path, content row",
			path:     TreePath{},
			isHeader: false,
			want:     "   ", // margin(1) + contentIndent(2) = 3 chars
		},
		{
			name: "file header (depth 1)",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
				},
				Current: &TreeLevel{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			isHeader: true,
			want:     " │    ├━", // margin(1) + continuation(5) + branch(2) = 8 chars
		},
		{
			name: "last file header",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
				},
				Current: &TreeLevel{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			isHeader: true,
			want:     " │    └━", // margin(1) + continuation(5) + branch(2)
		},
		{
			name: "content row under non-last file",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit (outer)
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1}, // file (innermost parent)
				},
				Current: nil,
			},
			isHeader: false,
			want:     " │    │      ", // margin(1) + innermost(│+4=5) + contentIndent(2)
		},
		{
			name: "content row under last file in non-last commit",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit (not last)
					{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 1},  // file (last in commit)
				},
				Current: nil,
			},
			isHeader: false,
			want:     " │           ", // margin(1) + outer(0) + innermost(5 spaces for last) + contentIndent(2)
		},
		{
			name: "content under folded parent",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
					{IsLast: false, IsFolded: true, Style: plainStyle, Depth: 1},  // file (folded)
				},
				Current: nil,
			},
			isHeader: false,
			want:     " │           ", // margin(1) + outer(0) + innermost(5 spaces for folded) + contentIndent(2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTreePrefix(tt.path, tt.isHeader)
			if got != tt.want {
				t.Errorf("renderTreePrefix() = %q (len=%d), want %q (len=%d)",
					got, len(got), tt.want, len(tt.want))
			}
		})
	}
}

// =============================================================================
// headerIsLast: last file uses ├ when content exists, └ when no content
// =============================================================================

// makeLogModeModel creates a log-mode model with one commit containing the given files.
func makeLogModeModel(files []sidebyside.FilePair) Model {
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123",
				Author:  "Test",
				Date:    "Mon Jan 1 00:00:00 2024 +0000",
				Subject: "Test commit",
			},
			FoldLevel:   sidebyside.CommitFileHeaders,
			FilesLoaded: true,
			Files:       files,
		},
	}
	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.focused = true
	m.RefreshLayout()
	return m
}

// findLastFileHeader returns the header displayRow for the last file in the model's rows.
func findLastFileHeader(rows []displayRow) *displayRow {
	var lastHeader *displayRow
	for i := range rows {
		if rows[i].isHeader && !rows[i].isCommitHeader {
			lastHeader = &rows[i]
		}
	}
	return lastHeader
}

// Test: Last file with hunk content uses ├ (not └) in log mode
func TestTree_LastFileHeader_WithHunkContent_UsesTBranch(t *testing.T) {
	m := makeLogModeModel([]sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHeader,
			Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "x"}, New: sidebyside.Line{Num: 1, Content: "y"}}},
		},
		{
			OldPath:   "a/last.go",
			NewPath:   "b/last.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added}}},
		},
	})

	rows := m.buildRows()
	header := findLastFileHeader(rows)
	require.NotNil(t, header, "should find last file header")
	require.NotNil(t, header.treePath.Current, "header should have Current tree level")
	assert.False(t, header.treePath.Current.IsLast,
		"last file with hunk content should use ├ (IsLast=false)")
}

// Test: Last file when folded with no preview uses └ (no content below)
func TestTree_LastFileHeader_FoldedNoPreview_UsesLBranch(t *testing.T) {
	m := makeLogModeModel([]sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHeader,
			Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "x"}, New: sidebyside.Line{Num: 1, Content: "y"}}},
		},
		{
			OldPath:   "a/last.go",
			NewPath:   "b/last.go",
			FoldLevel: sidebyside.FoldHeader,
			Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added}}},
		},
	})
	// No structureMaps set, so no structural diff preview

	rows := m.buildRows()
	header := findLastFileHeader(rows)
	require.NotNil(t, header, "should find last file header")
	require.NotNil(t, header.treePath.Current, "header should have Current tree level")
	// Fully collapsed last file uses └ — no content or terminator row follows.
	assert.True(t, header.treePath.Current.IsLast,
		"last file folded with no preview should use └ (IsLast=true)")
}

// Test: Last file with expanded content uses ├ in log mode
func TestTree_LastFileHeader_Expanded_UsesTBranch(t *testing.T) {
	m := makeLogModeModel([]sidebyside.FilePair{
		{
			OldPath:    "a/only.go",
			NewPath:    "b/only.go",
			FoldLevel:  sidebyside.FoldHunks,
			Pairs:      []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added}}},
			NewContent: []string{"new"},
		},
	})

	rows := m.buildRows()
	header := findLastFileHeader(rows)
	require.NotNil(t, header, "should find last file header")
	require.NotNil(t, header.treePath.Current, "header should have Current tree level")
	assert.False(t, header.treePath.Current.IsLast,
		"last file with expanded content should use ├ (IsLast=false)")
}

// Test: Non-last files always use ├ regardless of content
func TestTree_NonLastFileHeader_AlwaysUsesTBranch(t *testing.T) {
	m := makeLogModeModel([]sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHeader,
			Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "x"}, New: sidebyside.Line{Num: 1, Content: "y"}}},
		},
		{
			OldPath:   "a/last.go",
			NewPath:   "b/last.go",
			FoldLevel: sidebyside.FoldHeader,
			Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}}},
		},
	})

	rows := m.buildRows()
	// Find the FIRST file header
	for _, r := range rows {
		if r.isHeader && !r.isCommitHeader {
			require.NotNil(t, r.treePath.Current, "header should have Current tree level")
			assert.False(t, r.treePath.Current.IsLast,
				"non-last file should always use ├ (IsLast=false)")
			return
		}
	}
	t.Fatal("did not find first file header")
}

// Test: In diff mode (no commits), last file uses ├ since ╵ terminator follows
func TestTree_LastFileHeader_DiffMode_UsesTBranch(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/only.go",
				NewPath:   "b/only.go",
				FoldLevel: sidebyside.FoldHunks,
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added}}},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()
	header := findLastFileHeader(rows)
	require.NotNil(t, header, "should find last file header")
	require.NotNil(t, header.treePath.Current, "header should have Current tree level")
	assert.False(t, header.treePath.Current.IsLast,
		"last file should use ├ (IsLast=false) since ╵ terminator follows")
}

// Test: Content rows of last file in log mode show │ continuation
func TestTree_LastFileContentRows_LogMode_ShowContinuation(t *testing.T) {
	m := makeLogModeModel([]sidebyside.FilePair{
		{
			OldPath:   "a/only.go",
			NewPath:   "b/only.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added}}},
		},
	})

	rows := m.buildRows()
	for _, r := range rows {
		if r.kind == RowKindContent {
			require.Greater(t, len(r.treePath.Ancestors), 0,
				"content row should have tree ancestors")
			assert.False(t, r.treePath.Ancestors[0].IsLast,
				"last file's content row in log mode should have IsLast=false for │ continuation")
			return
		}
	}
	t.Fatal("did not find content row")
}

// Test: Content rows of last file in diff mode render │ faintly (no parent commit)
func TestTree_LastFileContentRows_DiffMode_Faint(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/only.go",
				NewPath:   "b/only.go",
				FoldLevel: sidebyside.FoldHunks,
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added}}},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()
	for _, r := range rows {
		if r.kind == RowKindContent {
			require.Greater(t, len(r.treePath.Ancestors), 0,
				"content row should have tree ancestors")
			assert.False(t, r.treePath.Ancestors[0].IsLast,
				"last file's content row in diff mode should have IsLast=false (terminator follows)")
			assert.True(t, r.treePath.Ancestors[0].Faint,
				"diff mode should render │ faintly (no parent commit)")
			return
		}
	}
	t.Fatal("did not find content row")
}

// Test: In diff mode with multiple folded files, last file uses └ with no terminator
func TestTree_DiffMode_MultiFile_LastFoldedUsesLBranch(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				FoldLevel: sidebyside.FoldHeader,
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "x"}, New: sidebyside.Line{Num: 1, Content: "y"}}},
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				FoldLevel: sidebyside.FoldHeader,
				Pairs:     []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}}},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// First header uses ├, last uses └ (fully collapsed, nothing below)
	var headers []displayRow
	for _, r := range rows {
		if r.isHeader {
			headers = append(headers, r)
		}
	}
	require.Len(t, headers, 2, "should have 2 file headers")
	assert.False(t, headers[0].treePath.Current.IsLast, "first file header should use ├")
	assert.True(t, headers[1].treePath.Current.IsLast, "last folded file header should use └")

	// No terminator row when last file is fully collapsed
	var terminators []displayRow
	for _, r := range rows {
		if r.treeTerminator {
			terminators = append(terminators, r)
		}
	}
	assert.Empty(t, terminators, "no terminator row when last file is fully collapsed")
}

func TestRenderEmptyTreeRow(t *testing.T) {
	plainStyle := lipgloss.NewStyle()

	t.Run("no ancestors, no cursor", func(t *testing.T) {
		path := TreePath{}
		got := renderEmptyTreeRow(path, false, true, false)
		assert.Equal(t, "", got, "empty tree path with no cursor should render nothing")
	})

	t.Run("no ancestors, cursor focused", func(t *testing.T) {
		path := TreePath{}
		got := renderEmptyTreeRow(path, true, true, false)
		assert.Contains(t, got, "▌", "cursor row should have arrow")
		assert.NotContains(t, got, "│", "no ancestors means no continuation")
	})

	t.Run("no ancestors, cursor unfocused", func(t *testing.T) {
		path := TreePath{}
		got := renderEmptyTreeRow(path, true, false, false)
		assert.NotContains(t, got, "▌", "unfocused cursor should be hidden (no block character)")
		assert.NotContains(t, got, "│", "no ancestors means no continuation")
	})

	t.Run("one ancestor non-last, no cursor", func(t *testing.T) {
		path := TreePath{
			Ancestors: []TreeLevel{{IsLast: false, Style: plainStyle}},
		}
		got := renderEmptyTreeRow(path, false, true, false)
		assert.Contains(t, got, "│", "non-last ancestor should show continuation")
	})

	t.Run("one ancestor non-last, cursor focused", func(t *testing.T) {
		path := TreePath{
			Ancestors: []TreeLevel{{IsLast: false, Style: plainStyle}},
		}
		got := renderEmptyTreeRow(path, true, true, false)
		assert.Contains(t, got, "▌", "cursor row should have arrow")
		assert.Contains(t, got, "│", "tree continuation must survive cursor rendering")
	})

	t.Run("one ancestor last, no cursor", func(t *testing.T) {
		path := TreePath{
			Ancestors: []TreeLevel{{IsLast: true, Style: plainStyle}},
		}
		got := renderEmptyTreeRow(path, false, true, false)
		assert.NotContains(t, got, "│", "last ancestor should not show continuation")
	})

	t.Run("one ancestor folded, no cursor", func(t *testing.T) {
		path := TreePath{
			Ancestors: []TreeLevel{{IsFolded: true, Style: plainStyle}},
		}
		got := renderEmptyTreeRow(path, false, true, false)
		assert.NotContains(t, got, "│", "folded ancestor should not show continuation")
	})

	t.Run("terminator renders ╵ instead of │", func(t *testing.T) {
		path := TreePath{
			Ancestors: []TreeLevel{{IsLast: false, Style: plainStyle}},
		}
		got := renderEmptyTreeRow(path, false, true, true)
		assert.Contains(t, got, "╵", "terminator should render ╵")
		assert.NotContains(t, got, "│", "terminator should not render │")
	})

	t.Run("terminator with cursor", func(t *testing.T) {
		path := TreePath{
			Ancestors: []TreeLevel{{IsLast: false, Style: plainStyle}},
		}
		got := renderEmptyTreeRow(path, true, true, true)
		assert.Contains(t, got, "▌", "cursor should show arrow")
		assert.Contains(t, got, "╵", "terminator with cursor should render ╵")
		assert.NotContains(t, got, "│", "terminator should not render │")
	})

	t.Run("no shading characters", func(t *testing.T) {
		// Verify empty rows never contain ░ shading
		path := TreePath{
			Ancestors: []TreeLevel{{IsLast: false, Style: plainStyle}},
		}
		for _, cursor := range []bool{true, false} {
			got := renderEmptyTreeRow(path, cursor, true, false)
			assert.NotContains(t, got, "░", "empty tree rows should never contain shading")
		}
	})
}

func TestTreeWidth(t *testing.T) {
	tests := []struct {
		name         string
		numAncestors int
		isHeader     bool
		want         int
	}{
		// Headers: margin(1) + outer*(5) + innermost(5) + branch(2)
		{"0 ancestors header (root)", 0, true, 3},  // margin(1) + branch(2) = 3
		{"1 ancestor header (file)", 1, true, 8},   // 1 + 5 + 2 = 8
		{"2 ancestors header (hunk)", 2, true, 13}, // 1 + 10 + 2 = 13
		{"3 ancestors header", 3, true, 18},        // 1 + 15 + 2 = 18

		// Content: margin(1) + ancestors*(5) + contentIndent(2)
		{"0 ancestors content", 0, false, 3},                 // margin(1) + contentIndent(2) = 3
		{"1 ancestor content (file content)", 1, false, 8},   // 1 + 5 + 2 = 8
		{"2 ancestors content", 2, false, 13},                // 1 + 10 + 2 = 13
		{"3 ancestors content (hunk content)", 3, false, 18}, // 1 + 15 + 2 = 18
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := treeWidth(tt.numAncestors, tt.isHeader)
			if got != tt.want {
				t.Errorf("treeWidth(%d, %v) = %d, want %d", tt.numAncestors, tt.isHeader, got, tt.want)
			}
		})
	}
}
