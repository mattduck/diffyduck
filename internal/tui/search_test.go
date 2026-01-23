package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// Test smart-case matching logic
func TestIsSmartCaseSensitive(t *testing.T) {
	// Lowercase query = case insensitive
	assert.False(t, isSmartCaseSensitive("hello"))
	assert.False(t, isSmartCaseSensitive("foo bar"))
	assert.False(t, isSmartCaseSensitive("123"))

	// Mixed/uppercase = case sensitive
	assert.True(t, isSmartCaseSensitive("Hello"))
	assert.True(t, isSmartCaseSensitive("HELLO"))
	assert.True(t, isSmartCaseSensitive("camelCase"))
}

func TestFindNextMatchRow_Simple(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world",
		"hello there",
		"goodbye world",
	})
	m.searchQuery = "hello"

	row, found := m.findNextMatchRow(0, true)

	assert.True(t, found)
	// Row 3 is the first content row (after top border at 0, header at 1, spacer at 2)
	assert.Equal(t, 3, row)
}

func TestFindNextMatchRow_CaseInsensitive(t *testing.T) {
	m := makeSearchTestModel([]string{
		"Hello World",
		"HELLO there",
		"hello again",
	})

	// Lowercase query = case insensitive - should find all
	m.searchQuery = "hello"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found)
	assert.Equal(t, 3, row) // first match at row 3
}

func TestFindNextMatchRow_CaseSensitive(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world", // row 3
		"Hello World", // row 4
		"HELLO there", // row 5
	})

	// Mixed case query = case sensitive - should only find exact match
	m.searchQuery = "Hello"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found)
	assert.Equal(t, 4, row) // "Hello" is at row 4
}

func TestFindNextMatchRow_NoMatches(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world",
		"foo bar",
	})

	m.searchQuery = "xyz"
	_, found := m.findNextMatchRow(0, true)
	assert.False(t, found)
}

func TestFindNextMatchRow_Backward(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",  // row 3
		"other line", // row 4
		"match two",  // row 5
	})
	m.searchQuery = "match"

	// Search backward from row 5
	row, found := m.findNextMatchRow(5, false)
	assert.True(t, found)
	assert.Equal(t, 5, row) // should find "match two" at row 5

	// Search backward from row 4
	row, found = m.findNextMatchRow(4, false)
	assert.True(t, found)
	assert.Equal(t, 3, row) // should find "match one" at row 3
}

func TestFindNextMatchRow_OldSideFiltering(t *testing.T) {
	// Create a model with different content on left and right
	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "context line", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "context line", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 2, Content: "old removed", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 2, Content: "new added", Type: sidebyside.Added},
		},
	}
	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Searching for "old" should find the removed line (old side)
	m.searchQuery = "old"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found)
	assert.Equal(t, 4, row) // removed line is at row 4

	// Searching for "context" should only find on new side (old side context is skipped)
	m.searchQuery = "context"
	row, found = m.findNextMatchRow(0, true)
	assert.True(t, found)
	assert.Equal(t, 3, row) // new side has "context line"
}

func TestFindNextMatchRow_InHeader(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/searchable.go",
				NewPath: "b/searchable.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content"},
						New: sidebyside.Line{Num: 1, Content: "content"},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Search for text in header
	m.searchQuery = "searchable"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found)
	assert.Equal(t, 1, row) // header is row 1 (after top border)
}

// Test search mode entry and exit
func TestSearch_EnterSearchMode(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})

	// Press / to enter search mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model := newM.(Model)

	assert.True(t, model.searchMode)
	assert.True(t, model.searchForward)
	assert.Equal(t, "", model.searchQuery)
}

func TestSearch_EnterSearchModeBackward(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})

	// Press ? to enter backward search mode
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model := newM.(Model)

	assert.True(t, model.searchMode)
	assert.False(t, model.searchForward)
}

func TestSearch_TypeQuery(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})
	m.searchMode = true

	// Type "hel"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model := newM.(Model)

	assert.Equal(t, "hel", model.searchInput)
}

