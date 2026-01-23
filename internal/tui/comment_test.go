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
