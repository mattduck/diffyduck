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

func TestFindMatches_Simple(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world",
		"hello there",
		"goodbye world",
	})

	matches := m.findMatches("hello")

	assert.Len(t, matches, 2)
	assert.Equal(t, 3, matches[0].Row) // row 3 (after top border at 0, header at 1, bottom border at 2)
	assert.Equal(t, 4, matches[1].Row)
}

func TestFindMatches_CaseInsensitive(t *testing.T) {
	m := makeSearchTestModel([]string{
		"Hello World",
		"HELLO there",
		"hello again",
	})

	// Lowercase query = case insensitive
	matches := m.findMatches("hello")

	assert.Len(t, matches, 3)
}

func TestFindMatches_CaseSensitive(t *testing.T) {
	m := makeSearchTestModel([]string{
		"Hello World",
		"HELLO there",
		"hello again",
	})

	// Mixed case query = case sensitive
	matches := m.findMatches("Hello")

	assert.Len(t, matches, 1)
	assert.Equal(t, 3, matches[0].Row) // row 3 (after top border + header + bottom border)
}

func TestFindMatches_NoMatches(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world",
		"foo bar",
	})

	matches := m.findMatches("xyz")

	assert.Empty(t, matches)
}

func TestFindMatches_MultiplePerLine(t *testing.T) {
	m := makeSearchTestModel([]string{
		"foo foo foo",
	})

	matches := m.findMatches("foo")

	// Should find all 3 occurrences
	assert.Len(t, matches, 3)
	assert.Equal(t, 0, matches[0].Col)
	assert.Equal(t, 4, matches[1].Col)
	assert.Equal(t, 8, matches[2].Col)
}

func TestFindMatches_BothSides(t *testing.T) {
	// Create a model with different content on left and right
	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "left match", Type: sidebyside.Removed},
			New: sidebyside.Line{Num: 1, Content: "right match", Type: sidebyside.Added},
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

	matches := m.findMatches("match")

	// Should find match on both left and right side
	assert.Len(t, matches, 2)
}

func TestFindMatches_InHeader(t *testing.T) {
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

	matches := m.findMatches("searchable")

	// Should find match in header
	assert.Len(t, matches, 1)
	assert.Equal(t, 1, matches[0].Row) // header is row 1 (after top border)
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
	assert.Len(t, model.matches, 1)
	assert.Equal(t, 0, model.currentMatch)
	// Match at row 2 is already visible with height=20, so no scroll needed
	assert.Equal(t, 0, model.scroll)
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
		"match one",
		"other line",
		"match two",
		"match three",
	})
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 0

	// Press n to go to next match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	assert.Equal(t, 1, model.currentMatch)
}

func TestSearch_PrevMatch(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",
		"other line",
		"match two",
	})
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 1

	// Press N to go to previous match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)

	assert.Equal(t, 0, model.currentMatch)
}

func TestSearch_NextMatch_AtEnd_NoWrap(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",
		"match two",
	})
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 1     // at last match
	m.lastSearchScroll = 0 // hasn't scrolled since last search nav

	// Press n - should stay at last match (no wrap)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	assert.Equal(t, 1, model.currentMatch)
}

func TestSearch_PrevMatch_AtStart_NoWrap(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",
		"match two",
	})
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 0     // at first match
	m.lastSearchScroll = 0 // hasn't scrolled since last search nav

	// Press N - should stay at first match (no wrap)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model := newM.(Model)

	assert.Equal(t, 0, model.currentMatch)
}

func TestSearch_NextMatch_AfterScrollToTop(t *testing.T) {
	// User navigates to last match, then presses gg to go to top, then n
	// Should find first match from top, not stay at last match
	m := makeSearchTestModel([]string{
		"match one",   // row 3 (after top border + header + bottom border)
		"line 2",      // row 4
		"match two",   // row 5
		"match three", // row 6
	})
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 2     // at last match (row 5)
	m.scroll = 5           // scrolled to last match
	m.lastSearchScroll = 5 // last search nav was at scroll 5

	// Press gg to go to top (now goes to minScroll)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := newM.(Model)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = newM.(Model)
	assert.Equal(t, m.minScroll(), model.scroll)

	// Press n to go to next match - should find first match from top
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = newM.(Model)

	// Should be at first match (row 3), not stuck at last match
	assert.Equal(t, 0, model.currentMatch)
	assert.Equal(t, 3, model.matches[model.currentMatch].Row)
}

