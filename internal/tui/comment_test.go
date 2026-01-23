package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// =============================================================================
// Comment Row Integration Tests
// =============================================================================

// makeCommentableTestModel creates a model with Added lines that can have comments.
func makeCommentableTestModel(numLines int) Model {
	pairs := make([]sidebyside.LinePair, numLines)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "old line", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: i + 1, Content: "new line", Type: sidebyside.Added},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]string)
	return m
}

// Test: Adding a comment should increase totalLines to account for comment rows
func TestComment_AddingCommentIncreasesTotalLines(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()
	initialTotalLines := m.totalLines

	// Add a comment on the first content line
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "This is a test comment"

	// Rebuild the cache to reflect the comment
	m.rebuildRowsCache()

	// Total lines should increase by the number of comment box rows (3 minimum: top border, content, bottom border)
	assert.Greater(t, m.totalLines, initialTotalLines,
		"totalLines should increase after adding a comment (was %d, now %d)",
		initialTotalLines, m.totalLines)
}

// Test: Comment rows should be included in buildRows
func TestComment_BuildRowsIncludesCommentRows(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Count rows before comment
	rowsBefore := len(m.buildRows())

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Test comment"
	m.rowsCacheValid = false

	// Count rows after comment
	rowsAfter := len(m.buildRows())

	assert.Greater(t, rowsAfter, rowsBefore,
		"buildRows should return more rows after adding a comment")
}

// Test: Cursor should track through comment rows correctly
// This is the main bug - after adding a comment and scrolling, new comments attach to wrong line
func TestComment_CursorTracksCommentRows(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Position cursor on first content line (line 3 after top border, header, bottom border)
	// With height=30, cursor offset = 5 (20% of 28)
	// To have cursor on line 3: scroll = 3 - 5 = -2
	m.scroll = -2
	cursorPos := m.cursorLine()
	require.Equal(t, 3, cursorPos, "cursor should be on first content line")

	// Verify we can add a comment on this line
	rows := m.buildRows()
	row := rows[cursorPos]
	require.True(t, m.canComment(row), "should be able to comment on this row")

	// Add a comment on line 1 (first content line)
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Comment on line 1"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Now scroll down so cursor is on line 5 (which should be offset by comment rows)
	// The comment box adds rows, so line 5 of the original content is now at a higher row index
	// Move cursor to what should be line 3 of the file (third content line)
	// Without the fix, the cursor position calculation is wrong

	// Find the row for file line 3 in the new row list
	rows = m.buildRows()
	targetRowIdx := -1
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 3 {
			targetRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, targetRowIdx, "should find row for file line 3")

	// Position cursor on that row
	m.scroll = targetRowIdx - m.cursorOffset()
	cursorPos = m.cursorLine()

	// Now try to start a comment - it should attach to line 3, not some offset line
	success := m.startComment()
	require.True(t, success, "should be able to start a comment")

	assert.Equal(t, 3, m.commentKey.newLineNum,
		"comment should attach to file line 3, not an offset line (got line %d)", m.commentKey.newLineNum)
}

// Test: After adding comment, scrolling, and adding another comment,
// second comment should be on the visually selected line
func TestComment_MultipleCommentsCorrectlyPositioned(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add first comment on line 1
	key1 := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key1] = "First comment"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Find row for file line 5
	rows := m.buildRows()
	var line5RowIdx int
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 5 {
			line5RowIdx = i
			break
		}
	}

	// Position cursor on line 5
	m.scroll = line5RowIdx - m.cursorOffset()

	// Start and submit a comment
	success := m.startComment()
	require.True(t, success, "should be able to start comment on line 5")

	// The comment should be for line 5
	assert.Equal(t, 5, m.commentKey.newLineNum,
		"second comment should be on line 5, got line %d", m.commentKey.newLineNum)

	// Submit the comment
	m.commentInput = "Second comment"
	m.submitComment()

	// Verify we now have two comments
	assert.Len(t, m.comments, 2, "should have 2 comments")
	assert.Contains(t, m.comments, key1, "should have comment on line 1")
	key5 := commentKey{fileIndex: 0, newLineNum: 5}
	assert.Contains(t, m.comments, key5, "should have comment on line 5")
}

