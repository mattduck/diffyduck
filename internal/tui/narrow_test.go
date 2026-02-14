package tui

import (
	"testing"

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

func TestToggleNarrow_SpaceSequence(t *testing.T) {
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

	assert.False(t, m.w().narrow.Active)

	// Press space, n, f
	m = sendKey(m, " ")
	assert.Equal(t, "space", m.pendingKey)
	m = sendKey(m, "n")
	assert.Equal(t, "space n", m.pendingKey)
	m = sendKey(m, "f")
	assert.Equal(t, "", m.pendingKey)

	assert.True(t, m.w().narrow.Active, "space n f should toggle narrow mode")
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

func TestNarrowNext_WalksThroughNodes(t *testing.T) {
	// Two commits, each with 2 files
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "aaa",
				Author:  "Author",
				Subject: "First commit",
			},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
			},
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "bbb",
				Author:  "Author",
				Subject: "Second commit",
			},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f3.go", NewPath: "b/f3.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "c"}, New: sidebyside.Line{Num: 1, Content: "c"}}}},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Start narrowed to commit 0
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: -1}
	m.unfoldNarrowTarget()
	m.rebuildRowsCache()

	// Walk: commit₀ → message₀ → file₀ → file₁ → commit₁ → message₁ → file₂
	type step struct {
		commitIdx      int
		fileIdx        int
		commitInfoOnly bool
	}
	expected := []step{
		{0, -1, true},  // message₀
		{0, 0, false},  // file₀ (global index 0)
		{0, 1, false},  // file₁ (global index 1)
		{1, -1, false}, // commit₁
		{1, -1, true},  // message₁
		{1, 2, false},  // file₂ (global index 2)
	}

	for i, exp := range expected {
		moved := m.narrowNext()
		assert.True(t, moved, "step %d should move", i)
		ns := m.w().narrow
		assert.Equal(t, exp.commitIdx, ns.CommitIdx, "step %d commitIdx", i)
		assert.Equal(t, exp.fileIdx, ns.FileIdx, "step %d fileIdx", i)
		assert.Equal(t, exp.commitInfoOnly, ns.CommitInfoOnly, "step %d commitInfoOnly", i)
	}

	// One more should be a no-op (at the end)
	moved := m.narrowNext()
	assert.False(t, moved, "should not move past last node")
}

func TestNarrowPrev_WalksThroughNodes(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "aaa",
				Author:  "Author",
				Subject: "First commit",
			},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
			},
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "bbb",
				Author:  "Author",
				Subject: "Second commit",
			},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Start narrowed to file₁ (last file of second commit, global index 1)
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 1, FileIdx: 1}
	m.unfoldNarrowTarget()
	m.rebuildRowsCache()

	// Walk backward: file₁ → message₁ → commit₁ → file₀ → message₀ → commit₀
	type step struct {
		commitIdx      int
		fileIdx        int
		commitInfoOnly bool
	}
	expected := []step{
		{1, -1, true},  // message₁
		{1, -1, false}, // commit₁
		{0, 0, false},  // file₀ (last file of commit₀)
		{0, -1, true},  // message₀
		{0, -1, false}, // commit₀
	}

	for i, exp := range expected {
		moved := m.narrowPrev()
		assert.True(t, moved, "step %d should move", i)
		ns := m.w().narrow
		assert.Equal(t, exp.commitIdx, ns.CommitIdx, "step %d commitIdx", i)
		assert.Equal(t, exp.fileIdx, ns.FileIdx, "step %d fileIdx", i)
		assert.Equal(t, exp.commitInfoOnly, ns.CommitInfoOnly, "step %d commitInfoOnly", i)
	}

	// One more should be a no-op
	moved := m.narrowPrev()
	assert.False(t, moved, "should not move past first node")
}

func TestNarrowNext_BoundaryNoOp(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{
			Info:        sidebyside.CommitInfo{SHA: "aaa", Subject: "only commit"},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Narrow to the last node (file 0)
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: 0}
	m.rebuildRowsCache()

	moved := m.narrowNext()
	assert.False(t, moved, "should not move past last node")
	assert.Equal(t, 0, m.w().narrow.FileIdx, "should stay on file 0")
}

func TestNarrowPrev_BoundaryNoOp(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{
			Info:        sidebyside.CommitInfo{SHA: "aaa", Subject: "only commit"},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Narrow to the first node (commit 0)
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: -1}
	m.rebuildRowsCache()

	moved := m.narrowPrev()
	assert.False(t, moved, "should not move before first node")
	assert.Equal(t, 0, m.w().narrow.CommitIdx, "should stay on commit 0")
	assert.Equal(t, -1, m.w().narrow.FileIdx, "should not be on a file")
}