func TestSearch_PrevMatch_AfterScrollToBottom(t *testing.T) {
	// User is at first match, presses G to go to bottom, then N
	// Should find last match from bottom, not stay at first match
	m := makeSearchTestModel([]string{
		"match one",   // row 3 (after top border + header + bottom border)
		"line 2",      // row 4
		"match two",   // row 5
		"match three", // row 6
	})
	m.height = 3 // small viewport so we can scroll
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 0 // at first match
	m.scroll = 0
	m.lastSearchScroll = 0 // last search nav was at scroll 0

	// Press G to go to bottom
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := newM.(Model)

	// Press N to go to previous match - should find last match visible from bottom
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	model = newM.(Model)

	// Should be at last match (row 6), not stuck at first match
	assert.Equal(t, 2, model.currentMatch)
	assert.Equal(t, 6, model.matches[model.currentMatch].Row)
}

func TestSearch_NextMatch_ScrollsToMatch(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"match two", // this is row 6
	})
	m.height = 5 // small viewport (4 content lines + 1 status bar)
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 0
	m.scroll = 0

	// Press n to go to next match
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := newM.(Model)

	// Should scroll so match is visible
	assert.Equal(t, 1, model.currentMatch)
	// Row 6 should be visible - scroll needs to be at least 3 (6 - contentHeight + 1)
	// With height=5, contentHeight=4, so scroll >= 6-4+1 = 3
	assert.GreaterOrEqual(t, model.scroll, 3)
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

func TestSearch_StatusBar_MatchCount(t *testing.T) {
	m := makeSearchTestModel([]string{
		"match one",
		"match two",
		"match three",
	})
	m.searchQuery = "match"
	m.searchForward = true
	m.matches = m.findMatches("match")
	m.currentMatch = 1

	output := m.View()

	// Should show match count (2/3 since currentMatch is 1-indexed for display)
	assert.Contains(t, output, "2/3")
}

func TestSearch_StatusBar_NoMatches(t *testing.T) {
	m := makeSearchTestModel([]string{"hello"})
	m.searchQuery = "xyz"
	m.searchForward = true
	m.matches = m.findMatches("xyz")

	output := m.View()

	// Should indicate no matches
	assert.Contains(t, output, "No matches")
}

// NOTE: Tests for search highlighting with inline diff are skipped.
// lipgloss disables ANSI output when no TTY is detected, making it impossible
// to test ANSI escape codes in the rendered output. See plans/next-steps.org
// for details on potential solutions.
//
// The implementation in applyInlineSpans does handle search highlighting with
// precedence over inline diff - it's just not testable with the current approach.

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

func TestRefreshSearch_UpdatesMatches(t *testing.T) {
	// Create a model with search active
	m := makeSearchTestModel([]string{
		"hello world",
		"foo bar",
	})

	// Execute a search
	m.searchInput = "hello"
	m.executeSearch()

	assert.Len(t, m.matches, 1, "should find 1 match initially")
	assert.Equal(t, "hello", m.searchQuery)

	// Change fold level - matches should update
	m.files[0].FoldLevel = sidebyside.FoldFolded
	m.calculateTotalLines()
	m.refreshSearch()

	// With folded view, there's no content to search (only header)
	// If "hello" isn't in the header, there should be no matches
	// The header is "test.go", so no "hello" matches
	assert.Len(t, m.matches, 0, "should have no matches after folding")

	// Unfold back to normal
	m.files[0].FoldLevel = sidebyside.FoldNormal
	m.calculateTotalLines()
	m.refreshSearch()

	// Should find the match again
	assert.Len(t, m.matches, 1, "should find match again after unfolding")
}

func TestRefreshSearch_NoQueryDoesNothing(t *testing.T) {
	m := makeSearchTestModel([]string{
		"hello world",
	})

	// No search query
	assert.Empty(t, m.searchQuery)

	// Call refresh - should not panic or change anything
	m.refreshSearch()

	assert.Empty(t, m.matches)
}

func TestRefreshSearch_ExpandedViewWithMoreContent(t *testing.T) {
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
	m.searchInput = "search"
	m.executeSearch()

	assert.Len(t, m.matches, 0, "should not find 'search' in normal view")

	// Expand to full view
	m.files[0].FoldLevel = sidebyside.FoldExpanded
	m.calculateTotalLines()
	m.refreshSearch()

	// Should now find "search" in the expanded content
	assert.Len(t, m.matches, 1, "should find 'search' in expanded view")
}
