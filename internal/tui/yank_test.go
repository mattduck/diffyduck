package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/comments"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// makeYankTestModel creates a model with commentable lines for yank testing.
func makeYankTestModel() Model {
	pairs := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 1, Content: "context line 1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "context line 1", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 2, Content: "context line 2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "context line 2", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 3, Content: "old line 3", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 3, Content: "new line 3", Type: sidebyside.Added}},
		{Old: sidebyside.Line{Num: 4, Content: "context line 4", Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Content: "context line 4", Type: sidebyside.Context}},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]*comments.Comment)
	m.clipboard = &MemoryClipboard{}
	return m
}

// Test: findCommentForCursor returns false when no comment exists
func TestYank_FindCommentForCursor_NoComment(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Move to a content line
	m.w().scroll = 3 // Skip header rows

	_, found := m.findCommentForCursor()
	assert.False(t, found, "should not find comment when none exists")
}

// Test: findCommentForCursor returns true when cursor is on line with comment
func TestYank_FindCommentForCursor_OnCommentedLine(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{ID: "1700000000000", Text: "test comment"}
	m.rebuildRowsCache()

	// Find the row index for line 3
	rows := m.getRows()
	var lineRowIdx int
	for i, row := range rows {
		if row.kind == RowKindContent && row.pair.New.Num == 3 {
			lineRowIdx = i
			break
		}
	}

	// Position cursor on the commented line (scroll = rowIdx - cursorOffset)
	m.w().scroll = lineRowIdx

	ck, found := m.findCommentForCursor()
	assert.True(t, found, "should find comment when cursor is on commented line")
	assert.Equal(t, 3, ck.newLineNum, "should return correct line number")
}

// Test: findCommentForCursor returns true when cursor is on comment row
func TestYank_FindCommentForCursor_OnCommentRow(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{ID: "1700000000000", Text: "test comment"}
	m.rebuildRowsCache()

	// Find the comment row index
	rows := m.getRows()
	var commentRowIdx int
	for i, row := range rows {
		if row.kind == RowKindComment && row.commentLineNum == 3 {
			commentRowIdx = i
			break
		}
	}

	// Position cursor on the comment row (scroll = rowIdx - cursorOffset)
	m.w().scroll = commentRowIdx

	ck, found := m.findCommentForCursor()
	assert.True(t, found, "should find comment when cursor is on comment row")
	assert.Equal(t, 3, ck.newLineNum, "should return correct line number")
}

// Test: buildDiffSnippet produces valid unified diff format
func TestYank_BuildDiffSnippet_Format(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 3}
	c := &comments.Comment{ID: "1700000000000", Text: "This is my comment"}

	snippet := m.buildDiffSnippet(key, c)

	// Should have file headers
	assert.Contains(t, snippet, "--- a/test.go", "should have old file header")
	assert.Contains(t, snippet, "+++ b/test.go", "should have new file header")

	// Should have hunk header
	assert.Contains(t, snippet, "@@ -", "should have hunk header")

	// Should have the comment with # prefix and comment ID
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000000:\n# This is my comment\n#\n#\n", "should have comment with # COMMENT_ID ID prefix and trailing blank # lines")

	// Should have diff lines
	assert.Contains(t, snippet, "-old line 3", "should have removed line")
	assert.Contains(t, snippet, "+new line 3", "should have added line")
}

// Test: buildDiffSnippet includes context lines before commented line
func TestYank_BuildDiffSnippet_Context(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 3}
	c := &comments.Comment{ID: "1700000000000", Text: "Comment on line 3"}

	snippet := m.buildDiffSnippet(key, c)

	// Should include context lines before the commented line
	assert.Contains(t, snippet, " context line 1", "should have first context line")
	assert.Contains(t, snippet, " context line 2", "should have second context line")

	// Should NOT include context after (as per spec)
	assert.NotContains(t, snippet, "context line 4", "should not have context after commented line")
}