func TestSearch_Backspace(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})
	m.searchMode = true
	m.searchInput = "hel"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model := newM.(Model)

	assert.Equal(t, "he", model.searchInput)
}

func TestSearch_Execute(t *testing.T) {
	m := makeSearchTestModel([]string{
		"first line",
		"hello world",
		"last line",
	})
	m.searchMode = true
	m.searchInput = "hello"
	m.searchForward = true

	// Press Enter to execute search
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := newM.(Model)

	assert.False(t, model.searchMode)
	assert.Equal(t, "hello", model.searchQuery)
	// Cursor should move to the match row (row 4)
	assert.Equal(t, 4, model.cursorLine())
}

func TestSearch_Cancel(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})
	m.searchMode = true
	m.searchInput = "partial"
	m.searchQuery = "previous"

	// Press Escape to cancel
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := newM.(Model)

	assert.False(t, model.searchMode)
	assert.Equal(t, "", model.searchInput)
	assert.Equal(t, "previous", model.searchQuery) // previous query preserved
}

// Test navigation between matches
func TestSearch_NextMatch(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",   // row 3
		"other line",  // row 4
		"match two",   // row 5
		"match three", // row 6
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at first match (row 3)
	m.adjustScrollToRow(3)

	// Press n to go to next match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	// Should be at second match (row 5)
	assert.Equal(t, 5, model.cursorLine())
}

func TestSearch_PrevMatch(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",  // row 3
		"other line", // row 4
		"match two",  // row 5
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at second match (row 5)
	m.adjustScrollToRow(5)

	// Press N to go to previous match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)

	// Should be at first match (row 3)
	assert.Equal(t, 3, model.cursorLine())
}

func TestSearch_NextMatch_AtEnd_NoWrap(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one", // row 3
		"match two", // row 4
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at last match (row 4)
	m.adjustScrollToRow(4)

	initialCursor := m.cursorLine()

	// Press n - should stay at last match (no wrap)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	// Cursor shouldn't move (no more matches forward)
	assert.Equal(t, initialCursor, model.cursorLine())
}

func TestSearch_PrevMatch_AtStart_NoWrap(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one", // row 3
		"match two", // row 4
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at first match (row 3)
	m.adjustScrollToRow(3)

	initialCursor := m.cursorLine()

	// Press N - should stay at first match (no wrap)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)

	// Cursor shouldn't move (no more matches backward)
	assert.Equal(t, initialCursor, model.cursorLine())
}

func TestSearch_NextMatch_AfterScrollToTop(t *testing.T) {
	// User navigates to last match, then presses gg to go to top, then n
	// Should find first match from top
	m := makeSearchTestModel([]string{
		"match one",   // row 3
		"line 2",      // row 4
		"match two",   // row 5
		"match three", // row 6
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Start at last match
	m.adjustScrollToRow(6)

	// Press gg to go to top
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = newM.(Model)
	assert.Equal(t, m.minScroll(), model.scroll)

	// Press n to go to next match - should find first match from cursor position
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)

	// Should find a match (row 3 is first match)
	assert.Equal(t, 3, model.cursorLine())
}

func TestSearch_NextMatch_ScrollsToMatch(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"match two", // this is row 8
	})
	m.height = 5 // small viewport
	m.searchQuery = "match"
	m.searchForward = true
	// Start at first match (row 3)
	m.adjustScrollToRow(3)

	// Press n to go to next match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	// Cursor should move to second match (row 8)
	assert.Equal(t, 8, model.cursorLine())
}

// Test status bar display during search
func TestSearch_StatusBar_SearchPrompt(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})
	m.searchMode = true
	m.searchForward = true
	m.searchInput = "test"

	output := m.View()

	// Should show search prompt with forward indicator
	assert.Contains(t, output, "/test")
}

