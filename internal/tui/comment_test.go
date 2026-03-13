package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/comments"
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
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]*comments.Comment)
	return m
}

// makeMixedLineTypeTestModel creates a model with a mix of Added, Context, and Removed lines.
func makeMixedLineTypeTestModel() Model {
	pairs := []sidebyside.LinePair{
		// Context line (unchanged)
		{
			Old: sidebyside.Line{Num: 1, Content: "context line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "context line 1", Type: sidebyside.Context},
		},
		// Added line (new content)
		{
			Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			New: sidebyside.Line{Num: 2, Content: "added line", Type: sidebyside.Added},
		},
		// Removed line (deleted content - no new line number)
		{
			Old: sidebyside.Line{Num: 2, Content: "removed line", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
		},
		// Changed line (modified)
		{
			Old: sidebyside.Line{Num: 3, Content: "old version", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 3, Content: "new version", Type: sidebyside.Added},
		},
		// Another context line
		{
			Old: sidebyside.Line{Num: 4, Content: "context line 2", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 4, Content: "context line 2", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]*comments.Comment)
	return m
}

// =============================================================================
// Comment Line Type Tests
// =============================================================================

// Test: Context lines (unchanged lines) should be commentable
func TestComment_ContextLinesAreCommentable(t *testing.T) {
	m := makeMixedLineTypeTestModel()
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find a context line row
	var contextRow displayRow
	foundContext := false
	for _, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Type == sidebyside.Context {
			contextRow = r
			foundContext = true
			break
		}
	}
	require.True(t, foundContext, "should have a context line in the test model")

	// Context lines should be commentable
	assert.True(t, m.canComment(contextRow),
		"context lines should be commentable (have valid new line number)")
}

// Test: Added lines should be commentable
func TestComment_AddedLinesAreCommentable(t *testing.T) {
	m := makeMixedLineTypeTestModel()
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find an added line row
	var addedRow displayRow
	foundAdded := false
	for _, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Type == sidebyside.Added {
			addedRow = r
			foundAdded = true
			break
		}
	}
	require.True(t, foundAdded, "should have an added line in the test model")

	// Added lines should be commentable
	assert.True(t, m.canComment(addedRow),
		"added lines should be commentable")
}

// Test: Removed-only lines (no new line number) should NOT be commentable
func TestComment_RemovedOnlyLinesNotCommentable(t *testing.T) {
	m := makeMixedLineTypeTestModel()
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find a removed-only line (New.Num == 0)
	var removedRow displayRow
	foundRemoved := false
	for _, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 0 {
			removedRow = r
			foundRemoved = true
			break
		}
	}
	require.True(t, foundRemoved, "should have a removed-only line in the test model")

	// Removed-only lines should NOT be commentable (no new line number)
	assert.False(t, m.canComment(removedRow),
		"removed-only lines should not be commentable (no new line number)")
}

// Test: Adding and viewing a comment on a context line
func TestComment_AddCommentOnContextLine(t *testing.T) {
	m := makeMixedLineTypeTestModel()
	m.calculateTotalLines()

	// Add a comment on context line 1 (first line in our test model)
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: "Comment on context line"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	rows := m.buildRows()

	// Find comment rows
	foundCommentRow := false
	for _, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 1 {
			foundCommentRow = true
			break
		}
	}

	assert.True(t, foundCommentRow,
		"comment on context line should appear in buildRows")
}

// Test: Starting a comment on a context line via Enter key
func TestComment_StartCommentOnContextLine(t *testing.T) {
	m := makeMixedLineTypeTestModel()
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the row index for context line 1
	contextRowIdx := -1
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 1 && r.pair.New.Type == sidebyside.Context {
			contextRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, contextRowIdx, "should find context line row")

	// Position cursor on the context line
	m.w().scroll = contextRowIdx
	cursorPos := m.cursorLine()
	require.Equal(t, contextRowIdx, cursorPos, "cursor should be on context line row")

	// Try to start a comment
	success := m.startComment()
	require.True(t, success, "should be able to start a comment on context line")

	// Verify the comment key is for line 1
	assert.Equal(t, 1, m.w().commentKey.newLineNum,
		"comment should be attached to line 1")
	assert.True(t, m.w().commentMode, "should be in comment mode")
}

// Test: canComment returns false for non-content rows
func TestComment_CanCommentRequiresContentRow(t *testing.T) {
	m := makeMixedLineTypeTestModel()
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find a header row
	var headerRow displayRow
	for _, r := range rows {
		if r.kind == RowKindHeader {
			headerRow = r
			break
		}
	}

	assert.False(t, m.canComment(headerRow),
		"header rows should not be commentable")
}

// Test: Adding a comment should increase totalLines to account for comment rows
func TestComment_AddingCommentIncreasesTotalLines(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()
	initialTotalLines := m.w().totalLines

	// Add a comment on the first content line
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: "This is a test comment"}

	// Rebuild the cache to reflect the comment
	m.rebuildRowsCache()

	// Total lines should increase by the number of comment box rows (3 minimum: top border, content, bottom border)
	assert.Greater(t, m.w().totalLines, initialTotalLines,
		"totalLines should increase after adding a comment (was %d, now %d)",
		initialTotalLines, m.w().totalLines)
}

// Test: Comment rows should be included in buildRows
func TestComment_BuildRowsIncludesCommentRows(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Count rows before comment
	rowsBefore := len(m.buildRows())

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: "Test comment"}
	m.w().rowsCacheValid = false

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
	// In new model, scroll = cursorLine, so scroll = 3
	m.w().scroll = 3
	cursorPos := m.cursorLine()
	require.Equal(t, 3, cursorPos, "cursor should be on first content line")

	// Verify we can add a comment on this line
	rows := m.buildRows()
	row := rows[cursorPos]
	require.True(t, m.canComment(row), "should be able to comment on this row")

	// Add a comment on line 1 (first content line)
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: "Comment on line 1"}
	m.w().rowsCacheValid = false
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
	m.w().scroll = targetRowIdx
	cursorPos = m.cursorLine()

	// Now try to start a comment - it should attach to line 3, not some offset line
	success := m.startComment()
	require.True(t, success, "should be able to start a comment")

	assert.Equal(t, 3, m.w().commentKey.newLineNum,
		"comment should attach to file line 3, not an offset line (got line %d)", m.w().commentKey.newLineNum)
}

// Test: After adding comment, scrolling, and adding another comment,
// second comment should be on the visually selected line
func TestComment_MultipleCommentsCorrectlyPositioned(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add first comment on line 1
	key1 := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key1] = &comments.Comment{Text: "First comment"}
	m.w().rowsCacheValid = false
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
	m.w().scroll = line5RowIdx

	// Start and submit a comment
	success := m.startComment()
	require.True(t, success, "should be able to start comment on line 5")

	// The comment should be for line 5
	assert.Equal(t, 5, m.w().commentKey.newLineNum,
		"second comment should be on line 5, got line %d", m.w().commentKey.newLineNum)

	// Submit the comment
	m.w().commentInput = "Second comment"
	m.submitComment()

	// Verify we now have two comments
	assert.Len(t, m.comments, 2, "should have 2 comments")
	assert.Contains(t, m.comments, key1, "should have comment on line 1")
	key5 := commentKey{fileIndex: 0, newLineNum: 5}
	assert.Contains(t, m.comments, key5, "should have comment on line 5")
}