// Test: buildDiffSnippet handles multi-line comments
func TestYank_BuildDiffSnippet_MultilineComment(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 3}
	c := &comments.Comment{ID: "1700000000000", Text: "Line one\nLine two\nLine three"}

	snippet := m.buildDiffSnippet(key, c)

	// Each line should be prefixed with #
	assert.Contains(t, snippet, "# Line one\n", "should have first comment line")
	assert.Contains(t, snippet, "# Line two\n", "should have second comment line")
	assert.Contains(t, snippet, "# Line three\n", "should have third comment line")
}

// Test: handleYank sets status message on success (mocked clipboard)
func TestYank_HandleYank_SetsStatusMessage(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{ID: "1700000000000", Text: "test comment"}
	m.rebuildRowsCache()

	// Find and position on the commented line (scroll = rowIdx - cursorOffset)
	rows := m.getRows()
	for i, row := range rows {
		if row.kind == RowKindContent && row.pair.New.Num == 3 {
			m.w().scroll = i
			break
		}
	}

	newModel, _ := m.handleYank()
	m2 := newModel.(Model)

	assert.Contains(t, m2.statusMessage, "Copied", "status message should indicate success")
	assert.Contains(t, m2.statusMessage, "test.go", "success message should contain filename")

	// Verify clipboard received the snippet
	mc := m2.clipboard.(*MemoryClipboard)
	assert.Contains(t, mc.Content, "# COMMENT_ID 1700000000000:", "clipboard should contain the comment snippet")
}

// Test: calculateHunkHeader computes correct values
func TestYank_CalculateHunkHeader(t *testing.T) {
	m := makeYankTestModel()

	// Test with pairs from index 0 to 2 (context, context, changed)
	oldStart, oldCount, newStart, newCount := m.calculateHunkHeader(m.files[0].Pairs, 0, 2)

	assert.Equal(t, 1, oldStart, "old start should be 1")
	assert.Equal(t, 1, newStart, "new start should be 1")
	assert.Equal(t, 3, oldCount, "old count should be 3 (2 context + 1 removed)")
	assert.Equal(t, 3, newCount, "new count should be 3 (2 context + 1 added)")
}

// Test: writeDiffLines handles different line types correctly
func TestYank_WriteDiffLines(t *testing.T) {
	m := makeYankTestModel()

	tests := []struct {
		name     string
		pair     sidebyside.LinePair
		expected string
	}{
		{
			name: "context line",
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: 1, Content: "same", Type: sidebyside.Context},
				New: sidebyside.Line{Num: 1, Content: "same", Type: sidebyside.Context},
			},
			expected: " same\n",
		},
		{
			name: "added line",
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
				New: sidebyside.Line{Num: 1, Content: "added", Type: sidebyside.Added},
			},
			expected: "+added\n",
		},
		{
			name: "removed line",
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: 1, Content: "removed", Type: sidebyside.Removed},
				New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			},
			expected: "-removed\n",
		},
		{
			name: "changed line",
			pair: sidebyside.LinePair{
				Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
				New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
			},
			expected: "-old\n+new\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			m.writeDiffLines(&sb, tt.pair)
			assert.Equal(t, tt.expected, sb.String())
		})
	}
}

// =============================================================================
// Status Message Clear-on-Keypress Tests
// =============================================================================

// Test: status message is cleared on keypress after minimum duration
func TestYank_StatusMessage_ClearedOnKeypress(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Set a status message in the past (older than minimum duration)
	m.statusMessage = "Old message"
	m.statusMessageTime = time.Now().Add(-2 * time.Second)

	// Any keypress should clear it
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m2 := newModel.(Model)

	assert.Empty(t, m2.statusMessage, "should clear status message after min duration on keypress")
}

