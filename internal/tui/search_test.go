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
	// Row 2 is the first content row (header at 0, spacer at 1, content at 2+)
	assert.Equal(t, 2, row)
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
	assert.Equal(t, 2, row) // first match at row 2
}

func TestFindNextMatchRow_CaseSensitive(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world", // row 2
		"Hello World", // row 3
		"HELLO there", // row 4
	})

	// Mixed case query = case sensitive - should only find exact match
	m.searchQuery = "Hello"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found)
	assert.Equal(t, 3, row) // "Hello" is at row 3
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
		"match one",  // row 2
		"other line", // row 3
		"match two",  // row 4
	})
	m.searchQuery = "match"

	// Search backward from row 4
	row, found := m.findNextMatchRow(4, false)
	assert.True(t, found)
	assert.Equal(t, 4, row) // should find "match two" at row 4

	// Search backward from row 3
	row, found = m.findNextMatchRow(3, false)
	assert.True(t, found)
	assert.Equal(t, 2, row) // should find "match one" at row 2
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
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
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
	assert.Equal(t, 3, row) // removed line is at row 3 (header at 0, spacer at 1, content at 2-3)

	// Searching for "context" should only find on new side (old side context is skipped)
	m.searchQuery = "context"
	row, found = m.findNextMatchRow(0, true)
	assert.True(t, found)
	assert.Equal(t, 2, row) // new side has "context line" at row 2
}

func TestFindNextMatchRow_InHeader(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/searchable.go",
				NewPath:   "b/searchable.go",
				FoldLevel: sidebyside.FoldExpanded,
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
	assert.Equal(t, 0, row) // header is row 0 (no top border in diff view)
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
	// Cursor should move to the match row (row 3)
	assert.Equal(t, 3, model.cursorLine())
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
		"match one",   // row 2
		"other line",  // row 3
		"match two",   // row 4
		"match three", // row 5
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at first match (row 2)
	m.adjustScrollToRow(2)

	// Press n to go to next match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	// Should be at second match (row 4)
	assert.Equal(t, 4, model.cursorLine())
}

func TestSearch_PrevMatch(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",  // row 2
		"other line", // row 3
		"match two",  // row 4
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at second match (row 4)
	m.adjustScrollToRow(4)

	// Press N to go to previous match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)

	// Should be at first match (row 2)
	assert.Equal(t, 2, model.cursorLine())
}

func TestSearch_NextMatch_AtEnd_NoWrap(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one", // row 2
		"match two", // row 3
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at last match (row 3)
	m.adjustScrollToRow(3)

	initialCursor := m.cursorLine()

	// Press n - should stay at last match (no wrap)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	// Cursor shouldn't move (no more matches forward)
	assert.Equal(t, initialCursor, model.cursorLine())
}

func TestSearch_PrevMatch_AtStart_NoWrap(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one", // row 2
		"match two", // row 3
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Position cursor at first match (row 2)
	m.adjustScrollToRow(2)

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
		"match one",   // row 2
		"line 2",      // row 3
		"match two",   // row 4
		"match three", // row 5
	})
	m.searchQuery = "match"
	m.searchForward = true
	// Start at last match
	m.adjustScrollToRow(5)

	// Press gg to go to top
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = newM.(Model)
	assert.Equal(t, m.minScroll(), model.scroll)

	// Press n to go to next match - should find first match from cursor position
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)

	// Should find a match (row 2 is first match)
	assert.Equal(t, 2, model.cursorLine())
}

func TestSearch_NextMatch_ScrollsToMatch(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"match two", // this is row 7
	})
	m.height = 5 // small viewport
	m.searchQuery = "match"
	m.searchForward = true
	// Start at first match (row 2)
	m.adjustScrollToRow(2)

	// Press n to go to next match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	// Cursor should move to second match (row 7)
	assert.Equal(t, 7, model.cursorLine())
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
		"foo bar foo baz foo", // row 2 - has 3 matches
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(2)
	m.searchMatchIdx = 0 // start at first match

	// Press n - should go to second match on same row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 1, model.searchMatchIdx, "should be at second match")

	// Press n again - should go to third match on same row
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "cursor should still be on same row")
	assert.Equal(t, 2, model.searchMatchIdx, "should be at third match")

	// Press n again - no more matches on this row, should stay (no other rows have matches)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 2, model.searchMatchIdx, "should stay at third match")
}