// Test: startComment from a comment row opens that comment for editing
func TestComment_StartFromCommentRow(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{Text: "Existing comment"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Find a comment row for this comment
	rows := m.getRows()
	commentRowIdx := -1
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 3 {
			commentRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, commentRowIdx, "should find a comment row")

	// Position cursor on the comment row
	m.w().scroll = commentRowIdx

	// Start a comment from the comment row
	success := m.startComment()
	require.True(t, success, "should be able to start editing from a comment row")

	assert.True(t, m.w().commentMode, "should be in comment mode")
	assert.Equal(t, key, m.w().commentKey, "should edit the correct comment")
	assert.Equal(t, "Existing comment", m.w().commentInput, "should load existing text")
}

// Test: Cursor stays on the same comment row after editing and submitting
func TestComment_CursorPreservedOnCommentRowAfterSubmit(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{Text: "Original"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Find a content row within the comment box (not a border)
	rows := m.getRows()
	commentContentIdx := -1
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 3 && r.commentLineIndex >= 0 {
			commentContentIdx = i
			break
		}
	}
	require.NotEqual(t, -1, commentContentIdx, "should find comment content row")

	// Position cursor on the comment content row and start editing
	m.w().scroll = commentContentIdx
	success := m.startComment()
	require.True(t, success)

	// Edit and submit
	m.w().commentInput = "Updated"
	m.submitComment()

	// Cursor should still be on the same comment content row
	rows = m.getRows()
	cursorPos := m.cursorLine()
	require.True(t, cursorPos >= 0 && cursorPos < len(rows))
	assert.Equal(t, RowKindComment, rows[cursorPos].kind,
		"cursor should still be on a comment row after submit")
	assert.Equal(t, 3, rows[cursorPos].commentLineNum,
		"cursor should be on the same comment")
}

// Test: When comment is deleted, cursor falls back to the content line above
func TestComment_CursorFallsToContentLineOnDelete(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{Text: "Will be deleted"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Find a comment row
	rows := m.getRows()
	commentRowIdx := -1
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 3 {
			commentRowIdx = i
			break
		}
	}
	require.NotEqual(t, -1, commentRowIdx, "should find a comment row")

	// Position cursor on the comment row and start editing
	m.w().scroll = commentRowIdx
	success := m.startComment()
	require.True(t, success)

	// Delete by submitting empty
	m.w().commentInput = ""
	m.submitComment()

	// Cursor should fall back to the content line the comment was on
	rows = m.getRows()
	cursorPos := m.cursorLine()
	require.True(t, cursorPos >= 0 && cursorPos < len(rows))
	assert.Equal(t, RowKindContent, rows[cursorPos].kind,
		"cursor should fall back to content line after comment deletion")
	assert.Equal(t, 3, rows[cursorPos].pair.New.Num,
		"cursor should be on the line the comment was attached to")
}

// Test: toggleResolveComment toggles resolved state from content line
func TestComment_ResolveToggleFromContentLine(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{Text: "Unresolved comment"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Position cursor on the content line (line 3)
	rows := m.getRows()
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 3 {
			m.w().scroll = i
			break
		}
	}

	// Toggle resolve
	toggled := m.toggleResolveComment()
	require.True(t, toggled, "should toggle from content line")
	assert.True(t, m.comments[key].Resolved, "comment should now be resolved")

	// Toggle again to unresolve — should work even when comment is hidden
	toggled = m.toggleResolveComment()
	require.True(t, toggled)
	assert.False(t, m.comments[key].Resolved, "comment should now be unresolved")
}

// Test: toggleResolveComment toggles resolved state from comment row
func TestComment_ResolveToggleFromCommentRow(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment on line 5
	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = &comments.Comment{Text: "Another comment"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Position cursor on a comment row
	rows := m.getRows()
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 5 {
			m.w().scroll = i
			break
		}
	}

	toggled := m.toggleResolveComment()
	require.True(t, toggled, "should toggle from comment row")
	assert.True(t, m.comments[key].Resolved, "comment should now be resolved")
}

// Test: toggleResolveComment returns false when no comment at cursor
func TestComment_ResolveToggleNoComment(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Position cursor on a content line with no comment
	rows := m.getRows()
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 1 {
			m.w().scroll = i
			break
		}
	}

	toggled := m.toggleResolveComment()
	assert.False(t, toggled, "should not toggle when no comment at cursor")
}

// Test: resolved comment displays with checkbox checked
func TestComment_ResolveToggleUpdatesDisplay(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{Text: "Test comment"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Toggle to resolved
	rows := m.getRows()
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 3 {
			m.w().scroll = i
			break
		}
	}
	m.toggleResolveComment()

	// Verify resolved flag on comment rows
	rows = m.getRows()
	for _, r := range rows {
		if r.kind == RowKindComment && r.commentLineNum == 3 {
			assert.True(t, r.commentResolved,
				"comment row should show resolved after toggle")
			break
		}
	}
}

// Test: submitComment should invalidate the row cache
func TestComment_SubmitInvalidatesCache(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Ensure cache is valid
	_ = m.getRows()
	require.True(t, m.w().rowsCacheValid, "cache should be valid after getRows")

	// Start a comment
	m.w().scroll = 0
	rows := m.buildRows()
	// Find first commentable row
	for i, r := range rows {
		if m.canComment(r) {
			m.w().scroll = i
			break
		}
	}

	success := m.startComment()
	require.True(t, success, "should be able to start a comment")

	m.w().commentInput = "New comment"
	m.submitComment()

	// Cache should be rebuilt (valid) with updated totalLines after submit
	assert.True(t, m.w().rowsCacheValid,
		"row cache should be rebuilt after submitting a comment")
}

// Test: Deleting a comment (submitting empty) should invalidate the row cache
func TestComment_DeleteInvalidatesCache(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: "Existing comment"}
	m.calculateTotalLines()

	// Ensure cache is valid
	_ = m.getRows()
	require.True(t, m.w().rowsCacheValid, "cache should be valid after getRows")

	// Start editing the comment
	m.w().commentKey = key
	m.w().commentInput = m.comments[key].Text
	m.w().commentMode = true

	// Delete it (submit empty)
	m.w().commentInput = ""
	m.submitComment()

	assert.True(t, m.w().rowsCacheValid,
		"row cache should be rebuilt after deleting a comment")
	assert.NotContains(t, m.comments, key, "comment should be deleted")
}

// Test: Comment rows should have a RowKind to identify them
func TestComment_RowKindExists(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: "Test comment"}
	m.w().rowsCacheValid = false

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
	m.comments[key] = &comments.Comment{Text: "Comment on line 2"}
	m.w().rowsCacheValid = false

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
	m.comments[key] = &comments.Comment{Text: "Line 1\nLine 2\nLine 3"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	initialTotal := m.w().totalLines

	// The comment should add rows (border + 3 content lines + border = 5 rows)
	// Removing the comment should reduce totalLines
	delete(m.comments, key)
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	assert.Less(t, m.w().totalLines, initialTotal,
		"removing multi-line comment should reduce totalLines")
}

// Test: maxScroll should account for comment rows
func TestComment_MaxScrollAccountsForComments(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	maxScrollBefore := m.maxScroll()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: "Test comment"}
	m.w().rowsCacheValid = false
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
	m.comments[key] = &comments.Comment{Text: "A comment"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Position cursor on line 5 content
	rows := m.buildRows()
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 5 {
			m.w().scroll = i
			break
		}
	}

	info := m.StatusInfo()

	// CurrentLine should reflect the actual row position (including comment rows)
	// TotalLines should include comment rows
	assert.Equal(t, m.w().totalLines, info.TotalLines,
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
	m.comments[key] = &comments.Comment{Text: "Comment on last line"}
	m.w().rowsCacheValid = false
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
	m.comments[commentKey{fileIndex: 0, newLineNum: 1}] = &comments.Comment{Text: "Comment 1"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{Text: "Comment 3"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 5}] = &comments.Comment{Text: "Comment 5"}
	m.w().rowsCacheValid = false
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
	m.comments[key] = &comments.Comment{Text: "A comment near the top"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Scroll to bottom
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Should be able to scroll to the end without issues
	assert.Equal(t, model.maxScroll(), model.w().scroll,
		"should be able to scroll to max position")

	// Cursor should be on last content line
	cursorPos := model.cursorLine()
	rows := model.buildRows()
	if cursorPos >= 0 && cursorPos < len(rows) {
		// Last few rows are the last content line (possibly followed by comment rows for it)
		// Find what line number we're on
		row := rows[cursorPos]
		assert.True(t, row.kind == RowKindContent || row.kind == RowKindComment || row.kind == RowKindBlank,
			"cursor should be on content, comment, or blank margin row at bottom")
	}
}

// =============================================================================
// Cursor Integration Tests for Comments
// =============================================================================

// Test: Resize preserves cursor on comment row
func TestComment_ResizePreservesCursorOnCommentRow(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.initialFoldSet = true // prevent resize from overriding FoldLevel
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{Text: "Test comment"}
	m.w().rowsCacheValid = false
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
	m.w().scroll = commentRowIdx
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

// Test: TAB on comment row does nothing (only works on content line)
func TestComment_FoldToggle_CursorOnCommentRow_NoEffect(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldHunks
	m.collapsedComments = make(map[commentKey]bool)
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = &comments.Comment{Text: "Comment on line 5"}
	m.w().rowsCacheValid = false
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

	m.w().scroll = commentRowIdx

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// No per-comment override should have been created
	assert.False(t, model.collapsedComments[key],
		"TAB on comment row should not toggle collapse")

	// Fold level unchanged
	assert.Equal(t, sidebyside.FoldHunks, model.fileFoldLevel(0))
}

// Test: TAB on content line with a comment collapses it, TAB again expands
func TestComment_FoldToggle_ContentLine_CollapseExpand(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldHunks
	m.collapsedComments = make(map[commentKey]bool)
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = &comments.Comment{Text: "Comment on line 5"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Find the content line for line 5
	rows := m.buildRows()
	contentIdx := -1
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 5 {
			contentIdx = i
			break
		}
	}
	require.NotEqual(t, -1, contentIdx, "should find content row for line 5")

	// Position cursor on content line
	m.w().scroll = contentIdx

	// TAB should collapse the comment
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)
	assert.True(t, model.collapsedComments[key], "comment should be collapsed")

	// TAB again should expand the comment
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)
	assert.False(t, model2.collapsedComments[key], "comment should be expanded again")

	// Verify comment rows are back
	rows2 := model2.buildRows()
	found := false
	for _, r := range rows2 {
		if r.kind == RowKindComment && r.commentLineNum == 5 {
			found = true
			break
		}
	}
	assert.True(t, found, "comment rows should reappear after expanding")
}