// Test: submitComment should invalidate the row cache
func TestComment_SubmitInvalidatesCache(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Ensure cache is valid
	_ = m.getRows()
	require.True(t, m.rowsCacheValid, "cache should be valid after getRows")

	// Start a comment
	m.scroll = 0
	rows := m.buildRows()
	// Find first commentable row
	for i, r := range rows {
		if m.canComment(r) {
			m.scroll = i - m.cursorOffset()
			break
		}
	}

	success := m.startComment()
	require.True(t, success, "should be able to start a comment")

	m.commentInput = "New comment"
	m.submitComment()

	// Cache should be invalidated after submit
	assert.False(t, m.rowsCacheValid,
		"row cache should be invalidated after submitting a comment")
}

// Test: Deleting a comment (submitting empty) should invalidate the row cache
func TestComment_DeleteInvalidatesCache(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Existing comment"
	m.calculateTotalLines()

	// Ensure cache is valid
	_ = m.getRows()
	require.True(t, m.rowsCacheValid, "cache should be valid after getRows")

	// Start editing the comment
	m.commentKey = key
	m.commentInput = m.comments[key]
	m.commentMode = true

	// Delete it (submit empty)
	m.commentInput = ""
	m.submitComment()

	assert.False(t, m.rowsCacheValid,
		"row cache should be invalidated after deleting a comment")
	assert.NotContains(t, m.comments, key, "comment should be deleted")
}

// Test: Comment rows should have a RowKind to identify them
func TestComment_RowKindExists(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Test comment"
	m.rowsCacheValid = false

	rows := m.buildRows()

	// Find comment rows
	foundCommentRow := false
	for _, r := range rows {
		if r.kind == RowKindComment {
			foundCommentRow = true
			break
		}
	}

	assert.True(t, foundCommentRow,
		"buildRows should include RowKindComment rows when comments exist")
}

// Test: Comment row should be associated with the line above it
func TestComment_RowBelongsToLineAbove(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Add a comment on line 2
	key := commentKey{fileIndex: 0, newLineNum: 2}
	m.comments[key] = "Comment on line 2"
	m.rowsCacheValid = false

	rows := m.buildRows()

	// Find the comment row(s)
	for i, r := range rows {
		if r.kind == RowKindComment {
			// The comment row should have the same file index and reference the line above
			assert.Equal(t, 0, r.fileIndex, "comment row should have correct file index")
			// The row above should be the content line with newLineNum == 2
			if i > 0 {
				prevRow := rows[i-1]
				// Either it's the first comment row (prev is content) or it's a continuation
				if prevRow.kind == RowKindContent {
					assert.Equal(t, 2, prevRow.pair.New.Num,
						"comment should follow the content row for line 2")
				}
			}
			break
		}
	}
}

// Test: Navigating with j/k should move through comment rows
func TestComment_NavigationIncludesCommentRows(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Add a multi-line comment (will take multiple rows in the box)
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Line 1\nLine 2\nLine 3"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	initialTotal := m.totalLines

	// The comment should add rows (border + 3 content lines + border = 5 rows)
	// Removing the comment should reduce totalLines
	delete(m.comments, key)
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	assert.Less(t, m.totalLines, initialTotal,
		"removing multi-line comment should reduce totalLines")
}

// Test: maxScroll should account for comment rows
func TestComment_MaxScrollAccountsForComments(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	maxScrollBefore := m.maxScroll()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Test comment"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	maxScrollAfter := m.maxScroll()

	assert.Greater(t, maxScrollAfter, maxScrollBefore,
		"maxScroll should increase after adding a comment")
}

// Test: StatusInfo should correctly report position when comments exist
func TestComment_StatusInfoCorrectWithComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add comment on line 1
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "A comment"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Position cursor on line 5 content
	rows := m.buildRows()
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 5 {
			m.scroll = i - m.cursorOffset()
			break
		}
	}

	info := m.StatusInfo()

	// CurrentLine should reflect the actual row position (including comment rows)
	// TotalLines should include comment rows
	assert.Equal(t, m.totalLines, info.TotalLines,
		"StatusInfo.TotalLines should include comment rows")
}