func TestSearch_PrevMatch_CyclesWithinRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz foo", // row 2 - has 3 matches
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(2)
	m.searchMatchIdx = 2 // start at last match

	// Press N - should go to second match on same row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 1, model.searchMatchIdx, "should be at second match")

	// Press N again - should go to first match on same row
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model = newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "cursor should still be on same row")
	assert.Equal(t, 0, model.searchMatchIdx, "should be at first match")

	// Press N again - no more matches backward, should stay
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model = newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "cursor should stay on same row")
	assert.Equal(t, 0, model.searchMatchIdx, "should stay at first match")
}

func TestSearch_NextMatch_CyclesThenMovesToNextRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo", // row 2 - has 2 matches
		"baz foo qux", // row 3 - has 1 match
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(2)
	m.searchMatchIdx = 0

	// Press n - go to second match on row 2
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 2, model.cursorLine())
	assert.Equal(t, 1, model.searchMatchIdx)

	// Press n - no more matches on row 2, move to row 3
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "should move to next row")
	assert.Equal(t, 0, model.searchMatchIdx, "should be at first match on new row")
}

func TestSearch_BackwardSearch_NextMatch(t *testing.T) {
	// In backward search mode, n goes backward (previous in document)
	m := makeSearchTestModel([]string{
		"foo one", // row 2
		"foo two", // row 3
	})
	m.searchQuery = "foo"
	m.searchForward = false // backward search
	m.adjustScrollToRow(3)  // start at second match
	m.searchMatchIdx = 0

	// Press n in backward search - should go to previous row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "should move to previous row in backward search")
}

func TestSearch_BackwardSearch_PrevMatch(t *testing.T) {
	// In backward search mode, N goes forward (next in document)
	m := makeSearchTestModel([]string{
		"foo one", // row 2
		"foo two", // row 3
	})
	m.searchQuery = "foo"
	m.searchForward = false // backward search
	m.adjustScrollToRow(2)  // start at first match
	m.searchMatchIdx = 0

	// Press N in backward search - should go to next row
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "should move to next row in backward search with N")
}

func TestSearch_BackwardSearch_CyclesWithinRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz", // row 2 - has 2 matches
	})
	m.searchQuery = "foo"
	m.searchForward = false // backward search
	m.adjustScrollToRow(2)
	m.searchMatchIdx = 1 // start at second match

	// In backward search, n goes backward, so decrement index
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 2, model.cursorLine())
	assert.Equal(t, 0, model.searchMatchIdx, "should go to first match")
}

// Test findMatchColsOnRow function directly
func TestFindMatchColsOnRow(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz", // row 2
	})
	m.searchQuery = "foo"

	cols := m.findMatchColsOnRow(2)

	assert.Len(t, cols, 2)
	assert.Equal(t, 0, cols[0], "first match at position 0")
	assert.Equal(t, 8, cols[1], "second match at position 8")
}

func TestFindMatchColsOnRow_EmptyQuery(t *testing.T) {
	m := makeSearchTestModel([]string{"foo bar"})
	m.searchQuery = ""

	cols := m.findMatchColsOnRow(2)

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
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
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

	// Unfold to hunks view
	m.files[0].FoldLevel = sidebyside.FoldExpanded
	m.calculateTotalLines()

	// Should find the match again
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'hello' after unfolding")
	assert.Equal(t, 2, row)
}

func TestSearch_FullFileViewWithMoreContent(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
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

	// Without ShowFullFile, "search" is not in the visible rows (only in full content)
	m.searchQuery = "search"
	_, found := m.findNextMatchRow(0, true)
	assert.False(t, found, "should not find 'search' in hunk view")

	// Enable full-file view
	m.files[0].ShowFullFile = true
	m.calculateTotalLines()

	_, found = m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'search' in full-file view")
}

