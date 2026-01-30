package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// === File Stats Tests ===

func TestCountFileStats(t *testing.T) {
	tests := []struct {
		name        string
		pairs       []sidebyside.LinePair
		wantAdded   int
		wantRemoved int
	}{
		{
			name:        "empty file",
			pairs:       []sidebyside.LinePair{},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "only context lines",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
				},
			},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "pure additions",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 1, Content: "new 1", Type: sidebyside.Added},
				},
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 2, Content: "new 2", Type: sidebyside.Added},
				},
			},
			wantAdded:   2,
			wantRemoved: 0,
		},
		{
			name: "pure deletions",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "old 1", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "old 2", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
				{
					Old: sidebyside.Line{Num: 3, Content: "old 3", Type: sidebyside.Removed},
					New: sidebyside.Line{Type: sidebyside.Empty},
				},
			},
			wantAdded:   0,
			wantRemoved: 3,
		},
		{
			name: "mixed changes",
			pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "context", Type: sidebyside.Context},
				},
				{
					Old: sidebyside.Line{Num: 2, Content: "old", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 2, Content: "new", Type: sidebyside.Added},
				},
				{
					Old: sidebyside.Line{Type: sidebyside.Empty},
					New: sidebyside.Line{Num: 3, Content: "added", Type: sidebyside.Added},
				},
			},
			wantAdded:   2,
			wantRemoved: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := sidebyside.FilePair{Pairs: tt.pairs}
			added, removed := countFileStats(fp)
			assert.Equal(t, tt.wantAdded, added, "added count")
			assert.Equal(t, tt.wantRemoved, removed, "removed count")
		})
	}
}

func TestFormatStatsBar(t *testing.T) {
	tests := []struct {
		name     string
		added    int
		removed  int
		maxWidth int
		want     string
	}{
		{
			name:     "no changes",
			added:    0,
			removed:  0,
			maxWidth: 24,
			want:     "",
		},
		{
			name:     "only additions - small",
			added:    5,
			removed:  0,
			maxWidth: 24,
			want:     "+5 +++++",
		},
		{
			name:     "only deletions - small",
			added:    0,
			removed:  3,
			maxWidth: 24,
			want:     "-3 ---",
		},
		{
			name:     "mixed - fits in max",
			added:    10,
			removed:  5,
			maxWidth: 24,
			want:     "+10 -5 ++++++++++-----",
		},
		{
			name:     "mixed - needs scaling",
			added:    30,
			removed:  18,
			maxWidth: 24,
			want:     "+30 -18 +++++++++++++++---------", // scaled: 30+18=48, scale=24/48=0.5, so 15+ and 9-
		},
		{
			name:     "large numbers - heavy scaling",
			added:    100,
			removed:  100,
			maxWidth: 24,
			want:     "+100 -100 ++++++++++++------------", // scaled: 12+ and 12-
		},
		{
			name:     "pure addition - needs scaling",
			added:    48,
			removed:  0,
			maxWidth: 24,
			want:     "+48 ++++++++++++++++++++++++", // scaled to 24
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatsBar(tt.added, tt.removed, tt.maxWidth)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileHeaderWithStats_FoldedOnly(t *testing.T) {
	// Stats should only appear in folded view, not in normal/expanded
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldNormal, // Normal view - no stats
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()
	lines := strings.Split(output, "\n")
	header := lines[0]

	// Normal view should NOT contain stats
	assert.Contains(t, header, "main.go", "header should contain filename")
	assert.NotContains(t, header, "|", "normal view header should not contain stats separator")
}

func TestFileHeaderWithStats_Folded(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/main.go",
				NewPath:   "b/main.go",
				FoldLevel: sidebyside.FoldFolded, // Folded view - show stats
				Pairs: []sidebyside.LinePair{
					// 3 additions, 2 deletions
					{
						Old: sidebyside.Line{Num: 1, Content: "old1", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 1, Content: "new1", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "old2", Type: sidebyside.Removed},
						New: sidebyside.Line{Num: 2, Content: "new2", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 3, Content: "new3", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.scroll = m.minScroll() // Position cursor at top so header is visible

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout at minScroll: [topBar, divider, padding, header, ...]
	// lines[3] = header (at cursorOffset position)
	header := lines[3]
	assert.Contains(t, header, "main.go", "header should contain filename")
	assert.Contains(t, header, "+3", "header should show addition count")
	assert.Contains(t, header, "-2", "header should show deletion count")
	// Note: With tree-style layout, headers no longer have trailing shading
}

func TestFileHeaderWithStats_StatsColumnAlignment(t *testing.T) {
	// Stats columns (+N, -M) should appear immediately after filename
	// e.g., "+100" vs "+5" should have their + signs at the same position
	pairs100 := make([]sidebyside.LinePair, 100)
	for i := range pairs100 {
		pairs100[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	pairs5 := make([]sidebyside.LinePair, 5)
	for i := range pairs5 {
		pairs5[i] = sidebyside.LinePair{
			Old: sidebyside.Line{Type: sidebyside.Empty},
			New: sidebyside.Line{Num: i + 1, Content: "added", Type: sidebyside.Added},
		}
	}

	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/view.go",
				NewPath:   "b/view.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     pairs100, // +100 additions
			},
			{
				OldPath:   "a/test.go",
				NewPath:   "b/test.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs:     pairs5, // +5 additions
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.scroll = m.minScroll() // Position cursor at top so headers are visible

	output := m.View()
	lines := strings.Split(output, "\n")

	// Layout at minScroll: [topBar, divider, padding, header1, header2, ...]
	header1 := lines[3] // +100
	header2 := lines[4] // +5

	// Both headers should contain their respective counts
	assert.Contains(t, header1, "+100", "first header should show +100")
	assert.Contains(t, header2, "+5", "second header should show +5")
}

func TestFileHeaderWithStats_OnlyAdditions(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "/dev/null",
				NewPath:   "b/newfile.go",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Added},
					},
					{
						Old: sidebyside.Line{Type: sidebyside.Empty},
						New: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Added},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.scroll = m.minScroll() // Position cursor at top so header is visible

	output := m.View()
	lines := strings.Split(output, "\n")
	// Layout at minScroll: [topBar, divider, padding, header, ...]
	header := lines[3]

	assert.Contains(t, header, "newfile.go", "header should contain filename")
	assert.Contains(t, header, "+2", "header should show addition count")
	assert.NotContains(t, header, "-", "header should not show deletion count when zero")
}

func TestFileHeaderWithStats_OnlyDeletions(t *testing.T) {
	m := Model{
		focused: true,
		files: []sidebyside.FilePair{
			{
				OldPath:   "a/deleted.go",
				NewPath:   "/dev/null",
				FoldLevel: sidebyside.FoldFolded,
				Pairs: []sidebyside.LinePair{
					{
						Old: sidebyside.Line{Num: 1, Content: "line1", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 2, Content: "line2", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
					{
						Old: sidebyside.Line{Num: 3, Content: "line3", Type: sidebyside.Removed},
						New: sidebyside.Line{Type: sidebyside.Empty},
					},
				},
			},
		},
		width:  100,
		height: 10,
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()
	m.scroll = m.minScroll() // Position cursor at top so header is visible

	output := m.View()
	lines := strings.Split(output, "\n")
	// Layout at minScroll: [topBar, divider, padding, header, ...]
	header := lines[3]

	assert.Contains(t, header, "deleted.go", "header should contain filename")
	assert.Contains(t, header, "-3", "header should show deletion count")
	// Check there's no + count (but the filename might contain + in other contexts)
	// The format should be "-3 ---" not "+0 -3 ---"
}
