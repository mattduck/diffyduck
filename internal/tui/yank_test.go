package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]string)
	return m
}

// Test: findCommentForCursor returns false when no comment exists
func TestYank_FindCommentForCursor_NoComment(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Move to a content line
	m.scroll = 3 // Skip header rows

	_, found := m.findCommentForCursor()
	assert.False(t, found, "should not find comment when none exists")
}

// Test: findCommentForCursor returns true when cursor is on line with comment
func TestYank_FindCommentForCursor_OnCommentedLine(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Add a comment on line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = "test comment"
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
	m.scroll = lineRowIdx

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
	m.comments[key] = "test comment"
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
	m.scroll = commentRowIdx

	ck, found := m.findCommentForCursor()
	assert.True(t, found, "should find comment when cursor is on comment row")
	assert.Equal(t, 3, ck.newLineNum, "should return correct line number")
}

// Test: buildDiffSnippet produces valid unified diff format
func TestYank_BuildDiffSnippet_Format(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 3}
	comment := "This is my comment"

	snippet := m.buildDiffSnippet(key, comment)

	// Should have file headers
	assert.Contains(t, snippet, "--- a/test.go", "should have old file header")
	assert.Contains(t, snippet, "+++ b/test.go", "should have new file header")

	// Should have hunk header
	assert.Contains(t, snippet, "@@ -", "should have hunk header")

	// Should have the comment with # prefix
	assert.Contains(t, snippet, "#\n# This is my comment", "should have comment with # prefix")

	// Should have diff lines
	assert.Contains(t, snippet, "-old line 3", "should have removed line")
	assert.Contains(t, snippet, "+new line 3", "should have added line")
}

// Test: buildDiffSnippet includes context lines before commented line
func TestYank_BuildDiffSnippet_Context(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 3}
	comment := "Comment on line 3"

	snippet := m.buildDiffSnippet(key, comment)

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
	comment := "Line one\nLine two\nLine three"

	snippet := m.buildDiffSnippet(key, comment)

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
	m.comments[key] = "test comment"
	m.rebuildRowsCache()

	// Find and position on the commented line (scroll = rowIdx - cursorOffset)
	rows := m.getRows()
	for i, row := range rows {
		if row.kind == RowKindContent && row.pair.New.Num == 3 {
			m.scroll = i
			break
		}
	}

	// Call handleYank - may fail if pbcopy not available, but status should be set either way
	newModel, _ := m.handleYank()
	m2 := newModel.(Model)

	// Status message should be set (either success or error)
	require.NotEmpty(t, m2.statusMessage, "status message should be set after yank")

	// If pbcopy succeeded, check for success message
	if strings.HasPrefix(m2.statusMessage, "Copied") {
		assert.Contains(t, m2.statusMessage, "test.go", "success message should contain filename")
	}
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
// ClearStatusMsg Tests
// =============================================================================

// Test: ClearStatusMsg clears message when timestamps match
func TestYank_ClearStatusMsg_ClearsWhenTimestampsMatch(t *testing.T) {
	m := makeYankTestModel()
	now := time.Now()

	m.statusMessage = "Test message"
	m.statusMessageTime = now

	// Send ClearStatusMsg with matching timestamp
	newModel, _ := m.Update(ClearStatusMsg{SetTime: now})
	m2 := newModel.(Model)

	assert.Empty(t, m2.statusMessage, "message should be cleared when timestamps match")
}

// Test: ClearStatusMsg does NOT clear message when timestamps differ
func TestYank_ClearStatusMsg_KeepsWhenTimestampsDiffer(t *testing.T) {
	m := makeYankTestModel()
	oldTime := time.Now()
	newTime := oldTime.Add(time.Second)

	m.statusMessage = "Newer message"
	m.statusMessageTime = newTime

	// Send ClearStatusMsg with OLD timestamp (simulating delayed clear)
	newModel, _ := m.Update(ClearStatusMsg{SetTime: oldTime})
	m2 := newModel.(Model)

	assert.Equal(t, "Newer message", m2.statusMessage, "message should NOT be cleared when timestamps differ")
}

// Test: ClearStatusMsg handles empty message gracefully
func TestYank_ClearStatusMsg_HandlesEmptyMessage(t *testing.T) {
	m := makeYankTestModel()
	now := time.Now()

	m.statusMessage = ""
	m.statusMessageTime = now

	// Should not panic
	newModel, _ := m.Update(ClearStatusMsg{SetTime: now})
	m2 := newModel.(Model)

	assert.Empty(t, m2.statusMessage)
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
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]string)
	m.calculateTotalLines()

	key := commentKey{fileIndex: 0, newLineNum: 1}
	comment := "Comment on first line"

	snippet := m.buildDiffSnippet(key, comment)

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
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
	})
	m.width = 80
	m.height = 30
	m.comments = make(map[commentKey]string)
	m.calculateTotalLines()

	// Comment on context line 3
	key := commentKey{fileIndex: 0, newLineNum: 3}
	comment := "Note about this line"

	snippet := m.buildDiffSnippet(key, comment)

	// Should show context lines with space prefix
	assert.Contains(t, snippet, " line 1")
	assert.Contains(t, snippet, " line 2")
	assert.Contains(t, snippet, " line 3")
	assert.Contains(t, snippet, "# Note about this line")
}

// Test: clearStatusAfter returns a tea.Cmd
func TestYank_ClearStatusAfter_ReturnsCmd(t *testing.T) {
	m := makeYankTestModel()
	now := time.Now()

	cmd := m.clearStatusAfter(now)

	assert.NotNil(t, cmd, "clearStatusAfter should return a non-nil command")

	// The command is a tea.Tick which we can't easily inspect,
	// but we can verify it's callable
	// (actual behavior is tested via ClearStatusMsg tests)
}

// Test: handleYank returns nil cmd when no comment at cursor
func TestYank_HandleYank_NoComment_ReturnsNil(t *testing.T) {
	m := makeYankTestModel()
	m.calculateTotalLines()

	// Don't add any comments, just position cursor somewhere
	m.scroll = 3

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
	m.comments[key] = "integration test comment"
	m.rebuildRowsCache()

	// Position cursor on the commented line
	rows := m.getRows()
	for i, row := range rows {
		if row.kind == RowKindContent && row.pair.New.Num == 3 {
			m.scroll = i
			break
		}
	}

	// Simulate pressing 'y'
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m2 := newModel.(Model)

	// Should have set a status message
	assert.NotEmpty(t, m2.statusMessage, "pressing y should set status message")

	// Should return a command (the clear timer)
	assert.NotNil(t, cmd, "should return clear timer command")
}