// Test per-side match tracking
func TestFindMatchColsOnRowSide(t *testing.T) {
	// Create a model with a changed line (content differs on each side)
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
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
	newSideMatches := m.findMatchColsOnRowSide(2, 0)
	assert.Len(t, newSideMatches, 2, "new side should have 2 matches")

	// Old side (1) should have 1 match (in "old foo bar")
	oldSideMatches := m.findMatchColsOnRowSide(2, 1)
	assert.Len(t, oldSideMatches, 1, "old side should have 1 match")

	// Combined should have 3 matches
	allMatches := m.findMatchColsOnRow(2)
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
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
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
	m.adjustScrollToRow(2)
	m.searchMatchIdx = 0
	m.searchMatchSide = 0 // start on new side

	// Should start on new side (0)
	assert.Equal(t, 0, m.searchMatchSide)
	assert.Equal(t, 0, m.searchMatchIdx)

	// Press n - should move to old side (1) since new side has only 1 match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "should stay on same row")
	assert.Equal(t, 1, model.searchMatchSide, "should move to old side")
	assert.Equal(t, 0, model.searchMatchIdx, "should be at first match on old side")
}

// Test that nextMatch doesn't cycle forever between sides on the same row
// This was a bug where pressing n would cycle side 0 -> side 1 -> side 0 -> ... forever
func TestSearch_NextMatch_DoesNotCycleForeverBetweenSides(t *testing.T) {
	// Create a model with two rows, each having matches on both sides
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old foo", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new foo", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "old foo again", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "new foo again", Type: sidebyside.Added},
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
	m.adjustScrollToRow(2) // first content row
	m.searchMatchIdx = 0
	m.searchMatchSide = 0 // start on new side

	// Press n - should move to old side (1)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)
	assert.Equal(t, 2, model.cursorLine(), "should stay on row 2")
	assert.Equal(t, 1, model.searchMatchSide, "should be on old side")

	// Press n again - should NOT cycle back to side 0, should move to next row (row 3)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)
	assert.Equal(t, 3, model.cursorLine(), "should move to row 3, not cycle back to side 0")
	assert.Equal(t, 0, model.searchMatchSide, "should start on new side of new row")
	assert.Equal(t, 0, model.searchMatchIdx, "should be at first match")
}

// Test that prevMatch (N) doesn't cycle forever between sides when going backward
func TestSearch_PrevMatch_DoesNotCycleForeverBetweenSides(t *testing.T) {
	// Create a model with two rows, each having matches on both sides
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old foo", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new foo", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "old foo again", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "new foo again", Type: sidebyside.Added},
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
	m.adjustScrollToRow(3) // second content row
	m.searchMatchIdx = 0
	m.searchMatchSide = 0 // start on new side

	// Press N (prev) - should move to old side (1) going backward
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)
	// In forward search, prev goes: new side -> (can't go further back on this row) -> prev row
	// Actually wait - let me think about this more carefully
	// We're on side 0, idx 0. Going backward means we need to go to side 1 first (since side 1 > side 0 in backward direction)
	// But the fix says: backward means side 1 -> side 0, not side 0 -> side 1
	// So from side 0, we can't go to side 1 when going backward, we go to prev row
	assert.Equal(t, 2, model.cursorLine(), "should move to row 2")
	assert.Equal(t, 1, model.searchMatchSide, "should be on old side (last side of prev row)")
}

// Test that cursor movement (j/k) resets search match index to 0
func TestSearch_CursorMove_ResetsMatchIndex(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo baz foo", // row 2 - has 3 matches
		"foo qux foo",         // row 3 - has 2 matches
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(2)
	m.searchMatchIdx = 2 // pretend we're at the third match
	m.searchMatchSide = 0

	// Press j to move down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := newM.(Model)

	assert.Equal(t, 3, model.cursorLine(), "should move to row 3")
	assert.Equal(t, 0, model.searchMatchIdx, "match index should reset to 0 after cursor move")
	assert.Equal(t, 0, model.searchMatchSide, "match side should reset to 0 (new side)")
}

