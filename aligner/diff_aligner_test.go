package aligner

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"diffyduck/parser"
)

func TestDiffAligner_AlignFile(t *testing.T) {
	tests := []struct {
		name          string
		oldLines      []string
		newLines      []string
		hunks         []parser.Hunk
		expectedLines []AlignedLine
	}{
		{
			name: "simple modification",
			oldLines: []string{
				"line1",
				"old line",
				"line3",
			},
			newLines: []string{
				"line1",
				"new line",
				"line3",
			},
			hunks: []parser.Hunk{
				{
					OldStart: 2,
					OldCount: 1,
					NewStart: 2,
					NewCount: 1,
					Lines: []string{
						"-old line",
						"+new line",
					},
				},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old line"), NewLine: stringPtr("new line"), LineType: Modified, OldLineNum: 2, NewLineNum: 2},
				{OldLine: stringPtr("line3"), NewLine: stringPtr("line3"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 3},
			},
		},
		{
			name: "line addition",
			oldLines: []string{
				"line1",
				"line2",
			},
			newLines: []string{
				"line1",
				"new line",
				"line2",
			},
			hunks: []parser.Hunk{
				{
					OldStart: 2,
					OldCount: 0,
					NewStart: 2,
					NewCount: 1,
					Lines: []string{
						"+new line",
					},
				},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: nil, NewLine: stringPtr("new line"), LineType: Added, OldLineNum: 0, NewLineNum: 2},
				{OldLine: stringPtr("line2"), NewLine: stringPtr("line2"), LineType: Unchanged, OldLineNum: 2, NewLineNum: 3},
			},
		},
		{
			name: "line deletion",
			oldLines: []string{
				"line1",
				"deleted line",
				"line2",
			},
			newLines: []string{
				"line1",
				"line2",
			},
			hunks: []parser.Hunk{
				{
					OldStart: 2,
					OldCount: 1,
					NewStart: 2,
					NewCount: 0,
					Lines: []string{
						"-deleted line",
					},
				},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("deleted line"), NewLine: nil, LineType: Deleted, OldLineNum: 2, NewLineNum: 0},
				{OldLine: stringPtr("line2"), NewLine: stringPtr("line2"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 2},
			},
		},
		{
			name: "multiple hunks",
			oldLines: []string{
				"line1",
				"old2",
				"line3",
				"line4",
				"old5",
			},
			newLines: []string{
				"line1",
				"new2",
				"line3",
				"line4",
				"new5",
			},
			hunks: []parser.Hunk{
				{
					OldStart: 2,
					OldCount: 1,
					NewStart: 2,
					NewCount: 1,
					Lines: []string{
						"-old2",
						"+new2",
					},
				},
				{
					OldStart: 5,
					OldCount: 1,
					NewStart: 5,
					NewCount: 1,
					Lines: []string{
						"-old5",
						"+new5",
					},
				},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old2"), NewLine: stringPtr("new2"), LineType: Modified, OldLineNum: 2, NewLineNum: 2},
				{OldLine: stringPtr("line3"), NewLine: stringPtr("line3"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 3},
				{OldLine: stringPtr("line4"), NewLine: stringPtr("line4"), LineType: Unchanged, OldLineNum: 4, NewLineNum: 4},
				{OldLine: stringPtr("old5"), NewLine: stringPtr("new5"), LineType: Modified, OldLineNum: 5, NewLineNum: 5},
			},
		},
		{
			name: "context lines in hunk",
			oldLines: []string{
				"context1",
				"old line",
				"context2",
			},
			newLines: []string{
				"context1",
				"new line",
				"context2",
			},
			hunks: []parser.Hunk{
				{
					OldStart: 1,
					OldCount: 3,
					NewStart: 1,
					NewCount: 3,
					Lines: []string{
						" context1",
						"-old line",
						"+new line",
						" context2",
					},
				},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("context1"), NewLine: stringPtr("context1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old line"), NewLine: stringPtr("new line"), LineType: Modified, OldLineNum: 2, NewLineNum: 2},
				{OldLine: stringPtr("context2"), NewLine: stringPtr("context2"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 3},
			},
		},
		{
			name:          "empty files",
			oldLines:      []string{},
			newLines:      []string{},
			hunks:         []parser.Hunk{},
			expectedLines: nil,
		},
		{
			name: "multiple consecutive deletions and additions",
			oldLines: []string{
				"line1",
				"del1",
				"del2",
				"line4",
			},
			newLines: []string{
				"line1",
				"add1",
				"add2",
				"line4",
			},
			hunks: []parser.Hunk{
				{
					OldStart: 2,
					OldCount: 2,
					NewStart: 2,
					NewCount: 2,
					Lines: []string{
						"-del1",
						"-del2",
						"+add1",
						"+add2",
					},
				},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("del1"), NewLine: stringPtr("add1"), LineType: Modified, OldLineNum: 2, NewLineNum: 2},
				{OldLine: stringPtr("del2"), NewLine: stringPtr("add2"), LineType: Modified, OldLineNum: 3, NewLineNum: 3},
				{OldLine: stringPtr("line4"), NewLine: stringPtr("line4"), LineType: Unchanged, OldLineNum: 4, NewLineNum: 4},
			},
		},
		{
			name: "unequal deletions and additions",
			oldLines: []string{
				"line1",
				"del1",
				"del2",
				"del3",
				"line5",
			},
			newLines: []string{
				"line1",
				"add1",
				"add2",
				"line5",
			},
			hunks: []parser.Hunk{
				{
					OldStart: 2,
					OldCount: 3,
					NewStart: 2,
					NewCount: 2,
					Lines: []string{
						"-del1",
						"-del2",
						"-del3",
						"+add1",
						"+add2",
					},
				},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("del1"), NewLine: stringPtr("add1"), LineType: Modified, OldLineNum: 2, NewLineNum: 2},
				{OldLine: stringPtr("del2"), NewLine: stringPtr("add2"), LineType: Modified, OldLineNum: 3, NewLineNum: 3},
				{OldLine: stringPtr("del3"), NewLine: nil, LineType: Deleted, OldLineNum: 4, NewLineNum: 0},
				{OldLine: stringPtr("line5"), NewLine: stringPtr("line5"), LineType: Unchanged, OldLineNum: 5, NewLineNum: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			aligner := NewDiffAligner()
			result := aligner.AlignFile(tt.oldLines, tt.newLines, tt.hunks)

			// Compare everything except WordDiff field (new feature)
			assert.Equal(t, len(tt.expectedLines), len(result))
			for i, expected := range tt.expectedLines {
				actual := result[i]
				assert.Equal(t, expected.OldLine, actual.OldLine)
				assert.Equal(t, expected.NewLine, actual.NewLine)
				assert.Equal(t, expected.LineType, actual.LineType)
				assert.Equal(t, expected.OldLineNum, actual.OldLineNum)
				assert.Equal(t, expected.NewLineNum, actual.NewLineNum)
				// WordDiff is ignored as it's new functionality
			}
		})
	}
}