// Test: status message is NOT cleared if minimum duration hasn't elapsed
func TestYank_StatusMessage_KeptDuringMinDuration(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Set a status message just now (within minimum duration)
	m.statusMessage = "Fresh message"
	m.statusMessageTime = time.Now()

	// Keypress should NOT clear it
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m2 := newModel.(Model)

	assert.Equal(t, "Fresh message", m2.statusMessage, "should keep status message during minimum duration")
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// Test: buildDiffSnippet handles comment on first line (limited context)
func TestYank_BuildDiffSnippet_FirstLine(t *testing.T) {
	// Create model with comment on line 1
	pairs := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 1, Content: "first line", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 1, Content: "new first", Type: sidebyside.Added}},
		{Old: sidebyside.Line{Num: 2, Content: "second", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "second", Type: sidebyside.Context}},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]*comments.Comment)
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 1}
	c := &comments.Comment{ID: "1700000000000", Text: "Comment on first line"}

	snippet := m.buildDiffSnippet(key, c)

	// Should still have valid format even with no context before
	assert.Contains(t, snippet, "--- a/test.go")
	assert.Contains(t, snippet, "+++ b/test.go")
	assert.Contains(t, snippet, "@@ -")
	assert.Contains(t, snippet, "+new first")
	assert.Contains(t, snippet, "# Comment on first line")
}

// Test: buildDiffSnippet handles comment on context line
func TestYank_BuildDiffSnippet_ContextLine(t *testing.T) {
	pairs := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 2, Content: "line 2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "line 2", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 3, Content: "line 3", Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Content: "line 3", Type: sidebyside.Context}},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]*comments.Comment)
	m.calculateTotalLines()

	// Comment on context line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	c := &comments.Comment{ID: "1700000000000", Text: "Note about this line"}

	snippet := m.buildDiffSnippet(key, c)

	// Should show context lines with space prefix
	assert.Contains(t, snippet, " line 1")
	assert.Contains(t, snippet, " line 2")
	assert.Contains(t, snippet, " line 3")
	assert.Contains(t, snippet, "# Note about this line")
}

// Test: clearStatusAfter returns nil (status is cleared on keypress now)
func TestYank_ClearStatusAfter_ReturnsNil(t *testing.T) {
	m := makeYankTestModel()
	now := time.Now()

	cmd := m.clearStatusAfter(now)

	assert.Nil(t, cmd, "clearStatusAfter should return nil (clear happens on keypress)")
}

// Test: handleYank returns nil cmd when no comment at cursor
func TestYank_HandleYank_NoComment_ReturnsNil(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Don't add any comments, just position cursor somewhere
	m.w().scroll = 3

	newModel, cmd := m.handleYank()
	m2 := newModel.(Model)

	assert.Nil(t, cmd, "should return nil cmd when no comment to yank")
	assert.Empty(t, m2.statusMessage, "should not set status message when no comment")
}

// Test: Status message appears in rendered view
func TestYank_StatusMessage_AppearsInView(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()
	m.focused = true

	m.statusMessage = "Test status message"
	m.statusMessageTime = time.Now()

	view := m.View()

	assert.Contains(t, view, "Test status message", "status message should appear in view")
}

// Test: handleYank with yank key press (integration)
func TestYank_KeyPress_Integration(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{ID: "1700000000000", Text: "integration test comment"}
	m.rebuildRowsCache()

	// Position cursor on the commented line
	rows := m.getRows()
	for i, row := range rows {
		if row.kind == RowKindContent && row.pair.New.Num == 3 {
			m.w().scroll = i
			break
		}
	}

	// Simulate pressing 'y'
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m2 := newModel.(Model)

	// Should have set a status message
	assert.NotEmpty(t, m2.statusMessage, "pressing y should set status message")

	// Should return a command (the clear timer)
	// cmd is nil — status is cleared on next keypress, not by timer
	_ = cmd
}

// =============================================================================
// File Path Yank Tests
// =============================================================================