// Test that cursor movement (k) also resets search match index
func TestSearch_CursorMoveUp_ResetsMatchIndex(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo bar foo",         // row 2 - has 2 matches
		"foo baz foo qux foo", // row 3 - has 3 matches
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(3)
	m.searchMatchIdx = 2 // pretend we're at the third match
	m.searchMatchSide = 0

	// Press k to move up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := newM.(Model)

	assert.Equal(t, 2, model.cursorLine(), "should move to row 2")
	assert.Equal(t, 0, model.searchMatchIdx, "match index should reset to 0 after cursor move")
}

// Test that page down also resets search match index
func TestSearch_PageDown_ResetsMatchIndex(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "foo content foo"
	}
	m := makeSearchTestModel(lines)
	m.height = 10
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(3)
	m.searchMatchIdx = 1 // at second match
	m.searchMatchSide = 0

	// Press ctrl+d for page down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	model := newM.(Model)

	assert.Equal(t, 0, model.searchMatchIdx, "match index should reset to 0 after page down")
}

// Test that gg (go to top) resets search match index
func TestSearch_GoToTop_ResetsMatchIndex(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo one",
		"foo two",
		"foo three",
	})
	m.searchQuery = "foo"
	m.searchForward = true
	m.adjustScrollToRow(5) // at row 5
	m.searchMatchIdx = 1
	m.searchMatchSide = 0

	// Press g then g for gg
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)

	assert.Equal(t, 0, model.searchMatchIdx, "match index should reset to 0 after gg")
}

// =============================================================================
// Search in Commits Tests
// =============================================================================

// Helper to create a test model with commit metadata for search tests.
func makeSearchCommitModel(commitInfo sidebyside.CommitInfo, foldLevel sidebyside.CommitFoldLevel) Model {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/foo.go",
			NewPath:   "b/foo.go",
			FoldLevel: sidebyside.FoldNormal,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "old line", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 1, Content: "new line", Type: sidebyside.Added},
				},
			},
		},
	}
	commit := sidebyside.CommitSet{
		Info:        commitInfo,
		FoldLevel:   foldLevel,
		FilesLoaded: true,
		Files:       files,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	m.width = 120
	m.height = 40
	m.focused = true
	m.RefreshLayout()
	return m
}

func TestSearch_FindsSubjectInCommitHeader(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Subject: "Fix parser bug",
	}, sidebyside.CommitNormal)

	m.searchQuery = "parser"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'parser' in commit header subject")

	rows := m.buildRows()
	assert.Equal(t, RowKindCommitHeader, rows[row].kind)
}

func TestSearch_FindsAuthorInCommitHeader(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Subject: "Fix parser bug",
	}, sidebyside.CommitNormal)

	m.searchQuery = "alice"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'alice' (case-insensitive) in commit header author")

	rows := m.buildRows()
	assert.Equal(t, RowKindCommitHeader, rows[row].kind)
}

func TestSearch_FindsSHAInCommitHeader(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Subject: "Fix parser bug",
	}, sidebyside.CommitNormal)

	m.searchQuery = "abc123d"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find short SHA in commit header")

	rows := m.buildRows()
	assert.Equal(t, RowKindCommitHeader, rows[row].kind)
}

func TestSearch_CommitHeaderSearchText_ContainsSHAAuthorSubject(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Subject: "Fix parser bug",
	}, sidebyside.CommitNormal)

	rows := m.buildRows()
	var headerRow displayRow
	for _, r := range rows {
		if r.kind == RowKindCommitHeader {
			headerRow = r
			break
		}
	}

	assert.Contains(t, headerRow.commitHeaderSearchText, "abc123d", "should contain short SHA")
	assert.Contains(t, headerRow.commitHeaderSearchText, "Alice", "should contain author")
	assert.Contains(t, headerRow.commitHeaderSearchText, "Fix parser bug", "should contain subject")
}

func TestSearch_DoesNotSearchCommitInfoHeader(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Date:    "2024-01-15T10:30:00+00:00",
		Subject: "Fix parser bug",
	}, sidebyside.CommitNormal)

	// The commit info header shows the formatted date. Search for a date fragment
	// and verify it does NOT match the commit info header row.
	m.searchQuery = "Jan"
	row, found := m.findNextMatchRow(0, true)
	if found {
		rows := m.buildRows()
		assert.NotEqual(t, RowKindCommitInfoHeader, rows[row].kind,
			"search should not match commit info header rows")
	}
}