// =============================================================================
// Edge Cases
// =============================================================================

// Test: Comment on last line of file should work correctly
func TestComment_OnLastLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Add comment on last content line (line 5)
	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = "Comment on last line"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Should not panic or have issues
	rows := m.buildRows()
	assert.Greater(t, len(rows), 0, "should have rows")

	// Comment should appear after the last content line
	foundContent5 := false
	foundCommentAfter := false
	for _, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 5 {
			foundContent5 = true
		} else if foundContent5 && r.kind == RowKindComment {
			foundCommentAfter = true
			break
		}
	}

	assert.True(t, foundContent5, "should find content line 5")
	assert.True(t, foundCommentAfter, "should find comment after line 5")
}

// Test: Multiple comments should not interfere with each other
func TestComment_MultipleCommentsInFile(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add comments on lines 1, 3, and 5
	m.comments[commentKey{fileIndex: 0, newLineNum: 1}] = "Comment 1"
	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = "Comment 3"
	m.comments[commentKey{fileIndex: 0, newLineNum: 5}] = "Comment 5"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	rows := m.buildRows()

	// Count comment rows
	commentRowCount := 0
	for _, r := range rows {
		if r.kind == RowKindComment {
			commentRowCount++
		}
	}

	// Each comment box has at least 3 rows (top border, content, bottom border)
	assert.GreaterOrEqual(t, commentRowCount, 9,
		"should have at least 9 comment rows for 3 comments (got %d)", commentRowCount)

	// Verify all content lines are still accessible
	contentLineNums := make(map[int]bool)
	for _, r := range rows {
		if r.kind == RowKindContent {
			contentLineNums[r.pair.New.Num] = true
		}
	}

	for i := 1; i <= 10; i++ {
		assert.True(t, contentLineNums[i],
			"content line %d should still be present", i)
	}
}

// Test: Scrolling past a comment should work correctly
func TestComment_ScrollPastComment(t *testing.T) {
	m := makeCommentableTestModel(20)
	m.calculateTotalLines()

	// Add comment near the top
	key := commentKey{fileIndex: 0, newLineNum: 2}
	m.comments[key] = "A comment near the top"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Scroll to bottom
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Should be able to scroll to the end without issues
	assert.Equal(t, model.maxScroll(), model.scroll,
		"should be able to scroll to max position")

	// Cursor should be on last content line
	cursorPos := model.cursorLine()
	rows := model.buildRows()
	if cursorPos >= 0 && cursorPos < len(rows) {
		// Last few rows are the last content line (possibly followed by comment rows for it)
		// Find what line number we're on
		row := rows[cursorPos]
		assert.True(t, row.kind == RowKindContent || row.kind == RowKindComment,
			"cursor should be on content or comment row at bottom")
	}
}

// =============================================================================
// Cursor Integration Tests for Comments
// =============================================================================

// Test: Resize preserves cursor on comment row
func TestComment_ResizePreservesCursorOnCommentRow(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = "Test comment"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Find the comment row position
	rows := m.buildRows()
	commentRowIdx := -1
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 3 {
			commentRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, commentRowIdx, "should find comment row")

	// Position cursor on comment row
	m.scroll = commentRowIdx - m.cursorOffset()
	cursorPos := m.cursorLine()
	require.Equal(t, commentRowIdx, cursorPos, "cursor should be on comment row")

	// Resize the terminal
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 35})
	model := newM.(Model)

	// Cursor should still be on the comment row (same absolute position, rows are stable)
	newCursorPos := model.cursorLine()
	newRows := model.buildRows()
	assert.True(t, newRows[newCursorPos].kind == RowKindComment,
		"after resize, cursor should still be on comment row (got cursorPos=%d, kind=%d)",
		newCursorPos, newRows[newCursorPos].kind)
	assert.Equal(t, 3, newRows[newCursorPos].commentLineNum,
		"comment should still be for line 3")
}