// Test: C key resets per-comment overrides before cycling mode (org-mode style)
func TestComment_GlobalToggle_ResetsOverridesFirst(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldHunks
	m.collapsedComments = make(map[commentKey]bool)
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = &comments.Comment{Text: "Comment on line 5"}

	// Manually collapse one comment via Tab override
	m.collapsedComments[key] = true
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// C with overrides always snaps to unresolved-only default
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	model := newM.(Model)

	assert.Equal(t, CommentShowUnresolved, model.commentDisplayMode,
		"C with overrides should always snap to unresolved-only")
	assert.Empty(t, model.collapsedComments,
		"per-comment overrides should be cleared")

	// Next C (no overrides) cycles normally to ShowAll
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	model2 := newM2.(Model)
	assert.Equal(t, CommentShowAll, model2.commentDisplayMode)

	// Tab to hide a comment in ShowAll, then C should snap back to unresolved
	model2.collapsedComments[key] = true
	model2.w().rowsCacheValid = false
	model2.rebuildRowsCache()

	newM3, _ := model2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	model3 := newM3.(Model)

	assert.Equal(t, CommentShowUnresolved, model3.commentDisplayMode,
		"C from ShowAll with overrides should snap to unresolved-only")
	assert.Empty(t, model3.collapsedComments)
}

// Test: Tab works independently of global display mode (can show a comment in ShowNone)
func TestComment_TabToggle_IndependentOfGlobalMode(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.files[0].FoldLevel = sidebyside.FoldHunks
	m.collapsedComments = make(map[commentKey]bool)
	m.commentDisplayMode = CommentShowNone
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = &comments.Comment{Text: "Comment on line 5"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// In ShowNone, comment rows should not appear
	rows := m.buildRows()
	for _, r := range rows {
		assert.False(t, r.kind == RowKindComment && r.commentLineNum == 5,
			"comment should be hidden in ShowNone mode")
	}

	// Find content line for line 5 and Tab to show it
	contentIdx := -1
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 5 {
			contentIdx = i
			break
		}
	}
	require.NotEqual(t, -1, contentIdx)
	m.w().scroll = contentIdx

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// The per-comment override should make it visible despite global ShowNone
	newRows := model.buildRows()
	found := false
	for _, r := range newRows {
		if r.kind == RowKindComment && r.commentLineNum == 5 {
			found = true
			break
		}
	}
	assert.True(t, found, "Tab should show comment even in ShowNone mode")
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
		{OldPath: "a/first.go", NewPath: "b/first.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs1},
		{OldPath: "a/second.go", NewPath: "b/second.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs2},
	})
	m.width = 80
	m.height = 40
	m.comments = make(map[commentKey]*comments.Comment)

	// Add comments in both files
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = &comments.Comment{Text: "Comment in first file"}
	m.comments[commentKey{fileIndex: 1, newLineNum: 3}] = &comments.Comment{Text: "Comment in second file"}
	m.w().rowsCacheValid = false
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
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]*comments.Comment)

	// Add comment on last line before hunk boundary
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = &comments.Comment{Text: "Comment before boundary"}
	// Add comment on first line after hunk boundary
	m.comments[commentKey{fileIndex: 0, newLineNum: 100}] = &comments.Comment{Text: "Comment after boundary"}
	m.w().rowsCacheValid = false
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
	m.comments[key] = &comments.Comment{Text: longComment}
	m.w().rowsCacheValid = false
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
	assert.Equal(t, longComment, m.comments[key].Text, "comment should be stored correctly")
}

// Test: Unicode characters in comments
func TestComment_UnicodeCharacters(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()

	// Add a comment with various unicode characters
	unicodeComment := "This has émojis 🎉 and special chars: ñ, ü, 中文, 日本語"
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = &comments.Comment{Text: unicodeComment}
	m.w().rowsCacheValid = false
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
	assert.Equal(t, unicodeComment, m.comments[key].Text, "unicode comment should be stored correctly")
}

// Test: Breadcrumb shows correctly when cursor on comment row
func TestComment_BreadcrumbOnCommentRow(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 5}
	m.comments[key] = &comments.Comment{Text: "A comment"}
	m.w().rowsCacheValid = false
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
	m.w().scroll = commentRowIdx

	// Get breadcrumbs (this tests that commentLineNum is used for lookups)
	row := rows[commentRowIdx]
	assert.Equal(t, 5, row.commentLineNum,
		"comment row should have correct commentLineNum for breadcrumb lookup")
}

// Test: StatusInfo reports correct position with comments above cursor
func TestComment_StatusInfo_CorrectPositionWithComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	totalBefore := m.w().totalLines

	// Add comments on lines 1, 2, 3
	m.comments[commentKey{fileIndex: 0, newLineNum: 1}] = &comments.Comment{Text: "Comment 1"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = &comments.Comment{Text: "Comment 2"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{Text: "Comment 3"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Total lines should have increased
	assert.Greater(t, m.w().totalLines, totalBefore,
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

	m.w().scroll = line8Idx

	info := m.StatusInfo()

	// CurrentLine should reflect the row position including comment rows
	assert.Equal(t, line8Idx+1, info.CurrentLine,
		"StatusInfo.CurrentLine should account for comment rows")
	// TotalLines should include comment rows
	assert.Equal(t, m.w().totalLines, info.TotalLines,
		"StatusInfo.TotalLines should include comment rows")
}

// Test: Go to top (gg) with comments present
func TestComment_GoToTop_WithComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add a comment near the top
	m.comments[commentKey{fileIndex: 0, newLineNum: 1}] = &comments.Comment{Text: "Comment at top"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Start somewhere in the middle
	m.w().scroll = 10

	// Press gg to go to top
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = newM.(Model)

	// Should be at minimum scroll (cursor on first row)
	assert.Equal(t, model.minScroll(), model.w().scroll,
		"gg should go to top even with comments present")
}

// Test: Go to bottom (G) with comments present
func TestComment_GoToBottom_WithComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()

	// Add comments throughout
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = &comments.Comment{Text: "Comment 2"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 5}] = &comments.Comment{Text: "Comment 5"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 10}] = &comments.Comment{Text: "Comment at end"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Start at top
	m.w().scroll = m.minScroll()

	// Press G to go to bottom
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Should be at maximum scroll (cursor on last row)
	assert.Equal(t, model.maxScroll(), model.w().scroll,
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
		m.comments[commentKey{fileIndex: 0, newLineNum: i}] = &comments.Comment{Text: "Comment on line"}
	}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	initialScroll := m.w().scroll

	// Page down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model := newM.(Model)

	// Should have scrolled forward
	assert.Greater(t, model.w().scroll, initialScroll,
		"page down should increase scroll")

	// Page down again
	secondScroll := model.w().scroll
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = newM.(Model)

	// Should continue to scroll
	assert.GreaterOrEqual(t, model.w().scroll, secondScroll,
		"second page down should scroll further or stay at max")
}

// Test: Page up through comment rows
func TestComment_PageUp_ThroughComments(t *testing.T) {
	m := makeCommentableTestModel(30)
	m.height = 15
	m.calculateTotalLines()

	// Add comments sprinkled throughout
	for i := 1; i <= 30; i += 5 {
		m.comments[commentKey{fileIndex: 0, newLineNum: i}] = &comments.Comment{Text: "Comment on line"}
	}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Start at bottom
	m.w().scroll = m.maxScroll()
	initialScroll := m.w().scroll

	// Page up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model := newM.(Model)

	// Should have scrolled backward
	assert.Less(t, model.w().scroll, initialScroll,
		"page up should decrease scroll")

	// Page up again
	secondScroll := model.w().scroll
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model = newM.(Model)

	// Should continue to scroll
	assert.LessOrEqual(t, model.w().scroll, secondScroll,
		"second page up should scroll further or stay at min")
}

// Test: j/k navigation includes comment rows in totalLines
func TestComment_JK_NavigationIncludesComments(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.calculateTotalLines()
	totalBefore := m.w().totalLines

	// Add a multi-line comment
	m.comments[commentKey{fileIndex: 0, newLineNum: 2}] = &comments.Comment{Text: "Line 1\nLine 2\nLine 3"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	// Comment box adds rows: border + meta + blank + 3 lines + border = 7 rows
	expectedIncrease := 7
	assert.Equal(t, totalBefore+expectedIncrease, m.w().totalLines,
		"totalLines should increase by comment box rows (got %d, expected %d)",
		m.w().totalLines, totalBefore+expectedIncrease)

	// Navigate with j through all rows
	m.w().scroll = m.minScroll()
	visitedRows := 0
	for m.w().scroll < m.maxScroll() {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = newM.(Model)
		visitedRows++
	}

	// Should have visited all rows (minus cursor offset positions)
	assert.Greater(t, visitedRows, 0, "should visit multiple rows with j")
}

// Test: Comments appear in expanded (hunk) view but not in folded view
func TestComment_ExpandedVsNormalView(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.files[0].FoldLevel = sidebyside.FoldHunks
	m.calculateTotalLines()

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = &comments.Comment{Text: "Test comment"}
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	expandedRows := m.buildRows()

	// Count comment rows in expanded (hunk) view
	expandedCommentRows := 0
	for _, r := range expandedRows {
		if r.kind == RowKindComment {
			expandedCommentRows++
		}
	}

	// Switch to folded view
	m.files[0].FoldLevel = sidebyside.FoldHeader
	m.w().rowsCacheValid = false
	m.rebuildRowsCache()

	foldedRows := m.buildRows()

	// Count comment rows in folded view
	foldedCommentRows := 0
	for _, r := range foldedRows {
		if r.kind == RowKindComment {
			foldedCommentRows++
		}
	}

	// Comment rows should exist in expanded view but not folded
	assert.Greater(t, expandedCommentRows, 0, "should have comment rows in expanded view")
	assert.Equal(t, 0, foldedCommentRows, "should have no comment rows in folded view")
}

// =============================================================================
// Comment Input Navigation Tests
// =============================================================================

// Test: Up arrow moves cursor to previous line
func TestComment_MoveUp_BasicNavigation(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "line1\nline2\nline3"
	m.w().commentCursor = 12 // at 'l' of "line3"

	m.commentMoveUp()

	// Should be at same column on line2
	assert.Equal(t, 6, m.w().commentCursor, "cursor should move to line2")
}

// Test: Down arrow moves cursor to next line
func TestComment_MoveDown_BasicNavigation(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "line1\nline2\nline3"
	m.w().commentCursor = 0 // at 'l' of "line1"

	m.commentMoveDown()

	// Should be at same column on line2
	assert.Equal(t, 6, m.w().commentCursor, "cursor should move to line2")
}

// Test: Up arrow on first line does nothing
func TestComment_MoveUp_FirstLine_NoOp(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "line1\nline2"
	m.w().commentCursor = 2 // in middle of "line1"

	m.commentMoveUp()

	assert.Equal(t, 2, m.w().commentCursor, "cursor should stay on first line")
}

// Test: Down arrow on last line does nothing
func TestComment_MoveDown_LastLine_NoOp(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "line1\nline2"
	m.w().commentCursor = 8 // in middle of "line2"

	m.commentMoveDown()

	assert.Equal(t, 8, m.w().commentCursor, "cursor should stay on last line")
}

// Test: Up arrow preserves column position
func TestComment_MoveUp_PreservesColumn(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "abcdef\nghijkl"
	m.w().commentCursor = 10 // at 'j' in "ghijkl" (col 3)

	m.commentMoveUp()

	// Should be at 'd' in "abcdef" (col 3)
	assert.Equal(t, 3, m.w().commentCursor)
}

// Test: Down arrow preserves column position
func TestComment_MoveDown_PreservesColumn(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "abcdef\nghijkl"
	m.w().commentCursor = 3 // at 'd' in "abcdef" (col 3)

	m.commentMoveDown()

	// Should be at 'j' in "ghijkl" (col 3)
	assert.Equal(t, 10, m.w().commentCursor)
}

// Test: Up arrow clamps to shorter line
func TestComment_MoveUp_ClampsToShorterLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "short\nvery long line"
	m.w().commentCursor = 15 // near end of "very long line"

	m.commentMoveUp()

	// Should clamp to end of "short" (position 5)
	assert.Equal(t, 5, m.w().commentCursor)
}

// Test: Down arrow clamps to shorter line
func TestComment_MoveDown_ClampsToShorterLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "very long line\nshort"
	m.w().commentCursor = 10 // at 'l' in "very long line" (col 10)

	m.commentMoveDown()

	// Should clamp to end of "short" (position 15+5=20)
	assert.Equal(t, 20, m.w().commentCursor)
}