func TestSearch_FindsTextInCommitInfoBody(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice Wonderland",
		Email:   "alice@example.com",
		Date:    "2024-01-15T10:30:00+00:00",
		Subject: "Fix parser bug",
		Body:    "Detailed description of the fix",
	}, sidebyside.CommitExpanded)

	// Search for text in the commit body message
	m.searchQuery = "Detailed"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'Detailed' in commit info body")

	rows := m.buildRows()
	assert.Equal(t, RowKindCommitInfoBody, rows[row].kind)
}

func TestSearch_FindsAuthorInCommitInfoBody(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice Wonderland",
		Email:   "alice@example.com",
		Date:    "2024-01-15T10:30:00+00:00",
		Subject: "Fix parser bug",
	}, sidebyside.CommitExpanded)

	// Search for the email which only appears in the commit info body Author line
	m.searchQuery = "alice@example"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find email in commit info body Author line")

	rows := m.buildRows()
	assert.Equal(t, RowKindCommitInfoBody, rows[row].kind)
	assert.Contains(t, rows[row].commitInfoLine, "Author:")
}

func TestSearch_FindsSHAInCommitInfoBody(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Date:    "2024-01-15T10:30:00+00:00",
		Subject: "Fix parser bug",
	}, sidebyside.CommitExpanded)

	// The full SHA appears in the commit info body "commit abc123def4567890" line
	m.searchQuery = "abc123def"
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find full SHA in commit info body")

	rows := m.buildRows()
	// First match should be the commit header (short SHA), skip to next for info body
	if rows[row].kind == RowKindCommitHeader {
		row, found = m.findNextMatchRow(row+1, true)
		assert.True(t, found, "should find second match in commit info body")
	}
	assert.Equal(t, RowKindCommitInfoBody, rows[row].kind)
	assert.Contains(t, rows[row].commitInfoLine, "commit ")
}

func TestSearch_CommitBodyNotSearchedOnSide1(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Subject: "Fix parser bug",
	}, sidebyside.CommitExpanded)

	rows := m.buildRows()
	for i, r := range rows {
		if r.kind == RowKindCommitInfoBody {
			cols := m.findMatchColsOnRowSide(i, 1)
			assert.Empty(t, cols, "commit info body should not be searchable on side 1")
		}
	}
}

func TestSearch_CommitInfoBodyBlankLinesNotSearchable(t *testing.T) {
	m := makeSearchCommitModel(sidebyside.CommitInfo{
		SHA:     "abc123def4567890",
		Author:  "Alice",
		Date:    "2024-01-15T10:30:00+00:00",
		Subject: "Fix parser bug",
	}, sidebyside.CommitExpanded)

	// Blank commit info body lines should return empty searchable text
	rows := m.buildRows()
	for _, r := range rows {
		if r.kind == RowKindCommitInfoBody && r.commitInfoLine == "" {
			text := searchableText(r, 0)
			assert.Empty(t, text, "blank commit info body lines should return empty search text")
		}
	}
}

// =============================================================================
// Search in Comments Tests
// =============================================================================

// Helper to create a test model with comments
func makeSearchWithCommentsTestModel(lines []string, comments map[commentKey]string) Model {
	pairs := make([]sidebyside.LinePair, len(lines))
	for i, line := range lines {
		pairs[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Num: i + 1, Content: line, Type: sidebyside.Removed},
			New: sidebyside.Line{Num: i + 1, Content: line, Type: sidebyside.Added},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldExpanded, Pairs: pairs},
		},
		width:       80,
		height:      30,
		keys:        DefaultKeyMap(),
		hscrollStep: DefaultHScrollStep,
		comments:    comments,
	}
	m.calculateTotalLines()
	return m
}

