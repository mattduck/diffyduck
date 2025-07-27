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

func TestProgressiveParsing(t *testing.T) {
	// Create test content with multiple files
	files := createMultipleTestFiles()

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	viewport.SetSize(80, 25)

	// Start in progressive mode
	if !viewport.progressiveMode {
		t.Error("Expected progressive mode to be enabled by default")
	}

	// Verify syntax highlighting is enabled
	if !viewport.enableSyntaxHighlighting {
		t.Error("Expected syntax highlighting to be enabled")
	}

	// Test that first render doesn't parse all files
	start := time.Now()

	// Trigger first render by getting style spans for visible lines
	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		Line:      files[0].AlignedLines[0],
		FilePath:  "test0.go",
	}

	spans := viewport.getHighlightedStyleSpans("package main", "test0.go", false, lineInfo)

	// Should now have syntax highlighting on first render for first file
	if len(spans) == 0 {
		t.Error("Expected syntax highlighting on first render for visible file")
	} else {
		t.Logf("Got %d style spans on first render", len(spans))
	}

	firstRenderTime := time.Since(start)
	t.Logf("First render with visible lines took: %v", firstRenderTime)

	// Should be fast since it only processes visible content
	if firstRenderTime > 100*time.Millisecond {
		t.Errorf("First render too slow: %v (expected < 100ms)", firstRenderTime)
	}

	// Test incremental background parsing
	start = time.Now()
	parseCount := 0
	for {
		allDone := viewport.ParseNextFileInBackground()
		parseCount++
		if allDone {
			break
		}
		if parseCount > 50 { // Safety limit - more iterations needed for complete parsing
			t.Error("Background parsing didn't complete within expected iterations")
			break
		}
	}

	backgroundParseTime := time.Since(start)
	t.Logf("Background parsing %d files took: %v", len(files), backgroundParseTime)

	// Verify all files are now parsed
	if viewport.enhancedHighlighter != nil {
		for _, file := range files {
			if !viewport.enhancedHighlighter.IsFileParsed(file.FileDiff.NewPath) {
				t.Errorf("File %s was not parsed by background parsing", file.FileDiff.NewPath)
			}
		}
	}
}

func createMultipleTestFiles() []models.FileWithLines {
	var files []models.FileWithLines

	// Create 5 test files with different content
	for fileNum := 0; fileNum < 5; fileNum++ {
		var alignedLines []aligner.AlignedLine

		// Each file has 50 lines
		for i := 0; i < 50; i++ {
			lineNum := i + 1
			content := ""

			if i == 0 {
				content = "package main"
			} else if i == 1 {
				content = "import \"fmt\""
			} else {
				content = fmt.Sprintf("func test%d_%d() { fmt.Println(\"file %d line %d\") }", fileNum, i, fileNum, i)
			}

			alignedLines = append(alignedLines, aligner.AlignedLine{
				OldLine:    &content,
				NewLine:    &content,
				LineType:   aligner.Unchanged,
				OldLineNum: lineNum,
				NewLineNum: lineNum,
			})
		}

		fileName := fmt.Sprintf("test%d.go", fileNum)
		fileDiff := parser.FileDiff{
			OldPath: fileName,
			NewPath: fileName,
		}

		files = append(files, models.FileWithLines{
			FileDiff:     fileDiff,
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		})
	}

	return files
}
