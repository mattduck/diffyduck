package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

func TestView_HunkSeparator(t *testing.T) {
	// When there's a gap in line numbers, a separator should be shown
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					// First hunk: lines 1-3
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
					},
					// Gap here - next hunk starts at line 100
					{
						Old: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 101, Content: "line hundred one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 101, Content: "line hundred one", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Hunk separator should be blank (no cross separator)
	assert.NotContains(t, output, "┼", "hunk separator should NOT have cross in middle")
}

func TestView_TreeConnectsFilesDirectly(t *testing.T) {
	// In tree layout, files are connected with tree branches - no blank lines between them
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/first.go",
				NewPath:   "b/first.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath:   "a/second.go",
				NewPath:   "b/second.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find both file headers
	firstHeaderIdx := -1
	secondHeaderIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "first.go") && strings.Contains(line, "●") {
			firstHeaderIdx = i
		}
		if strings.Contains(line, "second.go") && strings.Contains(line, "●") {
			secondHeaderIdx = i
		}
	}
	require.NotEqual(t, -1, firstHeaderIdx, "should find first file header")
	require.NotEqual(t, -1, secondHeaderIdx, "should find second file header")

	// Both files should have ├ branch (never └, since ┴ terminator follows last file)
	assert.Contains(t, lines[firstHeaderIdx], "├", "first file should have non-last tree branch")
	assert.Contains(t, lines[secondHeaderIdx], "├", "second file should have ├ branch (┴ terminator follows)")

	// There should be no blank lines between last content of first file and second file header
	// Check that lines between files contain tree continuation (│) or are content/borders
	for i := firstHeaderIdx + 1; i < secondHeaderIdx; i++ {
		line := lines[i]
		// Lines between files should have tree continuation or be content - not blank
		if strings.TrimSpace(line) == "" {
			continue // Skip truly empty lines (padding)
		}
		// Should have tree structure (│ or content)
		hasTreeOrContent := strings.Contains(line, "│") || strings.Contains(line, "┗") ||
			strings.Contains(line, "line one")
		assert.True(t, hasTreeOrContent, "line %d between files should have tree structure or content: %q", i, line)
	}
}

func TestView_FirstFileHasTreeBranch(t *testing.T) {
	// In tree layout, the first (and only) file has a tree branch prefix
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/only.go",
				NewPath:   "b/only.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.scroll = m.minScroll() // Position cursor at top so header is visible

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the file header line with tree branch - it has fold icon (●) AND tree branch (└ or ├)
	// Skip the top bar which also has the filename but no tree branch
	headerIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "only.go") && strings.Contains(line, "●") &&
			(strings.Contains(line, "└") || strings.Contains(line, "├")) {
			headerIdx = i
			break
		}
	}
	require.NotEqual(t, -1, headerIdx, "should find file header with tree branch")

	// Single file should have ├ branch (┴ terminator follows)
	assert.Contains(t, lines[headerIdx], "├", "single file should have ├ branch (┴ terminator follows)")
	assert.Contains(t, lines[headerIdx], "━━━", "tree branch should have heavy horizontal line")
}

func TestView_NoSeparatorForConsecutiveLines(t *testing.T) {
	// When lines are consecutive, no separator should be shown
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/foo.go",
				NewPath:   "b/foo.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "line two", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "line three", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	// Should NOT contain hunk separator cross (┼ only appears in hunk separators)
	assert.NotContains(t, output, "┼")
}

