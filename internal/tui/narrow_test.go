package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestNarrowScope_IncludesCommit(t *testing.T) {
	tests := []struct {
		name      string
		scope     NarrowScope
		commitIdx int
		want      bool
	}{
		{
			name:      "inactive scope includes all",
			scope:     NarrowScope{Active: false},
			commitIdx: 5,
			want:      true,
		},
		{
			name:      "commit scope includes matching commit",
			scope:     NarrowScope{Active: true, CommitIdx: 2, FileIdx: -1},
			commitIdx: 2,
			want:      true,
		},
		{
			name:      "commit scope excludes non-matching commit",
			scope:     NarrowScope{Active: true, CommitIdx: 2, FileIdx: -1},
			commitIdx: 3,
			want:      false,
		},
		{
			name:      "file scope includes the file's commit",
			scope:     NarrowScope{Active: true, CommitIdx: 1, FileIdx: 5},
			commitIdx: 1,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.IncludesCommit(tt.commitIdx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNarrowScope_IncludesFile(t *testing.T) {
	tests := []struct {
		name    string
		scope   NarrowScope
		fileIdx int
		want    bool
	}{
		{
			name:    "inactive scope includes all",
			scope:   NarrowScope{Active: false},
			fileIdx: 5,
			want:    true,
		},
		{
			name:    "commit scope includes all files",
			scope:   NarrowScope{Active: true, CommitIdx: 2, FileIdx: -1},
			fileIdx: 5,
			want:    true,
		},
		{
			name:    "file scope includes matching file",
			scope:   NarrowScope{Active: true, CommitIdx: 1, FileIdx: 3},
			fileIdx: 3,
			want:    true,
		},
		{
			name:    "file scope excludes non-matching file",
			scope:   NarrowScope{Active: true, CommitIdx: 1, FileIdx: 3},
			fileIdx: 4,
			want:    false,
		},
		{
			name:    "commit-info-only excludes all files",
			scope:   NarrowScope{Active: true, CommitIdx: 0, FileIdx: -1, CommitInfoOnly: true},
			fileIdx: 0,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.IncludesFile(tt.fileIdx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNarrowScope_IsCommitInfoOnly(t *testing.T) {
	tests := []struct {
		name  string
		scope NarrowScope
		want  bool
	}{
		{
			name:  "inactive scope is not commit-info-only",
			scope: NarrowScope{Active: false, CommitInfoOnly: true},
			want:  false,
		},
		{
			name:  "active commit scope without flag is not commit-info-only",
			scope: NarrowScope{Active: true, CommitIdx: 0, FileIdx: -1, CommitInfoOnly: false},
			want:  false,
		},
		{
			name:  "active commit scope with flag is commit-info-only",
			scope: NarrowScope{Active: true, CommitIdx: 0, FileIdx: -1, CommitInfoOnly: true},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.IsCommitInfoOnly()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildRows_NarrowToFile(t *testing.T) {
	// Create a model with 2 files
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
				{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
			},
		},
		{
			OldPath:   "a/second.go",
			NewPath:   "b/second.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "other1"}, New: sidebyside.Line{Num: 1, Content: "other1"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Count rows without narrow scope
	fullRows := m.buildRows()
	fullRowCount := len(fullRows)

	// Now narrow to the first file only
	m.w().narrow = NarrowScope{
		Active:    true,
		CommitIdx: -1, // legacy mode has no commits
		FileIdx:   0,
		HunkIdx:   -1,
	}

	narrowRows := m.buildRows()
	narrowRowCount := len(narrowRows)

	// Narrowed view should have fewer rows
	require.Less(t, narrowRowCount, fullRowCount, "narrowed view should have fewer rows (full=%d, narrow=%d)", fullRowCount, narrowRowCount)

	// All rows in narrowed view should belong to file 0
	for i, row := range narrowRows {
		if row.fileIndex >= 0 {
			assert.Equal(t, 0, row.fileIndex, "row %d (kind=%v) should belong to file 0, got file %d", i, row.kind, row.fileIndex)
		}
	}
}

func TestToggleNarrow_EnterAndExit(t *testing.T) {
	// Create a model with 2 files
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
				{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
			},
		},
		{
			OldPath:   "a/second.go",
			NewPath:   "b/second.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "other1"}, New: sidebyside.Line{Num: 1, Content: "other1"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Move scroll to a file row (skip header, go to first content)
	m.w().scroll = 2 // should be on first file's content
	initialScroll := m.w().scroll
	initialTotalLines := m.w().totalLines

	// Verify not in narrow mode
	assert.False(t, m.w().narrow.Active)

	// Toggle narrow mode (enter)
	m.toggleNarrow()

	// Should now be in narrow mode
	assert.True(t, m.w().narrow.Active, "should be in narrow mode after toggle")
	assert.Equal(t, 0, m.w().narrow.FileIdx, "should be narrowed to file 0")
	// Cursor should stay on the same content row (not reset to 0)
	assert.GreaterOrEqual(t, m.w().scroll, 0, "scroll should be valid in narrow mode")
	assert.Less(t, m.w().totalLines, initialTotalLines, "total lines should be reduced in narrow mode")

	// Toggle narrow mode (exit)
	m.toggleNarrow()

	// Should be out of narrow mode
	assert.False(t, m.w().narrow.Active, "should exit narrow mode after second toggle")
	assert.Equal(t, m.w().totalLines, initialTotalLines, "total lines should be restored")

	// Scroll should be restored to approximately the original position
	// (cursor stays at the same content it was viewing)
	assert.GreaterOrEqual(t, m.w().scroll, 0, "scroll should be valid")
	assert.LessOrEqual(t, m.w().scroll, m.w().totalLines-1, "scroll should be within bounds")

	// The cursor should be close to the original position
	assert.InDelta(t, initialScroll, m.w().scroll, 3, "scroll should be close to original position")
}

func TestToggleNarrow_ShiftN_WithNoSearch(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line"}, New: sidebyside.Line{Num: 1, Content: "line"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40
	m.w().scroll = 2 // on file content

	// No search query - N should toggle narrow mode
	assert.Empty(t, m.searchQuery)
	assert.False(t, m.w().narrow.Active)

	// Press N (Shift+N)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)

	assert.True(t, model.w().narrow.Active, "N should toggle narrow mode when no search query")
}

func TestNarrowMode_StatusBarIndicator(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line"}, New: sidebyside.Line{Num: 1, Content: "line"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40
	m.focused = true

	// Not in narrow mode - view should not contain <N>
	view := m.View()
	assert.NotContains(t, view, "<N>", "should not show <N> indicator when not in narrow mode")

	// Enter narrow mode
	m.w().scroll = 2
	m.toggleNarrow()

	// In narrow mode - view should contain <N>
	view = m.View()
	assert.Contains(t, view, "<N>", "should show <N> indicator when in narrow mode")
}

func TestToggleNarrow_ShiftN_WithActiveSearch(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "match"}, New: sidebyside.Line{Num: 1, Content: "match"}},
				{Old: sidebyside.Line{Num: 2, Content: "other"}, New: sidebyside.Line{Num: 2, Content: "other"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40
	m.searchQuery = "match"
	m.searchForward = true
	m.w().scroll = 3 // on second content row

	// With active search query - N should do prevMatch, not narrow
	assert.NotEmpty(t, m.searchQuery)
	assert.False(t, m.w().narrow.Active)

	// Press N (Shift+N) - should do search, not narrow
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)

	// Should NOT enter narrow mode
	assert.False(t, model.w().narrow.Active, "N should do prevMatch when search query is active")
}

func TestFoldToggleAll_NarrowedToFile(t *testing.T) {
	// Create a model with 2 files at FoldHunks level
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
			},
		},
		{
			OldPath:   "a/second.go",
			NewPath:   "b/second.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line2"}, New: sidebyside.Line{Num: 1, Content: "line2"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Narrow to first file
	m.w().narrow = NarrowScope{
		Active:    true,
		CommitIdx: -1,
		FileIdx:   0,
		HunkIdx:   -1,
	}

	// Both files start at FoldHunks
	assert.Equal(t, sidebyside.FoldHunks, m.fileFoldLevel(0))
	assert.Equal(t, sidebyside.FoldHunks, m.fileFoldLevel(1))

	// Fold-toggle-all while narrowed to file 0
	newM, _ := m.handleFoldToggleAllFiles()
	model := newM.(Model)

	// Only file 0 should change (to FoldHeader, next in cycle)
	assert.Equal(t, sidebyside.FoldHeader, model.fileFoldLevel(0), "narrowed file should toggle")
	assert.Equal(t, sidebyside.FoldHunks, model.fileFoldLevel(1), "other file should NOT change")
}

func TestNarrow_NavigationBounds(t *testing.T) {
	// Test that gg and G respect narrow bounds
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
				{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
				{Old: sidebyside.Line{Num: 3, Content: "line3"}, New: sidebyside.Line{Num: 3, Content: "line3"}},
			},
		},
		{
			OldPath:   "a/second.go",
			NewPath:   "b/second.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "other"}, New: sidebyside.Line{Num: 1, Content: "other"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Enter narrow mode on first file
	m.w().scroll = 2 // on first file content
	m.toggleNarrow()

	assert.True(t, m.w().narrow.Active)
	narrowTotalLines := m.w().totalLines

	// Press G (go to end)
	m.w().scroll = m.maxScroll()
	assert.Equal(t, narrowTotalLines-1, m.w().scroll, "G should go to end of narrowed view")

	// Press gg (go to top)
	m.w().scroll = m.minScroll()
	assert.Equal(t, 0, m.w().scroll, "gg should go to start of narrowed view")

	// Verify we can't scroll beyond narrow bounds
	m.w().scroll = 100
	m.clampScroll()
	assert.LessOrEqual(t, m.w().scroll, narrowTotalLines-1, "scroll should be clamped to narrow bounds")
}

func TestNarrow_FoldWithinNarrowMode(t *testing.T) {
	// Test folding a file while narrowed to it
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line1"}, New: sidebyside.Line{Num: 1, Content: "line1"}},
				{Old: sidebyside.Line{Num: 2, Content: "line2"}, New: sidebyside.Line{Num: 2, Content: "line2"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Enter narrow mode
	m.w().scroll = 2
	m.toggleNarrow()
	assert.True(t, m.w().narrow.Active)

	initialTotalLines := m.w().totalLines
	assert.Greater(t, initialTotalLines, 1, "should have multiple rows when expanded")

	// Fold the file via toggle-all (since we're narrowed to single file)
	newM, _ := m.handleFoldToggleAllFiles()
	model := newM.(Model)

	// Should still be in narrow mode
	assert.True(t, model.w().narrow.Active, "should remain in narrow mode after fold")

	// Total lines should decrease (file is now folded)
	assert.Less(t, model.w().totalLines, initialTotalLines, "total lines should decrease when folded")

	// Scroll should be clamped to valid range
	assert.GreaterOrEqual(t, model.w().scroll, 0)
	assert.LessOrEqual(t, model.w().scroll, model.w().totalLines-1)
}

func TestNarrow_SearchScopedToNarrowedView(t *testing.T) {
	// Test that search only finds matches within narrowed view
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/first.go",
			NewPath:   "b/first.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "match here"}, New: sidebyside.Line{Num: 1, Content: "match here"}},
			},
		},
		{
			OldPath:   "a/second.go",
			NewPath:   "b/second.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "match there"}, New: sidebyside.Line{Num: 1, Content: "match there"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Narrow to first file only
	m.w().scroll = 2 // first file content
	m.toggleNarrow()
	assert.True(t, m.w().narrow.Active)
	assert.Equal(t, 0, m.w().narrow.FileIdx)

	// Set up search
	m.searchQuery = "match"
	m.searchForward = true
	m.w().scroll = 0

	// Find next match - should find it in the narrowed view (first file)
	m.nextMatch()

	// Verify we're still in narrow mode and on a valid row
	assert.True(t, m.w().narrow.Active)
	assert.GreaterOrEqual(t, m.w().scroll, 0)
	assert.Less(t, m.w().scroll, m.w().totalLines)

	// The match should be in the first file (the only file visible in narrow mode)
	rows := m.buildRows()
	if m.w().scroll < len(rows) {
		row := rows[m.w().scroll]
		// In narrow mode, all rows should belong to file 0 or be headers
		if row.fileIndex >= 0 {
			assert.Equal(t, 0, row.fileIndex, "match should be in narrowed file")
		}
	}
}

func TestNarrow_BlankRowBelongsToFile(t *testing.T) {
	// Pressing N on a blank row belonging to a file should narrow to that file
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "line"}, New: sidebyside.Line{Num: 1, Content: "line"}},
			},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Find a blank row (after the file content)
	rows := m.buildRows()
	blankRowIdx := -1
	blankFileIdx := -1
	for i, row := range rows {
		if row.kind == RowKindBlank && row.fileIndex >= 0 {
			blankRowIdx = i
			blankFileIdx = row.fileIndex
			break
		}
	}

	if blankRowIdx >= 0 {
		m.w().scroll = blankRowIdx
		m.toggleNarrow()

		// Blank rows belong to a file, so should narrow to that file
		assert.True(t, m.w().narrow.Active, "should enter narrow mode from blank row")
		assert.Equal(t, blankFileIdx, m.w().narrow.FileIdx, "should narrow to the file the blank row belongs to")
	}
}

func TestNarrow_OnCommitInfoHeader(t *testing.T) {
	// Test narrowing when cursor is on a commit info header row
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123def456789",
				Author:  "Test Author",
				Subject: "Test commit subject",
			},
			FoldLevel:   sidebyside.CommitFileHunks, // Expanded so info is visible
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/file1.go",
					NewPath:   "b/file1.go",
					FoldLevel: sidebyside.FoldHunks,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}},
					},
				},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Find a commit info row
	rows := m.buildRows()
	infoRowIdx := -1
	for i, row := range rows {
		if row.kind == RowKindCommitInfoHeader || row.kind == RowKindCommitInfoBody {
			infoRowIdx = i
			t.Logf("Found commit info row at %d: kind=%v, fileIndex=%d, commitIndex=%d, commitBodyIsBlank=%v",
				i, row.kind, row.fileIndex, row.commitIndex, row.commitBodyIsBlank)
			break
		}
	}

	require.GreaterOrEqual(t, infoRowIdx, 0, "should find a commit info row")

	// Move cursor to commit info row and toggle narrow
	m.w().scroll = infoRowIdx
	m.toggleNarrow()

	assert.True(t, m.w().narrow.Active, "should enter narrow mode on commit info row")
	assert.Equal(t, 0, m.w().narrow.CommitIdx, "should narrow to commit 0")
	assert.Equal(t, -1, m.w().narrow.FileIdx, "should not be file-scoped")
	assert.True(t, m.w().narrow.CommitInfoOnly, "should be commit-info-only mode")

	// Verify that only commit info rows are visible (no files, no commit header)
	narrowedRows := m.getRows()
	hasCommitInfoRows := false
	for i, row := range narrowedRows {
		// No file rows
		assert.Equal(t, -1, row.fileIndex, "row %d (kind=%v) should not have file index in commit-info-only mode", i, row.kind)
		// No commit header rows
		assert.NotEqual(t, RowKindCommitHeader, row.kind, "row %d should not be commit header in commit-info-only mode", i)
		assert.NotEqual(t, RowKindCommitHeaderBottomBorder, row.kind, "row %d should not be commit header bottom border", i)
		// Track that we have commit info rows
		if row.kind == RowKindCommitInfoHeader || row.kind == RowKindCommitInfoBody || row.kind == RowKindCommitInfoBottomBorder {
			hasCommitInfoRows = true
		}
	}
	assert.True(t, hasCommitInfoRows, "should have commit info rows in narrowed view")
	assert.Greater(t, len(narrowedRows), 0, "should have some rows in narrowed view")
}

func TestNarrow_OnCommitInfoHeader_NormalFoldLevel(t *testing.T) {
	// Test that commit info header row exists and is narrowable even at CommitFileHeaders level
	// (CommitFileHeaders shows the info header but not the body)
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123def456789",
				Author:  "Test Author",
				Subject: "Test commit subject",
			},
			FoldLevel:   sidebyside.CommitFileHeaders, // Normal level - header visible, body hidden
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/file1.go",
					NewPath:   "b/file1.go",
					FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}},
					},
				},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Find a commit info row
	rows := m.buildRows()
	infoRowIdx := -1
	for i, row := range rows {
		if row.kind == RowKindCommitInfoHeader {
			infoRowIdx = i
			t.Logf("Found commit info header at %d: commitIndex=%d", i, row.commitIndex)
			break
		}
	}

	require.GreaterOrEqual(t, infoRowIdx, 0, "should find a commit info header row even at CommitFileHeaders level")

	// Move cursor to commit info row and toggle narrow
	m.w().scroll = infoRowIdx
	m.toggleNarrow()

	assert.True(t, m.w().narrow.Active, "should enter narrow mode on commit info header")
	assert.Equal(t, 0, m.w().narrow.CommitIdx, "should narrow to commit 0")
	assert.True(t, m.w().narrow.CommitInfoOnly, "should be commit-info-only mode")

	// Verify that only commit info rows are visible (no files, no commit header)
	narrowedRows := m.getRows()
	hasCommitInfoRows := false
	for i, row := range narrowedRows {
		// No file rows
		assert.Equal(t, -1, row.fileIndex, "row %d (kind=%v) should not have file index in commit-info-only mode", i, row.kind)
		// No commit header rows
		assert.NotEqual(t, RowKindCommitHeader, row.kind, "row %d should not be commit header in commit-info-only mode", i)
		// Track that we have commit info rows
		if row.kind == RowKindCommitInfoHeader || row.kind == RowKindCommitInfoBody || row.kind == RowKindCommitInfoBottomBorder {
			hasCommitInfoRows = true
		}
	}
	assert.True(t, hasCommitInfoRows, "should have commit info rows in narrowed view")
}