func TestYank_FilePath_OnFileHeader(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Find the file header row
	rows := m.getRows()
	headerIdx := -1
	for i, row := range rows {
		if row.kind == RowKindHeader {
			headerIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, headerIdx, 0, "should find a file header row")

	m.w().scroll = headerIdx
	newModel, _ := m.handleYank()
	m2 := newModel.(Model)

	clip := m2.clipboard.(*MemoryClipboard)
	assert.Equal(t, "test.go", clip.Content, "should copy relative file path")
	assert.Contains(t, m2.statusMessage, "test.go")
}

func TestYank_FilePath_KeyPress(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Find the file header row and position cursor there
	rows := m.getRows()
	for i, row := range rows {
		if row.kind == RowKindHeader {
			m.w().scroll = i
			break
		}
	}

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m2 := newModel.(Model)

	clip := m2.clipboard.(*MemoryClipboard)
	assert.Equal(t, "test.go", clip.Content)
	assert.Contains(t, m2.statusMessage, "Copied test.go")
}

// =============================================================================
// Commit SHA Yank Tests
// =============================================================================

// makeCommitYankTestModel creates a model with commits for SHA yank testing.
func makeCommitYankTestModel() Model {
	commits := []sidebyside.CommitSet{
		{
			Info: sidebyside.CommitInfo{
				SHA:     "abc123def456789abcdef0123456789abcdef01",
				Author:  "Test Author",
				Subject: "Test commit subject",
			},
			FoldLevel:   sidebyside.CommitFileHeaders,
			FilesLoaded: true,
			Files: []sidebyside.FilePair{
				{
					OldPath:   "a/file1.go",
					NewPath:   "b/file1.go",
					FoldLevel: sidebyside.FoldHunks,
					Pairs: []sidebyside.LinePair{
						{Old: sidebyside.Line{Num: 1, Content: "old"}, New: sidebyside.Line{Num: 1, Content: "new"}},
					},
				},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.clipboard = &MemoryClipboard{}
	return m
}

func TestYank_CommitSHA_OnCommitHeader(t *testing.T) {
	m := makeCommitYankTestModel()

	// Find a commit header row
	rows := m.getRows()
	headerIdx := -1
	for i, row := range rows {
		if row.kind == RowKindCommitHeader {
			headerIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, headerIdx, 0, "should find a commit header row")

	m.w().scroll = headerIdx
	newModel, cmd := m.handleYank()
	m2 := newModel.(Model)

	clip := m2.clipboard.(*MemoryClipboard)
	assert.Equal(t, "abc123def456789abcdef0123456789abcdef01", clip.Content, "should copy full SHA")
	assert.Contains(t, m2.statusMessage, "abc123d", "status should show short SHA")
	// cmd is nil — status is cleared on next keypress, not by timer
	_ = cmd
}

func TestYank_CommitSHA_NotOnBorder(t *testing.T) {
	m := makeCommitYankTestModel()

	// Find a commit border row — y should NOT copy SHA here
	rows := m.getRows()
	borderIdx := -1
	for i, row := range rows {
		if row.kind == RowKindCommitHeaderTopBorder || row.kind == RowKindCommitHeaderBottomBorder {
			borderIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, borderIdx, 0, "should find a commit border row")

	m.w().scroll = borderIdx
	newModel, cmd := m.handleYank()
	m2 := newModel.(Model)

	clip := m2.clipboard.(*MemoryClipboard)
	assert.Empty(t, clip.Content, "should NOT copy SHA from border row")
	assert.Nil(t, cmd, "should return nil when not on a yankable row")
	assert.Empty(t, m2.statusMessage)
}

func TestYank_CommitSHA_KeyPress(t *testing.T) {
	m := makeCommitYankTestModel()

	// Find a commit header row
	rows := m.getRows()
	for i, row := range rows {
		if row.kind == RowKindCommitHeader {
			m.w().scroll = i
			break
		}
	}

	// Press y
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m2 := newModel.(Model)

	clip := m2.clipboard.(*MemoryClipboard)
	assert.Equal(t, "abc123def456789abcdef0123456789abcdef01", clip.Content)
	_ = cmd
}

// =============================================================================
// YankAll (capital Y) Tests
// =============================================================================

// makeYankAllTestModel creates a model with multiple files and commentable lines.
func makeYankAllTestModel() Model {
	pairs1 := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 1, Content: "context line 1", Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Content: "context line 1", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 2, Content: "context line 2", Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Content: "context line 2", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 3, Content: "old line 3", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 3, Content: "new line 3", Type: sidebyside.Added}},
		{Old: sidebyside.Line{Num: 4, Content: "context line 4", Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Content: "context line 4", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 5, Content: "context line 5", Type: sidebyside.Context}, New: sidebyside.Line{Num: 5, Content: "context line 5", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 6, Content: "old line 6", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 6, Content: "new line 6", Type: sidebyside.Added}},
		{Old: sidebyside.Line{Num: 7, Content: "context line 7", Type: sidebyside.Context}, New: sidebyside.Line{Num: 7, Content: "context line 7", Type: sidebyside.Context}},
	}
	pairs2 := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 10, Content: "other context", Type: sidebyside.Context}, New: sidebyside.Line{Num: 10, Content: "other context", Type: sidebyside.Context}},
		{Old: sidebyside.Line{Num: 11, Content: "other old", Type: sidebyside.Removed}, New: sidebyside.Line{Num: 11, Content: "other new", Type: sidebyside.Added}},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/file1.go", NewPath: "b/file1.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs1},
		{OldPath: "a/file2.go", NewPath: "b/file2.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs2},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]*comments.Comment)
	m.clipboard = &MemoryClipboard{}
	return m
}

// Test: handleYankAll returns nil when no comments exist
func TestYankAll_NoComments_ReturnsNil(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	newModel, cmd := m.handleYankComments(false)
	m2 := newModel.(Model)

	assert.Nil(t, cmd)
	assert.Empty(t, m2.statusMessage)
}

// Test: buildAllCommentsSnippet with a single comment
func TestYankAll_SingleComment(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "only comment"}

	snippet, _ := m.buildCommentsSnippet(false)

	assert.Contains(t, snippet, "--- a/file1.go")
	assert.Contains(t, snippet, "+++ b/file1.go")
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000001:")
	assert.Contains(t, snippet, "# only comment")
}

