package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// Test the fix for: "scroll position can't go below a few lines"
// The bug was that structural diff rows weren't counted in totalLines until
// a fold/unfold triggered a recalculation. Now storeHighlightSpans calls
// calculateTotalLines when structural diffs are computed.
func TestStartup_TotalLinesUpdatedWhenStructuralDiffsLoad(t *testing.T) {
	oldContent := `package main

func FuncA() {
	x := 1
}
`
	newContent := `package main

func FuncA() {
	x := 1
	y := 2
}

func FuncB() {
	z := 3
}
`
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldStructure, // Start at structural diff view
			OldContent: oldLines,
			NewContent: newLines,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 3, Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 4, Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 5, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 5, Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Type: sidebyside.Context}},
			},
		},
	}

	m := New(files)
	m.width = 100
	m.height = 40
	m.initialFoldSet = true // prevent WindowSizeMsg from changing fold levels
	defer m.highlighter.Close()

	// Initial state: just 1 header row (no structural diff yet)
	m.calculateTotalLines()
	totalLinesBefore := m.w().totalLines

	// Simulate highlighting which computes structural diff
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	require.True(t, ok, "expected HighlightReadyMsg")

	m.storeHighlightSpans(hlMsg)

	// totalLines should now include structural diff rows
	assert.Greater(t, m.w().totalLines, totalLinesBefore,
		"totalLines should increase when structural diff is computed")

	// Verify structural diff rows are in the cache
	rows := m.getRows()
	structDiffCount := 0
	for _, r := range rows {
		if r.isStructuralDiff {
			structDiffCount++
		}
	}
	assert.Greater(t, structDiffCount, 0, "Should have structural diff rows")

	// maxScroll should allow reaching all rows
	assert.Equal(t, m.w().totalLines-1, m.maxScroll(), "maxScroll should be totalLines-1")
}