func TestBuildRows_NarrowToCommit(t *testing.T) {
	// Create a model with 2 commits
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123def",
				Author:  "Test Author",
				Subject: "First commit",
			},
			FoldLevel:   sidebyside.CommitFileHeaders,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/file1.go",
					NewPath:   "b/file1.go",
					FoldLevel: sidebyside.FoldHunks,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}},
					},
				},
			},
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "def456ghi",
				Author:  "Test Author",
				Subject: "Second commit",
			},
			FoldLevel:   sidebyside.CommitFileHeaders,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/file2.go",
					NewPath:   "b/file2.go",
					FoldLevel: sidebyside.FoldHunks,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}},
					},
				},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Count rows without narrow scope
	fullRows := m.buildRows()
	fullRowCount := len(fullRows)

	// Now narrow to the first commit only
	m.w().narrow = NarrowScope{
		Active:    true,
		CommitIdx: 0,
		FileIdx:   -1,
		HunkIdx:   -1,
	}
	m.rebuildRowsCache()

	narrowRows := m.buildRows()
	narrowRowCount := len(narrowRows)

	// Narrowed view should have fewer rows
	require.Less(t, narrowRowCount, fullRowCount, "narrowed view should have fewer rows")

	// All rows in narrowed view should belong to commit 0 or have commitIndex = -1 (file rows)
	for i, row := range narrowRows {
		if row.commitIndex >= 0 {
			assert.Equal(t, 0, row.commitIndex, "row %d (kind=%v) should belong to commit 0, got commit %d", i, row.kind, row.commitIndex)
		}
		// File rows should have fileIndex 0 (which is in commit 0)
		if row.fileIndex >= 0 {
			assert.Equal(t, 0, row.fileIndex, "row %d (kind=%v) should belong to file 0, got file %d", i, row.kind, row.fileIndex)
		}
	}
}

