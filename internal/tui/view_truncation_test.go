package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// === Truncation Indicator Tests ===

// Tests for per-side truncation indicator display.
// When a file is truncated, the truncation indicator should show:
// - On the left (old) side only if only the old content was truncated
// - On the right (new) side only if only the new content was truncated
// - On both sides if both were truncated

func TestBuildRows_TruncationIndicator_OldSideOnly(t *testing.T) {
	// When only old (left) side is truncated, indicator should have truncateOld=true, truncateNew=false
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old content", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true")
	assert.False(t, truncRow.truncateNew, "truncateNew should be false")
}

func TestBuildRows_TruncationIndicator_NewSideOnly(t *testing.T) {
	// When only new (right) side is truncated, indicator should have truncateOld=false, truncateNew=true
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: false,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new content", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.False(t, truncRow.truncateOld, "truncateOld should be false")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true")
}

func TestBuildRows_TruncationIndicator_BothSides(t *testing.T) {
	// When both sides are truncated, indicator should have both flags true
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Find the truncation indicator row
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true")
}

func TestBuildRows_TruncationIndicator_NotTruncated(t *testing.T) {
	// When file is not truncated, should have no truncation indicator
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    false,
				OldTruncated: false,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	// Should have no truncation indicator row
	for _, row := range rows {
		assert.False(t, row.isTruncationIndicator, "should have no truncation indicator when not truncated")
	}
}

func TestBuildRows_TruncationIndicator_FullFileView_OldSideOnly(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:             "a/test.go",
				NewPath:             "b/test.go",
				FoldLevel:           sidebyside.FoldExpanded,
				ShowFullFile:        true,
				OldContent:          []string{"line1", "line2"},
				NewContent:          []string{"line1"},
				ContentTruncated:    true,
				OldContentTruncated: true,
				NewContentTruncated: false,
				Pairs:               []sidebyside.LinePair{},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	rows := m.buildRows()
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}
	require.NotNil(t, truncRow)
	assert.True(t, truncRow.truncateOld)
	assert.False(t, truncRow.truncateNew)
}

func TestBuildRows_TruncationIndicator_FullFileView_NewSideOnly(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:             "a/test.go",
				NewPath:             "b/test.go",
				FoldLevel:           sidebyside.FoldExpanded,
				ShowFullFile:        true,
				OldContent:          []string{"line1"},
				NewContent:          []string{"line1", "line2"},
				ContentTruncated:    true,
				OldContentTruncated: false,
				NewContentTruncated: true,
				Pairs:               []sidebyside.LinePair{},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	rows := m.buildRows()
	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}
	require.NotNil(t, truncRow)
	assert.False(t, truncRow.truncateOld)
	assert.True(t, truncRow.truncateNew)
}

func TestBuildRows_TruncationIndicator_NewFile(t *testing.T) {
	// For a new file (OldPath=/dev/null), only new side can be truncated
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "/dev/null",
				NewPath:      "b/newfile.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: false, // Can't truncate non-existent old file
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new content", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.False(t, truncRow.truncateOld, "truncateOld should be false for new file")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true")
}

func TestBuildRows_TruncationIndicator_DeletedFile(t *testing.T) {
	// For a deleted file (NewPath=/dev/null), only old side can be truncated
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/deleted.go",
				NewPath:      "/dev/null",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: false, // Can't truncate non-existent new file
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old content", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  80,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "should have a truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true")
	assert.False(t, truncRow.truncateNew, "truncateNew should be false for deleted file")
}

func TestRenderTruncationIndicator_LeftSideOnly(t *testing.T) {
	// Verify rendering shows truncation on left side only
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Test rendering with truncateOld=true, truncateNew=false
	output := m.renderTruncationIndicator("[truncated]", false, true, false)

	// Should contain dots on left side
	assert.Contains(t, output, "·", "should contain dots")

	// The output should have the side-by-side structure with separator
	assert.Contains(t, output, "│", "should contain vertical separator")

	// Left side should have the message, right side should be empty/blank
	parts := strings.Split(output, "│")
	require.Len(t, parts, 2, "should have left and right parts separated by │")

	leftPart := parts[0]
	rightPart := parts[1]

	assert.Contains(t, leftPart, "truncated", "left side should contain truncation message")
	assert.NotContains(t, rightPart, "truncated", "right side should not contain truncation message")
}