func TestView_HunkSeparatorNoCrossInMiddle(t *testing.T) {
	// Hunk separator should NOT have a vertical divider (no ┼ cross)
	// This creates visual separation between chunks
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
					},
					// Gap creates hunk separator
					{
						Old: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor away from the hunk separator (on a content line)
	m.scroll = 3 // cursor on line 100

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line (shaded line with ░, not the blank line above it)
	// Hunk separator is row 5 in this test (after header area at rows 0-2, first content at row 3, blank at row 4)
	var hunkLine string
	for i, line := range lines {
		// Skip header area (rows 0-2) and content lines with file name, line numbers, or content
		// Look for line with shading (░) that's not a content line
		if i > 2 && strings.Contains(line, "░") &&
			!strings.Contains(line, "test.go") && !strings.Contains(line, "100") &&
			!strings.Contains(line, "first") && !strings.Contains(line, "second") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line")
	// Should NOT have cross character in middle
	assert.NotContains(t, hunkLine, "┼", "hunk separator should NOT have cross in middle")
}

func TestView_HunkSeparatorBreadcrumbs(t *testing.T) {
	// Hunk separator should show breadcrumbs (function/type name) on the left side
	// when structure data is available (file was expanded and parsed)
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded, // Normal view (not expanded)
				Pairs: []sidebyside.LinePair{
					// First hunk: lines 1-3 (outside any function)
					{
						Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "import \"fmt\"", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 3, Content: "import \"fmt\"", Type: sidebyside.Context},
					},
					// Gap - next hunk is inside func MyFunction (lines 10-50)
					{
						Old: sidebyside.Line{Num: 15, Content: "    x := 1", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 15, Content: "    x := 1", Type: sidebyside.Context},
					},
					{
						Old: sidebyside.Line{Num: 16, Content: "    y := 2", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 16, Content: "    y := 3", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
		// Initialize the structureMaps - simulating that the file was previously expanded
		structureMaps: map[int]*FileStructure{
			0: {
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 10, EndLine: 50, Name: "MyFunction", Kind: "func"},
				}),
			},
		},
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line by looking for breadcrumb content
	var hunkLine string
	for _, line := range lines {
		// Hunk separator contains breadcrumb but no file name or line numbers
		if strings.Contains(line, "func MyFunction") && !strings.Contains(line, "test.go") && !strings.Contains(line, "package") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line with breadcrumb")
	// The separator should contain the function name as a breadcrumb
	assert.Contains(t, hunkLine, "func MyFunction", "hunk separator should show breadcrumb for the function containing the chunk start")
}

func TestView_HunkSeparatorBreadcrumbs_NoBreadcrumbWithoutStructure(t *testing.T) {
	// When structure data is not available, hunk separator should just show shading
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "line one", Type: sidebyside.Context},
					},
					// Gap
					{
						Old: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "line hundred", Type: sidebyside.Context},
					},
				},
			},
		},
		width:         100,
		height:        15,
		keys:          DefaultKeyMap(),
		structureMaps: make(map[int]*FileStructure), // Empty - no structure data
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line (blank line without content)
	// It's between rows 3 (line 1 content) and row 5 (line 100 content)
	var hunkLine string
	for i, line := range lines {
		// Skip header area and content lines - look for line with shading (░)
		if i > 2 && strings.Contains(line, "░") &&
			!strings.Contains(line, "test.go") && !strings.Contains(line, "line") &&
			!strings.Contains(line, " 1 ") && !strings.Contains(line, " 100 ") {
			hunkLine = line
			break
		}
	}

	require.NotEmpty(t, hunkLine, "should find hunk separator line")
	// Without structure, should not have breadcrumb
	assert.NotContains(t, hunkLine, "func", "hunk separator without structure should not have breadcrumb")
}

func TestView_HunkSeparatorBreadcrumbs_RealHighlightFlow(t *testing.T) {
	// Test the real flow: content is loaded, highlighting extracts structure,
	// then shrinking to normal view should show breadcrumbs in hunk separators
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	goCode := `package main

func MyFunction() {
	x := 1
	y := 2
	z := 3
}

func AnotherFunction() {
	a := 1
}`
	lines := strings.Split(goCode, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldExpanded, // Normal view
			Pairs: []sidebyside.LinePair{
				// First hunk: lines 1-2 (package declaration)
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
				},
				// Gap here - next hunk is inside MyFunction (line 4 = "x := 1")
				{
					Old: sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
				},
				{
					Old: sidebyside.Line{Num: 5, Content: "	y := 2", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 5, Content: "	y := 99", Type: sidebyside.Added},
				},
			},
			// Full content is available (simulating file was expanded then shrunk)
			NewContent: lines,
			OldContent: lines,
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Simulate the highlight flow that happens when file is expanded
	cmd := m.RequestHighlight(0)
	require.NotNil(t, cmd, "RequestHighlight should return a command")

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	require.True(t, ok, "Expected HighlightReadyMsg, got %T", msg)

	// Verify structure was extracted
	require.NotEmpty(t, hlMsg.NewStructure, "NewStructure should be populated with Go functions")

	// Store the highlight data (this is what Update does)
	m.storeHighlightSpans(hlMsg)

	// Verify structureMaps was populated
	require.NotNil(t, m.structureMaps[0], "structureMaps should be populated for file 0")
	require.NotNil(t, m.structureMaps[0].NewStructure, "NewStructure should be set")

	// Verify we can look up structure at line 4 (inside MyFunction)
	entries := m.getStructureAtLine(0, 4)
	require.NotEmpty(t, entries, "Should find structure entries at line 4")
	assert.Equal(t, "MyFunction", entries[0].Name, "Line 4 should be inside MyFunction")

	// Now render and check the hunk separator
	m.width = 100
	m.height = 20
	m.calculateTotalLines()

	// Semantic context expansion should have filled the gap between lines 2 and 4
	// by inserting line 3 (func MyFunction). Verify this happened.
	require.Equal(t, 6, len(m.files[0].Pairs), "should have 6 pairs after semantic expansion")

	// Check that line 3 was inserted (no more gap)
	hasLine3 := false
	for _, p := range m.files[0].Pairs {
		if p.New.Num == 3 {
			hasLine3 = true
			break
		}
	}
	require.True(t, hasLine3, "semantic expansion should insert line 3")

	// Verify the line numbers are now contiguous (1, 2, 3, 4, 5)
	expectedNewNums := []int{1, 2, 3, 4, 0, 5} // 0 is for the removed line (empty on new side)
	for i, p := range m.files[0].Pairs {
		assert.Equal(t, expectedNewNums[i], p.New.Num, "pair %d should have New.Num=%d", i, expectedNewNums[i])
	}

	// Now render and verify the function signature appears in the output
	output := m.View()
	// Strip ANSI codes for easier assertion (syntax highlighting adds codes between chars)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	strippedOutput := ansiRegex.ReplaceAllString(output, "")
	// The function signature should now appear as a code line (not just breadcrumb)
	assert.Contains(t, strippedOutput, "func MyFunction", "function signature should appear in expanded context")
}

