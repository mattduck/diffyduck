package ui

import (
	"testing"
	"time"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/models"
	"github.com/stretchr/testify/assert"
)

func TestNewDiffViewport(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	assert.NotNil(t, viewport)
	assert.Equal(t, content, viewport.content)
	assert.NotNil(t, viewport.highlightCache)
	assert.Equal(t, defaultCacheSize, viewport.cacheSize)
}

func TestViewportSizing(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	viewport.SetSize(100, 50)
	assert.Equal(t, 100, viewport.width)
	assert.Equal(t, 50, viewport.height)
	assert.Equal(t, 50, viewport.GetHeight())
}

func TestVerticalScrolling(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	// Test scrolling down (limited by content bounds)
	viewport.ScrollVertical(5)
	if content.TotalLines <= 5 {
		assert.Equal(t, content.TotalLines-1, viewport.offsetY)
	} else {
		assert.Equal(t, 5, viewport.offsetY)
	}

	// Test scrolling to bounds
	viewport.ScrollVertical(1000)
	assert.Equal(t, content.TotalLines-1, viewport.offsetY)

	// Test scrolling up
	viewport.ScrollVertical(-1)
	assert.Equal(t, content.TotalLines-2, viewport.offsetY)

	// Test scrolling to negative bounds
	viewport.ScrollVertical(-1000)
	assert.Equal(t, 0, viewport.offsetY)
}

func TestHorizontalScrolling(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	// Test scrolling right
	viewport.ScrollHorizontal(10)
	assert.Equal(t, 10, viewport.offsetX)

	// Test scrolling left
	viewport.ScrollHorizontal(-5)
	assert.Equal(t, 5, viewport.offsetX)

	// Test scrolling to negative bounds
	viewport.ScrollHorizontal(-100)
	assert.Equal(t, 0, viewport.offsetX)
}

func TestHorizontalOffsetApplication(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	tests := []struct {
		name     string
		input    string
		offset   int
		width    int
		expected string
	}{
		{
			name:     "no offset",
			input:    "hello world",
			offset:   0,
			width:    20,
			expected: "hello world         ", // padded to width
		},
		{
			name:     "with offset",
			input:    "hello world",
			offset:   6,
			width:    10,
			expected: "world     ", // padded to width
		},
		{
			name:     "offset beyond content",
			input:    "short",
			offset:   10,
			width:    5,
			expected: "     ", // all spaces
		},
		{
			name:     "truncate long content",
			input:    "this is a very long line",
			offset:   0,
			width:    10,
			expected: "this is a ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viewport.offsetX = tt.offset
			result := viewport.applyHorizontalOffset(tt.input, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHighlightCaching(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		FilePath:  "test.go",
	}

	// First call should cache the result
	result1 := viewport.getHighlightedContent("func test() {}", "test.go", false, lineInfo)
	assert.NotEmpty(t, result1)
	assert.Len(t, viewport.highlightCache, 1)

	// Second call should use cache
	result2 := viewport.getHighlightedContent("func test() {}", "test.go", false, lineInfo)
	assert.Equal(t, result1, result2)
	assert.Len(t, viewport.highlightCache, 1) // Should still be 1
}

func TestCacheCleanup(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)
	viewport.cacheSize = 3 // Small cache for testing

	// Fill cache beyond capacity
	for i := 0; i < 5; i++ {
		lineInfo := models.LineInfo{
			FileIndex: 0,
			LineIndex: i,
			FilePath:  "test.go",
		}
		viewport.getHighlightedContent("test content", "test.go", false, lineInfo)
	}

	// Cache should be cleaned up
	assert.LessOrEqual(t, len(viewport.highlightCache), viewport.cacheSize)
}

func TestRenderStatsTracking(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	// Initial stats should be zero
	renderTime, renderCount := viewport.GetRenderStats()
	assert.Equal(t, time.Duration(0), renderTime)
	assert.Equal(t, 0, renderCount)
}

func TestCloseCleanup(t *testing.T) {
	content := createTestContent()
	viewport := NewDiffViewport(content)

	// Should not panic
	viewport.Close()

	// Should be safe to call multiple times
	viewport.Close()
}

// Helper to create test content
func createTestContent() *models.DiffContent {
	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test.go",
				NewPath: "test.go",
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("func oldTest() {"),
					NewLine:    stringPtr("func newTest() {"),
					LineType:   aligner.Modified,
					OldLineNum: 1,
					NewLineNum: 1,
				},
				{
					OldLine:    stringPtr("    return nil"),
					NewLine:    stringPtr("    return nil"),
					LineType:   aligner.Unchanged,
					OldLineNum: 2,
					NewLineNum: 2,
				},
				{
					OldLine:    nil,
					NewLine:    stringPtr("    // Added comment"),
					LineType:   aligner.Added,
					OldLineNum: 0,
					NewLineNum: 3,
				},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
		},
	}

	return models.NewDiffContent(files)
}

func stringPtr(s string) *string {
	return &s
}
