package tui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

var update = flag.Bool("update", false, "update golden files")

func init() {
	// Force ASCII color profile for consistent test output
	lipgloss.SetColorProfile(termenv.Ascii)
}

func TestView_BasicRender(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "old line", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 2, Content: "new line", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "func main() {}", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 3, Content: "func main() {}", Type: sidebyside.Context},
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

	goldenPath := filepath.Join("testdata", "basic_render.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_WithScroll(t *testing.T) {
	// Create enough lines to require scrolling
	pairs := make([]sidebyside.LinePair, 20)
	for i := range pairs {
		pairs[i] = sidebyside.LinePair{
			Left:  sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: i + 1, Content: "line content", Type: sidebyside.Context},
		}
	}

	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", Pairs: pairs},
		},
		width:  80,
		height: 5,
		scroll: 10, // Start scrolled down
		keys:   DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "scrolled_view.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_AddedAndRemovedLines(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					// Pure addition (empty left)
					{
						Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
						Right: sidebyside.Line{Num: 1, Content: "added line", Type: sidebyside.Added},
					},
					// Pure removal (empty right)
					{
						Left:  sidebyside.Line{Num: 1, Content: "removed line", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
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

	goldenPath := filepath.Join("testdata", "added_removed.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_MultipleFiles(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/one.go",
				NewPath: "b/one.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "file one", Type: sidebyside.Context},
					},
				},
			},
			{
				OldPath: "a/two.go",
				NewPath: "b/two.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "file two", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "file two", Type: sidebyside.Context},
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

	goldenPath := filepath.Join("testdata", "multiple_files.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

func TestView_EmptyModel(t *testing.T) {
	m := Model{
		files:  nil,
		width:  80,
		height: 10,
		keys:   DefaultKeyMap(),
	}

	output := m.View()
	assert.Equal(t, "", output)
}

func TestView_ZeroSize(t *testing.T) {
	m := Model{
		files: []sidebyside.FilePair{
			{OldPath: "a/foo.go", NewPath: "b/foo.go"},
		},
		width:  0,
		height: 0,
		keys:   DefaultKeyMap(),
	}

	output := m.View()
	assert.Equal(t, "", output)
}

func TestView_HorizontalScroll(t *testing.T) {
	// Create content with long lines to test horizontal scrolling
	m := Model{
		files: []sidebyside.FilePair{
			{
				OldPath: "a/foo.go",
				NewPath: "b/foo.go",
				Pairs: []sidebyside.LinePair{
					{
						Left:  sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 1, Content: "short", Type: sidebyside.Context},
					},
					{
						Left:  sidebyside.Line{Num: 2, Content: "this is a much longer line that will be truncated without scroll", Type: sidebyside.Removed},
						Right: sidebyside.Line{Num: 2, Content: "this is a much longer line that will be truncated without scroll", Type: sidebyside.Added},
					},
					{
						Left:  sidebyside.Line{Num: 3, Content: "0123456789abcdefghij", Type: sidebyside.Context},
						Right: sidebyside.Line{Num: 3, Content: "0123456789abcdefghij", Type: sidebyside.Context},
					},
				},
			},
		},
		width:       80,
		height:      10,
		hscroll:     8, // Scroll right by 8 columns
		hscrollStep: DefaultHScrollStep,
		keys:        DefaultKeyMap(),
	}
	m.calculateTotalLines()

	output := m.View()

	goldenPath := filepath.Join("testdata", "horizontal_scroll.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}