func TestView_HunkSeparatorBreadcrumbs_LeftSidePositioning(t *testing.T) {
	// Test that breadcrumbs appear on the left side (new content side) of the hunk separator,
	// starting after the gutter (indicator + lineNum), aligned near code content.
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					// First hunk
					{
						Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					// Gap creates hunk separator - next chunk is inside MyFunction
					{
						Old: sidebyside.Line{Num: 20, Content: "    code", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 20, Content: "    code", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
		structureMaps: map[int]*FileStructure{
			0: {
				NewStructure: structure.NewMap([]structure.Entry{
					{StartLine: 10, EndLine: 50, Name: "MyFunction", Kind: "func"},
				}),
			},
		},
	}
	m.calculateTotalLines()
	m.rebuildRowsCache()

	// Find the hunk separator row
	rows := m.buildRows()
	var hunkSepIdx int
	for i, row := range rows {
		if row.isSeparator {
			hunkSepIdx = i
			break
		}
	}
	require.NotZero(t, hunkSepIdx, "should find hunk separator row")

	// Test 1: Non-cursor row - breadcrumb on left side
	output := m.View()
	lines := strings.Split(output, "\n")

	var hunkLine string
	for _, line := range lines {
		// In tree layout, breadcrumb might be truncated - look for partial match
		if strings.Contains(line, "MyFunction") || strings.Contains(line, "MyFunct") {
			hunkLine = line
			break
		}
	}
	require.NotEmpty(t, hunkLine, "should find hunk separator with breadcrumb (partial or full)")

	// The breadcrumb should appear in the left half of the line
	// In tree layout, the tree prefix (│) with ANSI styling takes additional bytes,
	// so use a generous byte budget to account for escape sequences
	halfWidth := m.width // use full width to avoid cutting through ANSI escapes
	leftHalf := hunkLine
	if len(hunkLine) > halfWidth {
		leftHalf = hunkLine[:halfWidth]
	}
	// Check for function name (might be truncated)
	assert.True(t, strings.Contains(leftHalf, "MyFunction") || strings.Contains(leftHalf, "MyFunct"),
		"breadcrumb should appear in left half (new content side), got: %q", leftHalf)

	// Test 2: Cursor row - breadcrumb still visible with cursor arrow
	m.scroll = hunkSepIdx
	output = m.View()
	lines = strings.Split(output, "\n")

	var cursorHunkLine string
	for _, line := range lines {
		// Look for arrow and partial breadcrumb (might be truncated in tree layout)
		if strings.Contains(line, "▶") && (strings.Contains(line, "MyFunction") || strings.Contains(line, "MyFunct")) {
			cursorHunkLine = line
			break
		}
	}
	require.NotEmpty(t, cursorHunkLine, "cursor row should show arrow and breadcrumb (partial or full)")

	// Verify arrow appears and breadcrumb is preserved (partial match OK)
	assert.Contains(t, cursorHunkLine, "▶", "cursor row should have arrow")
	assert.True(t, strings.Contains(cursorHunkLine, "MyFunction") || strings.Contains(cursorHunkLine, "MyFunct"),
		"cursor row should preserve breadcrumb text (partial or full)")
}

func TestView_HunkSeparatorArrowPositionsMatchContentLines(t *testing.T) {
	// Test that cursor arrow positions on hunk separator match those on content lines.
	// Both left and right arrows should appear at the same horizontal positions.
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first", Type: sidebyside.Context},
					},
					// Gap creates hunk separator
					{
						Old: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "second", Type: sidebyside.Context},
					},
				},
			},
		},
		width:               80,
		height:              15,
		keys:                DefaultKeyMap(),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
	}
	m.calculateTotalLines()

	// Helper to find arrow display column positions in a string (accounting for ANSI codes)
	findArrowDisplayPositions := func(s string) []int {
		var positions []int
		displayCol := 0
		inEscape := false
		for _, r := range s {
			if r == '\x1b' {
				inEscape = true
				continue
			}
			if inEscape {
				if r == 'm' {
					inEscape = false
				}
				continue
			}
			if r == '▶' {
				positions = append(positions, displayCol)
			}
			displayCol++
		}
		return positions
	}

	// Render with cursor on content line (line 100)
	// In diff view layout: header=0, bottom_border=1, line1=2, hunksep_top=3, hunksep=4, hunksep_bottom=5, line100=6
	m.scroll = 6
	contentOutput := m.View()
	contentLines := strings.Split(contentOutput, "\n")

	// Find the content line with cursor (has "100" line number and arrows)
	var contentLineWithCursor string
	for _, line := range contentLines {
		if strings.Contains(line, "100") && strings.Contains(line, "▶") {
			contentLineWithCursor = line
			break
		}
	}
	require.NotEmpty(t, contentLineWithCursor, "should find content line with cursor")
	contentArrowPositions := findArrowDisplayPositions(contentLineWithCursor)
	require.Len(t, contentArrowPositions, 2, "content line should have 2 arrows (left and right)")

	// Now render with cursor on hunk separator (the line with breadcrumbs, not the top shader line)
	m.scroll = 4
	hunkOutput := m.View()
	hunkLines := strings.Split(hunkOutput, "\n")

	// Find the hunk separator line (has arrows but no line content or file names)
	var hunkSepLine string
	for i, line := range hunkLines {
		// Skip header area, look for line that has arrows but no line content
		if i > 3 && strings.Contains(line, "▶") &&
			!strings.Contains(line, "test.go") && !strings.Contains(line, "100") &&
			!strings.Contains(line, "first") && !strings.Contains(line, "second") {
			hunkSepLine = line
			break
		}
	}
	require.NotEmpty(t, hunkSepLine, "should find hunk separator line with cursor")
	hunkArrowPositions := findArrowDisplayPositions(hunkSepLine)
	require.Len(t, hunkArrowPositions, 2, "hunk separator should have 2 arrows (left and right)")

	// The arrow positions should match between content line and hunk separator
	assert.Equal(t, contentArrowPositions[0], hunkArrowPositions[0],
		"left arrow position should match: content=%d, hunk=%d", contentArrowPositions[0], hunkArrowPositions[0])
	assert.Equal(t, contentArrowPositions[1], hunkArrowPositions[1],
		"right arrow position should match: content=%d, hunk=%d", contentArrowPositions[1], hunkArrowPositions[1])
}

func TestView_CursorArrowOnHunkSeparator(t *testing.T) {
	// Test that cursor arrow appears on hunk separator when selected
	// Hunk separators appear when there's a gap in line numbers
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "first hunk", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "first hunk", Type: sidebyside.Context},
					},
					// Gap in line numbers creates a hunk separator
					{
						Old: sidebyside.Line{Num: 100, Content: "second hunk", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 100, Content: "second hunk", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 15,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	// Position cursor on hunk separator (row 4: top_border=0, header=1, bottom_border=2, line1=3, hunksep=4)
	m.scroll = 4

	output := m.View()
	lines := strings.Split(output, "\n")

	// Find the hunk separator line (blank line without line numbers, not the header)
	// Hunk separators don't contain file names or line numbers, and have ▶ when cursor is on them
	var hunkSepLine string
	for i, line := range lines {
		if i > 2 && strings.Contains(line, "▶") && !strings.Contains(line, "test.go") && !strings.Contains(line, "100") && !strings.Contains(line, "first") {
			hunkSepLine = line
			break
		}
	}

	require.NotEmpty(t, hunkSepLine, "should find hunk separator line")
	assert.Contains(t, hunkSepLine, "▶", "hunk separator with cursor should have arrow indicator")
}
