package models

import (
	"testing"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/stretchr/testify/assert"
)

func TestNewDiffContent(t *testing.T) {
	// Create test data
	files := []FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test1.go",
				NewPath: "test1.go",
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("line 1"),
					NewLine:    stringPtr("line 1"),
					LineType:   aligner.Unchanged,
					OldLineNum: 1,
					NewLineNum: 1,
				},
				{
					OldLine:    stringPtr("old line 2"),
					NewLine:    stringPtr("new line 2"),
					LineType:   aligner.Modified,
					OldLineNum: 2,
					NewLineNum: 2,
				},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
		},
		{
			FileDiff: parser.FileDiff{
				OldPath: "test2.go",
				NewPath: "test2.go",
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("another line"),
					NewLine:    stringPtr("another line"),
					LineType:   aligner.Unchanged,
					OldLineNum: 1,
					NewLineNum: 1,
				},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
		},
	}

	content := NewDiffContent(files)

	// Should have: file1 header + 2 lines + separator + file2 header + 1 line = 6 total
	expectedTotalLines := 6
	assert.Equal(t, expectedTotalLines, content.TotalLines)
	assert.Len(t, content.Lines, expectedTotalLines)
	assert.Len(t, content.FileStartLines, 2)

	// Check file start positions
	assert.Equal(t, 0, content.FileStartLines[0]) // First file starts at line 0
	assert.Equal(t, 4, content.FileStartLines[1]) // Second file starts at line 4 (after separator)

	// Check line types
	assert.True(t, content.Lines[0].IsFileHeader())    // File 1 header
	assert.True(t, content.Lines[1].IsContentLine())   // File 1 line 1
	assert.True(t, content.Lines[2].IsContentLine())   // File 1 line 2
	assert.True(t, content.Lines[3].IsFileSeparator()) // Separator
	assert.True(t, content.Lines[4].IsFileHeader())    // File 2 header
	assert.True(t, content.Lines[5].IsContentLine())   // File 2 line 1
}

func TestGetVisibleLines(t *testing.T) {
	files := []FileWithLines{
		{
			FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
			AlignedLines: []aligner.AlignedLine{
				{OldLine: stringPtr("line 1"), NewLine: stringPtr("line 1"), LineType: aligner.Unchanged},
				{OldLine: stringPtr("line 2"), NewLine: stringPtr("line 2"), LineType: aligner.Unchanged},
				{OldLine: stringPtr("line 3"), NewLine: stringPtr("line 3"), LineType: aligner.Unchanged},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
		},
	}

	content := NewDiffContent(files)

	// Test normal case
	visible := content.GetVisibleLines(1, 2)
	assert.Len(t, visible, 2)
	assert.True(t, visible[0].IsContentLine())
	assert.True(t, visible[1].IsContentLine())

	// Test start beyond bounds
	visible = content.GetVisibleLines(100, 5)
	assert.Nil(t, visible)

	// Test count exceeds available lines
	visible = content.GetVisibleLines(0, 100)
	assert.Len(t, visible, content.TotalLines)
}

func TestLineInfoMethods(t *testing.T) {
	// Test file header
	headerLine := LineInfo{FileIndex: 0, LineIndex: -1}
	assert.True(t, headerLine.IsFileHeader())
	assert.False(t, headerLine.IsFileSeparator())
	assert.False(t, headerLine.IsContentLine())

	// Test file separator
	separatorLine := LineInfo{FileIndex: 0, LineIndex: -2}
	assert.False(t, separatorLine.IsFileHeader())
	assert.True(t, separatorLine.IsFileSeparator())
	assert.False(t, separatorLine.IsContentLine())

	// Test content line
	contentLine := LineInfo{FileIndex: 0, LineIndex: 5}
	assert.False(t, contentLine.IsFileHeader())
	assert.False(t, contentLine.IsFileSeparator())
	assert.True(t, contentLine.IsContentLine())
}

func TestSingleFileContent(t *testing.T) {
	files := []FileWithLines{
		{
			FileDiff: parser.FileDiff{OldPath: "single.go", NewPath: "single.go"},
			AlignedLines: []aligner.AlignedLine{
				{OldLine: stringPtr("only line"), NewLine: stringPtr("only line"), LineType: aligner.Unchanged},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
		},
	}

	content := NewDiffContent(files)

	// Should have: file header + 1 line = 2 total (no separator for single file)
	assert.Equal(t, 2, content.TotalLines)
	assert.Len(t, content.Lines, 2)
}

func TestEmptyFiles(t *testing.T) {
	content := NewDiffContent([]FileWithLines{})
	assert.Equal(t, 0, content.TotalLines)
	assert.Len(t, content.Lines, 0)
	assert.Len(t, content.FileStartLines, 0)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