// Test: Up/Down with key message
func TestComment_UpDownKeys_Integration(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "line1\nline2\nline3"
	m.w().commentCursor = 12 // at 'l' of "line3"

	// Press Up
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := newModel.(Model)

	assert.Equal(t, 6, m2.w().commentCursor, "Up key should move to previous line")

	// Press Down
	newModel, _ = m2.Update(tea.KeyMsg{Type: tea.KeyDown})
	m3 := newModel.(Model)

	assert.Equal(t, 12, m3.w().commentCursor, "Down key should move back to next line")
}

// Test: Ctrl+P and Ctrl+N also work for up/down
func TestComment_CtrlPN_Navigation(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "line1\nline2"
	m.w().commentCursor = 6 // at 'l' of "line2"

	// Press Ctrl+P (up)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m2 := newModel.(Model)

	assert.Equal(t, 0, m2.w().commentCursor, "Ctrl+P should move to previous line")

	// Press Ctrl+N (down)
	newModel, _ = m2.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m3 := newModel.(Model)

	assert.Equal(t, 6, m3.w().commentCursor, "Ctrl+N should move to next line")
}

// =============================================================================
// Comment Editing Primitive Tests
// =============================================================================

// Test: insertCommentRune inserts at cursor
func TestComment_InsertRune(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 2

	m.insertCommentRune('X')

	assert.Equal(t, "heXllo", m.w().commentInput)
	assert.Equal(t, 3, m.w().commentCursor)
}

// Test: insertCommentRune handles unicode
func TestComment_InsertRune_Unicode(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 5

	m.insertCommentRune('世')

	assert.Equal(t, "hello世", m.w().commentInput)
}

// Test: commentDeleteBackward at start does nothing
func TestComment_DeleteBackward_AtStart(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 0

	m.commentDeleteBackward()

	assert.Equal(t, "hello", m.w().commentInput)
	assert.Equal(t, 0, m.w().commentCursor)
}

// Test: commentDeleteBackward deletes previous char
func TestComment_DeleteBackward_Middle(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 3

	m.commentDeleteBackward()

	assert.Equal(t, "helo", m.w().commentInput)
	assert.Equal(t, 2, m.w().commentCursor)
}

// Test: commentDeleteForward at end does nothing
func TestComment_DeleteForward_AtEnd(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 5

	m.commentDeleteForward()

	assert.Equal(t, "hello", m.w().commentInput)
	assert.Equal(t, 5, m.w().commentCursor)
}

// Test: commentDeleteForward deletes next char
func TestComment_DeleteForward_Middle(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 2

	m.commentDeleteForward()

	assert.Equal(t, "helo", m.w().commentInput)
	assert.Equal(t, 2, m.w().commentCursor)
}

// Test: commentMoveForward at end does nothing
func TestComment_MoveForward_AtEnd(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 5

	m.commentMoveForward()

	assert.Equal(t, 5, m.w().commentCursor)
}

// Test: commentMoveForward moves one char
func TestComment_MoveForward_Middle(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 2

	m.commentMoveForward()

	assert.Equal(t, 3, m.w().commentCursor)
}

// Test: commentMoveBack at start does nothing
func TestComment_MoveBack_AtStart(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 0

	m.commentMoveBack()

	assert.Equal(t, 0, m.w().commentCursor)
}

// Test: commentMoveBack moves one char
func TestComment_MoveBack_Middle(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello"
	m.w().commentCursor = 3

	m.commentMoveBack()

	assert.Equal(t, 2, m.w().commentCursor)
}

// Test: commentMoveLineStart on first line
func TestComment_MoveLineStart_FirstLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 3

	m.commentMoveLineStart()

	assert.Equal(t, 0, m.w().commentCursor)
}

// Test: commentMoveLineStart on second line
func TestComment_MoveLineStart_SecondLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 9 // in "world"

	m.commentMoveLineStart()

	assert.Equal(t, 6, m.w().commentCursor) // start of "world"
}

// Test: commentMoveLineEnd on first line
func TestComment_MoveLineEnd_FirstLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 2

	m.commentMoveLineEnd()

	assert.Equal(t, 5, m.w().commentCursor) // before newline
}

// Test: commentMoveLineEnd on last line
func TestComment_MoveLineEnd_LastLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 7

	m.commentMoveLineEnd()

	assert.Equal(t, 11, m.w().commentCursor) // end of input
}

// Test: commentKillToEnd kills to newline
func TestComment_KillToEnd_MiddleLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 2

	m.commentKillToEnd()

	assert.Equal(t, "he\nworld", m.w().commentInput)
}

// Test: commentKillToEnd kills to end of input
func TestComment_KillToEnd_LastLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 8

	m.commentKillToEnd()

	assert.Equal(t, "hello\nwo", m.w().commentInput)
}

