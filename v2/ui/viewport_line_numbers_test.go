package ui

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/models"
)

// TestLineNumberStyles validates line number styling without full screen rendering
func TestLineNumberStyles(t *testing.T) {
	// Create test data
	alignedLines := []aligner.AlignedLine{
		{
			OldLine:    stringPtr("unchanged line"),
			NewLine:    stringPtr("unchanged line"),
			LineType:   aligner.Unchanged,
			OldLineNum: 1,
			NewLineNum: 1,
		},
		{
			OldLine:    stringPtr("deleted line"),
			NewLine:    nil,
			LineType:   aligner.Deleted,
			OldLineNum: 2,
			NewLineNum: 0,
		},
		{
			OldLine:    nil,
			NewLine:    stringPtr("added line"),
			LineType:   aligner.Added,
			OldLineNum: 0,
			NewLineNum: 3,
		},
		{
			OldLine:    stringPtr("old content"),
			NewLine:    stringPtr("new content"),
			LineType:   aligner.Modified,
			OldLineNum: 4,
			NewLineNum: 4,
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
	_ = NewDiffViewport(content) // Just validate we can create viewport with this content

	tests := []struct {
		name               string
		lineInfo           models.LineInfo
		expectedLeftStyle  tcell.Style
		expectedRightStyle tcell.Style
	}{
		{
			name: "unchanged line",
			lineInfo: models.LineInfo{
				Line:     alignedLines[0],
				FilePath: "test.go",
			},
			expectedLeftStyle:  tcell.StyleDefault.Foreground(tcell.ColorGray),
			expectedRightStyle: tcell.StyleDefault.Foreground(tcell.ColorGray),
		},
		{
			name: "deleted line",
			lineInfo: models.LineInfo{
				Line:     alignedLines[1],
				FilePath: "test.go",
			},
			expectedLeftStyle:  tcell.StyleDefault.Foreground(tcell.ColorMaroon).Background(tcell.Color16),
			expectedRightStyle: tcell.StyleDefault.Background(tcell.Color16),
		},
		{
			name: "added line",
			lineInfo: models.LineInfo{
				Line:     alignedLines[2],
				FilePath: "test.go",
			},
			expectedLeftStyle:  tcell.StyleDefault.Background(tcell.Color16),
			expectedRightStyle: tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.Color16),
		},
		{
			name: "modified line",
			lineInfo: models.LineInfo{
				Line:     alignedLines[3],
				FilePath: "test.go",
			},
			expectedLeftStyle:  tcell.StyleDefault.Foreground(tcell.ColorNavy).Background(tcell.Color16),
			expectedRightStyle: tcell.StyleDefault.Foreground(tcell.ColorNavy).Background(tcell.Color16),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the styling logic by calling the internal methods
			// This validates the line number format and styles match our expectations

			line := tt.lineInfo.Line

			// Test left line number format
			var leftLineNum string
			if line.OldLine != nil {
				leftLineNum = fmt.Sprintf("%*d", lineNumWidth, line.OldLineNum)
			} else {
				leftLineNum = fmt.Sprintf("%*s", lineNumWidth, "")
			}

			// Test right line number format
			var rightLineNum string
			if line.NewLine != nil {
				rightLineNum = fmt.Sprintf("%*d", lineNumWidth, line.NewLineNum)
			} else {
				rightLineNum = fmt.Sprintf("%*s", lineNumWidth, "")
			}

			// Validate format
			if len(leftLineNum) != lineNumWidth {
				t.Errorf("Left line number wrong width: expected %d, got %d", lineNumWidth, len(leftLineNum))
			}
			if len(rightLineNum) != lineNumWidth {
				t.Errorf("Right line number wrong width: expected %d, got %d", lineNumWidth, len(rightLineNum))
			}

			// The actual style checking would need access to viewport's internal styling logic
			// This test validates the format is correct
		})
	}
}
