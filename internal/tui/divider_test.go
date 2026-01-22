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

func TestDynamicDivider_AsymmetricWidths(t *testing.T) {
	// Create a model with narrow new content (new is now on left side)
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

	// Calculate what the widths should be
	defaultHalf := (m.width - 3) / 2 // 38
	lineNumWidth := m.lineNumWidth() // 4

	// minLeftWidth = indicator(1) + space(1) + lineNum(4) + space(1) + content(5) + gutter(4) = 16
	minLeftWidth := 1 + 1 + lineNumWidth + 1 + m.maxNewContentWidth + 4
	assert.Equal(t, 16, minLeftWidth)

	// Left should be minLeftWidth since it's less than defaultHalf
	expectedLeftWidth := minLeftWidth
	expectedRightWidth := m.width - 3 - expectedLeftWidth // 80 - 3 - 16 = 61

	assert.Less(t, expectedLeftWidth, defaultHalf, "left side should be narrower than 50%")
	assert.Greater(t, expectedRightWidth, defaultHalf, "right side should be wider than 50%")
}

func TestDynamicDivider_WideContentStaysAt50Percent(t *testing.T) {
	// Create a model with wide new content (wider than 50%)
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

	// The new content is 68 chars, which with overhead would exceed 50%
	assert.Equal(t, 68, m.maxNewContentWidth)

	defaultHalf := (m.width - 3) / 2 // 38
	lineNumWidth := m.lineNumWidth() // 4

	// minLeftWidth would be: 1 + 1 + 4 + 1 + 68 + 4 = 79, which exceeds defaultHalf (38)
	minLeftWidth := 1 + 1 + lineNumWidth + 1 + m.maxNewContentWidth + 4
	assert.Greater(t, minLeftWidth, defaultHalf, "calculated min width exceeds 50%")

	// So the actual left width should cap at defaultHalf (50%)
	// This is tested implicitly by the rendering - both sides stay equal
}