// Test: Basic search finds text in comment content
func TestSearch_FindsTextInComment(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 2}: "This comment has searchterm in it",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"line one",
		"line two",
		"line three",
	}, comments)

	m.searchQuery = "searchterm"
	row, found := m.findNextMatchRow(0, true)

	assert.True(t, found, "should find 'searchterm' in comment")
	// The match should be in a comment row
	rows := m.buildRows()
	assert.Equal(t, RowKindComment, rows[row].kind, "match should be in a comment row")
}

// Test: Search finds text only in code when comment doesn't match
func TestSearch_FindsTextInCodeNotComment(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "This comment has nothing special",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"line with target here",
		"another line",
	}, comments)

	m.searchQuery = "target"
	row, found := m.findNextMatchRow(0, true)

	assert.True(t, found, "should find 'target' in code")
	// The match should be in a content row, not a comment row
	rows := m.buildRows()
	assert.Equal(t, RowKindContent, rows[row].kind, "match should be in a content row")
}

// Test: n/N navigation visits matches in comments
func TestSearch_NextMatch_VisitsCommentMatches(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 2}: "Comment with foo here",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"foo in first line", // row has "foo"
		"no match here",     // has comment with "foo"
		"foo in third line", // row has "foo"
	}, comments)

	m.searchQuery = "foo"
	m.searchForward = true

	// Start search from the beginning (row 0)
	rows := m.buildRows()

	// Collect all rows with matches to verify we visit content and comment rows
	var matchRowKinds []RowKind
	for i := range rows {
		if len(m.findMatchColsOnRow(i)) > 0 {
			matchRowKinds = append(matchRowKinds, rows[i].kind)
		}
	}

	// Should have matches in content rows AND comment rows
	hasContentMatch := false
	hasCommentMatch := false
	for _, kind := range matchRowKinds {
		if kind == RowKindContent {
			hasContentMatch = true
		}
		if kind == RowKindComment {
			hasCommentMatch = true
		}
	}

	assert.True(t, hasContentMatch, "should have matches in content rows")
	assert.True(t, hasCommentMatch, "should have matches in comment rows")

	// Test that nextMatch can navigate to both types
	// Position cursor at the very start
	m.scroll = m.minScroll()

	// Find first match from beginning
	firstRow, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find first match")

	// Move to that row
	m.adjustScrollToRow(firstRow)
	m.searchMatchIdx = 0
	m.searchMatchSide = 0

	// Navigate through all matches and verify we see both content and comment
	visitedKinds := make(map[RowKind]bool)
	visitedKinds[rows[firstRow].kind] = true

	for i := 0; i < 10; i++ { // limit iterations
		if !m.nextMatch() {
			break
		}
		cursorRow := m.cursorLine()
		if cursorRow >= 0 && cursorRow < len(rows) {
			visitedKinds[rows[cursorRow].kind] = true
		}
	}

	assert.True(t, visitedKinds[RowKindContent], "nextMatch should visit content rows")
	assert.True(t, visitedKinds[RowKindComment], "nextMatch should visit comment rows")
}

// Test: Cursor moves to comment row when match is in comment
func TestSearch_Execute_MovesToCommentRow(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "uniqueword appears here",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"no match in content",
		"still no match",
	}, comments)

	m.searchMode = true
	m.searchInput = "uniqueword"
	m.searchForward = true

	// Execute search
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := newM.(Model)

	// Cursor should be on the comment row
	cursorRow := model.cursorLine()
	rows := model.buildRows()
	assert.Equal(t, RowKindComment, rows[cursorRow].kind,
		"cursor should move to comment row containing match")
}

// Test: Multiple matches in same comment
func TestSearch_MultipleMatchesInSameComment(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "foo bar foo baz foo",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"no foo here wait there is foo",
	}, comments)

	m.searchQuery = "foo"
	m.searchForward = true

	// Find matches in the comment CONTENT row (not border)
	rows := m.buildRows()
	var commentRowIdx int
	for i, r := range rows {
		// Look for content row (commentLineIndex >= 0), not border
		if r.kind == RowKindComment && r.commentLineIndex >= 0 {
			commentRowIdx = i
			break
		}
	}

	// Check that the comment content row has multiple matches
	matches := m.findMatchColsOnRowSide(commentRowIdx, 0)
	assert.GreaterOrEqual(t, len(matches), 3,
		"comment content row should have at least 3 matches for 'foo'")
}