// Test: commentKillToStart kills to beginning of line (first line)
func TestComment_KillToStart_FirstLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 3

	m.commentKillToStart()

	assert.Equal(t, "lo\nworld", m.w().commentInput)
	assert.Equal(t, 0, m.w().commentCursor)
}

// Test: commentKillToStart kills to newline (not first line)
func TestComment_KillToStart_SecondLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 9 // at 'l' in "world"

	m.commentKillToStart()

	assert.Equal(t, "hello\nld", m.w().commentInput)
	assert.Equal(t, 6, m.w().commentCursor)
}

// Test: commentKillToStart at beginning of line is a no-op
func TestComment_KillToStart_AtLineStart(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello\nworld"
	m.w().commentCursor = 6 // at start of "world"

	m.commentKillToStart()

	assert.Equal(t, "hello\nworld", m.w().commentInput)
	assert.Equal(t, 6, m.w().commentCursor)
}

// Test: Ctrl+U key triggers kill to start
func TestComment_CtrlU_KillsToStart(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello world"
	m.w().commentCursor = 6 // at 'w'

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m2 := newModel.(Model)

	assert.Equal(t, "world", m2.w().commentInput)
	assert.Equal(t, 0, m2.w().commentCursor)
	assert.True(t, m2.w().commentMode)
}

// =============================================================================
// Paste Tests
// =============================================================================

// Test: commentPaste inserts single line text at cursor
func TestComment_Paste_SingleLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "hello world"
	m.w().commentCursor = 6 // at 'w'

	// Simulate paste by directly calling the insert logic
	pasteText := "beautiful "
	before := m.w().commentInput[:m.w().commentCursor]
	after := m.w().commentInput[m.w().commentCursor:]
	m.w().commentInput = before + pasteText + after
	m.w().commentCursor += len(pasteText)

	assert.Equal(t, "hello beautiful world", m.w().commentInput)
	assert.Equal(t, 16, m.w().commentCursor)
}

// Test: commentPaste inserts multi-line text at cursor
func TestComment_Paste_MultiLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "start end"
	m.w().commentCursor = 6 // at 'e' in "end"

	// Simulate paste of multi-line content
	pasteText := "line1\nline2\nline3 "
	before := m.w().commentInput[:m.w().commentCursor]
	after := m.w().commentInput[m.w().commentCursor:]
	m.w().commentInput = before + pasteText + after
	m.w().commentCursor += len(pasteText)

	assert.Equal(t, "start line1\nline2\nline3 end", m.w().commentInput)
	assert.Equal(t, 24, m.w().commentCursor) // 6 + len("line1\nline2\nline3 ") = 6 + 18 = 24
}

// Test: paste into empty comment
func TestComment_Paste_IntoEmpty(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = ""
	m.w().commentCursor = 0

	pasteText := "first line\nsecond line"
	before := m.w().commentInput[:m.w().commentCursor]
	after := m.w().commentInput[m.w().commentCursor:]
	m.w().commentInput = before + pasteText + after
	m.w().commentCursor += len(pasteText)

	assert.Equal(t, "first line\nsecond line", m.w().commentInput)
	assert.Equal(t, 22, m.w().commentCursor)
}

// Test: paste at end of existing multi-line comment
func TestComment_Paste_AtEndOfMultiLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "existing\ncomment"
	m.w().commentCursor = 16 // at end

	pasteText := "\nnew line"
	before := m.w().commentInput[:m.w().commentCursor]
	after := m.w().commentInput[m.w().commentCursor:]
	m.w().commentInput = before + pasteText + after
	m.w().commentCursor += len(pasteText)

	assert.Equal(t, "existing\ncomment\nnew line", m.w().commentInput)
	assert.Equal(t, 25, m.w().commentCursor)
}

// Test: cursor position is correct after multi-line paste
func TestComment_Paste_CursorPositionAfterMultiLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "AB"
	m.w().commentCursor = 1 // between A and B

	pasteText := "X\nY\nZ"
	before := m.w().commentInput[:m.w().commentCursor]
	after := m.w().commentInput[m.w().commentCursor:]
	m.w().commentInput = before + pasteText + after
	m.w().commentCursor += len(pasteText)

	// Result should be "AX\nY\nZB" with cursor after Z (before B)
	assert.Equal(t, "AX\nY\nZB", m.w().commentInput)
	assert.Equal(t, 6, m.w().commentCursor) // 1 + 5 = 6

	// Verify cursor is at the right position by checking what's before/after
	assert.Equal(t, "AX\nY\nZ", m.w().commentInput[:m.w().commentCursor])
	assert.Equal(t, "B", m.w().commentInput[m.w().commentCursor:])
}

// Test: renderCommentPrompt with multi-line input shows all lines
func TestComment_RenderPrompt_MultiLine(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.width = 80
	m.w().commentMode = true
	m.w().commentInput = "line1\nline2\nline3"
	m.w().commentCursor = 12 // at 'l' of "line3"

	// renderCommentPrompt is called via renderStatusBar
	output := m.renderStatusBar()

	lines := strings.Split(output, "\n")

	// Should have 4 lines: 3 content lines + 1 help line
	assert.Equal(t, 4, len(lines), "should have 4 lines: 3 content + 1 help")

	// Check that each line has the right prefix
	assert.True(t, strings.HasPrefix(lines[0], " . "), "first line should have continuation prefix")
	assert.True(t, strings.HasPrefix(lines[1], " . "), "second line should have continuation prefix")
	assert.True(t, strings.HasPrefix(lines[2], " > "), "third (cursor) line should have cursor prefix")
	assert.True(t, strings.Contains(lines[3], "C-j to submit"), "last line should be help text")

	// Verify content is present
	assert.Contains(t, lines[0], "line1")
	assert.Contains(t, lines[1], "line2")
	assert.Contains(t, lines[2], "line3")
}

// Test: renderCommentPrompt with pasted content ending in newline
func TestComment_RenderPrompt_TrailingNewline(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.width = 80
	m.w().commentMode = true
	m.w().commentInput = "pasted\n" // trailing newline creates empty line
	m.w().commentCursor = 7         // after the newline (on empty line)

	output := m.renderStatusBar()
	lines := strings.Split(output, "\n")

	// Should have 3 lines: "pasted", empty line (cursor), help
	assert.Equal(t, 3, len(lines), "should have 3 lines: content, empty cursor line, help")

	// First line should have continuation prefix (cursor is on second line)
	assert.True(t, strings.HasPrefix(lines[0], " . "), "first line should have continuation prefix")
	assert.Contains(t, lines[0], "pasted")

	// Second line should have cursor prefix (it's the empty line after newline)
	assert.True(t, strings.HasPrefix(lines[1], " > "), "second line should have cursor prefix")
}

// Test: paste with trailing newline updates cursor correctly
func TestComment_Paste_TrailingNewline(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = ""
	m.w().commentCursor = 0

	// Simulate pasting text with trailing newline (common when copying lines)
	pasteText := "copied line\n"
	m.w().commentInput = pasteText
	m.w().commentCursor = len(pasteText)

	// Cursor should be at position 12 (after the newline)
	assert.Equal(t, 12, m.w().commentCursor)

	// The input should be "copied line\n"
	assert.Equal(t, "copied line\n", m.w().commentInput)

	// When we split this, we get ["copied line", ""]
	lines := strings.Split(m.w().commentInput, "\n")
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, "copied line", lines[0])
	assert.Equal(t, "", lines[1])
}

// Test: Full View() with multi-line comment input contains all lines
func TestComment_FullView_MultiLineInput(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.width = 80
	m.height = 30
	m.w().commentMode = true
	m.w().commentInput = "first line\nsecond line\nthird line"
	m.w().commentCursor = 33 // at end

	m.calculateTotalLines()
	output := m.View()

	// The view should contain all three comment input lines
	assert.Contains(t, output, "first line", "view should contain first line")
	assert.Contains(t, output, "second line", "view should contain second line")
	assert.Contains(t, output, "third line", "view should contain third line")

	// Should have the cursor indicator on the last line
	assert.Contains(t, output, " > ", "view should have cursor indicator")

	// Should have continuation indicators on other lines
	assert.Contains(t, output, " . ", "view should have continuation indicators")
}

// Test: commentPromptHeight calculation with multi-line input
func TestComment_PromptHeight_MultiLine(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Not in comment mode - should return 1
	m.w().commentMode = false
	assert.Equal(t, 1, m.commentPromptHeight())

	// In comment mode with single line
	m.w().commentMode = true
	m.w().commentInput = "single line"
	assert.Equal(t, 2, m.commentPromptHeight()) // 1 content line + 1 help line

	// In comment mode with multiple lines
	m.w().commentInput = "line1\nline2\nline3"
	assert.Equal(t, 4, m.commentPromptHeight()) // 3 content lines + 1 help line

	// With trailing newline (adds empty line)
	m.w().commentInput = "line1\nline2\n"
	assert.Equal(t, 4, m.commentPromptHeight()) // 3 lines (including empty) + 1 help line
}