// Test: Fold toggle with comments - cursor on comment row when file folds
func TestComment_FoldToggle_CursorOnCommentRow_JumpsToHeader(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()

	// Add a comment on a content line
	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = "Comment on line 5"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Find the comment row position
	rows := m.buildRows()
	commentRowIdx := -1
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 5 {
			commentRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, commentRowIdx, "should find comment row")

	// Position cursor on comment row
	m.scroll = commentRowIdx - m.cursorOffset()
	cursorPos := m.cursorLine()
	require.Equal(t, commentRowIdx, cursorPos, "cursor should be on comment row")

	// Fold the file (Normal -> Expanded -> Folded)
	// First toggle: Normal -> Expanded
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)
	// Second toggle: Expanded -> Folded
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(Model)

	// In folded state, comments are hidden, cursor should jump to header
	assert.Equal(t, sidebyside.FoldFolded, model.files[0].FoldLevel)

	newCursorPos := model.cursorLine()
	newRows := model.buildRows()
	assert.True(t, newRows[newCursorPos].isHeader,
		"after folding, cursor should be on header (comment is hidden)")
}

// Test: Multiple files with comments - navigation between them
func TestComment_MultipleFiles_Navigation(t *testing.T) {
	// Create two files with different comments
	pairs1 := make([]sidebyside.LinePair, 5)
	pairs2 := make([]sidebyside.LinePair, 5)
	for i := range pairs1 {
		pairs1[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "old line", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: i + 1, Content: "new line", Type: sidebyside.Added},
		}
	}
	for i := range pairs2 {
		pairs2[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: "old line", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: i + 1, Content: "new line", Type: sidebyside.Added},
		}
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/first.go", NewPath: "b/first.go", Pairs: pairs1},
		{OldPath: "a/second.go", NewPath: "b/second.go", Pairs: pairs2},
	})
	m.width = 80
	m.height = 40
	m.comments = make(map[commentKey]string)

	// Add comments in both files
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = "Comment in first file"
	m.comments[commentKey{fileIndex: 1, newLineNum: 3}] = "Comment in second file"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Navigate to second file using gj
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newM.(Model)

	info := model.StatusInfo()
	assert.Equal(t, "second.go", info.FileName, "should be on second file")

	// Navigate back to first file using gk
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = newM.(Model)

	info = model.StatusInfo()
	assert.Equal(t, "first.go", info.FileName, "should be back on first file")
}

// Test: Comment on line near hunk boundary
func TestComment_NearHunkBoundary(t *testing.T) {
	// Create a file with a gap in line numbers (hunk boundary)
	pairs := []sidebyside.LinePair{
		{Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 1, Content: "line 1 new", Type: sidebyside.Added}},
		{Old: sidebyside.Line{Num: 2, Content: "line 2", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 2, Content: "line 2 new", Type: sidebyside.Added}},
		// Gap here - next line is 100
		{Old: sidebyside.Line{Num: 100, Content: "line 100", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 100, Content: "line 100 new", Type: sidebyside.Added}},
		{Old: sidebyside.Line{Num: 101, Content: "line 101", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 101, Content: "line 101 new", Type: sidebyside.Added}},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]string)

	// Add comment on last line before hunk boundary
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = "Comment before boundary"
	// Add comment on first line after hunk boundary
	m.comments[commentKey{fileIndex: 0, newLineNum: 100}] = "Comment after boundary"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	rows := m.buildRows()

	// Find the hunk separator
	separatorIdx := -1
	for i, r := range rows {
		if r.isSeparator {
			separatorIdx = i
			break
		}
	}
	require.NotEqual(t, -1, separatorIdx, "should have a hunk separator")

	// Find comment rows
	var commentIndices []int
	for i, r := range rows {
		if r.kind == RowKindComment {
			commentIndices = append(commentIndices, i)
		}
	}

	// Should have at least 6 comment rows (3 per comment box: top border, content, bottom border)
	assert.GreaterOrEqual(t, len(commentIndices), 6,
		"should have comment rows for both comments")

	// Verify comments appear after their respective content lines (not after separator)
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 2 {
			// This comment should appear before the separator
			assert.Less(t, i, separatorIdx,
				"comment on line 2 should appear before separator")
			break
		}
	}
}