func TestRenderTruncationIndicator_RightSideOnly(t *testing.T) {
	// Verify rendering shows truncation on right side only
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Test rendering with truncateOld=false, truncateNew=true
	output := m.renderTruncationIndicator("[truncated]", false, false, true)

	// Should have the side-by-side structure
	assert.Contains(t, output, "│", "should contain vertical separator")

	parts := strings.Split(output, "│")
	require.Len(t, parts, 2, "should have left and right parts separated by │")

	leftPart := parts[0]
	rightPart := parts[1]

	assert.NotContains(t, leftPart, "truncated", "left side should not contain truncation message")
	assert.Contains(t, rightPart, "truncated", "right side should contain truncation message")
}

func TestRenderTruncationIndicator_BothSides(t *testing.T) {
	// Verify rendering shows truncation on both sides
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldExpanded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "x", Type: sidebyside.Context},
					},
				},
			},
		},
		width:  100,
		height: 20,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	// Test rendering with truncateOld=true, truncateNew=true
	output := m.renderTruncationIndicator("[truncated]", false, true, true)

	// Should have the side-by-side structure
	assert.Contains(t, output, "│", "should contain vertical separator")

	parts := strings.Split(output, "│")
	require.Len(t, parts, 2, "should have left and right parts separated by │")

	leftPart := parts[0]
	rightPart := parts[1]

	// Both sides should have dots
	assert.Contains(t, leftPart, "·", "left side should have dots")
	assert.Contains(t, rightPart, "·", "right side should have dots")
}

// === Pager Mode Truncation Tests ===
// In pager mode, only diff-parsing truncation applies (no content fetching)

func TestBuildRows_TruncationIndicator_PagerMode_OldSideOnly(t *testing.T) {
	// Pager mode with only old side truncated from diff parsing
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "pager mode should have truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true in pager mode")
	assert.False(t, truncRow.truncateNew, "truncateNew should be false in pager mode")
}

func TestBuildRows_TruncationIndicator_PagerMode_NewSideOnly(t *testing.T) {
	// Pager mode with only new side truncated from diff parsing
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: false,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "pager mode should have truncation indicator row")
	assert.False(t, truncRow.truncateOld, "truncateOld should be false in pager mode")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true in pager mode")
}

func TestBuildRows_TruncationIndicator_PagerMode_BothSides(t *testing.T) {
	// Pager mode with both sides truncated from diff parsing
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    true,
				OldTruncated: true,
				NewTruncated: true,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new", Type: sidebyside.Added},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	var truncRow *displayRow
	for i := range rows {
		if rows[i].isTruncationIndicator {
			truncRow = &rows[i]
			break
		}
	}

	require.NotNil(t, truncRow, "pager mode should have truncation indicator row")
	assert.True(t, truncRow.truncateOld, "truncateOld should be true in pager mode")
	assert.True(t, truncRow.truncateNew, "truncateNew should be true in pager mode")
}

func TestBuildRows_TruncationIndicator_PagerMode_NotTruncated(t *testing.T) {
	// Pager mode with no truncation
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:      "a/test.go",
				NewPath:      "b/test.go",
				FoldLevel:    sidebyside.FoldExpanded,
				Truncated:    false,
				OldTruncated: false,
				NewTruncated: false,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
						New: sidebyside.Line{Num: 1, Content: "content", Type: sidebyside.Context},
					},
				},
			},
		},
		width:     80,
		height:    20,
		pagerMode: true,
		keys:      DefaultKeyMap(),
	}
	m.calculateTotalLines()

	rows := m.buildRows()

	for _, row := range rows {
		assert.False(t, row.isTruncationIndicator, "pager mode should have no truncation indicator when not truncated")
	}
}
