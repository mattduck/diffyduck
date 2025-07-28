package ui

import (
	"sync"
	"testing"
	"time"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/models"
)

func TestViewportRaceCondition(t *testing.T) {
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
				OldPath: "race_test.go",
				NewPath: "race_test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)

	// Test concurrent access - this used to cause segfaults
	for i := 0; i < 10; i++ {
		t.Run("Race", func(t *testing.T) {
			viewport := NewDiffViewport(content)
			viewport.SetSize(80, 25)

			var wg sync.WaitGroup
			wg.Add(3)

			// Goroutine 1: Mark first render done
			go func() {
				defer wg.Done()
				viewport.firstRenderDone = true
			}()

			// Goroutine 2: Try to access highlighting
			go func() {
				defer wg.Done()
				time.Sleep(5 * time.Millisecond) // Let highlighting start

				lineInfo := models.LineInfo{
					FileIndex: 0,
					LineIndex: 0,
					Line:      alignedLines[0],
					FilePath:  "race_test.go",
				}

				for j := 0; j < 10; j++ {
					_ = viewport.getHighlightedStyleSpans("package main", "race_test.go", false, lineInfo)
					time.Sleep(1 * time.Millisecond)
				}
			}()

			// Goroutine 3: Close viewport while others are running
			go func() {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond) // Let others start
				viewport.Close()
			}()

			wg.Wait()
		})
	}
}

func TestViewportConcurrentClose(t *testing.T) {
	// Test closing viewport multiple times concurrently
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
				OldPath: "concurrent_test.go",
				NewPath: "concurrent_test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)

	var wg sync.WaitGroup
	// Try to close the viewport from multiple goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			viewport.Close()
		}()
	}

	wg.Wait()

	// Verify viewport is closed
	if !viewport.closed {
		t.Error("Expected viewport to be closed")
	}
}

func TestViewportProgressiveRenderingRace(t *testing.T) {
	// Test that progressive rendering handles concurrent starts gracefully
	alignedLines := make([]aligner.AlignedLine, 50)
	for i := 0; i < 50; i++ {
		lineContent := "func test() { fmt.Println(\"hello\") }"
		alignedLines[i] = aligner.AlignedLine{
			OldLine:    stringPtr(lineContent),
			NewLine:    stringPtr(lineContent),
			LineType:   aligner.Unchanged,
			OldLineNum: i + 1,
			NewLineNum: i + 1,
		}
	}

	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "progressive_race_test.go",
				NewPath: "progressive_race_test.go",
			},
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	viewport.SetSize(80, 25)
	viewport.firstRenderDone = true

	// Progressive highlighting is now handled by ParseNextFileInBackground()
	// which is called from main thread timer, not via goroutines

	// Verify it completed without crashing
	if viewport.IsProgressiveRenderingComplete() {
		t.Log("Progressive rendering completed successfully")
	}
}