func TestDiffAligner_addUnchangedLines(t *testing.T) {
	tests := []struct {
		name          string
		oldLines      []string
		newLines      []string
		oldStart      int
		newStart      int
		oldEnd        int
		newEnd        int
		expectedLines []AlignedLine
	}{
		{
			name:     "add unchanged lines from start",
			oldLines: []string{"line1", "line2", "line3"},
			newLines: []string{"line1", "line2", "line3"},
			oldStart: 1,
			newStart: 1,
			oldEnd:   3,
			newEnd:   3,
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("line2"), NewLine: stringPtr("line2"), LineType: Unchanged, OldLineNum: 2, NewLineNum: 2},
			},
		},
		{
			name:     "add unchanged lines in middle",
			oldLines: []string{"line1", "line2", "line3", "line4"},
			newLines: []string{"line1", "line2", "line3", "line4"},
			oldStart: 2,
			newStart: 2,
			oldEnd:   4,
			newEnd:   4,
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("line2"), NewLine: stringPtr("line2"), LineType: Unchanged, OldLineNum: 2, NewLineNum: 2},
				{OldLine: stringPtr("line3"), NewLine: stringPtr("line3"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 3},
			},
		},
		{
			name:          "no unchanged lines",
			oldLines:      []string{"line1", "line2"},
			newLines:      []string{"line1", "line2"},
			oldStart:      2,
			newStart:      2,
			oldEnd:        2,
			newEnd:        2,
			expectedLines: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			aligner := NewDiffAligner()
			result := aligner.addUnchangedLines(tt.oldLines, tt.newLines, tt.oldStart, tt.newStart, tt.oldEnd, tt.newEnd)

			assert.Equal(t, tt.expectedLines, result)
		})
	}
}