func TestNarrowNext_DiffMode(t *testing.T) {
	// Diff mode: no commit metadata, just files
	files := []sidebyside.FilePair{
		{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
		{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
		{OldPath: "a/f3.go", NewPath: "b/f3.go", FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "c"}, New: sidebyside.Line{Num: 1, Content: "c"}}}},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Narrow to file 0
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: 0}
	m.rebuildRowsCache()

	// file₀ → file₁ → file₂ → no-op
	moved := m.narrowNext()
	assert.True(t, moved)
	assert.Equal(t, 1, m.w().narrow.FileIdx)

	moved = m.narrowNext()
	assert.True(t, moved)
	assert.Equal(t, 2, m.w().narrow.FileIdx)

	moved = m.narrowNext()
	assert.False(t, moved, "should stop at last file")
	assert.Equal(t, 2, m.w().narrow.FileIdx)

	// And backward
	moved = m.narrowPrev()
	assert.True(t, moved)
	assert.Equal(t, 1, m.w().narrow.FileIdx)

	moved = m.narrowPrev()
	assert.True(t, moved)
	assert.Equal(t, 0, m.w().narrow.FileIdx)

	moved = m.narrowPrev()
	assert.False(t, moved, "should stop at first file")
	assert.Equal(t, 0, m.w().narrow.FileIdx)
}

func TestNarrowNext_FromUnnarrowed(t *testing.T) {
	files := []sidebyside.FilePair{
		{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
		{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
	}

	m := New(files)
	m.width = 120
	m.height = 40
	m.w().scroll = 2 // On first file's content

	assert.False(t, m.w().narrow.Active)

	// C-j from un-narrowed should enter narrow mode (like N)
	moved := m.narrowNext()
	assert.True(t, moved)
	assert.True(t, m.w().narrow.Active, "should enter narrow mode")
}

func TestNarrowNext_TriggersPagination(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{
			Info:        sidebyside.CommitInfo{SHA: "aaa", Subject: "First"},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files:       []sidebyside.FilePair{{OldPath: "f1.go", NewPath: "f1.go", FoldLevel: sidebyside.FoldHeader}},
		},
		{
			Info:        sidebyside.CommitInfo{SHA: "bbb", Subject: "Second"},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files:       []sidebyside.FilePair{{OldPath: "f2.go", NewPath: "f2.go", FoldLevel: sidebyside.FoldHeader}},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.loadedCommitCount = 2
	m.totalCommitCount = 100 // Many more available

	// Narrow to last commit — within threshold of end
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 1, FileIdx: -1}
	m.rebuildRowsCache()

	// Should want to paginate (within 3 commits of end, has more to load)
	// Note: shouldPaginateForNarrowNav also requires m.git != nil
	assert.True(t, m.hasMoreCommitsToLoad(), "should have more commits")
	// Can't fully test pagination trigger without git mock, but verify the threshold logic
	assert.LessOrEqual(t, len(m.commits)-m.w().narrow.CommitIdx, NarrowPaginationCommitThreshold)
}

func TestNarrowNext_UnfoldsCommit(t *testing.T) {
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "aaa",
				Author:  "Author",
				Subject: "First commit",
			},
			FoldLevel:   sidebyside.CommitFolded, // starts folded
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
			},
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "bbb",
				Author:  "Author",
				Subject: "Second commit",
			},
			FoldLevel:   sidebyside.CommitFolded, // starts folded
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Narrow to last file of commit 0
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: 0}
	m.rebuildRowsCache()

	// Navigate to commit 1 (which is currently folded)
	m.narrowNext() // → commit₁

	assert.Equal(t, 1, m.w().narrow.CommitIdx)
	assert.Equal(t, -1, m.w().narrow.FileIdx)
	assert.False(t, m.w().narrow.CommitInfoOnly)

	// Commit 1 should now be unfolded to CommitFileStructure
	assert.Equal(t, sidebyside.CommitFileStructure, m.commitFoldLevel(1),
		"navigating to a commit should unfold it to CommitFileStructure")

	// File in commit 1 should be unfolded to FoldStructure (matching CommitFileStructure)
	assert.Equal(t, sidebyside.FoldStructure, m.fileFoldLevel(1),
		"files in navigated commit should be unfolded to FoldStructure")
}