// Test: buildAllCommentsSnippet with multiple comments across files, correct global numbering
func TestYankAll_MultipleFiles_GlobalNumbering(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "first comment"}
	m.comments[commentKey{fileIndex: 1, newLineNum: 11}] = &comments.Comment{ID: "1700000000002", Text: "second comment"}

	snippet, _ := m.buildCommentsSnippet(false)

	// File 1
	assert.Contains(t, snippet, "--- a/file1.go")
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000001:")
	assert.Contains(t, snippet, "# first comment")

	// File 2
	assert.Contains(t, snippet, "--- a/file2.go")
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000002:")
	assert.Contains(t, snippet, "# second comment")

	// File 1 comment should appear before file 2 comment
	idx1 := strings.Index(snippet, "# COMMENT_ID 1700000000001:")
	idx2 := strings.Index(snippet, "# COMMENT_ID 1700000000002:")
	assert.True(t, idx1 < idx2, "file 1 comment should appear before file 2 comment")
}

// Test: nearby comments in same file are merged into one hunk
func TestYankAll_MergedHunks(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	// Lines 3 and 4 are adjacent — with 2 context lines before each,
	// their ranges overlap so they should merge into one hunk
	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "comment on 3"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 4}] = &comments.Comment{ID: "1700000000002", Text: "comment on 4"}

	snippet, _ := m.buildCommentsSnippet(false)

	// Should have only ONE hunk header for this file
	hunkCount := strings.Count(snippet, "@@ -")
	assert.Equal(t, 1, hunkCount, "adjacent comments should be merged into one hunk")

	// Both comments should be present
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000001:")
	assert.Contains(t, snippet, "# comment on 3")
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000002:")
	assert.Contains(t, snippet, "# comment on 4")
}