func TestDiffAligner_processHunk(t *testing.T) {
	tests := []struct {
		name           string
		hunk           parser.Hunk
		oldLines       []string
		newLines       []string
		oldPos         int
		newPos         int
		expectedLines  []AlignedLine
		expectedOldPos int
		expectedNewPos int
	}{
		{
			name: "simple hunk with deletion and addition",
			hunk: parser.Hunk{
				Lines: []string{
					"-old line",
					"+new line",
				},
			},
			oldLines: []string{"old line"},
			newLines: []string{"new line"},
			oldPos:   1,
			newPos:   1,
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("old line"), NewLine: nil, LineType: Deleted, OldLineNum: 1, NewLineNum: 0},
				{OldLine: nil, NewLine: stringPtr("new line"), LineType: Added, OldLineNum: 0, NewLineNum: 1},
			},
			expectedOldPos: 2,
			expectedNewPos: 2,
		},
		{
			name: "hunk with context lines",
			hunk: parser.Hunk{
				Lines: []string{
					" context",
					"-old line",
					"+new line",
					" context2",
				},
			},
			oldLines: []string{"context", "old line", "context2"},
			newLines: []string{"context", "new line", "context2"},
			oldPos:   1,
			newPos:   1,
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("context"), NewLine: stringPtr("context"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old line"), NewLine: nil, LineType: Deleted, OldLineNum: 2, NewLineNum: 0},
				{OldLine: nil, NewLine: stringPtr("new line"), LineType: Added, OldLineNum: 0, NewLineNum: 2},
				{OldLine: stringPtr("context2"), NewLine: stringPtr("context2"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 3},
			},
			expectedOldPos: 4,
			expectedNewPos: 4,
		},
		{
			name: "empty hunk",
			hunk: parser.Hunk{
				Lines: []string{},
			},
			oldLines:       []string{},
			newLines:       []string{},
			oldPos:         1,
			newPos:         1,
			expectedLines:  nil,
			expectedOldPos: 1,
			expectedNewPos: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			aligner := NewDiffAligner()
			result, oldPos, newPos := aligner.processHunk(tt.hunk, tt.oldLines, tt.newLines, tt.oldPos, tt.newPos)

			assert.Equal(t, tt.expectedLines, result)
			assert.Equal(t, tt.expectedOldPos, oldPos)
			assert.Equal(t, tt.expectedNewPos, newPos)
		})
	}
}