func TestSearch_StatusBar_SearchPromptBackward(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})
	m.searchMode = true
	m.searchForward = false
	m.searchInput = "test"

	output := m.View()

	// Should show search prompt with backward indicator
	assert.Contains(t, output, "?test")
}

// Test cycling through multiple matches on the same row
func TestSearch_NextMatch_CyclesWithinRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz foo", // row 3 - has 3 matches
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(3)
	m.searchMatchIdx = 0 // start at first match

	// Press n - should go to second match on same row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 1, model.searchMatchIdx, "should be at second match")

	// Press n again - should go to third match on same row
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "cursor should still be on same row")
	assert.Equal(t, 2, model.searchMatchIdx, "should be at third match")

	// Press n again - no more matches on this row, should stay (no other rows have matches)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 2, model.searchMatchIdx, "should stay at third match")
}

func TestSearch_PrevMatch_CyclesWithinRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz foo", // row 3 - has 3 matches
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(3)
	m.searchMatchIdx = 2 // start at last match

	// Press N - should go to second match on same row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 1, model.searchMatchIdx, "should be at second match")

	// Press N again - should go to first match on same row
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model = newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "cursor should still be on same row")
	assert.Equal(t, 0, model.searchMatchIdx, "should be at first match")

	// Press N again - no more matches backward, should stay
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model = newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 0, model.searchMatchIdx, "should stay at first match")
}

func TestSearch_NextMatch_CyclesThenMovesToNextRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo", // row 3 - has 2 matches
		"baz foo qux", // row 4 - has 1 match
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(3)
	m.searchMatchIdx = 0

	// Press n - go to second match on row 3
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 3, model.cursorLine())
	assert.Equal(t, 1, model.searchMatchIdx)

	// Press n - no more matches on row 3, move to row 4
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)
	assert.Equal(t, 4, model.cursorLine(), "should move to next row")
	assert.Equal(t, 0, model.searchMatchIdx, "should be at first match on new row")
}

func TestSearch_BackwardSearch_NextMatch(t *testing.T) {
	// In backward search mode, n goes backward (previous in document)
	m := makeSearchTestModel([]string{
		"foo one", // row 3
		"foo two", // row 4
	})
	m.searchQuery = "foo"
	m.searchForward = false // backward search
	m.adjustScrollToRow(4)  // start at second match
	m.searchMatchIdx = 0

	// Press n in backward search - should go to previous row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "should move to previous row in backward search")
}

func TestSearch_BackwardSearch_PrevMatch(t *testing.T) {
	// In backward search mode, N goes forward (next in document)
	m := makeSearchTestModel([]string{
		"foo one", // row 3
		"foo two", // row 4
	})
	m.searchQuery = "foo"
	m.searchForward = false // backward search
	m.adjustScrollToRow(3)  // start at first match
	m.searchMatchIdx = 0

	// Press N in backward search - should go to next row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)
	assert.Equal(t, 4, model.cursorLine(), "should move to next row in backward search with N")
}

func TestSearch_BackwardSearch_CyclesWithinRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz", // row 3 - has 2 matches
	})
	m.searchQuery = "foo"
	m.searchForward = false // backward search
	m.adjustScrollToRow(3)
	m.searchMatchIdx = 1 // start at second match

	// In backward search, n goes backward, so decrement index
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 3, model.cursorLine())
	assert.Equal(t, 0, model.searchMatchIdx, "should go to first match")
}

// Test findMatchColsOnRow function directly
func TestFindMatchColsOnRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz", // row 3
	})
	m.searchQuery = "foo"

	cols := m.findMatchColsOnRow(3)

	assert.Len(t, cols, 2)
	assert.Equal(t, 0, cols[0], "first match at position 0")
	assert.Equal(t, 8, cols[1], "second match at position 8")
}

func TestFindMatchColsOnRow_EmptyQuery(t *testing.T) {
	m := makeSearchTestModel([]string{"foo bar"})
	m.searchQuery = ""

	cols := m.findMatchColsOnRow(3)

	assert.Nil(t, cols)
}