// Test: Very long comment (wrapping behavior)
func TestComment_VeryLongComment(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Add a very long comment (longer than typical terminal width)
	longComment := "This is a very long comment that should probably wrap or be truncated. " +
		"It contains multiple sentences and lots of text to test how the display handles " +
		"comments that exceed the available width."
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = longComment
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	rows := m.buildRows()

	// Find comment rows
	var commentRows []displayRow
	for _, r := range rows {
		if r.kind == RowKindComment {
			commentRows = append(commentRows, r)
		}
	}

	// Should have at least 3 rows (top border, content, bottom border)
	assert.GreaterOrEqual(t, len(commentRows), 3,
		"long comment should have at least 3 rows")

	// The comment text should be stored correctly
	assert.Equal(t, longComment, m.comments[key], "comment should be stored correctly")
}

// Test: Unicode characters in comments
func TestComment_UnicodeCharacters(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Add a comment with various unicode characters
	unicodeComment := "This has émojis 🎉 and special chars: ñ, ü, 中文, 日本語"
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = unicodeComment
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	rows := m.buildRows()

	// Find comment rows
	foundCommentRow := false
	for _, r := range rows {
		if r.kind == RowKindComment {
			foundCommentRow = true
			break
		}
	}

	assert.True(t, foundCommentRow, "should have comment rows for unicode comment")

	// Verify the comment is stored correctly
	assert.Equal(t, unicodeComment, m.comments[key], "unicode comment should be stored correctly")
}

// Test: Breadcrumb shows correctly when cursor on comment row
func TestComment_BreadcrumbOnCommentRow(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = "A comment"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Find the comment row position
	rows := m.buildRows()
	commentRowIdx := -1
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 5 {
			commentRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, commentRowIdx, "should find comment row")

	// Position cursor on comment row
	m.scroll = commentRowIdx - m.cursorOffset()

	// Get breadcrumbs (this tests that commentLineNum is used for lookups)
	row := rows[commentRowIdx]
	assert.Equal(t, 5, row.commentLineNum,
		"comment row should have correct commentLineNum for breadcrumb lookup")
}

// Test: StatusInfo reports correct position with comments above cursor
func TestComment_StatusInfo_CorrectPositionWithComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	totalBefore := m.totalLines

	// Add comments on lines 1, 2, 3
	m.comments[commentKey{fileIndex: 0, newLineNum: 1}] = "Comment 1"
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = "Comment 2"
	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = "Comment 3"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Total lines should have increased
	assert.Greater(t, m.totalLines, totalBefore,
		"totalLines should increase with comments")

	// Position cursor on line 8 content
	rows := m.buildRows()
	line8Idx := -1
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 8 {
			line8Idx = i
			break
		}
	}
	require.NotEqual(t, -1, line8Idx, "should find line 8")

	m.scroll = line8Idx - m.cursorOffset()

	info := m.StatusInfo()

	// CurrentLine should reflect the row position including comment rows
	assert.Equal(t, line8Idx+1, info.CurrentLine,
		"StatusInfo.CurrentLine should account for comment rows")
	// TotalLines should include comment rows
	assert.Equal(t, m.totalLines, info.TotalLines,
		"StatusInfo.TotalLines should include comment rows")
}

// Test: Go to top (gg) with comments present
func TestComment_GoToTop_WithComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment near the top
	m.comments[commentKey{fileIndex: 0, newLineNum: 1}] = "Comment at top"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Start somewhere in the middle
	m.scroll = 10

	// Press gg to go to top
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = newM.(Model)

	// Should be at minimum scroll (cursor on first row)
	assert.Equal(t, model.minScroll(), model.scroll,
		"gg should go to top even with comments present")
}

// Test: Go to bottom (G) with comments present
func TestComment_GoToBottom_WithComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add comments throughout
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = "Comment 2"
	m.comments[commentKey{fileIndex: 0, newLineNum: 5}] = "Comment 5"
	m.comments[commentKey{fileIndex: 0, newLineNum: 10}] = "Comment at end"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Start at top
	m.scroll = m.minScroll()

	// Press G to go to bottom
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Should be at maximum scroll (cursor on last row)
	assert.Equal(t, model.maxScroll(), model.scroll,
		"G should go to bottom even with comments present")

	// Cursor should be on valid row
	cursorPos := model.cursorLine()
	rows := model.buildRows()
	assert.True(t, cursorPos >= 0 && cursorPos < len(rows),
		"cursor should be on valid row after G")
}

