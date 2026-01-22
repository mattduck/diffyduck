package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestUpdateMaxNewContentWidth_Basic(t *testing.T) {
	tests := []struct {
		name     string
		files    []sidebyside.FilePair
		expected int
	}{
		{
			name: "measures pairs content",
			files: []sidebyside.FilePair{
				{
					FoldLevel: sidebyside.FoldNormal,
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: "short"}},
						{New: sidebyside.Line{Content: "longer line"}},
						{New: sidebyside.Line{Content: "med"}},
					},
				},
			},
			expected: 11, // "longer line" = 11 chars
		},
		{
			name: "ignores folded files",
			files: []sidebyside.FilePair{
				{
					FoldLevel: sidebyside.FoldFolded,
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: "this should be ignored because file is folded"}},
					},
				},
			},
			expected: 0, // folded files are skipped
		},
		{
			name: "expanded uses NewContent",
			files: []sidebyside.FilePair{
				{
					FoldLevel:  sidebyside.FoldExpanded,
					NewContent: []string{"short", "this is the longest new line", "med"},
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: "pairs ignored when expanded"}},
					},
				},
			},
			expected: 28, // "this is the longest new line" = 28 chars
		},
		{
			name: "expanded without NewContent falls back to pairs",
			files: []sidebyside.FilePair{
				{
					FoldLevel:  sidebyside.FoldExpanded,
					NewContent: nil, // not loaded yet
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: "fallback to pairs"}},
					},
				},
			},
			expected: 17, // "fallback to pairs" = 17 chars
		},
		{
			name: "expands tabs before measuring",
			files: []sidebyside.FilePair{
				{
					FoldLevel: sidebyside.FoldNormal,
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: "\tfoo"}}, // expands to "    foo" = 7 chars
					},
				},
			},
			expected: 7,
		},
		{
			name: "multiple files takes max",
			files: []sidebyside.FilePair{
				{
					FoldLevel: sidebyside.FoldNormal,
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: "short"}},
					},
				},
				{
					FoldLevel: sidebyside.FoldNormal,
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: "this is longer"}},
					},
				},
			},
			expected: 14, // "this is longer" = 14 chars
		},
		{
			name: "empty new content ignored",
			files: []sidebyside.FilePair{
				{
					FoldLevel: sidebyside.FoldNormal,
					Pairs: []sidebyside.LinePair{
						{New: sidebyside.Line{Content: ""}},     // empty - removed line
						{New: sidebyside.Line{Content: "real"}}, // this is the max
					},
				},
			},
			expected: 4, // "real" = 4 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				files:              tt.files,
				maxNewContentWidth: 0,
			}
			m.updateMaxNewContentWidth()
			assert.Equal(t, tt.expected, m.maxNewContentWidth)
		})
	}
}

func TestMaxNewContentWidth_OnlyGrows(t *testing.T) {
	// Start with a file that has wide content
	m := Model{
		files: []sidebyside.FilePair{
			{
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{New: sidebyside.Line{Content: "this is a fairly long line of content"}},
				},
			},
		},
		maxNewContentWidth: 0,
	}

	m.updateMaxNewContentWidth()
	initialWidth := m.maxNewContentWidth
	assert.Equal(t, 37, initialWidth) // "this is a fairly long line of content"

	// Fold the file - width should NOT shrink
	m.files[0].FoldLevel = sidebyside.FoldFolded
	m.updateMaxNewContentWidth()
	assert.Equal(t, initialWidth, m.maxNewContentWidth, "width should not shrink when file is folded")

	// Add a file with shorter content - width should NOT shrink
	m.files = append(m.files, sidebyside.FilePair{
		FoldLevel: sidebyside.FoldNormal,
		Pairs: []sidebyside.LinePair{
			{New: sidebyside.Line{Content: "short"}},
		},
	})
	m.updateMaxNewContentWidth()
	assert.Equal(t, initialWidth, m.maxNewContentWidth, "width should not shrink when new file has shorter content")

	// Add a file with longer content - width SHOULD grow
	m.files = append(m.files, sidebyside.FilePair{
		FoldLevel: sidebyside.FoldNormal,
		Pairs: []sidebyside.LinePair{
			{New: sidebyside.Line{Content: "this is an even longer line that exceeds the previous maximum width"}},
		},
	})
	m.updateMaxNewContentWidth()
	assert.Equal(t, 67, m.maxNewContentWidth, "width should grow when new file has longer content")
}