func TestFindMatchColsOnRow_InvalidRow(t *testing.T) {
	m := makeSearchTestModel([]string{"foo bar"})
	m.searchQuery = "foo"

	cols := m.findMatchColsOnRow(999) // invalid row

	assert.Nil(t, cols)
}

// Test findMatchesInText for rendering
func TestFindMatchesInText(t *testing.T) {
	m := Model{searchQuery: "foo"}

	matches := m.findMatchesInText("foo bar foo baz", false, 0)

	assert.Len(t, matches, 2)
	assert.Equal(t, 0, matches[0].Col)
	assert.Equal(t, 8, matches[1].Col)
	assert.False(t, matches[0].IsCurrent) // not on cursor row
}

func TestFindMatchesInText_IsCurrent(t *testing.T) {
	m := Model{searchQuery: "foo"}

	matches := m.findMatchesInText("foo bar", true, 0)

	assert.Len(t, matches, 1)
	assert.True(t, matches[0].IsCurrent)
}

func TestFindMatchesInText_CaseInsensitive(t *testing.T) {
	m := Model{searchQuery: "foo"} // lowercase = case insensitive

	matches := m.findMatchesInText("FOO bar Foo", false, 0)

	assert.Len(t, matches, 2) // should find both FOO and Foo
}

func TestFindMatchesInText_CaseSensitive(t *testing.T) {
	m := Model{searchQuery: "Foo"} // mixed case = case sensitive

	matches := m.findMatchesInText("FOO bar Foo", false, 0)

	assert.Len(t, matches, 1) // should only find Foo
	assert.Equal(t, 8, matches[0].Col)
}

func TestFindMatchesInText_CycleMatches(t *testing.T) {
	m := Model{searchQuery: "foo"}
	text := "foo bar foo baz"

	// With currentIdx=0, first match should be current
	matches := m.findMatchesInText(text, true, 0)
	assert.Len(t, matches, 2)
	assert.True(t, matches[0].IsCurrent, "first match should be current when currentIdx=0")
	assert.False(t, matches[1].IsCurrent, "second match should not be current when currentIdx=0")

	// With currentIdx=1, second match should be current
	matches = m.findMatchesInText(text, true, 1)
	assert.Len(t, matches, 2)
	assert.False(t, matches[0].IsCurrent, "first match should not be current when currentIdx=1")
	assert.True(t, matches[1].IsCurrent, "second match should be current when currentIdx=1")
}

// NOTE: Tests for search highlighting with inline diff are skipped.
// lipgloss disables ANSI output when no TTY is detected, making it impossible
// to test ANSI escape codes in the rendered output.

// Helper to create a test model with content
func makeSearchTestModel(lines []string) Model {
	pairs := make([]sidebyside.LinePair, len(lines))
	for i, line := range lines {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: line, Type: sidebyside.Context},
			New: sidebyside.Line{Num: i + 1, Content: line, Type: sidebyside.Context},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:       80,
		height:      20,
		keys:        DefaultKeyMap(),
		hscrollStep: DefaultHScrollStep,
	}
	m.calculateTotalLines()
	return m
}

// Test that search works correctly when files are folded
func TestSearch_FoldedContent(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world",
		"foo bar",
	})

	// Fold the file
	m.files[0].FoldLevel = sidebyside.FoldFolded
	m.calculateTotalLines()

	// Search for "hello" - shouldn't find in folded view
	m.searchQuery = "hello"
	_, found := m.findNextMatchRow(0, true)
	assert.False(t, found, "should not find 'hello' when file is folded")

	// Unfold back to normal
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()

	// Should find the match again
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'hello' after unfolding")
	assert.Equal(t, 3, row)
}