func TestDiffAligner_detectModifications(t *testing.T) {
	tests := []struct {
		name          string
		inputLines    []AlignedLine
		expectedLines []AlignedLine
	}{
		{
			name: "simple deletion followed by addition becomes modification",
			inputLines: []AlignedLine{
				{OldLine: stringPtr("old"), NewLine: nil, LineType: Deleted, OldLineNum: 1, NewLineNum: 0},
				{OldLine: nil, NewLine: stringPtr("new"), LineType: Added, OldLineNum: 0, NewLineNum: 1},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: Modified, OldLineNum: 1, NewLineNum: 1},
			},
		},
		{
			name: "multiple deletions and additions",
			inputLines: []AlignedLine{
				{OldLine: stringPtr("old1"), NewLine: nil, LineType: Deleted, OldLineNum: 1, NewLineNum: 0},
				{OldLine: stringPtr("old2"), NewLine: nil, LineType: Deleted, OldLineNum: 2, NewLineNum: 0},
				{OldLine: nil, NewLine: stringPtr("new1"), LineType: Added, OldLineNum: 0, NewLineNum: 1},
				{OldLine: nil, NewLine: stringPtr("new2"), LineType: Added, OldLineNum: 0, NewLineNum: 2},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("old1"), NewLine: stringPtr("new1"), LineType: Modified, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old2"), NewLine: stringPtr("new2"), LineType: Modified, OldLineNum: 2, NewLineNum: 2},
			},
		},
		{
			name: "more deletions than additions",
			inputLines: []AlignedLine{
				{OldLine: stringPtr("old1"), NewLine: nil, LineType: Deleted, OldLineNum: 1, NewLineNum: 0},
				{OldLine: stringPtr("old2"), NewLine: nil, LineType: Deleted, OldLineNum: 2, NewLineNum: 0},
				{OldLine: stringPtr("old3"), NewLine: nil, LineType: Deleted, OldLineNum: 3, NewLineNum: 0},
				{OldLine: nil, NewLine: stringPtr("new1"), LineType: Added, OldLineNum: 0, NewLineNum: 1},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("old1"), NewLine: stringPtr("new1"), LineType: Modified, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old2"), NewLine: nil, LineType: Deleted, OldLineNum: 2, NewLineNum: 0},
				{OldLine: stringPtr("old3"), NewLine: nil, LineType: Deleted, OldLineNum: 3, NewLineNum: 0},
			},
		},
		{
			name: "more additions than deletions",
			inputLines: []AlignedLine{
				{OldLine: stringPtr("old1"), NewLine: nil, LineType: Deleted, OldLineNum: 1, NewLineNum: 0},
				{OldLine: nil, NewLine: stringPtr("new1"), LineType: Added, OldLineNum: 0, NewLineNum: 1},
				{OldLine: nil, NewLine: stringPtr("new2"), LineType: Added, OldLineNum: 0, NewLineNum: 2},
				{OldLine: nil, NewLine: stringPtr("new3"), LineType: Added, OldLineNum: 0, NewLineNum: 3},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("old1"), NewLine: stringPtr("new1"), LineType: Modified, OldLineNum: 1, NewLineNum: 1},
				{OldLine: nil, NewLine: stringPtr("new2"), LineType: Added, OldLineNum: 0, NewLineNum: 2},
				{OldLine: nil, NewLine: stringPtr("new3"), LineType: Added, OldLineNum: 0, NewLineNum: 3},
			},
		},
		{
			name: "unchanged lines remain unchanged",
			inputLines: []AlignedLine{
				{OldLine: stringPtr("same"), NewLine: stringPtr("same"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("same"), NewLine: stringPtr("same"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
			},
		},
		{
			name: "mixed types",
			inputLines: []AlignedLine{
				{OldLine: stringPtr("same"), NewLine: stringPtr("same"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old"), NewLine: nil, LineType: Deleted, OldLineNum: 2, NewLineNum: 0},
				{OldLine: nil, NewLine: stringPtr("new"), LineType: Added, OldLineNum: 0, NewLineNum: 2},
				{OldLine: stringPtr("same2"), NewLine: stringPtr("same2"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 3},
			},
			expectedLines: []AlignedLine{
				{OldLine: stringPtr("same"), NewLine: stringPtr("same"), LineType: Unchanged, OldLineNum: 1, NewLineNum: 1},
				{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: Modified, OldLineNum: 2, NewLineNum: 2},
				{OldLine: stringPtr("same2"), NewLine: stringPtr("same2"), LineType: Unchanged, OldLineNum: 3, NewLineNum: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			aligner := NewDiffAligner()
			result := aligner.detectModifications(tt.inputLines)

			// Compare everything except WordDiff field (new feature)
			assert.Equal(t, len(tt.expectedLines), len(result))
			for i, expected := range tt.expectedLines {
				actual := result[i]
				assert.Equal(t, expected.OldLine, actual.OldLine)
				assert.Equal(t, expected.NewLine, actual.NewLine)
				assert.Equal(t, expected.LineType, actual.LineType)
				assert.Equal(t, expected.OldLineNum, actual.OldLineNum)
				assert.Equal(t, expected.NewLineNum, actual.NewLineNum)
				// WordDiff is ignored as it's new functionality
			}
		})
	}
}

func TestNewDiffAligner(t *testing.T) {
	aligner := NewDiffAligner()
	assert.NotNil(t, aligner)
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Benchmark tests
func BenchmarkDiffAligner_AlignFile(b *testing.B) {
	oldLines := []string{
		"line1",
		"old line",
		"line3",
		"line4",
		"line5",
	}

	newLines := []string{
		"line1",
		"new line",
		"line3",
		"line4",
		"line5",
	}

	hunks := []parser.Hunk{
		{
			OldStart: 2,
			OldCount: 1,
			NewStart: 2,
			NewCount: 1,
			Lines: []string{
				"-old line",
				"+new line",
			},
		},
	}

	aligner := NewDiffAligner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = aligner.AlignFile(oldLines, newLines, hunks)
	}
}

func BenchmarkDiffAligner_AlignFileLarge(b *testing.B) {
	// Create large files with many lines
	oldLines := make([]string, 1000)
	newLines := make([]string, 1000)

	for i := 0; i < 1000; i++ {
		if i == 500 {
			oldLines[i] = "old line 500"
			newLines[i] = "new line 500"
		} else {
			oldLines[i] = "line " + string(rune(i))
			newLines[i] = "line " + string(rune(i))
		}
	}

	hunks := []parser.Hunk{
		{
			OldStart: 501,
			OldCount: 1,
			NewStart: 501,
			NewCount: 1,
			Lines: []string{
				"-old line 500",
				"+new line 500",
			},
		},
	}

	aligner := NewDiffAligner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = aligner.AlignFile(oldLines, newLines, hunks)
	}
}