func TestNarrow_ShouldNotLoadMoreCommits(t *testing.T) {
	// Create a model with pagination enabled (simulating partial commit load)
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123",
				Subject: "First commit",
			},
			FoldLevel:   sidebyside.CommitFileHeaders,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "file1.go", NewPath: "file1.go", FoldLevel: sidebyside.FoldStructure},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.loadedCommitCount = 1
	m.totalCommitCount = 10 // More commits available

	// Without narrow: should want to load more when near end
	m.w().scroll = m.maxScroll() // scroll to end
	assert.True(t, m.hasMoreCommitsToLoad(), "should have more commits to load")
	// Note: shouldLoadMoreCommits also requires m.git != nil, so we test the narrow behavior directly

	// With narrow: should NOT try to load more commits
	m.w().narrow = NarrowScope{
		Active:    true,
		CommitIdx: 0,
		FileIdx:   -1,
	}
	assert.False(t, m.shouldLoadMoreCommits(), "should not load more commits when narrowed")
}

func TestNarrow_HidesPaginationIndicator(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123",
				Subject: "First commit",
			},
			FoldLevel:   sidebyside.CommitFileHeaders,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "file1.go", NewPath: "file1.go", FoldLevel: sidebyside.FoldStructure},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.loadedCommitCount = 1
	m.totalCommitCount = 10 // More commits available

	// Without narrow: pagination indicator should appear
	rows := m.buildRows()
	hasPaginationIndicator := false
	for _, row := range rows {
		if row.kind == RowKindPaginationIndicator {
			hasPaginationIndicator = true
			break
		}
	}
	assert.True(t, hasPaginationIndicator, "pagination indicator should appear without narrow")

	// With narrow: pagination indicator should NOT appear
	m.w().narrow = NarrowScope{
		Active:    true,
		CommitIdx: 0,
		FileIdx:   -1,
	}
	m.rebuildRowsCache()

	rows = m.buildRows()
	hasPaginationIndicator = false
	for _, row := range rows {
		if row.kind == RowKindPaginationIndicator {
			hasPaginationIndicator = true
			break
		}
	}
	assert.False(t, hasPaginationIndicator, "pagination indicator should NOT appear when narrowed")
}