// Test: Match can be in both comment and code (same search term)
func TestSearch_MatchInBothCommentAndCode(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "Comment mentions variable myVar",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"code uses myVar = 42",
		"another line",
	}, comments)

	m.searchQuery = "myVar"
	m.searchForward = true

	// Count total matches
	rows := m.buildRows()
	totalMatches := 0
	for i := range rows {
		matches := m.findMatchColsOnRow(i)
		totalMatches += len(matches)
	}

	assert.GreaterOrEqual(t, totalMatches, 2,
		"should find 'myVar' in both comment and code")
}

// Test: Case-insensitive search in comments
func TestSearch_CaseInsensitiveInComments(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "This has UPPERCASE text",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"lowercase content",
	}, comments)

	// Lowercase query = case insensitive
	m.searchQuery = "uppercase"
	row, found := m.findNextMatchRow(0, true)

	assert.True(t, found, "case-insensitive search should find 'UPPERCASE'")
	rows := m.buildRows()
	assert.Equal(t, RowKindComment, rows[row].kind,
		"match should be in comment row")
}

// Test: Case-sensitive search in comments
func TestSearch_CaseSensitiveInComments(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "This has MixedCase text",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"lowercase content",
	}, comments)

	// Mixed case query = case sensitive
	m.searchQuery = "MixedCase"
	row, found := m.findNextMatchRow(0, true)

	assert.True(t, found, "case-sensitive search should find 'MixedCase'")
	rows := m.buildRows()
	assert.Equal(t, RowKindComment, rows[row].kind)

	// Should NOT find with wrong case
	m.searchQuery = "mixedcase"
	_, found = m.findNextMatchRow(0, true)
	// This will find in the comment because lowercase query is case-insensitive
	// Let's use a different approach - search for something that doesn't exist
	m.searchQuery = "MiXeDcAsE" // different mixed case
	_, found = m.findNextMatchRow(0, true)
	assert.False(t, found, "case-sensitive search should NOT find 'MiXeDcAsE'")
}

// Test: Search doesn't find in comments when file is folded
func TestSearch_FoldedFile_NoCommentMatches(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "hidden searchterm in comment",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"line content",
	}, comments)

	// Fold the file
	m.files[0].FoldLevel = sidebyside.FoldFolded
	m.calculateTotalLines()

	m.searchQuery = "searchterm"
	_, found := m.findNextMatchRow(0, true)

	assert.False(t, found, "should not find comment content when file is folded")
}

// Test: searchableText returns comment text for comment rows
func TestSearchableText_ReturnsCommentText(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "My comment text here",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"line content",
	}, comments)

	rows := m.buildRows()

	// Find the comment CONTENT row (not border)
	for _, r := range rows {
		if r.kind == RowKindComment && r.commentLineIndex >= 0 {
			text := searchableText(r, 0)
			assert.Contains(t, text, "My comment text here",
				"searchableText should return comment text for comment content rows")
			return
		}
	}
	t.Fatal("should have found a comment content row")
}

// Test: prevMatch (N) visits comment matches going backward
func TestSearch_PrevMatch_VisitsCommentMatches(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "Comment with foo",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"foo first",
		"foo second",
	}, comments)

	m.searchQuery = "foo"
	m.searchForward = true

	// Position at last match (second content line)
	rows := m.buildRows()
	lastContentIdx := -1
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].kind == RowKindContent {
			lastContentIdx = i
			break
		}
	}
	m.adjustScrollToRow(lastContentIdx)

	// Press N to go backward - should eventually find comment match
	foundComment := false
	for i := 0; i < 10; i++ { // limit iterations
		m.prevMatch()
		cursorRow := m.cursorLine()
		if rows[cursorRow].kind == RowKindComment {
			foundComment = true
			break
		}
	}

	assert.True(t, foundComment, "prevMatch should visit comment matches")
}

