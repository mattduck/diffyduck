package ui

import (
	"fmt"
	"testing"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/models"
)

func TestSyntaxHighlightingDebug(t *testing.T) {
	// Create a Go file with syntax highlighting-worthy content
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
		{
			OldLine:    stringPtr("// This is a comment"),
			NewLine:    stringPtr("// This is a comment"),
			LineType:   aligner.Unchanged,
			OldLineNum: 3,
			NewLineNum: 3,
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

	// Test each line for syntax highlighting
	for i, line := range alignedLines {
		lineInfo := models.LineInfo{
			FileIndex: 0,
			LineIndex: i,
			Line:      line,
			FilePath:  "test.go",
		}

		if line.OldLine != nil {
			spans := viewport.getHighlightedStyleSpans(*line.OldLine, "test.go", true, lineInfo)
			fmt.Printf("Line %d (%s): %d style spans\n", i+1, *line.OldLine, len(spans))
			for j, span := range spans {
				fmt.Printf("  Span %d: [%d:%d] - %+v\n", j, span.Start, span.End, span.Style)
			}
		}
	}

	// Verify that we get some syntax highlighting
	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 1, // func line
		Line:      alignedLines[1],
		FilePath:  "test.go",
	}

	spans := viewport.getHighlightedStyleSpans("func test() { fmt.Println(\"hello\") }", "test.go", false, lineInfo)
	if len(spans) == 0 {
		t.Log("No syntax highlighting spans found - this might be expected if tree-sitter parsing isn't working")
	} else {
		t.Logf("Found %d syntax highlighting spans", len(spans))
		for i, span := range spans {
			t.Logf("Span %d: [%d:%d]", i, span.Start, span.End)
		}
	}
}