// Test: commentMaxVisibleLines returns max(10, 20% of height)
func TestComment_MaxVisibleLines(t *testing.T) {
	m := makeCommentableTestModel(5)

	// Small viewport - should return 10 (minimum)
	m.height = 30 // 20% = 6, so min is 10
	assert.Equal(t, 10, m.commentMaxVisibleLines())

	// Large viewport - should return 20% of height
	m.height = 100 // 20% = 20
	assert.Equal(t, 20, m.commentMaxVisibleLines())

	// Exactly at threshold
	m.height = 50 // 20% = 10
	assert.Equal(t, 10, m.commentMaxVisibleLines())
}

// Test: comment scrolling with many lines
func TestComment_Scrolling_ManyLines(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.height = 30 // maxVisible = 10
	m.w().commentMode = true

	// Create 15 lines of content
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = "line" + string(rune('A'+i))
	}
	m.w().commentInput = strings.Join(lines, "\n")

	// Initially scroll is 0, cursor at end
	m.w().commentCursor = len(m.w().commentInput)
	m.w().commentScroll = 0
	m.commentEnsureCursorVisible()

	// Cursor is on line 14 (0-indexed), maxVisible is 10
	// Scroll should be at least 14 - 10 + 1 = 5
	assert.GreaterOrEqual(t, m.w().commentScroll, 5)

	// commentPromptHeight should include scroll indicators
	// We have 15 lines, showing 10, scroll > 0 so indicator above
	// scroll + 10 < 15 so indicator below
	height := m.commentPromptHeight()
	// 10 visible lines + up to 2 indicators + 1 help line
	assert.LessOrEqual(t, height, 13)
	assert.GreaterOrEqual(t, height, 11) // at least 10 lines + 1 help

	// Move cursor to beginning
	m.w().commentCursor = 0
	m.commentEnsureCursorVisible()

	// Scroll should be 0 now
	assert.Equal(t, 0, m.w().commentScroll)
}

// Test: cursor movement keeps cursor visible in scroll window
func TestComment_CursorMovement_KeepsVisible(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.height = 30 // maxVisible = 10
	m.w().commentMode = true

	// Create 20 lines of content
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "content"
	}
	m.w().commentInput = strings.Join(lines, "\n")
	m.w().commentCursor = 0
	m.w().commentScroll = 0

	// Move down repeatedly
	for i := 0; i < 15; i++ {
		m.commentMoveDown()
	}

	// Cursor should be on line 15
	assert.Equal(t, 15, m.commentCursorLineIndex())

	// Scroll should have adjusted to keep cursor visible
	// With maxVisible=10, scroll should be at least 15 - 10 + 1 = 6
	assert.GreaterOrEqual(t, m.w().commentScroll, 6)

	// Move back up
	for i := 0; i < 15; i++ {
		m.commentMoveUp()
	}

	// Cursor should be on line 0
	assert.Equal(t, 0, m.commentCursorLineIndex())

	// Scroll should be 0
	assert.Equal(t, 0, m.w().commentScroll)
}

// Test: scroll indicators appear when content is scrolled
func TestComment_ScrollIndicators_Rendering(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.height = 30 // maxVisible = 10
	m.width = 80
	m.w().commentMode = true

	// Create 15 lines of content
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = "line" + string(rune('A'+i))
	}
	m.w().commentInput = strings.Join(lines, "\n")

	// Scroll to middle (show both indicators)
	m.w().commentScroll = 3
	m.w().commentCursor = 50 // somewhere in middle

	output := m.renderCommentPrompt()

	// Should have "more lines" indicator at top
	assert.Contains(t, output, "↑")
	assert.Contains(t, output, "more line")

	// Should have "more lines" indicator at bottom
	assert.Contains(t, output, "↓")

	// Should still have the help line
	assert.Contains(t, output, "C-j to submit")

	// Scroll to top - only down indicator
	m.w().commentScroll = 0
	output = m.renderCommentPrompt()
	assert.NotContains(t, output, "↑")
	assert.Contains(t, output, "↓")

	// Scroll to bottom - only up indicator
	m.w().commentScroll = 5 // 15 lines - 10 visible = 5 max scroll
	output = m.renderCommentPrompt()
	assert.Contains(t, output, "↑")
	assert.NotContains(t, output, "↓ ")
}

// Test: cursor position stays stable as comment prompt grows
func TestComment_CursorPosition_StableWithGrowingPrompt(t *testing.T) {
	m := makeCommentableTestModel(20)
	m.height = 40
	m.width = 80
	m.w().scroll = 5

	// Record cursor position before entering comment mode
	cursorLineBefore := m.cursorLine()
	cursorOffsetBefore := m.cursorOffset()

	// Enter comment mode
	m.w().commentMode = true
	m.w().commentInput = "short"

	// Cursor calculations should use baseContentHeight, so they stay stable
	assert.Equal(t, cursorOffsetBefore, m.cursorOffset(), "cursorOffset should not change with small comment")
	assert.Equal(t, cursorLineBefore, m.cursorLine(), "cursorLine should not change with small comment")

	// Grow the comment significantly
	m.w().commentInput = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8"

	// Cursor calculations should STILL be stable
	assert.Equal(t, cursorOffsetBefore, m.cursorOffset(), "cursorOffset should not change with large comment")
	assert.Equal(t, cursorLineBefore, m.cursorLine(), "cursorLine should not change with large comment")

	// But contentHeight (for rendering) should have shrunk
	baseHeight := m.baseContentHeight()
	renderHeight := m.contentHeight()
	assert.Less(t, renderHeight, baseHeight, "contentHeight should be smaller than baseContentHeight when comment prompt is large")
}

