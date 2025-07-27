package models

import (
	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
)

// LineInfo represents a single line in the diff with all metadata
type LineInfo struct {
	FileIndex   int
	LineIndex   int // Index within the file's aligned lines
	Line        aligner.AlignedLine
	OldFileType git.FileType
	NewFileType git.FileType
	FilePath    string // For syntax highlighting
}

// DiffContent holds the flattened view of all diff lines for efficient viewport access
type DiffContent struct {
	Files          []FileWithLines // Original file data
	Lines          []LineInfo      // Flattened list of all lines
	FileStartLines []int           // Index in Lines where each file starts
	TotalLines     int             // Total number of lines
}

// FileWithLines represents a file and its diff lines
type FileWithLines struct {
	FileDiff     parser.FileDiff
	AlignedLines []aligner.AlignedLine
	OldFileType  git.FileType
	NewFileType  git.FileType
}

// NewDiffContent creates a new DiffContent from the provided files
func NewDiffContent(files []FileWithLines) *DiffContent {
	dc := &DiffContent{
		Files:          files,
		Lines:          make([]LineInfo, 0),
		FileStartLines: make([]int, len(files)),
	}

	dc.buildFlattenedView()
	return dc
}

// buildFlattenedView creates a flat list of all lines for O(1) viewport access
func (dc *DiffContent) buildFlattenedView() {
	lineIndex := 0

	for fileIndex, file := range dc.Files {
		dc.FileStartLines[fileIndex] = lineIndex

		// Add file header line
		dc.Lines = append(dc.Lines, LineInfo{
			FileIndex:   fileIndex,
			LineIndex:   -1, // Special marker for file header
			OldFileType: file.OldFileType,
			NewFileType: file.NewFileType,
			FilePath:    file.FileDiff.NewPath,
		})
		lineIndex++

		// Add all aligned lines for this file
		for alignedIndex, alignedLine := range file.AlignedLines {
			dc.Lines = append(dc.Lines, LineInfo{
				FileIndex:   fileIndex,
				LineIndex:   alignedIndex,
				Line:        alignedLine,
				OldFileType: file.OldFileType,
				NewFileType: file.NewFileType,
				FilePath:    file.FileDiff.NewPath,
			})
			lineIndex++
		}

		// Add separator line between files (except after last file)
		if fileIndex < len(dc.Files)-1 {
			dc.Lines = append(dc.Lines, LineInfo{
				FileIndex: fileIndex,
				LineIndex: -2, // Special marker for file separator
			})
			lineIndex++
		}
	}

	dc.TotalLines = lineIndex
}

// GetVisibleLines returns the lines that should be visible in the viewport
func (dc *DiffContent) GetVisibleLines(startLine, count int) []LineInfo {
	if startLine >= dc.TotalLines {
		return nil
	}

	endLine := startLine + count
	if endLine > dc.TotalLines {
		endLine = dc.TotalLines
	}

	return dc.Lines[startLine:endLine]
}

// IsFileHeader returns true if the line is a file header
func (li *LineInfo) IsFileHeader() bool {
	return li.LineIndex == -1
}

// IsFileSeparator returns true if the line is a file separator
func (li *LineInfo) IsFileSeparator() bool {
	return li.LineIndex == -2
}

// IsContentLine returns true if the line contains actual diff content
func (li *LineInfo) IsContentLine() bool {
	return li.LineIndex >= 0
}