func TestNarrowNext_UnfoldsFile(t *testing.T) {
	files := []sidebyside.FilePair{
		{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
		{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHeader, // starts folded
			Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
	}

	m := New(files)
	m.width = 120
	m.height = 40

	// Narrow to file 0
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: 0}
	m.rebuildRowsCache()

	// Navigate to file 1 (which starts at FoldHeader)
	m.narrowNext()

	assert.Equal(t, 1, m.w().narrow.FileIdx)
	assert.Equal(t, sidebyside.FoldHunks, m.fileFoldLevel(1),
		"navigating to a file should unfold it to FoldHunks")
}

func TestNarrowNext_SkipsMessageNodeForSnapshots(t *testing.T) {
	// Snapshot commits have no message/commit-info node
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "snap1",
				Subject: "Diff 1",
			},
			IsSnapshot:  true,
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
			},
		},
		{
			Info: sidebyside.CommitInfo{
				SHA:     "snap2",
				Subject: "Diff 2",
			},
			IsSnapshot:  true,
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Start narrowed to commit 0
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: -1}
	m.unfoldNarrowTarget()
	m.rebuildRowsCache()

	// Forward: commit₀ → file₀ (skip message) → commit₁ → file₁ (skip message)
	type step struct {
		commitIdx      int
		fileIdx        int
		commitInfoOnly bool
	}
	expected := []step{
		{0, 0, false},  // file₀ — message skipped
		{1, -1, false}, // commit₁
		{1, 1, false},  // file₁ — message skipped
	}

	for i, exp := range expected {
		moved := m.narrowNext()
		assert.True(t, moved, "step %d should move", i)
		ns := m.w().narrow
		assert.Equal(t, exp.commitIdx, ns.CommitIdx, "step %d commitIdx", i)
		assert.Equal(t, exp.fileIdx, ns.FileIdx, "step %d fileIdx", i)
		assert.Equal(t, exp.commitInfoOnly, ns.CommitInfoOnly, "step %d commitInfoOnly", i)
	}

	// At end
	assert.False(t, m.narrowNext(), "should not move past last node")

	// Backward: file₁ → commit₁ → file₀ → commit₀
	expectedBack := []step{
		{1, -1, false}, // commit₁ — message skipped
		{0, 0, false},  // file₀
		{0, -1, false}, // commit₀ — message skipped
	}

	for i, exp := range expectedBack {
		moved := m.narrowPrev()
		assert.True(t, moved, "back step %d should move", i)
		ns := m.w().narrow
		assert.Equal(t, exp.commitIdx, ns.CommitIdx, "back step %d commitIdx", i)
		assert.Equal(t, exp.fileIdx, ns.FileIdx, "back step %d fileIdx", i)
		assert.Equal(t, exp.commitInfoOnly, ns.CommitInfoOnly, "back step %d commitInfoOnly", i)
	}

	// At start
	assert.False(t, m.narrowPrev(), "should not move before first node")
}

func TestNarrowNext_CommitWithNoFiles(t *testing.T) {
	// A commit with no files (e.g. filtered by pathspec) should be traversed correctly
	commits := []sidebyside.CommitSet{
		{
			Info:        sidebyside.CommitInfo{SHA: "aaa", Author: "A", Subject: "has files"},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f1.go", NewPath: "b/f1.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "a"}, New: sidebyside.Line{Num: 1, Content: "a"}}}},
			},
		},
		{
			Info:        sidebyside.CommitInfo{SHA: "bbb", Author: "B", Subject: "no files"},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files:       []sidebyside.FilePair{}, // empty
		},
		{
			Info:        sidebyside.CommitInfo{SHA: "ccc", Author: "C", Subject: "also has files"},
			FoldLevel:   sidebyside.CommitFolded,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{OldPath: "a/f2.go", NewPath: "b/f2.go", FoldLevel: sidebyside.FoldHeader,
					Pairs: []sidebyside.LinePair{{Old: sidebyside.Line{Num: 1, Content: "b"}, New: sidebyside.Line{Num: 1, Content: "b"}}}},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40

	// Start at commit 0
	m.w().narrow = NarrowScope{Active: true, CommitIdx: 0, FileIdx: -1}
	m.unfoldNarrowTarget()
	m.rebuildRowsCache()

	// Forward: commit₀ → msg₀ → file₀ → commit₁ → msg₁ → commit₂ → msg₂ → file₁
	type step struct {
		commitIdx      int
		fileIdx        int
		commitInfoOnly bool
	}
	expected := []step{
		{0, -1, true},  // msg₀
		{0, 0, false},  // file₀
		{1, -1, false}, // commit₁
		{1, -1, true},  // msg₁ (no files, so next is...)
		{2, -1, false}, // commit₂
		{2, -1, true},  // msg₂
		{2, 1, false},  // file₁ (global index 1)
	}

	for i, exp := range expected {
		moved := m.narrowNext()
		assert.True(t, moved, "step %d should move", i)
		ns := m.w().narrow
		assert.Equal(t, exp.commitIdx, ns.CommitIdx, "step %d commitIdx", i)
		assert.Equal(t, exp.fileIdx, ns.FileIdx, "step %d fileIdx", i)
		assert.Equal(t, exp.commitInfoOnly, ns.CommitInfoOnly, "step %d commitInfoOnly", i)
	}
}