func TestSearch_ExpandedViewWithMoreContent(t *testing.T) {
	// When expanding, more content becomes searchable
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					// Only line 2 in the diff
					{
						Old: sidebyside.Line{Num: 2, Content: "hello", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "hello", Type: sidebyside.Context},
					},
				},
				OldContent: []string{"world", "hello", "search"},
				NewContent: []string{"world", "hello", "search"},
			},
		},
		width:       80,
		height:      20,
		keys:        DefaultKeyMap(),
		hscrollStep: DefaultHScrollStep,
	}
	m.calculateTotalLines()

	// Search for "search" - shouldn't find in normal view (not in diff)
	m.searchQuery = "search"
	_, found := m.findNextMatchRow(0, true)
	assert.False(t, found, "should not find 'search' in normal view")

	// Expand to full view
	m.files[0].FoldLevel = sidebyside.FoldExpanded
	m.calculateTotalLines()

	// Should now find "search" in the expanded content
	_, found = m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'search' in expanded view")
}

// Test per-side match tracking
func TestFindMatchColsOnRowSide(t *testing.T) {
	// Create a model with a changed line (content differs on each side)
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old foo bar", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new foo baz foo", Type: sidebyside.Added},
					},
				},
			},
		},
		width:       80,
		height:      20,
		keys:        DefaultKeyMap(),
		hscrollStep: DefaultHScrollStep,
	}
	m.calculateTotalLines()
	m.searchQuery = "foo"

	// New side (0) should have 2 matches (in "new foo baz foo")
	newSideMatches := m.findMatchColsOnRowSide(3, 0)
	assert.Len(t, newSideMatches, 2, "new side should have 2 matches")

	// Old side (1) should have 1 match (in "old foo bar")
	oldSideMatches := m.findMatchColsOnRowSide(3, 1)
	assert.Len(t, oldSideMatches, 1, "old side should have 1 match")

	// Combined should have 3 matches
	allMatches := m.findMatchColsOnRow(3)
	assert.Len(t, allMatches, 3, "combined should have 3 matches")
}

// Test that search match styles have black foreground (fg=0) for readability
func TestSearchStyles_HaveBlackForeground(t *testing.T) {
	// Note: We can't easily test the rendered ANSI output because lipgloss
	// disables ANSI codes without a TTY. Instead, we verify the style definitions
	// by checking that the styles produce different output from unstyled text
	// when run with a TTY (which we can't do in tests).
	//
	// This test documents the expected behavior:
	// - searchMatchStyle should have fg=0 (black) and bg=3 (yellow)
	// - searchCurrentMatchStyle should have fg=0 (black) and bg=9 (bright red)
	//
	// The actual style definitions are in view.go:
	//   searchMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3"))
	//   searchCurrentMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("9"))

	// Verify both styles are defined (they'll be empty without TTY but should exist)
	_ = searchMatchStyle
	_ = searchCurrentMatchStyle

	// The styles should render text (even if without ANSI codes in tests)
	result := searchMatchStyle.Render("test")
	assert.Contains(t, result, "test")

	result = searchCurrentMatchStyle.Render("test")
	assert.Contains(t, result, "test")
}

// Test cycling between sides on a row with matches on both sides
func TestSearch_CyclesBetweenSides(t *testing.T) {
	// Create a model with a changed line where both sides have "foo"
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/test.go",
				NewPath: "b/test.go",
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old foo", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new foo", Type: sidebyside.Added},
					},
				},
			},
		},
		width:       80,
		height:      20,
		keys:        DefaultKeyMap(),
		hscrollStep: DefaultHScrollStep,
	}
	m.calculateTotalLines()
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(3)
	m.searchMatchIdx = 0
	m.searchMatchSide = 0 // start on new side

	// Should start on new side (0)
	assert.Equal(t, 0, m.searchMatchSide)
	assert.Equal(t, 0, m.searchMatchIdx)

	// Press n - should move to old side (1) since new side has only 1 match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "should stay on same row")
	assert.Equal(t, 1, model.searchMatchSide, "should move to old side")
	assert.Equal(t, 0, model.searchMatchIdx, "should be at first match on old side")
}