// Test: distant comments in same file get separate hunks
func TestYankAll_SeparateHunks(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	// Lines 3 and 6 are far enough apart that ranges don't overlap
	// Line 3: range [1,3], Line 6: range [4,6] — actually these are adjacent (endIdx=3, startIdx=4)
	// so they'd merge. Let me use line 7 instead which has range [5,7]
	// Line 3: range [1,3], Line 7: range [5,7] — startIdx(5) > endIdx(3)+1, separate
	m.comments[commentKey{fileIndex: 0, newLineNum: 1}] = &comments.Comment{ID: "1700000000001", Text: "comment on 1"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 7}] = &comments.Comment{ID: "1700000000002", Text: "comment on 7"}

	snippet, _ := m.buildCommentsSnippet(false)

	// Should have TWO hunk headers for this file
	hunkCount := strings.Count(snippet, "@@ -")
	assert.Equal(t, 2, hunkCount, "distant comments should get separate hunks")
}

// Test: multiline comment gets proper formatting
func TestYankAll_MultilineComment(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "line one\nline two"}

	snippet, _ := m.buildCommentsSnippet(false)

	assert.Contains(t, snippet, "# COMMENT_ID 1700000000001:")
	assert.Contains(t, snippet, "# line one\n")
	assert.Contains(t, snippet, "# line two\n")
}

// Test: handleYankAll sets status message with comment count
func TestYankAll_StatusMessage(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "a"}
	m.comments[commentKey{fileIndex: 1, newLineNum: 11}] = &comments.Comment{ID: "1700000000002", Text: "b"}

	newModel, _ := m.handleYankComments(false)
	m2 := newModel.(Model)

	assert.Contains(t, m2.statusMessage, "Copied 2 unresolved comments")
}

// Test: empty comments are excluded
func TestYankAll_SkipsEmptyComments(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "real comment"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 4}] = &comments.Comment{ID: "1700000000002", Text: ""}

	snippet, count := m.buildCommentsSnippet(false)

	assert.Equal(t, 1, count)
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000001:")
	assert.NotContains(t, snippet, "# COMMENT_ID 1700000000002:")
}

// Test: resolved comments are excluded
func TestYankAll_SkipsResolvedComments(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "keep this one"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 6}] = &comments.Comment{ID: "1700000000002", Text: "skip this one", Resolved: true}

	snippet, count := m.buildCommentsSnippet(false)

	assert.Equal(t, 1, count)
	assert.Contains(t, snippet, "# COMMENT_ID 1700000000001:")
	assert.Contains(t, snippet, "# keep this one")
	assert.NotContains(t, snippet, "skip this one")
}

// Test: handleYankAll returns nil when all comments are resolved
func TestYankAll_AllResolved_ReturnsNil(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "done", Resolved: true}

	newModel, cmd := m.handleYankComments(false)
	m2 := newModel.(Model)

	assert.Nil(t, cmd)
	assert.Empty(t, m2.statusMessage)
}

// Test: handleYankComments(false) sets status message (unresolved)
func TestYankUnresolved_Handler_Integration(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "test"}
	m.rebuildRowsCache()

	newModel, _ := m.handleYankComments(false)
	m2 := newModel.(Model)

	assert.Contains(t, m2.statusMessage, "unresolved comments")
}

// Test: handleYankComments(true) includes resolved comments
func TestYankAllComments_Handler_Integration(t *testing.T) {
	m := makeYankAllTestModel()
	m.calculateTotalLines()

	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{ID: "1700000000001", Text: "unresolved"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 6}] = &comments.Comment{ID: "1700000000002", Text: "resolved", Resolved: true}
	m.rebuildRowsCache()

	newModel, _ := m.handleYankComments(true)
	m2 := newModel.(Model)

	assert.Contains(t, m2.statusMessage, "Copied 2 comments")
}