// Test: Page down through comment rows
func TestComment_PageDown_ThroughComments(t *testing.T) {
	m := makeCommentableTestModel(30)
	m.height = 15 // Small viewport to ensure multiple pages
	m.calculateTotalLines()

	// Add comments sprinkled throughout
	for i := 1; i <= 30; i += 5 {
		m.comments[commentKey{fileIndex: 0, newLineNum: i}] = "Comment on line"
	}
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	initialScroll := m.scroll

	// Page down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model := newM.(Model)

	// Should have scrolled forward
	assert.Greater(t, model.scroll, initialScroll,
		"page down should increase scroll")

	// Page down again
	secondScroll := model.scroll
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = newM.(Model)

	// Should continue to scroll
	assert.GreaterOrEqual(t, model.scroll, secondScroll,
		"second page down should scroll further or stay at max")
}

// Test: Page up through comment rows
func TestComment_PageUp_ThroughComments(t *testing.T) {
	m := makeCommentableTestModel(30)
	m.height = 15
	m.calculateTotalLines()

	// Add comments sprinkled throughout
	for i := 1; i <= 30; i += 5 {
		m.comments[commentKey{fileIndex: 0, newLineNum: i}] = "Comment on line"
	}
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Start at bottom
	m.scroll = m.maxScroll()
	initialScroll := m.scroll

	// Page up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model := newM.(Model)

	// Should have scrolled backward
	assert.Less(t, model.scroll, initialScroll,
		"page up should decrease scroll")

	// Page up again
	secondScroll := model.scroll
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model = newM.(Model)

	// Should continue to scroll
	assert.LessOrEqual(t, model.scroll, secondScroll,
		"second page up should scroll further or stay at min")
}

// Test: j/k navigation includes comment rows in totalLines
func TestComment_JK_NavigationIncludesComments(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()
	totalBefore := m.totalLines

	// Add a multi-line comment
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = "Line 1\nLine 2\nLine 3"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	// Comment box adds rows: border + 3 lines + border = 5 rows
	expectedIncrease := 5
	assert.Equal(t, totalBefore+expectedIncrease, m.totalLines,
		"totalLines should increase by comment box rows (got %d, expected %d)",
		m.totalLines, totalBefore+expectedIncrease)

	// Navigate with j through all rows
	m.scroll = m.minScroll()
	visitedRows := 0
	for m.scroll < m.maxScroll() {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = newM.(Model)
		visitedRows++
	}

	// Should have visited all rows (minus cursor offset positions)
	assert.Greater(t, visitedRows, 0, "should visit multiple rows with j")
}

// Test: Comments in expanded view vs normal view
func TestComment_ExpandedVsNormalView(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = "Test comment"
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	normalTotalLines := m.totalLines
	normalRows := m.buildRows()

	// Count comment rows in normal view
	normalCommentRows := 0
	for _, r := range normalRows {
		if r.kind == RowKindComment {
			normalCommentRows++
		}
	}

	// Switch to expanded view
	m.files[0].FoldLevel = sidebyside.FoldExpanded
	// Provide some content so expanded view works
	content := make([]string, 10)
	for i := range content {
		content[i] = "content line"
	}
	m.files[0].OldContent = content
	m.files[0].NewContent = content
	m.rowsCacheValid = false
	m.rebuildRowsCache()

	expandedTotalLines := m.totalLines
	expandedRows := m.buildRows()

	// Count comment rows in expanded view
	expandedCommentRows := 0
	for _, r := range expandedRows {
		if r.kind == RowKindComment {
			expandedCommentRows++
		}
	}

	// Comment rows should exist in both views
	assert.Greater(t, normalCommentRows, 0, "should have comment rows in normal view")
	assert.Greater(t, expandedCommentRows, 0, "should have comment rows in expanded view")

	// Expanded view has more content, so total lines should be different
	// (expanded shows full file, normal shows only diff hunks)
	assert.NotEqual(t, normalTotalLines, expandedTotalLines,
		"expanded view should have different total lines than normal")
}