// Test: paste normalizes line endings and removes problematic characters
func TestComment_Paste_SanitizesText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unix line endings unchanged",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Windows CRLF converted to LF",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Old Mac CR converted to LF",
			input:    "line1\rline2\rline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Mixed line endings normalized",
			input:    "unix\nwindows\r\nmac\r",
			expected: "unix\nwindows\nmac",
		},
		{
			name:     "Zero-width space removed",
			input:    "hello\u200Bworld",
			expected: "helloworld",
		},
		{
			name:     "BOM removed",
			input:    "\uFEFFhello",
			expected: "hello",
		},
		{
			name:     "Direction marks preserved",
			input:    "hello\u200Eworld\u200F",
			expected: "hello\u200Eworld\u200F",
		},
		{
			name:     "Control characters removed",
			input:    "hello\x00\x01\x02world",
			expected: "helloworld",
		},
		{
			name:     "Tabs preserved",
			input:    "hello\tworld",
			expected: "hello\tworld",
		},
		{
			name:     "Unicode line separator converted",
			input:    "hello\u2028world",
			expected: "hello\nworld",
		},
		{
			name:     "Trailing newline stripped",
			input:    "hello\n",
			expected: "hello",
		},
		{
			name:     "Trailing whitespace stripped",
			input:    "hello  \t\n",
			expected: "hello",
		},
		{
			name:     "Internal newlines preserved",
			input:    "hello\nworld\n",
			expected: "hello\nworld",
		},
		{
			name:     "Trailing NO-BREAK SPACE stripped",
			input:    "hello\u00A0",
			expected: "hello",
		},
		{
			name:     "Trailing Unicode whitespace stripped",
			input:    "hello\u2003\u2009", // EM SPACE + THIN SPACE
			expected: "hello",
		},
		{
			name:     "Internal NO-BREAK SPACE preserved",
			input:    "hello\u00A0world",
			expected: "hello\u00A0world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePastedText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test: cancelComment exits comment mode
func TestComment_Cancel(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.w().commentMode = true
	m.w().commentInput = "some text"
	m.w().commentCursor = 5

	m.cancelComment()

	assert.False(t, m.w().commentMode)
	assert.Empty(t, m.w().commentInput)
	assert.Equal(t, 0, m.w().commentCursor)
}

// Test: handleCommentInput dispatches keys correctly
func TestComment_HandleInput_AllKeys(t *testing.T) {
	tests := []struct {
		name       string
		key        tea.KeyMsg
		input      string
		cursor     int
		wantInput  string
		wantCursor int
		wantMode   bool
	}{
		{
			name:       "Ctrl+C cancels",
			key:        tea.KeyMsg{Type: tea.KeyCtrlC},
			input:      "test",
			cursor:     2,
			wantInput:  "",
			wantCursor: 0,
			wantMode:   false,
		},
		{
			name:       "Ctrl+G cancels",
			key:        tea.KeyMsg{Type: tea.KeyCtrlG},
			input:      "test",
			cursor:     2,
			wantInput:  "",
			wantCursor: 0,
			wantMode:   false,
		},
		{
			name:       "Escape cancels",
			key:        tea.KeyMsg{Type: tea.KeyEsc},
			input:      "test",
			cursor:     2,
			wantInput:  "",
			wantCursor: 0,
			wantMode:   false,
		},
		{
			name:       "Enter inserts newline",
			key:        tea.KeyMsg{Type: tea.KeyEnter},
			input:      "ab",
			cursor:     1,
			wantInput:  "a\nb",
			wantCursor: 2,
			wantMode:   true,
		},
		{
			name:       "Backspace deletes",
			key:        tea.KeyMsg{Type: tea.KeyBackspace},
			input:      "abc",
			cursor:     2,
			wantInput:  "ac",
			wantCursor: 1,
			wantMode:   true,
		},
		{
			name:       "Delete forward",
			key:        tea.KeyMsg{Type: tea.KeyDelete},
			input:      "abc",
			cursor:     1,
			wantInput:  "ac",
			wantCursor: 1,
			wantMode:   true,
		},
		{
			name:       "Ctrl+H deletes backward",
			key:        tea.KeyMsg{Type: tea.KeyCtrlH},
			input:      "abc",
			cursor:     2,
			wantInput:  "ac",
			wantCursor: 1,
			wantMode:   true,
		},
		{
			name:       "Ctrl+D deletes forward",
			key:        tea.KeyMsg{Type: tea.KeyCtrlD},
			input:      "abc",
			cursor:     1,
			wantInput:  "ac",
			wantCursor: 1,
			wantMode:   true,
		},
		{
			name:       "Ctrl+A moves to line start",
			key:        tea.KeyMsg{Type: tea.KeyCtrlA},
			input:      "hello",
			cursor:     3,
			wantInput:  "hello",
			wantCursor: 0,
			wantMode:   true,
		},
		{
			name:       "Ctrl+E moves to line end",
			key:        tea.KeyMsg{Type: tea.KeyCtrlE},
			input:      "hello",
			cursor:     2,
			wantInput:  "hello",
			wantCursor: 5,
			wantMode:   true,
		},
		{
			name:       "Ctrl+F moves forward",
			key:        tea.KeyMsg{Type: tea.KeyCtrlF},
			input:      "hello",
			cursor:     2,
			wantInput:  "hello",
			wantCursor: 3,
			wantMode:   true,
		},
		{
			name:       "Right moves forward",
			key:        tea.KeyMsg{Type: tea.KeyRight},
			input:      "hello",
			cursor:     2,
			wantInput:  "hello",
			wantCursor: 3,
			wantMode:   true,
		},
		{
			name:       "Ctrl+B moves back",
			key:        tea.KeyMsg{Type: tea.KeyCtrlB},
			input:      "hello",
			cursor:     3,
			wantInput:  "hello",
			wantCursor: 2,
			wantMode:   true,
		},
		{
			name:       "Left moves back",
			key:        tea.KeyMsg{Type: tea.KeyLeft},
			input:      "hello",
			cursor:     3,
			wantInput:  "hello",
			wantCursor: 2,
			wantMode:   true,
		},
		{
			name:       "Ctrl+K kills to end",
			key:        tea.KeyMsg{Type: tea.KeyCtrlK},
			input:      "hello",
			cursor:     2,
			wantInput:  "he",
			wantCursor: 2,
			wantMode:   true,
		},
		{
			name:       "Ctrl+U kills to start",
			key:        tea.KeyMsg{Type: tea.KeyCtrlU},
			input:      "hello",
			cursor:     3,
			wantInput:  "lo",
			wantCursor: 0,
			wantMode:   true,
		},
		{
			name:       "Space inserts space",
			key:        tea.KeyMsg{Type: tea.KeySpace},
			input:      "ab",
			cursor:     1,
			wantInput:  "a b",
			wantCursor: 2,
			wantMode:   true,
		},
		{
			name:       "Runes insert text",
			key:        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}},
			input:      "ab",
			cursor:     1,
			wantInput:  "aXb",
			wantCursor: 2,
			wantMode:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeCommentableTestModel(5)
			m.w().commentMode = true
			m.w().commentInput = tt.input
			m.w().commentCursor = tt.cursor

			newModel, _ := m.Update(tt.key)
			m2 := newModel.(Model)

			assert.Equal(t, tt.wantInput, m2.w().commentInput, "input mismatch")
			assert.Equal(t, tt.wantCursor, m2.w().commentCursor, "cursor mismatch")
			assert.Equal(t, tt.wantMode, m2.w().commentMode, "mode mismatch")
		})
	}
}

// =============================================================================
// Word Wrap Tests
// =============================================================================

// Test: wrapWithOffsets wraps at word boundaries and returns correct byte offsets
func TestWrapWithOffsets(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		maxWidth    int
		wantLines   []string
		wantOffsets []int
	}{
		{
			name:        "short line no wrap",
			line:        "hello",
			maxWidth:    20,
			wantLines:   []string{"hello"},
			wantOffsets: []int{0},
		},
		{
			name:        "exact width no wrap",
			line:        "hello",
			maxWidth:    5,
			wantLines:   []string{"hello"},
			wantOffsets: []int{0},
		},
		{
			name:        "two words wrap",
			line:        "hello world",
			maxWidth:    5,
			wantLines:   []string{"hello", "world"},
			wantOffsets: []int{0, 6},
		},
		{
			name:        "three words two wraps",
			line:        "one two three",
			maxWidth:    7,
			wantLines:   []string{"one two", "three"},
			wantOffsets: []int{0, 8},
		},
		{
			name:        "empty line",
			line:        "",
			maxWidth:    10,
			wantLines:   []string{""},
			wantOffsets: []int{0},
		},
		{
			name:        "single long word no break possible",
			line:        "abcdefghij",
			maxWidth:    5,
			wantLines:   []string{"abcdefghij"},
			wantOffsets: []int{0},
		},
		{
			name:        "zero width returns original",
			line:        "hello",
			maxWidth:    0,
			wantLines:   []string{"hello"},
			wantOffsets: []int{0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, offsets := wrapWithOffsets(tt.line, tt.maxWidth)
			assert.Equal(t, tt.wantLines, lines, "lines mismatch")
			assert.Equal(t, tt.wantOffsets, offsets, "offsets mismatch")
		})
	}
}

// Test: commentCursorVisualPos maps cursor position through word wrapping
func TestCommentCursorVisualPos(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		cursor    int
		wrapWidth int
		wantLine  int
		wantCol   int
	}{
		{
			name:      "cursor at start of unwrapped line",
			input:     "hello",
			cursor:    0,
			wrapWidth: 20,
			wantLine:  0,
			wantCol:   0,
		},
		{
			name:      "cursor at end of unwrapped line",
			input:     "hello",
			cursor:    5,
			wrapWidth: 20,
			wantLine:  0,
			wantCol:   5,
		},
		{
			name:      "cursor on second wrapped line",
			input:     "hello world",
			cursor:    6, // 'w' in "world"
			wrapWidth: 5,
			wantLine:  1,
			wantCol:   0,
		},
		{
			name:      "cursor mid second wrapped line",
			input:     "hello world",
			cursor:    8, // 'r' in "world"
			wrapWidth: 5,
			wantLine:  1,
			wantCol:   2,
		},
		{
			name:      "multiline input cursor on second logical line",
			input:     "first\nsecond line here",
			cursor:    6, // 's' in "second"
			wrapWidth: 20,
			wantLine:  1,
			wantCol:   0,
		},
		{
			name:      "multiline with wrap on second logical line",
			input:     "first\nhello world",
			cursor:    12, // 'w' in "world"
			wrapWidth: 5,
			wantLine:  2, // line 0: "first", line 1: "hello", line 2: "world"
			wantCol:   0,
		},
		{
			name:      "empty input",
			input:     "",
			cursor:    0,
			wrapWidth: 20,
			wantLine:  0,
			wantCol:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col := commentCursorVisualPos(tt.input, tt.cursor, tt.wrapWidth)
			assert.Equal(t, tt.wantLine, line, "visual line mismatch")
			assert.Equal(t, tt.wantCol, col, "visual col mismatch")
		})
	}
}

// =============================================================================
// Comment Tree Prefix Tests
// =============================================================================

// Test: buildCommentRows propagates treePath to all rows (top border, content, bottom border)
func TestComment_BuildCommentRows_PropagatesTreePath(t *testing.T) {
	tp := TreePath{
		Ancestors: []TreeLevel{
			{IsLast: false, Depth: 0},
			{IsLast: false, Depth: 1},
		},
	}

	rows := buildCommentRows(0, 1, &comments.Comment{Text: "hello"}, 40, tp)
	require.GreaterOrEqual(t, len(rows), 3, "should have top border, content, and bottom border")

	for i, r := range rows {
		assert.Equal(t, tp, r.treePath, "row %d should carry the treePath", i)
	}
}

