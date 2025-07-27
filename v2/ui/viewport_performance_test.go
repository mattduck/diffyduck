package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/models"
)

func TestViewportPerformanceWithLargeFile(t *testing.T) {
	// Create a large Go file to test performance
	alignedLines := make([]aligner.AlignedLine, 0, 1000)

	// Add package declaration
	alignedLines = append(alignedLines, aligner.AlignedLine{
		OldLine:    stringPtr("package main"),
		NewLine:    stringPtr("package main"),
		LineType:   aligner.Unchanged,
		OldLineNum: 1,
		NewLineNum: 1,
	})

	// Add imports
	alignedLines = append(alignedLines, aligner.AlignedLine{
		OldLine:    stringPtr("import ("),
		NewLine:    stringPtr("import ("),
		LineType:   aligner.Unchanged,
		OldLineNum: 2,
		NewLineNum: 2,
	})

	// Add many function definitions
	for i := 0; i < 500; i++ {
		lineNum := i + 3
		funcLine := fmt.Sprintf("func test%d() { fmt.Println(\"hello %d\") }", i, i)
		alignedLines = append(alignedLines, aligner.AlignedLine{
			OldLine:    stringPtr(funcLine),
			NewLine:    stringPtr(funcLine),
			LineType:   aligner.Unchanged,
			OldLineNum: lineNum,
			NewLineNum: lineNum,
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

	// Time viewport creation (includes pre-parsing)
	start := time.Now()
	viewport := NewDiffViewport(content)
	creationTime := time.Since(start)
	defer viewport.Close()

	t.Logf("Viewport creation took: %v", creationTime)
	if creationTime > 100*time.Millisecond {
		t.Errorf("Viewport creation too slow: %v (expected < 100ms)", creationTime)
	}

	// Test line highlighting performance (after first render)
	viewport.firstRenderDone = true // Simulate first render complete

	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 10,
		Line:      alignedLines[10],
		FilePath:  "large_test.go",
	}

	// Time multiple highlighting calls
	start = time.Now()
	for i := 0; i < 50; i++ {
		lineInfo.LineIndex = i
		lineInfo.Line = alignedLines[i]
		_ = viewport.getHighlightedStyleSpans(*alignedLines[i].OldLine, "large_test.go", true, lineInfo)
	}
	highlightingTime := time.Since(start)

	t.Logf("50 line highlighting calls took: %v (avg: %v per line)",
		highlightingTime, highlightingTime/50)

	avgPerLine := highlightingTime / 50
	if avgPerLine > 2*time.Millisecond {
		t.Errorf("Line highlighting too slow: %v per line (expected < 2ms)", avgPerLine)
	}
}

func TestViewportScrollingPerformance(t *testing.T) {
	// Create medium-sized file
	alignedLines := make([]aligner.AlignedLine, 0, 200)

	for i := 0; i < 200; i++ {
		lineNum := i + 1
		lineContent := fmt.Sprintf("func test%d() { return %d }", i, i)
		alignedLines = append(alignedLines, aligner.AlignedLine{
			OldLine:    stringPtr(lineContent),
			NewLine:    stringPtr(lineContent),
			LineType:   aligner.Unchanged,
			OldLineNum: lineNum,
			NewLineNum: lineNum,
		})
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "scroll_test.go",
				NewPath: "scroll_test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	viewport.SetSize(80, 25) // Typical terminal size

	// Test page scrolling performance
	start := time.Now()
	for i := 0; i < 10; i++ {
		viewport.ScrollVertical(25) // One page down
	}
	scrollTime := time.Since(start)

	t.Logf("10 page scrolls took: %v (avg: %v per scroll)",
		scrollTime, scrollTime/10)

	avgPerScroll := scrollTime / 10
	if avgPerScroll > 5*time.Millisecond {
		t.Errorf("Page scrolling too slow: %v per scroll (expected < 5ms)", avgPerScroll)
	}
}

func BenchmarkLineHighlighting(b *testing.B) {
	// Create test file
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
				OldPath: "bench_test.go",
				NewPath: "bench_test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 1,
		Line:      alignedLines[1],
		FilePath:  "bench_test.go",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = viewport.getHighlightedStyleSpans("func test() { fmt.Println(\"hello\") }", "bench_test.go", true, lineInfo)
	}
}

func BenchmarkViewportCreation(b *testing.B) {
	// Create test content once
	alignedLines := make([]aligner.AlignedLine, 100)
	for i := 0; i < 100; i++ {
		lineNum := i + 1
		lineContent := fmt.Sprintf("func test%d() { return %d }", i, i)
		alignedLines[i] = aligner.AlignedLine{
			OldLine:    stringPtr(lineContent),
			NewLine:    stringPtr(lineContent),
			LineType:   aligner.Unchanged,
			OldLineNum: lineNum,
			NewLineNum: lineNum,
		}
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "bench_creation.go",
				NewPath: "bench_creation.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		content := models.NewDiffContent(files)
		viewport := NewDiffViewport(content)
		viewport.Close()
	}
}

func TestFirstRenderPerformance(t *testing.T) {
	// Create test content
	alignedLines := make([]aligner.AlignedLine, 0, 100)

	for i := 0; i < 100; i++ {
		lineNum := i + 1
		funcLine := fmt.Sprintf("func test%d() { fmt.Println(\"hello %d\") }", i, i)
		alignedLines = append(alignedLines, aligner.AlignedLine{
			OldLine:    stringPtr(funcLine),
			NewLine:    stringPtr(funcLine),
			LineType:   aligner.Unchanged,
			OldLineNum: lineNum,
			NewLineNum: lineNum,
		})
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "first_render_test.go",
				NewPath: "first_render_test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// Test first render performance (without syntax highlighting)
	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		Line:      alignedLines[0],
		FilePath:  "first_render_test.go",
	}

	start := time.Now()
	for i := 0; i < 25; i++ { // Simulate viewport height
		lineInfo.LineIndex = i
		lineInfo.Line = alignedLines[i]
		spans := viewport.getHighlightedStyleSpans(*alignedLines[i].OldLine, "first_render_test.go", true, lineInfo)

		// Should return nil during progressive mode before first render
		if spans != nil {
			t.Error("Expected no highlighting during first render in progressive mode")
		}
	}
	firstRenderTime := time.Since(start)

	t.Logf("First render (25 lines, no highlighting) took: %v", firstRenderTime)
	if firstRenderTime > 5*time.Millisecond {
		t.Errorf("First render too slow: %v (expected < 5ms)", firstRenderTime)
	}
}
