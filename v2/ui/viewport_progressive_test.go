package ui

import (
	"testing"
	"time"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/models"
)

func TestProgressiveRendering(t *testing.T) {
	// Create test content
	alignedLines := []aligner.AlignedLine{
		{
			OldLine:    stringPtr("package main"),
			NewLine:    stringPtr("package main"),
			LineType:   aligner.Unchanged,
			OldLineNum: 1,
			NewLineNum: 1,
		},
		{
			OldLine:    stringPtr("func test() { fmt.Println(\"hello\") }"),
			NewLine:    stringPtr("func test() { fmt.Println(\"hello\") }"),
			LineType:   aligner.Unchanged,
			OldLineNum: 2,
			NewLineNum: 2,
		},
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test.go",
				NewPath: "test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// Verify progressive mode is enabled by default
	if !viewport.progressiveMode {
		t.Error("Expected progressive mode to be enabled by default")
	}

	// Force complete highlighting for testing
	viewport.ForceCompleteHighlighting()

	// After forcing complete highlighting, it should work
	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		Line:      alignedLines[0],
		FilePath:  "test.go",
	}

	spans := viewport.getHighlightedStyleSpans("package main", "test.go", false, lineInfo)
	if spans == nil {
		t.Error("Expected highlighting to work after forcing complete highlighting")
	}

	// Verify the highlighter was initialized
	if viewport.enhancedHighlighter == nil {
		t.Error("Expected enhanced highlighter to be initialized")
	}
}

func TestProgressiveRenderingDisabled(t *testing.T) {
	// Create test content
	alignedLines := []aligner.AlignedLine{
		{
			OldLine:    stringPtr("package main"),
			NewLine:    stringPtr("package main"),
			LineType:   aligner.Unchanged,
			OldLineNum: 1,
			NewLineNum: 1,
		},
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test.go",
				NewPath: "test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// Disable progressive mode
	viewport.SetProgressiveMode(false)

	// Verify progressive mode is disabled
	if viewport.progressiveMode {
		t.Error("Expected progressive mode to be disabled")
	}

	// Verify first render is marked as done
	if !viewport.firstRenderDone {
		t.Error("Expected firstRenderDone to be true when progressive mode is disabled")
	}

	// Highlighting should work immediately
	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		Line:      alignedLines[0],
		FilePath:  "test.go",
	}

	_ = viewport.getHighlightedStyleSpans("package main", "test.go", false, lineInfo)
	// Should not error even if no highlighting available
}

func TestProgressiveRenderingCompletion(t *testing.T) {
	// Create test content
	alignedLines := []aligner.AlignedLine{
		{
			OldLine:    stringPtr("package main"),
			NewLine:    stringPtr("package main"),
			LineType:   aligner.Unchanged,
			OldLineNum: 1,
			NewLineNum: 1,
		},
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test.go",
				NewPath: "test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// Initially not complete
	if viewport.IsProgressiveRenderingComplete() {
		t.Error("Expected progressive rendering to not be complete initially")
	}

	// Mark first render as done but background highlighting still in progress
	viewport.firstRenderDone = true
	viewport.backgroundHighlighting = true

	if viewport.IsProgressiveRenderingComplete() {
		t.Error("Expected progressive rendering to not be complete while background highlighting")
	}

	// Mark background highlighting as done
	viewport.backgroundHighlighting = false

	if !viewport.IsProgressiveRenderingComplete() {
		t.Error("Expected progressive rendering to be complete")
	}
}

func TestBackgroundHighlightingPerformance(t *testing.T) {
	// Create larger test content
	alignedLines := make([]aligner.AlignedLine, 0, 50)
	for i := 0; i < 50; i++ {
		lineContent := "func test() { fmt.Println(\"hello\") }"
		alignedLines = append(alignedLines, aligner.AlignedLine{
			OldLine:    stringPtr(lineContent),
			NewLine:    stringPtr(lineContent),
			LineType:   aligner.Unchanged,
			OldLineNum: i + 1,
			NewLineNum: i + 1,
		})
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "large_test.go",
				NewPath: "large_test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	viewport.SetSize(80, 25) // Set reasonable viewport size

	// Test that background highlighting doesn't block
	start := time.Now()
	viewport.startProgressiveHighlighting()
	elapsed := time.Since(start)

	// Background highlighting should complete quickly (mostly async)
	if elapsed > 200*time.Millisecond {
		t.Errorf("Background highlighting took too long: %v", elapsed)
	}

	// Verify it completed
	if viewport.backgroundHighlighting {
		t.Error("Expected background highlighting to be complete")
	}
}