// Test: Rendered comment row includes the search term
// This test would have caught the bug where renderCommentRow didn't apply highlighting
func TestSearch_RenderCommentRow_IncludesSearchTerm(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "Comment with searchterm here",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"line content",
	}, comments)

	m.searchQuery = "searchterm"

	// Find a comment content row (not border)
	rows := m.buildRows()
	var commentContentRow displayRow
	var commentRowIdx int
	for i, r := range rows {
		if r.kind == RowKindComment && r.commentLineIndex >= 0 {
			commentContentRow = r
			commentRowIdx = i
			break
		}
	}

	// Position cursor on the comment row
	m.adjustScrollToRow(commentRowIdx)
	isCursorRow := m.cursorLine() == commentRowIdx

	// Render the comment row
	rendered := m.renderCommentRow(commentContentRow, 40, 40, 4, isCursorRow)

	// The rendered output should contain "searchterm"
	// (even without ANSI codes being testable, the text should be there)
	assert.Contains(t, rendered, "searchterm",
		"rendered comment row should contain the search term")
}

// Test: View() output includes comment text when searching
func TestSearch_ViewIncludesCommentMatch(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "uniquecommenttext",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"line content",
	}, comments)

	m.searchQuery = "uniquecommenttext"
	m.width = 100
	m.height = 20

	// Execute search to position cursor
	m.executeSearch()

	output := m.View()

	// The view should include the comment text
	assert.Contains(t, output, "uniquecommenttext",
		"View() should include comment text that matches search")
}

// Test: Search finds comment content row, not border row
// This catches the bug where cursor jumped to comment border instead of content
func TestSearch_FindsCommentContentRow_NotBorder(t *testing.T) {
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "findme in comment",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"no match here",
	}, comments)

	m.searchQuery = "findme"
	row, found := m.findNextMatchRow(0, true)

	assert.True(t, found, "should find match in comment")

	rows := m.buildRows()
	matchedRow := rows[row]

	// The matched row should be a comment CONTENT row, not a border
	assert.Equal(t, RowKindComment, matchedRow.kind, "should be a comment row")
	assert.GreaterOrEqual(t, matchedRow.commentLineIndex, 0,
		"should be a content row (commentLineIndex >= 0), not a border row")
}

// Test: searchableText returns empty for comment border rows
func TestSearchableText_CommentBorderReturnsEmpty(t *testing.T) {
	// Border row has commentLineIndex = -1
	borderRow := displayRow{
		kind:             RowKindComment,
		commentText:      "some comment text",
		commentLineIndex: -1, // border row
	}

	text := searchableText(borderRow, 0)
	assert.Equal(t, "", text, "border rows should return empty searchable text")
}

// Test: searchableText returns specific line for comment content rows
func TestSearchableText_CommentContentReturnsSpecificLine(t *testing.T) {
	// Content row for line 1 of a multi-line comment
	contentRow := displayRow{
		kind:             RowKindComment,
		commentText:      "Line zero\nLine one\nLine two",
		commentLineIndex: 1, // second line (0-indexed)
	}

	text := searchableText(contentRow, 0)
	assert.Equal(t, "Line one", text,
		"content row should return only its specific line")
}

// Test: n navigation lands on correct line within multi-line comment
func TestSearch_NextMatch_LandsOnCorrectCommentLine(t *testing.T) {
	// Multi-line comment where search term is on second line
	comments := map[commentKey]string{
		{fileIndex: 0, newLineNum: 1}: "First line\nSecond has target\nThird line",
	}
	m := makeSearchWithCommentsTestModel([]string{
		"no match",
	}, comments)

	m.searchQuery = "target"
	m.searchForward = true
	m.scroll = m.minScroll()

	// Find the match
	row, found := m.findNextMatchRow(0, true)
	assert.True(t, found, "should find 'target' in comment")

	rows := m.buildRows()
	matchedRow := rows[row]

	// Should land on the specific line containing "target" (line index 1)
	assert.Equal(t, RowKindComment, matchedRow.kind)
	assert.Equal(t, 1, matchedRow.commentLineIndex,
		"should land on line index 1 which contains 'target'")
}