// Test: buildCommentRows with empty treePath still works (no ancestors)
func TestComment_BuildCommentRows_EmptyTreePath(t *testing.T) {
	tp := TreePath{}

	rows := buildCommentRows(0, 1, &comments.Comment{Text: "hello"}, 40, tp)
	require.GreaterOrEqual(t, len(rows), 3)

	for i, r := range rows {
		assert.Equal(t, tp, r.treePath, "row %d should carry empty treePath", i)
	}
}

// Test: Comment rows in buildRows() carry the same treePath as adjacent content rows
func TestComment_BuildRows_CommentTreePathMatchesContent(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.comments = map[commentKey]*comments.Comment{
		{fileIndex: 0, newLineNum: 2}: {Text: "A comment"},
	}
	m.w().rowsCacheValid = false

	rows := m.buildRows()

	// Find the content row for line 2 and the comment rows that follow it
	for i, r := range rows {
		if r.kind == RowKindContent && r.pair.New.Num == 2 {
			contentTreePath := r.treePath
			// The next rows should be comment rows with the same treePath
			for j := i + 1; j < len(rows) && rows[j].kind == RowKindComment; j++ {
				assert.Equal(t, contentTreePath, rows[j].treePath,
					"comment row %d should have same treePath as parent content row", j)
			}
			return
		}
	}
	t.Fatal("did not find content row for line 2")
}

// Test: renderCommentRow includes tree continuation when ancestors are present
func TestComment_RenderCommentRow_IncludesTreePrefix(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.focused = true

	tp := TreePath{
		Ancestors: []TreeLevel{
			{IsLast: false, Depth: 0},
		},
	}

	// Build a comment content row with a treePath
	row := displayRow{
		kind:             RowKindComment,
		fileIndex:        0,
		commentText:      "test",
		commentLineNum:   1,
		commentRowIndex:  1,
		commentRowCount:  3,
		commentLineIndex: 0,
		treePath:         tp,
	}

	rendered := m.renderCommentRow(row, 60, 60, 4, false)

	// The tree prefix for a non-last ancestor should contain │
	assert.Contains(t, rendered, "│",
		"rendered comment row should include tree continuation character")
}

// Test: renderCommentRow without ancestors has no tree continuation
func TestComment_RenderCommentRow_NoTreePrefixWithoutAncestors(t *testing.T) {
	m := makeCommentableTestModel(5)
	m.focused = true

	row := displayRow{
		kind:             RowKindComment,
		fileIndex:        0,
		commentText:      "test",
		commentLineNum:   1,
		commentRowIndex:  3, // meta(1) + blank(2) + first text line(3)
		commentRowCount:  5, // border + meta + blank + text + border
		commentLineIndex: 2, // 0=meta, 1=blank, 2+=text
		treePath:         TreePath{},
	}

	rendered := m.renderCommentRow(row, 60, 60, 4, false)

	// Without ancestors, tree prefix is just margin spaces — should still render the comment
	assert.Contains(t, rendered, "test",
		"rendered comment row should contain comment text")
}

// Test: Comment rows in log mode (with commits) carry tree path with ancestors
func TestComment_LogMode_CommentRowsHaveTreePath(t *testing.T) {
	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
		},
		{
			Old: sidebyside.Line{Num: 2, Content: "old2", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 2, Content: "new2", Type: sidebyside.Added},
		},
	}

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
			Files: []sidebyside.FilePair{
				{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
			},
		},
	}

	m := NewWithCommits(commits)
	m.width = 120
	m.height = 40
	m.comments = map[commentKey]*comments.Comment{
		{fileIndex: 0, newLineNum: 1}: {Text: "Log mode comment"},
	}
	m.RefreshLayout()

	rows := m.buildRows()

	// Find comment rows and verify they have non-empty tree paths
	foundComment := false
	for _, r := range rows {
		if r.kind == RowKindComment {
			foundComment = true
			assert.Greater(t, len(r.treePath.Ancestors), 0,
				"comment row in log mode should have tree ancestors")
		}
	}
	assert.True(t, foundComment, "should find comment rows in log mode")
}

// Test: commentVisualLines wraps multiline input correctly
func TestCommentVisualLines(t *testing.T) {
	lines := commentVisualLines("hello world\nfoo bar baz", 5)
	assert.Equal(t, []string{"hello", "world", "foo", "bar", "baz"}, lines)
}

// Test: commentVisualLines handles empty input
func TestCommentVisualLinesEmpty(t *testing.T) {
	lines := commentVisualLines("", 20)
	assert.Equal(t, []string{""}, lines)
}

// =============================================================================
// Comment Navigation Tests
// =============================================================================

func TestGoToNextComment(t *testing.T) {
	m := makeCommentableTestModel(10)

	// Add comments on lines 3 and 7
	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{Text: "Comment A"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 7}] = &comments.Comment{Text: "Comment B"}
	m.w().rowsCacheValid = false
	m.calculateTotalLines()

	// Position at top
	m.w().scroll = 0

	// Navigate to first comment — should land on the content line
	m.goToNextComment(false)
	rows := m.getRows()
	cursor := m.cursorLine()
	assert.Equal(t, RowKindContent, rows[cursor].kind)
	assert.Equal(t, 3, rows[cursor].pair.New.Num)

	// Navigate to second comment
	m.goToNextComment(false)
	cursor = m.cursorLine()
	assert.Equal(t, RowKindContent, rows[cursor].kind)
	assert.Equal(t, 7, rows[cursor].pair.New.Num)

	// No more comments — should stay put
	prevCursor := m.cursorLine()
	m.goToNextComment(false)
	assert.Equal(t, prevCursor, m.cursorLine(), "should not wrap")
}

func TestGoToPrevComment(t *testing.T) {
	m := makeCommentableTestModel(10)

	// Add comments on lines 3 and 7
	m.comments[commentKey{fileIndex: 0, newLineNum: 3}] = &comments.Comment{Text: "Comment A"}
	m.comments[commentKey{fileIndex: 0, newLineNum: 7}] = &comments.Comment{Text: "Comment B"}
	m.w().rowsCacheValid = false
	m.calculateTotalLines()

	// Position at bottom
	m.w().scroll = m.maxScroll()

	// Navigate to last comment — should land on the content line
	m.goToPrevComment(false)
	rows := m.getRows()
	cursor := m.cursorLine()
	assert.Equal(t, RowKindContent, rows[cursor].kind)
	assert.Equal(t, 7, rows[cursor].pair.New.Num)

	// Navigate to first comment
	m.goToPrevComment(false)
	cursor = m.cursorLine()
	assert.Equal(t, RowKindContent, rows[cursor].kind)
	assert.Equal(t, 3, rows[cursor].pair.New.Num)

	// No more comments — should stay put
	prevCursor := m.cursorLine()
	m.goToPrevComment(false)
	assert.Equal(t, prevCursor, m.cursorLine(), "should not wrap")
}

func TestCommentNavigation_SpaceSequence(t *testing.T) {
	m := makeCommentableTestModel(10)

	// Add a comment on line 5
	m.comments[commentKey{fileIndex: 0, newLineNum: 5}] = &comments.Comment{Text: "Test comment"}
	m.w().rowsCacheValid = false
	m.calculateTotalLines()
	m.w().scroll = 0

	// Simulate pressing space, c, j
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = newM.(Model)
	assert.Equal(t, "space", m.pendingKey)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = newM.(Model)
	assert.Equal(t, "space c", m.pendingKey)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)
	assert.Equal(t, "", m.pendingKey)

	// Should have moved to the content line with the comment
	rows := m.getRows()
	cursor := m.cursorLine()
	assert.Equal(t, RowKindContent, rows[cursor].kind)
	assert.Equal(t, 5, rows[cursor].pair.New.Num)
}

func TestGoToNextComment_FoldedFile(t *testing.T) {
	m := makeCommentableTestModel(10)

	// Add a comment and fold the file to header-only
	m.comments[commentKey{fileIndex: 0, newLineNum: 5}] = &comments.Comment{Text: "Hidden comment"}
	m.setFileFoldLevel(0, sidebyside.FoldHeader)
	m.w().rowsCacheValid = false
	m.calculateTotalLines()
	m.w().scroll = 0

	// Navigate should unfold the file and land on the content line
	m.goToNextComment(false)
	rows := m.getRows()
	cursor := m.cursorLine()
	assert.Equal(t, RowKindContent, rows[cursor].kind)
	assert.Equal(t, 5, rows[cursor].pair.New.Num)
	// File should now be unfolded to hunks
	assert.Equal(t, sidebyside.FoldHunks, m.fileFoldLevel(0))
}

func TestGoToNextComment_NoComments(t *testing.T) {
	m := makeCommentableTestModel(10)
	m.calculateTotalLines()
	m.w().scroll = 0
	prevScroll := m.w().scroll

	// Should be a no-op when no comments exist
	m.goToNextComment(false)
	assert.Equal(t, prevScroll, m.w().scroll, "scroll should not change")

	m.goToPrevComment(false)
	assert.Equal(t, prevScroll, m.w().scroll, "scroll should not change")
}