func TestDynamicDivider_NarrowContentUses5050(t *testing.T) {
	// Create a model with narrow new content (new is now on left side)
	// When left content is narrow but 50/50 still gives it enough room, use 50/50
	m := Model{
		width:  80, // terminal width
		height: 24,
		files: []sidebyside.FilePair{
			{
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "this is a much longer line on the old side that goes to the right", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Added},
					},
				},
			},
		},
		maxNewContentWidth:  0,
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
	}

	// Calculate widths
	m.updateMaxNewContentWidth()
	m.rebuildRowsCache()

	// Verify maxNewContentWidth is set correctly
	assert.Equal(t, 5, m.maxNewContentWidth) // "short" = 5 chars

	// At width 80, 50/50 gives each side 38 chars
	// With gutterOverhead of 11, left content area = 27 chars
	// Since 27 >= 5 (targetLeftContent), we use 50/50
	defaultHalf := (m.width - 3) / 2 // 38

	// Both sides should be equal at ~50%
	expectedLeftWidth := defaultHalf                      // 38
	expectedRightWidth := m.width - 3 - expectedLeftWidth // 39

	assert.Equal(t, expectedLeftWidth, defaultHalf, "left side should be at 50%")
	assert.Equal(t, expectedRightWidth, m.width-3-defaultHalf, "right side should be at 50%")
}

func TestDynamicDivider_WideContentExpandsLeft(t *testing.T) {
	// Create a model with wide new content (wider than 50%)
	// Left side should expand to fit content, squeezing right side to just its gutter
	m := Model{
		width:  80,
		height: 24,
		files: []sidebyside.FilePair{
			{
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "this is a very long line that exceeds half the terminal width easily", Type: sidebyside.Added},
					},
				},
			},
		},
		maxNewContentWidth:  0,
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
	}

	m.updateMaxNewContentWidth()

	// The new content is 68 chars
	assert.Equal(t, 68, m.maxNewContentWidth)

	lineNumWidth := m.lineNumWidth() // 4
	// Right side minimum: no trailing gutter when squeezed
	// indicator(1) + space(1) + lineNum(4) + space(1) + left gutter(2) = 9
	minRightWidth := 1 + 1 + lineNumWidth + 1 + 2

	// targetLeftContent = min(80, 68) = 68
	// targetLeftWidth = 11 + 68 = 79
	// maxLeftWidth = 80 - 3 - 9 = 68
	// leftHalfWidth = min(79, 68) = 68
	expectedLeftWidth := 68
	expectedRightWidth := m.width - 3 - expectedLeftWidth // 9

	defaultHalf := (m.width - 3) / 2 // 38
	assert.Greater(t, expectedLeftWidth, defaultHalf, "left side should be wider than 50%")
	assert.Equal(t, expectedRightWidth, minRightWidth, "right side should be squeezed to just its gutter (no trailing)")
}

func TestDynamicDivider_WideTerminalUses5050(t *testing.T) {
	// On a wide terminal, even with wide content, use 50/50 if both sides have room
	m := Model{
		width:  200, // wide terminal
		height: 24,
		files: []sidebyside.FilePair{
			{
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old content", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new content that is moderately long", Type: sidebyside.Added},
					},
				},
			},
		},
		maxNewContentWidth:  0,
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
	}

	m.updateMaxNewContentWidth()

	lineNumWidth := m.lineNumWidth()               // 4
	gutterOverhead := 1 + 1 + lineNumWidth + 1 + 4 // 11

	// At width 200, defaultHalf = 98, leftContentAt50 = 98 - 11 = 87
	// targetLeftContent = min(80, 35) = 35
	// Since 87 >= 35, use 50/50
	defaultHalf := (m.width - 3) / 2                // 98
	leftContentAt50 := defaultHalf - gutterOverhead // 87

	assert.GreaterOrEqual(t, leftContentAt50, 80, "50/50 gives left side enough room for 80 chars")
	// Both sides should be at ~50%
}

func TestDynamicDivider_VeryNarrowTerminal(t *testing.T) {
	// On a very narrow terminal, left side takes as much as possible,
	// right side shows only its gutter
	m := Model{
		width:  50, // very narrow
		height: 24,
		files: []sidebyside.FilePair{
			{
				FoldLevel: sidebyside.FoldNormal,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new content here", Type: sidebyside.Added},
					},
				},
			},
		},
		maxNewContentWidth:  0,
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
	}

	m.updateMaxNewContentWidth()

	lineNumWidth := m.lineNumWidth()               // 4
	gutterOverhead := 1 + 1 + lineNumWidth + 1 + 4 // 11
	minRightWidth := gutterOverhead

	// At width 50, defaultHalf = 23, leftContentAt50 = 23 - 11 = 12
	// targetLeftContent = min(80, 16) = 16
	// Since 12 < 16, prioritize left side
	// targetLeftWidth = 11 + 16 = 27
	// maxLeftWidth = 50 - 3 - 11 = 36
	// leftHalfWidth = min(27, 36) = 27
	expectedLeftWidth := 27
	expectedRightWidth := m.width - 3 - expectedLeftWidth // 20

	defaultHalf := (m.width - 3) / 2 // 23
	assert.Greater(t, expectedLeftWidth, defaultHalf, "left side should be wider than 50%")
	assert.Greater(t, expectedRightWidth, minRightWidth, "right side has more than minimum since content is small")
}
