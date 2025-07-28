package ui

import (
	"testing"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/models"
)

func TestViewportWithSyntaxHighlighting(t *testing.T) {
	// Create Go file content with syntax highlighting
	alignedLines := []aligner.AlignedLine{
		{
			OldLine:    stringPtr("package main"),
			NewLine:    stringPtr("package main"),
			LineType:   aligner.Unchanged,
			OldLineNum: 1,
			NewLineNum: 1,
		},
		{
			OldLine:    stringPtr("func old() {}"),
			NewLine:    nil,
			LineType:   aligner.Deleted,
			OldLineNum: 2,
			NewLineNum: 0,
		},
		{
			OldLine:    nil,
			NewLine:    stringPtr("func new() {}"),
			LineType:   aligner.Added,
			OldLineNum: 0,
			NewLineNum: 2,
		},
		{
			OldLine:    stringPtr("// old comment"),
			NewLine:    stringPtr("// new comment"),
			LineType:   aligner.Modified,
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

	// Create diff content
	content := models.NewDiffContent(files)

	// Create viewport with enhanced highlighting
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// Verify viewport was created successfully
	if viewport == nil {
		t.Fatal("Expected non-nil viewport")
	}

	// Force complete highlighting for testing
	viewport.ForceCompleteHighlighting()

	if viewport.enhancedHighlighter == nil {
		t.Error("Expected enhanced highlighter to be initialized")
	}

	// Test that highlighting works without crashing
	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		Line:      alignedLines[0],
		FilePath:  "test.go",
	}

	spans := viewport.getHighlightedStyleSpans("package main", "test.go", false, lineInfo)
	if spans == nil {
		t.Error("Expected highlighting style spans")
	}

	// Verify highlighting works consistently
	spans2 := viewport.getHighlightedStyleSpans("package main", "test.go", false, lineInfo)
	if len(spans) != len(spans2) {
		t.Error("Style spans should be consistent")
	}
}

func TestViewportPreParsing(t *testing.T) {
	// Create simple Go file content
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

	// Create content and viewport
	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// Files should be pre-parsed during viewport creation
	// This should not crash and should be efficient on subsequent highlighting calls
	lineInfo := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		Line:      alignedLines[0],
		FilePath:  "test.go",
	}

	// Multiple calls should be fast due to parsing cache
	for i := 0; i < 10; i++ {
		spans := viewport.getHighlightedStyleSpans("package main", "test.go", false, lineInfo)
		if spans == nil {
			t.Errorf("Call %d returned no style spans", i)
		}
	}
}

func TestViewportBinaryFileHandling(t *testing.T) {
	// Create binary file content (should be skipped for highlighting)
	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "binary.bin",
				NewPath: "binary.bin",
			},
			AlignedLines: []aligner.AlignedLine{},
			OldFileType:  git.BinaryFile,
			NewFileType:  git.BinaryFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// Force complete highlighting for testing
	viewport.ForceCompleteHighlighting()

	// Should handle binary files gracefully
	if viewport.enhancedHighlighter == nil {
		t.Error("Expected enhanced highlighter to be initialized even for binary files")
	}
}

func TestViewportPerformanceMetrics(t *testing.T) {
	// Create content with multiple files
	files := []models.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "file1.go",
				NewPath: "file1.go",
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("package file1"),
					NewLine:    stringPtr("package file1"),
					LineType:   aligner.Unchanged,
					OldLineNum: 1,
					NewLineNum: 1,
				},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
		},
		{
			FileDiff: parser.FileDiff{
				OldPath: "file2.go",
				NewPath: "file2.go",
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("package file2"),
					NewLine:    stringPtr("package file2"),
					LineType:   aligner.Unchanged,
					OldLineNum: 1,
					NewLineNum: 1,
				},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
		},
	}

	content := models.NewDiffContent(files)
	viewport := NewDiffViewport(content)
	defer viewport.Close()

	// The enhanced highlighter should have parsed both files
	// Verify that line highlighting works for both files
	lineInfo1 := models.LineInfo{
		FileIndex: 0,
		LineIndex: 0,
		Line:      files[0].AlignedLines[0],
		FilePath:  "file1.go",
	}

	lineInfo2 := models.LineInfo{
		FileIndex: 1,
		LineIndex: 0,
		Line:      files[1].AlignedLines[0],
		FilePath:  "file2.go",
	}

	spans1 := viewport.getHighlightedStyleSpans("package file1", "file1.go", false, lineInfo1)
	spans2 := viewport.getHighlightedStyleSpans("package file2", "file2.go", false, lineInfo2)

	if spans1 == nil {
		t.Error("Expected highlighting spans for file1")
	}
	if spans2 == nil {
		t.Error("Expected highlighting spans for file2")
	}
}
